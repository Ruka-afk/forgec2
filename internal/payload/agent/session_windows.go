//go:build windows
// +build windows

package main

import (
	"fmt"
	"os/exec"
	"strings"
	"unsafe"
)

var (
	procGetLastInputInfo           = user32.NewProc("GetLastInputInfo")
	procGetTickCount               = k32.NewProc("GetTickCount")
	procWTSGetActiveConsoleSession = k32.NewProc("WTSGetActiveConsoleSessionId")
)

type lastInputInfo struct {
	cbSize uint32
	dwTime uint32
}

func sessionRecon() (string, error) {
	var sb strings.Builder
	sb.WriteString("=== session recon (Windows) ===\n")

	console, _, _ := procWTSGetActiveConsoleSession.Call()
	fmt.Fprintf(&sb, "console_session_id=%d\n", uint32(console))
	fmt.Fprintf(&sb, "active_window=%s\n", getActiveWindowTitle())
	if idle, ok := windowsIdle(); ok {
		fmt.Fprintf(&sb, "idle_ms=%d\n", idle)
	} else {
		sb.WriteString("idle_ms=(GetLastInputInfo failed)\n")
	}

	for _, args := range [][]string{
		{"query", "user"},
		{"qwinsta"},
		{"whoami", "/upn"},
		{"whoami"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		applyHideWindow(cmd)
		out, err := cmd.CombinedOutput()
		fmt.Fprintf(&sb, "\n--- %s ---\n", strings.Join(args, " "))
		if len(out) > 0 {
			sb.Write(out)
			if !strings.HasSuffix(string(out), "\n") {
				sb.WriteByte('\n')
			}
		}
		if err != nil && len(out) == 0 {
			fmt.Fprintf(&sb, "(failed: %v)\n", err)
		}
	}
	return sb.String(), nil
}

func windowsIdle() (uint32, bool) {
	info := lastInputInfo{cbSize: uint32(unsafe.Sizeof(lastInputInfo{}))}
	r, _, _ := procGetLastInputInfo.Call(uintptr(unsafe.Pointer(&info)))
	if r == 0 {
		return 0, false
	}
	tick, _, _ := procGetTickCount.Call()
	idle := uint32(tick) - info.dwTime
	return idle, true
}
