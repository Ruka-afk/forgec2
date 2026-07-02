//go:build !windows && !linux
// +build !windows,!linux

package main

import "fmt"

func sendICMPBeacon(body []byte) []byte {
	fmt.Println("[!] ICMP transport not available (excluded from build)")
	return nil
}
