package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	forgecrypto "github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// POST /lateral/execute used to call s.createTask directly. That skipped
// getAgentOrFail (the tenant scope) and the collaboration-lock gate inside
// createTask, so an operator could queue a lateral move against another
// tenant's agent and silently overwrite an agent another operator was
// actively working. It now goes through issueAgentTask like every other
// handler-issued task.

func lateralExecuteCtx(s *Server, t *testing.T, username string, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	// A lateral spec carries a credential, so createTask refuses to store it in
	// plaintext; the vault must be up for the same-tenant and lock paths.
	forgecrypto.InitLootEncryption(tenantVisibilityMasterHex)
	c, w := tenantScopedAdminContext(s, t, username, 1)
	// tenantScopedAdminContext mirrors the auth middleware's "user"/"user_role"
	// but not "user_id", which callerOpts reads. Without it the collab-lock
	// gate sees callerUserID 0 and treats the call as a system/automation task
	// that is exempt from locks.
	var user db.User
	if err := s.db.Where("username = ?", username).First(&user).Error; err != nil {
		t.Fatalf("seed caller: %v", err)
	}
	c.Set("user_id", user.ID)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/lateral/execute", bytes.NewReader([]byte(body)))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, w
}

func TestLateralExecuteCrossTenant404(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "g1-lat", 2)

	c, w := lateralExecuteCtx(s, t, "g1-lat-viewer", `{"source":"g1-lat","target":"10.0.0.9","method":"smbexec","username":"admin","password":"pw"}`)
	s.handleAPILateralExecute(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s, want 404", w.Code, w.Body.String())
	}
	var n int64
	if err := s.db.Model(&db.Task{}).Where("agent_id = ?", "g1-lat").Count(&n).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("cross-tenant lateral execute created %d task(s)", n)
	}
}

func TestLateralExecuteSameTenantOK(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "g1-lat-ok", 1)

	c, w := lateralExecuteCtx(s, t, "g1-lat-ok-viewer", `{"source":"g1-lat-ok","target":"10.0.0.9","method":"smbexec","username":"admin","password":"pw"}`)
	s.handleAPILateralExecute(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", w.Code, w.Body.String())
	}
	var task db.Task
	if err := s.db.Where("agent_id = ?", "g1-lat-ok").First(&task).Error; err != nil {
		t.Fatalf("no task created for own-tenant agent: %v", err)
	}
	if task.Type != "lateral" {
		t.Fatalf("task type = %q, want lateral", task.Type)
	}
}

// A lateral move writes into a live SMB/WinRM session on the source agent, so
// it must respect another operator's collaboration lock like every other task
// type; otherwise two operators drive the same host through one channel.
func TestLateralExecuteRespectsCollabLock(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "g1-lat-lock", 1)

	now := time.Now()
	if err := s.db.Create(&db.AgentLock{
		AgentID:   "g1-lat-lock",
		LockedBy:  "other-operator",
		LockedAt:  now,
		ExpiresAt: now.Add(15 * time.Minute),
	}).Error; err != nil {
		t.Fatalf("seed lock: %v", err)
	}

	c, w := lateralExecuteCtx(s, t, "g1-lat-lock-viewer", `{"source":"g1-lat-lock","target":"10.0.0.9","method":"smbexec","username":"admin","password":"pw"}`)
	s.handleAPILateralExecute(c)

	if w.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s, want 409", w.Code, w.Body.String())
	}
	var n int64
	if err := s.db.Model(&db.Task{}).Where("agent_id = ?", "g1-lat-lock").Count(&n).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("lateral execute on a locked agent created %d task(s)", n)
	}
}

// The tenant/lock gate must run before any spec validation work is observable,
// and a pivot request (unsupported by the implant) is still a 400 -- not a 404
// that would leak whether the agent exists in another tenant.
func TestLateralExecutePivotStillBadRequest(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "g1-lat-pivot", 1)

	c, w := lateralExecuteCtx(s, t, "g1-lat-pivot-viewer", `{"source":"g1-lat-pivot","target":"10.0.0.9","method":"pivot","pivot":"tcp://1.2.3.4:9000"}`)
	s.handleAPILateralExecute(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s, want 400", w.Code, w.Body.String())
	}
}
