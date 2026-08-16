//go:build windows && arm64

package main

// runBeaconSendSpoofed runs fn inline on arm64 Windows: the native-thread
// call-stack spoof technique used on amd64 is not available on arm64, so the
// beacon send executes normally. Stack spoofing for injection syscalls is also
// disabled on arm64 (see syscall_stubs_windows_arm64.go).
func runBeaconSendSpoofed(fn func()) {
	fn()
}
