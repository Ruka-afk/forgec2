package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gin-gonic/gin"
)

func newTasksTestServer(t *testing.T) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	s := &Server{db: testutil.SetupTestDB(t)}
	s.agentPendingTasks = make(map[string]int)
	s.metrics = NewMetricsCollector(s)
	return s
}

func seedTask(t *testing.T, s *Server, agentID, taskType, command, status string) db.Task {
	t.Helper()
	task := db.Task{
		AgentID: agentID,
		Type:    taskType,
		Command: command,
		Status:  status,
	}
	if err := s.db.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}
	return task
}

func TestHandleGetAgentTasks_Empty(t *testing.T) {
	s := newTasksTestServer(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/agents/agent-1/tasks", nil)
	c.Params = gin.Params{{Key: "id", Value: "agent-1"}}

	s.handleGetAgentTasks(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Tasks []db.Task `json:"tasks"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v; body=%s", err, w.Body.String())
	}
	if len(resp.Tasks) != 0 {
		t.Fatalf("expected empty tasks, got %d", len(resp.Tasks))
	}
}

func TestHandleGetAgentTasks_WithData(t *testing.T) {
	s := newTasksTestServer(t)
	seedTask(t, s, "agent-tasks", "shell", "whoami", "completed")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/agents/agent-tasks/tasks", nil)
	c.Params = gin.Params{{Key: "id", Value: "agent-tasks"}}

	s.handleGetAgentTasks(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Tasks []db.Task `json:"tasks"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v; body=%s", err, w.Body.String())
	}
	if len(resp.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(resp.Tasks))
	}
	if resp.Tasks[0].Command != "whoami" {
		t.Fatalf("expected command 'whoami', got %q", resp.Tasks[0].Command)
	}
}

func TestAPI_BulkTaskStatus_Success(t *testing.T) {
	s := newTasksTestServer(t)
	t1 := seedTask(t, s, "agent-1", "shell", "whoami", "completed")
	t2 := seedTask(t, s, "agent-1", "ls", "ls /", "failed")

	body, _ := json.Marshal(map[string]interface{}{"task_ids": []uint{t1.ID, t2.ID}})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/tasks/status", bytes.NewReader(body))

	s.apiBulkTaskStatus(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Success bool                       `json:"success"`
		Data    map[uint]db.Task           `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v; body=%s", err, w.Body.String())
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 task results, got %d", len(resp.Data))
	}
	if resp.Data[t1.ID].Status != "completed" {
		t.Fatalf("expected task %d status completed, got %q", t1.ID, resp.Data[t1.ID].Status)
	}
	if resp.Data[t2.ID].Status != "failed" {
		t.Fatalf("expected task %d status failed, got %q", t2.ID, resp.Data[t2.ID].Status)
	}
}

func TestAPI_BulkTaskStatus_RequiresTaskIDs(t *testing.T) {
	s := newTasksTestServer(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/tasks/status", nil)

	s.apiBulkTaskStatus(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestAPI_BulkTaskStatus_MaxTaskIDs(t *testing.T) {
	s := newTasksTestServer(t)

	ids := make([]uint, MaxTaskIDsPerRequest+1)
	body, _ := json.Marshal(map[string]interface{}{"task_ids": ids})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/tasks/status", bytes.NewReader(body))

	s.apiBulkTaskStatus(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestHandleExportTasks_CSV(t *testing.T) {
	s := newTasksTestServer(t)
	seedTask(t, s, "agent-1", "shell", "whoami", "completed")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/tasks/export", nil)

	s.handleExportTasks(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/csv") {
		t.Fatalf("expected csv content type, got %q", ct)
	}
	if !strings.Contains(w.Body.String(), "whoami") {
		t.Fatalf("expected task command in CSV, body=%s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Agent") {
		t.Fatalf("expected CSV header row, body=%s", w.Body.String())
	}
}

func TestHandleTaskHistory_Renders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	cfg.Server.OfflineThreshold = 60
	s := &Server{db: testutil.SetupTestDB(t), cfg: cfg}
	seedTask(t, s, "agent-1", "shell", "whoami", "completed")
	seedTask(t, s, "agent-1", "shell", "badcommand", "failed")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/tasks", nil)
	c.Set("Accept", "text/html")

	s.handleTaskHistory(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "whoami") {
		t.Fatalf("expected task command in rendered page, body=%s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "badcommand") {
		t.Fatalf("expected failed task in rendered page, body=%s", w.Body.String())
	}
}
