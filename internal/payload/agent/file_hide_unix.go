//go:build linux || darwin
// +build linux darwin

package main

// setHidden is only invoked on Windows (guarded by runtime.GOOS checks), but
// the symbol must exist for Unix builds. Dot-hiding is a convention worth
// honoring on Unix where an operator passes an explicit path; keep this a
// no-op so the artifact path is never altered under our feet.
func setHidden(path string) {
	_ = path
}