//go:build !windows
// +build !windows

package plugin

import (
	"os/exec"
	"syscall"
)

// isolateProcessGroup asks the child to start in its own process group so
// guard.release can signal the whole tree with kill(-pgid, …).
func isolateProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}
