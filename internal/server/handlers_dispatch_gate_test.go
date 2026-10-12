package server

import (
	"net/http"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// The four handlers below used to create their task rows with a bare
// s.db.Create, skipping the tenant scope, the collaboration-lock gate, the
// pending counter, the task_created plugin hook and (for the two that
// hard-coded it) the operator identity in CreatedBy. They now route through
// issueAgentTask / createSystemTaskWithData like every other dispatch path.

// TestScanTaskCrossTenant404 proves POST /api/scan refuses another tenant's
// agent and queues nothing (the endpoint takes agent_id from the form body,
// so an unvalidated id is a direct cross-tenant dispatch).
func TestScanTaskCrossTenant404(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "scan-x", 2)

	c, w := evasionCtx(s, t, "scan-viewer", "agent_id=scan-x&target=10.0.0.5&top_ports=10")
	s.handleScanTask(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s, want 404 for another tenant's agent", w.Code, w.Body.String())
	}
	assertNoTasksForAgent(t, s, "scan-x")
}

// TestScanTaskSameTenantDispatch proves the happy path still queues the scan
// with the built command (target:ports) and the scan_type shell.
func TestScanTaskSameTenantDispatch(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "scan-ok", 1)

	c, w := evasionCtx(s, t, "scan-op", "agent_id=scan-ok&target=10.0.0.5&top_ports=2&scan_type=nmap")
	s.handleScanTask(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", w.Code, w.Body.String())
	}
	var task db.Task
	if err := s.db.Where("agent_id = ?", "scan-ok").First(&task).Error; err != nil {
		t.Fatalf("no scan task queued: %v", err)
	}
	if task.Type != "portscan" {
		t.Fatalf("type = %q, want portscan", task.Type)
	}
	if task.Command != "10.0.0.5:21,22" {
		t.Fatalf("command = %q, want 10.0.0.5:21,22 (top-2 ports)", task.Command)
	}
	if task.Shell != "nmap" {
		t.Fatalf("shell = %q, want the scan_type nmap", task.Shell)
	}
}

// TestScanTaskValidationPreserved proves the existing 400 guards survived the
// reroute (missing target, and the port ceiling).
func TestScanTaskValidationPreserved(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "scan-val", 1)

	c, w := evasionCtx(s, t, "scan-op", "agent_id=scan-val&top_ports=10")
	s.handleScanTask(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("missing target: status=%d, want 400", w.Code)
	}
	assertNoTasksForAgent(t, s, "scan-val")

	// A port range wider than the 10000 ceiling must still be refused.
	c2, w2 := evasionCtx(s, t, "scan-op", "agent_id=scan-val&target=10.0.0.5&port_range=1-20000")
	s.handleScanTask(c2)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("oversized range: status=%d, want 400", w2.Code)
	}
	assertNoTasksForAgent(t, s, "scan-val")
}

// TestProfileRotateCrossTenant404 proves the OPSEC rotation endpoint refuses
// another tenant's agent.
func TestProfileRotateCrossTenant404(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "rot-x", 2)

	c, w := privescCtx(s, t, "rot-viewer", `{"user_agent":"UA","c2_url":"https://x.invalid"}`)
	c.Params = gin.Params{{Key: "id", Value: "rot-x"}}
	s.handleProfileRotate(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404 for another tenant's agent", w.Code)
	}
	assertNoTasksForAgent(t, s, "rot-x")
}

// TestProfileRotateDispatchAndAttribution proves the happy path queues the
// rotation payload and stamps the REAL operator as creator (the old code
// hard-coded "operator", which broke the audit chain).
func TestProfileRotateDispatchAndAttribution(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "rot-ok", 1)

	c, w := privescCtx(s, t, "rot-op", `{"user_agent":"UA","c2_url":"https://x.invalid"}`)
	c.Params = gin.Params{{Key: "id", Value: "rot-ok"}}
	s.handleProfileRotate(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", w.Code, w.Body.String())
	}
	var task db.Task
	if err := s.db.Where("agent_id = ?", "rot-ok").First(&task).Error; err != nil {
		t.Fatalf("no rotation task queued: %v", err)
	}
	if task.Type != "profile_rotate" {
		t.Fatalf("type = %q, want profile_rotate", task.Type)
	}
	if task.Data == "" || !contains(task.Data, "https://x.invalid") {
		t.Fatalf("rotation payload lost: %q", task.Data)
	}
	if task.CreatedBy != "rot-op" {
		t.Fatalf("CreatedBy = %q, want the calling operator rot-op", task.CreatedBy)
	}
}

// TestProfileRotateRequiresOperator proves the endpoint now gates on the
// operator role (it previously had no authorization check at all — a viewer
// could push a profile rotation).
func TestProfileRotateRequiresOperator(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "rot-view", 1)

	c, w := privescCtx(s, t, "rot-viewer", `{"user_agent":"UA"}`)
	c.Set("user_role", "viewer")
	c.Params = gin.Params{{Key: "id", Value: "rot-view"}}
	s.handleProfileRotate(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403 for a viewer", w.Code)
	}
	assertNoTasksForAgent(t, s, "rot-view")
}

// TestPushAgentConfigCrossTenant404 proves the config-push endpoint refuses
// another tenant's agent — this task can carry the OTA update signing key,
// so a cross-tenant push is a trust-boundary violation.
func TestPushAgentConfigCrossTenant404(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "cfg-x", 2)

	c, w := privescCtx(s, t, "cfg-viewer", `{"sleep":30}`)
	c.Params = gin.Params{{Key: "id", Value: "cfg-x"}}
	s.handlePushAgentConfig(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404 for another tenant's agent", w.Code)
	}
	assertNoTasksForAgent(t, s, "cfg-x")
}

// TestPushAgentConfigDispatchAndAttribution proves the happy path queues the
// config_push task with the real operator as creator.
func TestPushAgentConfigDispatchAndAttribution(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "cfg-ok", 1)

	c, w := privescCtx(s, t, "cfg-op", `{"sleep":42,"user_agent":"UA"}`)
	c.Params = gin.Params{{Key: "id", Value: "cfg-ok"}}
	s.handlePushAgentConfig(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", w.Code, w.Body.String())
	}
	var task db.Task
	if err := s.db.Where("agent_id = ?", "cfg-ok").First(&task).Error; err != nil {
		t.Fatalf("no config_push task queued: %v", err)
	}
	if task.Type != "config_push" {
		t.Fatalf("type = %q, want config_push", task.Type)
	}
	if task.Data == "" || !contains(task.Data, "42") {
		t.Fatalf("config payload lost the pushed sleep value: %q", task.Data)
	}
	if task.CreatedBy != "cfg-op" {
		t.Fatalf("CreatedBy = %q, want the calling operator cfg-op", task.CreatedBy)
	}
}

// TestListenerBurnRotationInheritsTenant proves the circuit-breaker rotation
// path (a system path, no request context) now goes through the system-task
// creator: the row inherits the agent's tenant (a bare Create left it 0,
// which made tenant-scoped reads 404 on it), keeps the "system" creator, and
// feeds the pending counter like every other task.
func TestListenerBurnRotationInheritsTenant(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "burn-agent", 3)
	if err := s.db.Model(&db.Implant{}).Where("id = ?", "burn-agent").
		Update("listener_id", 11).Error; err != nil {
		t.Fatalf("set listener: %v", err)
	}
	// A fallback listener must exist, otherwise the rotation short-circuits.
	if err := s.db.Create(&db.Listener{
		ID: 12, Name: "fallback", Scheme: "https", Host: "alt.example.com", Port: 443, Enabled: true,
	}).Error; err != nil {
		t.Fatalf("seed fallback listener: %v", err)
	}
	// The burned listener itself (id 11) may exist or not; the query is by
	// implant.listener_id, so the row is optional.

	s.rotateAgentsOnBurnedListener("11")

	var tasks []db.Task
	if err := s.db.Where("agent_id = ?", "burn-agent").Find(&tasks).Error; err != nil {
		t.Fatalf("find: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("rotation created %d task(s), want 1", len(tasks))
	}
	task := tasks[0]
	if task.Type != "profile_rotate" {
		t.Fatalf("type = %q, want profile_rotate", task.Type)
	}
	if task.CreatedBy != "system" {
		t.Fatalf("CreatedBy = %q, want system", task.CreatedBy)
	}
	if task.TenantID != 3 {
		t.Fatalf("tenant = %d, want inheritance of the agent's 3", task.TenantID)
	}
	if task.Data == "" || !contains(task.Data, "alt.example.com") {
		t.Fatalf("rotation payload missing the fallback URL: %q", task.Data)
	}
	s.agentPendingTasksMu.Lock()
	pending := s.agentPendingTasks["burn-agent"]
	s.agentPendingTasksMu.Unlock()
	if pending != 1 {
		t.Fatalf("pending counter = %d, want 1 (system tasks now feed it)", pending)
	}
}

// contains is a tiny local helper (strings.Contains needs an import in every
// test file; this keeps the assertions above dependency-free).
func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
