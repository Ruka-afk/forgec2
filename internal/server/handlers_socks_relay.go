package server

import (
	"context"
	"encoding/binary"
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
	connections map[uint64]*socksRelayConn    // connID → conn
	nextConnID  uint64

	controlFrames   map[string][]socksFrame
	controlFramesMu sync.Mutex
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
	udpConn    *net.UDPConn
	udpClient  *net.UDPAddr
	isUDP      bool
	agentID    string
	destAddr   string
	mu         sync.Mutex
	outbound   [][]byte
	closed     bool
	lastActive time.Time
}

func newSocksRelayEngine() *socksRelayEngine {
	return &socksRelayEngine{
		sessions:      make(map[string]*socksRelaySession),
		connections:   make(map[uint64]*socksRelayConn),
		controlFrames: make(map[string][]socksFrame),
	}
}

// ─── Lifecycle ───────────────────────────────────────────────────────────────

func (e *socksRelayEngine) startSession(s *Server, agentID string, port int) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if sess, ok := e.sessions[agentID]; ok {
		return sess.port, nil // already running
	}

	listenHost := "127.0.0.1"
	if s.cfg != nil && s.cfg.Server.SocksListenHost != "" {
		listenHost = s.cfg.Server.SocksListenHost
	}
	addr := fmt.Sprintf("%s:%d", listenHost, port)
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
	if err := s.db.Create(&session).Error; err != nil {
		slog.Error("Failed to persist SOCKS session", "agent_id", agentID, "err", err)
	}
	sess.dbID = session.ID

	e.sessions[agentID] = sess

	slog.Info("SOCKS relay started", "agent_id", agentID, "port", actualPort)
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

	if err := s.db.Model(&db.SocksSession{}).Where("id = ?", sess.dbID).Updates(map[string]interface{}{
		"status":     "stopped",
		"updated_at": time.Now(),
	}).Error; err != nil {
		slog.Error("Failed to update SOCKS session on stop", "session_id", sess.dbID, "err", err)
	}

	slog.Info("SOCKS relay stopped", "agent_id", agentID, "port", sess.port)
	return nil
}

func (e *socksRelayEngine) getSession(agentID string) *socksRelaySession {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.sessions[agentID]
}

// ─── Accept Loop (Operator → Server) ─────────────────────────────────────────

func (e *socksRelayEngine) acceptLoop(s *Server, sess *socksRelaySession) {
	slog.Info("SOCKS relay accept loop started", "agent_id", sess.agentID, "port", sess.port)
	for {
		conn, err := sess.listener.Accept()
		if err != nil {
			select {
			case <-sess.ctx.Done():
				return
			default:
			}
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				time.Sleep(50 * time.Millisecond)
				continue
			}
			slog.Error("SOCKS relay accept error", "agent_id", sess.agentID, "err", err)
			return
		}
		go e.handleOperatorConn(s, sess, conn)
	}
}

// handleOperatorConn performs the SOCKS5 handshake with the operator's client
// locally, then relays the connection through the beacon tunnel to the agent.
func (e *socksRelayEngine) handleOperatorConn(s *Server, sess *socksRelaySession, conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(SOCKSHandshakeTimeout)) // handshake timeout

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
	if reqHeader[0] != 0x05 || (reqHeader[1] != 0x01 && reqHeader[1] != 0x03) {
		conn.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	udpAssociate := reqHeader[1] == 0x03

	var destAddr string
	readAddr := func(b []byte) error {
		_, err := io.ReadFull(conn, b)
		return err
	}
	switch reqHeader[3] {
	case 0x01: // IPv4
		ip := make([]byte, 4)
		portb := make([]byte, 2)
		if err := readAddr(ip); err != nil {
			return
		}
		if err := readAddr(portb); err != nil {
			return
		}
		destAddr = fmt.Sprintf("%d.%d.%d.%d:%d", ip[0], ip[1], ip[2], ip[3], int(portb[0])<<8|int(portb[1]))
	case 0x03: // Domain
		lb := make([]byte, 1)
		if err := readAddr(lb); err != nil {
			return
		}
		dom := make([]byte, int(lb[0]))
		if err := readAddr(dom); err != nil {
			return
		}
		portb := make([]byte, 2)
		if err := readAddr(portb); err != nil {
			return
		}
		destAddr = fmt.Sprintf("%s:%d", string(dom), int(portb[0])<<8|int(portb[1]))
	case 0x04: // IPv6
		ip := make([]byte, 16)
		portb := make([]byte, 2)
		if err := readAddr(ip); err != nil {
			return
		}
		if err := readAddr(portb); err != nil {
			return
		}
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
		conn.Write([]byte{0x05, 0x02, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		slog.Warn("SOCKS relay: max connections reached", "agent_id", sess.agentID, "limit", SocksMaxConns)
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
		isUDP:      udpAssociate,
	}
	e.connections[connID] = rc
	e.mu.Unlock()

	sess.mu.Lock()
	sess.connCount++
	countCopy := sess.connCount
	sess.mu.Unlock()

	// Update database atomically
	if err := s.db.Model(&db.SocksSession{}).Where("id = ?", sess.dbID).Updates(map[string]interface{}{
		"conn_count": countCopy,
		"updated_at": time.Now(),
	}).Error; err != nil {
		slog.Error("Failed to update SOCKS conn_count", "session_id", sess.dbID, "err", err)
	}

	if udpAssociate {
		e.startUDPAssociate(s, sess, rc, conn)
		return
	}

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

	slog.Info("SOCKS relay: operator connected", "agent_id", sess.agentID, "conn_id", connID, "dest", destAddr)

	// Read from operator → buffer for agent
	buf := make([]byte, SocksMaxFrameSize)
	var bytesAccum int64
	var pktCount int
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
			// Accumulate stats, flush to DB every 100 packets
			bytesAccum += int64(n)
			pktCount++
			if pktCount >= SocksStatsFlushEvery {
				s.db.Model(&db.SocksSession{}).Where("id = ?", sess.dbID).
					UpdateColumn("bytes_in", gorm.Expr("bytes_in + ?", bytesAccum))
				bytesAccum = 0
				pktCount = 0
			}
			rc.mu.Lock()
			rc.lastActive = time.Now()
			rc.mu.Unlock()
		}
		if err != nil {
			break
		}
	}

	// Flush remaining accumulated bytes on connection close
	if bytesAccum > 0 {
		s.db.Model(&db.SocksSession{}).Where("id = ?", sess.dbID).
			UpdateColumn("bytes_in", gorm.Expr("bytes_in + ?", bytesAccum))
	}

	// Connection closed
	e.mu.Lock()
	delete(e.connections, connID)
	e.mu.Unlock()
	e.enqueueFrame(sess.agentID, socksFrame{ConnID: connID, Action: "close"})
	slog.Info("SOCKS relay: operator disconnected", "conn_id", connID)
}

// ─── Frame Queue ─────────────────────────────────────────────────────────────

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
			if len(conn.outbound) < 500 {
				conn.outbound = append(conn.outbound, f.Data)
			}
			conn.mu.Unlock()
		}
		return
	}
	// Control frame — bounds protect (max 200 per agent)
	e.controlFramesMu.Lock()
	if len(e.controlFrames[agentID]) < 200 {
		e.controlFrames[agentID] = append(e.controlFrames[agentID], f)
	}
	e.controlFramesMu.Unlock()
}

// collectPendingFrames gathers all pending frames for an agent.
func (e *socksRelayEngine) collectPendingFrames(agentID string) []socksFrame {
	var frames []socksFrame

	// 1. Control frames first (connect/close must arrive before data)
	e.controlFramesMu.Lock()
	if cf, ok := e.controlFrames[agentID]; ok && len(cf) > 0 {
		frames = append(frames, cf...)
		e.controlFrames[agentID] = nil
	}
	e.controlFramesMu.Unlock()

	// 2. Data frames from connections — minimize lock hold time
	type connDrain struct {
		connID   uint64
		outbound [][]byte
	}
	var drained []connDrain

	e.mu.Lock()
	for _, conn := range e.connections {
		if conn.agentID != agentID {
			continue
		}
		conn.mu.Lock()
		if len(conn.outbound) > 0 {
			// Snapshot and drain under conn lock
			snapshot := make([][]byte, len(conn.outbound))
			copy(snapshot, conn.outbound)
			conn.outbound = conn.outbound[:0]
			conn.mu.Unlock()
			drained = append(drained, connDrain{connID: conn.connID, outbound: snapshot})
		} else {
			conn.mu.Unlock()
		}
	}
	e.mu.Unlock()

	// Process drained data outside e.mu
	for _, d := range drained {
		var merged []byte
		for _, chunk := range d.outbound {
			merged = append(merged, chunk...)
		}
		for len(merged) > 0 {
			sz := len(merged)
			if sz > SocksMaxFrameSize {
				sz = SocksMaxFrameSize
			}
			frames = append(frames, socksFrame{
				ConnID: d.connID,
				Action: "data",
				Data:   merged[:sz],
			})
			merged = merged[sz:]
		}
	}

	return frames
}

// ─── Process Agent Data (from beacon request) ───────────────────────────────

func (e *socksRelayEngine) processAgentData(s *Server, agentID string, frames []socksFrame) {
	for _, f := range frames {
		// Ownership check (P1): ConnID is agent-controlled and the map is
		// global. Without this, implant B could write data into / close
		// implant A's live operator socket by replaying A's conn id.
		e.mu.Lock()
		conn, owned := e.connections[f.ConnID]
		if owned && conn.agentID != agentID {
			e.mu.Unlock()
			slog.Warn("SOCKS relay: dropped frame for foreign conn id",
				"agent_id", agentID, "conn", f.ConnID, "action", f.Action)
			continue
		}
		e.mu.Unlock()
		switch f.Action {
		case "data":
			if owned && len(f.Data) > 0 {
				conn.mu.Lock()
				conn.tcpConn.SetWriteDeadline(time.Now().Add(SOCKSRelayWriteTimeout))
				if _, err := conn.tcpConn.Write(f.Data); err != nil {
					slog.Warn("SOCKS relay: write to operator failed, closing conn",
						"conn", f.ConnID, "error", err)
					conn.tcpConn.Close()
					conn.mu.Unlock()
					continue
				}
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
		case "udp_data":
			if !owned || !conn.isUDP || conn.udpConn == nil || conn.udpClient == nil {
				continue
			}
			_, port, payload, err := decodeSocksUDPFrame(f.Data)
			if err != nil {
				continue
			}
			_ = port
			conn.mu.Lock()
			_, _ = conn.udpConn.WriteToUDP(payload, conn.udpClient)
			conn.lastActive = time.Now()
			conn.mu.Unlock()
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
		case "tun_up", "tun_data", "tun_down":
			if s != nil && s.tunEngine != nil {
				s.tunEngine.handleAgentFrame(agentID, f.Action, f.Data)
			}
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
		if c.udpConn != nil {
			c.udpConn.Close()
		}
	}
}

func (e *socksRelayEngine) startUDPAssociate(s *Server, sess *socksRelaySession, rc *socksRelayConn, ctrl net.Conn) {
	udpConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		ctrl.Write([]byte{0x05, 0x01, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	port := udpConn.LocalAddr().(*net.UDPAddr).Port
	rc.mu.Lock()
	rc.udpConn = udpConn
	rc.mu.Unlock()

	e.enqueueFrame(sess.agentID, socksFrame{ConnID: rc.connID, Action: "udp_associate"})

	// Bind IPv4 0.0.0.0:port
	reply := []byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, byte(port >> 8), byte(port)}
	_, _ = ctrl.Write(reply)
	_ = ctrl.SetDeadline(time.Time{})

	go func() {
		buf := make([]byte, 65535)
		for {
			n, src, err := udpConn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			rc.mu.Lock()
			if rc.udpClient == nil {
				rc.udpClient = src
			}
			client := rc.udpClient
			rc.lastActive = time.Now()
			rc.mu.Unlock()
			if client == nil || src.Port != client.Port || !src.IP.Equal(client.IP) {
				// First datagram sets the client; ignore others.
				if client != nil {
					continue
				}
			}
			// SOCKS UDP header: RSV RSV FRAG ATYP DST.ADDR DST.PORT DATA
			if n < 10 || buf[0] != 0 || buf[1] != 0 {
				continue
			}
			dst, payload, err := parseOperatorUDPHeader(buf[:n])
			if err != nil {
				continue
			}
			host, portStr, _ := net.SplitHostPort(dst)
			p, _ := strconv.Atoi(portStr)
			e.enqueueFrame(sess.agentID, socksFrame{
				ConnID: rc.connID,
				Action: "udp_data",
				Data:   encodeSocksUDPFrame(host, p, payload),
			})
		}
	}()

	// Hold the TCP control connection until the client disconnects.
	tmp := make([]byte, 1)
	_, _ = ctrl.Read(tmp)
	udpConn.Close()
	e.enqueueFrame(sess.agentID, socksFrame{ConnID: rc.connID, Action: "close"})
}

func encodeSocksUDPFrame(addr string, port int, payload []byte) []byte {
	ab := []byte(addr)
	out := make([]byte, 2+len(ab)+2+len(payload))
	binary.BigEndian.PutUint16(out[0:2], uint16(len(ab)))
	copy(out[2:], ab)
	binary.BigEndian.PutUint16(out[2+len(ab):], uint16(port))
	copy(out[4+len(ab):], payload)
	return out
}

func decodeSocksUDPFrame(data []byte) (addr string, port int, payload []byte, err error) {
	if len(data) < 4 {
		return "", 0, nil, fmt.Errorf("short")
	}
	n := int(binary.BigEndian.Uint16(data[0:2]))
	if len(data) < 2+n+2 {
		return "", 0, nil, fmt.Errorf("truncated")
	}
	addr = string(data[2 : 2+n])
	port = int(binary.BigEndian.Uint16(data[2+n : 4+n]))
	payload = data[4+n:]
	return
}

func parseOperatorUDPHeader(b []byte) (dst string, payload []byte, err error) {
	if len(b) < 7 {
		return "", nil, fmt.Errorf("short")
	}
	atyp := b[3]
	switch atyp {
	case 0x01:
		if len(b) < 10 {
			return "", nil, fmt.Errorf("short ipv4")
		}
		ip := net.IP(b[4:8])
		port := int(b[8])<<8 | int(b[9])
		return net.JoinHostPort(ip.String(), strconv.Itoa(port)), b[10:], nil
	case 0x03:
		l := int(b[4])
		if len(b) < 5+l+2 {
			return "", nil, fmt.Errorf("short domain")
		}
		host := string(b[5 : 5+l])
		port := int(b[5+l])<<8 | int(b[6+l])
		return net.JoinHostPort(host, strconv.Itoa(port)), b[7+l:], nil
	case 0x04:
		if len(b) < 22 {
			return "", nil, fmt.Errorf("short ipv6")
		}
		ip := net.IP(b[4:20])
		port := int(b[20])<<8 | int(b[21])
		return net.JoinHostPort(ip.String(), strconv.Itoa(port)), b[22:], nil
	default:
		return "", nil, fmt.Errorf("bad atyp")
	}
}

func listenHostMsg(s *Server) string {
	if s != nil && s.cfg != nil && s.cfg.Server.SocksListenHost != "" {
		return s.cfg.Server.SocksListenHost
	}
	return "127.0.0.1"
}

// ─── HTTP Handlers ───────────────────────────────────────────────────────────

func (s *Server) handleStartSocksRelay(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
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
		respondError(c, http.StatusBadRequest, "invalid port")
		return
	}

	actualPort, err := s.socksEngine.startSession(s, id, port)
	if err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "SOCKS relay"))
		return
	}

	s.LogAuditRecord(c, "socks_relay_start", "agent", id, fmt.Sprintf("port %d", actualPort), true, nil)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"port":    actualPort,
		"message": fmt.Sprintf("SOCKS5 relay listening on %s:%d → Agent %s", listenHostMsg(s), actualPort, id),
	})
}

func (s *Server) handleStopSocksRelay(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	if err := s.socksEngine.stopSession(s, id); err != nil {
		respondErrorSafe(c, http.StatusBadRequest, err, "")
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
		"active":       true,
		"port":         sess.port,
		"conn_count":   cc,
		"active_conns": activeConns,
		"bytes_in":     dbSession.BytesIn,
		"bytes_out":    dbSession.BytesOut,
		"created_at":   dbSession.CreatedAt,
	})
}

func (s *Server) handleGetSocksSessions(c *gin.Context) {
	var sessions []db.SocksSession
	q := s.db.Order("created_at desc").Limit(50)
	if agentID := c.Query("agent_id"); agentID != "" {
		q = q.Where("agent_id = ?", agentID)
	}
	if err := q.Limit(50).Find(&sessions).Error; err != nil {
		slog.Error("Failed to query SOCKS sessions", "err", err)
	}

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
	frames := s.socksEngine.collectPendingFrames(agentID)
	if s.tunEngine != nil {
		frames = append(frames, s.tunEngine.drain(agentID)...)
	}
	return frames
}

// hasActiveSocks checks if an agent has an active SOCKS relay session.
func (s *Server) hasActiveSocks(agentID string) bool {
	return s.socksEngine.getSession(agentID) != nil
}

// cleanupStaleSocks runs periodically to clean dead connections.
func (s *Server) cleanupStaleSocks() {
	ticker := time.NewTicker(SocksCleanupTimeout)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if s.socksEngine != nil {
				s.socksEngine.cleanup()
			}
		case <-s.ctx.Done():
			return
		}
	}
}
