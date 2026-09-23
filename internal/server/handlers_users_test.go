package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func newUserTestServer(t *testing.T) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	cfg.Server.OfflineThreshold = 60
	return &Server{db: testutil.SetupTestDB(t), cfg: cfg, wsClients: make(map[*websocket.Conn]*wsClientConn)}
}

func TestHandleListUsers_Empty(t *testing.T) {
	s := newUserTestServer(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/users", nil)

	s.handleUsersPage(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v; body=%s", err, w.Body.String())
	}
	usersRaw, ok := resp["Users"]
	if !ok {
		t.Fatal("expected 'Users' key in response")
	}
	users, ok := usersRaw.([]any)
	if !ok {
		t.Fatalf("expected 'Users' to be array, got %T", usersRaw)
	}
	if len(users) != 0 {
		t.Fatalf("expected empty users list, got %d", len(users))
	}
}

func TestHandleListUsers_WithData(t *testing.T) {
	s := newUserTestServer(t)
	user := db.User{
		Username:     "testadmin",
		PasswordHash: "hash",
		Role:         db.RoleAdmin,
		IsActive:     true,
	}
	if err := s.db.Create(&user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/users", nil)

	s.handleUsersPage(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v; body=%s", err, w.Body.String())
	}
	usersRaw, ok := resp["Users"]
	if !ok {
		t.Fatal("expected 'Users' key in response")
	}
	users, ok := usersRaw.([]any)
	if !ok {
		t.Fatalf("expected 'Users' to be array, got %T", usersRaw)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
}

// TestHandleToggleUserDisableRevokesSessions proves disabling a user both
// deactivates the account and bumps ForceLogoutAt, so pre-disable JWTs can
// never resurrect — not during the auth-cache TTL, and not if the account
// is later re-enabled.
func TestHandleToggleUserDisableRevokesSessions(t *testing.T) {
	s := newUserTestServer(t)
	admin := db.User{Username: "toggle-admin", Role: db.RoleAdmin, IsActive: true}
	if err := s.db.Create(&admin).Error; err != nil {
		t.Fatalf("seed admin: %v", err)
	}
	victim := db.User{Username: "toggle-victim", Role: db.RoleUser, IsActive: true}
	if err := s.db.Create(&victim).Error; err != nil {
		t.Fatalf("seed victim: %v", err)
	}
	sessToken := "toggle-victim-live-session"
	if err := s.createSession(sessToken, victim.ID, "127.0.0.1", "test", "", 3600); err != nil {
		t.Fatalf("create session: %v", err)
	}

	toggle := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodPost, fmt.Sprintf("/users/%d/toggle", victim.ID), nil)
		c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", victim.ID)}}
		c.Set("user", admin.Username)
		c.Set("user_role", db.RoleAdmin)
		s.handleToggleUser(c)
		return w
	}

	if w := toggle(); w.Code != http.StatusOK {
		t.Fatalf("disable got %d body=%s, want 200", w.Code, w.Body.String())
	}
	var after db.User
	if err := s.db.First(&after, victim.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if after.IsActive {
		t.Fatal("victim should be disabled")
	}
	if after.ForceLogoutAt.IsZero() {
		t.Fatal("disable must bump force_logout_at so pre-disable JWTs cannot resurrect on re-enable")
	}
	if revoked, err := s.isSessionRevoked(sessToken); err != nil || !revoked {
		t.Fatalf("disable must revoke live session rows (revoked=%v err=%v)", revoked, err)
	}

	// Re-enable: account active again, but ForceLogoutAt stays bumped so
	// tokens issued before the disable remain rejected.
	if w := toggle(); w.Code != http.StatusOK {
		t.Fatalf("re-enable got %d body=%s, want 200", w.Code, w.Body.String())
	}
	var re db.User
	if err := s.db.First(&re, victim.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !re.IsActive {
		t.Fatal("victim should be re-enabled")
	}
	if re.ForceLogoutAt.IsZero() {
		t.Fatal("re-enable must not clear force_logout_at")
	}
}

// TestHandleForceLogoutUserRevokesSessionRows proves force-logout marks
// every live session row revoked. Without that, a later login clears
// force_logout_at and pre-force-logout JWTs resurrect (auth checks
// force_logout_at before isSessionRevoked; the row was never revoked).
func TestHandleForceLogoutUserRevokesSessionRows(t *testing.T) {
	s := newUserTestServer(t)
	admin := db.User{Username: "fl-admin", Role: db.RoleAdmin, IsActive: true}
	if err := s.db.Create(&admin).Error; err != nil {
		t.Fatalf("seed admin: %v", err)
	}
	victim := db.User{Username: "fl-victim", Role: db.RoleUser, IsActive: true}
	if err := s.db.Create(&victim).Error; err != nil {
		t.Fatalf("seed victim: %v", err)
	}
	token := "force-logout-live-session"
	if err := s.createSession(token, victim.ID, "127.0.0.1", "test", "", 3600); err != nil {
		t.Fatalf("create session: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, fmt.Sprintf("/users/%d/force-logout", victim.ID), nil)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", victim.ID)}}
	c.Set("user", admin.Username)
	c.Set("user_role", db.RoleAdmin)
	s.handleForceLogoutUser(c)
	if w.Code != http.StatusOK {
		t.Fatalf("force-logout got %d body=%s, want 200", w.Code, w.Body.String())
	}

	var after db.User
	if err := s.db.First(&after, victim.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if after.ForceLogoutAt.IsZero() {
		t.Fatal("force-logout must set force_logout_at")
	}
	if revoked, err := s.isSessionRevoked(token); err != nil || !revoked {
		t.Fatalf("session row must be revoked after force-logout (revoked=%v err=%v)", revoked, err)
	}

	// Simulate the next login clearing force_logout_at: the session row
	// must still reject the old token (the actual resurrection hole).
	if err := s.db.Model(&db.User{}).Where("id = ?", victim.ID).Update("force_logout_at", nil).Error; err != nil {
		t.Fatalf("clear force_logout_at: %v", err)
	}
	if revoked, err := s.isSessionRevoked(token); err != nil || !revoked {
		t.Fatalf("session must stay revoked after force_logout_at cleared (revoked=%v err=%v)", revoked, err)
	}
}
