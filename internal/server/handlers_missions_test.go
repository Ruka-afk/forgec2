package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func TestHandleActiveMissions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := newDashboardTestServer(t)

	agent := db.Implant{ID: "agent-1", Hostname: "corp-win10", IP: "10.0.0.5", OS: "windows"}
	if err := s.db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}

	now := time.Now()
	for i, st := range []string{"pending", "running", "completed", "failed"} {
		if err := s.db.Create(&db.Task{
			AgentID:   agent.ID,
			Type:      "shell",
			Command:   "cmd-" + st,
			Status:    st,
			CreatedAt: now.Add(time.Duration(i) * time.Second),
			CreatedBy: "op",
		}).Error; err != nil {
			t.Fatalf("create task %s: %v", st, err)
		}
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/dashboard/active-missions", nil)
	s.handleActiveMissions(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("cache-control = %q, want no-store", got)
	}
	var resp struct {
		Missions []struct {
			ID          uint   `json:"id"`
			AgentID     string `json:"agent_id"`
			Hostname    string `json:"hostname"`
			IP          string `json:"ip"`
			OS          string `json:"os"`
			Type        string `json:"type"`
			Command     string `json:"command"`
			Status      string `json:"status"`
			CreatedBy   string `json:"created_by"`
			CreatedAt   string `json:"created_at"`
			Progress    int    `json:"progress"`
			TotalBytes  int64  `json:"total_bytes"`
			Transferred int64  `json:"transferred"`
		} `json:"missions"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v; body=%s", err, w.Body.String())
	}
	if len(resp.Missions) != 2 {
		t.Fatalf("expected 2 active missions (pending+running), got %d", len(resp.Missions))
	}
	// Oldest first
	if resp.Missions[0].Status != "pending" || resp.Missions[1].Status != "running" {
		t.Fatalf("unexpected order: %s, %s", resp.Missions[0].Status, resp.Missions[1].Status)
	}
	m := resp.Missions[0]
	if m.Hostname != "corp-win10" || m.IP != "10.0.0.5" || m.OS != "windows" {
		t.Fatalf("agent enrichment missing: %+v", m)
	}
	if m.CreatedBy != "op" || m.Command != "cmd-pending" {
		t.Fatalf("unexpected mission payload: %+v", m)
	}
}