//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"encoding/base64"
	"fmt"
	"runtime"
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
	sb.WriteString("3. Harvest credentials via creds/mimikatz if permitted.\n")
	return sb.String()
}

func runLinuxPrivescChecks() string {
	var sb strings.Builder
	sections := []struct {
		title string
		cmd   string
	}{
		{"Identity", "id"},
		{"Sudo Permissions", "sudo -n -l 2>/dev/null || sudo -l 2>&1"},
		{"SUID Binaries", "find / -perm -4000 -type f 2>/dev/null | head -50"},
		{"Writable /etc/passwd", "ls -la /etc/passwd /etc/shadow 2>/dev/null"},
		{"Cron Jobs", "ls -la /etc/cron* 2>/dev/null; crontab -l 2>/dev/null"},
		{"Capabilities", "getcap -r / 2>/dev/null | head -30"},
		{"Docker Socket", "ls -la /var/run/docker.sock 2>/dev/null"},
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

	sb.WriteString("--- Suggestions ---\n")
	sb.WriteString("1. Check sudo -l for NOPASSWD commands.\n")
	sb.WriteString("2. Audit SUID binaries (find / -perm -4000).\n")
	sb.WriteString("3. Review writable paths in cron/systemd services.\n")
	return sb.String()
}

func runDarwinPrivescChecks() string {
	var sb strings.Builder
	sections := []struct {
		title string
		cmd   string
	}{
		{"Identity", "id"},
		{"Sudo Permissions", "sudo -n -l 2>/dev/null || sudo -l 2>&1"},
		{"TCC / Privacy DB", "ls -la ~/Library/Application\\ Support/com.apple.TCC 2>/dev/null"},
		{"Launch Agents", "ls -la ~/Library/LaunchAgents /Library/LaunchAgents /Library/LaunchDaemons 2>/dev/null"},
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
	return sb.String()
}