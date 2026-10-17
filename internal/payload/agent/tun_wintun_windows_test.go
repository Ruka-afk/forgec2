//go:build windows && wintun

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wintun"
)

// These tests only run on a Windows host that built the agent with
// `-tags wintun` (the tag requires the staged runtime; see
// scripts/fetch-wintun.ps1). The live cases are opt-in because they load the
// real DLL; none of them needs elevation.

func TestStartWintunRejectsBadCidr(t *testing.T) {
	for _, cidr := range []string{"", "not-a-cidr", "fd00::2/64"} {
		if _, err := startAgentTUN(cidr); err == nil {
			t.Fatalf("startAgentTUN(%q) accepted; want a validation error", cidr)
		}
	}
}

func requireLiveWintun(t *testing.T) {
	t.Helper()
	if os.Getenv("FORGEC2_WINTUN_LIVE_TEST") != "1" {
		t.Skip("set FORGEC2_WINTUN_LIVE_TEST=1 to exercise the real Wintun runtime")
	}
}

// signaturePolicyFlags reads PROCESS_MITIGATION_BINARY_SIGNATURE_POLICY (class
// 8) for the current process. Bit 0 (MicrosoftSignedOnly) is what the agent's
// adaptive evasion applies through blockDLLs().
func signaturePolicyFlags(t *testing.T) uint32 {
	t.Helper()
	k32 := syscall.NewLazyDLL("kernel32.dll")
	proc := k32.NewProc("GetProcessMitigationPolicy")
	var flags uint32
	ret, _, err := proc.Call(
		uintptr(windows.CurrentProcess()),
		8,
		uintptr(unsafe.Pointer(&flags)),
		unsafe.Sizeof(flags),
	)
	if ret == 0 {
		t.Skipf("GetProcessMitigationPolicy unavailable: %v", err)
	}
	return flags
}

// TestWintunPreloadSurvivesHardening pins the hardening preload invariant: the
// test binary runs the same package init() as the implant, so on hosts where
// the evasion strategy applied MicrosoftSignedOnly before any test ran, the
// runtime must already be bound by the blockDLLs() hook — a fresh
// non-Microsoft-signed load is refused from that point on. The control load
// proves the refusal is actually active.
func TestWintunPreloadSurvivesHardening(t *testing.T) {
	flags := signaturePolicyFlags(t)
	if flags&1 == 0 {
		t.Skipf("process signature policy not active (flags=0x%x); nothing to survive", flags)
	}
	if v := wintun.Version(); v == "" || v == "unknown" {
		t.Fatalf("process is signature-hardened (flags=0x%x) but the Wintun runtime is not bound; blockDLLs() preload did not run", flags)
	}
	// Control: a fresh copy under a different base name must still be refused,
	// i.e. the loader really is under MicrosoftSignedOnly and the binding works
	// only because the runtime was loaded before the policy took effect.
	fresh := filepath.Join(t.TempDir(), "wintun-fresh-copy.dll")
	if err := os.WriteFile(fresh, wintunDLLBytes, 0600); err != nil {
		t.Fatal(err)
	}
	if h, err := windows.LoadLibraryEx(fresh, 0, windows.LOAD_LIBRARY_SEARCH_DLL_LOAD_DIR|windows.LOAD_LIBRARY_SEARCH_SYSTEM32); err == nil {
		_ = windows.FreeLibrary(h)
		t.Fatal("a fresh non-Microsoft-signed image was loadable under the signature policy")
	}
}

// TestPreloadWintunFallback exercises the read-only-implant-directory fallback
// helper: the runtime is loaded from a scratch directory by full path and the
// binding must then see it. Only meaningful while the process is not
// signature-hardened; afterwards a fresh same-image load is refused by design
// (covered by TestWintunPreloadSurvivesHardening).
func TestPreloadWintunFallback(t *testing.T) {
	requireLiveWintun(t)
	if flags := signaturePolicyFlags(t); flags&1 != 0 {
		t.Skipf("process is signature-hardened (flags=0x%x); fresh loads are refused by design", flags)
	}
	dll := filepath.Join(t.TempDir(), "wintun.dll")
	if err := os.WriteFile(dll, wintunDLLBytes, 0600); err != nil {
		t.Fatal(err)
	}
	if err := preloadWintunDLL(dll); err != nil {
		t.Fatalf("preloadWintunDLL: %v", err)
	}
	if v := wintun.Version(); v == "" || v == "unknown" {
		t.Fatalf("wintun.dll not found after preload (Version()=%q)", v)
	}
}

// TestWintunLiveAdapter stages the embedded runtime and drives a real
// startAgentTUN. Unelevated hosts must fail with the honest elevation error;
// elevated hosts must come up, report the adapter, and tear down cleanly.
//
// The elevated lab run on 2026-10-17 caught a teardown race on the first stop
// (session.End() while the read loop was still inside ReceivePacket; the
// in-flight DLL call faulted on the freed session with 0xc0000005), so the
// stop path is now serialized and this test deliberately runs several
// start/stop cycles to keep that class of race caught.
func TestWintunLiveAdapter(t *testing.T) {
	requireLiveWintun(t)
	if err := stageWintunDLL(wintunDLLBytes); err != nil {
		t.Fatalf("stageWintunDLL: %v", err)
	}
	if v := wintun.Version(); v == "" || v == "unknown" {
		t.Fatalf("staged wintun.dll did not load (Version()=%q)", v)
	}
	for cycle := 0; cycle < 3; cycle++ {
		msg, err := startAgentTUN("10.66.0.2/24")
		if err != nil {
			var errno syscall.Errno
			if !errors.As(err, &errno) || (errno != windows.ERROR_ACCESS_DENIED && errno != windows.ERROR_ELEVATION_REQUIRED) {
				t.Fatalf("cycle %d: startAgentTUN failed for a non-elevation reason: %v", cycle, err)
			}
			if !strings.Contains(err.Error(), "elevated") {
				t.Fatalf("cycle %d: elevation failure lacks the actionable hint: %v", cycle, err)
			}
			t.Logf("adapter creation refused without elevation (expected on this host): %v", err)
			return
		}
		if !strings.Contains(msg, "fc2tun0 up") || !strings.Contains(msg, "10.66.0.2") {
			t.Fatalf("cycle %d: unexpected start message: %q", cycle, msg)
		}
		if err := stopAgentTUN(); err != nil {
			t.Fatalf("cycle %d: stopAgentTUN: %v", cycle, err)
		}
	}
}
