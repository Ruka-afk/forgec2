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

// procCtx builds an operator-authenticated form POST pointed at :id = agentID.
func procCtx(username, agentID, form string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.Set("user", username)
	c.Set("user_role", "operator")
	c.Params = gin.Params{{Key: "id", Value: agentID}}
	return c, w
}

// tenantProcCtx is procCtx with the tenant resolution pinned so
// tenantScope scopes the agent lookup (a bare context would fall back to
// legacy-unscoped and hide cross-tenant regressions). The loot vault is
// initialized because Task.Command is encrypted at rest by a write hook
// (fail-closed).
func tenantProcCtx(s *Server, t *testing.T, username string, tenantID uint, agentID, form string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	forgecrypto.InitLootEncryption(tenantVisibilityMasterHex)
	c, w := procCtx(username, agentID, form)
	c.Set(tenantIDContextKey, tenantResolution{tid: tenantID, ok: true})
	var user db.User
	if err := s.db.Where("username = ?", username).First(&user).Error; err != nil {
		t.Fatalf("seed caller: %v", err)
	}
	c.Set("user_id", user.ID)
	return c, w
}

// TestSimpleTaskHandlersDispatch covers the createSimpleTask family (no
// request body): every one must 404 a foreign agent, queue the right task
// type for an own-tenant agent, and refuse a viewer.
func TestSimpleTaskHandlersDispatch(t *testing.T) {
	cases := []struct {
		name     string
		invoke   func(s *Server, c *gin.Context)
		wantType string
	}{
		{"ps", func(s *Server, c *gin.Context) { s.handleRequestPS(c) }, "ps"},
		{"process_tree", func(s *Server, c *gin.Context) { s.handleCollectProcesses(c) }, "process_tree"},
		{"window_list", func(s *Server, c *gin.Context) { s.handleWindowList(c) }, "window_list"},
		{"clipboard_get", func(s *Server, c *gin.Context) { s.handleClipboardGet(c) }, "clipboard_get"},
		{"drives", func(s *Server, c *gin.Context) { s.handleListDrives(c) }, "drives"},
		{"beacon_now", func(s *Server, c *gin.Context) { s.handleBeaconNow(c) }, "beacon_now"},
		{"services", func(s *Server, c *gin.Context) { s.handleListServices(c) }, "services"},
		{"netstat", func(s *Server, c *gin.Context) { s.handleNetstat(c) }, "netstat"},
		{"users", func(s *Server, c *gin.Context) { s.handleUsers(c) }, "users"},
		{"av", func(s *Server, c *gin.Context) { s.handleAV(c) }, "av"},
		{"reboot", func(s *Server, c *gin.Context) { s.handleReboot(c) }, "reboot"},
		{"shutdown", func(s *Server, c *gin.Context) { s.handleShutdown(c) }, "shutdown"},
		{"uninstall", func(s *Server, c *gin.Context) { s.handleUninstall(c) }, "uninstall"},
	}
	for _, tc := range cases {
		t.Run(tc.name+"/cross-tenant", func(t *testing.T) {
			s := mustTenantServer(t)
			seedTenantAgent(t, s, "sp-x", 2)
			if err := s.db.Create(&db.User{Username: "sp-op", TenantID: 1, Role: "operator", IsActive: true}).Error; err != nil {
				t.Fatalf("seed caller: %v", err)
			}

			c, w := tenantProcCtx(s, t, "sp-op", 1, "sp-x", "")
			tc.invoke(s, c)

			if w.Code != http.StatusNotFound {
				t.Fatalf("status=%d body=%s, want 404 for another tenant's agent", w.Code, w.Body.String())
			}
			assertNoTasksForAgent(t, s, "sp-x")
		})
		t.Run(tc.name+"/same-tenant", func(t *testing.T) {
			s := mustTenantServer(t)
			seedTenantAgent(t, s, "sp-ok", 1)
			if err := s.db.Create(&db.User{Username: "sp-op2", TenantID: 1, Role: "operator", IsActive: true}).Error; err != nil {
				t.Fatalf("seed caller: %v", err)
			}

			c, w := tenantProcCtx(s, t, "sp-op2", 1, "sp-ok", "")
			tc.invoke(s, c)

			if w.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s, want 200", w.Code, w.Body.String())
			}
			var task db.Task
			if err := s.db.Where("agent_id = ?", "sp-ok").First(&task).Error; err != nil {
				t.Fatalf("no task queued: %v", err)
			}
			if task.Type != tc.wantType {
				t.Fatalf("task type = %q, want %q", task.Type, tc.wantType)
			}
		})
		t.Run(tc.name+"/viewer-403", func(t *testing.T) {
			s := mustTenantServer(t)
			seedTenantAgent(t, s, "sp-view", 1)
			if err := s.db.Create(&db.User{Username: "sp-vw", TenantID: 1, Role: "viewer", IsActive: true}).Error; err != nil {
				t.Fatalf("seed viewer: %v", err)
			}

			c, w := tenantProcCtx(s, t, "sp-vw", 1, "sp-view", "")
			c.Set("user_role", "viewer")
			tc.invoke(s, c)

			if w.Code != http.StatusForbidden {
				t.Fatalf("status=%d, want 403 for a viewer", w.Code)
			}
			assertNoTasksForAgent(t, s, "sp-view")
		})
	}
}

// TestParamTaskHandlersDispatch covers the handlers that read a form field
// and forward it as the task command.
func TestParamTaskHandlersDispatch(t *testing.T) {
	cases := []struct {
		name     string
		form     string
		invoke   func(s *Server, c *gin.Context)
		wantType string
		wantCmd  string
		wantCode int
	}{
		{"suspend ok", `target=4242`, func(s *Server, c *gin.Context) { s.handleSuspendProcess(c) }, "suspend", `4242`, http.StatusOK},
		{"suspend command fallback", `command=99`, func(s *Server, c *gin.Context) { s.handleSuspendProcess(c) }, "suspend", `99`, http.StatusOK},
		{"suspend missing target", "", func(s *Server, c *gin.Context) { s.handleSuspendProcess(c) }, "", "", http.StatusBadRequest},
		{"suspend bad pid", `target=not-a-pid`, func(s *Server, c *gin.Context) { s.handleSuspendProcess(c) }, "", "", http.StatusBadRequest},
		{"suspend negative pid", `target=-1`, func(s *Server, c *gin.Context) { s.handleSuspendProcess(c) }, "", "", http.StatusBadRequest},
		{"resume ok", `target=7`, func(s *Server, c *gin.Context) { s.handleResumeProcess(c) }, "resume", `7`, http.StatusOK},
		{"resume missing target", "", func(s *Server, c *gin.Context) { s.handleResumeProcess(c) }, "", "", http.StatusBadRequest},
		{"kill ok", `target=13`, func(s *Server, c *gin.Context) { s.handleKillProcess(c) }, "killproc", `13`, http.StatusOK},
		{"kill missing target", "", func(s *Server, c *gin.Context) { s.handleKillProcess(c) }, "", "", http.StatusBadRequest},
		{"window close ok", `target=MyWindow`, func(s *Server, c *gin.Context) { s.handleWindowClose(c) }, "window_close", `MyWindow`, http.StatusOK},
		{"window close missing", "", func(s *Server, c *gin.Context) { s.handleWindowClose(c) }, "", "", http.StatusBadRequest},
		{"clipboard set ok", `data=hello`, func(s *Server, c *gin.Context) { s.handleClipboardSet(c) }, "clipboard_set", `hello`, http.StatusOK},
		{"clipboard set missing", "", func(s *Server, c *gin.Context) { s.handleClipboardSet(c) }, "", "", http.StatusBadRequest},
		{"reg get ok", `key=HKLM\SOFTWARE`, func(s *Server, c *gin.Context) { s.handleRegGet(c) }, "reg_get", `HKLM\SOFTWARE`, http.StatusOK},
		{"reg get missing", "", func(s *Server, c *gin.Context) { s.handleRegGet(c) }, "", "", http.StatusBadRequest},
		{"reg set ok", `path=HKCU\X&data=v`, func(s *Server, c *gin.Context) { s.handleRegSet(c) }, "reg_set", ``, http.StatusOK},
		{"reg set missing path", `data=v`, func(s *Server, c *gin.Context) { s.handleRegSet(c) }, "", "", http.StatusBadRequest},
		{"reg delete ok", `key=HKCU\X`, func(s *Server, c *gin.Context) { s.handleRegDelete(c) }, "reg_delete", `HKCU\X`, http.StatusOK},
		{"reg delete missing", "", func(s *Server, c *gin.Context) { s.handleRegDelete(c) }, "", "", http.StatusBadRequest},
		{"portscan ok", `target=10.0.0.1`, func(s *Server, c *gin.Context) { s.handlePortScan(c) }, "portscan", `10.0.0.1`, http.StatusOK},
		// find's spec requires `command`; the form's pattern is forwarded
		// when present and the search root is carried in Path (plus mirrored
		// into command when no pattern was supplied, since the gate rejects
		// an empty command).
		{"find ok", `path=C:\&pattern=*.txt`, func(s *Server, c *gin.Context) { s.handleFindFiles(c) }, "find", `*.txt`, http.StatusOK},
		{"find command fallback", `command=C:\Windows`, func(s *Server, c *gin.Context) { s.handleFindFiles(c) }, "find", `C:\Windows`, http.StatusOK},
		{"wallpaper ok", `image=https://x/a.png`, func(s *Server, c *gin.Context) { s.handleWallpaper(c) }, "wallpaper", `https://x/a.png`, http.StatusOK},
		{"wallpaper missing", "", func(s *Server, c *gin.Context) { s.handleWallpaper(c) }, "", "", http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := mustTenantServer(t)
			seedTenantAgent(t, s, "pp-ok", 1)
			if err := s.db.Create(&db.User{Username: "pp-op", TenantID: 1, Role: "operator", IsActive: true}).Error; err != nil {
				t.Fatalf("seed caller: %v", err)
			}

			c, w := tenantProcCtx(s, t, "pp-op", 1, "pp-ok", tc.form)
			tc.invoke(s, c)

			if w.Code != tc.wantCode {
				t.Fatalf("status=%d body=%s, want %d", w.Code, w.Body.String(), tc.wantCode)
			}
			if tc.wantCode != http.StatusOK {
				assertNoTasksForAgent(t, s, "pp-ok")
				return
			}
			var task db.Task
			if err := s.db.Where("agent_id = ?", "pp-ok").First(&task).Error; err != nil {
				t.Fatalf("no task queued: %v", err)
			}
			if task.Type != tc.wantType {
				t.Fatalf("task type = %q, want %q", task.Type, tc.wantType)
			}
			if tc.wantCmd != "" && task.Command != tc.wantCmd {
				t.Fatalf("command = %q, want %q", task.Command, tc.wantCmd)
			}
		})
	}
}

// TestParamTaskHandlersCrossTenant proves the param family enforces the
// tenant scope on the agent (id from the path) before dispatching.
func TestParamTaskHandlersCrossTenant(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "pp-x", 2)
	if err := s.db.Create(&db.User{Username: "pp-o", TenantID: 1, Role: "operator", IsActive: true}).Error; err != nil {
		t.Fatalf("seed caller: %v", err)
	}

	c, w := tenantProcCtx(s, t, "pp-o", 1, "pp-x", "target=4242")
	s.handleKillProcess(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404 for another tenant's agent", w.Code)
	}
	assertNoTasksForAgent(t, s, "pp-x")
}

// TestRegSetCarriesPathAndData proves reg_set stores the registry path and
// value in the right task fields (they map to Path/Data, not Command).
func TestRegSetCarriesPathAndData(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "rs-ok", 1)
	if err := s.db.Create(&db.User{Username: "rs-op", TenantID: 1, Role: "operator", IsActive: true}).Error; err != nil {
		t.Fatalf("seed caller: %v", err)
	}

	c, w := tenantProcCtx(s, t, "rs-op", 1, "rs-ok", `path=HKCU%5CEnvironment&data=evil`)
	s.handleRegSet(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var task db.Task
	if err := s.db.Where("agent_id = ?", "rs-ok").First(&task).Error; err != nil {
		t.Fatalf("no task: %v", err)
	}
	if task.Path != `HKCU\Environment` || task.Data != "evil" {
		t.Fatalf("task path=%q data=%q, want the registry path and value", task.Path, task.Data)
	}
}

// TestDownloadURLValidationPreserved proves the SSRF guard on the
// download-URL endpoint survived (it validates like any other
// server-initiated fetch).
func TestDownloadURLValidationPreserved(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "dl-ok", 1)
	if err := s.db.Create(&db.User{Username: "dl-op", TenantID: 1, Role: "operator", IsActive: true}).Error; err != nil {
		t.Fatalf("seed caller: %v", err)
	}

	// A loopback URL must be refused before a task is queued.
	c, w := tenantProcCtx(s, t, "dl-op", 1, "dl-ok", `url=http://127.0.0.1/x&dest=C:\x`)
	s.handleDownloadURL(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("loopback url: status=%d, want 400", w.Code)
	}
	assertNoTasksForAgent(t, s, "dl-ok")
}
