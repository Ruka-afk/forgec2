//go:build linux
// +build linux

package main

func decodeShellOutput(raw []byte, shell string) string {
	return string(raw)
}