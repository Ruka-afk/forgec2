package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// ── Helpers ─────────────────────────────────────────────────────────────────

// alertCtx builds a tenant-scoped operator context for the alert API.
func alertCtx(s *Server, t *testing.T, username, role string, tenantID uint) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	c, w := tenantScopedAdminContext(s, t, username, tenantID)
	c.Set("user_role", role)
	return c, w
}

// alertCtxFor pins the :id path parameter used by the rule/alert detail routes.
func alertCtxFor(s *Server, t *testing.T, username, role string, tenantID uint, id uint) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	c, w := alertCtx(s, t, username, role, tenantID)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(id)}}
	return c, w
}

func alertJSONCtx(s *Server, t *testing.T, username, role string, tenantID uint, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	c, w := alertCtx(s, t, username, role, tenantID)
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, w
}

func seedAlertRule(t *testing.T, s *Server, tenantID uint, name, ruleType string, enabled bool) db.AlertRule {
	t.Helper()
	rule := db.AlertRule{Name: name, Type: ruleType, Threshold: 90, Enabled: enabled, TenantID: tenantID}
	if err := s.db.Create(&rule).Error; err != nil {
		t.Fatalf("seed rule: %v", err)
	}
	return rule
}

func seedAlert(t *testing.T, s *Server, tenantID uint, ruleID uint, severity, source, sourceName string) db.Alert {
	t.Helper()
	alert := db.Alert{
		TenantID: tenantID, RuleID: ruleID, Type: "agent_offline", Severity: severity,
		Title: "agent offline", Message: "gone", Source: source, SourceName: sourceName,
		Status: "active",
	}
	if err := s.db.Create(&alert).Error; err != nil {
		t.Fatalf("seed alert: %v", err)
	}
	return alert
}

func alertCount(t *testing.T, s *Server, where string, args ...interface{}) int64 {
	t.Helper()
	q := s.db.Model(&db.Alert{})
	if where != "" {
		q = q.Where(where, args...)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		t.Fatalf("count alerts: %v", err)
	}
	return n
}

// ── Rule isolation ──────────────────────────────────────────────────────────

// TestAlertRuleListIsTenantScoped proves the rule list and its count stay inside
// the caller's tenant. Rules name the thresholds an operator watches, so a
// global list disclosed another tenant's monitoring posture.
func TestAlertRuleListIsTenantScoped(t *testing.T) {
	s := mustTenantServer(t)
	seedAlertRule(t, s, 1, "mine", "cpu_high", true)
	seedAlertRule(t, s, 2, "theirs", "agent_offline", true)
	seedAlertRule(t, s, 0, "legacy", "memory_high", false)

	c, w := alertCtx(s, t, "al-op", "operator", 1)
	s.handleGetAlertRules(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var body struct {
		Rules []db.AlertRule `json:"rules"`
		Total int64          `json:"total"`
	}
	unmarshalData(t, w.Body.Bytes(), &body)
	if body.Total != 1 || len(body.Rules) != 1 || body.Rules[0].Name != "mine" {
		t.Fatalf("rules = %+v total = %d, want only the caller's own rule", body.Rules, body.Total)
	}

	// An unscoped operator keeps the fleet-wide view.
	c2, w2 := alertCtx(s, t, "al-global", "operator", 0)
	s.handleGetAlertRules(c2)
	var global struct {
		Total int64 `json:"total"`
	}
	unmarshalData(t, w2.Body.Bytes(), &global)
	if global.Total != 3 {
		t.Fatalf("unscoped operator sees %d rules, want all 3", global.Total)
	}
}

// TestAlertRuleCreateStampsCallerTenant proves a created rule is owned by the
// caller and that the body cannot dictate its identity.
func TestAlertRuleCreateStampsCallerTenant(t *testing.T) {
	s := mustTenantServer(t)

	// A body smuggling id/tenant_id must not stick: the allowlist bind drops
	// both and the tenant comes from the authenticated operator.
	body := `{"id":4242,"tenant_id":99,"name":"cpu","type":"cpu_high","threshold":85}`
	c, w := alertJSONCtx(s, t, "al-op", "operator", 7, body)
	s.handleCreateAlertRule(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	var created struct {
		Rule db.AlertRule `json:"rule"`
	}
	unmarshalData(t, w.Body.Bytes(), &created)
	if created.Rule.ID == 4242 {
		t.Fatal("client-supplied id was honoured")
	}
	var stored db.AlertRule
	if err := s.db.First(&stored, "id = ?", created.Rule.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.TenantID != 7 {
		t.Fatalf("TenantID = %d, want the operator's tenant 7 (body said 99)", stored.TenantID)
	}
	if stored.Name != "cpu" || stored.Type != "cpu_high" || stored.Threshold != 85 {
		t.Fatalf("stored rule = %+v", stored)
	}
	// Omitting "enabled" must not silently create a disarmed rule.
	if !stored.Enabled {
		t.Fatal("omitted enabled defaulted to false; the model default is true")
	}
}

// TestAlertRuleCreateHonoursExplicitEnabled pins that an explicit false is
// still respected — the pointer default must not override a deliberate choice.
func TestAlertRuleCreateHonoursExplicitEnabled(t *testing.T) {
	s := mustTenantServer(t)

	c, w := alertJSONCtx(s, t, "al-op", "operator", 1, `{"name":"m","type":"memory_high","enabled":false}`)
	s.handleCreateAlertRule(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	var stored db.AlertRule
	if err := s.db.First(&stored, "name = ?", "m").Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.Enabled {
		t.Fatal("explicit enabled=false was overridden")
	}
}

// TestAlertRuleCreateRejectsIncompleteBody proves the required-field contract
// still holds after the bind was narrowed.
func TestAlertRuleCreateRejectsIncompleteBody(t *testing.T) {
	s := mustTenantServer(t)
	for _, body := range []string{`{}`, `{"name":"x"}`, `{"type":"cpu_high"}`, `{"name":"","type":""}`, `{"name":`} {
		c, w := alertJSONCtx(s, t, "al-op", "operator", 1, body)
		s.handleCreateAlertRule(c)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("body %s: status = %d, want 400", body, w.Code)
		}
	}
	var n int64
	if err := s.db.Model(&db.AlertRule{}).Count(&n).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("a rejected create still stored %d rules", n)
	}
}

// TestAlertRuleMutationsAreTenantScoped proves update and delete both refuse a
// foreign rule and leave it armed.
func TestAlertRuleMutationsAreTenantScoped(t *testing.T) {
	s := mustTenantServer(t)
	foreign := seedAlertRule(t, s, 2, "theirs", "agent_offline", true)

	c, w := alertCtxFor(s, t, "al-op", "operator", 1, foreign.ID)
	c.Request = httptest.NewRequest(http.MethodPut, "/", strings.NewReader(`{"name":"pwned","threshold":1}`))
	c.Request.Header.Set("Content-Type", "application/json")
	s.handleUpdateAlertRule(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("update: status = %d, want 404", w.Code)
	}

	c2, w2 := alertCtxFor(s, t, "al-op", "operator", 1, foreign.ID)
	c2.Request = httptest.NewRequest(http.MethodDelete, "/", nil)
	s.handleDeleteAlertRule(c2)
	if w2.Code != http.StatusNotFound {
		t.Fatalf("delete: status = %d, want 404", w2.Code)
	}

	var survivor db.AlertRule
	if err := s.db.First(&survivor, "id = ?", foreign.ID).Error; err != nil {
		t.Fatalf("rule was destroyed by a refused request: %v", err)
	}
	if survivor.Name != "theirs" || !survivor.Enabled || survivor.Threshold != 90 {
		t.Fatalf("refused requests mutated the rule: %+v", survivor)
	}
}

// TestAlertRuleDeleteReportsPhantomSuccess proves deleting an id that does not
// exist answers 404 instead of a bare 200 — the operator must not be told an
// armed rule is gone when nothing was deleted.
func TestAlertRuleDeleteReportsPhantomSuccess(t *testing.T) {
	s := mustTenantServer(t)

	for _, id := range []uint{999999} {
		c, w := alertCtxFor(s, t, "al-op", "operator", 1, id)
		c.Request = httptest.NewRequest(http.MethodDelete, "/", nil)
		s.handleDeleteAlertRule(c)
		if w.Code != http.StatusNotFound {
			t.Fatalf("ghost id %d: status = %d, want 404", id, w.Code)
		}
	}
}

// TestAlertRuleUpdateAppliesWithinTenant proves the owner's update still works
// and keeps the rule in its own tenant.
func TestAlertRuleUpdateAppliesWithinTenant(t *testing.T) {
	s := mustTenantServer(t)
	rule := seedAlertRule(t, s, 4, "cpu", "cpu_high", true)

	c, w := alertCtxFor(s, t, "al-op", "operator", 4, rule.ID)
	c.Request = httptest.NewRequest(http.MethodPut, "/", strings.NewReader(`{"name":"cpu tuned","threshold":42.5,"enabled":false,"description":"d"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	s.handleUpdateAlertRule(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	var stored db.AlertRule
	if err := s.db.First(&stored, "id = ?", rule.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.Name != "cpu tuned" || stored.Threshold != 42.5 || stored.Enabled || stored.Description != "d" {
		t.Fatalf("stored rule = %+v", stored)
	}
	if stored.TenantID != 4 {
		t.Fatalf("TenantID drifted to %d", stored.TenantID)
	}

	// An empty name/description keeps the existing values.
	c2, w2 := alertCtxFor(s, t, "al-op", "operator", 4, rule.ID)
	c2.Request = httptest.NewRequest(http.MethodPut, "/", strings.NewReader(`{"name":"","description":"","threshold":50,"enabled":true}`))
	c2.Request.Header.Set("Content-Type", "application/json")
	s.handleUpdateAlertRule(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("partial update: %d %s", w2.Code, w2.Body.String())
	}
	if err := s.db.First(&stored, "id = ?", rule.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.Name != "cpu tuned" || stored.Description != "d" {
		t.Fatalf("an empty field overwrote stored values: %+v", stored)
	}
}

// ── Alert isolation ─────────────────────────────────────────────────────────

// TestAlertListIsTenantScoped proves the alert feed stays inside the caller's
// tenant. Alerts embed SourceName (the host's hostname), so a global feed
// enumerated every other tenant's machines.
func TestAlertListIsTenantScoped(t *testing.T) {
	s := mustTenantServer(t)
	rule := seedAlertRule(t, s, 1, "offline", "agent_offline", true)
	seedAlert(t, s, 1, rule.ID, "critical", "mine-agent", "victim-pc-1")
	seedAlert(t, s, 2, rule.ID, "critical", "their-agent", "victim-pc-2")

	c, w := alertCtx(s, t, "al-op", "operator", 1)
	s.handleGetAlerts(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "victim-pc-2") || strings.Contains(w.Body.String(), "their-agent") {
		t.Fatalf("another tenant's host leaked into the feed: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "victim-pc-1") {
		t.Fatalf("own alert missing from the feed: %s", w.Body.String())
	}
}

// TestAlertListFiltersCombine proves the status/severity filters still apply on
// top of the tenant predicate rather than replacing it.
func TestAlertListFiltersCombine(t *testing.T) {
	s := mustTenantServer(t)
	rule := seedAlertRule(t, s, 1, "offline", "agent_offline", true)
	seedAlert(t, s, 1, rule.ID, "critical", "a1", "host-a")
	seedAlert(t, s, 1, rule.ID, "warning", "a2", "host-b")
	other := seedAlert(t, s, 2, rule.ID, "critical", "a3", "host-c")
	if err := s.db.Model(&db.Alert{}).Where("id = ?", other.ID).Update("status", "resolved").Error; err != nil {
		t.Fatalf("resolve foreign alert: %v", err)
	}

	c, w := alertCtx(s, t, "al-op", "operator", 1)
	c.Request = httptest.NewRequest(http.MethodGet, "/?status=active&severity=critical", nil)
	s.handleGetAlerts(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "host-b") {
		t.Fatalf("severity filter ignored: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "host-a") {
		t.Fatalf("matching alert missing: %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "host-c") {
		t.Fatalf("another tenant's alert leaked through the filters: %s", w.Body.String())
	}
}

// TestAlertStatusMutationsAreTenantScoped proves acknowledge and resolve refuse
// a foreign alert and never mutate its status.
func TestAlertStatusMutationsAreTenantScoped(t *testing.T) {
	s := mustTenantServer(t)
	rule := seedAlertRule(t, s, 2, "theirs", "agent_offline", true)
	foreign := seedAlert(t, s, 2, rule.ID, "critical", "their-agent", "victim-pc")

	for name, invoke := range map[string]func(*Server, *gin.Context){
		"acknowledge": (*Server).handleAcknowledgeAlert,
		"resolve":     (*Server).handleResolveAlert,
	} {
		c, w := alertCtxFor(s, t, "al-op", "operator", 1, foreign.ID)
		c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
		invoke(s, c)
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s: status = %d, want 404", name, w.Code)
		}
		if strings.Contains(w.Body.String(), "victim-pc") {
			t.Fatalf("%s leaked the foreign host: %s", name, w.Body.String())
		}
		var stored db.Alert
		if err := s.db.First(&stored, "id = ?", foreign.ID).Error; err != nil {
			t.Fatalf("%s: alert destroyed by a refused request: %v", name, err)
		}
		if stored.Status != "active" {
			t.Fatalf("%s mutated a foreign alert's status to %q", name, stored.Status)
		}
	}
}

// TestAlertStatusMutationsApplyWithinTenant proves the owner's path still works.
func TestAlertStatusMutationsApplyWithinTenant(t *testing.T) {
	s := mustTenantServer(t)
	rule := seedAlertRule(t, s, 1, "offline", "agent_offline", true)
	alert := seedAlert(t, s, 1, rule.ID, "critical", "a1", "host-a")

	c, w := alertCtxFor(s, t, "al-op", "operator", 1, alert.ID)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	s.handleAcknowledgeAlert(c)
	if w.Code != http.StatusOK {
		t.Fatalf("ack: %d %s", w.Code, w.Body.String())
	}
	var stored db.Alert
	if err := s.db.First(&stored, "id = ?", alert.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.Status != "acknowledged" {
		t.Fatalf("status = %q, want acknowledged", stored.Status)
	}

	c2, w2 := alertCtxFor(s, t, "al-op", "operator", 1, alert.ID)
	c2.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	s.handleResolveAlert(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("resolve: %d %s", w2.Code, w2.Body.String())
	}
	if err := s.db.First(&stored, "id = ?", alert.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.Status != "resolved" {
		t.Fatalf("status = %q, want resolved", stored.Status)
	}
}

// ── Stats scoping ───────────────────────────────────────────────────────────

// TestAlertStatsAreTenantScoped proves the raw-SQL aggregate carries the tenant
// predicate. Raw().Scan() ignores an appended Where, so this endpoint had no
// scoping at all and leaked fleet-wide counters.
func TestAlertStatsAreTenantScoped(t *testing.T) {
	s := mustTenantServer(t)
	rule := seedAlertRule(t, s, 1, "offline", "agent_offline", true)
	seedAlert(t, s, 1, rule.ID, "critical", "a1", "host-a")
	seedAlert(t, s, 1, rule.ID, "warning", "a2", "host-b")
	resolved := seedAlert(t, s, 1, rule.ID, "info", "a3", "host-c")
	if err := s.db.Model(&db.Alert{}).Where("id = ?", resolved.ID).Update("status", "resolved").Error; err != nil {
		t.Fatalf("resolve: %v", err)
	}
	seedAlert(t, s, 2, rule.ID, "critical", "b1", "host-x")
	seedAlert(t, s, 2, rule.ID, "critical", "b2", "host-y")

	c, w := alertCtx(s, t, "al-op", "operator", 1)
	s.handleGetAlertStats(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var stats struct {
		Active   int64 `json:"active"`
		Critical int64 `json:"critical"`
		Warning  int64 `json:"warning"`
		Info     int64 `json:"info"`
	}
	unmarshalData(t, w.Body.Bytes(), &stats)
	// Only tenant 1's two still-active alerts; the resolved one and both of
	// tenant 2's alerts must be excluded.
	if stats.Active != 2 || stats.Critical != 1 || stats.Warning != 1 || stats.Info != 0 {
		t.Fatalf("stats = %+v, want active=2 critical=1 warning=1 info=0", stats)
	}

	// The unscoped operator keeps the fleet-wide numbers.
	c2, w2 := alertCtx(s, t, "al-global", "operator", 0)
	s.handleGetAlertStats(c2)
	var global struct {
		Active int64 `json:"active"`
	}
	unmarshalData(t, w2.Body.Bytes(), &global)
	if global.Active != 4 {
		t.Fatalf("unscoped active = %d, want 4", global.Active)
	}
}

// TestAlertStatsFailClosedWithoutTenant proves a tenant lookup that cannot be
// resolved denies the aggregate instead of returning fleet-wide counters.
func TestAlertStatsFailClosedWithoutTenant(t *testing.T) {
	s := mustTenantServer(t)
	rule := seedAlertRule(t, s, 1, "offline", "agent_offline", true)
	seedAlert(t, s, 1, rule.ID, "critical", "a1", "host-a")

	c, w := alertCtx(s, t, "al-ghost", "operator", 1)
	// Poison the cached resolution so resolveTenant reports a failed lookup,
	// exactly as a deleted/stale identity row would.
	c.Set(tenantIDContextKey, tenantResolution{ok: false})
	s.handleGetAlertStats(c)
	if w.Code == http.StatusOK {
		t.Fatalf("status = 200 with an unresolvable tenant; body = %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), `"active"`) {
		t.Fatalf("a failed lookup still returned counters: %s", w.Body.String())
	}
}

// ── Collector tenant inheritance ────────────────────────────────────────────

// TestTriggerAlertInheritsAgentTenant is the core regression: alerts were minted
// with no tenant at all, so they looked legacy and were hidden from the very
// tenant whose host went offline.
func TestTriggerAlertInheritsAgentTenant(t *testing.T) {
	s := mustTenantServer(t)
	m := NewMonitorCollector(s)
	rule := seedAlertRule(t, s, 1, "agent offline", "agent_offline", true)
	seedTenantAgent(t, s, "alert-agent", 3)

	m.triggerAlert(&rule, "alert-agent", "victim-pc", "600s", map[string]interface{}{"value": 600})
	var stored db.Alert
	if err := s.db.First(&stored, "source = ?", "alert-agent").Error; err != nil {
		t.Fatalf("alert was not created: %v", err)
	}
	if stored.TenantID != 3 {
		t.Fatalf("TenantID = %d, want the alerting agent's tenant 3", stored.TenantID)
	}
	if stored.Severity != "critical" {
		t.Fatalf("severity = %q, want critical for agent_offline", stored.Severity)
	}
}

// TestTriggerAlertForSystemSourceUsesRuleTenant proves a server-wide alert with
// no agent inherits the rule's tenant rather than guessing one.
func TestTriggerAlertForSystemSourceUsesRuleTenant(t *testing.T) {
	s := mustTenantServer(t)
	m := NewMonitorCollector(s)
	rule := seedAlertRule(t, s, 5, "cpu", "cpu_high", true)

	m.triggerAlert(&rule, "system", "forge-host", "95.0%", map[string]interface{}{"value": 95})
	var stored db.Alert
	if err := s.db.First(&stored, "source = ?", "system").Error; err != nil {
		t.Fatalf("alert was not created: %v", err)
	}
	if stored.TenantID != 5 {
		t.Fatalf("TenantID = %d, want the rule's tenant 5", stored.TenantID)
	}
}

// TestTriggerAlertUnknownAgentFallsBackToRuleTenant proves an alert whose
// source agent no longer exists is still attributed rather than left unattributed.
func TestTriggerAlertUnknownAgentFallsBackToRuleTenant(t *testing.T) {
	s := mustTenantServer(t)
	m := NewMonitorCollector(s)
	rule := seedAlertRule(t, s, 6, "offline", "agent_offline", true)

	m.triggerAlert(&rule, "vanished-agent", "ghost-pc", "600s", map[string]interface{}{"value": 600})
	var stored db.Alert
	if err := s.db.First(&stored, "source = ?", "vanished-agent").Error; err != nil {
		t.Fatalf("alert was not created: %v", err)
	}
	if stored.TenantID != 6 {
		t.Fatalf("TenantID = %d, want the rule's tenant 6 as fallback", stored.TenantID)
	}
}

// TestTriggerAlertDedupeIsPerRuleAndSource proves the existing "don't re-raise an
// active alert" check still suppresses the repeat while letting a genuinely new
// alert through.
func TestTriggerAlertDedupeIsPerRuleAndSource(t *testing.T) {
	s := mustTenantServer(t)
	m := NewMonitorCollector(s)
	rule := seedAlertRule(t, s, 1, "offline", "agent_offline", true)
	seedTenantAgent(t, s, "dedupe-agent", 1)

	m.triggerAlert(&rule, "dedupe-agent", "pc", "600s", nil)
	m.triggerAlert(&rule, "dedupe-agent", "pc", "1200s", nil)
	if got := alertCount(t, s, "source = ?", "dedupe-agent"); got != 1 {
		t.Fatalf("%d alerts raised for one active source, want 1", got)
	}

	// Resolving the alert must re-arm the source.
	var stored db.Alert
	if err := s.db.First(&stored, "source = ?", "dedupe-agent").Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if err := s.db.Model(&db.Alert{}).Where("id = ?", stored.ID).Update("status", "resolved").Error; err != nil {
		t.Fatalf("resolve: %v", err)
	}
	m.triggerAlert(&rule, "dedupe-agent", "pc", "1800s", nil)
	if got := alertCount(t, s, "source = ?", "dedupe-agent"); got != 2 {
		t.Fatalf("%d alerts after re-arming, want 2", got)
	}
}

// TestTriggerAlertSkipsUnmarshalableDetails proves a details payload that cannot
// be marshalled is dropped instead of stored as a half-written alert.
func TestTriggerAlertSkipsUnmarshalableDetails(t *testing.T) {
	s := mustTenantServer(t)
	m := NewMonitorCollector(s)
	rule := seedAlertRule(t, s, 1, "offline", "agent_offline", true)
	seedTenantAgent(t, s, "bad-detail-agent", 1)

	m.triggerAlert(&rule, "bad-detail-agent", "pc", "600s", map[string]interface{}{"chan": make(chan int)})
	if got := alertCount(t, s, "source = ?", "bad-detail-agent"); got != 0 {
		t.Fatalf("%d alerts stored with unmarshalable details, want 0", got)
	}
}

// ── Role gate ───────────────────────────────────────────────────────────────

// TestAlertMutationsRequireOperator proves every mutating alert endpoint keeps
// its role gate after the scoping rewrite.
func TestAlertMutationsRequireOperator(t *testing.T) {
	s := mustTenantServer(t)
	rule := seedAlertRule(t, s, 1, "offline", "agent_offline", true)
	alert := seedAlert(t, s, 1, rule.ID, "critical", "a1", "host-a")

	cases := []struct {
		name   string
		method string
		id     uint
		body   string
		invoke func(*Server, *gin.Context)
	}{
		{"create", http.MethodPost, 0, `{"name":"x","type":"cpu_high"}`, (*Server).handleCreateAlertRule},
		{"update", http.MethodPut, rule.ID, `{"name":"x"}`, (*Server).handleUpdateAlertRule},
		{"delete", http.MethodDelete, rule.ID, "", (*Server).handleDeleteAlertRule},
		{"acknowledge", http.MethodPost, alert.ID, "", (*Server).handleAcknowledgeAlert},
		{"resolve", http.MethodPost, alert.ID, "", (*Server).handleResolveAlert},
	}
	for _, tc := range cases {
		c, w := alertCtxFor(s, t, "al-viewer", "viewer", 1, tc.id)
		if tc.id == 0 {
			c.Params = nil
		}
		c.Request = httptest.NewRequest(tc.method, "/", strings.NewReader(tc.body))
		c.Request.Header.Set("Content-Type", "application/json")
		tc.invoke(s, c)
		if w.Code != http.StatusForbidden {
			t.Fatalf("%s: viewer status = %d, want 403", tc.name, w.Code)
		}
	}
	if got := alertCount(t, s, ""); got != 1 {
		t.Fatalf("a viewer changed alert state (%d alerts remain)", got)
	}
}

// ── Severity mapping ────────────────────────────────────────────────────────

// TestAlertSeverityMapping pins the rule type → severity table, since the
// filtered counters the API reports are derived from it.
func TestAlertSeverityMapping(t *testing.T) {
	s := mustTenantServer(t)
	m := NewMonitorCollector(s)
	cases := map[string]string{
		"agent_offline":    "critical",
		"cpu_high":         "critical",
		"memory_high":      "critical",
		"disk_high":        "critical",
		"credential_found": "info",
		"agent_online":     "info",
		"something_new":    "warning",
		"":                 "warning",
	}
	for ruleType, want := range cases {
		if got := m.getSeverity(ruleType); got != want {
			t.Fatalf("getSeverity(%q) = %q, want %q", ruleType, got, want)
		}
	}
}
