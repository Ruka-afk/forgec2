//go:build windows
// +build windows

package main

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
)

// runPowerShellStdin executes a PowerShell script passed over STDIN instead of
// the process command line. Lateral-movement passwords embedded in a
// PSCredential would otherwise sit in powershell.exe's argv, readable by any
// local user via tasklist / Get-CimInstance Win32_Process. With -Command -
// the script lives only in this process's memory and the anonymous pipe.
func runPowerShellStdin(script string) (string, error) {
	if !strings.Contains(script, "OutputEncoding") {
		script = "[Console]::OutputEncoding = [System.Text.Encoding]::UTF8; $OutputEncoding = [System.Text.Encoding]::UTF8; " + script
	}
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "-")
	cmd.Stdin = strings.NewReader(script)
	applyHideWindow(cmd)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return decodeShellOutput(out.Bytes(), "powershell.exe"), err
}

// runCmdScriptFile writes a batch script to a temp file and runs it with
// `cmd /c <file>` so any secrets in the script never appear in a process
// command line. The temp file is zeroized and removed afterwards (memory
// hygiene: the password must not linger on disk).
func runCmdScriptFile(script string) (string, error) {
	f, err := os.CreateTemp("", "fc_lat_*.cmd")
	if err != nil {
		return "", err
	}
	path := f.Name()
	defer os.Remove(path)
	if _, err := f.WriteString(script); err != nil {
		f.Close()
		return "", err
	}
	f.Close()

	cmd := exec.Command("cmd.exe", "/C", "chcp 65001 >nul & "+path)
	applyHideWindow(cmd)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	runErr := cmd.Run()

	// Best-effort scrub of the script (which may contain a password) before the
	// deferred os.Remove reclaims the inode.
	zeroOutFile(path, int64(len(script)))

	return decodeShellOutput(out.Bytes(), "cmd.exe"), runErr
}
