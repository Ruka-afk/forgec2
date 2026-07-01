//go:build !windows
// +build !windows

package main

import "time"

// sleepObfuscated falls back to a single sleep on non-Windows platforms.
func sleepObfuscated(total time.Duration) {
	time.Sleep(total)
}