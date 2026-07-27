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

func newOpsTestServer(t *testing.T) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	return &Server{db: testutil.SetupTestDB(t)}
}

func TestHandleGetOpsLog_Empty(t *testing.T) {
	s := newOpsTestServer(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/opsec/history", nil)

	s.handleOpsecHistory(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		History []db.OpsecHistory `json:"history"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v; body=%s", err, w.Body.String())
	}
	if len(resp.History) != 0 {
		t.Fatalf("expected empty history, got %d entries", len(resp.History))
	}
}

func TestHandleGetOpsLog_WithData(t *testing.T) {
	s := newOpsTestServer(t)
	entry := db.OpsecHistory{
		AgentID:  "agent-ops",
		TaskType: "shell",
		RuleName: "test-rule",
		Allowed:  true,
		Message:  "test message",
	}
	if err := s.db.Create(&entry).Error; err != nil {
		t.Fatalf("seed opsec history: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/opsec/history", nil)

	s.handleOpsecHistory(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		History []db.OpsecHistory `json:"history"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v; body=%s", err, w.Body.String())
	}
	if len(resp.History) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(resp.History))
	}
	if resp.History[0].RuleName != "test-rule" {
		t.Fatalf("expected rule name 'test-rule', got %q", resp.History[0].RuleName)
	}
}
