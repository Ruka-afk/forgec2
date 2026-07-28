//go:build !windows
// +build !windows

package main

import "fmt"

func debugLog(msg string) {
	if Debug {
		fmt.Printf("[*] %s\n", msg)
	}
}

func logDebug(msg string) {
	debugLog(msg)
}

func logDebugf(format string, args ...interface{}) {
	if Debug {
		fmt.Printf("[*] "+format+"\n", args...)
	}
}
