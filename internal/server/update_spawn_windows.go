//go:build windows

package server

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// restartCommand builds the command that runs the generated restarter.
//
// The restarter is a PowerShell script: the wait-for-exit loop, the retry
// around the file swap and the rollback are far easier to get right there
// than in cmd batch syntax, and PowerShell ships with every supported
// Windows release.
func restartCommand(scriptPath string) (*exec.Cmd, error) {
	shell, err := powershellPath()
	if err != nil {
		return nil, err
	}
	return exec.Command(shell, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", scriptPath), nil
}

// powershellPath locates the PowerShell interpreter.
func powershellPath() (string, error) {
	if p, err := exec.LookPath("powershell.exe"); err == nil {
		return p, nil
	}
	system := os.Getenv("SystemRoot")
	if system == "" {
		system = `C:\Windows`
	}
	candidate := filepath.Join(system, "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	if _, err := os.Stat(candidate); err == nil {
		return candidate, nil
	}
	return "", fmt.Errorf("powershell.exe not found: the hot update needs it to run the restart script")
}

// detachCommand keeps the restarter alive when this process exits.
func detachCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}
