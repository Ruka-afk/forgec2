package server

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
)

func TestValidateCommandArg(t *testing.T) {
	tests := []struct {
		name    string
		v       string
		maxLen  int
		wantErr bool
	}{
		{"normal", "createremotethread", 64, false},
		{"pipe delimiter", "a|b", 64, true},
		{"newline", "foo\nbar", 64, true},
		{"null byte", "foo\x00bar", 64, true},
		{"too long", strings.Repeat("x", 65), 64, true},
		{"at max length", strings.Repeat("x", 64), 64, false},
		{"empty", "", 64, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCommandArg(tc.v, tc.maxLen, "test")
			if tc.wantErr && err == nil {
				t.Errorf("expected error for %q", tc.v)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error for %q: %v", tc.v, err)
			}
		})
	}
}

func newInjectTestServer(t *testing.T) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	return &Server{
		db:                testutil.SetupTestDB(t),
		cfg:               &config.Config{},
		ctx:               context.Background(),
		wsClients:         make(map[*websocket.Conn]*wsClientConn),
		agentPendingTasks: make(map[string]int),
		metrics:           &MetricsCollector{TasksTotal: prometheus.NewCounter(prometheus.CounterOpts{})},
	}
}

func doInjectRequest(s *Server, t *testing.T, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "a1"}}
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/agents/a1/inject", strings.NewReader(form.Encode()))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.Set("user_role", "operator")
	s.handleInject(c)
	return w
}

func TestHandleInject_RejectsInvalidInput(t *testing.T) {
	validSC := base64.StdEncoding.EncodeToString([]byte{0x90, 0x90, 0x90})

	t.Run("invalid pid", func(t *testing.T) {
		s := newInjectTestServer(t)
		w := doInjectRequest(s, t, url.Values{"pid": {"abc"}, "shellcode": {validSC}})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("zero pid", func(t *testing.T) {
		s := newInjectTestServer(t)
		w := doInjectRequest(s, t, url.Values{"pid": {"0"}, "shellcode": {validSC}})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("invalid base64", func(t *testing.T) {
		s := newInjectTestServer(t)
		w := doInjectRequest(s, t, url.Values{"pid": {"1234"}, "shellcode": {"!!!not base64!!!"}})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("empty shellcode", func(t *testing.T) {
		s := newInjectTestServer(t)
		w := doInjectRequest(s, t, url.Values{"pid": {"1234"}, "shellcode": {""}})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("oversized shellcode", func(t *testing.T) {
		s := newInjectTestServer(t)
		big := base64.StdEncoding.EncodeToString(make([]byte, MaxShellcodeSize+1))
		w := doInjectRequest(s, t, url.Values{"pid": {"1234"}, "shellcode": {big}})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("technique with pipe", func(t *testing.T) {
		s := newInjectTestServer(t)
		w := doInjectRequest(s, t, url.Values{"pid": {"1234"}, "tech": {"crt|evil"}, "shellcode": {validSC}})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("technique too long", func(t *testing.T) {
		s := newInjectTestServer(t)
		w := doInjectRequest(s, t, url.Values{"pid": {"1234"}, "tech": {strings.Repeat("x", MaxTechniqueLength+1)}, "shellcode": {validSC}})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})
}

func TestHandleInject_CreatesTask(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := newInjectTestServer(t)
	if err := s.db.Create(&db.Implant{ID: "a1", Hostname: "TEST", IP: "10.0.0.1", LastSeen: time.Now()}).Error; err != nil {
		t.Fatalf("seed agent: %v", err)
	}

	sc := []byte{0x90, 0x90, 0x90}
	scB64 := base64.StdEncoding.EncodeToString(sc)
	w := doInjectRequest(s, t, url.Values{"pid": {"4321"}, "tech": {"crt"}, "shellcode": {scB64}})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}

	var task db.Task
	if err := s.db.Where("agent_id = ? AND type = ?", "a1", "inject").First(&task).Error; err != nil {
		t.Fatalf("task not created: %v", err)
	}
	if task.Command != "4321|crt" {
		t.Errorf("unexpected command %q", task.Command)
	}
	if task.Data != scB64 {
		t.Errorf("shellcode payload not stored")
	}
}

func TestHandleSpawn_RejectsInvalidArgs(t *testing.T) {
	t.Run("target with pipe", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		s := newInjectTestServer(t)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "a1"}}
		c.Request, _ = http.NewRequest(http.MethodPost, "/api/agents/a1/spawn", strings.NewReader(url.Values{"target": {"rundll32|evil"}}.Encode()))
		c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		c.Set("user_role", "operator")
		s.handleSpawn(c)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("technique with newline", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		s := newInjectTestServer(t)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "a1"}}
		c.Request, _ = http.NewRequest(http.MethodPost, "/api/agents/a1/spawn", strings.NewReader(url.Values{"technique": {"CreateRemoteThread\npwn"}}.Encode()))
		c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		c.Set("user_role", "operator")
		s.handleSpawn(c)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})
}

func doSendCommandRequest(s *Server, t *testing.T, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	agent := db.Implant{
		ID:       "a1",
		Hostname: "TEST-HOST",
		Username: "testuser",
		OS:       "windows",
		Arch:     "amd64",
		IP:       "10.0.0.1",
	}
	if err := s.db.Create(&agent).Error; err != nil {
		t.Fatalf("seed agent: %v", err)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "a1"}}
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/agents/a1/send_command", strings.NewReader(form.Encode()))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.Set("user_role", "operator")
	s.handleSendCommand(c)
	return w
}

func TestHandleSendCommand_InvalidCallbackLeavesNoTask(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := newInjectTestServer(t)

	// Private-IP callback URL must be rejected by validateCallbackURL.
	form := url.Values{"command": {"whoami"}, "callback_url": {"http://10.0.0.5/cb"}}
	w := doSendCommandRequest(s, t, form)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for private callback URL, got %d", w.Code)
	}

	// No task must have been persisted and pending count must remain 0.
	var count int64
	if err := s.db.Model(&db.Task{}).Count(&count).Error; err != nil {
		t.Fatalf("count tasks: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 tasks after rejected callback, got %d", count)
	}
	if got := s.agentPendingTasks["a1"]; got != 0 {
		t.Fatalf("pending count leaked: got %d, want 0", got)
	}
}

func TestHandleSendCommand_InvalidCallbackMethodLeavesNoTask(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := newInjectTestServer(t)

	form := url.Values{"command": {"whoami"}, "callback_url": {"http://8.8.8.8/cb"}, "callback_method": {"FROB"}}
	w := doSendCommandRequest(s, t, form)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid callback method, got %d", w.Code)
	}

	var count int64
	if err := s.db.Model(&db.Task{}).Count(&count).Error; err != nil {
		t.Fatalf("count tasks: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 tasks after rejected callback method, got %d", count)
	}
	if got := s.agentPendingTasks["a1"]; got != 0 {
		t.Fatalf("pending count leaked: got %d, want 0", got)
	}
}

func TestHandleSendCommand_ValidCreatesTaskAndPendingCount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := newInjectTestServer(t)

	form := url.Values{"command": {"whoami"}, "callback_url": {"http://8.8.8.8/cb"}, "callback_method": {"GET"}}
	w := doSendCommandRequest(s, t, form)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var task db.Task
	if err := s.db.Where("agent_id = ? AND type = ?", "a1", "shell").First(&task).Error; err != nil {
		t.Fatalf("task not created: %v", err)
	}
	if task.Command != "whoami" {
		t.Errorf("unexpected command %q", task.Command)
	}
	if task.CallbackURL != "http://8.8.8.8/cb" {
		t.Errorf("callback URL not stored: %q", task.CallbackURL)
	}
	if task.CallbackMethod != "GET" {
		t.Errorf("callback method not stored: %q", task.CallbackMethod)
	}
	if got := s.agentPendingTasks["a1"]; got != 1 {
		t.Fatalf("pending count: got %d, want 1", got)
	}
}
