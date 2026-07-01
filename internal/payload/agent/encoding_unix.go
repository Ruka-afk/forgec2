//go:build !windows
// +build !windows

package main

func decodeShellOutput(raw []byte, shell string) string {
	return string(raw)
}