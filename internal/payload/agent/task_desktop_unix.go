//go:build linux || darwin
// +build linux darwin

package main

import "errors"

// setDesktopWallpaper is Windows-only. handleWallpaper rejects non-Windows
// before reaching here; this stub exists so the shared dispatch file
// compiles on every platform.
func setDesktopWallpaper(imgPath, style string) error {
	return errors.New("wallpaper is only supported on Windows agents")
}
