//go:build linux || windows || darwin

package main

import (
	"strings"
	"sync"
)

var (
	tunMu     sync.Mutex
	tunActive bool
)

func handleTunStart(task Task, res *TaskResult) {
	cidr := strings.TrimSpace(task.Command)
	if cidr == "" {
		cidr = "10.66.0.2/24"
	}
	tunMu.Lock()
	if tunActive {
		tunMu.Unlock()
		res.Error = "tun already running"
		return
	}
	tunMu.Unlock()
	msg, err := startAgentTUN(cidr)
	if err != nil {
		res.Error = err.Error()
		return
	}
	tunMu.Lock()
	tunActive = true
	tunMu.Unlock()
	res.Output = msg
}

func handleTunStop(task Task, res *TaskResult) {
	_ = task
	if err := stopAgentTUN(); err != nil {
		res.Error = err.Error()
		return
	}
	tunMu.Lock()
	tunActive = false
	tunMu.Unlock()
	res.Output = "tun stopped"
}
