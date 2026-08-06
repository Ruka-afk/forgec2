//go:build windows || darwin
// +build windows darwin

package main

import (
	"fmt"
	"os/exec"
	"runtime"
)

// startInteractiveShell spawns a persistent interactive shell. Linux has its
// own implementation (shell_interactive_linux.go) with process-group isolation.
func startInteractiveShell(shellType string) (*InteractiveShell, error) {
	var shellPath string
	var shellArgs []string

	switch runtime.GOOS {
	case "windows":
		switch shellType {
		case "powershell", "pwsh":
			shellPath = "powershell.exe"
			shellArgs = []string{"-NoLogo", "-NoProfile", "-NonInteractive"}
		default:
			shellPath = "cmd.exe"
			shellArgs = []string{"/Q"}
		}
	default:
		switch shellType {
		case "zsh":
			shellPath = "/bin/zsh"
		default:
			shellPath = "/bin/bash"
		}
		shellArgs = []string{"--norc", "--noediting"}
	}

	cmd := exec.Command(shellPath, shellArgs...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if runtime.GOOS == "windows" {
		applyHideWindow(cmd)
	}

	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return nil, fmt.Errorf("start shell: %w", err)
	}

	ish := &InteractiveShell{
		shellID:   nextShellID(),
		PID:       cmd.Process.Pid,
		active:    true,
		shellType: shellType,
		cmd:       cmd,
		stdin:     stdin,
		stdout:    stdout,
		stderr:    stderr,
		closeCh:   make(chan struct{}),
	}

	setShell(ish.shellID, ish)

	go ish.readOutput()

	return ish, nil
}