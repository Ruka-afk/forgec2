//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"fmt"
	"runtime"
)

// ── Window management (gh0st C_SYSTEM parity) ──────────────────────────────

func handleWindowList(task Task, res *TaskResult) {
	out, err := listWindows() //nolint:staticcheck // dual-GOOS dispatch: one target's impl makes this branch statically dead; the other can genuinely fail
	if err != nil {           //nolint:staticcheck // dual-GOOS dispatch: one target's impl makes this branch statically dead; the other can genuinely fail
		res.Error = err.Error()
	} else {
		res.Output = out
	}
}

func handleWindowClose(task Task, res *TaskResult) {
	out, err := closeWindow(task.Command) //nolint:staticcheck // dual-GOOS dispatch: one target's impl makes this branch statically dead; the other can genuinely fail
	if err != nil {                       //nolint:staticcheck // dual-GOOS dispatch: one target's impl makes this branch statically dead; the other can genuinely fail
		res.Error = err.Error()
	} else {
		res.Output = out
	}
}

func listWindows() (string, error) { //nolint:staticcheck // dual-GOOS dispatch: one target's impl makes this branch statically dead; the other can genuinely fail
	if runtime.GOOS == "windows" {
		return listWindowsWindows()
	}
	return "", fmt.Errorf("window_list only on Windows")
}

func closeWindow(target string) (string, error) { //nolint:staticcheck // dual-GOOS dispatch: one target's impl makes this branch statically dead; the other can genuinely fail
	if runtime.GOOS == "windows" {
		return closeWindowWindows(target)
	}
	return "", fmt.Errorf("window_close only on Windows")
}
