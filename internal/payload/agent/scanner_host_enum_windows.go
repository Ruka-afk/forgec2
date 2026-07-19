//go:build windows
// +build windows

package main

import (
	"fmt"
	"os/exec"
	"strings"
)

func netEnumHosts(domain string) (string, error) {
	ps := `Get-ADDomainController -Discover -Domain ` + domain + ` | Select-Object -ExpandProperty Name`
	if domain == "" {
		ps = `(nltest /dclist:) | Select-Object -Skip 2`
	}
	c := exec.Command("powershell", "-NoP", "-NonI", "-Command", ps)
	applyHideWindow(c)
	out, err := c.CombinedOutput()
	result := strings.TrimSpace(string(out))
	if result == "" {
		result = "(no hosts found)"
	}
	return fmt.Sprintf("Host enumeration (domain=%s):\n%s\n", domain, result), err
}

func netScanSMBDiscovery() (string, error) {
	cmd := exec.Command("cmd", "/c", "net view")
	applyHideWindow(cmd)
	out, err := cmd.CombinedOutput()
	return fmt.Sprintf("Net view:\n%s", string(out)), err
}
