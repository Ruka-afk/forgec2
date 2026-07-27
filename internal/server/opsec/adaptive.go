package opsec

import (
	"log/slog"
	"sync"
	"time"
)

type ThreatLevel int

const (
	ThreatNormal   ThreatLevel = 0
	ThreatElevated ThreatLevel = 1
	ThreatHigh     ThreatLevel = 2
	ThreatCritical ThreatLevel = 3
)

type AgentThreatState struct {
	AgentID         string
	Level           ThreatLevel
	IntegrityCount  int
	FirstFailureAt  time.Time
	LastFailureAt   time.Time
	ActiveChecks    []string
	BlockedActions  []string
	EnvironmentInfo map[string]string
}

type AdaptiveManager struct {
	mu     sync.RWMutex
	states map[string]*AgentThreatState
}

func NewAdaptiveManager() *AdaptiveManager {
	return &AdaptiveManager{
		states: make(map[string]*AgentThreatState),
	}
}

func (am *AdaptiveManager) RecordIntegrityFailure(agentID string) ThreatLevel {
	am.mu.Lock()
	defer am.mu.Unlock()

	state, exists := am.states[agentID]
	if !exists {
		state = &AgentThreatState{
			AgentID:         agentID,
			EnvironmentInfo: make(map[string]string),
			FirstFailureAt:  time.Now(),
		}
		am.states[agentID] = state
	}

	state.IntegrityCount++
	state.LastFailureAt = time.Now()

	state.Level = am.calculateThreatLevel(state)

	slog.Warn("Agent threat level updated",
		"agent_id", agentID,
		"threat_level", state.Level,
		"integrity_failures", state.IntegrityCount)

	return state.Level
}

func (am *AdaptiveManager) calculateThreatLevel(state *AgentThreatState) ThreatLevel {
	switch {
	case state.IntegrityCount >= 10:
		return ThreatCritical
	case state.IntegrityCount >= 5:
		return ThreatHigh
	case state.IntegrityCount >= 2:
		return ThreatElevated
	default:
		return ThreatNormal
	}
}

func (am *AdaptiveManager) GetThreatLevel(agentID string) ThreatLevel {
	am.mu.RLock()
	defer am.mu.RUnlock()

	state, exists := am.states[agentID]
	if !exists {
		return ThreatNormal
	}
	return state.Level
}

func (am *AdaptiveManager) GetRecommendedSleepParams(agentID string) (sleepTime int, jitter int, rotateKey bool) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	state, exists := am.states[agentID]
	if !exists {
		return 60, 25, false
	}

	switch state.Level {
	case ThreatCritical:
		return 10, 50, true
	case ThreatHigh:
		return 20, 40, true
	case ThreatElevated:
		return 30, 35, false
	default:
		return 60, 25, false
	}
}

func (am *AdaptiveManager) ShouldBlockAction(agentID string, action string) bool {
	am.mu.RLock()
	defer am.mu.RUnlock()

	state, exists := am.states[agentID]
	if !exists {
		return false
	}

	if state.Level == ThreatCritical {
		blockList := map[string]bool{"mimikatz": true, "dcsync": true, "kerberoast": true, "shinject": true}
		if blockList[action] {
			slog.Warn("Blocked high-risk action due to critical threat level",
				"agent_id", agentID, "action", action)
			return true
		}
	}
	return false
}

func (am *AdaptiveManager) RecordEnvironmentCheck(agentID, checkName, result string) {
	am.mu.Lock()
	defer am.mu.Unlock()

	state, exists := am.states[agentID]
	if !exists {
		state = &AgentThreatState{
			AgentID:         agentID,
			EnvironmentInfo: make(map[string]string),
		}
		am.states[agentID] = state
	}
	state.EnvironmentInfo[checkName] = result
}

func (am *AdaptiveManager) DecayThreatLevel(agentID string) {
	am.mu.Lock()
	defer am.mu.Unlock()

	state, exists := am.states[agentID]
	if !exists {
		return
	}

	if time.Since(state.LastFailureAt) > 10*time.Minute && state.Level > ThreatNormal {
		state.Level--
		slog.Info("Agent threat level decayed",
			"agent_id", agentID, "new_level", state.Level)
	}
}

func (am *AdaptiveManager) StartDecayLoop() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			am.mu.Lock()
			for id := range am.states {
				am.decayLocked(id)
			}
			am.mu.Unlock()
		}
	}()
}

func (am *AdaptiveManager) decayLocked(agentID string) {
	state, exists := am.states[agentID]
	if !exists {
		return
	}
	if time.Since(state.LastFailureAt) > 10*time.Minute && state.Level > ThreatNormal {
		state.Level--
	}
}

func (am *AdaptiveManager) GetState(agentID string) *AgentThreatState {
	am.mu.RLock()
	defer am.mu.RUnlock()
	state, exists := am.states[agentID]
	if !exists {
		return nil
	}
	return state
}
