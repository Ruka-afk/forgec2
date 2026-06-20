package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// ── Collaboration State ───────────────────────────────────────────────────────

// wsConn wraps a WebSocket connection with user identity
type wsConn struct {
	conn        *websocket.Conn
	userID      uint
	username    string
	role        string
	currentPage string // e.g. "/agents", "/agents/abc123", "/dashboard"
}

type collabState struct {
	mu        sync.RWMutex
	wsConns   map[*websocket.Conn]*wsConn
	agentLocks map[string]db.AgentLock
	chatMsgs  []chatMessage
}

type chatMessage struct {
	Username  string    `json:"username"`
	Content   string    `json:"content"`
	Role      string    `json:"role,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

const maxChatHistory = 100

// ── Initialization ────────────────────────────────────────────────────────────

func newCollabState() *collabState {
	return &collabState{
		wsConns:    make(map[*websocket.Conn]*wsConn),
		agentLocks: make(map[string]db.AgentLock),
	}
}

// ── Load existing locks from DB ───────────────────────────────────────────────

func (s *Server) loadAgentLocks() {
	var locks []db.AgentLock
	s.db.Find(&locks)
	s.collab.mu.Lock()
	for i := range locks {
		s.collab.agentLocks[locks[i].AgentID] = locks[i]
	}
	s.collab.mu.Unlock()
}

// ── WebSocket Connection Management ───────────────────────────────────────────

func (s *Server) addWSConn(conn *websocket.Conn, userID uint, username, role string) {
	s.collab.mu.Lock()
	s.collab.wsConns[conn] = &wsConn{conn: conn, userID: userID, username: username, role: role}
	online := s.onlineUserListLocked()
	s.collab.mu.Unlock()

	s.broadcastCollab(gin.H{"type": "user_online", "users": online})
	slog.Info("WS user connected", "user", username, "role", role)
}

func (s *Server) removeWSConn(conn *websocket.Conn) {
	s.collab.mu.Lock()
	delete(s.collab.wsConns, conn)
	online := s.onlineUserListLocked()
	s.collab.mu.Unlock()

	s.broadcastCollab(gin.H{"type": "user_offline", "users": online})
	slog.Info("WS user disconnected")
}

func (s *Server) onlineUserListLocked() []gin.H {
	seen := make(map[string]*wsConn)
	for _, wc := range s.collab.wsConns {
		seen[wc.username] = wc
	}
	var list []gin.H
	for _, wc := range seen {
		pageLabel := wc.currentPage
		if pageLabel == "" {
			pageLabel = "空闲"
		}
		list = append(list, gin.H{
			"username":     wc.username,
			"role":         wc.role,
			"current_page": pageLabel,
		})
	}
	return list
}

// updateUserPage sets the current page for a WS connection
func (s *Server) updateUserPage(conn *websocket.Conn, page string) {
	s.collab.mu.Lock()
	var username string
	var role string
	if wc, ok := s.collab.wsConns[conn]; ok {
		username = wc.username
		role = wc.role
		wc.currentPage = page
	}
	online := s.onlineUserListLocked()
	s.collab.mu.Unlock()
	s.broadcastCollab(gin.H{"type": "user_online", "users": online})

	// If viewing an agent detail page, broadcast agent_view event
	if page != "" && username != "" {
		agentID := page
		// strip /agents/ prefix if present
		if len(agentID) > 8 && agentID[:8] == "/agents/" {
			agentID = agentID[8:]
		}
		if agentID != "" && len(agentID) >= 8 {
			s.broadcastCollab(gin.H{
				"type":     "user_viewing_agent",
				"agent_id": agentID,
				"username": username,
				"role":     role,
			})
		}
	}
}

func (s *Server) getOnlineUsers() []gin.H {
	s.collab.mu.RLock()
	defer s.collab.mu.RUnlock()
	return s.onlineUserListLocked()
}

// ── Broadcast ─────────────────────────────────────────────────────────────────

func (s *Server) broadcastCollab(payload gin.H) {
	payload["timestamp"] = time.Now()
	msg, err := json.Marshal(payload)
	if err != nil {
		slog.Error("collab marshal", "err", err)
		return
	}
	s.collab.mu.RLock()
	conns := make([]*wsConn, 0, len(s.collab.wsConns))
	for _, wc := range s.collab.wsConns {
		conns = append(conns, wc)
	}
	s.collab.mu.RUnlock()

	for _, wc := range conns {
		wc.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := wc.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			slog.Error("WS write error", "user", wc.username, "err", err)
			wc.conn.Close()
			s.collab.mu.Lock()
			delete(s.collab.wsConns, wc.conn)
			s.collab.mu.Unlock()
		}
	}
}

// ── Agent Locking ─────────────────────────────────────────────────────────────

func (s *Server) handleLockAgent(c *gin.Context) {
	agentID := c.Param("id")
	user, _ := c.Get("user")
	username := fmt.Sprintf("%v", user)
	userID, _ := c.Get("user_id")
	uid, _ := userID.(uint)

	s.collab.mu.Lock()
	if existing, ok := s.collab.agentLocks[agentID]; ok {
		s.collab.mu.Unlock()
		c.JSON(http.StatusConflict, gin.H{
			"error":    fmt.Sprintf("agent已被 %s 锁定", existing.Username),
			"locked_by": existing.Username,
		})
		return
	}
	lock := db.AgentLock{
		AgentID:  agentID,
		UserID:   uid,
		Username: username,
		LockedAt: time.Now(),
		Note:     c.DefaultPostForm("note", ""),
	}
	s.collab.agentLocks[agentID] = lock
	s.collab.mu.Unlock()

	s.db.Where("agent_id = ?", agentID).Delete(&db.AgentLock{})
	s.db.Create(&lock)

	s.LogAuditRecord(c, "agent_lock", "agent", agentID, fmt.Sprintf("locked by %s", username), true, nil)
	s.broadcastCollab(gin.H{"type": "agent_locked", "agent_id": agentID, "username": username})
	// System chat message for lock event
	s.addChatMessage("[系统]", fmt.Sprintf("%s 锁定了 Agent %s", username, agentID[:8]))
	slog.Info("Agent locked", "agent", agentID, "by", username)
	c.JSON(http.StatusOK, gin.H{"success": true, "lock": lock})
}

func (s *Server) handleUnlockAgent(c *gin.Context) {
	agentID := c.Param("id")
	user, _ := c.Get("user")
	username := fmt.Sprintf("%v", user)

	s.collab.mu.Lock()
	delete(s.collab.agentLocks, agentID)
	s.collab.mu.Unlock()

	s.db.Where("agent_id = ?", agentID).Delete(&db.AgentLock{})

	s.LogAuditRecord(c, "agent_unlock", "agent", agentID, fmt.Sprintf("unlocked by %s", username), true, nil)
	s.broadcastCollab(gin.H{"type": "agent_unlocked", "agent_id": agentID, "username": username})
	// System chat message for unlock event
	s.addChatMessage("[系统]", fmt.Sprintf("%s 解锁了 Agent %s", username, agentID[:8]))
	slog.Info("Agent unlocked", "agent", agentID, "by", username)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleGetLocks(c *gin.Context) {
	s.collab.mu.RLock()
	locks := make([]db.AgentLock, 0, len(s.collab.agentLocks))
	for _, l := range s.collab.agentLocks {
		locks = append(locks, l)
	}
	s.collab.mu.RUnlock()
	c.JSON(http.StatusOK, gin.H{"success": true, "locks": locks})
}

// agentCommandMiddleware checks agent lock + viewer role for agent command routes
func (s *Server) agentCommandMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		agentID := c.Param("id")
		user, _ := c.Get("user")
		username := fmt.Sprintf("%v", user)
		role, _ := c.Get("user_role")
		roleStr := fmt.Sprintf("%v", role)

		// Viewers cannot issue any agent commands
		if roleStr == "viewer" {
			c.JSON(http.StatusForbidden, gin.H{"error": "viewers cannot issue agent commands"})
			c.Abort()
			return
		}

		// Check agent lock
		lockHolder, available := s.checkAgentLock(agentID, username)
		if !available {
			c.JSON(http.StatusLocked, gin.H{
				"error":    fmt.Sprintf("agent已被 %s 锁定", lockHolder),
				"locked_by": lockHolder,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// checkAgentLock returns the lock holder if locked by another user, else empty string
func (s *Server) checkAgentLock(agentID string, currentUser string) (string, bool) {
	s.collab.mu.RLock()
	defer s.collab.mu.RUnlock()
	lock, exists := s.collab.agentLocks[agentID]
	if !exists {
		return "", true
	}
	if lock.Username == currentUser {
		return "", true
	}
	return lock.Username, false
}

// ── Chat ──────────────────────────────────────────────────────────────────────

// addChatMessage appends a chat message (internal use for system messages)
func (s *Server) addChatMessage(username, content string) {
	msg := chatMessage{
		Username:  username,
		Content:   content,
		CreatedAt: time.Now(),
	}
	s.collab.mu.Lock()
	s.collab.chatMsgs = append(s.collab.chatMsgs, msg)
	if len(s.collab.chatMsgs) > maxChatHistory {
		s.collab.chatMsgs = s.collab.chatMsgs[len(s.collab.chatMsgs)-maxChatHistory:]
	}
	s.collab.mu.Unlock()

	s.broadcastCollab(gin.H{"type": "chat", "message": msg})
}

func (s *Server) handleChatSend(c *gin.Context) {
	var req struct {
		Content string `json:"content" form:"content"`
	}
	if err := c.ShouldBind(&req); err != nil || req.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty message"})
		return
	}
	user, _ := c.Get("user")
	username := fmt.Sprintf("%v", user)
	role, _ := c.Get("user_role")
	roleStr := fmt.Sprintf("%v", role)

	msg := chatMessage{
		Username:  username,
		Content:   req.Content,
		Role:      roleStr,
		CreatedAt: time.Now(),
	}

	s.collab.mu.Lock()
	s.collab.chatMsgs = append(s.collab.chatMsgs, msg)
	if len(s.collab.chatMsgs) > maxChatHistory {
		s.collab.chatMsgs = s.collab.chatMsgs[len(s.collab.chatMsgs)-maxChatHistory:]
	}
	s.collab.mu.Unlock()

	s.broadcastCollab(gin.H{"type": "chat", "message": msg})
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleChatHistory(c *gin.Context) {
	s.collab.mu.RLock()
	msgs := make([]chatMessage, len(s.collab.chatMsgs))
	copy(msgs, s.collab.chatMsgs)
	s.collab.mu.RUnlock()
	c.JSON(http.StatusOK, gin.H{"success": true, "messages": msgs})
}

// ── Online Users ──────────────────────────────────────────────────────────────

func (s *Server) handleOnlineUsers(c *gin.Context) {
	users := s.getOnlineUsers()
	c.JSON(http.StatusOK, gin.H{"success": true, "users": users})
}

// ── Page Presence ──────────────────────────────────────────────────────────────

func (s *Server) handlePagePresence(c *gin.Context) {
	s.collab.mu.RLock()
	type pageInfo struct {
		Username string `json:"username"`
		Role     string `json:"role"`
		Page     string `json:"page"`
	}
	seen := make(map[string]*wsConn)
	for _, wc := range s.collab.wsConns {
		seen[wc.username] = wc
	}
	var pages []pageInfo
	for _, wc := range seen {
		page := wc.currentPage
		if page == "" {
			page = "空闲"
		}
		pages = append(pages, pageInfo{Username: wc.username, Role: wc.role, Page: page})
	}
	s.collab.mu.RUnlock()
	c.JSON(http.StatusOK, gin.H{"success": true, "pages": pages})
}

// ── Agent Viewers ──────────────────────────────────────────────────────────────

func (s *Server) handleAgentViewers(c *gin.Context) {
	agentID := c.Param("id")
	s.collab.mu.RLock()
	var viewers []gin.H
	for _, wc := range s.collab.wsConns {
		if wc.currentPage == agentID || wc.currentPage == "/agents/"+agentID {
			viewers = append(viewers, gin.H{
				"username": wc.username,
				"role":     wc.role,
			})
		}
	}
	s.collab.mu.RUnlock()
	c.JSON(http.StatusOK, gin.H{"success": true, "viewers": viewers})
}

// ── Process incoming WS messages from client ──────────────────────────────────

func (s *Server) handleWSMessage(conn *websocket.Conn, data map[string]interface{}) {
	msgType, _ := data["type"].(string)
	switch msgType {
	case "page_update":
		if page, ok := data["page"].(string); ok {
			s.updateUserPage(conn, page)
		}
	case "agent_view":
		if agentID, ok := data["agent_id"].(string); ok {
			s.updateUserPage(conn, agentID)
		}
	}
}
