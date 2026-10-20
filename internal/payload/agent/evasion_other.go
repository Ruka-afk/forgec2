//go:build !windows
// +build !windows

package main

import "time"

// sleepObfuscated falls back to a single sleep on non-Windows platforms.
func sleepObfuscated(total time.Duration) {
	// Non-Windows builds use a plain sleep; keep it interruptible for the same
	// network-change/ghost wakes the regular path observes.
	sleepInterruptible(total)
}
