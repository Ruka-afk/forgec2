//go:build !windows
// +build !windows

package main

// Non-Windows hostinfo collectors. Linux/macOS expose an honest subset:
// system basics from /proc and network interfaces; everything that depends
// on Windows-only surfaces reports available=false instead of pretending.

import (
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

func unavailable(reason string) map[string]any {
	return map[string]any{"available": false, "reason": reason}
}

// collectHostSecurity: AV/EDR enumeration on POSIX would require product
// specific knowledge per distro; report honestly instead of guessing.
func collectHostSecurity() map[string]any {
	out := unavailable("AV/EDR enumeration is Windows-only in this build")
	// A minimal honest signal: presence of common EDR kernel modules / agents.
	hits := []string{}
	if entries, err := os.ReadDir("/opt"); err == nil {
		for _, e := range entries {
			name := strings.ToLower(e.Name())
			for _, marker := range []string{"crowdstrike", "sentinelone", "carbonblack", "cylance", "sophos"} {
				if strings.Contains(name, marker) {
					hits = append(hits, "/opt/"+e.Name())
				}
			}
		}
	}
	out["opt_indicators"] = hits
	return out
}

// collectHostSystem reads uptime and memory from /proc where present.
func collectHostSystem() map[string]any {
	out := map[string]any{"platform": runtime.GOOS}
	if b, err := os.ReadFile("/proc/uptime"); err == nil {
		fields := strings.Fields(string(b))
		if len(fields) > 0 {
			if secs, perr := strconv.ParseFloat(fields[0], 64); perr == nil {
				out["uptime_seconds"] = int64(secs)
			}
		}
	}
	if b, err := os.ReadFile("/proc/meminfo"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "MemTotal:") || strings.HasPrefix(line, "MemAvailable:") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					if kb, perr := strconv.ParseUint(parts[1], 10, 64); perr == nil {
						key := "mem_total_kb"
						if strings.HasPrefix(line, "MemAvailable") {
							key = "mem_available_kb"
						}
						out[key] = kb
					}
				}
			}
		}
	}
	if out["uptime_seconds"] == nil && out["mem_total_kb"] == nil {
		return unavailable("/proc not available")
	}
	if info := getSystemInfo(); info != nil {
		for _, k := range []string{"username", "integrity", "elevated", "domain"} {
			if v, ok := info[k]; ok && v != "" {
				out[k] = v
			}
		}
	}
	return out
}

// collectHostSoftware: package-manager sweeps are distro-specific; keep the
// contract honest.
func collectHostSoftware(filter string) map[string]any {
	return unavailable("installed-software enumeration is Windows-only in this build")
}

// collectHostNetwork reports interfaces plus proxy environment variables.
func collectHostNetwork() map[string]any {
	out := map[string]any{"platform": runtime.GOOS}
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
			if list, aerr := ifc.Addrs(); aerr == nil {
				for _, a := range list {
					addrs = append(addrs, a.String())
				}
			}
			entry["addresses"] = addrs
			adapters = append(adapters, entry)
		}
	}
	out["adapters"] = adapters
	proxy := map[string]any{}
	for _, envName := range []string{"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "no_proxy"} {
		if v := os.Getenv(envName); v != "" {
			proxy[strings.ToLower(envName)] = v
		}
	}
	out["proxy"] = proxy
	return out
}

// collectHostRuntime: autoruns/scheduled tasks/logon are platform-specific;
// surface cron as the closest honest equivalent when present.
func collectHostRuntime() map[string]any {
	out := map[string]any{"platform": runtime.GOOS}
	if raw, err := exec.Command("crontab", "-l").Output(); err == nil && len(raw) > 0 {
		lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
		if len(lines) > 25 {
			lines = lines[:25]
		}
		out["cron_sample"] = strings.Join(lines, "\n")
	} else {
		out["cron_sample"] = ""
	}
	return out
}
