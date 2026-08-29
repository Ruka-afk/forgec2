//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"strings"
)

func sccmRecon() (string, error) {
	var sb strings.Builder
	sb.WriteString("=== SCCM / MECM recon ===\n")
	sb.WriteString("(Authorized lab use. This is discovery only — no client push.)\n")

	reg := exec.Command("reg", "query", `HKLM\SOFTWARE\Microsoft\CCM\Security`, "/s")
	applyHideWindow(reg)
	if out, err := reg.CombinedOutput(); err == nil {
		sb.WriteString("\n--- HKLM\\SOFTWARE\\Microsoft\\CCM\\Security ---\n")
		sb.Write(out)
	}

	for _, key := range []string{
		`HKLM\SOFTWARE\Microsoft\CCM`,
		`HKLM\SOFTWARE\Microsoft\SMS`,
		`HKLM\SOFTWARE\Microsoft\CCMSetup`,
	} {
		cmd := exec.Command("reg", "query", key)
		applyHideWindow(cmd)
		out, err := cmd.CombinedOutput()
		if err != nil {
			sb.WriteString(fmt.Sprintf("\n--- %s ---\n(not present)\n", key))
			continue
		}
		sb.WriteString("\n--- " + key + " ---\n")
		sb.Write(out)
	}

	wmi := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command",
		`Get-CimInstance -Namespace root\ccm -ClassName SMS_Client -ErrorAction SilentlyContinue | Format-List *; Get-CimInstance -Namespace root\ccm -ClassName SMS_Authority -ErrorAction SilentlyContinue | Format-List *`)
	applyHideWindow(wmi)
	if out, err := wmi.CombinedOutput(); err == nil && len(strings.TrimSpace(string(out))) > 0 {
		sb.WriteString("\n--- WMI root\\ccm ---\n")
		sb.Write(out)
	} else {
		sb.WriteString("\n--- WMI root\\ccm ---\n(client WMI namespace not available)\n")
	}
	return sb.String(), nil
}

func entraPRTRecon() (string, error) {
	var sb strings.Builder
	sb.WriteString("=== Entra ID / PRT recon ===\n")
	sb.WriteString("dsregcmd status (PRT dump requires SYSTEM + CloudAP DPAPI; this is recon only).\n")
	cmd := exec.Command("dsregcmd.exe", "/status")
	applyHideWindow(cmd)
	out, err := cmd.CombinedOutput()
	if len(out) > 0 {
		sb.Write(out)
	}
	if err != nil && len(out) == 0 {
		return "", fmt.Errorf("dsregcmd: %w", err)
	}
	reg := exec.Command("reg", "query", `HKLM\SYSTEM\CurrentControlSet\Control\CloudDomainJoin\JoinInfo`)
	applyHideWindow(reg)
	if o, e := reg.CombinedOutput(); e == nil {
		sb.WriteString("\n--- CloudDomainJoin ---\n")
		sb.Write(o)
	}
	return sb.String(), nil
}
