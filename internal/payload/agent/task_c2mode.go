//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"fmt"
	"strings"
	"sync/atomic"
)

func handleSetC2Mode(task Task, res *TaskResult) {
	modeStr := strings.TrimSpace(task.Command)
	if modeStr == "" {
		res.Error = "c2 mode required: single, failover, roundrobin, random, split, parallel"
		return
	}

	switch strings.ToLower(modeStr) {
	case "single":
		c2Mode = C2ModeSingle
	case "failover":
		c2Mode = C2ModeFailover
	case "roundrobin", "round_robin":
		c2Mode = C2ModeRoundRobin
	case "random":
		c2Mode = C2ModeRandom
	case "split":
		c2Mode = C2ModeSplit
	case "parallel":
		c2Mode = C2ModeParallel
	default:
		res.Error = fmt.Sprintf("unknown c2 mode: %s (valid: single, failover, roundrobin, random, split, parallel)", modeStr)
		return
	}

	// Reset dead mode on manual mode switch
	atomic.StoreInt32(&deadMode, 0)

	res.Output = fmt.Sprintf("c2 mode set to %s", modeStr)
}
