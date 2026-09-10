//go:build linux || darwin
// +build linux darwin

package main

import "fmt"

// Non-Windows stubs for the EDR pack. Handlers already gate on
// runtime.GOOS, so these are unreachable off-Windows; they exist so the
// shared edr_pack.go compiles on every platform.

func edrBlind() string {
	return "edr_blind is Windows-only"
}

func byovdLoadDriver(driverPath, svcName string) (string, error) {
	return "", fmt.Errorf("byovd_load is Windows-only")
}

func pplProtectionLevel() string {
	return "ppl_check is Windows-only"
}
