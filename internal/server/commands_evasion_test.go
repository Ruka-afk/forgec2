package server

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	forgecrypto "github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// evasionCtx builds an operator-authenticated POST context for the evasion
// handlers. The user row is seeded with tenant 1 (a bare context would make
// lookupTenantID fall back to legacy-unscoped), and the loot vault is up
// because several of these tasks carry an encrypted Data payload.
func evasionCtx(s *Server, t *testing.T, username, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	forgecrypto.InitLootEncryption(tenantVisibilityMasterHex)
	c, w := tenantScopedAdminContext(s, t, username, 1)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(body)))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.Set("user_role", "operator")
	var user db.User
	if err := s.db.Where("username = ?", username).First(&user).Error; err != nil {
		t.Fatalf("seed caller: %v", err)
	}
	c.Set("user_id", user.ID)
	return c, w
}

// assertNoTasksForAgent is the refusal guard: a refused dispatch must leave
// zero rows for the target agent.
func assertNoTasksForAgent(t *testing.T, s *Server, agentID string) {
	t.Helper()
	var n int64
	if err := s.db.Model(&db.Task{}).Where("agent_id = ?", agentID).Count(&n).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("refused dispatch still created %d task(s) for %s", n, agentID)
	}
}

// TestEvasionHandlersCrossTenant404 proves every simple task-issuing evasion
// handler refuses another tenant's agent (404) and queues nothing. Each goes
// through issueAgentTask, whose getAgentOrFail carries the tenant scope; this
// locks that contract for the whole family.
func TestEvasionHandlersCrossTenant404(t *testing.T) {
	cases := []struct {
		name   string
		invoke func(s *Server, c *gin.Context)
	}{
		{"amsi_bypass", func(s *Server, c *gin.Context) { s.handleAMSIByPass(c) }},
		{"etw_bypass", func(s *Server, c *gin.Context) { s.handleETWByPass(c) }},
		{"amsi_hardware_bp", func(s *Server, c *gin.Context) { s.handleAMSIHardwareBP(c) }},
		{"etw_hardware_bp", func(s *Server, c *gin.Context) { s.handleETWHardwareBP(c) }},
		{"sandbox_detect_advanced", func(s *Server, c *gin.Context) { s.handleSandboxDetectAdvanced(c) }},
		{"set_sleep_mask_advanced", func(s *Server, c *gin.Context) { s.handleSetSleepMaskAdvanced(c) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := mustTenantServer(t)
			seedTenantAgent(t, s, "ev-x", 2) // another tenant's agent

			c, w := evasionCtx(s, t, "ev-viewer", "")
			c.Params = gin.Params{{Key: "id", Value: "ev-x"}}
			tc.invoke(s, c)

			if w.Code != http.StatusNotFound {
				t.Fatalf("status=%d body=%s, want 404 for another tenant's agent", w.Code, w.Body.String())
			}
			assertNoTasksForAgent(t, s, "ev-x")
		})
	}
}

// TestEvasionHandlersSameTenantDispatch proves each handler queues exactly one
// task of the right type against an own-tenant agent.
func TestEvasionHandlersSameTenantDispatch(t *testing.T) {
	cases := []struct {
		name    string
		invoke  func(s *Server, c *gin.Context)
		wantTyp string
	}{
		{"amsi_bypass", func(s *Server, c *gin.Context) { s.handleAMSIByPass(c) }, "amsi_bypass"},
		{"etw_bypass", func(s *Server, c *gin.Context) { s.handleETWByPass(c) }, "etw_bypass"},
		{"amsi_hardware_bp", func(s *Server, c *gin.Context) { s.handleAMSIHardwareBP(c) }, "amsi_hardware_bp"},
		{"etw_hardware_bp", func(s *Server, c *gin.Context) { s.handleETWHardwareBP(c) }, "etw_hardware_bp"},
		{"sandbox_detect_advanced", func(s *Server, c *gin.Context) { s.handleSandboxDetectAdvanced(c) }, "sandbox_detect_advanced"},
		{"set_sleep_mask_advanced", func(s *Server, c *gin.Context) { s.handleSetSleepMaskAdvanced(c) }, "set_sleep_mask_advanced"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := mustTenantServer(t)
			seedTenantAgent(t, s, "ev-ok", 1)

			c, w := evasionCtx(s, t, "ev-op", "")
			c.Params = gin.Params{{Key: "id", Value: "ev-ok"}}
			tc.invoke(s, c)

			if w.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s, want 200", w.Code, w.Body.String())
			}
			var task db.Task
			if err := s.db.Where("agent_id = ?", "ev-ok").First(&task).Error; err != nil {
				t.Fatalf("no task queued: %v", err)
			}
			if task.Type != tc.wantTyp {
				t.Fatalf("task type = %q, want %q", task.Type, tc.wantTyp)
			}
		})
	}
}

// TestEvasionHandlersRequireOperator proves viewers are refused before any
// task is queued (the first gate in every handler).
func TestEvasionHandlersRequireOperator(t *testing.T) {
	cases := []struct {
		name   string
		invoke func(s *Server, c *gin.Context)
	}{
		{"amsi_bypass", func(s *Server, c *gin.Context) { s.handleAMSIByPass(c) }},
		{"net", func(s *Server, c *gin.Context) { s.handleNetCommand(c) }},
		{"run_evasion", func(s *Server, c *gin.Context) { s.handleRunEvasion(c) }},
		{"self_update", func(s *Server, c *gin.Context) { s.handleSelfUpdate(c) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := mustTenantServer(t)
			seedTenantAgent(t, s, "ev-view", 1)

			c, w := evasionCtx(s, t, "ev-viewer", "")
			c.Set("user_role", "viewer")
			c.Params = gin.Params{{Key: "id", Value: "ev-view"}}
			tc.invoke(s, c)

			if w.Code != http.StatusForbidden {
				t.Fatalf("status=%d, want 403 for a viewer", w.Code)
			}
			assertNoTasksForAgent(t, s, "ev-view")
		})
	}
}

// TestNetCommandRequiresCommand proves the parameter guard (no default) and
// that a supplied command is carried onto the task.
func TestNetCommandRequiresCommand(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "ev-net", 1)

	c, w := evasionCtx(s, t, "ev-op", "")
	c.Params = gin.Params{{Key: "id", Value: "ev-net"}}
	s.handleNetCommand(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s, want 400 (command required)", w.Code, w.Body.String())
	}
	assertNoTasksForAgent(t, s, "ev-net")

	c2, w2 := evasionCtx(s, t, "ev-op", "command=view")
	c2.Params = gin.Params{{Key: "id", Value: "ev-net"}}
	s.handleNetCommand(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200 with a command", w2.Code, w2.Body.String())
	}
	var task db.Task
	if err := s.db.Where("agent_id = ?", "ev-net").First(&task).Error; err != nil {
		t.Fatalf("no net task queued: %v", err)
	}
	if task.Type != "net" || task.Command != "view" {
		t.Fatalf("task = %+v, want net/view", task)
	}
}

// TestRunEvasionRequiresTechnique proves the technique parameter is mandatory
// (an empty technique would dispatch a no-op the implant cannot interpret).
func TestRunEvasionRequiresTechnique(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "ev-run", 1)

	c, w := evasionCtx(s, t, "ev-op", "")
	c.Params = gin.Params{{Key: "id", Value: "ev-run"}}
	s.handleRunEvasion(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400 (technique required)", w.Code)
	}
	assertNoTasksForAgent(t, s, "ev-run")
}

// TestMimikatzDefaultsCommandAndScopes proves the default command is applied
// and the tenant gate still runs first.
func TestMimikatzDefaultsCommandAndScopes(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "ev-mk", 1)
	seedTenantAgent(t, s, "ev-mk-foreign", 2)

	c, w := evasionCtx(s, t, "ev-op", "")
	c.Params = gin.Params{{Key: "id", Value: "ev-mk"}}
	s.handleMimikatz(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", w.Code, w.Body.String())
	}
	var task db.Task
	if err := s.db.Where("agent_id = ?", "ev-mk").First(&task).Error; err != nil {
		t.Fatalf("no mimikatz task: %v", err)
	}
	if task.Type != "mimikatz" || task.Command != "sekurlsa::logonpasswords" {
		t.Fatalf("task = %+v, want mimikatz with the default command", task)
	}

	c2, w2 := evasionCtx(s, t, "ev-op", "")
	c2.Params = gin.Params{{Key: "id", Value: "ev-mk-foreign"}}
	s.handleMimikatz(c2)
	if w2.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404 for another tenant's agent", w2.Code)
	}
	assertNoTasksForAgent(t, s, "ev-mk-foreign")
}

// TestSelfUpdateRequiresURLAndSha proves the signature-enforced contract: both
// the download URL and the binary digest are mandatory before the teamserver
// signs anything.
func TestSelfUpdateRequiresURLAndSha(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "ev-upd", 1)

	c, w := evasionCtx(s, t, "ev-op", "url=https://example.invalid/a.exe")
	c.Params = gin.Params{{Key: "id", Value: "ev-upd"}}
	s.handleSelfUpdate(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s, want 400 (sha256 required)", w.Code, w.Body.String())
	}
	assertNoTasksForAgent(t, s, "ev-upd")

	// Neither field at all is also a 400, and still queues nothing.
	c2, w2 := evasionCtx(s, t, "ev-op", "")
	c2.Params = gin.Params{{Key: "id", Value: "ev-upd"}}
	s.handleSelfUpdate(c2)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400 (url required)", w2.Code)
	}
	assertNoTasksForAgent(t, s, "ev-upd")
}

// TestBOFUploadRequiresFile proves the BOF endpoint rejects a request without
// the object file instead of dispatching an empty payload.
func TestBOFUploadRequiresFile(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "ev-bof", 1)

	c, w := evasionCtx(s, t, "ev-op", "")
	c.Params = gin.Params{{Key: "id", Value: "ev-bof"}}
	s.handleBOF(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s, want 400 (bof file required)", w.Code, w.Body.String())
	}
	assertNoTasksForAgent(t, s, "ev-bof")
}

// TestBOFCrossTenantUploadRefused proves a foreign agent is refused before the
// upload is even parsed.
func TestBOFCrossTenantUploadRefused(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "ev-bof-foreign", 2)

	c, w := evasionCtx(s, t, "ev-op", "")
	c.Params = gin.Params{{Key: "id", Value: "ev-bof-foreign"}}
	s.handleBOF(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404 for another tenant's agent", w.Code)
	}
	assertNoTasksForAgent(t, s, "ev-bof-foreign")
}

// TestBOFUploadDispatches proves a well-formed multipart upload queues the BOF
// task with the base64 payload attached.
func TestBOFUploadDispatches(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "ev-bof-ok", 1)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("bof", "whoami.o")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write([]byte{0x00, 0x01, 0x02, 0x03}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := mw.WriteField("args", "--list"); err != nil {
		t.Fatalf("write field: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	forgecrypto.InitLootEncryption(tenantVisibilityMasterHex)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", &buf)
	c.Request.Header.Set("Content-Type", mw.FormDataContentType())
	c.Set("user", "ev-op")
	c.Set("user_role", "operator")
	var user db.User
	if err := s.db.Where("username = ?", "ev-op").FirstOrCreate(&user, db.User{Username: "ev-op", TenantID: 1, Role: "operator", IsActive: true}).Error; err != nil {
		t.Fatalf("seed caller: %v", err)
	}
	c.Set("user_id", user.ID)
	c.Params = gin.Params{{Key: "id", Value: "ev-bof-ok"}}
	s.handleBOF(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", w.Code, w.Body.String())
	}
	var task db.Task
	if err := s.db.Where("agent_id = ?", "ev-bof-ok").First(&task).Error; err != nil {
		t.Fatalf("no BOF task queued: %v", err)
	}
	if task.Type != "bof" || task.Data == "" {
		t.Fatalf("task = %+v, want bof with the encoded payload", task)
	}
}
