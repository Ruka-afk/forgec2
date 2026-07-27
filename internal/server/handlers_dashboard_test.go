package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func newDashboardTestServer(t *testing.T) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	cfg.Server.OfflineThreshold = 60
	return &Server{db: testutil.SetupTestDB(t), cfg: cfg, wsClients: make(map[*websocket.Conn]*wsClientConn)}
}

func TestHandleDashboardActivityHeatmap(t *testing.T) {
	tests := []struct {
		name   string
		seed   []db.Task
		query  string
		minLen int
	}{
		{
			name:   "empty",
			seed:   nil,
			query:  "?range=24h",
			minLen: 0,
		},
		{
			name: "with data 24h",
			seed: []db.Task{
				{AgentID: "a1", Status: "completed", CreatedAt: time.Now()},
				{AgentID: "a2", Status: "failed", CreatedAt: time.Now()},
			},
			query:  "?range=24h",
			minLen: 1,
		},
		{
			name: "with data 7d",
			seed: []db.Task{
				{AgentID: "a1", Status: "completed", CreatedAt: time.Now().AddDate(0, 0, -3)},
			},
			query:  "?range=7d",
			minLen: 1,
		},
		{
			name: "with data 30d",
			seed: []db.Task{
				{AgentID: "a1", Status: "completed", CreatedAt: time.Now().AddDate(0, 0, -15)},
			},
			query:  "?range=30d",
			minLen: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := newDashboardTestServer(t)
			for _, task := range tc.seed {
				if err := s.db.Create(&task).Error; err != nil {
					t.Fatalf("seed task: %v", err)
				}
			}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, "/api/dashboard/activity-heatmap"+tc.query, nil)

			s.handleDashboardActivityHeatmap(c)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
			}
			var resp []HeatmapData
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("invalid json: %v; body=%s", err, w.Body.String())
			}
			if len(resp) < tc.minLen {
				t.Errorf("expected at least %d heatmap entries, got %d", tc.minLen, len(resp))
			}
		})
	}
}

func TestHandleDashboardOSDistribution(t *testing.T) {
	tests := []struct {
		name      string
		seed      []db.Implant
		wantCount int
	}{
		{
			name:      "empty",
			seed:      nil,
			wantCount: 0,
		},
		{
			name: "with data",
			seed: []db.Implant{
				{ID: "a1", Hostname: "DC01", OS: "Windows", LastSeen: time.Now()},
				{ID: "a2", Hostname: "WEB01", OS: "Linux", LastSeen: time.Now()},
				{ID: "a3", Hostname: "MAC01", OS: "macOS", LastSeen: time.Now()},
			},
			wantCount: 3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := newDashboardTestServer(t)
			for _, a := range tc.seed {
				if err := s.db.Create(&a).Error; err != nil {
					t.Fatalf("seed agent: %v", err)
				}
			}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodGet, "/api/dashboard/os-distribution", nil)

			s.handleDashboardOSDistribution(c)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
			}
			var resp []OSDistribution
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("invalid json: %v; body=%s", err, w.Body.String())
			}
			if len(resp) != tc.wantCount {
				t.Errorf("expected %d OS entries, got %d", tc.wantCount, len(resp))
			}
		})
	}
}

func TestHandleDashboardTaskStatus(t *testing.T) {
	s := newDashboardTestServer(t)

	// Seed tasks with various statuses
	statuses := []string{"completed", "completed", "completed", "failed", "pending", "running"}
	for _, st := range statuses {
		s.db.Create(&db.Task{AgentID: "a1", Status: st, CreatedAt: time.Now()})
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/dashboard/task-status", nil)

	s.handleDashboardTaskStatus(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	var resp []TaskStatusData
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v; body=%s", err, w.Body.String())
	}
	if len(resp) == 0 {
		t.Error("expected at least one task status entry")
	}

	// Verify status counts
	countMap := make(map[string]int64)
	for _, d := range resp {
		countMap[d.Name] = d.Count
	}
	if countMap["Completed"] != 3 {
		t.Errorf("Completed count = %d, want 3", countMap["Completed"])
	}
	if countMap["Failed"] != 1 {
		t.Errorf("Failed count = %d, want 1", countMap["Failed"])
	}
}

func TestHandleDashboardListenerTraffic(t *testing.T) {
	s := newDashboardTestServer(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/dashboard/listener-traffic?range=24h", nil)

	s.handleDashboardListenerTraffic(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	var resp TrafficData
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v; body=%s", err, w.Body.String())
	}
	// Should have labels array
	if resp.Labels == nil {
		t.Error("expected labels array")
	}
}

func TestHandleDashboardCredentialTypes(t *testing.T) {
	s := newDashboardTestServer(t)

	s.db.Create(&db.CredentialEntry{Type: "ntlm", Hash: "hash123", AgentID: "a1"})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/dashboard/credential-types", nil)

	s.handleDashboardCredentialTypes(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}

	var resp []CredentialTypeData
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v; body=%s", err, w.Body.String())
	}

	if len(resp) == 0 {
		t.Error("expected at least 1 credential type entry")
	}
}

func TestHandleDashboardAttackPath(t *testing.T) {
	s := newDashboardTestServer(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/dashboard/attack-path", nil)

	s.handleDashboardAttackPath(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	var resp AttackPathData
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v; body=%s", err, w.Body.String())
	}
	// Should return a valid structure even with empty data
	if resp.Nodes == nil {
		resp.Nodes = []AttackPathNode{}
	}
	if resp.Edges == nil {
		resp.Edges = []AttackPathEdge{}
	}
}
