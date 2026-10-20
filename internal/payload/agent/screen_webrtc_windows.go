//go:build windows
// +build windows

package main

// nativeScreenRect returns the virtual desktop rectangle in the coordinate
// space used by SetCursorPos. gdigrab's "desktop" input captures the same
// virtual desktop, so live pointer coordinates can be mapped from the encoded
// video back to native pixels.
func nativeScreenRect() (x, y, w, h int, ok bool) {
	vx, vy, vw, vh := getVirtualScreen()
	if vw <= 0 || vh <= 0 {
		return 0, 0, 0, 0, false
	}
	return int(vx), int(vy), int(vw), int(vh), true
}
