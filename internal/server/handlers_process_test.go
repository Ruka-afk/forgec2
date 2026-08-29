package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func TestProcessTreeEnvelope(t *testing.T) {
	env := processTreeEnvelope("1 0 systemd", "process_tree")
	if env["live"] != false {
		t.Fatalf("live=%v, want false", env["live"])
	}
	if env["kind"] != "process_tree" || env["source"] != "process_tree" {
		t.Fatalf("envelope=%#v", env)
	}
	ps := processTreeEnvelope("svchost.exe", "ps")
	if ps["kind"] != "last_ps_snapshot" || ps["source"] != "ps" {
		t.Fatalf("ps fallback envelope=%#v", ps)
	}
}

func TestHandleGetProcessTreePrefersTreeSnapshot(t *testing.T) {
	s := newAgentTestServer(t)
	if err := s.db.Create(&db.Implant{ID: "a1", Hostname: "box"}).Error; err != nil {
		t.Fatalf("seed implant: %v", err)
	}
	if err := s.db.Create(&db.Task{AgentID: "a1", Type: "ps", Status: "completed", Result: "svchost.exe"}).Error; err != nil {
		t.Fatalf("seed ps: %v", err)
	}
	if err := s.db.Create(&db.Task{AgentID: "a1", Type: "process_tree", Status: "completed", Result: "1\t0\tsystem\tsystemd"}).Error; err != nil {
		t.Fatalf("seed tree: %v", err)
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
		Live      bool   `json:"live"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v body=%s", err, w.Body.String())
	}
	if resp.Live {
		t.Fatal("handler claimed a live process tree")
	}
	if resp.Kind != "process_tree" || resp.Source != "process_tree" {
		t.Fatalf("envelope: %+v", resp)
	}
	if resp.Processes != "1\t0\tsystem\tsystemd" {
		t.Fatalf("processes=%q", resp.Processes)
	}
}

func TestHandleGetProcessTreeFallsBackToPS(t *testing.T) {
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
		Kind   string `json:"kind"`
		Source string `json:"source"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v", err)
	}
	if resp.Kind != "last_ps_snapshot" || resp.Source != "ps" {
		t.Fatalf("envelope: %+v", resp)
	}
}

func TestHandleGetProcessTreeRequiresCompletedSnapshot(t *testing.T) {
	s := newAgentTestServer(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "missing"}}
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/agents/missing/process-tree", nil)

	s.handleGetProcessTree(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d; body=%s", w.Code, w.Body.String())
	}
}
