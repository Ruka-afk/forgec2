//go:build !windows

package server

import (
	"os/exec"
	"syscall"
)

// restartCommand builds the command that runs the generated restarter.
func restartCommand(scriptPath string) (*exec.Cmd, error) {
	return exec.Command("/bin/sh", scriptPath), nil
}

// detachCommand keeps the restarter alive when this process exits and stops
// it from receiving the terminal's interrupt: the restarter outlives the
// server it replaces.
func detachCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
