package server

import (
	"log/slog"
	"strconv"
	"strings"

	"github.com/forgec2/forgec2/internal/db"
)

const redirectorTargetPrefix = "redirector:"

// listenerTargetID returns the circuit-breaker target id for a listener.
// Bare numeric form so rotateAgentsOnBurnedListener can query listener_id
// directly from the burned target id.
func listenerTargetID(l *db.Listener) string {
	return strconv.FormatUint(uint64(l.ID), 10)
}

func redirectorTargetID(rd *db.Redirector) string {
	return redirectorTargetPrefix + strconv.FormatUint(uint64(rd.ID), 10)
}

// probeableScheme reports whether the scheme can be health-probed.
func probeableScheme(scheme string) bool {
	switch scheme {
	case "http", "https", "tcp", "tls":
		return true
	default:
		return false
	}
}

// syncListenerProbe registers an enabled listener with the circuit breaker
// (so its health is probed and agents rotate off it when burned) and removes
// the target when the listener is disabled or uses a non-probeable scheme.
func (s *Server) syncListenerProbe(l *db.Listener) {
	if s.circuitBreaker == nil {
		return
	}
	scheme := l.Scheme
	if scheme == "" {
		scheme = l.Type
	}
	if !l.Enabled || !probeableScheme(scheme) {
		s.circuitBreaker.UnregisterTarget(listenerTargetID(l))
		return
	}
	host := l.Host
	if host == "" {
		host = "localhost"
	}
	s.circuitBreaker.RegisterTarget(listenerTargetID(l), scheme, host, l.Port)
}

// syncRedirectorProbe registers an active redirector with the circuit
// breaker. Redirectors are probed on 443/80 (https fallback http) since the
// redirector's public front-end is what agents connect through; a burned or
// unreachable redirector is surfaced in /api/circuit-breaker/status.
func (s *Server) syncRedirectorProbe(rd *db.Redirector) {
	if s.circuitBreaker == nil {
		return
	}
	id := redirectorTargetID(rd)
	if rd.Status != "active" || rd.Host == "" {
		s.circuitBreaker.UnregisterTarget(id)
		return
	}
	// Prefer https on 443; fall back to probing 80 as the standard
	// redirector front-door when the configured host is plain http.
	port, scheme := 443, "https"
	if rd.Config != "" && strings.Contains(rd.Config, "listen 80;") {
		port, scheme = 80, "http"
	}
	s.circuitBreaker.RegisterTarget(id, scheme, rd.Host, port)
}

// registerExistingProbeTargets scans the DB on startup so the probe loop
// never spins on an empty map (the pre-wiring behavior: RegisterTarget had
// no callers and health-driven failover was a dead capability).
func (s *Server) registerExistingProbeTargets() {
	var listeners []db.Listener
	if err := s.db.Find(&listeners).Error; err != nil {
		slog.Error("Circuit breaker: failed to load listeners for probing", "err", err)
	} else {
		for i := range listeners {
			s.syncListenerProbe(&listeners[i])
		}
	}

	var redirectors []db.Redirector
	if err := s.db.Find(&redirectors).Error; err != nil {
		slog.Error("Circuit breaker: failed to load redirectors for probing", "err", err)
	} else {
		for i := range redirectors {
			s.syncRedirectorProbe(&redirectors[i])
		}
	}
	if s.circuitBreaker != nil {
		slog.Info("Circuit breaker targets registered", "targets", len(s.circuitBreaker.GetAllStatus()))
	}
}