//go:build linux || darwin
// +build linux darwin

package main

// lastHollowHost mirrors inject_host_windows.go for non-Windows builds so
// task_injection.go compiles everywhere. Hollow injection is Windows-only;
// this stays empty off-Windows.
var lastHollowHost string
