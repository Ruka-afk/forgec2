package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/forgec2/forgec2/pkg/encoding"
	"github.com/gin-gonic/gin"
)

func TestE2E_Smoke_HealthEndpoint(t *testing.T) {
	db := testutil.SetupTestDB(t)
	s := &Server{db: db}
	gin.SetMode(gin.TestMode)

	t.Run("health returns ok", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/health", nil)
		s.handleHealth(c)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
		}
		testutil.AssertValidJSON(t, w.Body.Bytes(), "health")
	})

	t.Run("ready returns ok", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/ready", nil)
		s.handleHealth(c)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
		}
		testutil.AssertValidJSON(t, w.Body.Bytes(), "ready")
	})
}

func TestE2E_Smoke_ListEndpoints(t *testing.T) {
	db := testutil.SetupTestDB(t)
	s := &Server{db: db}
	gin.SetMode(gin.TestMode)

	t.Run("list agents returns empty", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/api/agents", nil)
		s.handleListAgents(c)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
		}
		testutil.AssertKeyExists(t, w.Body.Bytes(), "list agents", "agents")
	})

	t.Run("list listeners returns empty", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/api/listeners", nil)
		s.handleListListeners(c)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
		}
		testutil.AssertKeyExists(t, w.Body.Bytes(), "list listeners", "data")
	})

	t.Run("dashboard heatmap returns ok", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/api/dashboard/activity-heatmap?range=24h", nil)
		s.handleDashboardActivityHeatmap(c)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
		}
		testutil.AssertValidJSON(t, w.Body.Bytes(), "dashboard heatmap")
	})
}

// TestE2E_Beacon_Task_ResultFlow tests the full lifecycle:
// 1. Agent registers via beacon
// 2. Server assigns a task
// 3. Agent receives task on next beacon
// 4. Agent submits result
// 5. Server stores result and marks task completed
func TestE2E_Beacon_Task_ResultFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := newContractDB(t)
	s := &Server{
		db:               database,
		cfg:              &config.Config{},
		beaconDedupCache: make(map[string]time.Time),
		eventManager:     NewEventManager(database),
		socksEngine:      newSocksRelayEngine(),
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/beacon", s.handleBeacon)
	s.router = r

	agentUUID := "e2e-test-agent-001"

	// Helper: clear dedup cache so each beacon is processed
	clearDedup := func() {
		s.beaconDedupMu.Lock()
		s.beaconDedupCache = make(map[string]time.Time)
		s.beaconDedupMu.Unlock()
	}

	// Helper: send a beacon and return the raw response body
	sendBeacon := func(t *testing.T, body string) *httptest.ResponseRecorder {
		t.Helper()
		clearDedup()
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/beacon", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("beacon: expected 200, got %d; body=%s", w.Code, w.Body.String())
		}
		return w
	}

	// Helper: decode a beacon response (strips format byte prefix)
	decodeResp := func(t *testing.T, body []byte) beaconResponse {
		t.Helper()
		var resp beaconResponse
		if err := encoding.Unmarshal(body, &resp); err != nil {
			t.Fatalf("failed to unmarshal beacon response: %v", err)
		}
		return resp
	}

	// Step 1: First beacon — agent registers (no results, no acks)
	t.Run("step1_agent_registers", func(t *testing.T) {
		beaconJSON := `{"uuid":"` + agentUUID + `","info":{"hostname":"TESTPC","username":"testuser","ip":"10.0.0.1"},"pv":1}`
		sendBeacon(t, beaconJSON)

		// Verify agent was created
		var agent db.Implant
		if err := database.Where("id = ?", agentUUID).First(&agent).Error; err != nil {
			t.Fatalf("agent not found in DB: %v", err)
		}
		if agent.Hostname != "TESTPC" {
			t.Errorf("hostname = %q, want TESTPC", agent.Hostname)
		}
		if agent.Username != "testuser" {
			t.Errorf("username = %q, want testuser", agent.Username)
		}
	})

	// Step 2: Create a task for the agent via direct DB insert
	var taskID uint
	t.Run("step2_create_task", func(t *testing.T) {
		task := &db.Task{
			AgentID: agentUUID,
			Type:    "shell",
			Command: "whoami",
			Status:  "pending",
		}
		if err := database.Create(task).Error; err != nil {
			t.Fatalf("create task: %v", err)
		}
		taskID = task.ID
		if taskID == 0 {
			t.Fatal("task ID should be > 0 after create")
		}
	})

	// Step 3: Agent beacons again — should receive the pending task
	var resp3 beaconResponse
	t.Run("step3_agent_receives_task", func(t *testing.T) {
		beaconJSON := `{"uuid":"` + agentUUID + `","pv":1}`
		w := sendBeacon(t, beaconJSON)
		resp3 = decodeResp(t, w.Body.Bytes())

		if len(resp3.Tasks) == 0 {
			t.Fatal("expected at least 1 task, got 0")
		}
		found := false
		for _, task := range resp3.Tasks {
			if task.ID == taskID {
				found = true
				if task.Type != "shell" {
					t.Errorf("task type = %q, want shell", task.Type)
				}
			}
		}
		if !found {
			t.Errorf("task %d not found in response tasks (got %v)", taskID, resp3.Tasks)
		}
	})

	// Step 4: Agent submits result
	t.Run("step4_agent_submits_result", func(t *testing.T) {
		resultsJSON, _ := json.Marshal([]taskResult{{
			TaskID: taskID,
			Type:   "shell",
			Output: "testdomain\\testuser",
		}})
		beaconJSON := `{"uuid":"` + agentUUID + `","pv":1,"results":` + string(resultsJSON) + `,"acks":[` + itoa(int(taskID)) + `]}`
		sendBeacon(t, beaconJSON)
	})

	// Step 5: Verify result was stored
	t.Run("step5_verify_result_stored", func(t *testing.T) {
		var task db.Task
		if err := database.Where("id = ?", taskID).First(&task).Error; err != nil {
			t.Fatalf("task not found: %v", err)
		}
		if task.Status != "completed" {
			t.Errorf("task status = %q, want completed", task.Status)
		}
		if task.Result != "testdomain\\testuser" {
			t.Errorf("task result = %q, want testdomain\\testuser", task.Result)
		}
	})

	// Step 6: Second beacon returns empty tasks (already completed)
	t.Run("step6_no_more_tasks", func(t *testing.T) {
		beaconJSON := `{"uuid":"` + agentUUID + `","pv":1}`
		w := sendBeacon(t, beaconJSON)
		resp := decodeResp(t, w.Body.Bytes())
		if len(resp.Tasks) != 0 {
			t.Errorf("expected 0 tasks, got %d", len(resp.Tasks))
		}
	})

	// Step 7: Protocol version rejection
	t.Run("step7_protocol_version_rejection", func(t *testing.T) {
		beaconJSON := `{"uuid":"old-agent-999","pv":0,"info":{"hostname":"old"}}`
		clearDedup()
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/beacon", strings.NewReader(beaconJSON))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		// pv=0 is not rejected (old agents that don't send pv)
		if w.Code != http.StatusOK {
			t.Errorf("pv=0 should be accepted for backward compat, got %d", w.Code)
		}
	})
}
