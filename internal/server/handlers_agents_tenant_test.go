package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gin-gonic/gin"
)

func seedTenantAgent(t *testing.T, s *Server, id string, tenantID uint) {
	t.Helper()
	if err := s.db.Create(&db.Implant{ID: id, TenantID: tenantID, Hostname: "H-" + id}).Error; err != nil {
		t.Fatalf("seed agent: %v", err)
	}
}

func crossTenantCtx(s *Server, t *testing.T, agentID, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	c, w := tenantScopedAdminContext(s, t, "g1-viewer", 1)
	c.Params = gin.Params{{Key: "id", Value: agentID}}
	if body != "" {
		c.Request, _ = http.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(body)))
		c.Request.Header.Set("Content-Type", "application/json")
	} else {
		c.Request, _ = http.NewRequest(http.MethodPost, "/", nil)
	}
	return c, w
}

func mustTenantServer(t *testing.T) *Server {
	t.Helper()
	ginSetTestMode(t)
	return initV3BeaconServer(t, testutil.SetupTestDB(t), tenantVisibilityMasterHex)
}

// TestAgentNoteCrossTenant404 proves operator notes/tags cannot be written
// to another tenant's agent (previously an unscoped direct write).
func TestAgentNoteCrossTenant404(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "g1-note", 2)

	c, w := crossTenantCtx(s, t, "g1-note", `{"notes":"pwned","tags":"x"}`)
	s.handleUpdateNote(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404", w.Code)
	}
	var agent db.Implant
	if err := s.db.First(&agent, "id = ?", "g1-note").Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if agent.Notes != "" || agent.Tags != "" {
		t.Fatalf("cross-tenant write landed: %+v", agent)
	}
}

// TestAgentNoteSameTenantOK guards the gate: own-tenant writes still work.
func TestAgentNoteSameTenantOK(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "g1-note-ok", 1)

	c, w := crossTenantCtx(s, t, "g1-note-ok", `{"notes":"hello"}`)
	s.handleUpdateNote(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", w.Code, w.Body.String())
	}
	var agent db.Implant
	if err := s.db.First(&agent, "id = ?", "g1-note-ok").Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if agent.Notes != "hello" {
		t.Fatalf("notes=%q, want hello", agent.Notes)
	}
}

// TestAgentTrustCrossTenant404 proves the trust toggle honors tenancy on
// both the read and the write.
func TestAgentTrustCrossTenant404(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "g1-trust", 2)

	c, w := crossTenantCtx(s, t, "g1-trust", "")
	s.handleToggleAgentTrust(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404", w.Code)
	}
	var agent db.Implant
	if err := s.db.First(&agent, "id = ?", "g1-trust").Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if agent.Trusted {
		t.Fatal("cross-tenant trust flip landed")
	}
}

// TestKillDateCrossTenant404 proves kill-date set honors tenancy.
func TestKillDateCrossTenant404(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "g1-kill", 2)

	c, w := crossTenantCtx(s, t, "g1-kill", `{"kill_date":"2030-01-01"}`)
	s.handleSetKillDate(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404", w.Code)
	}
}

// TestBlockCrossTenant404 proves block honors tenancy.
func TestBlockCrossTenant404(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "g1-block", 2)

	c, w := crossTenantCtx(s, t, "g1-block", `{"reason":"x"}`)
	s.handleBlockAgent(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404", w.Code)
	}
	var agent db.Implant
	if err := s.db.First(&agent, "id = ?", "g1-block").Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if agent.Blocked {
		t.Fatal("cross-tenant block landed")
	}
}

// TestSocksStopCrossTenant404 proves one tenant cannot tear down another
// tenant's relay.
func TestSocksStopCrossTenant404(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "g1-socks", 2)

	c, w := crossTenantCtx(s, t, "g1-socks", "")
	c.Set("user_role", "admin")
	s.handleStopSocksRelay(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404", w.Code)
	}
}

// TestSocksStatusCrossTenant404 proves relay status honors tenancy.
func TestSocksStatusCrossTenant404(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "g1-socks-st", 2)

	c, w := tenantScopedCtx(s, t, "g1-viewer", 1, "g1-socks-st")
	s.handleSocksRelayStatus(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404", w.Code)
	}
}

// TestSocksSessionsScopedToTenant proves the session list only shows
// tunnels owned by the caller's tenant (rows carry no tenant_id; scoping
// goes through the owning agent).
func TestSocksSessionsScopedToTenant(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "g1-socks-a", 1)
	seedTenantAgent(t, s, "g1-socks-b", 2)
	for _, agentID := range []string{"g1-socks-a", "g1-socks-b"} {
		if err := s.db.Create(&db.SocksSession{AgentID: agentID, ListenPort: 1080, Status: "active"}).Error; err != nil {
			t.Fatalf("seed session: %v", err)
		}
	}

	c, w := tenantScopedCtx(s, t, "g1-viewer", 1, "")
	s.handleGetSocksSessions(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "g1-socks-a") {
		t.Fatalf("own session missing: %s", body)
	}
	if strings.Contains(body, "g1-socks-b") {
		t.Fatalf("foreign session leaked: %s", body)
	}
}

// TestReportTasksScopedToTenant proves the report task endpoint only
// returns the caller's tenant rows (previously a full-fleet dump).
func TestReportTasksScopedToTenant(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "g1-rep-a", 1)
	seedTenantAgent(t, s, "g1-rep-b", 2)
	for _, agentID := range []string{"g1-rep-a", "g1-rep-b"} {
		var ag db.Implant
		if err := s.db.Where("id = ?", agentID).First(&ag).Error; err != nil {
			t.Fatalf("load agent: %v", err)
		}
		if err := s.db.Create(&db.Task{AgentID: agentID, TenantID: ag.TenantID, Type: "shell", Status: "completed"}).Error; err != nil {
			t.Fatalf("seed task: %v", err)
		}
	}

	c, w := tenantScopedAdminContext(s, t, "g1-viewer", 1)
	c.Request, _ = http.NewRequest(http.MethodGet, "/?start=2000-01-01&end=2100-01-01", nil)
	s.handleAPIGetReportTasks(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "g1-rep-a") {
		t.Fatalf("own task missing: %s", body)
	}
	if strings.Contains(body, "g1-rep-b") {
		t.Fatalf("foreign task leaked: %s", body)
	}
}

// TestReportCredsScopedToTenant proves credential rows scope through the
// owning agent (no tenant_id column on the table itself).
func TestReportCredsScopedToTenant(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "g1-creds-a", 1)
	seedTenantAgent(t, s, "g1-creds-b", 2)
	for _, agentID := range []string{"g1-creds-a", "g1-creds-b"} {
		if err := s.db.Create(&db.CredentialEntry{AgentID: agentID, Username: "u-" + agentID}).Error; err != nil {
			t.Fatalf("seed cred: %v", err)
		}
	}

	c, w := tenantScopedAdminContext(s, t, "g1-viewer", 1)
	c.Request, _ = http.NewRequest(http.MethodGet, "/?start=2000-01-01&end=2100-01-01", nil)
	s.handleAPIGetReportCredentials(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "g1-creds-a") {
		t.Fatalf("own cred missing: %s", body)
	}
	if strings.Contains(body, "g1-creds-b") {
		t.Fatalf("foreign cred leaked: %s", body)
	}
}

// TestAIAnalyzeResultCrossTenant404 proves one tenant cannot feed another
// tenant's raw task output into the LLM via a model-supplied task ID.
func TestAIAnalyzeResultCrossTenant404(t *testing.T) {
	s := mustTenantServer(t)
	s.cfg.AI.Enabled = true
	s.cfg.AI.APIKey = "test-key"
	seedTenantAgent(t, s, "g1-analyze", 2)
	task := db.Task{AgentID: "g1-analyze", Type: "shell", Status: "completed", Result: "secret-output"}
	if err := s.db.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}

	c, w := tenantScopedAdminContext(s, t, "g1-viewer", 1)
	body, _ := json.Marshal(map[string]uint{"task_id": task.ID})
	c.Request, _ = http.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	s.handleAIAnalyzeResult(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s, want 404", w.Code, w.Body.String())
	}
}

func tenantScopedCtx(s *Server, t *testing.T, username string, tenantID uint, agentID string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	c, w := tenantScopedAdminContext(s, t, username, tenantID)
	if agentID != "" {
		c.Params = gin.Params{{Key: "id", Value: agentID}}
	} else {
		c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)
	}
	return c, w
}

// TestChainSetCrossTenant404 proves topology chain writes honor tenancy.
func TestChainSetCrossTenant404(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "g1-chain", 2)

	c, w := crossTenantCtx(s, t, "g1-chain", `{"parent_id":"g1-other"}`)
	// handleAgentChainSet reads :id (not :taskId); remap param key below.
	c.Params = gin.Params{{Key: "id", Value: "g1-chain"}}
	s.handleAgentChainSet(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s, want 404", w.Code, w.Body.String())
	}
	var agent db.Implant
	if err := s.db.First(&agent, "id = ?", "g1-chain").Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if agent.ParentAgentID != "" {
		t.Fatalf("cross-tenant chain write landed: %q", agent.ParentAgentID)
	}
}
