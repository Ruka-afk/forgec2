package server

import (
	"strings"
	"sync"
)

// credCheckFuseMax is the number of consecutive invalid/locked results per
// (agent, domain) allowed before the server refuses to dispatch further
// credential checks. Mirrors the agent-side fuse to protect target accounts
// from lockouts even when agents are re-queued or redeployed.
const credCheckFuseMax = 5

// credCheckFuseTracker is a process-level consecutive-failure counter keyed by
// agent ID + lowercase domain. A "valid" result resets the counter.
type credCheckFuseTracker struct {
	mu       sync.Mutex
	failures map[string]int
}

func newCredCheckFuseTracker() *credCheckFuseTracker {
	return &credCheckFuseTracker{failures: make(map[string]int)}
}

func credCheckFuseKey(agentID, domain string) string {
	return agentID + "|" + strings.ToLower(strings.TrimSpace(domain))
}

func (f *credCheckFuseTracker) tripped(agentID, domain string) bool {
	if f == nil {
		return false
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.failures[credCheckFuseKey(agentID, domain)] >= credCheckFuseMax
}

func (f *credCheckFuseTracker) recordFailure(agentID, domain string) {
	if f == nil {
		return
	}
	f.mu.Lock()
	f.failures[credCheckFuseKey(agentID, domain)]++
	f.mu.Unlock()
}

func (f *credCheckFuseTracker) reset(agentID, domain string) {
	if f == nil {
		return
	}
	f.mu.Lock()
	delete(f.failures, credCheckFuseKey(agentID, domain))
	f.mu.Unlock()
}
