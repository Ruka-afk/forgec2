package server

import (
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"
)

// lportfwd server-side relay.
//
// The agent binds a loopback listener (lportfwd_start task) and frames every
// accepted local connection through the beacon channel:
//
//	agent -> server : lportfwd_connect(target), lportfwd_data, lportfwd_close
//	server -> agent : lportfwd_data, lportfwd_close
//
// On lportfwd_connect this side dials the target, spawns a read pump that
// converts target bytes into outbound frames for the agent, and registers the
// connection so inbound data frames are written to the target. Frames toward
// the agent ride a dedicated socksEngine FIFO (collectSocksFrames), isolated
// from the SOCKS control queue so bulk transfer can't evict closes.

const (
	lportfwdDialTimeout    = 10 * time.Second
	lportfwdWriteTimeout   = 30 * time.Second
	lportfwdMaxFrameTarget = 256 // sanity cap on target string length
)

// lportfwdMaxConnsPerAgent caps simultaneous tunneled legs per agent: each
// holds an FD plus a read-pump goroutine and beacon-queue backlog, so an
// unbounded lportfwd_connect flood is an FD/goroutine exhaustion primitive
// (rportfwd already caps at 64 per relay).
const lportfwdMaxConnsPerAgent = 64

// lportfwdConnIdleTimeout reaps tunneled legs idle this long. Without it a
// forgotten session pins the FD, the pump goroutine and the queue backlog.
const lportfwdConnIdleTimeout = 10 * time.Minute

type lportfwdTarget struct {
	agentID    string
	target     string
	tcpConn    net.Conn
	lastActive time.Time
}

// processLPortFwdData handles one inbound frame from an agent. Called from
// processSOCKSRelay for every frame whose action carries the lportfwd_ prefix.
func (s *Server) processLPortFwdData(agentID string, f socksFrame) {
	switch f.Action {
	case "lportfwd_connect":
		s.lportfwdConnect(agentID, f.ConnID, string(f.Data))
	case "lportfwd_data":
		s.lportfwdWrite(agentID, f.ConnID, f.Data)
	case "lportfwd_close":
		s.lportfwdClose(agentID, f.ConnID)
	}
}

// lportfwdKey namespaces connection ids per agent: connID counters restart
// at 1 inside each implant, so two agents using the same id would otherwise
// overwrite each other's entries (cross-agent data injection / closes).
func lportfwdKey(agentID string, connID uint64) string {
	return agentID + "|" + strconv.FormatUint(connID, 10)
}

func connIDFromKey(key string) uint64 {
	if idx := strings.LastIndex(key, "|"); idx >= 0 {
		if v, err := strconv.ParseUint(key[idx+1:], 10, 64); err == nil {
			return v
		}
	}
	return 0
}

func (s *Server) lportfwdConnect(agentID string, connID uint64, target string) {
	if len(target) == 0 || len(target) > lportfwdMaxFrameTarget {
		slog.Warn("lportfwd: rejecting connect with invalid target", "agent_id", agentID, "conn", connID)
		s.socksEngine.enqueueLPortFwdFrame(agentID, socksFrame{ConnID: connID, Action: "lportfwd_close"})
		return
	}
	// P1 fix: the connect frame is agent-controlled. Only dial targets the
	// operator declared via lportfwd_start (and that pass safety checks) —
	// otherwise any implant gets an arbitrary-dial SSRF primitive into the
	// teamserver's network (127.0.0.1 admin ports, cloud metadata, ...).
	if !s.lportfwdTargetAllowed(agentID, target) {
		slog.Warn("lportfwd: rejecting undeclared or unsafe target", "agent_id", agentID, "conn", connID)
		s.socksEngine.enqueueLPortFwdFrame(agentID, socksFrame{ConnID: connID, Action: "lportfwd_close"})
		return
	}
	// Per-agent connection cap: each leg pins an FD + pump goroutine.
	s.lportfwdMu.Lock()
	connsForAgent := 0
	for _, t := range s.lportfwdTargets {
		if t.agentID == agentID {
			connsForAgent++
		}
	}
	s.lportfwdMu.Unlock()
	if connsForAgent >= lportfwdMaxConnsPerAgent {
		slog.Warn("lportfwd: at connection cap, refusing", "agent_id", agentID, "cap", lportfwdMaxConnsPerAgent)
		s.socksEngine.enqueueLPortFwdFrame(agentID, socksFrame{ConnID: connID, Action: "lportfwd_close"})
		return
	}
	conn, err := net.DialTimeout("tcp", target, lportfwdDialTimeout)
	if err != nil {
		slog.Info("lportfwd: target dial failed", "agent_id", agentID, "conn", connID, "target", target, "error", err)
		s.socksEngine.enqueueLPortFwdFrame(agentID, socksFrame{ConnID: connID, Action: "lportfwd_close"})
		return
	}

	t := &lportfwdTarget{agentID: agentID, target: target, tcpConn: conn, lastActive: time.Now()}
	key := lportfwdKey(agentID, connID)
	s.lportfwdMu.Lock()
	// Duplicate connect for the same connID (replay/retry): close the existing
	// leg first so its socket + read pump are not orphaned, keeping the map
	// entry unique and preventing cross-talk on a reused connID.
	if prev, exists := s.lportfwdTargets[key]; exists {
		prev.tcpConn.Close()
	}
	s.lportfwdTargets[key] = t
	s.lportfwdMu.Unlock()

	slog.Info("lportfwd: target connected", "agent_id", agentID, "conn", connID, "target", target)

	// Read pump: target -> agent. Frames queue for the next beacon; when the
	// target closes we tell the agent to drop its local leg. Queue overflow
	// closes loudly (dropped data would corrupt the TCP stream silently).
	go func() {
		buf := make([]byte, 16*1024)
		for {
			n, rerr := conn.Read(buf)
			if n > 0 {
				ok := s.socksEngine.enqueueLPortFwdFrame(agentID, socksFrame{
					ConnID: connID,
					Action: "lportfwd_data",
					Data:   append([]byte{}, buf[:n]...),
				})
				s.lportfwdMu.Lock()
				if cur, tracked := s.lportfwdTargets[key]; tracked && cur == t {
					cur.lastActive = time.Now()
				}
				s.lportfwdMu.Unlock()
				if !ok {
					slog.Warn("lportfwd: queue overflow, closing leg", "agent_id", agentID, "conn", connID)
					s.lportfwdClose(agentID, connID)
					return
				}
			}
			if rerr != nil {
				s.lportfwdClose(agentID, connID)
				return
			}
		}
	}()
}

func (s *Server) lportfwdWrite(agentID string, connID uint64, data []byte) {
	s.lportfwdMu.Lock()
	t, ok := s.lportfwdTargets[lportfwdKey(agentID, connID)]
	s.lportfwdMu.Unlock()
	if !ok || len(data) == 0 {
		return
	}
	_ = agentID // ownership enforced by the key; kept in the signature for log symmetry
	t.tcpConn.SetWriteDeadline(time.Now().Add(lportfwdWriteTimeout))
	if _, err := t.tcpConn.Write(data); err != nil {
		slog.Warn("lportfwd: write to target failed, closing", "conn", connID, "error", err)
		s.lportfwdClose(t.agentID, connID)
		return
	}
	t.tcpConn.SetWriteDeadline(time.Time{})
	s.lportfwdMu.Lock()
	if cur, tracked := s.lportfwdTargets[lportfwdKey(agentID, connID)]; tracked && cur == t {
		cur.lastActive = time.Now()
	}
	s.lportfwdMu.Unlock()
}

func (s *Server) lportfwdClose(agentID string, connID uint64) {
	key := lportfwdKey(agentID, connID)
	s.lportfwdMu.Lock()
	t, ok := s.lportfwdTargets[key]
	if ok {
		delete(s.lportfwdTargets, key)
	}
	s.lportfwdMu.Unlock()
	if !ok {
		return
	}
	_ = t.tcpConn.Close()
	// Tell the agent its local leg should close too (idempotent there).
	// Close frames bypass the queue byte cap, so this notification can't be
	// swallowed by a full backlog (which used to leak both legs).
	s.socksEngine.enqueueLPortFwdFrame(agentID, socksFrame{ConnID: connID, Action: "lportfwd_close"})
	slog.Info("lportfwd: connection closed", "agent_id", agentID, "conn", connID, "target", t.target)
}

// closeAllLPortFwd tears down every tunneled leg. Shutdown path: sockets +
// pump goroutines are released even for agents that never sent close.
// (Close notifications are still queued best-effort; the agent is going
// away with the server anyway.)
func (s *Server) closeAllLPortFwd() {
	s.lportfwdMu.Lock()
	type leg struct {
		agentID string
		connID  uint64
	}
	legs := make([]leg, 0, len(s.lportfwdTargets))
	for key, t := range s.lportfwdTargets {
		legs = append(legs, leg{agentID: t.agentID, connID: connIDFromKey(key)})
	}
	s.lportfwdMu.Unlock()
	for _, l := range legs {
		s.lportfwdClose(l.agentID, l.connID)
	}
}

// cleanupLPortFwdForAgent tears down every tunneled connection belonging to an
// agent that disconnected, so dead sockets don't linger until process exit.
// Declared targets are dropped too — a reconnect must re-declare via a fresh
// lportfwd_start.
func (s *Server) cleanupLPortFwdForAgent(agentID string) {
	s.clearLPortFwdDecl(agentID)
	s.lportfwdMu.Lock()
	var stale []uint64
	for key, t := range s.lportfwdTargets {
		if t.agentID == agentID {
			stale = append(stale, connIDFromKey(key))
		}
	}
	s.lportfwdMu.Unlock()
	for _, connID := range stale {
		s.lportfwdClose(agentID, connID)
	}
}

// lportFwdAllowed reports whether the lportfwd kill switch permits task
// creation. A nil config (some tests) defaults to allowed.
func (s *Server) lportFwdAllowed() bool {
	return s.cfg == nil || s.cfg.Server.LPortFwdEnabled
}

// isSafeLPortFwdHost rejects dial targets that can never be legitimate:
// unspecified (0.0.0.0) and link-local ranges (incl. cloud metadata
// 169.254.169.254). Loopback stays allowed ONLY because every accepted value
// must have been byte-for-byte declared by an operator via lportfwd_start —
// the undeclared-arbitrary-dial hole is closed by that match requirement.
func isSafeLPortFwdHost(host string) bool {
	h := strings.ToLower(strings.Trim(host, "[]"))
	ip := net.ParseIP(h)
	if ip == nil {
		return h != "" // hostname: fine, exact-match enforced at connect
	}
	return !ip.IsUnspecified() && !ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast()
}

// parseLPortFwdCommand parses the "lport|host:port" task command format.
func parseLPortFwdCommand(cmd string) (lport int, target string, ok bool) {
	parts := strings.SplitN(cmd, "|", 2)
	if len(parts) != 2 {
		return 0, "", false
	}
	lp, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || lp < 1 || lp > 65535 {
		return 0, "", false
	}
	target = strings.TrimSpace(parts[1])
	host, portStr, err := net.SplitHostPort(target)
	if err != nil || !isSafeLPortFwdHost(host) {
		return 0, "", false
	}
	if p, err := strconv.Atoi(portStr); err != nil || p < 1 || p > 65535 {
		return 0, "", false
	}
	return lp, target, true
}

// registerLPortFwdDecl records a target declared via an lportfwd_start task.
func (s *Server) registerLPortFwdDecl(agentID, target string) {
	s.lportfwdMu.Lock()
	if s.lportfwdDeclared == nil {
		s.lportfwdDeclared = make(map[string]map[string]bool)
	}
	if s.lportfwdDeclared[agentID] == nil {
		s.lportfwdDeclared[agentID] = make(map[string]bool)
	}
	s.lportfwdDeclared[agentID][target] = true
	s.lportfwdMu.Unlock()
}

// clearLPortFwdDecl drops every declared target for an agent (stop task or
// disconnect).
func (s *Server) clearLPortFwdDecl(agentID string) {
	s.lportfwdMu.Lock()
	delete(s.lportfwdDeclared, agentID)
	s.lportfwdMu.Unlock()
}

// lportfwdTargetAllowed reports whether an agent-supplied connect target was
// operator-declared and passes safety checks. This closes the arbitrary-dial
// SSRF where any implant frame made the teamserver connect anywhere.
func (s *Server) lportfwdTargetAllowed(agentID, target string) bool {
	if !s.lportFwdAllowed() {
		return false
	}
	s.lportfwdMu.Lock()
	var declared bool
	if s.lportfwdDeclared != nil {
		declared = s.lportfwdDeclared[agentID][target]
	}
	s.lportfwdMu.Unlock()
	if !declared {
		return false
	}
	host, _, err := net.SplitHostPort(target)
	if err != nil || !isSafeLPortFwdHost(host) {
		return false
	}
	return true
}
