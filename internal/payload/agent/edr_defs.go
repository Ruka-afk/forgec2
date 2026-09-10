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
	EDRKaspersky   = "kaspersky"
	EDRESET        = "eset"
	EDRMcAfee      = "mcafee"
	EDRSophos      = "sophos"
	EDRCortex      = "cortex"
	EDRElastic     = "elastic"
	EDRSysmon      = "sysmon"
	EDR360         = "360"
	EDRHuorong     = "huorong"
	EDRTencent     = "tencent"
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
