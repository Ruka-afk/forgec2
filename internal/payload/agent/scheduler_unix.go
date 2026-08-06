//go:build linux || darwin
// +build linux darwin

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"sync"
)

// Portable trigger detection for the beacon scheduler on Unix targets.
var (
	netStateMu  sync.Mutex
	lastNetHash string
)

// detectUserActivity returns false on Unix: there is no reliable global input
// API, so the scheduler relies on its interval/office-hours modes instead.
func detectUserActivity() bool {
	return false
}

// detectNetworkChange diffs a hash of /proc/net/dev so an interface up/down or
// route change triggers the network scheduler trigger (one-shot per change).
func detectNetworkChange() bool {
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return false
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	netStateMu.Lock()
	defer netStateMu.Unlock()
	if lastNetHash == "" {
		lastNetHash = hash
		return false
	}
	if hash != lastNetHash {
		lastNetHash = hash
		return true
	}
	return false
}