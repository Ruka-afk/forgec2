//go:build windows
// +build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows/registry"
)

const (
	spiSetDesktopWallpaper = 0x0014
	spifUpdateIniFile      = 0x01
	spifSendChange         = 0x02
)

// setDesktopWallpaper persists the style under HKCU\Control Panel\Desktop
// and activates the image immediately. The style is written both before and
// after the SPI call: the repaint reads the values, but Windows resets them
// (observed: fit reverts to fill) as part of applying a new image, so a
// single pre-write does not survive. Values alone without SPI never repaint.
func setDesktopWallpaper(imgPath, style string) error {
	wallpaperStyle, tileWallpaper, err := wallpaperStyleValues(style)
	if err != nil {
		return err
	}
	writeStyle := func() error {
		key, _, err := registry.CreateKey(registry.CURRENT_USER, `Control Panel\Desktop`, registry.SET_VALUE)
		if err != nil {
			return fmt.Errorf("wallpaper: registry: %w", err)
		}
		defer key.Close()
		if err := key.SetStringValue("WallpaperStyle", wallpaperStyle); err != nil {
			return fmt.Errorf("wallpaper: registry WallpaperStyle: %w", err)
		}
		if err := key.SetStringValue("TileWallpaper", tileWallpaper); err != nil {
			return fmt.Errorf("wallpaper: registry TileWallpaper: %w", err)
		}
		return nil
	}
	if err := writeStyle(); err != nil {
		return err
	}
	imgUTF16, err := syscall.UTF16PtrFromString(imgPath)
	if err != nil {
		return fmt.Errorf("wallpaper: encode path: %w", err)
	}
	r1, _, errno := procSystemParametersInfoW.Call(
		uintptr(spiSetDesktopWallpaper),
		0,
		uintptr(unsafe.Pointer(imgUTF16)),
		uintptr(spifUpdateIniFile|spifSendChange))
	if r1 == 0 {
		return fmt.Errorf("wallpaper: SystemParametersInfo failed: %v", errno)
	}
	return writeStyle()
}
