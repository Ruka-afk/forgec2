package payload

import (
	"fmt"
	"sort"
	"strings"
)

// Agent modules: the build-time-selectable slices of the agent command
// surface.
//
// Every optional command lives in its own registry file under
// internal/payload/agent (task_registry_<module>.go) plus, where the
// implementation files are fully self-contained, a list of files that can be
// dropped along with it. The generator extracts the agent sources, drops the
// registry file (and any exclusively-owned files) of every excluded module,
// and injects modules_excluded.go — a generated map that lets the running
// agent answer an excluded command with an honest "module not compiled in"
// error instead of "unknown task type". Handlers that stay behind in mixed
// files are unreferenced once their registry entry is gone and the Go linker
// dead-codes them.
//
// "core" is not listed here: it is the always-compiled remainder (shell,
// file ops, config, timing, transport plumbing, session control — see
// task_registry.go / task_registry_v2.go).
type AgentModule struct {
	// Name is the stable wire name used by the generate form and the API.
	Name string
	// Title is the human-facing label (UI, docs).
	Title string
	// Description is a one-line operator-facing summary.
	Description string
	// Tasks lists every task type (canonical names and aliases) whose entry
	// point disappears when the module is excluded. It is pinned against the
	// registry file by TestAgentModuleCatalogMatchesRegistry.
	Tasks []string
	// RegistryFiles are the agent sources (normally one task_registry_*.go)
	// dropped when the module is excluded.
	RegistryFiles []string
	// ExclusiveFiles are further agent sources owned by the module whose
	// symbols nothing outside the module (and no always-compiled file)
	// references. Dropping them is safe only under that condition;
	// TestAgentModuleExclusionBuilds enforces it with a real build for every
	// module-off combination, so this list can only shrink toward truth.
	ExclusiveFiles []string
	// DependsOn names modules that must also be included (validated by
	// NormalizeAgentModules). Empty for every module today; kept so the
	// catalogue can grow without silently violating a dependency.
	DependsOn []string
	// Special marks catalogue entries that are not a task slice. The only
	// value today is SpecialTransports: the grpc/quic/wss transports, which
	// the legacy Slim flag excludes with exactly these files, stubs and go.mod
	// trims.
	Special string
}

// SpecialTransports marks the transports_advanced catalogue entry.
const SpecialTransports = "transports"

// agentModuleCatalog is the ordered catalogue (display order).
var agentModuleCatalog = []AgentModule{
	{
		Name:        "transports_advanced",
		Title:       "Advanced transports",
		Description: "gRPC, QUIC and WSS beacon transports (the legacy slim/light profile excludes exactly this module)",
		Special:     SpecialTransports,
		ExclusiveFiles: []string{
			"transport_grpc.go",
			"transport_quic.go",
			"transport_wss.go",
		},
	},
	{
		Name:        "screen",
		Title:       "Screen and desktop",
		Description: "screenshots, streamed screen, on-screen triggers, remote input and window control",
		Tasks: []string{
			"screenshot", "screenshot_window", "screen_stream_start", "screen_stream_stop",
			"screen_trigger_start", "screen_trigger_stop", "remote_input",
			"window_list", "window_close",
		},
		RegistryFiles: []string{"task_registry_screen.go"},
		ExclusiveFiles: []string{
			"task_screen_trigger.go",
			"task_window.go",
			"window_windows.go",
		},
	},
	{
		Name:        "webrtc",
		Title:       "WebRTC live desktop",
		Description: "real-time H.264 screen streaming over WebRTC (Pion) with TURN relay and remote input over the signaling channel; requires ffmpeg on the target and a server with webrtc.enabled",
		Tasks: []string{
			"screen_webrtc_start", "screen_webrtc_stop",
		},
		RegistryFiles: []string{"task_registry_webrtc.go"},
		ExclusiveFiles: []string{
			"task_screen_webrtc.go",
		},
	},
	{
		Name:        "collection",
		Title:       "Collection",
		Description: "clipboard, keylogger, webcam/mic capture and browser/WeChat history theft",
		Tasks: []string{
			"clipboard_get", "clipboard_set", "keylogger_start", "keylogger_stop", "keylogger_dump",
			"webcam", "mic", "browser_history", "browser_steal", "cookie_export", "wechat_history",
		},
		RegistryFiles: []string{"task_registry_collection.go"},
		ExclusiveFiles: []string{
			"agent_webcam.go",
			"browser_history.go",
			"wechat_history.go",
		},
	},
	{
		Name:        "creds",
		Title:       "Credentials",
		Description: "credential material extraction: LSASS/katz-style dumps, DPAPI, LSA bypass, cert store, password spray, saved VPN/WiFi secrets",
		Tasks: []string{
			"creds", "mimikatz", "dpapi_masterkey", "dpapi_blob", "dpapi_browser", "lsa_bypass",
			"cert_store_list", "cred_check", "password_spray", "vpn_creds", "wifi_creds",
		},
		RegistryFiles: []string{"task_registry_creds.go"},
		ExclusiveFiles: []string{
			"task_cert_steal.go",
			"cert_steal_windows.go",
			"task_cred_check.go",
			"task_password_spray.go",
			"password_spray_windows.go",
			"password_spray_stub.go",
		},
	},
	{
		Name:        "ad",
		Title:       "Active Directory attacks",
		Description: "ADCS ESC1-8, LDAP recon, DCSync, Kerberos tickets and delegation abuse, certificate-based attacks and SharpHound collection",
		Tasks: []string{
			"adcs_find", "adcs_request", "adcs_esc1", "adcs_esc2", "adcs_esc3", "adcs_esc4",
			"adcs_esc5", "adcs_esc6", "adcs_esc7", "adcs_esc8", "adcs_full_audit",
			"ldap_users", "ldap_groups", "ldap_computers", "ldap_spn", "ldap_acl", "ldap_query",
			"dcsync", "dcsync_machine", "golden_ticket", "silver_ticket", "asreproast", "kerberoast",
			"find_delegation", "constrained_deleg", "rbcd", "bronze_bit", "adminsdholder",
			"shadow_creds", "pass_the_hash", "pass_the_ticket", "sharphound",
		},
		RegistryFiles: []string{"task_registry_ad.go"},
		ExclusiveFiles: []string{
			"task_bloodhound.go",
		},
	},
	{
		Name:        "lateral",
		Title:       "Lateral movement",
		Description: "remote execution (WMI/WinRM/PsExec/DCOM/SCF), SSH/SCP, SMB host scan, coercion attacks and the NTLM relay",
		Tasks: []string{
			"lateral", "lateral_wmi", "lateral_winrm", "lateral_psexec", "lateral_dcom",
			"lateral_scf", "lateral_list", "net", "net_scan_smb", "net_enum_hosts",
			"ssh_lateral", "ssh_keygen", "ssh_tunnel", "scp_upload",
			"coerce_printerbug", "coerce_petitpotam", "coerce_dfs",
			"relay_ntlm_start", "relay_ntlm_stop", "ntlm_help",
		},
		RegistryFiles: []string{"task_registry_lateral.go"},
		ExclusiveFiles: []string{
			"task_lateral.go",
			"task_ntlm.go",
			"lateral_scf_windows.go",
			"coerce_windows.go",
			"relay_ntlm_windows.go",
			"scanner_smb_windows.go",
			"scanner_host_enum_windows.go",
		},
	},
	{
		Name:        "injection",
		Title:       "Injection and in-memory execution",
		Description: "process injection, reflective DLL/PE loading, BOF, .NET assembly execution and process migration",
		Tasks: []string{
			"inject", "inject_methods", "list_inject_methods", "shinject", "shspawn", "spawn",
			"migrate", "bof", "bof_infection", "execute_assembly", "execute_assembly_forkrun",
			"powerpick", "peloader", "reflectdll_inject", "clr_exec_assembly", "clr_powershell",
			"kill_av",
		},
		RegistryFiles: []string{"task_registry_injection.go"},
		ExclusiveFiles: []string{
			"inject_reflectdll_windows.go",
			"ntcreatethreadex_param_windows.go",
			"ntcreatethreadex_param_windows_arm64.go",
			"task_migrate.go",
			"task_migrate_unix.go",
			"task_migrate_windows.go",
		},
	},
	{
		Name:        "evasion",
		Title:       "Evasion",
		Description: "AMSI/ETW/hardware-breakpoint bypasses, DLL-blocking and unhooking, sleep masks, ghost protocol, cleanup/wipes, kernel telemetry tampering, EDR pack and sandbox detection",
		Tasks: []string{
			"amsi_bypass", "amsi_session_bypass", "amsi_hardware_bp", "etw_bypass", "etw_ntrace_bypass",
			"etw_hardware_bp", "blockdlls", "unhook_ntdll", "protect_process",
			"cleanup", "log_wipe", "track_wipe", "self_delete",
			"set_sleep_mask", "set_sleep_mask_advanced", "run_evasion",
			"kernel_callback", "etwti", "enum_callbacks", "objcb", "imgload",
			"edr_blind", "edr_kill", "byovd_load", "ppl_check",
			"ghost_mode_status", "ghost_mode_exit", "sandbox_detect", "sandbox_detect_advanced",
		},
		RegistryFiles: []string{"task_registry_evasion.go"},
		ExclusiveFiles: []string{
			"evasion_enum_callbacks_windows.go",
			"evasion_etwti_windows.go",
			"evasion_kernel_callback_windows.go",
			"evasion_objcb_windows.go",
			"edr_pack.go",
			"edr_pack_stub.go",
			"edr_pack_windows.go",
		},
	},
	{
		Name:        "tokens",
		Title:       "Token operations",
		Description: "token listing/stealing/creating, impersonation and named-pipe impersonation",
		Tasks: []string{
			"token_list_procs", "token_steal", "token_make", "token_revert", "rev2self",
			"token_whoami", "named_pipe_impersonate",
		},
		RegistryFiles: []string{"task_registry_tokens.go"},
	},
	{
		Name:        "privesc",
		Title:       "Local privilege escalation",
		Description: "privesc checks, UAC bypasses and potato-style SYSTEM escalation",
		Tasks: []string{
			"privesc_check", "elevate", "elevate_printnightmare", "uac_bypass",
			"fodhelper", "slui", "eventvwr", "computerdefaults", "juicy_potato",
		},
		RegistryFiles: []string{"task_registry_privesc.go"},
		ExclusiveFiles: []string{
			"task_privesc.go",
		},
	},
	{
		Name:        "socks_relay",
		Title:       "SOCKS and port forwarding",
		Description: "SOCKS5 pivoting, reverse/local port forwarding and tunnel routes",
		Tasks: []string{
			"socks", "rportfwd_start", "rportfwd_stop", "lportfwd_start", "lportfwd_stop",
			"tunnel_add_route", "tunnel_remove_route",
		},
		RegistryFiles: []string{"task_registry_socks.go"},
	},
	{
		Name:        "persistence",
		Title:       "Persistence",
		Description: "add/list/remove persistence (registry, scheduled task, startup)",
		Tasks: []string{
			"persistence_add", "persistence_list", "persistence_remove",
		},
		RegistryFiles: []string{"task_registry_persistence.go"},
	},
	{
		Name:        "tun",
		Title:       "TUN device",
		Description: "start/stop the fc2tun0 tunnel device (the implementation stays; the operator loses the task surface)",
		Tasks: []string{
			"tun_start", "tun_stop",
		},
		RegistryFiles: []string{"task_registry_tun.go"},
		ExclusiveFiles: []string{
			"task_tun.go",
		},
	},
	{
		Name:        "cloud",
		Title:       "Cloud and enterprise",
		Description: "cloud token theft (AWS/Azure/GCP), Entra PRT extraction and SCCM reconnaissance",
		Tasks: []string{
			"cloud_steal", "entra_prt", "sccm_recon",
		},
		RegistryFiles: []string{"task_registry_cloud.go"},
	},
	{
		Name:        "container",
		Title:       "Container operations",
		Description: "container detection and escape helpers (Docker/K8s)",
		Tasks: []string{
			"container_detect", "container_escape", "container_docker", "container_k8s",
		},
		RegistryFiles: []string{"task_registry_container.go"},
		ExclusiveFiles: []string{
			"task_container.go",
			"container_escape_linux.go",
			"container_escape_stub.go",
			"container_linux.go",
			"container_stub.go",
		},
	},
	{
		Name:        "usb",
		Title:       "USB operations",
		Description: "USB device enumeration and payload dropping (autorun)",
		Tasks: []string{
			"usb_enum", "usb_drop",
		},
		RegistryFiles: []string{"task_registry_usb.go"},
		ExclusiveFiles: []string{
			"task_usb.go",
			"usb_windows.go",
			"usb_unix.go",
		},
	},
	{
		Name:        "p2p",
		Title:       "P2P discovery",
		Description: "gossip-based peer discovery",
		Tasks: []string{
			"gossip_discover",
		},
		RegistryFiles: []string{"task_registry_p2p.go"},
	},
}

// AgentModuleCatalog returns the catalogue in display order (a copy; callers
// may not mutate the package state).
func AgentModuleCatalog() []AgentModule {
	out := make([]AgentModule, len(agentModuleCatalog))
	copy(out, agentModuleCatalog)
	return out
}

// LookupAgentModule finds a catalogue entry by its wire name.
func LookupAgentModule(name string) (AgentModule, bool) {
	for _, m := range agentModuleCatalog {
		if m.Name == name {
			return m, true
		}
	}
	return AgentModule{}, false
}

// AgentModuleNames returns every module name in display order.
func AgentModuleNames() []string {
	out := make([]string, 0, len(agentModuleCatalog))
	for _, m := range agentModuleCatalog {
		out = append(out, m.Name)
	}
	return out
}

// NormalizeAgentModules resolves an operator selection ("include these
// modules"; nil or empty = everything) into the effective included set in
// catalogue order. Slim forces transports_advanced off — the light profile is
// defined as exactly that exclusion. Unknown names and unmet dependencies are
// rejected with an actionable error.
func NormalizeAgentModules(requested []string, slim bool) ([]string, error) {
	included := make(map[string]bool, len(agentModuleCatalog))
	if len(requested) == 0 {
		for _, m := range agentModuleCatalog {
			included[m.Name] = true
		}
	} else {
		for _, raw := range requested {
			name := strings.TrimSpace(raw)
			if name == "" {
				continue
			}
			if name == "core" {
				continue // core is always compiled; accepted for convenience
			}
			if _, ok := LookupAgentModule(name); !ok {
				return nil, fmt.Errorf("unknown agent module %q (available: %s)", name, strings.Join(AgentModuleNames(), ", "))
			}
			included[name] = true
		}
		// "core" alone (or nothing selectable after trimming) leaves the set
		// empty on purpose: the effective selection is a core-only build.
	}
	if slim {
		included["transports_advanced"] = false
	}
	// Dependencies (catalogue order keeps the result deterministic).
	for _, m := range agentModuleCatalog {
		if !included[m.Name] {
			continue
		}
		for _, dep := range m.DependsOn {
			if !included[dep] {
				return nil, fmt.Errorf("agent module %q requires module %q, which the selection excludes", m.Name, dep)
			}
		}
	}
	out := make([]string, 0, len(included))
	for _, m := range agentModuleCatalog {
		if included[m.Name] {
			out = append(out, m.Name)
		}
	}
	return out, nil
}

// ExcludedAgentModules returns the catalogue entries NOT in the included set,
// in display order.
func ExcludedAgentModules(included []string) []AgentModule {
	keep := make(map[string]bool, len(included))
	for _, name := range included {
		keep[name] = true
	}
	out := make([]AgentModule, 0, len(agentModuleCatalog))
	for _, m := range agentModuleCatalog {
		if !keep[m.Name] {
			out = append(out, m)
		}
	}
	return out
}

// AgentModuleExcludedTasks maps every task type bound by an excluded module to
// that module's name. It becomes the generated modules_excluded.go map inside
// the built agent.
func AgentModuleExcludedTasks(excluded []AgentModule) map[string]string {
	out := make(map[string]string)
	for _, m := range excluded {
		for _, t := range m.Tasks {
			out[t] = m.Name
		}
	}
	return out
}

// SortedExcludedTaskTypes returns the excluded task types in deterministic
// order (used by the generated source and tests).
func SortedExcludedTaskTypes(excluded []AgentModule) []string {
	out := make([]string, 0)
	for _, m := range excluded {
		out = append(out, m.Tasks...)
	}
	sort.Strings(out)
	return out
}
