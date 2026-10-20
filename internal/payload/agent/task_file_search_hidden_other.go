//go:build linux || darwin
// +build linux darwin

package main

import "strings"

// fileSearchIsHidden is the portable hidden check: dot-prefixed names only.
// Linux and macOS expose no single equivalent of the Windows hidden attribute.
func fileSearchIsHidden(_ string, name string) bool {
	return strings.HasPrefix(name, ".")
}
