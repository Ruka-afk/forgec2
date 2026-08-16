//go:build linux

package main

import (
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// userIdleSeconds returns seconds since last user input on Linux via X11's
// xprintidle when available; falls back to a large value (unknown) so beacon
// cadence is left unchanged when no display/idle source can be probed. The
// probe is best-effort and never blocks the beacon loop for long.
func userIdleSeconds() int {
	if _, err := exec.LookPath("xprintidle"); err != nil {
		return 9999
	}
	out, err := exec.Command("xprintidle").Output()
	if err != nil {
		return 9999
	}
	ms, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 9999
	}
	return ms / 1000
}

var _ = time.Second
