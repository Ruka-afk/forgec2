package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	forgecrypto "github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// POST /toolkit/agents/:id/action used to insert its task with a bare
// s.db.Create(&task). That bypassed getAgentOrFail (the tenant scope) and
// createTask (the collaboration-lock gate), so an operator could queue work on
// another tenant's agent and silently overwrite an agent another operator was
// working. It also skipped the Mimikatz module auto-attach that
// handleMimikatz performs.

func toolkitActionCtx(s *Server, t *testing.T, agentID, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	// createTask refuses to store task data in plaintext, so the vault must be
	// up for any path that attaches a module.
	forgecrypto.InitLootEncryption(tenantVisibilityMasterHex)
	c, w := tenantScopedAdminContext(s, t, "g1-viewer", 1)
	c.Params = gin.Params{{Key: "id", Value: agentID}}
	c.Request, _ = http.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(body)))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, w
}

func TestToolkitActionCrossTenant404(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "g1-tool", 2)

	c, w := toolkitActionCtx(s, t, "g1-tool", `{"action":"mimikatz","param":"privilege::debug"}`)
	s.handleToolkitQuickAction(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s, want 404", w.Code, w.Body.String())
	}
	var n int64
	if err := s.db.Model(&db.Task{}).Where("agent_id = ?", "g1-tool").Count(&n).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("cross-tenant toolkit action created %d task(s)", n)
	}
}

func TestToolkitActionSameTenantOK(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "g1-tool-ok", 1)

	c, w := toolkitActionCtx(s, t, "g1-tool-ok", `{"action":"mimikatz","param":"privilege::debug"}`)
	s.handleToolkitQuickAction(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", w.Code, w.Body.String())
	}
	var task db.Task
	if err := s.db.Where("agent_id = ?", "g1-tool-ok").First(&task).Error; err != nil {
		t.Fatalf("no task created for own-tenant agent: %v", err)
	}
	if task.Type != "mimikatz" {
		t.Fatalf("task type = %q, want mimikatz", task.Type)
	}
}

// The toolkit's mimikatz quick action must carry the same auto-attached module
// as the dedicated /agents/:id/mimikatz handler, otherwise the implant has to
// fall back to a local script that the operator never staged.
func TestToolkitMimikatzAttachesModule(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "g1-mimi", 1)

	modPath := filepath.Join(s.modulesDir(), "Invoke-Mimikatz.ps1")
	if err := os.WriteFile(modPath, []byte("# staged mimikatz module"), 0640); err != nil {
		t.Fatalf("stage module: %v", err)
	}

	c, w := toolkitActionCtx(s, t, "g1-mimi", `{"action":"mimikatz","param":"privilege::debug"}`)
	s.handleToolkitQuickAction(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", w.Code, w.Body.String())
	}
	var task db.Task
	if err := s.db.Where("agent_id = ?", "g1-mimi").First(&task).Error; err != nil {
		t.Fatalf("reload task: %v", err)
	}
	if task.Data == "" {
		t.Fatal("mimikatz task carries no module; the dedicated handler attaches one")
	}
}
