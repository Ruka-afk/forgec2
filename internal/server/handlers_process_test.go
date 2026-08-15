package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func TestLastPSSnapshotEnvelopeIsNotLiveTree(t *testing.T) {
	env := lastPSSnapshotEnvelope("pid 1 explorer.exe")
	if env["live"] != false {
		t.Fatalf("live=%v, want false", env["live"])
	}
	if env["kind"] != "last_ps_snapshot" {
		t.Fatalf("kind=%v, want last_ps_snapshot", env["kind"])
	}
	if env["source"] != "ps" || env["alias_of"] != "ps" {
		t.Fatalf("source/alias not ps: %#v", env)
	}
	if env["processes"] != "pid 1 explorer.exe" {
		t.Fatalf("processes=%v", env["processes"])
	}
}

func TestHandleGetProcessTreeUsesLastPSSnapshot(t *testing.T) {
	s := newAgentTestServer(t)
	if err := s.db.Create(&db.Implant{ID: "a1", Hostname: "box"}).Error; err != nil {
		t.Fatalf("seed implant: %v", err)
	}
	if err := s.db.Create(&db.Task{AgentID: "a1", Type: "ps", Status: "completed", Result: "svchost.exe"}).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "a1"}}
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/agents/a1/process-tree", nil)

	s.handleGetProcessTree(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Processes string `json:"processes"`
		Source    string `json:"source"`
		Kind      string `json:"kind"`
		AliasOf   string `json:"alias_of"`
		Live      bool   `json:"live"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v body=%s", err, w.Body.String())
	}
	if resp.Live {
		t.Fatal("handler claimed a live process tree")
	}
	if resp.Kind != "last_ps_snapshot" || resp.Source != "ps" || resp.AliasOf != "ps" {
		t.Fatalf("dishonest envelope: %+v", resp)
	}
	if resp.Processes != "svchost.exe" {
		t.Fatalf("processes=%q", resp.Processes)
	}
}

func TestHandleGetProcessTreeRequiresCompletedPS(t *testing.T) {
	s := newAgentTestServer(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "missing"}}
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/agents/missing/process-tree", nil)

	s.handleGetProcessTree(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "not a live process tree") {
		t.Fatalf("error should say this is not a live tree: %s", w.Body.String())
	}
}
