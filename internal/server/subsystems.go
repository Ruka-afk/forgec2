package server

import (
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// TransportManager groups all network transport fields and provides
// lifecycle management for listeners. It does NOT hold handler logic —
// that remains on Server. This struct exists to establish clear field
// ownership boundaries for future decomposition.
type TransportManager struct {
	// WebSocket connections
	wsClients  map[*websocket.Conn]*wsClientConn
	wsMutex    sync.RWMutex
	wsUpgrader websocket.Upgrader

	// C2 listeners
	dnsListener      *DNSBeaconListener
	icmpListener     *ICMPBeaconListener
	grpcListener     *GRPCListener
	smbLn            net.Listener
	tcpLn            net.Listener
	tcpProtoListener *TCPProtoListener

	// External C2 channels (Discord/Slack/WebSocket relay)
	extC2Channels  map[string]*extC2WSChannel
	extC2ChannelsMu sync.Mutex
	extC2TaskQueue map[string][]extC2Task
	extC2TaskMu    sync.Mutex
	extC2Notify    map[string]chan struct{}

	// Extra listeners created dynamically via UI
	extraListeners   map[string]io.Closer
	extraListenersMu sync.Mutex

	// Reverse port forward
	rportfwdListeners map[string]*rportfwdRelay
	rportfwdMu        sync.Mutex

	// NTLM relay session tracking
	ntlmRelays *ntlmRelayStore

	// Socks proxy
	socksEngine *socksRelayEngine
}

// NewTransportManager initializes all maps and internal state.
func NewTransportManager() *TransportManager {
	return &TransportManager{
		wsClients:        make(map[*websocket.Conn]*wsClientConn),
		extC2Channels:    make(map[string]*extC2WSChannel),
		extC2TaskQueue:   make(map[string][]extC2Task),
		extC2Notify:      make(map[string]chan struct{}),
		extraListeners:   make(map[string]io.Closer),
		rportfwdListeners: make(map[string]*rportfwdRelay),
	}
}

// StopAllListeners gracefully shuts down all registered listeners.
func (tm *TransportManager) StopAllListeners() {
	tm.extraListenersMu.Lock()
	for id, l := range tm.extraListeners {
		if err := l.Close(); err != nil {
			slog.Warn("Failed to close extra listener", "id", id, "error", err)
		}
	}
	tm.extraListeners = make(map[string]io.Closer)
	tm.extraListenersMu.Unlock()

	if tm.socksEngine != nil {
		tm.socksEngine.cleanup()
	}
}

// ActiveListenerCount returns the number of dynamically-created listeners.
func (tm *TransportManager) ActiveListenerCount() int {
	tm.extraListenersMu.Lock()
	defer tm.extraListenersMu.Unlock()
	return len(tm.extraListeners)
}

// ExtC2ChannelCount returns the number of active external C2 channels.
func (tm *TransportManager) ExtC2ChannelCount() int {
	tm.extC2ChannelsMu.Lock()
	defer tm.extC2ChannelsMu.Unlock()
	return len(tm.extC2Channels)
}

// AgentTracker groups per-agent state tracking fields to separate
// agent monitoring concerns from transport and handler logic.
type AgentTracker struct {
	// Per-agent pending task depth
	agentPendingTasks   map[string]int
	agentPendingTasksMu sync.Mutex

	// Flapping suppression: track last status event time per agent
	agentStatusCooldown   map[string]time.Time
	agentStatusCooldownMu sync.Mutex

	// Screen capture monitoring
	screenMonitorImplants map[string]time.Time
	screenMonitorMu       sync.Mutex

	// Async build job tracking
	buildJobs   map[string]*BuildJob
	buildJobsMu sync.RWMutex
}

// NewAgentTracker initializes all maps.
func NewAgentTracker() *AgentTracker {
	return &AgentTracker{
		agentPendingTasks:   make(map[string]int),
		agentStatusCooldown: make(map[string]time.Time),
		screenMonitorImplants: make(map[string]time.Time),
		buildJobs:           make(map[string]*BuildJob),
	}
}

// SetPendingTasks sets the pending task count for an agent.
func (at *AgentTracker) SetPendingTasks(agentID string, count int) {
	at.agentPendingTasksMu.Lock()
	defer at.agentPendingTasksMu.Unlock()
	at.agentPendingTasks[agentID] = count
}

// GetPendingTasks returns the pending task count for an agent.
func (at *AgentTracker) GetPendingTasks(agentID string) int {
	at.agentPendingTasksMu.Lock()
	defer at.agentPendingTasksMu.Unlock()
	return at.agentPendingTasks[agentID]
}

// RemoveAgent cleans up all tracking state for an agent.
func (at *AgentTracker) RemoveAgent(agentID string) {
	at.agentPendingTasksMu.Lock()
	delete(at.agentPendingTasks, agentID)
	at.agentPendingTasksMu.Unlock()

	at.agentStatusCooldownMu.Lock()
	delete(at.agentStatusCooldown, agentID)
	at.agentStatusCooldownMu.Unlock()

	at.screenMonitorMu.Lock()
	delete(at.screenMonitorImplants, agentID)
	at.screenMonitorMu.Unlock()
}

// IsFlapping returns true if the agent's last status change was within
// the cooldown window (5 seconds), used to suppress flapping alerts.
func (at *AgentTracker) IsFlapping(agentID string) bool {
	at.agentStatusCooldownMu.Lock()
	defer at.agentStatusCooldownMu.Unlock()
	last, ok := at.agentStatusCooldown[agentID]
	if !ok {
		return false
	}
	return time.Since(last) < 5*time.Second
}

// RecordStatusChange records the time of a status change for flapping detection.
func (at *AgentTracker) RecordStatusChange(agentID string) {
	at.agentStatusCooldownMu.Lock()
	defer at.agentStatusCooldownMu.Unlock()
	at.agentStatusCooldown[agentID] = time.Now()
}
