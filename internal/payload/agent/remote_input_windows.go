//go:build windows
// +build windows

package main

import "fmt"

// remoteInputStub logs remote desktop input events.
// Future work: parse JSON {type,x,y,key} and inject via SendInput / mouse_event / keybd_event.
func remoteInputStub(payload string) string {
	if Debug {
		fmt.Printf("[*] remote_input stub received: %s\n", payload)
	}
	return "remote_input: stub (log only — SendInput/mouse_event/keybd_event not implemented yet)"
}