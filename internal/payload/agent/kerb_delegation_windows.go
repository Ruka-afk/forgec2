//go:build windows
// +build windows

package main

import (
	"fmt"
	"strings"
)

func findUnconstrainedDelegation() ([]string, error) {
	psCmd := `
$searcher = New-Object DirectoryServices.DirectorySearcher([ADSI]"LDAP://$( 
	([ADSI]"LDAP://RootDSE").dnsHostName
)/CN=Computers,$(([ADSI]"LDAP://RootDSE").defaultNamingContext)")
$searcher.PageSize = 1000
$searcher.Filter = "(&(objectClass=computer)(userAccountControl:1.2.840.113556.1.4.803:=524288))"
$searcher.PropertiesToLoad.AddRange(@("dnshostname","name","operatingSystem"))
$results = $searcher.FindAll() | ForEach-Object {
	$props = $_.Properties
	[PSCustomObject]@{
		Hostname = $props.dnshostname -join ''
		Name     = $props.name -join ''
		OS       = $props.operatingSystem -join ''
	}
}
$results | ConvertTo-Json -Compress
`
	out, err := runShell(psCmd, "powershell.exe")
	if err != nil {
		return nil, fmt.Errorf("unconstrained delegation LDAP query failed: %w\nOutput: %s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	var hosts []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, `"Hostname"`) {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				host := strings.Trim(strings.TrimSpace(parts[1]), `", `)
				if host != "" {
					hosts = append(hosts, host)
				}
			}
		}
	}
	if len(hosts) == 0 {
		return hosts, nil
	}
	return hosts, nil
}

func monitorIncomingTGTs() error {
	psCmd := `
try {
	$klist = klist -li 0x3e7 2>&1
	if ($LASTEXITCODE -ne 0) {
		Write-Output "No cached tickets found"
		return
	}
	Write-Output "Current LSA cache:"
	Write-Output $klist
} catch {
	Write-Output "TGT monitor: $_"
}
`
	out, err := runShell(psCmd, "powershell.exe")
	if err != nil {
		return fmt.Errorf("TGT monitor failed: %w\nOutput: %s", err, out)
	}
	_ = out
	return nil
}
