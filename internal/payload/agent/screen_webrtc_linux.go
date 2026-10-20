//go:build linux
// +build linux

package main

import (
	"strconv"
	"strings"
)

// nativeScreenRect returns the X11 display size used by the ffmpeg x11grab
// input. linuxScreenSize() already parses the first connected xrandr output.
func nativeScreenRect() (x, y, w, h int, ok bool) {
	size := strings.TrimSpace(linuxScreenSize())
	parts := strings.SplitN(size, "x", 2)
	if len(parts) != 2 {
		return 0, 0, 0, 0, false
	}
	width, errW := strconv.Atoi(parts[0])
	height, errH := strconv.Atoi(parts[1])
	if errW != nil || errH != nil || width <= 0 || height <= 0 {
		return 0, 0, 0, 0, false
	}
	return 0, 0, width, height, true
}
