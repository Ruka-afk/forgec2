package protocol

// KillChainPhases is the ordered list of MITRE ATT&CK tactics.
var KillChainPhases = []string{
	"Reconnaissance",
	"Resource Development",
	"Initial Access",
	"Execution",
	"Persistence",
	"Privilege Escalation",
	"Defense Evasion",
	"Credential Access",
	"Discovery",
	"Lateral Movement",
	"Collection",
	"Command and Control",
	"Exfiltration",
	"Impact",
}

// KillChainStep is a single step in a kill chain execution plan.
type KillChainStep struct {
	Phase    string            `json:"phase"`
	TaskType string            `json:"task_type"`
	Params   map[string]string `json:"params"`
	WaitTime int               `json:"wait_time"`
}

// KillChainTemplate is a pre-built attack path template.
type KillChainTemplate struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Steps       []KillChainStep `json:"steps"`
}

// AttackTechnique maps a task type to MITRE ATT&CK technique.
type AttackTechnique struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Tactic    string   `json:"tactic"`
	TaskTypes []string `json:"task_types"`
}

// AttackTechniques is the master list of all covered techniques.
// KillChainTemplates are pre-built attack path templates.
var KillChainTemplates = []KillChainTemplate{
	{
		Name:        "Full Domain Compromise",
		Description: "Initial Access → Credential Theft → Kerberos Attacks → DCSync for full domain takeover",
		Steps: []KillChainStep{
			{Phase: "Initial Access", TaskType: "shell", Params: map[string]string{"command": "whoami"}, WaitTime: 2},
			{Phase: "Execution", TaskType: "shell", Params: map[string]string{"command": "powershell -enc ZQBjAGgAbwAgAEgAZQBsAGwAbwA="}, WaitTime: 2},
			{Phase: "Defense Evasion", TaskType: "amsi_bypass", Params: map[string]string{}, WaitTime: 3},
			{Phase: "Credential Access", TaskType: "mimikatz", Params: map[string]string{"command": "sekurlsa::logonpasswords"}, WaitTime: 10},
			{Phase: "Discovery", TaskType: "ldap_query", Params: map[string]string{"query": "(objectClass=user)"}, WaitTime: 5},
			{Phase: "Credential Access", TaskType: "dumptickets", Params: map[string]string{}, WaitTime: 5},
			{Phase: "Credential Access", TaskType: "dcsync", Params: map[string]string{}, WaitTime: 15},
			{Phase: "Exfiltration", TaskType: "download", Params: map[string]string{"path": "C:\\Windows\\Temp\\dump.zip"}, WaitTime: 5},
		},
	},
	{
		Name:        "Silent Credential Theft",
		Description: "Stealthy credential harvesting — Execution → Defense Evasion → Credential Access → Exfil",
		Steps: []KillChainStep{
			{Phase: "Defense Evasion", TaskType: "amsi_bypass", Params: map[string]string{}, WaitTime: 2},
			{Phase: "Defense Evasion", TaskType: "etw_bypass", Params: map[string]string{}, WaitTime: 2},
			{Phase: "Credential Access", TaskType: "browser_steal", Params: map[string]string{}, WaitTime: 5},
			{Phase: "Collection", TaskType: "keylogger_start", Params: map[string]string{}, WaitTime: 10},
			{Phase: "Credential Access", TaskType: "mimikatz", Params: map[string]string{"command": "sekurlsa::logonpasswords"}, WaitTime: 10},
			{Phase: "Exfiltration", TaskType: "download", Params: map[string]string{"path": "C:\\Users\\Public\\output.txt"}, WaitTime: 3},
		},
	},
	{
		Name:        "Lateral Movement Campaign",
		Description: "Initial Access → Discovery → Lateral Movement → Collection across adjacent hosts",
		Steps: []KillChainStep{
			{Phase: "Initial Access", TaskType: "shell", Params: map[string]string{"command": "whoami"}, WaitTime: 2},
			{Phase: "Discovery", TaskType: "netstat", Params: map[string]string{}, WaitTime: 3},
			{Phase: "Discovery", TaskType: "portscan", Params: map[string]string{"ports": "445,3389,5985"}, WaitTime: 15},
			{Phase: "Lateral Movement", TaskType: "lateral_wmi", Params: map[string]string{}, WaitTime: 10},
			{Phase: "Collection", TaskType: "screenshot", Params: map[string]string{}, WaitTime: 5},
			{Phase: "Exfiltration", TaskType: "download", Params: map[string]string{"path": "C:\\Users\\Public\\screenshot.png"}, WaitTime: 3},
		},
	},
	{
		Name:        "Persistence & Privilege Escalation",
		Description: "Escalate privileges and establish persistent backdoor access",
		Steps: []KillChainStep{
			{Phase: "Execution", TaskType: "shell", Params: map[string]string{"command": "whoami /all"}, WaitTime: 2},
			{Phase: "Privilege Escalation", TaskType: "privesc_check", Params: map[string]string{}, WaitTime: 10},
			{Phase: "Privilege Escalation", TaskType: "uac_bypass", Params: map[string]string{}, WaitTime: 5},
			{Phase: "Defense Evasion", TaskType: "inject", Params: map[string]string{}, WaitTime: 5},
			{Phase: "Persistence", TaskType: "persistence_add", Params: map[string]string{"type": "scheduled_task"}, WaitTime: 3},
			{Phase: "Persistence", TaskType: "persistence_add", Params: map[string]string{"type": "registry_run"}, WaitTime: 3},
		},
	},
	{
		Name:        "Full Reconnaissance",
		Description: "Comprehensive network and system recon — user, network, domain, and service discovery",
		Steps: []KillChainStep{
			{Phase: "Execution", TaskType: "shell", Params: map[string]string{"command": "whoami"}, WaitTime: 1},
			{Phase: "Discovery", TaskType: "hostname", Params: map[string]string{}, WaitTime: 1},
			{Phase: "Discovery", TaskType: "whoami", Params: map[string]string{}, WaitTime: 1},
			{Phase: "Discovery", TaskType: "netstat", Params: map[string]string{}, WaitTime: 3},
			{Phase: "Discovery", TaskType: "portscan", Params: map[string]string{"ports": "1-1024"}, WaitTime: 30},
			{Phase: "Discovery", TaskType: "ldap_query", Params: map[string]string{"query": "(objectClass=computer)"}, WaitTime: 10},
			{Phase: "Discovery", TaskType: "services", Params: map[string]string{}, WaitTime: 5},
			{Phase: "Discovery", TaskType: "av", Params: map[string]string{}, WaitTime: 3},
		},
	},
}

var AttackTechniques = []AttackTechnique{
	{ID: "T1059.001", Name: "PowerShell", Tactic: "Execution", TaskTypes: []string{"exec", "powershell", "run", "powerpick", "clr_powershell"}},
	{ID: "T1059.003", Name: "Windows Command Shell", Tactic: "Execution", TaskTypes: []string{"cmd", "shell", "interactive_shell_start"}},
	{ID: "T1055", Name: "Process Injection", Tactic: "Defense Evasion", TaskTypes: []string{"inject", "shinject", "reflectdll_inject"}},
	{ID: "T1003", Name: "OS Credential Dumping", Tactic: "Credential Access", TaskTypes: []string{"dumpsam", "dumplsass", "mimikatz", "creds"}},
	{ID: "T1003.001", Name: "LSASS Memory", Tactic: "Credential Access", TaskTypes: []string{"dumplsass", "mimikatz"}},
	{ID: "T1003.003", Name: "NTDS", Tactic: "Credential Access", TaskTypes: []string{"dumppdc", "ntds"}},
	{ID: "T1003.005", Name: "Kerberos Ticket", Tactic: "Credential Access", TaskTypes: []string{"dumptickets", "golden_ticket", "silver_ticket"}},
	{ID: "T1003.006", Name: "DCSync", Tactic: "Credential Access", TaskTypes: []string{"dcsync"}},
	{ID: "T1555", Name: "Credentials from Password Stores", Tactic: "Credential Access", TaskTypes: []string{"browser_steal", "vpn_creds", "wifi_creds"}},
	{ID: "T1555.003", Name: "Web Browser Credentials", Tactic: "Credential Access", TaskTypes: []string{"browser_steal", "chrome_cookies", "cookie_export"}},
	{ID: "T1056.001", Name: "Keylogging", Tactic: "Collection", TaskTypes: []string{"keylogger_start", "keylogger_dump"}},
	{ID: "T1113", Name: "Screen Capture", Tactic: "Collection", TaskTypes: []string{"screenshot", "screenshot_window", "screen_stream_start"}},
	{ID: "T1115", Name: "Clipboard Data", Tactic: "Collection", TaskTypes: []string{"clipboard_get", "chrome_clipboard"}},
	{ID: "T1036", Name: "Masquerading", Tactic: "Defense Evasion", TaskTypes: []string{"blockdlls"}},
	{ID: "T1027", Name: "Obfuscated Files or Info", Tactic: "Defense Evasion", TaskTypes: []string{"unhook_ntdll", "protect_process"}},
	{ID: "T1562.001", Name: "Disable or Modify Tools", Tactic: "Defense Evasion", TaskTypes: []string{"kill_av", "killproc", "kill"}},
	{ID: "T1562.004", Name: "Disable or Modify AMSI", Tactic: "Defense Evasion", TaskTypes: []string{"amsi_bypass", "amsi_session_bypass"}},
	{ID: "T1562.006", Name: "Disable or Modify ETW", Tactic: "Defense Evasion", TaskTypes: []string{"etw_bypass", "etw_ntrace_bypass"}},
	{ID: "T1548.002", Name: "Bypass UAC", Tactic: "Privilege Escalation", TaskTypes: []string{"uac_bypass", "fodhelper", "slui", "eventvwr", "computerdefaults"}},
	{ID: "T1053.005", Name: "Scheduled Task", Tactic: "Persistence", TaskTypes: []string{"persistence_add"}},
	{ID: "T1543.003", Name: "Windows Service", Tactic: "Persistence", TaskTypes: []string{"persistence_add", "service_create"}},
	{ID: "T1547.001", Name: "Registry Run Keys", Tactic: "Persistence", TaskTypes: []string{"persistence_add"}},
	{ID: "T1098", Name: "Account Manipulation", Tactic: "Persistence", TaskTypes: []string{"token_make", "token_steal"}},
	{ID: "T1574.002", Name: "DLL Side-Loading", Tactic: "Persistence", TaskTypes: []string{"blockdlls"}},
	{ID: "T1046", Name: "Network Service Scanning", Tactic: "Discovery", TaskTypes: []string{"portscan", "net_scan_smb"}},
	{ID: "T1033", Name: "System Owner/User Discovery", Tactic: "Discovery", TaskTypes: []string{"whoami", "token_whoami", "users"}},
	{ID: "T1082", Name: "System Information Discovery", Tactic: "Discovery", TaskTypes: []string{"info", "sysinfo", "drives", "av", "services"}},
	{ID: "T1083", Name: "File and Directory Discovery", Tactic: "Discovery", TaskTypes: []string{"ls", "dir", "find"}},
	{ID: "T1016", Name: "System Network Configuration Discovery", Tactic: "Discovery", TaskTypes: []string{"ipconfig", "ifconfig", "netstat"}},
	{ID: "T1049", Name: "System Network Connections Discovery", Tactic: "Discovery", TaskTypes: []string{"netstat"}},
	{ID: "T1012", Name: "Query Registry", Tactic: "Discovery", TaskTypes: []string{"reg_get", "reg_query"}},
	{ID: "T1105", Name: "Ingress Tool Transfer", Tactic: "Command and Control", TaskTypes: []string{"download", "upload", "download_url"}},
	{ID: "T1071.001", Name: "Web Protocols", Tactic: "Command and Control", TaskTypes: []string{"beacon", "beacon_now", "set_sleep"}},
	{ID: "T1572", Name: "Protocol Tunneling", Tactic: "Command and Control", TaskTypes: []string{"socks", "rportfwd_start", "tunnel_add_route"}},
	{ID: "T1021.002", Name: "SMB/Windows Admin Shares", Tactic: "Lateral Movement", TaskTypes: []string{"lateral_wmi", "lateral_psexec", "lateral_dcom", "lateral_smb"}},
	{ID: "T1021.006", Name: "Windows Remote Management", Tactic: "Lateral Movement", TaskTypes: []string{"lateral_winrm"}},
	{ID: "T1550.002", Name: "Pass the Hash", Tactic: "Lateral Movement", TaskTypes: []string{"pass_the_hash"}},
	{ID: "T1550.003", Name: "Pass the Ticket", Tactic: "Lateral Movement", TaskTypes: []string{"pass_the_ticket"}},
	{ID: "T1212", Name: "Exploitation for Credential Access", Tactic: "Credential Access", TaskTypes: []string{"coerce_printerbug", "coerce_petitpotam", "coerce_dfs"}},
	{ID: "T1611", Name: "Escape to Host", Tactic: "Privilege Escalation", TaskTypes: []string{"container_escape", "container_docker"}},
	{ID: "T1552.001", Name: "Credentials in Files", Tactic: "Credential Access", TaskTypes: []string{"cloud_steal"}},
	{ID: "T1059.006", Name: "Python", Tactic: "Execution", TaskTypes: []string{"execute_assembly", "execute_assembly_forkrun", "clr_exec_assembly"}},
	{ID: "T1204", Name: "User Execution", Tactic: "Execution", TaskTypes: []string{"macro_execute"}},
	{ID: "T1557", Name: "Adversary-in-the-Middle", Tactic: "Collection", TaskTypes: []string{"relay_ntlm_start"}},
	{ID: "T1090", Name: "Proxy", Tactic: "Command and Control", TaskTypes: []string{"socks", "rportfwd_start"}},
	{ID: "T1087", Name: "Account Discovery", Tactic: "Discovery", TaskTypes: []string{"ldap_users", "ldap_groups", "ldap_computers"}},
	{ID: "T1482", Name: "Domain Trust Discovery", Tactic: "Discovery", TaskTypes: []string{"ldap_spn", "ldap_acl", "ldap_query"}},
	{ID: "T1069", Name: "Permission Groups Discovery", Tactic: "Discovery", TaskTypes: []string{"ldap_groups", "ldap_acl"}},
	{ID: "T1525", Name: "Implant Container Image", Tactic: "Persistence", TaskTypes: []string{"container_detect", "container_docker", "container_k8s"}},
	{ID: "T1003.002", Name: "Security Account Manager", Tactic: "Credential Access", TaskTypes: []string{"dumpsam", "mimikatz"}},
	{ID: "T1528", Name: "Steal Application Access Token", Tactic: "Credential Access", TaskTypes: []string{"token_steal"}},
	{ID: "T1134", Name: "Access Token Manipulation", Tactic: "Defense Evasion", TaskTypes: []string{"token_make", "token_steal", "token_revert", "rev2self"}},
	{ID: "T1218.011", Name: "Rundll32", Tactic: "Defense Evasion", TaskTypes: []string{"peloader", "bof"}},
}
