//go:build !windows
// +build !windows

package main

import "fmt"

func debugLog(msg string) {
	if Debug {
		fmt.Printf("[*] %s\n", msg)
	}
}