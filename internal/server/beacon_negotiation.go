package server

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/forgec2/forgec2/pkg/protocol"
)

// protocolNoticeTTL bounds how long a protocol notice is remembered per
// agent. An agent beacons every minute, so without this the notice would
// write an audit row (and a log line) every minute forever.
const protocolNoticeTTL = time.Hour

// defaultProtocolCompatWindow is the compatibility window used when no
// configuration is loaded (a bare test server).
const defaultProtocolCompatWindow = 1

// protocolCompatWindow returns the configured compatibility window: how many
// protocol versions older than the current one the server still serves.
func (s *Server) protocolCompatWindow() int {
	if s == nil || s.cfg == nil {
		return defaultProtocolCompatWindow
	}
	s.configMu.RLock()
	window := s.cfg.Protocol.CompatWindow
	s.configMu.RUnlock()
	return window
}

// negotiateProtocol decides whether this server serves an agent that speaks
// the given protocol version.
//
// The outcome is made visible instead of silent: a rejection is counted
// (per reason), audited and explained, and an agent that is served on a
// deprecated version is counted too, so the operator can see how many
// deployed agents still need a rebuild.
func (s *Server) negotiateProtocol(agentID string, agentVersion uint) protocol.Decision {
	decision := protocol.Negotiate(agentVersion, s.protocolCompatWindow())
	switch {
	case !decision.Serve:
		slog.Warn("Agent protocol version rejected",
			"agent_id", agentID,
			"agent_pv", agentVersion,
			"server_pv", protocol.CurrentProtocolVersion,
			"reason", decision.Reason)
		if s.metrics != nil {
			s.metrics.BeaconProtocolRejectedTotal.WithLabelValues(string(decision.Level)).Inc()
		}
		s.protocolNotice(agentID, "agent_protocol_rejected", decision.Reason, false)
	case decision.Level == protocol.SupportDeprecated:
		slog.Warn("Agent speaks a deprecated protocol version",
			"agent_id", agentID,
			"agent_pv", agentVersion,
			"server_pv", protocol.CurrentProtocolVersion)
		if s.metrics != nil {
			s.metrics.BeaconProtocolDeprecatedTotal.Inc()
		}
		s.protocolNotice(agentID, "agent_protocol_deprecated", decision.Reason, true)
	}
	return decision
}

// protocolNotice records a protocol notice for an agent at most once per
// TTL, so a beaconing agent does not flood the audit log with the same
// message.
func (s *Server) protocolNotice(agentID, action, details string, success bool) {
	if agentID == "" {
		return
	}
	now := time.Now()

	s.protocolNoticeMu.Lock()
	if s.protocolNotices == nil {
		s.protocolNotices = make(map[string]time.Time)
	}
	last, seen := s.protocolNotices[agentID]
	if seen && now.Sub(last) < protocolNoticeTTL {
		s.protocolNoticeMu.Unlock()
		return
	}
	s.protocolNotices[agentID] = now
	// Keep the map bounded: an agent that stops beaconing would otherwise
	// stay in it forever.
	if len(s.protocolNotices) > 4096 {
		for id, ts := range s.protocolNotices {
			if now.Sub(ts) > protocolNoticeTTL {
				delete(s.protocolNotices, id)
			}
		}
	}
	s.protocolNoticeMu.Unlock()

	s.LogAuditRecord(nil, action, "agent", agentID, details, success, nil)
}

// protocolState is the operator-facing summary of the protocol negotiation:
// what the server speaks, what it still serves, and what it refuses.
func (s *Server) protocolState() map[string]interface{} {
	window := s.protocolCompatWindow()
	versions := make([]map[string]interface{}, 0, len(protocol.Versions()))
	for _, v := range protocol.Versions() {
		entry := map[string]interface{}{
			"version":           v.Version,
			"deprecated":        v.Deprecated,
			"server_compatible": v.ServerCompatible,
			"since":             v.Since,
			"note":              v.Note,
		}
		switch {
		case v.Version == protocol.CurrentProtocolVersion:
			entry["status"] = "current"
		case v.ServerCompatible && protocol.CurrentProtocolVersion-v.Version <= uint(window):
			entry["status"] = "served"
		case v.ServerCompatible:
			entry["status"] = "window_closed"
		default:
			entry["status"] = "incompatible"
		}
		versions = append(versions, entry)
	}
	return map[string]interface{}{
		"current_version": protocol.CurrentProtocolVersion,
		"compat_window":   window,
		"versions":        versions,
	}
}

// protocolRejectionMessage is the operator-facing text for a refusal. It is
// used where the decision has to travel as a plain string (the beacon's
// rejection path on transports that cannot carry a body).
func protocolRejectionMessage(decision protocol.Decision) string {
	if decision.Reason != "" {
		return decision.Reason
	}
	return fmt.Sprintf("the agent speaks protocol v%d which this server does not serve",
		decision.NegotiatedVersion)
}
