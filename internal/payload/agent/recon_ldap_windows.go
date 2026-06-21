//go:build windows
// +build windows

package main

import (
	"fmt"
	"os/exec"
	"strings"
)

func ldapQuery(filter string) (string, error) {
	if filter == "" {
		filter = "(objectClass=*)"
	}
	ps := fmt.Sprintf(`
$TAB = [char]9
try {
	$root = [ADSI]"LDAP://RootDSE"
	$domainDN = $root.defaultNamingContext
	$searcher = New-Object DirectoryServices.DirectorySearcher([ADSI]"LDAP://$domainDN")
	$searcher.PageSize = 1000
	$searcher.Filter = "%s"
	$results = $searcher.FindAll()
	$out = @()
	$out += "LDAP query: %s"
	$out += "Found $($results.Count) objects:"
	$out += "---"
	foreach ($r in $results) {
		$dn = $r.Properties["distinguishedName"]
		$cls = $r.Properties["objectClass"][-1]
		$cn = $r.Properties["cn"]
		$out += "[$cls] $cn$TAB$dn"
	}
	Write-Output ($out -join [Environment]::NewLine)
} catch {
	Write-Output "LDAP query failed: $_"
}
`, filter, filter)
	c := exec.Command("powershell", "-NoP", "-NonI", "-Command", ps)
	applyHideWindow(c)
	out, err := c.CombinedOutput()
	result := strings.TrimSpace(string(out))
	if result == "" {
		result = "No results."
	}
	return result, err
}

func ldapUsers() (string, error) {
	ps := `
$TAB = [char]9
$root = [ADSI]"LDAP://RootDSE"
$domainDN = $root.defaultNamingContext
$searcher = New-Object DirectoryServices.DirectorySearcher([ADSI]"LDAP://$domainDN")
$searcher.PageSize = 1000
$searcher.Filter = "(&(objectClass=user)(objectCategory=person))"
$searcher.PropertiesToLoad.AddRange(@("cn","samaccountname","userprincipalname","mail","lockouttime","whencreated","pwdlastset","badpwdcount","admincount","distinguishedname"))
$results = $searcher.FindAll()
$out = @()
$out += "=== Domain Users ($($results.Count)) ==="
$out += "CN$TAB" + "SAM$TAB" + "UPN$TAB" + "Mail$TAB" + "Admin$TAB" + "Locked$TAB" + "Created"
$out += "---"
foreach ($r in $results) {
	$cn = $r.Properties["cn"] -join ","
	$sam = $r.Properties["samaccountname"] -join ","
	$upn = $r.Properties["userprincipalname"] -join ","
	$mail = $r.Properties["mail"] -join ","
	$admin = if ($r.Properties["admincount"]) { "YES" } else { "" }
	$locked = if ($r.Properties["lockouttime"] -and $r.Properties["lockouttime"][0] -gt 0) { "LOCKED" } else { "" }
	$created = $r.Properties["whencreated"] -join ","
	$out += "$cn$TAB$sam$TAB$upn$TAB$mail$TAB$admin$TAB$locked$TAB$created"
}
Write-Output ($out -join [Environment]::NewLine)
`
	c := exec.Command("powershell", "-NoP", "-NonI", "-Command", ps)
	applyHideWindow(c)
	out, err := c.CombinedOutput()
	result := strings.TrimSpace(string(out))
	if result == "" {
		result = "No users found or not domain-joined."
	}
	return result, err
}

func ldapGroups() (string, error) {
	ps := `
$TAB = [char]9
$root = [ADSI]"LDAP://RootDSE"
$domainDN = $root.defaultNamingContext
$searcher = New-Object DirectoryServices.DirectorySearcher([ADSI]"LDAP://$domainDN")
$searcher.PageSize = 1000
$searcher.Filter = "(objectClass=group)"
$searcher.PropertiesToLoad.AddRange(@("cn","samaccountname","member","distinguishedname","admincount","grouptype"))
$results = $searcher.FindAll()
$out = @()
$out += "=== Domain Groups ($($results.Count)) ==="
$out += "CN$TAB" + "SAM$TAB" + "Type$TAB" + "Admin$TAB" + "Members"
$out += "---"
foreach ($r in $results) {
	$cn = $r.Properties["cn"] -join ","
	$sam = $r.Properties["samaccountname"] -join ","
	$type = switch ($r.Properties["grouptype"][0]) { 2 {"Global"}; 4 {"Local"}; 8 {"Universal"}; default {"Other"} }
	$admin = if ($r.Properties["admincount"]) { "YES" } else { "" }
	$members = @($r.Properties["member"]).Count
	$out += "$cn$TAB$sam$TAB$type$TAB$admin$TAB$members"
}
Write-Output ($out -join [Environment]::NewLine)
`
	c := exec.Command("powershell", "-NoP", "-NonI", "-Command", ps)
	applyHideWindow(c)
	out, err := c.CombinedOutput()
	result := strings.TrimSpace(string(out))
	if result == "" {
		result = "No groups found."
	}
	return result, err
}

func ldapComputers() (string, error) {
	ps := `
$TAB = [char]9
$root = [ADSI]"LDAP://RootDSE"
$domainDN = $root.defaultNamingContext
$searcher = New-Object DirectoryServices.DirectorySearcher([ADSI]"LDAP://$domainDN")
$searcher.PageSize = 1000
$searcher.Filter = "(objectClass=computer)"
$searcher.PropertiesToLoad.AddRange(@("cn","operatingsystem","dnshostname","description","whencreated","lastlogontimestamp","distinguishedname"))
$results = $searcher.FindAll()
$out = @()
$out += "=== Domain Computers ($($results.Count)) ==="
$out += "CN$TAB" + "DNS$TAB" + "OS$TAB" + "Description$TAB" + "Created$TAB" + "LastLogon"
$out += "---"
foreach ($r in $results) {
	$cn = $r.Properties["cn"] -join ","
	$dns = $r.Properties["dnshostname"] -join ","
	$os = $r.Properties["operatingsystem"] -join ","
	$desc = $r.Properties["description"] -join ","
	$created = $r.Properties["whencreated"] -join ","
	$last = $r.Properties["lastlogontimestamp"] -join ","
	$out += "$cn$TAB$dns$TAB$os$TAB$desc$TAB$created$TAB$last"
}
Write-Output ($out -join [Environment]::NewLine)
`
	c := exec.Command("powershell", "-NoP", "-NonI", "-Command", ps)
	applyHideWindow(c)
	out, err := c.CombinedOutput()
	result := strings.TrimSpace(string(out))
	if result == "" {
		result = "No computers found."
	}
	return result, err
}

func ldapSPN() (string, error) {
	ps := `
$TAB = [char]9
$root = [ADSI]"LDAP://RootDSE"
$domainDN = $root.defaultNamingContext
$searcher = New-Object DirectoryServices.DirectorySearcher([ADSI]"LDAP://$domainDN")
$searcher.PageSize = 1000
$searcher.Filter = "(&(objectClass=user)(objectCategory=person)(servicePrincipalName=*))"
$searcher.PropertiesToLoad.AddRange(@("cn","samaccountname","serviceprincipalname","distinguishedname"))
$results = $searcher.FindAll()
$out = @()
$out += "=== Kerberoast Target SPNs ($($results.Count) users) ==="
$out += "SAM$TAB" + "SPN$TAB" + "DN"
$out += "---"
foreach ($r in $results) {
	$sam = $r.Properties["samaccountname"] -join ","
	$spns = $r.Properties["serviceprincipalname"] -join ";"
	$dn = $r.Properties["distinguishedname"] -join ","
	$out += "$sam$TAB$spns$TAB$dn"
}
Write-Output ($out -join [Environment]::NewLine)
`
	c := exec.Command("powershell", "-NoP", "-NonI", "-Command", ps)
	applyHideWindow(c)
	out, err := c.CombinedOutput()
	result := strings.TrimSpace(string(out))
	if result == "" {
		result = "No SPN-enabled users found."
	}
	return result, err
}

func ldapACL() (string, error) {
	ps := `
$TAB = [char]9
$root = [ADSI]"LDAP://RootDSE"
$domainDN = $root.defaultNamingContext
$searcher = New-Object DirectoryServices.DirectorySearcher([ADSI]"LDAP://$domainDN")
$searcher.PageSize = 1000
$searcher.Filter = "(objectClass=*)"
$searcher.PropertiesToLoad.AddRange(@("distinguishedname","ntsecuritydescriptor","objectclass"))
$results = $searcher.FindAll()
$out = @()
$out += "=== High-Risk ACLs ==="
$out += "Target$TAB" + "Right$TAB" + "Principal"
$out += "---"
$dangerous = @{
	"GenericAll" = 0x10000000 -bor 0x20000000 -bor 0x40000000 -bor 0x80000000
	"GenericWrite" = 0x40000000 -bor 0x20000000
	"WriteOwner" = 0x80000000
	"WriteDacl" = 0x40000
	"WriteMember" = 0x10
}

foreach ($r in $results) {
	$dn = $r.Properties["distinguishedname"]
	$sd = $r.Properties["ntsecuritydescriptor"][0]
	if (-not $sd) { continue }
	try {
		$acl = New-Object System.DirectoryServices.ActiveDirectoryAccessRule
		$acl = New-Object System.Security.AccessControl.RawSecurityDescriptor($sd,0)
		foreach ($ace in $acl.DiscretionaryAcl) {
			$rights = $ace.AccessMask
			$found = @()
			foreach ($name in $dangerous.Keys) {
				if (($rights -band $dangerous[$name]) -eq $dangerous[$name]) {
					$found += $name
				}
			}
			if ($found.Count -gt 0) {
				$sid = $ace.SecurityIdentifier
				try {
					$principal = $sid.Translate([System.Security.Principal.NTAccount]).Value
				} catch { $principal = $sid.Value }
				$out += "$dn$TAB$($found -join ',')$TAB$principal"
			}
		}
	} catch { }
}
if ($out.Count -eq 3) { $out += "(no high-risk ACLs found)" }
Write-Output ($out -join [Environment]::NewLine)
`
	c := exec.Command("powershell", "-NoP", "-NonI", "-Command", ps)
	applyHideWindow(c)
	out, err := c.CombinedOutput()
	result := strings.TrimSpace(string(out))
	if result == "" {
		result = "ACL enumeration failed."
	}
	return result, err
}
