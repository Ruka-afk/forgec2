//go:build linux || windows || darwin

package main

const (
	EDRCrowdStrike = "crowdstrike"
	EDRDefender    = "defender"
	EDRCarbonBlack = "carbonblack"
	EDRSentinelOne = "sentinelone"
	EDRCylance     = "cylance"
	EDRBitDefender = "bitdefender"
	EDRSymantec    = "symantec"
	EDRTrendMicro  = "trendmicro"
	EDRUnknown     = "unknown"
)

type EDRInfo struct {
	Detected  bool
	Name      string
	Processes []string
	Services  []string
	Drivers   []string
}

type EvasionStrategy struct {
	Name            string
	IndirectSyscall bool
	StackSpoofing   bool
	PEBBlockDLLs    bool
	AMSIPatch       bool
	ETWPatch        bool
	VEHUnhook       bool
	SleepMask       bool
	HeapEncryption  bool
	DelayLoad       bool
}
