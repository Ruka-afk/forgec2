package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func newCampaignSeedTestServer(t *testing.T) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	return &Server{
		db:                testutil.SetupTestDB(t),
		cfg:               cfg,
		wsClients:         make(map[*websocket.Conn]*wsClientConn),
		agentPendingTasks: make(map[string]int),
	}
}

func campaignSeedRequest(t *testing.T, s *Server, id, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: id}}
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/campaigns/"+id+"/killchain", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user", "alice")
	s.handleCampaignKillChain(c)
	return w
}

func seedCampaignWithAgents(t *testing.T, s *Server, campaignID string, agentIDs ...string) {
	t.Helper()
	if err := s.db.Create(&db.Campaign{ID: campaignID, Name: "test-campaign"}).Error; err != nil {
		t.Fatalf("seed campaign: %v", err)
	}
	for _, id := range agentIDs {
		if err := s.db.Create(&db.Implant{ID: id, Hostname: "host-" + id, OS: "Windows"}).Error; err != nil {
			t.Fatalf("seed agent: %v", err)
		}
		if err := s.db.Create(&db.CampaignAgent{CampaignID: campaignID, AgentID: id}).Error; err != nil {
			t.Fatalf("seed campaign agent: %v", err)
		}
	}
}

// TestCampaignKillChainSeedsTasks verifies the endpoint no longer fakes success:
// it must materialize a pending task per template step per campaign agent.
func TestCampaignKillChainSeedsTasks(t *testing.T) {
	s := newCampaignSeedTestServer(t)
	seedCampaignWithAgents(t, s, "c1", "a1", "a2")

	w := campaignSeedRequest(t, s, "c1", `{"template":"Standard Intrusion"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var resp struct {
		Success      bool `json:"success"`
		TasksCreated int  `json:"tasks_created"`
		Agents       int  `json:"agents"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Success || resp.TasksCreated != 14 || resp.Agents != 2 {
		t.Fatalf("unexpected response: %+v", resp)
	}

	var tasks []db.Task
	if err := s.db.Where("agent_id IN ?", []string{"a1", "a2"}).Find(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 14 {
		t.Fatalf("got %d tasks, want 14", len(tasks))
	}
	// Every seeded task must be pending and attributable to the operator.
	for _, tk := range tasks {
		if tk.Status != "pending" {
			t.Errorf("task %d (agent %s) status = %q, want pending", tk.ID, tk.AgentID, tk.Status)
		}
		if tk.CreatedBy != "alice" {
			t.Errorf("task %d created_by = %q, want alice", tk.ID, tk.CreatedBy)
		}
	}
	// The shell steps must carry their template command, not silence.
	shellCount := 0
	for _, tk := range tasks {
		if tk.Type == "shell" {
			shellCount++
			if tk.Command == "" {
				t.Errorf("shell task %d has empty command", tk.ID)
			}
		}
	}
	if shellCount != 6 {
		t.Errorf("shell tasks = %d, want 6 (3 per agent)", shellCount)
	}
}

// TestCampaignKillChainRejectsBadInput pins the honest failure paths: the
// endpoint must not report success when it cannot create any tasks.
func TestCampaignKillChainRejectsBadInput(t *testing.T) {
	s := newCampaignSeedTestServer(t)
	seedCampaignWithAgents(t, s, "c1", "a1")
	seedCampaignWithAgents(t, s, "empty")

	cases := []struct {
		name string
		id   string
		body string
	}{
		{"no body", "c1", ""},
		{"malformed body", "c1", `{"nope":1}`},
		{"unknown template", "c1", `{"template":"Not Real"}`},
		{"campaign without agents", "empty", `{"template":"Standard Intrusion"}`},
		{"missing campaign", "nope", `{"template":"Standard Intrusion"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := campaignSeedRequest(t, s, tc.id, tc.body)
			if w.Code == http.StatusOK {
				t.Fatalf("expected non-OK status for %q, got 200: %s", tc.name, w.Body.String())
			}
		})
	}

	var count int64
	s.db.Model(&db.Task{}).Count(&count)
	if count != 0 {
		t.Fatalf("no tasks should be created on failed seeds, got %d", count)
	}
}

// TestCampaignKillChainHonoursPendingCeiling ensures seeding cannot bypass the
// per-agent pending-task limit that blocks normal task creation.
func TestCampaignKillChainHonoursPendingCeiling(t *testing.T) {
	s := newCampaignSeedTestServer(t)
	seedCampaignWithAgents(t, s, "c1", "a1")
	s.agentPendingTasksMu.Lock()
	s.agentPendingTasks["a1"] = MaxPendingTasksPerAgent
	s.agentPendingTasksMu.Unlock()

	w := campaignSeedRequest(t, s, "c1", `{"template":"Standard Intrusion"}`)
	if w.Code == http.StatusOK {
		t.Fatalf("expected non-OK status when agent is at ceiling: %s", w.Body.String())
	}
	var count int64
	s.db.Model(&db.Task{}).Count(&count)
	if count != 0 {
		t.Fatalf("no tasks should be created when all agents are at ceiling, got %d", count)
	}
}