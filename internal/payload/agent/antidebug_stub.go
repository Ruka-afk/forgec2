//go:build !windows
// +build !windows

package main

// AntiDebugCheck is not implemented on non-Windows platforms.
// Returns 0 (no debugger detected) with a nil technique map.
func AntiDebugCheck() (int32, map[string]bool) {
	return 0, nil
}

func runAntiDebugMonitor() {
}
