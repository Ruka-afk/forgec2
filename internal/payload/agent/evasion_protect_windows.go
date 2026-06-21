//go:build windows
// +build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

// protectProcess enables PsProtectedProcess via NtSetInformationProcess.
func protectProcess() string {
	ntdll := syscall.NewLazyDLL("ntdll.dll")
	ntSetInfo := ntdll.NewProc("NtSetInformationProcess")

	// ProcessProtection (class 0x44 / 68)
	// Structure: { ProtectionLevel (BYTE), Flags (BYTE) }
	// ProtectionLevel: 0x01 = PsProtected, 0x03 = PsProtectedLight
	// Flags: 0x01 = Signer (WinTcb), 0x02 = Signer (Windows)
	type processProtection struct {
		Level byte
		Flags byte
	}
	prot := processProtection{Level: 0x01, Flags: 0x01}

	const ProcessProtection = 68
	const PROCESS_ALL_ACCESS = 0x1FFFFF
	const NtCurrentProcess = ^uintptr(0xffffffffffffffff)

	ret, _, _ := ntSetInfo.Call(
		NtCurrentProcess,
		ProcessProtection,
		uintptr(unsafe.Pointer(&prot)),
		uintptr(unsafe.Sizeof(prot)),
	)
	if ret != 0 {
		return fmt.Sprintf("protect_process: NtSetInformationProcess failed: 0x%x", ret)
	}
	return "protect_process: PsProtectedProcess enabled"
}
