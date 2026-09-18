package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func seedAutomationRule(t *testing.T, s *Server, id string, tenantID uint, eventType string) {
	t.Helper()
	rule := db.AutomationRule{
		ID:        id,
		Name:      "rule-" + id,
		Enabled:   true,
		EventType: eventType,
		Actions:   `[{"type":"command","params":{"command":"whoami"}}]`,
		TenantID:  tenantID,
	}
	if err := s.db.Create(&rule).Error; err != nil {
		t.Fatalf("seed rule: %v", err)
	}
	s.invalidateAutomationCache()
}

func taskCountFor(t *testing.T, s *Server, agentID string) int64 {
	t.Helper()
	var n int64
	if err := s.db.Model(&db.Task{}).Where("agent_id = ?", agentID).Count(&n).Error; err != nil {
		t.Fatalf("count tasks: %v", err)
	}
	return n
}

// TestAutomationTriggerCrossTenantBlocked proves a tenant-2 rule never fires
// for a tenant-1 agent's event (previously the engine matched event type
// only, so a foreign rule's webhook/command actions ran on our agents).
func TestAutomationTriggerCrossTenantBlocked(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "hx-agent", 1)
	seedAutomationRule(t, s, "hx-foreign-rule", 2, string(EventImplantCheckin))

	s.dispatchEvent(Event{Type: EventImplantCheckin, AgentID: "hx-agent"}, false)

	if n := taskCountFor(t, s, "hx-agent"); n != 0 {
		t.Fatalf("foreign rule fired: %d tasks created, want 0", n)
	}
}

// TestAutomationTriggerSameTenantFires guards the gate: an own-tenant rule
// still fires for its tenant's agent events.
func TestAutomationTriggerSameTenantFires(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "hx-agent-ok", 1)
	seedAutomationRule(t, s, "hx-own-rule", 1, string(EventImplantCheckin))

	s.dispatchEvent(Event{Type: EventImplantCheckin, AgentID: "hx-agent-ok"}, false)

	if n := taskCountFor(t, s, "hx-agent-ok"); n != 1 {
		t.Fatalf("own rule did not fire: %d tasks, want 1", n)
	}
}

// TestAutomationTriggerLegacyFires guards backward compat: tenant-0 rules
// still fire for tenant-0 (pre-tenant) agents.
func TestAutomationTriggerLegacyFires(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "hx-agent-legacy", 0)
	seedAutomationRule(t, s, "hx-legacy-rule", 0, string(EventImplantCheckin))

	s.dispatchEvent(Event{Type: EventImplantCheckin, AgentID: "hx-agent-legacy"}, false)

	if n := taskCountFor(t, s, "hx-agent-legacy"); n != 1 {
		t.Fatalf("legacy rule did not fire: %d tasks, want 1", n)
	}
}

func uitoa(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}

func automationCtx(s *Server, t *testing.T, method, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	c, w := tenantScopedAdminContext(s, t, "hx-viewer", 1)
	if body != "" {
		c.Request, _ = http.NewRequest(method, "/", bytes.NewReader([]byte(body)))
		c.Request.Header.Set("Content-Type", "application/json")
	} else {
		c.Request, _ = http.NewRequest(method, "/", nil)
	}
	return c, w
}

func decodeAutomationList(t *testing.T, w *httptest.ResponseRecorder) []map[string]interface{} {
	t.Helper()
	var resp struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode list: %v body=%s", err, w.Body.String())
	}
	return resp.Data
}

// TestAutomationRuleListScoped proves the rules API hides foreign rows.
func TestAutomationRuleListScoped(t *testing.T) {
	s := mustTenantServer(t)
	seedAutomationRule(t, s, "hx-own-list", 1, string(EventImplantCheckin))
	seedAutomationRule(t, s, "hx-foreign-list", 2, string(EventImplantCheckin))

	c, w := automationCtx(s, t, http.MethodGet, "")
	s.handleListAutomationRules(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", w.Code, w.Body.String())
	}
	rows := decodeAutomationList(t, w)
	if len(rows) != 1 {
		t.Fatalf("listed %d rules, want 1 (own only)", len(rows))
	}
	if rows[0]["id"] != "hx-own-list" {
		t.Fatalf("listed rule id=%v, want hx-own-list", rows[0]["id"])
	}
}

// TestAutomationRuleSaveStampsTenant proves created rules inherit the
// operator's tenant and the body cannot spoof it.
func TestAutomationRuleSaveStampsTenant(t *testing.T) {
	s := mustTenantServer(t)
	body := `{"name":"hx-stamped","enabled":true,"event_type":"` + string(EventImplantCheckin) +
		`","tenant_id":2,"actions":[{"type":"notify","params":{"message":"x"}}]}`
	c, w := automationCtx(s, t, http.MethodPost, body)
	s.handleSaveAutomationRule(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", w.Code, w.Body.String())
	}
	var stored db.AutomationRule
	if err := s.db.First(&stored, "name = ?", "hx-stamped").Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.TenantID != 1 {
		t.Fatalf("stored tenant=%d, want 1 (body spoof tenant_id:2 ignored)", stored.TenantID)
	}
}

// TestAutomationRuleMutateCrossTenant404 proves update/toggle/delete on a
// foreign rule are invisible (404) and leave the row untouched.
func TestAutomationRuleMutateCrossTenant404(t *testing.T) {
	s := mustTenantServer(t)
	seedAutomationRule(t, s, "hx-foreign-mut", 2, string(EventImplantCheckin))

	body := `{"name":"hx-foreign-mut","enabled":false,"event_type":"` + string(EventImplantCheckin) + `"}`
	c, w := automationCtx(s, t, http.MethodPut, body)
	c.Params = gin.Params{{Key: "id", Value: "hx-foreign-mut"}}
	s.handleUpdateAutomationRule(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("update status=%d, want 404", w.Code)
	}

	c, w = automationCtx(s, t, http.MethodPost, "")
	c.Params = gin.Params{{Key: "id", Value: "hx-foreign-mut"}}
	s.handleToggleAutomationRule(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("toggle status=%d, want 404", w.Code)
	}

	c, w = automationCtx(s, t, http.MethodDelete, "")
	c.Params = gin.Params{{Key: "id", Value: "hx-foreign-mut"}}
	s.handleDeleteAutomationRule(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("delete status=%d, want 404", w.Code)
	}

	var stored db.AutomationRule
	if err := s.db.First(&stored, "id = ?", "hx-foreign-mut").Error; err != nil {
		t.Fatalf("foreign rule vanished: %v", err)
	}
	if !stored.Enabled {
		t.Fatalf("foreign rule was toggled cross-tenant")
	}
}

// TestListenerCrossTenantInvisible proves listener rows are scoped on
// list, detail, and mutation entry points.
func TestListenerCrossTenantInvisible(t *testing.T) {
	s := mustTenantServer(t)
	foreign := db.Listener{Name: "hx-foreign-lis", Scheme: "http", Type: "http", Host: "127.0.0.1", Port: 18091, Enabled: true, TenantID: 2}
	if err := s.db.Create(&foreign).Error; err != nil {
		t.Fatalf("seed listener: %v", err)
	}

	c, w := automationCtx(s, t, http.MethodGet, "")
	s.handleListListeners(c)
	if w.Code != http.StatusOK {
		t.Fatalf("list status=%d, want 200", w.Code)
	}
	if strings.Contains(w.Body.String(), "hx-foreign-lis") {
		t.Fatalf("foreign listener leaked into list: %s", w.Body.String())
	}

	c2, w2 := automationCtx(s, t, http.MethodGet, "")
	c2.Params = gin.Params{{Key: "id", Value: uitoa(foreign.ID)}}
	s.handleAPIGetListener(c2)
	if w2.Code != http.StatusNotFound {
		t.Fatalf("get status=%d, want 404", w2.Code)
	}

	c3, w3 := automationCtx(s, t, http.MethodDelete, "")
	c3.Params = gin.Params{{Key: "id", Value: uitoa(foreign.ID)}}
	s.handleDeleteListener(c3)
	if w3.Code != http.StatusNotFound {
		t.Fatalf("delete status=%d, want 404", w3.Code)
	}
	var kept db.Listener
	if err := s.db.First(&kept, foreign.ID).Error; err != nil {
		t.Fatalf("foreign listener deleted cross-tenant: %v", err)
	}
}

// TestMacroCrossTenantInvisible proves macro rows are scoped on list and
// the mutation entry points.
func TestMacroCrossTenantInvisible(t *testing.T) {
	s := mustTenantServer(t)
	foreign := db.CommandMacro{Name: "hx-foreign-macro", Steps: "[]", TenantID: 2}
	if err := s.db.Create(&foreign).Error; err != nil {
		t.Fatalf("seed macro: %v", err)
	}

	c, w := automationCtx(s, t, http.MethodGet, "")
	s.handleListMacros(c)
	if w.Code != http.StatusOK {
		t.Fatalf("list status=%d, want 200", w.Code)
	}
	if strings.Contains(w.Body.String(), "hx-foreign-macro") {
		t.Fatalf("foreign macro leaked into list: %s", w.Body.String())
	}

	c2, w2 := automationCtx(s, t, http.MethodPut, `{"name":"x","steps":[]}`)
	c2.Params = gin.Params{{Key: "id", Value: uitoa(foreign.ID)}}
	s.handleUpdateMacro(c2)
	if w2.Code != http.StatusNotFound {
		t.Fatalf("update status=%d, want 404", w2.Code)
	}

	c3, w3 := automationCtx(s, t, http.MethodDelete, "")
	c3.Params = gin.Params{{Key: "id", Value: uitoa(foreign.ID)}}
	s.handleDeleteMacro(c3)
	if w3.Code != http.StatusNotFound {
		t.Fatalf("delete status=%d, want 404", w3.Code)
	}
	var kept db.CommandMacro
	if err := s.db.First(&kept, foreign.ID).Error; err != nil {
		t.Fatalf("foreign macro deleted cross-tenant: %v", err)
	}
}
