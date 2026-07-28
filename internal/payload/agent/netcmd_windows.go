//go:build windows
// +build windows

package main

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
)

func executeNetCommand(cmd string) string {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return "error: no net subcommand"
	}
	fullCmd := exec.Command("net.exe", parts...)
	fullCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	var out bytes.Buffer
	fullCmd.Stdout = &out
	fullCmd.Stderr = &out
	err := fullCmd.Run()
	if err != nil {
		return "error: " + err.Error() + "\n" + out.String()
	}
	parsed := parseNetOutput(parts[0], out.String())
	return parsed
}

func parseNetOutput(subcommand string, raw string) string {
	var result interface{}
	switch subcommand {
	case "view":
		result = parseNetView(raw)
	case "group":
		result = parseNetGroup(raw)
	case "localgroup":
		result = parseNetLocalGroup(raw)
	case "user":
		result = parseNetUser(raw)
	case "accounts":
		result = parseNetAccounts(raw)
	case "share":
		result = parseNetShare(raw)
	default:
		return raw
	}
	jsonBytes, _ := json.MarshalIndent(result, "", "  ")
	return string(jsonBytes)
}

func parseNetView(raw string) []map[string]string {
	var result []map[string]string
	lines := strings.Split(raw, "\n")
	inTable := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "---") {
			inTable = true
			continue
		}
		if !inTable {
			continue
		}
		if !strings.HasPrefix(trimmed, "\\\\") {
			continue
		}
		fields := regexp.MustCompile(`\s{2,}`).Split(trimmed, -1)
		entry := make(map[string]string)
		if len(fields) > 0 {
			entry["server"] = strings.TrimSpace(fields[0])
		}
		if len(fields) > 1 {
			entry["type"] = strings.TrimSpace(fields[1])
		}
		if len(fields) > 2 {
			entry["comment"] = strings.TrimSpace(strings.Join(fields[2:], " "))
		}
		result = append(result, entry)
	}
	return result
}

func parseNetGroup(raw string) []map[string]string {
	var result []map[string]string
	lines := strings.Split(raw, "\n")
	inTable := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "---") {
			inTable = true
			continue
		}
		if !inTable {
			continue
		}
		if strings.HasPrefix(trimmed, "The command completed") || strings.HasPrefix(trimmed, "Group Accounts") {
			continue
		}
		fields := regexp.MustCompile(`\s{2,}`).Split(trimmed, -1)
		if len(fields) == 0 || fields[0] == "" {
			continue
		}
		entry := make(map[string]string)
		entry["group"] = strings.TrimSpace(fields[0])
		if len(fields) > 1 {
			entry["comment"] = strings.TrimSpace(strings.Join(fields[1:], " "))
		}
		result = append(result, entry)
	}
	return result
}

func parseNetLocalGroup(raw string) []map[string]string {
	var result []map[string]string
	lines := strings.Split(raw, "\n")
	inTable := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "---") {
			inTable = true
			continue
		}
		if !inTable {
			continue
		}
		if strings.HasPrefix(trimmed, "The command completed") {
			continue
		}
		fields := regexp.MustCompile(`\s{2,}`).Split(trimmed, -1)
		if len(fields) == 0 || fields[0] == "" {
			continue
		}
		entry := make(map[string]string)
		entry["member"] = strings.TrimSpace(fields[0])
		if len(fields) > 1 {
			entry["info"] = strings.TrimSpace(strings.Join(fields[1:], " "))
		}
		result = append(result, entry)
	}
	return result
}

func parseNetUser(raw string) []map[string]string {
	var result []map[string]string
	lines := strings.Split(raw, "\n")

	isDetail := false
	for _, line := range lines {
		if strings.Contains(line, "\\\\") || strings.Contains(line, "---") {
			continue
		}
		if strings.Contains(line, ":") && len(line) < 80 {
			isDetail = true
		}
	}

	if isDetail {
		entry := make(map[string]string)
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "The command completed") {
				continue
			}
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				if key != "" {
					entry[key] = val
				}
			}
		}
		if len(entry) > 0 {
			result = append(result, entry)
		}
		return result
	}

	inTable := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "---") {
			inTable = true
			continue
		}
		if !inTable {
			continue
		}
		if strings.HasPrefix(trimmed, "The command completed") {
			continue
		}
		fields := regexp.MustCompile(`\s{2,}`).Split(trimmed, -1)
		if len(fields) == 0 || fields[0] == "" {
			continue
		}
		entry := make(map[string]string)
		entry["username"] = strings.TrimSpace(fields[0])
		if len(fields) > 1 {
			entry["detail"] = strings.TrimSpace(strings.Join(fields[1:], " "))
		}
		result = append(result, entry)
	}
	return result
}

func parseNetAccounts(raw string) map[string]string {
	result := make(map[string]string)
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "The command completed") {
			continue
		}
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			if key != "" {
				result[key] = val
			}
		}
	}
	return result
}

func parseNetShare(raw string) []map[string]string {
	var result []map[string]string
	lines := strings.Split(raw, "\n")
	inTable := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "---") {
			inTable = true
			continue
		}
		if !inTable {
			continue
		}
		if strings.HasPrefix(trimmed, "The command completed") {
			continue
		}
		fields := regexp.MustCompile(`\s{2,}`).Split(trimmed, -1)
		if len(fields) == 0 || fields[0] == "" {
			continue
		}
		entry := make(map[string]string)
		entry["share"] = strings.TrimSpace(fields[0])
		if len(fields) > 1 {
			entry["resource"] = strings.TrimSpace(fields[1])
		}
		if len(fields) > 2 {
			entry["remark"] = strings.TrimSpace(strings.Join(fields[2:], " "))
		}
		result = append(result, entry)
	}
	return result
}
