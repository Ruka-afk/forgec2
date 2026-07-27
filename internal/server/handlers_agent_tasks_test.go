package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gin-gonic/gin"
)

func newTasksTestServer(t *testing.T) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	return &Server{db: testutil.SetupTestDB(t)}
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
	task := db.Task{
		AgentID: "agent-tasks",
		Type:    "shell",
		Command: "whoami",
		Status:  "completed",
	}
	if err := s.db.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}

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
