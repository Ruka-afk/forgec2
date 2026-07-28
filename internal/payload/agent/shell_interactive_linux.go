//go:build linux
// +build linux

package main

import (
	"os/exec"
	"syscall"
)

func startInteractiveShell(shellType string) (*InteractiveShell, error) {
	shellID := nextShellID()
	shellPath := "/bin/bash"
	args := []string{"-i"}
	if shellType == "sh" {
		shellPath = "/bin/sh"
		args = []string{"-i"}
	}

	cmd := exec.Command(shellPath, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	ish := &InteractiveShell{
		shellID:   shellID,
		PID:       cmd.Process.Pid,
		active:    true,
		shellType: shellType,
		cmd:       cmd,
		stdin:     stdin,
		stdout:    stdout,
		stderr:    stderr,
		closeCh:   make(chan struct{}),
	}

	setShell(shellID, ish)

	go ish.readOutput()

	return ish, nil
}
