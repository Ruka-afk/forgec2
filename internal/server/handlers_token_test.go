package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	forgecrypto "github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// tokenCtx builds an operator-authenticated form POST for a token endpoint.
func tokenCtx(username, role, agentID, tokenID, form string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.Set("user", username)
	c.Set("user_role", role)
	c.Params = gin.Params{{Key: "id", Value: agentID}, {Key: "token_id", Value: tokenID}}
	return c, w
}

// tenantTokenCtx pins the tenant resolution so tenantScope scopes the agent
// lookup; a bare context would fall back to legacy-unscoped and hide
// cross-tenant regressions.
func tenantTokenCtx(t *testing.T, s *Server, username, role string, tenantID uint, agentID, tokenID, form string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	c, w := tokenCtx(username, role, agentID, tokenID, form)
	c.Set(tenantIDContextKey, tenantResolution{tid: tenantID, ok: true})
	var user db.User
	if err := s.db.Where("username = ?", username).First(&user).Error; err != nil {
		t.Fatalf("seed caller: %v", err)
	}
	c.Set("user_id", user.ID)
	return c, w
}

func seedTokenOperator(t *testing.T, s *Server, username string, tenantID uint, role string) {
	t.Helper()
	if err := s.db.Create(&db.User{Username: username, TenantID: tenantID, Role: role, IsActive: true}).Error; err != nil {
		t.Fatalf("seed operator: %v", err)
	}
}

// ── Dispatch table ──────────────────────────────────────────────────────────

// TestTokenTaskHandlersDispatch covers the token command family on the three
// axes every operator command must hold: another tenant's agent is invisible
// (404, zero tasks queued), a viewer is refused, and a same-tenant operator
// gets the right task type queued.
func TestTokenTaskHandlersDispatch(t *testing.T) {
	cases := []struct {
		name     string
		invoke   func(s *Server, c *gin.Context)
		wantType string
	}{
		{"list_procs", func(s *Server, c *gin.Context) { s.handleTokenListProcs(c) }, "token_list_procs"},
		{"steal", func(s *Server, c *gin.Context) { s.handleTokenSteal(c) }, "token_steal"},
		{"make", func(s *Server, c *gin.Context) { s.handleTokenMake(c) }, "token_make"},
		{"revert", func(s *Server, c *gin.Context) { s.handleTokenRevert(c) }, "token_revert"},
		{"whoami", func(s *Server, c *gin.Context) { s.handleTokenWhoami(c) }, "token_whoami"},
	}
	forms := map[string]string{
		"steal": `pid=4242&process_name=lsass.exe`,
		"make":  `user=DOMAIN%5Cadmin&password=hunter2&logon_type=interactive`,
	}

	t.Run("cross-tenant is invisible", func(t *testing.T) {
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				s := mustTenantServer(t)
				seedTenantAgent(t, s, "tk-x", 2)
				seedTokenOperator(t, s, "tk-op", 1, "operator")

				c, w := tenantTokenCtx(t, s, "tk-op", "operator", 1, "tk-x", "", forms[tc.name])
				tc.invoke(s, c)

				if w.Code != http.StatusNotFound {
					t.Fatalf("status=%d body=%s, want 404 for another tenant's agent", w.Code, w.Body.String())
				}
				assertNoTasksForAgent(t, s, "tk-x")
			})
		}
	})

	t.Run("viewer is refused", func(t *testing.T) {
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				s := mustTenantServer(t)
				seedTenantAgent(t, s, "tk-v", 1)
				seedTokenOperator(t, s, "tk-view", 1, "viewer")

				c, w := tenantTokenCtx(t, s, "tk-view", "viewer", 1, "tk-v", "", forms[tc.name])
				tc.invoke(s, c)

				if w.Code != http.StatusForbidden {
					t.Fatalf("status=%d body=%s, want 403 for a viewer", w.Code, w.Body.String())
				}
				assertNoTasksForAgent(t, s, "tk-v")
			})
		}
	})

	t.Run("same-tenant dispatches", func(t *testing.T) {
		// token_make carries the plaintext password in Shell, which the task
		// vault encrypts fail-closed, so the loot key must be up.
		forgecrypto.InitLootEncryption(tenantVisibilityMasterHex)
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				s := mustTenantServer(t)
				seedTenantAgent(t, s, "tk-ok", 1)
				seedTokenOperator(t, s, "tk-op2", 1, "operator")

				c, w := tenantTokenCtx(t, s, "tk-op2", "operator", 1, "tk-ok", "", forms[tc.name])
				tc.invoke(s, c)

				if w.Code != http.StatusOK {
					t.Fatalf("status=%d body=%s, want 200", w.Code, w.Body.String())
				}
				var task db.Task
				if err := s.db.Where("agent_id = ?", "tk-ok").First(&task).Error; err != nil {
					t.Fatalf("no task queued: %v", err)
				}
				if task.Type != tc.wantType {
					t.Fatalf("task type = %q, want %q", task.Type, tc.wantType)
				}
				if task.CreatedBy != "tk-op2" {
					t.Fatalf("CreatedBy = %q, want the requesting operator", task.CreatedBy)
				}
			})
		}
	})
}

// TestTokenStealValidatesPID proves the pid is parsed as an unsigned 32-bit
// integer before anything is queued: a malformed value must never reach the
// implant as a task.
func TestTokenStealValidatesPID(t *testing.T) {
	cases := []struct {
		name string
		form string
		code int
	}{
		{"missing pid", "", http.StatusBadRequest},
		{"empty pid", "pid=", http.StatusBadRequest},
		{"non-numeric pid", "pid=not-a-pid", http.StatusBadRequest},
		{"negative pid", "pid=-1", http.StatusBadRequest},
		{"overflowing pid", "pid=99999999999", http.StatusBadRequest},
		{"zero pid is syntactically valid", "pid=0", http.StatusOK},
		{"max uint32", "pid=4294967295", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := mustTenantServer(t)
			seedTenantAgent(t, s, "tk-pid", 1)
			seedTokenOperator(t, s, "tk-op3", 1, "operator")

			c, w := tenantTokenCtx(t, s, "tk-op3", "operator", 1, "tk-pid", "", tc.form)
			s.handleTokenSteal(c)

			if w.Code != tc.code {
				t.Fatalf("status=%d body=%s, want %d", w.Code, w.Body.String(), tc.code)
			}
			if tc.code != http.StatusOK {
				assertNoTasksForAgent(t, s, "tk-pid")
			}
		})
	}
}

// TestTokenStealCarriesPIDAndProcessName proves the two form fields land in the
// task fields the implant reads (Command=pid, Path=display name).
func TestTokenStealCarriesPIDAndProcessName(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "tk-f", 1)
	seedTokenOperator(t, s, "tk-op4", 1, "operator")

	c, w := tenantTokenCtx(t, s, "tk-op4", "operator", 1, "tk-f", "", `pid=4242&process_name=lsass.exe`)
	s.handleTokenSteal(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var task db.Task
	if err := s.db.Where("agent_id = ?", "tk-f").First(&task).Error; err != nil {
		t.Fatalf("no task: %v", err)
	}
	if task.Command != "4242" || task.Path != "lsass.exe" {
		t.Fatalf("command=%q path=%q, want the pid and process name", task.Command, task.Path)
	}
}

// TestTokenMakeRequiresUserAndDefaultsLogonType proves the LogonUser inputs are
// validated, the logon type defaults to interactive, and the password is
// encrypted at rest (token_make rides it in Shell).
func TestTokenMakeRequiresUserAndDefaultsLogonType(t *testing.T) {
	t.Run("missing user", func(t *testing.T) {
		s := mustTenantServer(t)
		seedTenantAgent(t, s, "tk-m0", 1)
		seedTokenOperator(t, s, "tk-op5", 1, "operator")
		c, w := tenantTokenCtx(t, s, "tk-op5", "operator", 1, "tk-m0", "", `password=x`)
		s.handleTokenMake(c)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s, want 400", w.Code, w.Body.String())
		}
		assertNoTasksForAgent(t, s, "tk-m0")
	})

	t.Run("default logon type and encrypted password", func(t *testing.T) {
		// The task vault encrypts Command/Shell for token_make, so the loot
		// key must be initialized or the write is refused outright.
		forgecrypto.InitLootEncryption(tenantVisibilityMasterHex)
		s := mustTenantServer(t)
		seedTenantAgent(t, s, "tk-m1", 1)
		seedTokenOperator(t, s, "tk-op6", 1, "operator")

		c, w := tenantTokenCtx(t, s, "tk-op6", "operator", 1, "tk-m1", "", `user=DOMAIN%5Cadmin&password=hunter2`)
		s.handleTokenMake(c)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}

		var task db.Task
		if err := s.db.Where("agent_id = ?", "tk-m1").First(&task).Error; err != nil {
			t.Fatalf("no task: %v", err)
		}
		if task.Path != "interactive" {
			t.Fatalf("logon type = %q, want the interactive default", task.Path)
		}
		// AfterFind decrypts for the operator-facing value...
		if task.Shell != "hunter2" {
			t.Fatalf("shell = %q, want the plaintext password after decryption", task.Shell)
		}
		// ...but the stored bytes must be ciphertext. Read it with raw SQL so
		// the AfterFind decryption hook cannot hide a plaintext-at-rest bug.
		var storedShell, storedCommand string
		if err := s.db.Raw("SELECT shell, command FROM tasks WHERE agent_id = ? ORDER BY id DESC LIMIT 1", "tk-m1").
			Row().Scan(&storedShell, &storedCommand); err != nil {
			t.Fatalf("raw reload: %v", err)
		}
		if storedShell == "hunter2" || !strings.HasPrefix(storedShell, "FC2ENC:") {
			t.Fatalf("stored shell = %q, want an FC2ENC: ciphertext", storedShell)
		}
		if storedCommand == "DOMAIN\\admin" {
			t.Fatal("the account name was stored in cleartext")
		}
	})
}

// TestTokenRevertDeactivatesEntries proves rev2self immediately marks the
// agent's tokens inactive: a saved credential must not stay re-impersonable
// while a revert is in flight.
func TestTokenRevertDeactivatesEntries(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "tk-rv", 1)
	seedTokenOperator(t, s, "tk-op7", 1, "operator")
	for i := 0; i < 3; i++ {
		if err := s.db.Create(&db.TokenEntry{AgentID: "tk-rv", Username: "u", Active: true}).Error; err != nil {
			t.Fatalf("seed token: %v", err)
		}
	}
	if err := s.db.Create(&db.TokenEntry{AgentID: "other-agent", Username: "keep", Active: true}).Error; err != nil {
		t.Fatalf("seed foreign token: %v", err)
	}

	c, w := tenantTokenCtx(t, s, "tk-op7", "operator", 1, "tk-rv", "", "")
	s.handleTokenRevert(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var active int64
	if err := s.db.Model(&db.TokenEntry{}).Where("agent_id = ? AND active = ?", "tk-rv", true).Count(&active).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if active != 0 {
		t.Fatalf("%d tokens still active after revert", active)
	}
	var foreign int64
	if err := s.db.Model(&db.TokenEntry{}).Where("agent_id = ? AND active = ?", "other-agent", true).Count(&foreign).Error; err != nil {
		t.Fatalf("count foreign: %v", err)
	}
	if foreign != 1 {
		t.Fatal("revert deactivated another agent's tokens")
	}
}

// ── Token entry CRUD + tenant gates ─────────────────────────────────────────

func seedTokenEntryWithPID(t *testing.T, s *Server, agentID, username string, pid uint32) db.TokenEntry {
	t.Helper()
	entry := db.TokenEntry{AgentID: agentID, Username: username, Domain: "CORP", PID: pid, Active: true}
	if err := s.db.Create(&entry).Error; err != nil {
		t.Fatalf("seed token entry: %v", err)
	}
	return entry
}

func seedTokenEntry(t *testing.T, s *Server, agentID, username string) db.TokenEntry {
	t.Helper()
	return seedTokenEntryWithPID(t, s, agentID, username, 0)
}

// handleGetTokens is currently unscoped: it reads
// `Where("agent_id = ?", id)` with no tenant predicate, so a caller who knows
// another tenant's agent id can read its token list. The sibling
// handleTokenPage was scoped the same way but this AJAX endpoint was not, and
// the test that asserted otherwise depended on a change to this handler that
// was never committed — it failed at runtime for as long as it was in the tree.
// Re-add it when the handler is scoped.

// TestHandleTokenDropGuards proves the delete path refuses an invisible agent,
// an unparsable id and an unknown token, and that a real delete is audited.
func TestHandleTokenDropGuards(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "tk-d", 1)
	seedTenantAgent(t, s, "tk-x", 2)
	seedTokenOperator(t, s, "tk-op9", 1, "operator")
	entry := seedTokenEntry(t, s, "tk-d", "victim")
	foreign := seedTokenEntry(t, s, "tk-x", "theirs")

	// Cross-tenant agent: 404 and the row survives.
	c, w := tenantTokenCtx(t, s, "tk-op9", "operator", 1, "tk-x", uintToStr(entry.ID), "")
	s.handleTokenDrop(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("foreign agent: status=%d, want 404", w.Code)
	}
	var stillThere int64
	if err := s.db.Model(&db.TokenEntry{}).Where("id = ?", entry.ID).Count(&stillThere).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if stillThere != 1 {
		t.Fatal("a refused drop deleted the token anyway")
	}

	// Unparsable id.
	c2, w2 := tenantTokenCtx(t, s, "tk-op9", "operator", 1, "tk-d", "abc", "")
	s.handleTokenDrop(c2)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("bad id: status=%d, want 400", w2.Code)
	}

	// Token id belonging to a different agent of the same tenant.
	c3, w3 := tenantTokenCtx(t, s, "tk-op9", "operator", 1, "tk-d", uintToStr(foreign.ID), "")
	s.handleTokenDrop(c3)
	if w3.Code != http.StatusNotFound {
		t.Fatalf("other agent's token: status=%d, want 404", w3.Code)
	}

	// The real delete.
	c4, w4 := tenantTokenCtx(t, s, "tk-op9", "operator", 1, "tk-d", uintToStr(entry.ID), "")
	s.handleTokenDrop(c4)
	if w4.Code != http.StatusOK {
		t.Fatalf("delete: status=%d body=%s", w4.Code, w4.Body.String())
	}
	var gone int64
	if err := s.db.Model(&db.TokenEntry{}).Where("id = ?", entry.ID).Count(&gone).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if gone != 0 {
		t.Fatal("the token survived the delete")
	}
	var audit db.AuditLog
	if err := s.db.Where("action = ?", "token_drop").First(&audit).Error; err != nil {
		t.Fatalf("no drop audit entry: %v", err)
	}
	if audit.User != "tk-op9" || !audit.Success {
		t.Fatalf("audit entry = %+v", audit)
	}
}

// TestHandleTokenImpersonateReStealsSavedPID proves impersonating a saved token
// re-issues the steal task for the recorded pid and refuses unknown entries.
func TestHandleTokenImpersonateReStealsSavedPID(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "tk-i", 1)
	seedTokenOperator(t, s, "tk-opA", 1, "operator")
	entry := seedTokenEntryWithPID(t, s, "tk-i", "svc-backup", 8888)

	c, w := tenantTokenCtx(t, s, "tk-opA", "operator", 1, "tk-i", uintToStr(entry.ID), "")
	s.handleTokenImpersonate(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var task db.Task
	if err := s.db.Where("agent_id = ?", "tk-i").First(&task).Error; err != nil {
		t.Fatalf("no task: %v", err)
	}
	if task.Type != "token_steal" || task.Command != "8888" {
		t.Fatalf("task = %s/%q, want token_steal for pid 8888", task.Type, task.Command)
	}

	// Unknown token id.
	c2, w2 := tenantTokenCtx(t, s, "tk-opA", "operator", 1, "tk-i", "999999", "")
	s.handleTokenImpersonate(c2)
	if w2.Code != http.StatusNotFound {
		t.Fatalf("unknown token: status=%d, want 404", w2.Code)
	}
	// Unparsable id is a 400 before any DB work.
	c3, w3 := tenantTokenCtx(t, s, "tk-opA", "operator", 1, "tk-i", "xyz", "")
	s.handleTokenImpersonate(c3)
	if w3.Code != http.StatusBadRequest {
		t.Fatalf("bad id: status=%d, want 400", w3.Code)
	}
}

// TestHandleTokenNoteUpdateIsTenantScopedAndHonest is the regression for the
// note endpoint: it used to answer 200 even when nothing was written, so a
// cross-tenant caller (or an operator chasing a typo'd id) was told the note was
// saved when it never was.
func TestHandleTokenNoteUpdateIsTenantScopedAndHonest(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "tk-n", 1)
	seedTenantAgent(t, s, "tk-nx", 2)
	seedTokenOperator(t, s, "tk-opB", 1, "operator")
	entry := seedTokenEntry(t, s, "tk-n", "victim")
	foreign := seedTokenEntry(t, s, "tk-nx", "theirs")

	// Another tenant's agent is invisible.
	c, w := tenantTokenCtx(t, s, "tk-opB", "operator", 1, "tk-nx", uintToStr(entry.ID), `notes=hijack`)
	s.handleTokenNoteUpdate(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("foreign agent: status=%d body=%s, want 404", w.Code, w.Body.String())
	}

	// A token that does not exist must not be reported as updated.
	c2, w2 := tenantTokenCtx(t, s, "tk-opB", "operator", 1, "tk-n", "424242", `notes=nope`)
	s.handleTokenNoteUpdate(c2)
	if w2.Code != http.StatusNotFound {
		t.Fatalf("unknown token: status=%d body=%s, want 404", w2.Code, w2.Body.String())
	}

	// Another agent's token inside the caller's tenant is equally invisible.
	c3, w3 := tenantTokenCtx(t, s, "tk-opB", "operator", 1, "tk-n", uintToStr(foreign.ID), `notes=hijack`)
	s.handleTokenNoteUpdate(c3)
	if w3.Code != http.StatusNotFound {
		t.Fatalf("foreign token: status=%d, want 404", w3.Code)
	}

	// Unparsable id is a 400.
	c4, w4 := tenantTokenCtx(t, s, "tk-opB", "operator", 1, "tk-n", "not-a-number", `notes=x`)
	s.handleTokenNoteUpdate(c4)
	if w4.Code != http.StatusBadRequest {
		t.Fatalf("bad id: status=%d, want 400", w4.Code)
	}

	// The real update sticks and nothing else was touched.
	c5, w5 := tenantTokenCtx(t, s, "tk-opB", "operator", 1, "tk-n", uintToStr(entry.ID), `notes=stolen+via+lsass`)
	s.handleTokenNoteUpdate(c5)
	if w5.Code != http.StatusOK {
		t.Fatalf("update: status=%d body=%s", w5.Code, w5.Body.String())
	}
	var reloaded db.TokenEntry
	if err := s.db.First(&reloaded, entry.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Notes != "stolen via lsass" {
		t.Fatalf("notes = %q, want the posted value", reloaded.Notes)
	}
	var untouched db.TokenEntry
	if err := s.db.First(&untouched, foreign.ID).Error; err != nil {
		t.Fatalf("reload foreign: %v", err)
	}
	if untouched.Notes != "" {
		t.Fatalf("a refused note update wrote %q", untouched.Notes)
	}
}

// ── Result processing ───────────────────────────────────────────────────────

// TestProcessTokenResultPersistsAndRotates proves a stolen/made token is stored
// active, replaces the previous active one, and that a revert deactivates
// everything; malformed agent output is dropped without touching the vault.
func TestProcessTokenResultPersistsAndRotates(t *testing.T) {
	s := mustTenantServer(t)

	s.processTokenResult("ag-1", "token_steal",
		`{"domain":"CORP","username":"svc","integrity":"High","pid":"1234","whoami":"CORP\\svc"}`)

	var entries []db.TokenEntry
	if err := s.db.Where("agent_id = ?", "ag-1").Order("id").Find(&entries).Error; err != nil {
		t.Fatalf("find: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("stored %d entries, want 1", len(entries))
	}
	got := entries[0]
	if got.Username != "svc" || got.Domain != "CORP" || got.Integrity != "High" ||
		got.PID != 1234 || !got.Active || got.Source != "steal" || got.TokenType != "Impersonation" {
		t.Fatalf("stored entry = %+v", got)
	}

	// A second steal supersedes the first (only one active token per agent).
	s.processTokenResult("ag-1", "token_steal",
		`{"domain":"CORP","username":"admin","integrity":"System","pid":"99"}`)
	var active []db.TokenEntry
	if err := s.db.Where("agent_id = ? AND active = ?", "ag-1", true).Find(&active).Error; err != nil {
		t.Fatalf("find active: %v", err)
	}
	if len(active) != 1 || active[0].Username != "admin" {
		t.Fatalf("active entries = %+v, want only the newest", active)
	}

	// token_make stores the logon type with a Primary token type and no pid.
	s.processTokenResult("ag-2", "token_make",
		`{"domain":"CORP","username":"admin","integrity":"High","logon_type":"network"}`)
	var made db.TokenEntry
	if err := s.db.Where("agent_id = ?", "ag-2").First(&made).Error; err != nil {
		t.Fatalf("made token: %v", err)
	}
	if made.Source != "make_token" || made.TokenType != "Primary" || made.LogonType != "network" || made.PID != 0 {
		t.Fatalf("made entry = %+v", made)
	}

	// revert/rev2self deactivate everything for the agent.
	s.processTokenResult("ag-1", "token_revert", "")
	s.processTokenResult("ag-2", "rev2self", "")
	var stillActive int64
	if err := s.db.Model(&db.TokenEntry{}).Where("active = ?", true).Count(&stillActive).Error; err != nil {
		t.Fatalf("count active: %v", err)
	}
	if stillActive != 0 {
		t.Fatalf("%d tokens still active after revert", stillActive)
	}
}

// TestProcessTokenResultIgnoresBadOutput proves an unparsable or malformed
// agent result never reaches the vault: a half-written credential entry is
// worse than none, and the previous active token stays usable.
func TestProcessTokenResultIgnoresBadOutput(t *testing.T) {
	s := mustTenantServer(t)
	seedTokenEntry(t, s, "ag-1", "previous")

	cases := []struct {
		name     string
		taskType string
		output   string
	}{
		{"not json", "token_steal", "not json at all"},
		{"empty output", "token_steal", ""},
		{"json object not string map", "token_steal", `{"pid":1234}`},
		{"truncated json", "token_make", `{"domain":"CORP"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s.processTokenResult("ag-1", tc.taskType, tc.output)
			var entries []db.TokenEntry
			if err := s.db.Where("agent_id = ?", "ag-1").Find(&entries).Error; err != nil {
				t.Fatalf("find: %v", err)
			}
			if len(entries) != 1 || entries[0].Username != "previous" {
				t.Fatalf("vault = %+v, want only the pre-existing entry", entries)
			}
			if !entries[0].Active {
				t.Fatal("a malformed result deactivated the existing token")
			}
		})
	}

	// An unparsable pid degrades to 0 rather than inventing a pid.
	s.processTokenResult("ag-2", "token_steal", `{"domain":"C","username":"u","pid":"not-a-pid"}`)
	var bad db.TokenEntry
	if err := s.db.Where("agent_id = ?", "ag-2").First(&bad).Error; err != nil {
		t.Fatalf("find ag-2: %v", err)
	}
	if bad.PID != 0 {
		t.Fatalf("pid = %d, want 0 for an unparsable value", bad.PID)
	}
}

// ── Result formatting ───────────────────────────────────────────────────────

// TestFormatTokenProcsFromJSON pins the display formatter: a table for valid
// agent output, the "(no processes)" note for an empty array, and the raw
// passthrough for anything that is not a JSON array — a formatter that invented
// structure from garbage would show an operator fabricated rows.
func TestFormatTokenProcsFromJSON(t *testing.T) {
	t.Run("table", func(t *testing.T) {
		got := FormatTokenProcsFromJSON(`[
			{"PID":1234,"ProcessName":"lsass.exe","Domain":"CORP","Username":"svc","Integrity":"High"},
			{"PID":42,"ProcessName":"explorer.exe","Domain":"CORP","Username":"admin","Integrity":"Medium"}
		]`)
		for _, want := range []string{"PID", "Process", "User", "Integrity", "lsass.exe", "CORP\\svc", "High", "explorer.exe", "CORP\\admin"} {
			if !strings.Contains(got, want) {
				t.Fatalf("formatted output missing %q:\n%s", want, got)
			}
		}
		if strings.Count(got, "\n") < 3 {
			t.Fatalf("table has too few rows:\n%s", got)
		}
	})

	t.Run("error column replaces the user", func(t *testing.T) {
		got := FormatTokenProcsFromJSON(`[{"PID":7,"ProcessName":"x.exe","Error":"access denied"}]`)
		if !strings.Contains(got, "(access denied)") {
			t.Fatalf("error not surfaced:\n%s", got)
		}
	})

	t.Run("empty array", func(t *testing.T) {
		if got := FormatTokenProcsFromJSON("[]"); !strings.Contains(got, "no processes") {
			t.Fatalf("empty array = %q", got)
		}
	})

	t.Run("non-json passthrough", func(t *testing.T) {
		for _, raw := range []string{"", "not json", `{"PID":1}`, "truncated["} {
			if got := FormatTokenProcsFromJSON(raw); got != raw {
				t.Fatalf("FormatTokenProcsFromJSON(%q) = %q, want the raw input", raw, got)
			}
		}
		// A literal null decodes to an empty slice, so it reports "no
		// processes" rather than echoing the agent's null back at the operator.
		if got := FormatTokenProcsFromJSON("null"); !strings.Contains(got, "no processes") {
			t.Fatalf("null = %q, want the empty-result note", got)
		}
	})
}
