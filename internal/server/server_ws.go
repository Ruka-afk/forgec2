package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// ── Operator WebSocket hub + broadcast fan-out ────────────────────────────

// handleWebSocket handles WebSocket connections for real-time notifications
// Auth is validated inside the handler (from cookie or query token) so this
// endpoint does NOT need to be behind the AuthRequired middleware.
func (s *Server) handleWebSocket(c *gin.Context) {
	origin := c.GetHeader("Origin")
	if origin != "" && !s.wsUpgrader.CheckOrigin(c.Request) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "error": "origin not allowed"})
		return
	}

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

	username := claims.Username

	conn, err := s.wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Error("Failed to upgrade WebSocket", "err", err)
		return
	}

	conn.SetReadLimit(WSMaxMessageSize)
	conn.SetReadDeadline(time.Now().Add(WSReadDeadline))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(WSReadDeadline))
		return nil
	})

	session := UserSession{Username: username, ConnectedAt: time.Now()}
	client := &wsClientConn{
		conn:    conn,
		session: session,
		ch:      make(chan []byte, wsWriteChanSize),
		done:    make(chan struct{}),
	}

	s.wsMutex.Lock()
	if len(s.wsClients) >= MaxWSConnections {
		s.wsMutex.Unlock()
		slog.Warn("WebSocket connection limit reached", "current", len(s.wsClients), "limit", MaxWSConnections)
		conn.Close()
		return
	}
	s.wsClients[conn] = client
	s.wsMutex.Unlock()

	// Writer goroutine: drains the buffered channel and writes to the socket.
	// Tracked by the shutdown WaitGroup: hijacked WS conns are invisible to
	// http.Server.Shutdown, so without wg tracking the DB could be closed
	// while these pumps were still running.
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() {
			close(client.done)
		}()
		ticker := time.NewTicker(WSPingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case msg, ok := <-client.ch:
				if !ok {
					return
				}
				conn.SetWriteDeadline(time.Now().Add(WSWriteDeadline))
				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					slog.Debug("Failed to send WebSocket message", "user", username, "err", err)
					// Close so the blocked reader exits immediately instead of
					// lingering for the full 60s read deadline while holding a
					// MaxWSConnections slot.
					conn.Close()
					return
				}
			case <-ticker.C:
				conn.SetWriteDeadline(time.Now().Add(WSWriteDeadline))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					conn.Close()
					return
				}
			}
		}
	}()

	slog.Info("WebSocket client connected", "user", username)
	// Reconnect sync: push the current build snapshot so a dashboard that was
	// offline (reconnect) or connected mid-build converges instantly. Routed
	// through the writer goroutine, never written from the reader.
	if msg, ok := marshalJSONSafe(gin.H{"type": "sync", "builds": s.buildJobSnapshots()}); ok {
		select {
		case client.ch <- msg:
		default:
		}
	}
	s.broadcastUserEvent("user_online", username, session)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("WebSocket reader panicked", "user", username, "recover", r)
			}
			s.wsMutex.Lock()
			delete(s.wsClients, conn)
			s.wsMutex.Unlock()
			close(client.ch)
			s.broadcastUserEvent("user_offline", username, session)
			conn.Close()
			slog.Info("WebSocket client disconnected", "user", username)
		}()

		for {
			conn.SetReadDeadline(time.Now().Add(WSReadDeadline))
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			// Answer application-level heartbeats. The frontend tracks liveness
			// with {type:"ping"}/{type:"pong"} application messages; without a
			// pong it closes the socket every HEARTBEAT_TIMEOUT and reconnects.
			// Route the reply through the writer goroutine: gorilla/websocket
			// allows only one concurrent writer, and the reader goroutine must
			// never WriteMessage on the same conn.
			var msg struct {
				Type string `json:"type"`
			}
			if len(data) > 0 && data[0] == '{' && json.Unmarshal(data, &msg) == nil && msg.Type == "ping" {
				select {
				case client.ch <- []byte(`{"type":"pong"}`):
				default:
					// Writer queue full — drop the pong rather than block the
					// read loop; the frontend will reconnect on heartbeat timeout.
				}
			}
		}
	}()
}

// UserSession holds metadata about a connected operator WebSocket session.
type UserSession struct {
	Username    string    `json:"username"`
	ConnectedAt time.Time `json:"connected_at"`
}

// wsClientConn wraps a WebSocket connection with a buffered write channel
// so that broadcastToClients never blocks on slow readers.
type wsClientConn struct {
	conn    *websocket.Conn
	session UserSession
	ch      chan []byte
	done    chan struct{}
}

const wsWriteChanSize = 64

// getOnlineUsers returns the list of currently connected operator sessions.
func (s *Server) getOnlineUsers() []UserSession {
	s.wsMutex.RLock()
	defer s.wsMutex.RUnlock()
	users := make([]UserSession, 0, len(s.wsClients))
	seen := make(map[string]bool)
	for _, client := range s.wsClients {
		if !seen[client.session.Username] {
			seen[client.session.Username] = true
			users = append(users, client.session)
		}
	}
	return users
}

// broadcastUserEvent sends a user online/offline event to all WebSocket clients.
func (s *Server) broadcastUserEvent(eventType, username string, session UserSession) {
	msg, ok := marshalJSONSafe(map[string]interface{}{
		"type":         eventType,
		"username":     username,
		"connected_at": session.ConnectedAt,
		"online_users": s.getOnlineUsers(),
	})
	if !ok {
		return
	}
	s.broadcastToClients(msg)
}

// broadcastToClients sends a message to all connected WebSocket clients.
// Uses buffered channels so the caller never blocks on slow readers.
func (s *Server) broadcastToClients(message []byte) {
	s.wsMutex.RLock()
	clients := make([]*wsClientConn, 0, len(s.wsClients))
	for _, client := range s.wsClients {
		clients = append(clients, client)
	}
	s.wsMutex.RUnlock()

	for _, client := range clients {
		func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Warn("Operator broadcast to closed channel", "user", client.session.Username)
				}
			}()
			select {
			case <-s.ctx.Done():
				return
			case client.ch <- message:
			default:
				slog.Debug("WebSocket write channel full, dropping message", "user", client.session.Username)
			}
		}()
	}
}

// broadcastAgentOnline pushes agent online events to all WebSocket clients.
func (s *Server) broadcastAgentOnline(agent db.Implant, isNew bool) {
	if !isNew && !s.suppressAgentStatusEvent(agent.ID) {
		return
	}
	payload := map[string]interface{}{
		"type":     "agent_online",
		"agent_id": agent.ID,
		"hostname": agent.Hostname,
		"username": agent.Username,
		"ip":       agent.IP,
		"new":      isNew,
	}
	notification, err := json.Marshal(payload)
	if err != nil {
		slog.Error("Failed to marshal agent online notification", "err", err)
		return
	}
	s.broadcastToClients(notification)
}

// broadcastAgentOffline pushes agent offline events to all WebSocket clients.
func (s *Server) broadcastAgentOffline(agent db.Implant) {
	if !s.suppressAgentStatusEvent(agent.ID) {
		return
	}
	payload := map[string]string{
		"type":     "agent_offline",
		"agent_id": agent.ID,
		"hostname": agent.Hostname,
		"ip":       agent.IP,
	}
	notification, err := json.Marshal(payload)
	if err != nil {
		slog.Error("Failed to marshal agent offline notification", "err", err)
		return
	}
	s.broadcastToClients(notification)
}

// suppressAgentStatusEvent ensures at most one status event per agent every 60 seconds.
// Returns true if the event should proceed, false if it should be suppressed.
func (s *Server) suppressAgentStatusEvent(agentID string) bool {
	s.agentStatusCooldownMu.Lock()
	defer s.agentStatusCooldownMu.Unlock()
	last, ok := s.agentStatusCooldown[agentID]
	now := time.Now()
	if ok && now.Sub(last) < AgentStatusCooldown {
		return false
	}
	s.agentStatusCooldown[agentID] = now
	return true
}

func (s *Server) handleWSBeaconDisconnect(agentID string) {
	slog.Info("WebSocket beacon disconnected", "agent_id", agentID)
	var agent db.Implant
	if err := s.db.Where("id = ?", agentID).First(&agent).Error; err != nil {
		return
	}
	if agent.Status != "offline" {
		if err := s.db.Model(&agent).Update("status", "stale").Error; err != nil {
			slog.Error("Failed to mark agent stale", "agent_id", agentID, "error", err)
		}
		s.recordAgentStatusEvent(agentID, "stale")
		if s.suppressAgentStatusEvent(agentID) {
			s.broadcastAgentOffline(agent)
		}
	}
}

// broadcastAgentDataUpdate pushes agent data changes to all WebSocket clients.
func (s *Server) broadcastAgentDataUpdate(agentID string, data map[string]interface{}) {
	payload := map[string]interface{}{
		"type":     "agent_data_update",
		"agent_id": agentID,
		"data":     data,
	}
	notification, err := json.Marshal(payload)
	if err != nil {
		slog.Error("Failed to marshal agent data update", "err", err)
		return
	}
	s.broadcastToClients(notification)
}

// broadcastTaskUpdate pushes task status (and result if completed) to WS clients
func (s *Server) broadcastTaskUpdate(agentID string, task db.Task) {
	task = taskForOperator(task)
	payload := map[string]interface{}{
		"type":       "task_update",
		"agent_id":   agentID,
		"task_id":    task.ID,
		"task_type":  task.Type,
		"status":     task.Status,
		"command":    task.Command,
		"created_by": task.CreatedBy,
	}
	if task.Result != "" && task.Type == "shell" && task.Status == "completed" {
		if len(task.Result) > TaskOutputStreamThreshold {
			// Large shell output is streamed in ordered frames. Do not attach a
			// second truncated value that can be mistaken for the full result.
			s.broadcastTaskOutputFrames(agentID, task)
			payload["result_complete"] = false
		} else {
			// Small interactive-shell results fit comfortably in one WS frame.
			// Mark this value explicitly so clients can distinguish it from the
			// previews used by task list notifications.
			payload["result"] = task.Result
			payload["result_complete"] = true
		}
	} else if task.Result != "" {
		payload["result"] = truncateString(task.Result, 200)
		payload["result_complete"] = false
	}
	if task.Error != "" {
		payload["error"] = task.Error
	}
	notification, err := json.Marshal(payload)
	if err != nil {
		slog.Error("Failed to marshal task update", "err", err)
		return
	}
	s.broadcastToClients(notification)
}

// taskForOperator is the final storage-boundary guard for task output. Most DB
// reads are decrypted by db.Task.AfterFind, but result objects created by map
// updates or passed directly from write paths may still contain FC2ENC values.
// Never expose that at-rest representation through REST or WebSocket APIs.
func taskForOperator(task db.Task) db.Task {
	for fieldName, field := range map[string]*string{
		"result": &task.Result,
		"error":  &task.Error,
	} {
		if !strings.HasPrefix(*field, "FC2ENC:") {
			continue
		}
		plain, err := crypto.DecryptLoot(*field)
		if err != nil {
			slog.Error("Failed to decrypt task field for operator response", "task_id", task.ID, "field", fieldName, "error", err)
			*field = ""
			continue
		}
		*field = plain
	}
	return task
}

// TaskOutputStreamThreshold: results above this size are chunked into
// task_output frames instead of a single task_update payload.
const TaskOutputStreamThreshold = 4 * 1024

// TaskOutputFrameSize: target size per streamed frame.
const TaskOutputFrameSize = 16 * 1024

// broadcastTaskOutputFrames pushes a completed shell result as ordered
// "task_output" frames, each ending on a line boundary when possible. The
// final frame carries done:true so clients know the stream is complete.
func (s *Server) broadcastTaskOutputFrames(agentID string, task db.Task) {
	result := task.Result
	for start := 0; start < len(result); {
		end := start + TaskOutputFrameSize
		if end >= len(result) {
			end = len(result)
		} else {
			// Prefer a chunk boundary that does not split a line or a
			// multi-byte UTF-8 rune.
			for i := end - 1; i > start; i-- {
				if result[i] == '\n' {
					end = i + 1
					break
				}
			}
			if end <= start {
				end = start + TaskOutputFrameSize
			}
		}
		frame := map[string]interface{}{
			"type":     "task_output",
			"agent_id": agentID,
			"task_id":  task.ID,
			"chunk":    result[start:end],
			"done":     end >= len(result),
		}
		raw, err := json.Marshal(frame)
		if err != nil {
			slog.Error("Failed to marshal task output frame", "err", err)
			return
		}
		s.broadcastToClients(raw)
		start = end
	}
}

func truncateString(s string, max int) string {
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}

func (s *Server) pushBulkResult(r BulkResult) {
	s.bulkHistoryMu.Lock()
	defer s.bulkHistoryMu.Unlock()
	if len(s.bulkHistory) >= maxBulkHistory {
		s.bulkHistory = s.bulkHistory[1:]
	}
	r.ID = len(s.bulkHistory) + 1
	s.bulkHistory = append(s.bulkHistory, r)
}
