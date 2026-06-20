package server

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ─── SOCKS Relay Frame Protocol ──────────────────────────────────────────────
// Server → Agent (carried in beacon response "socks_frames")
// Agent → Server (carried in beacon request  "socks_data")

type socksFrame struct {
	ConnID uint64 `json:"conn_id"`
	Action string `json:"action"` // connect, connected, data, close
	Data   []byte `json:"data,omitempty"`
}

// ─── In-memory Relay Engine ──────────────────────────────────────────────────

type socksRelayEngine struct {
	mu          sync.Mutex
	sessions    map[string]*socksRelaySession // agentID → session
	connections map[uint64]*socksRelayConn   // connID → conn
	nextConnID  uint64
}

type socksRelaySession struct {
	agentID   string
	port      int
	listener  net.Listener
	ctx       context.Context
	cancel    context.CancelFunc
	dbID      uint
	connCount int
	mu        sync.Mutex
}

type socksRelayConn struct {
	connID     uint64
	tcpConn    net.Conn
	agentID    string
	destAddr   string
	mu         sync.Mutex
	outbound   [][]byte
	closed     bool
	lastActive time.Time
}

func newSocksRelayEngine() *socksRelayEngine {
	return &socksRelayEngine{
		sessions:    make(map[string]*socksRelaySession),
		connections: make(map[uint64]*socksRelayConn),
	}
}

// ─── Lifecycle ───────────────────────────────────────────────────────────────

func (e *socksRelayEngine) startSession(s *Server, agentID string, port int) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if sess, ok := e.sessions[agentID]; ok {
		return sess.port, nil // already running
	}

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return 0, fmt.Errorf("SOCKS relay listen %s: %w", addr, err)
	}
	actualPort := ln.Addr().(*net.TCPAddr).Port

	ctx, cancel := context.WithCancel(context.Background())
	sess := &socksRelaySession{
		agentID:  agentID,
		port:     actualPort,
		listener: ln,
		ctx:      ctx,
		cancel:   cancel,
	}

	// Persist
	session := db.SocksSession{
		AgentID:    agentID,
		ListenPort: actualPort,
		Status:     "active",
	}
	s.db.Create(&session)
	sess.dbID = session.ID

	e.sessions[agentID] = sess

	slog.Info("SOCKS relay started", "agent", agentID, "port", actualPort)
	go e.acceptLoop(s, sess)

	return actualPort, nil
}

func (e *socksRelayEngine) stopSession(s *Server, agentID string) error {
	e.mu.Lock()
	sess, ok := e.sessions[agentID]
	if !ok {
		e.mu.Unlock()
		return fmt.Errorf("no active SOCKS relay for agent %s", agentID)
	}
	delete(e.sessions, agentID)
	e.mu.Unlock()

	sess.cancel()
	sess.listener.Close()

	// Close all active connections for this agent
	e.mu.Lock()
	for id, conn := range e.connections {
		if conn.agentID == agentID {
			conn.close()
			delete(e.connections, id)
		}
	}
	e.mu.Unlock()

	s.db.Model(&db.SocksSession{}).Where("id = ?", sess.dbID).Updates(map[string]interface{}{
		"status":     "stopped",
		"updated_at": time.Now(),
	})

	slog.Info("SOCKS relay stopped", "agent", agentID, "port", sess.port)
	return nil
}

func (e *socksRelayEngine) getSession(agentID string) *socksRelaySession {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sessions[agentID]
}

// ─── Accept Loop (Operator → Server) ─────────────────────────────────────────

func (e *socksRelayEngine) acceptLoop(s *Server, sess *socksRelaySession) {
	slog.Info("SOCKS relay accept loop started", "agent", sess.agentID, "port", sess.port)
	for {
		conn, err := sess.listener.Accept()
		if err != nil {
			select {
			case <-sess.ctx.Done():
				return
			default:
				continue
			}
		}
		go e.handleOperatorConn(s, sess, conn)
	}
}

// handleOperatorConn performs the SOCKS5 handshake with the operator's client
// locally, then relays the connection through the beacon tunnel to the agent.
func (e *socksRelayEngine) handleOperatorConn(s *Server, sess *socksRelaySession, conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(30 * time.Second)) // handshake timeout

	// ── SOCKS5 Greeting ──
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return
	}
	if header[0] != 0x05 {
		return
	}
	nmethods := int(header[1])
	methods := make([]byte, nmethods)
	if _, err := io.ReadFull(conn, methods); err != nil {
		return
	}
	// No auth required
	conn.Write([]byte{0x05, 0x00})

	// ── SOCKS5 Request ──
	reqHeader := make([]byte, 4)
	if _, err := io.ReadFull(conn, reqHeader); err != nil {
		return
	}
	if reqHeader[0] != 0x05 || reqHeader[1] != 0x01 { // CONNECT only
		conn.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}

	var destAddr string
	switch reqHeader[3] {
	case 0x01: // IPv4
		ip := make([]byte, 4)
		portb := make([]byte, 2)
		io.ReadFull(conn, ip)
		io.ReadFull(conn, portb)
		destAddr = fmt.Sprintf("%d.%d.%d.%d:%d", ip[0], ip[1], ip[2], ip[3], int(portb[0])<<8|int(portb[1]))
	case 0x03: // Domain
		lb := make([]byte, 1)
		io.ReadFull(conn, lb)
		dom := make([]byte, int(lb[0]))
		io.ReadFull(conn, dom)
		portb := make([]byte, 2)
		io.ReadFull(conn, portb)
		destAddr = fmt.Sprintf("%s:%d", string(dom), int(portb[0])<<8|int(portb[1]))
	case 0x04: // IPv6
		ip := make([]byte, 16)
		portb := make([]byte, 2)
		io.ReadFull(conn, ip)
		io.ReadFull(conn, portb)
		destAddr = fmt.Sprintf("[%s]:%d", net.IP(ip).String(), int(portb[0])<<8|int(portb[1]))
	default:
		conn.Write([]byte{0x05, 0x08, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}

	// Check connection limit per session
	var activeConns int
	e.mu.Lock()
	for _, c := range e.connections {
		if c.agentID == sess.agentID {
			activeConns++
		}
	}
	if activeConns >= SocksMaxConns {
		e.mu.Unlock()
		// SOCKS5 reply: Connection not allowed by ruleset
		conn.Write([]byte{0x05, 0x02, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		slog.Warn("SOCKS relay: max connections reached", "agent", sess.agentID, "limit", SocksMaxConns)
		return
	}

	// Allocate connection ID
	e.nextConnID++
	connID := e.nextConnID
	rc := &socksRelayConn{
		connID:     connID,
		tcpConn:    conn,
		agentID:    sess.agentID,
		destAddr:   destAddr,
		lastActive: time.Now(),
	}
	e.connections[connID] = rc
	e.mu.Unlock()

	sess.mu.Lock()
	sess.connCount++
	countCopy := sess.connCount
	sess.mu.Unlock()

	// Update database atomically
	s.db.Model(&db.SocksSession{}).Where("id = ?", sess.dbID).Updates(map[string]interface{}{
		"conn_count": countCopy,
		"updated_at": time.Now(),
	})

	// Send connect frame to agent
	e.enqueueFrame(sess.agentID, socksFrame{
		ConnID: connID,
		Action: "connect",
		Data:   []byte(destAddr),
	})

	// SOCKS5 success reply (bound address 0.0.0.0:0)
	conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})

	// Clear deadline, now we relay
	conn.SetDeadline(time.Time{})

	slog.Info("SOCKS relay: operator connected", "agent", sess.agentID, "conn_id", connID, "dest", destAddr)

	// Read from operator → buffer for agent
	buf := make([]byte, SocksMaxFrameSize)
	for {
		n, err := conn.Read(buf)
		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])
			e.enqueueFrame(sess.agentID, socksFrame{
				ConnID: connID,
				Action: "data",
				Data:   data,
			})
			// Update stats
			s.db.Model(&db.SocksSession{}).Where("id = ?", sess.dbID).
				UpdateColumn("bytes_in", gorm.Expr("bytes_in + ?", int64(n)))
			rc.mu.Lock()
			rc.lastActive = time.Now()
			rc.mu.Unlock()
		}
		if err != nil {
			break
		}
	}

	// Connection closed
	e.mu.Lock()
	delete(e.connections, connID)
	e.mu.Unlock()
	e.enqueueFrame(sess.agentID, socksFrame{ConnID: connID, Action: "close"})
	slog.Info("SOCKS relay: operator disconnected", "conn_id", connID)
}

// ─── Frame Queue ─────────────────────────────────────────────────────────────

// controlFrames stores control frames (connect/close) per agent session.
var (
	controlFramesMu sync.Mutex
	controlFrames   = make(map[string][]socksFrame)
)

// enqueueFrame adds a frame to the relay queue.
// "data" frames go to the connection's outbound buffer.
// Control frames (connect/close) go to the session-level control queue.
func (e *socksRelayEngine) enqueueFrame(agentID string, f socksFrame) {
	if f.Action == "data" {
		e.mu.Lock()
		conn, ok := e.connections[f.ConnID]
		e.mu.Unlock()
		if ok {
			conn.mu.Lock()
			conn.outbound = append(conn.outbound, f.Data)
			conn.mu.Unlock()
		}
		return
	}
	// Control frame
	controlFramesMu.Lock()
	controlFrames[agentID] = append(controlFrames[agentID], f)
	controlFramesMu.Unlock()
}

// collectPendingFrames gathers all pending frames for an agent.
func (e *socksRelayEngine) collectPendingFrames(agentID string) []socksFrame {
	var frames []socksFrame

	// 1. Control frames first (connect/close must arrive before data)
	controlFramesMu.Lock()
	if cf, ok := controlFrames[agentID]; ok && len(cf) > 0 {
		frames = append(frames, cf...)
		controlFrames[agentID] = nil
	}
	controlFramesMu.Unlock()

	// 2. Data frames from connections
	e.mu.Lock()
	for _, conn := range e.connections {
		if conn.agentID != agentID {
			continue
		}
		conn.mu.Lock()
		if len(conn.outbound) > 0 {
			// Merge buffered chunks into a single payload
			var merged []byte
			for _, chunk := range conn.outbound {
				merged = append(merged, chunk...)
			}
			conn.outbound = conn.outbound[:0] // drain
			conn.mu.Unlock()

			// Split into max-size frames
			for len(merged) > 0 {
				sz := len(merged)
				if sz > SocksMaxFrameSize {
					sz = SocksMaxFrameSize
				}
				frames = append(frames, socksFrame{
					ConnID: conn.connID,
					Action: "data",
					Data:   merged[:sz],
				})
				merged = merged[sz:]
			}
		} else {
			conn.mu.Unlock()
		}
	}
	e.mu.Unlock()

	return frames
}

// ─── Process Agent Data (from beacon request) ───────────────────────────────

func (e *socksRelayEngine) processAgentData(s *Server, agentID string, frames []socksFrame) {
	for _, f := range frames {
		switch f.Action {
		case "data":
			e.mu.Lock()
			conn, ok := e.connections[f.ConnID]
			e.mu.Unlock()
			if ok && len(f.Data) > 0 {
				conn.mu.Lock()
				conn.tcpConn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				conn.tcpConn.Write(f.Data)
				conn.tcpConn.SetWriteDeadline(time.Time{})
				conn.lastActive = time.Now()
				conn.mu.Unlock()
				// Update stats
				sess := e.getSession(agentID)
				if sess != nil {
					s.db.Model(&db.SocksSession{}).Where("id = ?", sess.dbID).
						UpdateColumn("bytes_out", gorm.Expr("bytes_out + ?", int64(len(f.Data))))
				}
			}
		case "connected":
			slog.Info("SOCKS relay: agent connected to target", "conn_id", f.ConnID)
		case "close":
			e.mu.Lock()
			conn, ok := e.connections[f.ConnID]
			if ok {
				delete(e.connections, f.ConnID)
			}
			e.mu.Unlock()
			if ok {
				conn.close()
			}
			slog.Info("SOCKS relay: agent closed connection", "conn_id", f.ConnID)
		}
	}
}

// cleanup removes stale connections.
// Collects stale conns under lock, then releases lock before closing TCP to avoid contention.
func (e *socksRelayEngine) cleanup() {
	e.mu.Lock()
	cutoff := time.Now().Add(-SocksCleanupTimeout)
	var stale []*socksRelayConn
	for id, conn := range e.connections {
		conn.mu.Lock()
		if conn.lastActive.Before(cutoff) {
			conn.closed = true
			stale = append(stale, conn)
			delete(e.connections, id)
			slog.Info("SOCKS relay: stale connection cleaned", "conn_id", id)
		}
		conn.mu.Unlock()
	}
	e.mu.Unlock()

	// Close TCP outside the engine lock to avoid contention with handleOperatorConn
	for _, conn := range stale {
		conn.tcpConn.Close()
	}
}

func (c *socksRelayConn) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.closed = true
		c.tcpConn.Close()
	}
}

// ─── HTTP Handlers ───────────────────────────────────────────────────────────

func (s *Server) handleStartSocksRelay(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	portStr := c.PostForm("port")
	if portStr == "" {
		portStr = "1080"
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid port"})
		return
	}

	actualPort, err := s.socksEngine.startSession(s, id, port)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	s.LogAuditRecord(c, "socks_relay_start", "agent", id, fmt.Sprintf("port %d", actualPort), true, nil)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"port":    actualPort,
		"message": fmt.Sprintf("SOCKS5 relay listening on 0.0.0.0:%d → Agent %s", actualPort, id),
	})
}

func (s *Server) handleStopSocksRelay(c *gin.Context) {
	id := c.Param("id")
	if err := s.socksEngine.stopSession(s, id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	s.LogAuditRecord(c, "socks_relay_stop", "agent", id, "", true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "SOCKS relay stopped"})
}

func (s *Server) handleSocksRelayStatus(c *gin.Context) {
	id := c.Param("id")
	sess := s.socksEngine.getSession(id)
	if sess == nil {
		c.JSON(http.StatusOK, gin.H{"active": false})
		return
	}

	sess.mu.Lock()
	cc := sess.connCount
	sess.mu.Unlock()

	var activeConns int
	s.socksEngine.mu.Lock()
	for _, conn := range s.socksEngine.connections {
		if conn.agentID == id {
			activeConns++
		}
	}
	s.socksEngine.mu.Unlock()

	var dbSession db.SocksSession
	s.db.Where("id = ?", sess.dbID).First(&dbSession)

	c.JSON(http.StatusOK, gin.H{
		"active":        true,
		"port":          sess.port,
		"conn_count":    cc,
		"active_conns":  activeConns,
		"bytes_in":      dbSession.BytesIn,
		"bytes_out":     dbSession.BytesOut,
		"created_at":    dbSession.CreatedAt,
	})
}

func (s *Server) handleGetSocksSessions(c *gin.Context) {
	var sessions []db.SocksSession
	q := s.db.Order("created_at desc").Limit(50)
	if agentID := c.Query("agent_id"); agentID != "" {
		q = q.Where("agent_id = ?", agentID)
	}
	q.Find(&sessions)

	// Enrich with live status
	type enrichedSession struct {
		db.SocksSession
		Active     bool `json:"active"`
		ActiveConn int  `json:"active_conn"`
	}

	var result []enrichedSession
	for _, sess := range sessions {
		es := enrichedSession{SocksSession: sess}
		live := s.socksEngine.getSession(sess.AgentID)
		if live != nil && live.dbID == sess.ID {
			es.Active = true
			s.socksEngine.mu.Lock()
			for _, conn := range s.socksEngine.connections {
				if conn.agentID == sess.AgentID {
					es.ActiveConn++
				}
			}
			s.socksEngine.mu.Unlock()
		}
		result = append(result, es)
	}

	c.JSON(http.StatusOK, gin.H{"sessions": result})
}

// processAgentSocksData is called from processBeacon to handle relay data from agent.
func (s *Server) processAgentSocksData(agentID string, frames []socksFrame) {
	if len(frames) == 0 {
		return
	}
	s.socksEngine.processAgentData(s, agentID, frames)
}

// collectSocksFrames gathers pending frames for an agent (called from processBeacon).
func (s *Server) collectSocksFrames(agentID string) []socksFrame {
	return s.socksEngine.collectPendingFrames(agentID)
}

// hasActiveSocks checks if an agent has an active SOCKS relay session.
func (s *Server) hasActiveSocks(agentID string) bool {
	return s.socksEngine.getSession(agentID) != nil
}

// cleanupStaleSocks runs periodically to clean dead connections.
func (s *Server) cleanupStaleSocks() {
	ticker := time.NewTicker(SocksCleanupTimeout)
	defer ticker.Stop()
	for range ticker.C {
		s.socksEngine.cleanup()
	}
}
