//go:build windows && !arm64
// +build windows,!arm64

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

func syscallNtCreateThreadExParam(mgr *syscallManager, hProc uintptr, startAddr uintptr, param uintptr) (uintptr, error) {
	stub, err := mgr.getSpoofedStub("NtCreateThreadEx")
	if err != nil {
		stub, err = mgr.getStub("NtCreateThreadEx")
		if err != nil {
			return 0, err
		}
	}
	var hThread uintptr
	r1, _, _ := syscall.Syscall9(stub, 8,
		uintptr(unsafe.Pointer(&hThread)),
		0x1FFFFF,
		0,
		hProc,
		startAddr,
		param,
		0,
		0,
		0,
	)
	if r1 != 0 {
		return 0, fmt.Errorf("NtCreateThreadEx failed: 0x%X", r1)
	}
	return hThread, nil
}
