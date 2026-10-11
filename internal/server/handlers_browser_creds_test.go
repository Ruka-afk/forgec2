package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	forgecrypto "github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// collectorCtx mirrors the operator context the browser-credential handlers
// run under: seeded user (so tenant scope resolves), post-form body.
func collectorCtx(s *Server, t *testing.T, username, body string) (*gin.Context, *httptest.ResponseRecorder) {
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

// TestBrowserCollectorsCrossTenant404 proves all five collectors refuse
// another tenant's agent. handleWifiCreds used to bypass issueAgentTask with
// a bare Create, so it accepted any agent id; this locks the regression.
func TestBrowserCollectorsCrossTenant404(t *testing.T) {
	cases := []struct {
		name   string
		invoke func(s *Server, c *gin.Context)
	}{
		{"cookie_export", func(s *Server, c *gin.Context) { s.handleCookieExport(c) }},
		{"sccm_recon", func(s *Server, c *gin.Context) { s.handleSccmRecon(c) }},
		{"entra_prt", func(s *Server, c *gin.Context) { s.handleEntraPRT(c) }},
		{"vpn_creds", func(s *Server, c *gin.Context) { s.handleVpnCreds(c) }},
		{"wifi_creds", func(s *Server, c *gin.Context) { s.handleWifiCreds(c) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := mustTenantServer(t)
			seedTenantAgent(t, s, "bc-x", 2)

			c, w := collectorCtx(s, t, "bc-viewer", "")
			c.Params = gin.Params{{Key: "id", Value: "bc-x"}}
			tc.invoke(s, c)

			if w.Code != http.StatusNotFound {
				t.Fatalf("status=%d body=%s, want 404 for another tenant's agent", w.Code, w.Body.String())
			}
			assertNoTasksForAgent(t, s, "bc-x")
		})
	}
}

// TestBrowserCollectorsSameTenantDispatch proves each collector queues the
// right task type against an own-tenant agent (cookie_export carries the
// browser selector, defaulting to "all").
func TestBrowserCollectorsSameTenantDispatch(t *testing.T) {
	cases := []struct {
		name    string
		invoke  func(s *Server, c *gin.Context)
		wantTyp string
	}{
		{"cookie_export", func(s *Server, c *gin.Context) { s.handleCookieExport(c) }, "cookie_export"},
		{"sccm_recon", func(s *Server, c *gin.Context) { s.handleSccmRecon(c) }, "sccm_recon"},
		{"entra_prt", func(s *Server, c *gin.Context) { s.handleEntraPRT(c) }, "entra_prt"},
		{"vpn_creds", func(s *Server, c *gin.Context) { s.handleVpnCreds(c) }, "vpn_creds"},
		{"wifi_creds", func(s *Server, c *gin.Context) { s.handleWifiCreds(c) }, "wifi_creds"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := mustTenantServer(t)
			seedTenantAgent(t, s, "bc-ok", 1)

			c, w := collectorCtx(s, t, "bc-op", "")
			c.Params = gin.Params{{Key: "id", Value: "bc-ok"}}
			tc.invoke(s, c)

			if w.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s, want 200", w.Code, w.Body.String())
			}
			var task db.Task
			if err := s.db.Where("agent_id = ?", "bc-ok").First(&task).Error; err != nil {
				t.Fatalf("no task queued: %v", err)
			}
			if task.Type != tc.wantTyp {
				t.Fatalf("task type = %q, want %q", task.Type, tc.wantTyp)
			}
		})
	}
}

// TestCookieExportDefaultsBrowserAll proves the browser selector defaults to
// "all" (sent as the task command).
func TestCookieExportDefaultsBrowserAll(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "bc-cookie", 1)

	c, w := collectorCtx(s, t, "bc-op", "")
	c.Params = gin.Params{{Key: "id", Value: "bc-cookie"}}
	s.handleCookieExport(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	var task db.Task
	if err := s.db.Where("agent_id = ?", "bc-cookie").First(&task).Error; err != nil {
		t.Fatalf("no task: %v", err)
	}
	if task.Command != "all" {
		t.Fatalf("command = %q, want the default all", task.Command)
	}
}

// TestProcessBrowserResultFailsClosedWithoutVault proves a vault failure
// stores nothing rather than plaintext: with the loot key uninitialized the
// per-credential encryption fails and the entry must be skipped.
func TestProcessBrowserResultFailsClosedWithoutVault(t *testing.T) {
	if !lootUninitialized() {
		t.Skip("loot key already initialized by another test in this binary")
	}
	gin.SetMode(gin.TestMode)
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "bc-res", 1)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(
		`{"task_id":1,"agent_id":"bc-res","credentials":["browser|https://x|alice|pw123|c=1"]}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	s.handleProcessBrowserResult(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var n int64
	if err := s.db.Model(&db.CredentialEntry{}).Count(&n).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("vault failure stored %d credential row(s); fail-closed is broken", n)
	}
	var payload struct {
		Created int `json:"created"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Created != 0 {
		t.Fatalf("created=%d, want 0 when the vault is down", payload.Created)
	}
}

// TestProcessBrowserResultStoresEncrypted proves the happy path: each
// credential is stored with encrypted notes (never the raw pipe string) and
// the task is marked completed.
func TestProcessBrowserResultStoresEncrypted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	forgecrypto.InitLootEncryption(tenantVisibilityMasterHex)
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "bc-res2", 1)
	if err := s.db.Create(&db.Task{AgentID: "bc-res2", Type: "browser_steal", Status: "running"}).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}

	const raw = "browser|https://x|alice|pw123|c=1"
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(
		`{"task_id":1,"agent_id":"bc-res2","credentials":["`+raw+`","browser|https://y|bob|pw456"]}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	s.handleProcessBrowserResult(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var rows []db.CredentialEntry
	if err := s.db.Find(&rows).Error; err != nil {
		t.Fatalf("find: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("stored %d rows, want 2", len(rows))
	}
	for _, r := range rows {
		if r.Notes == "" {
			t.Fatal("notes empty; the raw credential was not captured")
		}
		if bytes.Contains([]byte(r.Notes), []byte("pw123")) || bytes.Contains([]byte(r.Notes), []byte("browser|")) {
			t.Fatalf("notes stored the raw credential in cleartext: %q", r.Notes)
		}
		if r.TenantID != 1 {
			t.Fatalf("tenant = %d, want inheritance from the agent", r.TenantID)
		}
	}
	// Decrypting back must recover the original string.
	dec := decryptCredNotes(rows[0].Notes)
	if dec != raw {
		t.Fatalf("round-trip mismatch: %q", dec)
	}
	var task db.Task
	if err := s.db.First(&task, 1).Error; err != nil {
		t.Fatalf("reload task: %v", err)
	}
	if task.Status != "completed" {
		t.Fatalf("task status = %q, want completed", task.Status)
	}
}

// TestProcessWifiResultParsesSSIDWithColonPassword proves the SSID:password
// split keeps colons inside the password and skips malformed entries.
func TestProcessWifiResultParsesSSIDWithColonPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)
	forgecrypto.InitLootEncryption(tenantVisibilityMasterHex)
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "bc-wifi", 1)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(
		`{"task_id":1,"agent_id":"bc-wifi","networks":["HomeNet:p@ss:word","NoColonHere","Corp:secret"]}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	s.handleProcessWifiResult(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var rows []db.CredentialEntry
	if err := s.db.Find(&rows).Error; err != nil {
		t.Fatalf("find: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("stored %d rows, want 2 (the colonless entry must be skipped)", len(rows))
	}
	var home db.CredentialEntry
	if err := s.db.Where("username = ?", "HomeNet").First(&home).Error; err != nil {
		t.Fatalf("HomeNet row missing: %v", err)
	}
	if home.Password != "p@ss:word" {
		t.Fatalf("password = %q, want the full p@ss:word (SplitN 2 must keep inner colons)", home.Password)
	}
}
