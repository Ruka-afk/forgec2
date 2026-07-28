//go:build !windows
// +build !windows

package main

func AntiDebugCheck() (int32, map[string]bool) {
	return 0, nil
}

func runAntiDebugMonitor() {
}
