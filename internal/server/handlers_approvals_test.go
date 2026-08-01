package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func TestApprovalTaskNotClaimedUntilApproved(t *testing.T) {
	s := newTasksTestServer(t)
	task := seedTask(t, s, "agent-approve", "shell", "whoami", TaskStatusPendingApproval)

	if claimed := s.fetchPendingTasks("agent-approve"); len(claimed) != 0 {
		t.Fatalf("pending_approval task must not be claimed by beacon, got %d", len(claimed))
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/tasks/1/approve", nil)
	c.Params = gin.Params{{Key: "taskId", Value: "1"}}
	c.Set("user_role", "admin")

	s.handleApproveTask(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}

	var stored db.Task
	if err := s.db.First(&stored, task.ID).Error; err != nil {
		t.Fatalf("reload task: %v", err)
	}
	if stored.Status != "pending" {
		t.Fatalf("expected status pending after approve, got %q", stored.Status)
	}

	claimed := s.fetchPendingTasks("agent-approve")
	if len(claimed) != 1 {
		t.Fatalf("expected approved task to be claimable, got %d", len(claimed))
	}
}

func TestApproveNonPendingTaskRejected(t *testing.T) {
	s := newTasksTestServer(t)
	seedTask(t, s, "agent-approve", "shell", "whoami", "completed")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/tasks/1/approve", nil)
	c.Params = gin.Params{{Key: "taskId", Value: "1"}}
	c.Set("user_role", "admin")

	s.handleApproveTask(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestRejectTask(t *testing.T) {
	s := newTasksTestServer(t)
	task := seedTask(t, s, "agent-reject", "shell", "del /f important.txt", TaskStatusPendingApproval)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/tasks/1/reject", nil)
	c.Params = gin.Params{{Key: "taskId", Value: "1"}}
	c.Set("user_role", "admin")

	s.handleRejectTask(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}

	var stored db.Task
	if err := s.db.First(&stored, task.ID).Error; err != nil {
		t.Fatalf("reload task: %v", err)
	}
	if stored.Status != "cancelled" {
		t.Fatalf("expected status cancelled after reject, got %q", stored.Status)
	}

	if claimed := s.fetchPendingTasks("agent-reject"); len(claimed) != 0 {
		t.Fatalf("rejected task must not be claimable, got %d", len(claimed))
	}
}

func TestApproveTaskResponseShape(t *testing.T) {
	s := newTasksTestServer(t)
	seedTask(t, s, "agent-approve", "shell", "whoami", TaskStatusPendingApproval)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/tasks/1/approve", nil)
	c.Params = gin.Params{{Key: "taskId", Value: "1"}}
	c.Set("user_role", "admin")

	s.handleApproveTask(c)

	var resp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v; body=%s", err, w.Body.String())
	}
	if !resp.Success {
		t.Fatalf("expected success=true, got %+v", resp)
	}
	if resp.Message == "" {
		t.Fatal("expected non-empty message")
	}
}
