//go:build !windows

package main

// runBeaconSendSpoofed runs fn inline on non-Windows platforms. The native
// call-stack spoof technique targets the Windows userland stack-walker.
func runBeaconSendSpoofed(fn func()) {
	fn()
}
