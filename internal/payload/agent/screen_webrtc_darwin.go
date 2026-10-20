//go:build darwin
// +build darwin

package main

// nativeScreenRect is unavailable for the avfoundation capture path in this
// pass. Live input falls back to the existing macOS keystroke injector, which
// does not consume pointer coordinates.
func nativeScreenRect() (x, y, w, h int, ok bool) {
	return 0, 0, 0, 0, false
}
