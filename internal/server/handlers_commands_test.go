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
