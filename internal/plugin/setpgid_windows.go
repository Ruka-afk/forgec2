//go:build windows
// +build windows

package plugin

import "os/exec"

// isolateProcessGroup is a no-op on Windows — process trees are bounded by
// the Job Object created in attachProcessGuard (job_windows.go).
func isolateProcessGroup(cmd *exec.Cmd) {}
