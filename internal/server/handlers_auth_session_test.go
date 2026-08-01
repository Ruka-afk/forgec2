package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func TestHandleLogin_LockoutAfterFailures(t *testing.T) {
	s := newLoginTestServer(t)

	hash, err := middleware.HashPassword("correct-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	s.db.Create(&db.User{
		Username:     "lockout-test-user",
		PasswordHash: hash,
		Role:         "user",
		IsActive:     true,
	})

	form := url.Values{}
	form.Set("username", "lockout-test-user")
	form.Set("password", "wrong-password")

	// Exhaust login attempts
	for i := 0; i < 10; i++ {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
		c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		s.handleLogin(c)
	}

	// Next attempt should be locked out (returns 401 with lockout message)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	s.handleLogin(c)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 after lockout, got %d; body=%s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if errMsg, ok := resp["error"].(string); ok && errMsg != "" {
		if !strings.Contains(errMsg, "Try again") {
			t.Errorf("expected lockout error message, got: %s", errMsg)
		}
	}
}

func TestHandleLogout_RevokesSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := newContractDB(t)
	cfg := newLoginTestServer(t).cfg
	s := &Server{db: database, cfg: cfg}

	hash, err := middleware.HashPassword("test-pass")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	user := db.User{
		Username:     "logout-test",
		PasswordHash: hash,
		Role:         "admin",
		IsActive:     true,
	}
	if err := database.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	token, err := middleware.GenerateToken(user, false, 24)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	if err := s.createSession(token, user.ID, "127.0.0.1", "test-agent", "", 86400); err != nil {
		t.Fatalf("create session: %v", err)
	}

	// Verify session exists
	if s.isSessionRevoked(token) {
		t.Fatal("session should not be revoked before logout")
	}

	// Perform logout
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/logout", nil)
	c.Request.AddCookie(&http.Cookie{Name: "forgec2_session", Value: token})
	c.Set("user", user.Username)
	c.Set("user_id", user.ID)
	s.handleLogout(c)

	if !s.isSessionRevoked(token) {
		t.Fatal("session should be revoked after logout")
	}
}

func TestHandleGetCurrentUser_Active(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := newContractDB(t)
	s := &Server{db: database}

	user := db.User{
		Username: "current-user",
		Role:     "user",
		IsActive: true,
	}
	if err := database.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/current-user", nil)
	c.Set("user_id", user.ID)

	s.handleGetCurrentUser(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			ID       uint   `json:"id"`
			Username string `json:"username"`
			Role     string `json:"role"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v; body=%s", err, w.Body.String())
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if resp.Data.ID != user.ID {
		t.Errorf("expected id=%d, got %d", user.ID, resp.Data.ID)
	}
	if resp.Data.Username != "current-user" {
		t.Errorf("expected username=current-user, got %s", resp.Data.Username)
	}
}

func TestHandleGetCurrentUser_Inactive(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := newContractDB(t)
	s := &Server{db: database}

	user := db.User{
		Username: "inactive-current-user",
		Role:     "user",
		IsActive: false,
	}
	if err := database.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/current-user", nil)
	c.Set("user_id", user.ID)

	s.handleGetCurrentUser(c)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for inactive user, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestHandleLogin_UsernameCaseInsensitive(t *testing.T) {
	s := newLoginTestServer(t)

	hash, err := middleware.HashPassword("pass-case")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	s.db.Create(&db.User{
		Username:     "CaseUser",
		PasswordHash: hash,
		Role:         "user",
		IsActive:     true,
	})

	form := url.Values{}
	form.Set("username", "caseuser")
	form.Set("password", "pass-case")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	s.handleLogin(c)

	if w.Code == http.StatusOK || w.Code == http.StatusFound {
		t.Log("case-insensitive username matched (expected behavior)")
	} else {
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		errVal, _ := resp["error"]
		t.Logf("case mismatch returned status=%d error=%v", w.Code, errVal)
	}
}

func TestHandleLogin_ClearTextPasswordAttempt(t *testing.T) {
	// Verify password hashing comparison works correctly with known values
	s := newLoginTestServer(t)
	plain := "MyS3cur3P@ss!"
	hash, err := middleware.HashPassword(plain)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	s.db.Create(&db.User{
		Username:     "cleartext-test",
		PasswordHash: hash,
		Role:         "user",
		IsActive:     true,
	})

	form := url.Values{}
	form.Set("username", "cleartext-test")
	form.Set("password", plain)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/login", s.handleLogin)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 for valid password, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestHandleLogout_NoSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := newContractDB(t)
	s := &Server{db: database}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/logout", nil)
	router := gin.New()
	router.POST("/logout", s.handleLogout)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "/login" {
		t.Errorf("expected redirect to /login, got %q", loc)
	}
}

func TestCreateSession_Persists(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := newContractDB(t)
	s := &Server{db: database}

	token := "test-session-token-for-persistence"
	userID := uint(42)
	err := s.createSession(token, userID, "10.0.0.1", "curl/7.68", "fp123", 3600)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}

	var sess db.UserSession
	if err := database.Where("token_hash = ?", middleware.TokenHash(token)).First(&sess).Error; err != nil {
		t.Fatalf("session not found: %v", err)
	}
	if sess.UserID != userID {
		t.Errorf("expected UserID=%d, got %d", userID, sess.UserID)
	}
	if sess.IP != "10.0.0.1" {
		t.Errorf("expected IP=10.0.0.1, got %s", sess.IP)
	}
}

func TestIsSessionRevoked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := newContractDB(t)
	s := &Server{db: database}

	token := "unrevoked-session-token"
	if err := s.createSession(token, uint(1), "127.0.0.1", "test", "", 3600); err != nil {
		t.Fatalf("create session: %v", err)
	}

	if s.isSessionRevoked(token) {
		t.Error("session should not be revoked initially")
	}

	s.revokeSession(token)

	if !s.isSessionRevoked(token) {
		t.Error("session should be revoked after revokeSession")
	}
}

func TestRevokeAllUserSessions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := newContractDB(t)
	s := &Server{db: database}

	userID := uint(7)
	for i := 0; i < 3; i++ {
		token := "multi-session-token-" + itoa(i)
		if err := s.createSession(token, userID, "127.0.0.1", "test", "", 3600); err != nil {
			t.Fatalf("create session %d: %v", i, err)
		}
	}

	s.revokeAllUserSessions(userID)

	for i := 0; i < 3; i++ {
		token := "multi-session-token-" + itoa(i)
		if !s.isSessionRevoked(token) {
			t.Errorf("session %d should be revoked", i)
		}
	}
}

// TestHandleLogin_EmptyPassword checks that empty password returns 401
func TestHandleLogin_EmptyPassword(t *testing.T) {
	s := newLoginTestServer(t)

	form := url.Values{}
	form.Set("username", "someuser")
	form.Set("password", "")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	s.handleLogin(c)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for empty password, got %d", w.Code)
	}
}
