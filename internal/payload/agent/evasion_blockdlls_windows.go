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
	results := ""

	// Method 1: Set ProcessSignaturePolicy
	ntdll := syscall.NewLazyDLL("ntdll.dll")
	procSetProcessMitigationPolicy := ntdll.NewProc("SetProcessMitigationPolicy")

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
	procNtQueryInformationProcess := ntdll.NewProc("NtQueryInformationProcess")

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
		uintptr(0xffffffffffffffff),
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

	// Read ProcessParameters pointer from PEB
	ppPtr := *(*uintptr)(unsafe.Pointer(pbi.PebBaseAddress + ppOffset))
	if ppPtr == 0 {
		return "ProcessParameters is nil"
	}

	// Flags field is at offset 0x70 (Win10 22H2) or 0x74 (Win11) in RTL_USER_PROCESS_PARAMETERS
	// We try to set bit 0x20 (BlockDlls) via a calculated offset
	_ = ppPtr
	return "requires Windows-version-specific offset (Win10: 0x70, Win11: 0x74)"
}
