package server

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/forgec2/forgec2/internal/server/middleware"
	"github.com/forgec2/forgec2/internal/util"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func gzipCompress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write(data); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type batchedMessage struct {
	Messages []json.RawMessage `json:"messages"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  WSReadBufSize,
	WriteBufferSize: WSWriteBufSize,
	// Agent beacon WebSocket — agents are non-browser clients (no Origin header).
	// Accept empty origin unconditionally; if Origin IS present (browser), restrict.
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		u, err := url.Parse(origin)
		if err != nil {
			return false
		}
		originHost := u.Hostname()
		return originHost == "localhost" || originHost == "127.0.0.1" || originHost == "::1"
	},
}

// WebSocketBeacon represents an active WebSocket beacon connection
type WebSocketBeacon struct {
	Conn       *websocket.Conn
	AgentID    string
	// lastSeenNano is atomic: written from the read pump's pong handler,
	// read by duplicate-connection checks on other goroutines (P3-10).
	lastSeenNano atomic.Int64
	Send       chan []byte
	BatchQueue [][]byte
	BatchMutex sync.Mutex
	BatchTimer *time.Timer
	flush      chan struct{}
	closeOnce  sync.Once
	compress   bool
}

func (b *WebSocketBeacon) touchLastSeen() { b.lastSeenNano.Store(time.Now().UnixNano()) }
func (b *WebSocketBeacon) lastSeenAt() time.Time {
	return time.Unix(0, b.lastSeenNano.Load())
}

// WebSocketHub manages all active WebSocket beacon connections
type WebSocketHub struct {
	beacons map[string]*WebSocketBeacon
	mu      sync.RWMutex
}

func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		beacons: make(map[string]*WebSocketBeacon),
	}
}

func (h *WebSocketHub) Register(agentID string, beacon *WebSocketBeacon) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if existing, ok := h.beacons[agentID]; ok && existing != beacon {
		existing.closeOnce.Do(func() {
			close(existing.Send)
		})
	}
	h.beacons[agentID] = beacon
}

// Unregister removes the entry for agentID and closes its Send channel,
// but ONLY if the current entry is the same connection. This prevents one
// connection from closing another connection's channel (cross-connection DoS).
func (h *WebSocketHub) Unregister(agentID string, beacon *WebSocketBeacon) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if existing, ok := h.beacons[agentID]; ok && existing == beacon {
		beacon.closeOnce.Do(func() {
			close(beacon.Send)
		})
		delete(h.beacons, agentID)
	}
}

// Remove drops the entry for agentID without closing its Send channel.
// Used when a beacon connection migrates to its real agent UUID.
func (h *WebSocketHub) Remove(agentID string, beacon *WebSocketBeacon) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if existing, ok := h.beacons[agentID]; ok && existing == beacon {
		delete(h.beacons, agentID)
	}
}

func (h *WebSocketHub) Get(agentID string) *WebSocketBeacon {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.beacons[agentID]
}

func (h *WebSocketHub) Broadcast(data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, beacon := range h.beacons {
		func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Warn("WebSocket broadcast to closed channel", "agent_id", beacon.AgentID)
				}
			}()
			select {
			case beacon.Send <- data:
			default:
			}
		}()
	}
}

// handleWebSocketBeacon handles WebSocket beacon connections.
// v2: frame-level envelopes are authenticated by decodeBeaconEnvelope, so the
// upgrade itself requires no separate key check.
func (s *Server) handleWebSocketBeacon(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Error("WebSocket upgrade failed", "error", err)
		return
	}

	agentID := c.Query("agent_id")
	if agentID == "" {
		agentID = util.NewString()
	} else if !isValidAgentID(agentID) {
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseProtocolError, "invalid agent_id"))
		conn.Close()
		return
	}

	slog.Info("WebSocket beacon connected", "agent_id", agentID, "remote_addr", conn.RemoteAddr())

	beacon := &WebSocketBeacon{
		Conn:       conn,
		AgentID:    agentID,
		Send:       make(chan []byte, 256),
		BatchQueue: make([][]byte, 0, 16),
		flush:      make(chan struct{}, 1),
		compress:   true,
	}
	beacon.touchLastSeen()

	s.wsHubOnce.Do(func() {
		if s.wsHub == nil {
			s.wsHub = NewWebSocketHub()
		}
	})
	if existing := s.wsHub.Get(agentID); existing != nil && existing != beacon {
		existingLastSeen := existing.lastSeenAt()
		if time.Since(existingLastSeen) < 30*time.Second {
			slog.Warn("WebSocket hub: rejecting duplicate connection (recently active)", "agent_id", agentID)
			conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "agent already connected"))
			conn.Close()
			return
		}
	}
	s.wsHub.Register(agentID, beacon)

	// Start read and write pumps
	s.wg.Add(2)
	go s.wsWritePump(beacon)
	go s.wsReadPump(beacon)
}

func (s *Server) wsWritePump(beacon *WebSocketBeacon) {
	defer s.wg.Done()
	defer func() {
		beacon.BatchMutex.Lock()
		if beacon.BatchTimer != nil {
			beacon.BatchTimer.Stop()
		}
		beacon.BatchMutex.Unlock()
		beacon.Conn.Close()
		s.wsHub.Unregister(beacon.AgentID, beacon)
	}()

	ticker := time.NewTicker(BeaconPingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			beacon.Conn.WriteMessage(websocket.CloseMessage, []byte{})
			return

		case message, ok := <-beacon.Send:
			if !ok {
				beacon.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			s.enqueueBatchMessage(beacon, message)

		case <-beacon.flush:
			s.flushBatchQueue(beacon)

		case <-ticker.C:
			beacon.Conn.SetWriteDeadline(time.Now().Add(BeaconWriteDeadline))
			if err := beacon.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				slog.Error("WebSocket ping error", "agent_id", beacon.AgentID, "error", err)
				return
			}
		}
	}
}

// signalBatchFlush schedules a flush on the write pump goroutine (non-blocking).
func (s *Server) signalBatchFlush(beacon *WebSocketBeacon) {
	select {
	case beacon.flush <- struct{}{}:
	default:
	}
}

func (s *Server) enqueueBatchMessage(beacon *WebSocketBeacon, message []byte) {
	beacon.BatchMutex.Lock()
	beacon.BatchQueue = append(beacon.BatchQueue, message)
	currentLen := len(beacon.BatchQueue)
	beacon.BatchMutex.Unlock()

	if currentLen == 1 {
		beacon.BatchMutex.Lock()
		beacon.BatchTimer = time.AfterFunc(BatchFlushDelay, func() {
			s.signalBatchFlush(beacon)
		})
		beacon.BatchMutex.Unlock()
	} else if currentLen >= BatchFlushThreshold {
		beacon.BatchMutex.Lock()
		if beacon.BatchTimer != nil {
			beacon.BatchTimer.Stop()
		}
		beacon.BatchMutex.Unlock()
		s.signalBatchFlush(beacon)
	}
}

func (s *Server) flushBatchQueue(beacon *WebSocketBeacon) {
	beacon.BatchMutex.Lock()
	if len(beacon.BatchQueue) == 0 {
		beacon.BatchMutex.Unlock()
		return
	}

	var data []byte
	if len(beacon.BatchQueue) == 1 {
		data = beacon.BatchQueue[0]
	} else {
		batched := batchedMessage{
			Messages: make([]json.RawMessage, len(beacon.BatchQueue)),
		}
		for i, msg := range beacon.BatchQueue {
			batched.Messages[i] = json.RawMessage(msg)
		}
		var err error
		data, err = json.Marshal(batched)
		if err != nil {
			slog.Error("WebSocket batch marshal error", "agent_id", beacon.AgentID, "error", err)
			beacon.BatchQueue = beacon.BatchQueue[:0]
			beacon.BatchMutex.Unlock()
			return
		}
	}

	beacon.BatchQueue = beacon.BatchQueue[:0]
	beacon.BatchMutex.Unlock()

	// Keep the uncompressed form for retry: a gzip frame pushed back into the
	// queue would be double-compressed on the next flush.
	raw := data

	if beacon.compress && len(data) > 256 {
		var err error
		data, err = gzipCompress(data)
		if err != nil {
			slog.Warn("WebSocket gzip compress failed, sending uncompressed", "agent_id", beacon.AgentID, "err", err)
		} else {
			if serr := s.sendCompressedMessage(beacon, data); serr != nil {
				s.requeueBatch(beacon, raw)
			}
			return
		}
	}
	if serr := s.sendTextMessage(beacon, data); serr != nil {
		// A transient write failure must not silently drop the batch: the
		// agent would wait forever on results that were never delivered.
		s.requeueBatch(beacon, raw)
	}
}

// requeueBatch pushes an undelivered (uncompressed) batch back to the FRONT of
// the queue so the next flush retries it. The retained backlog is bounded:
// oversize batches are trimmed message-by-message from the front, and once
// individual messages are exhausted a single oversized payload is kept as-is
// (already bounded upstream by the per-result size caps).
func (s *Server) requeueBatch(beacon *WebSocketBeacon, data []byte) {
	const maxRetained = 8 * 1024 * 1024
	beacon.BatchMutex.Lock()
	defer beacon.BatchMutex.Unlock()

	payload := data
	for len(payload) > maxRetained {
		var bm batchedMessage
		if err := json.Unmarshal(payload, &bm); err != nil || len(bm.Messages) <= 1 {
			break // cannot trim further without losing everything
		}
		out, err := json.Marshal(batchedMessage{Messages: bm.Messages[1:]})
		if err != nil {
			break
		}
		payload = out
	}

	queue := append([][]byte{payload}, beacon.BatchQueue...)
	total := 0
	kept := make([][]byte, 0, len(queue))
	for _, b := range queue {
		if total+len(b) > maxRetained {
			slog.Warn("WebSocket requeue dropping backlog over retention cap", "agent_id", beacon.AgentID, "dropped_bytes", total+len(b))
			continue
		}
		total += len(b)
		kept = append(kept, b)
	}
	beacon.BatchQueue = kept
}

func (s *Server) sendTextMessage(beacon *WebSocketBeacon, data []byte) error {
	beacon.Conn.SetWriteDeadline(time.Now().Add(BeaconWriteDeadline))
	w, err := beacon.Conn.NextWriter(websocket.BinaryMessage)
	if err != nil {
		slog.Error("WebSocket write error", "agent_id", beacon.AgentID, "error", err)
		return err
	}
	w.Write(data)
	if err := w.Close(); err != nil {
		slog.Error("WebSocket close writer error", "agent_id", beacon.AgentID, "error", err)
		return err
	}
	return nil
}

func (s *Server) sendCompressedMessage(beacon *WebSocketBeacon, data []byte) error {
	beacon.Conn.SetWriteDeadline(time.Now().Add(BeaconWriteDeadline))
	w, err := beacon.Conn.NextWriter(websocket.BinaryMessage)
	if err != nil {
		slog.Error("WebSocket compressed write error", "agent_id", beacon.AgentID, "error", err)
		return err
	}
	w.Write(data)
	if err := w.Close(); err != nil {
		slog.Error("WebSocket compressed close writer error", "agent_id", beacon.AgentID, "error", err)
		return err
	}
	return nil
}

func (s *Server) wsReadPump(beacon *WebSocketBeacon) {
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("recovered from wsReadPump panic", "agent_id", beacon.AgentID, "err", r, "stack", string(debug.Stack()))
		}
	}()
	defer func() {
		s.wsHub.Unregister(beacon.AgentID, beacon)
		beacon.Conn.Close()
		s.handleWSBeaconDisconnect(beacon.AgentID)
	}()

	beacon.Conn.SetReadLimit(WSMaxMessageSize)
	beacon.Conn.SetReadDeadline(time.Now().Add(BeaconReadDeadline))
	beacon.Conn.SetPongHandler(func(string) error {
		beacon.Conn.SetReadDeadline(time.Now().Add(BeaconReadDeadline))
		beacon.touchLastSeen()
		return nil
	})

	for {
		_, message, err := beacon.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Error("WebSocket read error", "agent_id", beacon.AgentID, "error", err)
			}
			break
		}

		// Mirror the HTTP beacon path: strip request-side malleable wrapping
		// (no-op when the agent does not wrap) and the ContentLengthJitter
		// length-prefixed padding before decoding the envelope.
		message = s.stripMalleableRequest(message)
		message = s.stripBodyPadding(message)
		env, req, kind := s.decodeBeaconEnvelope(message)
		if kind == frameRejected {
			slog.Warn("WebSocket beacon envelope rejected", "agent_id", beacon.AgentID)
			continue
		}

		// Re-register under the agent's real UUID if it differs from connection UUID
		if req.UUID != beacon.AgentID && isValidAgentID(req.UUID) {
			s.wsHub.Remove(beacon.AgentID, beacon)
			beacon.AgentID = req.UUID
			s.wsHub.Register(beacon.AgentID, beacon)
		}

		var respJSON []byte
		if kind == frameEncrypted {
			// Process beacon (auth frames are handled without the pipeline:
			// the agent discards the auth response and re-beacons encrypted,
			// so claiming/fetching tasks here would lose them).
			resp := s.processBeacon(req, "")
			if s.sessionManager.NeedsRekey(req.UUID, BeaconSessionRekeyMessages) {
				resp.Rekey = true
			}
			var ok bool
			respJSON, ok = s.buildBeaconResponse(req.UUID, env.Seq, resp)
			if !ok {
				slog.Error("WebSocket response build error", "agent_id", beacon.AgentID)
				continue
			}
		} else {
			var ok bool
			respJSON, ok = s.processAuthFrame(env, kind)
			if !ok {
				slog.Warn("WebSocket auth frame rejected", "agent_id", beacon.AgentID)
				continue
			}
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Warn("WebSocket beacon send to closed channel", "agent_id", beacon.AgentID)
				}
			}()
			select {
			case beacon.Send <- respJSON:
			default:
				slog.Warn("WebSocket send channel full, closing connection", "agent_id", beacon.AgentID)
				beacon.Conn.Close()
			}
		}()
	}
}

// WSOperatorSession tracks an operator's WebSocket session.
type WSOperatorSession struct {
	Conn      *websocket.Conn
	UserID    uint
	Username  string
	Page      string
	AgentView string
	// lastSeenNano is atomic: the read pump writes it on every frame while
	// presence scans read it concurrently (P3-10 data race fix).
	lastSeenNano atomic.Int64
	writeMu      sync.Mutex // guards Conn.WriteMessage (gorilla requires single-writer)
	// Outbound queue + dedicated writer goroutine: broadcasts must never do a
	// synchronous socket write per session, or one wedged browser stalls every
	// event-producing path (head-of-line blocking for ALL operators).
	send     chan []byte
	closeOnce sync.Once
	done     chan struct{}
}

func (s *WSOperatorSession) touchLastSeen() { s.lastSeenNano.Store(time.Now().UnixNano()) }
func (s *WSOperatorSession) lastSeenAt() time.Time {
	return time.Unix(0, s.lastSeenNano.Load())
}

// enqueue delivers msg to the session's writer without blocking; a client
// that falls a full buffer behind is disconnected instead of stalling the hub.
func (s *WSOperatorSession) enqueue(msg []byte) {
	select {
	case s.send <- msg:
	case <-s.done:
	default:
		// Buffer full: this operator cannot keep up. Drop the connection so
		// the reader unblocks and cleanup runs.
		s.closeOnce.Do(func() { close(s.done) })
		s.Conn.Close()
	}
}

// writePump drains the send queue onto the socket until done is closed.
func (s *WSOperatorSession) writePump() {
	for {
		// Prefer exit when done is closed even if a queued message remains.
		select {
		case <-s.done:
			return
		default:
		}
		select {
		case <-s.done:
			return
		case msg := <-s.send:
			s.writeMu.Lock()
			s.Conn.SetWriteDeadline(time.Now().Add(OperatorWriteDeadline))
			err := s.Conn.WriteMessage(websocket.TextMessage, msg)
			s.writeMu.Unlock()
			if err != nil {
				slog.Warn("Operator WS writer failed, dropping connection", "user", s.Username, "error", err)
				s.closeOnce.Do(func() { close(s.done) })
				s.Conn.Close()
				return
			}
		}
	}
}

// operatorSessions tracks connected operator WebSocket sessions.
type operatorSessionTracker struct {
	mu       sync.Mutex
	sessions map[uint]*WSOperatorSession
}

func (t *operatorSessionTracker) add(s *WSOperatorSession) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if existing, ok := t.sessions[s.UserID]; ok && existing != s {
		existing.Conn.Close()
	}
	t.sessions[s.UserID] = s
}

func (t *operatorSessionTracker) remove(userID uint, s *WSOperatorSession) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if cur, ok := t.sessions[userID]; ok && cur == s {
		delete(t.sessions, userID)
	}
}

// BroadcastToOperators queues a message for all connected operator sessions.
// Non-blocking by design: the per-session writer goroutine performs the
// socket write, so a wedged client can never stall event producers.
func (t *operatorSessionTracker) BroadcastToOperators(msg []byte) {
	t.mu.Lock()
	sessions := make([]*WSOperatorSession, 0, len(t.sessions))
	for _, session := range t.sessions {
		sessions = append(sessions, session)
	}
	t.mu.Unlock()

	for _, session := range sessions {
		session.enqueue(msg)
	}
}

// operatorHeartbeatTimeout is how long since the last ping before an operator
// is considered inactive for soft-lock purposes.
const operatorHeartbeatTimeout = 60 * time.Second

// ActiveOperatorsForAgent returns the usernames of operators who are currently
// viewing the given agent and have pinged within operatorHeartbeatTimeout.
// The caller (identified by excludeUserID) is excluded from the list.
func (t *operatorSessionTracker) ActiveOperatorsForAgent(agentID string, excludeUserID uint) []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	cutoff := time.Now().Add(-operatorHeartbeatTimeout)
	var names []string
	for uid, s := range t.sessions {
		if uid == excludeUserID {
			continue
		}
		if s.AgentView == agentID && !s.lastSeenAt().Before(cutoff) {
			names = append(names, s.Username)
		}
	}
	return names
}

// ActiveOperatorCount returns the number of operators connected and seen
// within operatorHeartbeatTimeout.
func (t *operatorSessionTracker) ActiveOperatorCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	cutoff := time.Now().Add(-operatorHeartbeatTimeout)
	count := 0
	for _, s := range t.sessions {
		if !s.lastSeenAt().Before(cutoff) {
			count++
		}
	}
	return count
}

// OperatorPresenceSnapshot returns a snapshot of all active operators and the
// agent they are viewing (if any). Used for broadcasting presence events.
func (t *operatorSessionTracker) OperatorPresenceSnapshot() []map[string]interface{} {
	t.mu.Lock()
	defer t.mu.Unlock()
	cutoff := time.Now().Add(-operatorHeartbeatTimeout)
	var ops []map[string]interface{}
	for _, s := range t.sessions {
		if !s.lastSeenAt().Before(cutoff) {
			entry := map[string]interface{}{
				"user": s.Username,
			}
			if s.AgentView != "" {
				entry["agent_id"] = s.AgentView
			}
			ops = append(ops, entry)
		}
	}
	return ops
}

// broadcastOperatorEvent dispatches a JSON event to every connected operator
// dashboard. Events fan out to BOTH the legacy /ws hub and any /ws/operator
// sessions, so every browser client receives the same stream regardless of
// which endpoint it dialed. (The beacon wsHub is deliberately NOT used.)
func (s *Server) broadcastOperatorEvent(payload map[string]interface{}) {
	msg, ok := marshalJSONSafe(payload)
	if !ok {
		return
	}
	s.broadcastToClients(msg)
	if s.operatorSessions != nil {
		s.operatorSessions.BroadcastToOperators(msg)
	}
}

// broadcastOperatorPresence pushes the current operator presence snapshot to
// all connected clients. Called on connect/disconnect/heartbeat/agent_view
// changes so dashboards can show who is active and where.
func (s *Server) broadcastOperatorPresence() {
	if s.operatorSessions == nil {
		return
	}
	ops := s.operatorSessions.OperatorPresenceSnapshot()
	s.broadcastOperatorEvent(map[string]interface{}{
		"type":      "operator_presence",
		"operators": ops,
	})
}

// sendOperatorSyncSnapshot pushes the current state snapshot (active build
// jobs) to a freshly-connected operator socket, so dashboards can render the
// latest state without waiting for the next event and after a reconnect.
func (s *Server) sendOperatorSyncSnapshot(conn *websocket.Conn) {
	msg, ok := marshalJSONSafe(gin.H{"type": "sync", "builds": s.buildJobSnapshots()})
	if !ok {
		return
	}
	conn.SetWriteDeadline(time.Now().Add(OperatorWriteDeadline))
	if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
		slog.Warn("Failed to send operator sync snapshot", "error", err)
	}
}

// handleOperatorWS handles operator WebSocket connections for real-time updates.
// Requires a valid forgec2_session cookie (unlike agent beacons which use the
// transport envelope for auth).
func (s *Server) handleOperatorWS(c *gin.Context) {
	tokenStr, err := c.Cookie("forgec2_session")
	if err != nil || tokenStr == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "no session token"})
		return
	}
	claims, err := middleware.ParseToken(tokenStr)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "invalid token"})
		return
	}
	if s.isSessionRevoked(tokenStr) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "session_revoked"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Error("Operator WS upgrade failed", "error", err)
		return
	}

	session := &WSOperatorSession{
		Conn:     conn,
		UserID:   claims.UserID,
		Username: claims.Username,
		send:     make(chan []byte, 256),
		done:     make(chan struct{}),
	}
	session.touchLastSeen()

	s.wsHubOnce.Do(func() {
		if s.wsHub == nil {
			s.wsHub = NewWebSocketHub()
		}
	})
	// Reconnect sync: every connect (fresh or re-established) receives the
	// current build snapshot so a client that dropped events while offline
	// converges immediately. Sent before registering the session so no
	// broadcast can interleave with (and corrupt ordering of) the snapshot.
	s.sendOperatorSyncSnapshot(conn)

	s.operatorSessions.add(session)

	// Dedicated outbound writer: all broadcasts flow through session.send.
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		session.writePump()
	}()

	defer func() {
		session.closeOnce.Do(func() { close(session.done) })
		s.operatorSessions.remove(session.UserID, session)
		s.broadcastOperatorPresence()
		conn.Close()
	}()

	readDeadline := 90 * time.Second
	conn.SetReadLimit(WSMaxMessageSize)
	conn.SetReadDeadline(time.Now().Add(readDeadline))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(readDeadline))
		return nil
	})

	// Protocol-level pings are what keep the session alive: the pong handler
	// is the only code that refreshes the read deadline, so without a ping
	// ticker the absolute 90s deadline expired and every operator session was
	// force-disconnected exactly 90s after connect. The application-level
	// JSON "ping" below does NOT refresh it.
	//
	// The ticker is owned INSIDE the goroutine and exits via session.done:
	// time.Ticker.Stop() does not close its channel, so a `for range` loop
	// parked on it leaked one goroutine per operator connect/disconnect.
	go func() {
		pingTicker := time.NewTicker(BeaconPingInterval)
		defer pingTicker.Stop()
		for {
			select {
			case <-session.done:
				return
			case <-pingTicker.C:
			}
			session.writeMu.Lock()
			err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(OperatorWriteDeadline))
			session.writeMu.Unlock()
			if err != nil {
				// Write failed: close so the blocked ReadJSON unblocks.
				conn.Close()
				return
			}
		}
	}()

	for {
		var msg map[string]interface{}
		if err := conn.ReadJSON(&msg); err != nil {
			break
		}

		msgType, _ := msg["type"].(string)
		switch msgType {
		case "page_update":
			if page, ok := msg["page"].(string); ok {
				session.Page = page
			}
		case "agent_view":
			if agentID, ok := msg["agent_id"].(string); ok {
				session.AgentView = agentID
				session.touchLastSeen()
				s.broadcastOperatorPresence()
			}
		case "ping":
			session.touchLastSeen()
			session.writeMu.Lock()
			_ = conn.WriteJSON(map[string]string{"type": "pong"})
			session.writeMu.Unlock()
			s.broadcastOperatorPresence()
		}
	}
}
