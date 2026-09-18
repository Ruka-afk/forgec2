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
	"github.com/forgec2/forgec2/internal/scripting"
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

// TestAutomationTargetCrossTenantBlocked (P0-2) proves an action's
// params.agent_id override cannot redirect a task into another tenant:
// the fire gate (ruleMayFireOn) only validated the EVENT's agent.
func TestAutomationTargetCrossTenantBlocked(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "hp-own", 1)
	seedTenantAgent(t, s, "hp-foreign", 2)
	rule := db.AutomationRule{
		ID: "hp-xredir-rule", Name: "xredir", Enabled: true,
		EventType: string(EventImplantCheckin),
		Actions:   `[{"type":"command","params":{"command":"whoami","agent_id":"hp-foreign"}}]`,
		TenantID:  1,
	}
	if err := s.db.Create(&rule).Error; err != nil {
		t.Fatalf("seed rule: %v", err)
	}
	s.invalidateAutomationCache()

	s.dispatchEvent(Event{Type: EventImplantCheckin, AgentID: "hp-own"}, false)

	if n := taskCountFor(t, s, "hp-foreign"); n != 0 {
		t.Fatalf("cross-tenant redirect fired: %d tasks on foreign agent, want 0", n)
	}
	var blocked int64
	s.db.Model(&db.AuditLog{}).Where("action = ?", "automation_blocked_cross_tenant").Count(&blocked)
	if blocked == 0 {
		t.Fatalf("no automation_blocked_cross_tenant audit row")
	}
}

// TestAutomationTargetSameTenantOK guards the gate: an explicit same-tenant
// params.agent_id still delivers.
func TestAutomationTargetSameTenantOK(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "hp-evt", 1)
	seedTenantAgent(t, s, "hp-target", 1)
	rule := db.AutomationRule{
		ID: "hp-ownredir-rule", Name: "ownredir", Enabled: true,
		EventType: string(EventImplantCheckin),
		Actions:   `[{"type":"command","params":{"command":"whoami","agent_id":"hp-target"}}]`,
		TenantID:  1,
	}
	if err := s.db.Create(&rule).Error; err != nil {
		t.Fatalf("seed rule: %v", err)
	}
	s.invalidateAutomationCache()

	s.dispatchEvent(Event{Type: EventImplantCheckin, AgentID: "hp-evt"}, false)

	if n := taskCountFor(t, s, "hp-target"); n != 1 {
		t.Fatalf("same-tenant redirect did not fire: %d tasks, want 1", n)
	}
}

// TestAutomationRunMacroCrossTenantBlocked proves a rule cannot execute
// another tenant's macro playbook by ID.
func TestAutomationRunMacroCrossTenantBlocked(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "hm-agent", 1)
	foreign := db.CommandMacro{Name: "hm-foreign-macro", Steps: `[{"command":"whoami"}]`, TenantID: 2}
	if err := s.db.Create(&foreign).Error; err != nil {
		t.Fatalf("seed macro: %v", err)
	}
	rule := db.AutomationRule{
		ID: "hm-macro-rule", Name: "macrorule", Enabled: true,
		EventType: string(EventImplantCheckin),
		Actions:   `[{"type":"run_macro","params":{"macro_id":` + uitoa(foreign.ID) + `}}]`,
		TenantID:  1,
	}
	if err := s.db.Create(&rule).Error; err != nil {
		t.Fatalf("seed rule: %v", err)
	}
	s.invalidateAutomationCache()

	s.dispatchEvent(Event{Type: EventImplantCheckin, AgentID: "hm-agent"}, false)

	var runs int64
	s.db.Model(&db.MacroRun{}).Where("macro_id = ?", foreign.ID).Count(&runs)
	if runs != 0 {
		t.Fatalf("foreign macro executed: %d runs, want 0", runs)
	}
}

// TestAutomationRunMacroSameTenantOK guards the gate: an own-tenant macro
// still runs on the triggering agent.
func TestAutomationRunMacroSameTenantOK(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "hm-agent-ok", 1)
	own := db.CommandMacro{Name: "hm-own-macro", Steps: `[{"command":"whoami"}]`, TenantID: 1}
	if err := s.db.Create(&own).Error; err != nil {
		t.Fatalf("seed macro: %v", err)
	}
	rule := db.AutomationRule{
		ID: "hm-macro-rule-ok", Name: "macroruleok", Enabled: true,
		EventType: string(EventImplantCheckin),
		Actions:   `[{"type":"run_macro","params":{"macro_id":` + uitoa(own.ID) + `}}]`,
		TenantID:  1,
	}
	if err := s.db.Create(&rule).Error; err != nil {
		t.Fatalf("seed rule: %v", err)
	}
	s.invalidateAutomationCache()

	s.dispatchEvent(Event{Type: EventImplantCheckin, AgentID: "hm-agent-ok"}, false)

	var runs int64
	s.db.Model(&db.MacroRun{}).Where("macro_id = ?", own.ID).Count(&runs)
	if runs != 1 {
		t.Fatalf("own macro did not run: %d runs, want 1", runs)
	}
}

// TestScriptBridgeTenantScope (P0-1) proves the goja bridge honors the
// caller tenant on every data path automation scripts can reach.
func TestScriptBridgeTenantScope(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "hs-own", 1)
	seedTenantAgent(t, s, "hs-foreign", 2)
	br := &scriptingBridge{s: s}
	// Admin role isolates the tenant logic from role-permission seeding.
	scoped := scripting.Caller{Username: "t", Role: db.RoleAdmin, TenantID: 1}

	agents, err := br.Query(scoped, "agents", map[string]interface{}{})
	if err != nil {
		t.Fatalf("query agents: %v", err)
	}
	if rows, ok := agents.([]map[string]interface{}); !ok || len(rows) != 1 || rows[0]["id"] != "hs-own" {
		t.Fatalf("query agents leaked: %+v", agents)
	}
	if n, err := br.Query(scoped, "count_agents", map[string]interface{}{}); err != nil || n != int64(1) {
		t.Fatalf("count_agents=%v err=%v, want 1", n, err)
	}
	if _, err := br.GetAgent(scoped, "hs-foreign"); err == nil {
		t.Fatalf("GetAgent returned foreign agent")
	}
	if _, err := br.SendTask(scoped, "hs-foreign", "shell", "whoami"); err == nil {
		t.Fatalf("SendTask dispatched to foreign agent")
	}
	listed, err := br.ListAgents(scoped)
	if err != nil || len(listed) != 1 || listed[0]["id"] != "hs-own" {
		t.Fatalf("ListAgents leaked: %+v err=%v", listed, err)
	}
	// Legacy callers (tenant 0) keep the global view.
	legacy := scripting.Caller{Username: "t", Role: db.RoleAdmin}
	if n, err := br.Query(legacy, "count_agents", map[string]interface{}{}); err != nil || n != int64(2) {
		t.Fatalf("legacy count_agents=%v err=%v, want 2", n, err)
	}
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
