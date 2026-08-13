package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/forgec2/forgec2/internal/config"
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

func TestApproveSelfCreatedTaskRejected(t *testing.T) {
	s := newTasksTestServer(t)
	task := seedTask(t, s, "agent-approve", "shell", "whoami", TaskStatusPendingApproval)
	s.db.Model(&task).Update("created_by", "alice")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/tasks/1/approve", nil)
	c.Params = gin.Params{{Key: "taskId", Value: "1"}}
	c.Set("user_role", "admin")
	c.Set("user", "alice")

	s.handleApproveTask(c)

	// Two-man rule: the creator may not approve their own task.
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for self-approval, got %d; body=%s", w.Code, w.Body.String())
	}
	var stored db.Task
	if err := s.db.First(&stored, task.ID).Error; err != nil {
		t.Fatalf("reload task: %v", err)
	}
	if stored.Status != TaskStatusPendingApproval {
		t.Fatalf("self-approval must not promote task, got status %q", stored.Status)
	}
}

func TestApproveRecordsSecondOperator(t *testing.T) {
	s := newTasksTestServer(t)
	task := seedTask(t, s, "agent-approve", "shell", "whoami", TaskStatusPendingApproval)
	s.db.Model(&task).Update("created_by", "alice")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/tasks/1/approve", nil)
	c.Params = gin.Params{{Key: "taskId", Value: "1"}}
	c.Set("user_role", "admin")
	c.Set("user", "bob")

	s.handleApproveTask(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for second-operator approval, got %d; body=%s", w.Code, w.Body.String())
	}
	var stored db.Task
	if err := s.db.First(&stored, task.ID).Error; err != nil {
		t.Fatalf("reload task: %v", err)
	}
	if stored.Status != "pending" {
		t.Fatalf("expected pending, got %q", stored.Status)
	}
	if stored.ApprovedBy != "bob" {
		t.Fatalf("expected approved_by=bob, got %q", stored.ApprovedBy)
	}
	if stored.ApprovedAt == nil {
		t.Fatal("expected approved_at to be set")
	}
}

func TestDangerousTaskWithApprovalEnabledIsDormant(t *testing.T) {
	s := newTasksTestServer(t)
	s.cfg = &config.Config{}
	s.cfg.Security.RequireApproval = true

	task, err := s.createTask("agent-danger", "uninstall", "delete self", "", "", "", 0, 0)
	if err != nil {
		t.Fatalf("createTask: %v", err)
	}
	if task.Status != TaskStatusPendingApproval {
		t.Fatalf("dangerous task with require_approval must start pending_approval, got %q", task.Status)
	}
	// Beacon must not claim it until approved.
	if claimed := s.fetchPendingTasks("agent-danger"); len(claimed) != 0 {
		t.Fatalf("pending_approval dangerous task must not be claimable, got %d", len(claimed))
	}
}

func TestNonDangerousTaskUnaffectedByApproval(t *testing.T) {
	s := newTasksTestServer(t)
	s.cfg = &config.Config{}
	s.cfg.Security.RequireApproval = true

	task, err := s.createTask("agent-safe", "shell", "whoami", "", "", "", 0, 0)
	if err != nil {
		t.Fatalf("createTask: %v", err)
	}
	if task.Status != "pending" {
		t.Fatalf("non-dangerous task should stay pending, got %q", task.Status)
	}
}
