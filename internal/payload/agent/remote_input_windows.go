//go:build windows
// +build windows

package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"syscall"
)

var (
	user32Input        = syscall.NewLazyDLL("user32.dll")
	procSetCursorPos   = user32Input.NewProc("SetCursorPos")
	procMouseEvent     = user32Input.NewProc("mouse_event")
	procKeybdEvent     = user32Input.NewProc("keybd_event")
)

type remoteInputEvent struct {
	Type string `json:"type"`
	X    int    `json:"x"`
	Y    int    `json:"y"`
	Key  string `json:"key"`
}

const (
	mouseEventLeftDown = 0x0002
	mouseEventLeftUp   = 0x0004
	keyEventKeyUp      = 0x0002
)

func remoteInputStub(payload string) string {
	var ev remoteInputEvent
	if err := json.Unmarshal([]byte(payload), &ev); err != nil {
		return "remote_input: invalid json: " + err.Error()
	}

	switch strings.ToLower(ev.Type) {
	case "move":
		procSetCursorPos.Call(uintptr(ev.X), uintptr(ev.Y))
		return fmt.Sprintf("remote_input: move to (%d,%d)", ev.X, ev.Y)
	case "click":
		procSetCursorPos.Call(uintptr(ev.X), uintptr(ev.Y))
		procMouseEvent.Call(uintptr(mouseEventLeftDown), 0, 0, 0, 0)
		procMouseEvent.Call(uintptr(mouseEventLeftUp), 0, 0, 0, 0)
		return fmt.Sprintf("remote_input: click at (%d,%d)", ev.X, ev.Y)
	case "key":
		vk, ok := keyToVK(ev.Key)
		if !ok {
			return "remote_input: unknown key: " + ev.Key
		}
		procKeybdEvent.Call(uintptr(vk), 0, 0, 0)
		procKeybdEvent.Call(uintptr(vk), 0, uintptr(keyEventKeyUp), 0)
		return fmt.Sprintf("remote_input: key %s (vk=%d)", ev.Key, vk)
	default:
		return "remote_input: unknown type: " + ev.Type
	}
}

func keyToVK(key string) (uint16, bool) {
	k := strings.TrimSpace(strings.ToLower(key))
	if len(k) == 1 {
		ch := k[0]
		if ch >= 'a' && ch <= 'z' {
			return uint16(strings.ToUpper(k)[0]), true
		}
		if ch >= '0' && ch <= '9' {
			return uint16(ch), true
		}
	}
	named := map[string]uint16{
		"enter": 0x0D, "return": 0x0D, "tab": 0x09, "space": 0x20,
		"backspace": 0x08, "escape": 0x1B, "esc": 0x1B,
		"left": 0x25, "up": 0x26, "right": 0x27, "down": 0x28,
		"delete": 0x2E, "home": 0x24, "end": 0x23,
		"pageup": 0x21, "pagedown": 0x22,
		"f1": 0x70, "f2": 0x71, "f3": 0x72, "f4": 0x73,
		"f5": 0x74, "f6": 0x75, "f7": 0x76, "f8": 0x77,
		"f9": 0x78, "f10": 0x79, "f11": 0x7A, "f12": 0x7B,
	}
	if vk, ok := named[k]; ok {
		return vk, true
	}
	return 0, false
}