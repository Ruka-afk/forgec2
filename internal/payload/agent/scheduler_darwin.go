//go:build darwin

package main

// userIdleSeconds returns seconds since last user input on macOS. A precise
// reading requires the CoreGraphics HID idle time (objc bridge); we conservatively
// report "unknown" so beacon cadence is not shortened on hosts we cannot probe,
// keeping behaviour safe rather than falsely accelerating check-ins.
func userIdleSeconds() int {
	return 9999
}
