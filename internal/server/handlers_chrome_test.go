package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/pkg/protocol"
	"github.com/gin-gonic/gin"
)

const chromeTestUUID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

func chromeTestServer(t *testing.T) *Server {
	t.Helper()
	s := newAgentTestServer(t)
	s.agentPendingTasks = make(map[string]int)
	s.metrics = NewMetricsCollector(s)
	s.eventManager = NewEventManager(s.db)
	t.Cleanup(func() { s.eventManager.Shutdown() })
	return s
}

func TestChromeBeaconRegistersAndDispatches(t *testing.T) {
	s := chromeTestServer(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `{"uuid":"` + chromeTestUUID + `","info":{"platform":"Win32","browser":"Chrome","language":"en"}}`
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/chrome/beacon", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	s.handleChromeBeacon(c)
	if w.Code != http.StatusOK {
		t.Fatalf("register beacon: %d %s", w.Code, w.Body.String())
	}

	var agent db.Implant
	if err := s.db.First(&agent, "id = ?", chromeTestUUID).Error; err != nil {
		t.Fatalf("agent not persisted: %v", err)
	}
	if !s.isChromeAgentKind(nil, chromeTestUUID) {
		t.Fatalf("agent tags=%q, want chrome", agent.Tags)
	}

	task, err := s.createTask(chromeTestUUID, protocol.TaskTypeChromeTabs, "", "", "", "", 0, 0)
	if err != nil {
		t.Fatalf("create chrome task: %v", err)
	}

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/chrome/beacon", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	s.handleChromeBeacon(c)
	if w.Code != http.StatusOK {
		t.Fatalf("dispatch beacon: %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Tasks []chromeWireTask `json:"tasks"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v body=%s", err, w.Body.String())
	}
	if len(resp.Tasks) != 1 || resp.Tasks[0].ID != task.ID || resp.Tasks[0].Type != protocol.TaskTypeChromeTabs {
		t.Fatalf("tasks=%+v", resp.Tasks)
	}

	resultBody := `{"uuid":"` + chromeTestUUID + `","info":{"platform":"Win32"},"results":[{"task_id":` +
		jsonUint(task.ID) + `,"type":"chrome_tabs","output":"[tab]"}]}`
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/chrome/beacon", bytes.NewBufferString(resultBody))
	c.Request.Header.Set("Content-Type", "application/json")
	s.handleChromeBeacon(c)
	if w.Code != http.StatusOK {
		t.Fatalf("result beacon: %d %s", w.Code, w.Body.String())
	}
	var stored db.Task
	if err := s.db.First(&stored, task.ID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if stored.Status != "completed" || stored.Result != "[tab]" {
		t.Fatalf("status=%s result=%q", stored.Status, stored.Result)
	}
}

func TestChromeBeaconRejectsInvalidUUID(t *testing.T) {
	s := chromeTestServer(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/chrome/beacon", bytes.NewBufferString(`{"uuid":"not-a-uuid"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	s.handleChromeBeacon(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("got %d want 400", w.Code)
	}
}

func TestChromeBeaconTokenRequiredWhenConfigured(t *testing.T) {
	s := chromeTestServer(t)
	s.cfg.Server.BeaconKey = "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
	token := s.chromeExtensionToken()
	if token == "" {
		t.Fatal("expected derived chrome token")
	}

	body := `{"uuid":"` + chromeTestUUID + `","info":{"platform":"Win32"}}`
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/chrome/beacon", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	s.handleChromeBeacon(c)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("missing token: got %d want 401", w.Code)
	}

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/chrome/beacon", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("X-ForgeC2-Chrome-Token", token)
	s.handleChromeBeacon(c)
	if w.Code != http.StatusOK {
		t.Fatalf("valid token: got %d %s", w.Code, w.Body.String())
	}
}

func TestCreateTaskRejectsImplantTypesOnChromeAgent(t *testing.T) {
	s := chromeTestServer(t)
	if err := s.db.Create(&db.Implant{ID: chromeTestUUID, Hostname: "ext", Tags: "chrome"}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := s.createTask(chromeTestUUID, "shell", "whoami", "", "", "", 0, 0); err == nil {
		t.Fatal("expected shell on chrome agent to fail")
	}
}

func TestCreateTaskRejectsChromeTypesOnImplant(t *testing.T) {
	s := chromeTestServer(t)
	if err := s.db.Create(&db.Implant{ID: chromeTestUUID, Hostname: "box"}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := s.createTask(chromeTestUUID, protocol.TaskTypeChromeTabs, "", "", "", "", 0, 0); err == nil {
		t.Fatal("expected chrome_tabs on implant to fail")
	}
}

func jsonUint(n uint) string {
	b, _ := json.Marshal(n)
	return string(b)
}
