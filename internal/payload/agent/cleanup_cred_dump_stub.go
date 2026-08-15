//go:build !windows
// +build !windows

package main

// cleanupCredDumpFiles is Windows-specific (it removes LSASS/SAM dumps written
// by dumpCreds). On non-Windows builds there are no such artifacts, so this is
// a no-op stub so the cross-platform uninstall path compiles.
func cleanupCredDumpFiles() {}
