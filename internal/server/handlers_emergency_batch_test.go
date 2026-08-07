package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/server/middleware"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)
// smTest returns a fresh session manager for tests.
func smTest(t *testing.T) *crypto.SessionManager {
	t.Helper()
	sm, err := crypto.NewSessionManager()
	if err != nil {
		t.Fatalf("new session manager: %v", err)
	}
	return sm
}

// batchTestServer builds a Server with the fields handleBatchCommand and
// handleKillSwitch rely on (metrics + in-memory pending counts).
func batchTestServer(t *testing.T) (*Server, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	database := testutil.SetupTestDB(t)
	s := &Server{
		db:                database,
		cfg:               &config.Config{},
		sessionManager:    smTest(t),
		beaconDedupCache:  make(map[string]time.Time),
		eventManager:      NewEventManager(database),
		socksEngine:       newSocksRelayEngine(),
		agentPendingTasks: make(map[string]int),
		wsClients:         make(map[*websocket.Conn]*wsClientConn),
		ctx:               context.Background(),
		metrics:           NewMetricsCollector(&Server{}),
	}
	return s, database
}

// TestBatchCommandEnforcesCommandLengthGate verifies a batch command with an
// oversized command is rejected outright (same MaxCommandLength gate as the
// single-task path).
func TestBatchCommandRejectsOversizedCommand(t *testing.T) {
	s, _ := batchTestServer(t)
	s.db.Create(&db.Implant{ID: "b1", Hostname: "H", Status: "online"})

	body := `{"agent_ids":["b1"],"task_type":"shell","command":"` + strings.Repeat("A", MaxCommandLength+1) + `"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/batch/command", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	s.handleBatchCommand(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for oversized command, got %d; body=%s", w.Code, w.Body.String())
	}
	var count int64
	s.db.Model(&db.Task{}).Count(&count)
	if count != 0 {
		t.Fatalf("no tasks should be created for rejected batch, got %d", count)
	}
}

// TestBatchCommandHonorsPerAgentPendingGate verifies the bulk path enforces the
// same per-agent pending ceiling as createTask (K5): an agent already at the
// limit is skipped instead of overloaded.
func TestBatchCommandHonorsPerAgentPendingGate(t *testing.T) {
	s, database := batchTestServer(t)
	agentID := "batch-overload-1"
	database.Create(&db.Implant{ID: agentID, Hostname: "H", Status: "online"})
	database.Create(&db.Task{AgentID: agentID, Type: "shell", Command: "x", Status: "pending"})
	s.agentPendingTasksMu.Lock()
	s.agentPendingTasks[agentID] = MaxPendingTasksPerAgent
	s.agentPendingTasksMu.Unlock()

	body := `{"agent_ids":["` + agentID + `"],"task_type":"shell","command":"whoami"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/tasks/command", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	s.handleBatchCommand(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		TasksCreated int `json:"tasks_created"`
		Failed       int `json:"failed"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response parse: %v; body=%s", err, w.Body.String())
	}
	if resp.TasksCreated != 0 || resp.Failed != 1 {
		t.Fatalf("expected agent skipped (0 created, 1 failed), got created=%d failed=%d", resp.TasksCreated, resp.Failed)
	}
	s.agentPendingTasksMu.Lock()
	pending := s.agentPendingTasks[agentID]
	s.agentPendingTasksMu.Unlock()
	if pending != MaxPendingTasksPerAgent {
		t.Fatalf("pending counter must not change for skipped agent, got %d", pending)
	}
}

// TestBatchCommandAcceptsWithinLimit verifies the gate still allows a healthy
// agent through and the per-agent counter is incremented.
func TestBatchCommandAcceptsWithinLimit(t *testing.T) {
	s, _ := batchTestServer(t)
	s.db.Create(&db.Implant{ID: "b2", Hostname: "H", Status: "online"})

	body := `{"agent_ids":["b2"],"task_type":"shell","command":"whoami"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/tasks/command", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	s.handleBatchCommand(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	var count int64
	s.db.Model(&db.Task{}).Where("agent_id = ?", "b2").Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 task, got %d", count)
	}
}

// TestKillSwitchDisarmReclaimsUninstallTasks verifies disarming the kill switch
// cancels uninstall tasks that were dispatched at arm time but never executed,
// and rolls back the per-agent pending counters.
func TestKillSwitchDisarmReclaimsUninstallTasks(t *testing.T) {
	s, database := batchTestServer(t)

	agentID := "emergency-1"
	database.Create(&db.Implant{ID: agentID, Hostname: "H", Status: "online"})
	database.Create(&db.Task{AgentID: agentID, Type: "uninstall", Command: "", Status: "pending"})

	hashBytes, err := middleware.HashPassword("killpw")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	user := db.User{Username: "op", PasswordHash: hashBytes}
	if err := database.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	s.agentPendingTasksMu.Lock()
	s.agentPendingTasks[agentID] = 1
	s.agentPendingTasksMu.Unlock()

	body := `{"action":"disarm","password":"killpw"}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/killswitch", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", user.ID)
	s.handleKillSwitch(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Success   bool `json:"success"`
		Reclaimed int  `json:"reclaimed"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response parse: %v (body=%s)", err, w.Body.String())
	}
	if !resp.Success || resp.Reclaimed != 1 {
		t.Fatalf("expected 1 reclaimed uninstall task, success=%v reclaimed=%d; body=%s", resp.Success, resp.Reclaimed, w.Body.String())
	}

	var task db.Task
	if err := database.Where("type = ?", "uninstall").First(&task).Error; err != nil {
		t.Fatalf("uninstall task missing: %v", err)
	}
	if task.Status != "cancelled" {
		t.Fatalf("expected uninstall task cancelled, got %q", task.Status)
	}
	s.agentPendingTasksMu.Lock()
	pending := s.agentPendingTasks[agentID]
	s.agentPendingTasksMu.Unlock()
	if pending != 0 {
		t.Fatalf("pending counter not rolled back, got %d", pending)
	}
}