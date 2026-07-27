package server

import (
	"encoding/json"
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
