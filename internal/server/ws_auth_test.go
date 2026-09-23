package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func newWSAuthTestServer(t *testing.T) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := config.DefaultConfig()
	cfg.Server.JWTSecret = "test-secret-for-wsauth-32chars!"
	if err := middleware.InitJWTSecret(cfg, ""); err != nil {
		t.Fatalf("InitJWTSecret: %v", err)
	}
	database := newContractDB(t)
	s := &Server{db: database, cfg: cfg}
	s.operatorSessions = &operatorSessionTracker{sessions: make(map[uint]*WSOperatorSession)}
	return s
}

func seedWSAuthUser(t *testing.T, s *Server, username string) db.User {
	t.Helper()
	user := db.User{Username: username, Role: "admin", IsActive: true}
	if err := s.db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

func issueWSAuthToken(t *testing.T, s *Server, user db.User) string {
	t.Helper()
	token, err := middleware.GenerateToken(user, false, 24)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	if err := s.createSession(token, user.ID, "127.0.0.1", "test", "", 86400); err != nil {
		t.Fatalf("create session: %v", err)
	}
	return token
}

func callOperatorWS(s *Server, token string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/ws/operator", nil)
	if token != "" {
		c.Request.AddCookie(&http.Cookie{Name: "forgec2_session", Value: token})
	}
	s.handleOperatorWS(c)
	return w
}

// TestHandleOperatorWS_ForceLogoutRejected proves force-logout (user row bump)
// rejects a WS connect that still has a non-revoked session row — previously
// only isSessionRevoked was checked, so force-logout alone left sockets open.
func TestHandleOperatorWS_ForceLogoutRejected(t *testing.T) {
	s := newWSAuthTestServer(t)
	user := seedWSAuthUser(t, s, "ws-force-logout")
	token := issueWSAuthToken(t, s, user)

	// Simulate force-logout: bump ForceLogoutAt after the token IssuedAt.
	if err := s.db.Model(&user).Update("force_logout_at", time.Now().Add(time.Minute)).Error; err != nil {
		t.Fatalf("set force_logout_at: %v", err)
	}

	w := callOperatorWS(s, token)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("force-logout WS: got %d, want 401; body=%s", w.Code, w.Body.String())
	}
}

// TestHandleOperatorWS_InactiveUserRejected proves a disabled account cannot
// open a new operator socket even with a still-valid session row.
func TestHandleOperatorWS_InactiveUserRejected(t *testing.T) {
	s := newWSAuthTestServer(t)
	user := seedWSAuthUser(t, s, "ws-inactive")
	token := issueWSAuthToken(t, s, user)

	if err := s.db.Model(&user).Update("is_active", false).Error; err != nil {
		t.Fatalf("disable user: %v", err)
	}
	middleware.InvalidateUserCache(user.ID)

	w := callOperatorWS(s, token)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("inactive WS: got %d, want 401; body=%s", w.Code, w.Body.String())
	}
}

// TestHandleOperatorWS_RevokedSessionRejected keeps the session-row path covered.
func TestHandleOperatorWS_RevokedSessionRejected(t *testing.T) {
	s := newWSAuthTestServer(t)
	user := seedWSAuthUser(t, s, "ws-revoked")
	token := issueWSAuthToken(t, s, user)
	if !s.revokeSession(token) {
		t.Fatal("revokeSession failed")
	}

	w := callOperatorWS(s, token)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("revoked WS: got %d, want 401; body=%s", w.Code, w.Body.String())
	}
}

// TestWsSessionStillValid covers the periodic recheck helper used by live sockets.
func TestWsSessionStillValid(t *testing.T) {
	s := newWSAuthTestServer(t)
	user := seedWSAuthUser(t, s, "ws-recheck")
	token := issueWSAuthToken(t, s, user)

	claims, err := middleware.ParseToken(token)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if !s.wsSessionStillValid(token, claims) {
		t.Fatal("fresh session should be valid")
	}

	// Force-logout after IssuedAt → invalid.
	if err := s.db.Model(&user).Update("force_logout_at", time.Now().Add(time.Hour)).Error; err != nil {
		t.Fatalf("force logout: %v", err)
	}
	if s.wsSessionStillValid(token, claims) {
		t.Fatal("force-logout must invalidate live WS recheck")
	}

	// Clear force-logout, revoke row → invalid.
	if err := s.db.Model(&user).Update("force_logout_at", nil).Error; err != nil {
		t.Fatalf("clear force logout: %v", err)
	}
	if !s.revokeSession(token) {
		t.Fatal("revoke failed")
	}
	if s.wsSessionStillValid(token, claims) {
		t.Fatal("revoked session must fail WS recheck")
	}
}

// TestHandleWebSocket_ForceLogoutRejected covers the legacy /ws notification socket.
func TestHandleWebSocket_ForceLogoutRejected(t *testing.T) {
	s := newWSAuthTestServer(t)
	s.wsUpgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	s.wsClients = make(map[*websocket.Conn]*wsClientConn)
	s.ctx = t.Context()

	user := seedWSAuthUser(t, s, "ws-legacy-force")
	token := issueWSAuthToken(t, s, user)
	if err := s.db.Model(&user).Update("force_logout_at", time.Now().Add(time.Minute)).Error; err != nil {
		t.Fatalf("force logout: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/ws", nil)
	c.Request.AddCookie(&http.Cookie{Name: "forgec2_session", Value: token})
	s.handleWebSocket(c)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("legacy force-logout WS: got %d, want 401; body=%s", w.Code, w.Body.String())
	}
}
