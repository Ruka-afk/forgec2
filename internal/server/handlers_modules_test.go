package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
)

func newModulesTestServer(t *testing.T) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dataDir := t.TempDir()
	cfg := &config.Config{}
	cfg.Server.DataDir = dataDir
	return &Server{
		db:                newContractDB(t),
		cfg:               cfg,
		wsClients:         make(map[*websocket.Conn]*wsClientConn),
		agentPendingTasks: make(map[string]int),
		ctx:               context.Background(),
		metrics:           &MetricsCollector{TasksTotal: prometheus.NewCounter(prometheus.CounterOpts{})},
	}
}

func TestSanitizeModuleName(t *testing.T) {
	tests := []struct {
		in      string
		wantErr bool
		want    string
	}{
		{"Invoke-Mimikatz.ps1", false, "Invoke-Mimikatz.ps1"},
		{"../evil.ps1", false, "evil.ps1"},
		{"", true, ""},
		{".", true, ""},
		{"..", true, ""},
	}
	for _, tt := range tests {
		got, err := sanitizeModuleName(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Fatalf("sanitizeModuleName(%q): expected error", tt.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("sanitizeModuleName(%q): %v", tt.in, err)
		}
		if got != tt.want {
			t.Fatalf("sanitizeModuleName(%q)=%q want %q", tt.in, got, tt.want)
		}
	}
}

func TestHandleModulesDeploy_JSON(t *testing.T) {
	s := newModulesTestServer(t)
	agent := seedImplant(t, s.db)

	modDir := s.modulesDir()
	modName := "helper.ps1"
	if err := os.WriteFile(filepath.Join(modDir, modName), []byte("Write-Host ok"), 0600); err != nil {
		t.Fatalf("write module: %v", err)
	}

	body := map[string]string{
		"name": modName,
		"path": `C:\Windows\Temp\helper.ps1`,
	}
	raw, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(http.MethodPost, "/agents/"+agent.ID+"/modules/deploy", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: agent.ID}}
	c.Set("user_role", "operator")
	c.Set("user", "tester")

	s.handleModulesDeploy(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v", err)
	}
	if resp["success"] != true {
		t.Fatalf("expected success: %s", w.Body.String())
	}
	if _, ok := resp["task_id"]; !ok {
		t.Fatalf("missing task_id: %s", w.Body.String())
	}

	var task db.Task
	if err := s.db.First(&task, resp["task_id"]).Error; err != nil {
		t.Fatalf("task not found: %v", err)
	}
	if task.Type != "upload" {
		t.Fatalf("task type=%s want upload", task.Type)
	}
	if !strings.Contains(task.Path, "helper.ps1") {
		t.Fatalf("path=%q", task.Path)
	}
}

func TestHandleModulesDeploy_MissingModule(t *testing.T) {
	s := newModulesTestServer(t)
	agent := seedImplant(t, s.db)

	raw := []byte(`{"name":"missing.ps1"}`)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(http.MethodPost, "/agents/"+agent.ID+"/modules/deploy", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: agent.ID}}
	c.Set("user_role", "operator")

	s.handleModulesDeploy(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404 body=%s", w.Code, w.Body.String())
	}
}

func TestHandleModulesDeploy_MissingAgent(t *testing.T) {
	s := newModulesTestServer(t)
	id := uuid.New().String()
	raw := []byte(`{"name":"x.ps1"}`)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(http.MethodPost, "/agents/"+id+"/modules/deploy", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: id}}
	c.Set("user_role", "operator")

	s.handleModulesDeploy(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404 body=%s", w.Code, w.Body.String())
	}
}

func TestHandleModulesList_Empty(t *testing.T) {
	s := newModulesTestServer(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(http.MethodGet, "/api/modules", nil)
	c.Request = req

	s.handleModulesList(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Success bool  `json:"success"`
		Modules []any `json:"modules"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success")
	}
	if len(resp.Modules) != 0 {
		t.Fatalf("expected empty modules, got %d", len(resp.Modules))
	}
}

func TestGRPCListener_InsecureDefault(t *testing.T) {
	l := NewGRPCListener("127.0.0.1:0")
	if l.tlsCreds != nil {
		t.Fatal("expected insecure (no TLS) by default")
	}
	// SetTLS with nil must keep lab insecure mode
	l.SetTLS(nil)
	if l.tlsCreds != nil {
		t.Fatal("SetTLS(nil) should remain insecure")
	}
}
