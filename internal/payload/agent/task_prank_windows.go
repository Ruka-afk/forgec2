//go:build windows
// +build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	winmm   = syscall.NewLazyDLL("winmm.dll")
	shell32 = syscall.NewLazyDLL("shell32.dll")
)

// ── Wallpaper ───────────────────────────────────────────────────────────

func setWallpaperWindows(path string) string {
	proc := user32.NewProc("SystemParametersInfoW")
	pathPtr, _ := syscall.UTF16PtrFromString(path)
	ret, _, _ := proc.Call(0x0072, 0, uintptr(unsafe.Pointer(pathPtr)), 0x0003)
	if ret == 0 {
		return "SystemParametersInfoW failed"
	}
	return "wallpaper changed"
}

// ── MessageBox ──────────────────────────────────────────────────────────

func showMsgBoxWindows(msg, title string) string {
	proc := user32.NewProc("MessageBoxW")
	msgPtr, _ := syscall.UTF16PtrFromString(msg)
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	proc.Call(0, uintptr(unsafe.Pointer(msgPtr)), uintptr(unsafe.Pointer(titlePtr)), 0x00000040)
	return "MessageBox displayed"
}

// ── Play Sound ──────────────────────────────────────────────────────────

func playBeepWindows() string {
	proc := kernel32.NewProc("Beep")
	proc.Call(800, 300)
	return "beep played"
}

func playSoundWindows(path string) string {
	proc := winmm.NewProc("PlaySoundW")
	pathPtr, _ := syscall.UTF16PtrFromString(path)
	ret, _, _ := proc.Call(uintptr(unsafe.Pointer(pathPtr)), 0, 0x00020001)
	if ret == 0 {
		return "PlaySound failed (file not found?)"
	}
	return "sound played"
}

// ── Open URL ────────────────────────────────────────────────────────────

func openURLWindows(url string) string {
	proc := shell32.NewProc("ShellExecuteW")
	urlPtr, _ := syscall.UTF16PtrFromString(url)
	verbPtr, _ := syscall.UTF16PtrFromString("open")
	ret, _, _ := proc.Call(0, uintptr(unsafe.Pointer(verbPtr)), uintptr(unsafe.Pointer(urlPtr)), 0, 0, 0x00000001)
	if ret <= 32 {
		return "ShellExecuteW failed"
	}
	return "URL opened"
}

// ── Screen Rotate ───────────────────────────────────────────────────────

func screenRotateWindows() string {
	enumProc := user32.NewProc("EnumDisplaySettingsW")
	changeProc := user32.NewProc("ChangeDisplaySettingsExW")

	type devmode struct {
		_            [68]byte
		dmSize       uint16
		_            [16]byte
		dmFields     uint32
		dmPositionX  int32
		dmPositionY  int32
		_            [4]byte
		dmPelsWidth  uint32
		dmPelsHeight uint32
		_            [8]byte
		dmDisplayOrientation int32
		_            [20]byte
	}

	var dm devmode
	dm.dmSize = uint16(unsafe.Sizeof(dm))

	ret, _, _ := enumProc.Call(0, 0xFFFFFFFF, uintptr(unsafe.Pointer(&dm)), 0)
	if ret == 0 {
		return "EnumDisplaySettingsW failed"
	}

	if dm.dmDisplayOrientation == 2 {
		dm.dmDisplayOrientation = 0
		dm.dmFields &^= 0x80
	} else {
		dm.dmDisplayOrientation = 2
		dm.dmFields |= 0x80
	}

	ret2, _, _ := changeProc.Call(0, 0, uintptr(unsafe.Pointer(&dm)), 0)
	if ret2 != 0 {
		return "ChangeDisplaySettingsExW failed"
	}

	if dm.dmDisplayOrientation == 2 {
		return "screen rotated 180 degrees"
	}
	return "screen reset to normal"
}

// ── CD-ROM Tray ─────────────────────────────────────────────────────────

func cdRomTrayWindows(action string) string {
	proc := winmm.NewProc("mciSendStringW")
	var cmd string
	if action == "open" {
		cmd = "set cdaudio door open"
	} else {
		cmd = "set cdaudio door closed"
	}
	cmdPtr, _ := syscall.UTF16PtrFromString(cmd)
	ret, _, _ := proc.Call(uintptr(unsafe.Pointer(cmdPtr)), 0, 0, 0)
	if ret != 0 {
		return fmt.Sprintf("mciSendString failed: %d", ret)
	}
	return "CD-ROM tray " + action + "ed"
}

// ── Lock Workstation ────────────────────────────────────────────────────

func lockWorkstationWindows() string {
	proc := user32.NewProc("LockWorkStation")
	proc.Call()
	return "workstation locked"
}

// ── Set Volume ──────────────────────────────────────────────────────────

func setVolumeWindows(level int) string {
	volume := uint32(level * 65535 / 100)
	volume = (volume << 16) | volume
	proc := winmm.NewProc("waveOutSetVolume")
	ret, _, _ := proc.Call(0, uintptr(volume))
	if ret != 0 {
		return fmt.Sprintf("waveOutSetVolume failed: %d", ret)
	}
	return fmt.Sprintf("volume set to %d%%", level)
}

// ── Cursor Flip ─────────────────────────────────────────────────────────

func cursorFlipWindows() string {
	proc := user32.NewProc("SystemParametersInfoW")

	type mouseParams struct {
		MouseSpeed   int32
		MouseThreshold1 int32
		MouseThreshold2 int32
	}

	var mp mouseParams
	ret, _, _ := proc.Call(0x0003, unsafe.Sizeof(mp), uintptr(unsafe.Pointer(&mp)), 0)
	if ret == 0 {
		return "failed to get mouse settings"
	}

	if mp.MouseSpeed > 0 {
		mp.MouseSpeed = 0
	} else {
		mp.MouseSpeed = 1
	}

	ret2, _, _ := proc.Call(0x0004, unsafe.Sizeof(mp), uintptr(unsafe.Pointer(&mp)), 0x0002)
	if ret2 == 0 {
		return "failed to set mouse settings"
	}

	if mp.MouseSpeed > 0 {
		return "cursor inverted"
	}
	return "cursor reset to normal"
}
