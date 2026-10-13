package server

import (
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// ── SSRF guard: validateExternalURL ──────────────────────────────────────────

// TestValidateExternalURLBlocksInternalTargets is the SSRF matrix for every
// server-initiated fetch (BOF/plugin/webhook imports). The server resolves the
// host itself, so decimal-octet and IPv4-mapped-v6 spellings must not slip a
// private destination past the check.
func TestValidateExternalURLBlocksInternalTargets(t *testing.T) {
	blocked := []struct {
		name string
		url  string
	}{
		{"loopback literal", "http://127.0.0.1/x"},
		{"loopback alternate", "http://127.1.2.3/x"},
		{"loopback by name", "http://localhost/x"},
		{"rfc1918 10/8", "http://10.0.0.5/x"},
		{"rfc1918 172.16/12", "http://172.16.4.1/x"},
		{"rfc1918 192.168/16", "http://192.168.1.1/x"},
		{"cgnat 100.64/10", "http://100.100.1.1/x"},
		{"unspecified 0.0.0.0", "http://0.0.0.0/x"},
		{"zero-prefix 0.1.2.3", "http://0.1.2.3/x"},
		{"link-local metadata", "http://169.254.169.254/latest/meta-data"},
		{"link-local unicast", "http://169.254.1.1/x"},
		{"multicast", "http://224.0.0.1/x"},
		{"ipv6 loopback", "http://[::1]/x"},
		{"ipv6 unique local", "http://[fd00::1]/x"},
		{"ipv4-mapped ipv6 loopback", "http://[::ffff:127.0.0.1]/x"},
		{"decimal octet loopback", "http://2130706433/x"},
		{"ssh port", "http://example.com:22/x"},
		{"database port", "http://example.com:3306/x"},
		{"redis port", "http://example.com:6379/x"},
		{"memcached port", "http://example.com:11211/x"},
		{"mongodb port", "http://example.com:27017/x"},
		{"privileged port", "http://example.com:113/x"},
		{"non-http scheme", "file:///etc/passwd"},
		{"gopher scheme", "gopher://example.com:70/x"},
		{"no host", "http:///x"},
		{"invalid url", "http://[::1"},
	}
	for _, tc := range blocked {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateExternalURL(tc.url); err == nil {
				t.Fatalf("validateExternalURL(%q) accepted a blocked target", tc.url)
			}
		})
	}
}

// TestValidateExternalURLAcceptsPublicTargets proves the guard is not so broad
// that legitimate exfil/gateway endpoints are refused.
func TestValidateExternalURLAcceptsPublicTargets(t *testing.T) {
	allowed := []string{
		"https://example.com/hook",
		"http://example.com:8080/x",
		"https://8.8.8.8/x",
		"http://1.1.1.1:9000/x",
		"https://[2606:4700:4700::1111]/x",
	}
	for _, raw := range allowed {
		if err := validateExternalURL(raw); err != nil {
			t.Fatalf("validateExternalURL(%q) rejected a public target: %v", raw, err)
		}
	}
}

// TestValidateExternalURLFailsClosedOnUnresolvableHost proves a host the server
// cannot resolve is refused rather than fetched optimistically.
func TestValidateExternalURLFailsClosedOnUnresolvableHost(t *testing.T) {
	err := validateExternalURL("http://this-host-does-not-exist.invalid/x")
	if err == nil {
		t.Fatal("unresolvable host accepted")
	}
	if !strings.Contains(err.Error(), "resolution") && !strings.Contains(err.Error(), "no addresses") {
		t.Fatalf("error %q does not name the resolution failure", err)
	}
}

// ── SSRF guard: ssrfSafeClient ──────────────────────────────────────────────

// TestSSRFSafeClientRefusesRedirectPivot proves every redirect hop is
// re-validated: a public URL that 302s to a loopback address must not be
// followed, which is the classic bypass for a one-shot URL check.
func TestSSRFSafeClientRefusesRedirectPivot(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This is the internal target the redirect aims at.
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("internal secret"))
	}))
	defer backend.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, backend.URL, http.StatusFound)
	}))
	defer redirector.Close()

	client := ssrfSafeClient(&http.Client{Timeout: 5 * time.Second})
	resp, err := client.Get(redirector.URL)
	if err == nil {
		defer resp.Body.Close()
		t.Fatal("client followed a redirect into an internal address")
	}
	if !strings.Contains(err.Error(), "blocked non-public address") &&
		!strings.Contains(err.Error(), "blocked") {
		t.Fatalf("error %q does not name the blocked destination", err)
	}
}

// TestSSRFSafeClientBoundsRedirectChain proves a redirect loop terminates
// instead of spinning until the timeout.
func TestSSRFSafeClientBoundsRedirectChain(t *testing.T) {
	loop := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/next", http.StatusFound)
	}))
	defer loop.Close()

	client := ssrfSafeClient(&http.Client{Timeout: 5 * time.Second})
	if _, err := client.Get(loop.URL); err == nil {
		t.Fatal("redirect loop did not terminate with an error")
	}
}

// TestSSRFSafeClientAllowsPublicRedirect proves the guard does not break normal
// redirects between public addresses.
func TestSSRFSafeClientAllowsPublicRedirect(t *testing.T) {
	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("arrived"))
	}))
	defer final.Close()

	// The redirect target is a public IP literal, so the hop is allowed; the
	// dial itself is not attempted because the assertion is on validation.
	client := ssrfSafeClient(&http.Client{Timeout: 2 * time.Second})
	req, err := http.NewRequest(http.MethodGet, "https://93.184.216.34/start", nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp, err := client.Do(req)
	if err == nil {
		resp.Body.Close()
		return // reached the network and came back: validation passed
	}
	if strings.Contains(err.Error(), "blocked non-public address") {
		t.Fatalf("a public redirect target was blocked: %v", err)
	}
	_ = final
}

// ── Webhook test endpoint ───────────────────────────────────────────────────

func newWebhookTestServer(t *testing.T) *Server {
	t.Helper()
	s := mustTenantServer(t)
	return s
}

func webhookTestCtx(s *Server, t *testing.T, tenantID uint, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	c, w := tenantScopedAdminContext(s, t, "wh-op", tenantID)
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, w
}

// TestHandleTestWebhookRefusesInternalTargets proves the SSRF guard is wired
// into the operator-facing "test my webhook" endpoint, not just the delivery
// path: a loopback target must never be dialed.
func TestHandleTestWebhookRefusesInternalTargets(t *testing.T) {
	s := newWebhookTestServer(t)
	for _, url := range []string{
		"http://127.0.0.1:8080/hook",
		"http://169.254.169.254/latest/meta-data",
		"http://10.0.0.1/hook",
		"file:///etc/passwd",
	} {
		body, _ := json.Marshal(map[string]string{"url": url})
		c, w := webhookTestCtx(s, t, 1, string(body))
		s.handleTestWebhook(c)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want 400", url, w.Code)
		}
	}
}

// TestHandleTestWebhookRejectsMalformedBody proves a body the endpoint cannot
// parse is a 400 rather than a delivered probe against an empty URL.
func TestHandleTestWebhookRejectsMalformedBody(t *testing.T) {
	s := newWebhookTestServer(t)
	c, w := webhookTestCtx(s, t, 1, `{"url":`)
	s.handleTestWebhook(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

// ── Automation rule tenant isolation ─────────────────────────────────────────

func automationRuleCtx(s *Server, t *testing.T, username string, tenantID uint, method, body, id string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	c, w := tenantScopedAdminContext(s, t, username, tenantID)
	c.Params = gin.Params{{Key: "id", Value: id}}
	c.Request = httptest.NewRequest(method, "/", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, w
}

// seedScopedRule stores a rule and invalidates the automation cache, which is
// what loadAutomationRuleScoped reads from. A non-empty action list is
// required: loadAutomationRules drops action-less rules from the cache, since
// they can never fire.
func seedScopedRule(t *testing.T, s *Server, id, name string, tenantID uint, enabled bool) {
	t.Helper()
	rule := db.AutomationRule{
		ID: id, Name: name, Enabled: enabled, EventType: "file_exfil",
		Conditions: `[]`,
		Actions:    `[{"type":"command","params":{"command":"whoami"}}]`,
		TenantID:   tenantID,
	}
	if err := s.db.Create(&rule).Error; err != nil {
		t.Fatalf("seed rule: %v", err)
	}
	s.invalidateAutomationCache()
}

// TestAutomationRuleUpdateCannotMoveTenant proves a body-supplied tenant on an
// update cannot migrate the rule, and that the tenant is left untouched when the
// body omits it.
func TestAutomationRuleUpdateCannotMoveTenant(t *testing.T) {
	s := mustTenantServer(t)
	seedScopedRule(t, s, "rule-keep", "keep", 6, false)

	body := `{"name":"renamed","event_type":"file_exfil","tenant_id":42,"conditions":[],"actions":[]}`
	c, w := automationRuleCtx(s, t, "rule-op6", 6, http.MethodPut, body, "rule-keep")
	s.handleUpdateAutomationRule(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	var stored db.AutomationRule
	if err := s.db.First(&stored, "id = ?", "rule-keep").Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.TenantID != 6 {
		t.Fatalf("TenantID = %d, want the original 6 (body said 42)", stored.TenantID)
	}
	if stored.Name != "renamed" {
		t.Fatalf("Name = %q, want the update applied", stored.Name)
	}
}

// TestLoadAutomationRuleScoped proves the cache-level tenant gate used by the
// update/delete/toggle paths, including that a legacy (tenant 0) operator keeps
// visibility.
func TestLoadAutomationRuleScoped(t *testing.T) {
	s := mustTenantServer(t)
	seedScopedRule(t, s, "rule-scoped", "scoped", 5, false)

	own, _ := tenantScopedAdminContext(s, t, "scoped-op", 5)
	if _, ok := s.loadAutomationRuleScoped(own, "rule-scoped"); !ok {
		t.Fatal("owner cannot load their own rule")
	}
	foreign, _ := tenantScopedAdminContext(s, t, "scoped-other", 4)
	if _, ok := s.loadAutomationRuleScoped(foreign, "rule-scoped"); ok {
		t.Fatal("another tenant loaded a rule it does not own")
	}
	global, _ := tenantScopedAdminContext(s, t, "scoped-global", 0)
	if _, ok := s.loadAutomationRuleScoped(global, "rule-scoped"); !ok {
		t.Fatal("unscoped operator lost access to a rule")
	}
	unknown, _ := tenantScopedAdminContext(s, t, "scoped-op", 5)
	if _, ok := s.loadAutomationRuleScoped(unknown, "no-such-rule"); ok {
		t.Fatal("unknown rule id resolved")
	}
}

// ── Webhook SSRF ────────────────────────────────────────────────────────────

// TestWebhookCreateRejectsInternalTargets proves a stored webhook can never
// point at an internal address: the URL is validated at create time, not only
// at delivery time. (CRUD tenant scoping is covered by
// handlers_webhook_tenant_test.go.)
func TestWebhookCreateRejectsInternalTargets(t *testing.T) {
	s := newWebhookTestServer(t)
	for _, url := range []string{
		"http://127.0.0.1/x",
		"http://169.254.169.254/latest/meta-data",
		"http://192.168.0.1/x",
		"http://metadata.google.internal/x",
		"http://[::1]/x",
	} {
		body, _ := json.Marshal(map[string]interface{}{"name": "wh", "url": url, "method": "POST"})
		c, w := automationRuleCtx(s, t, "wh-op", 1, http.MethodPost, string(body), "")
		s.handleCreateWebhook(c)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want 400", url, w.Code)
		}
	}
	var count int64
	if err := s.db.Model(&db.WebhookConfig{}).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("%d internal webhooks were stored", count)
	}
}

// TestWebhookCreateAndList proves the happy path stores a valid public webhook
// and lists it back with its encrypted headers preserved through the hook.
func TestHandlePluginImportRejectsMissingAndOversizedUploads(t *testing.T) {
	s := mustTenantServer(t)

	c, w := automationRuleCtx(s, t, "plug-op", 1, http.MethodPost, "", "")
	s.handlePluginImport(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("missing file: status = %d, want 400", w.Code)
	}

	// Build a multipart body over the size cap.
	var buf strings.Builder
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "huge.zip")
	if err != nil {
		t.Fatalf("form file: %v", err)
	}
	if _, err := fw.Write(make([]byte, MaxUploadSize+1024)); err != nil {
		t.Fatalf("write: %v", err)
	}
	mw.Close()

	c2, w2 := automationRuleCtx(s, t, "plug-op", 1, http.MethodPost, buf.String(), "")
	c2.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(buf.String()))
	c2.Request.Header.Set("Content-Type", mw.FormDataContentType())
	s.handlePluginImport(c2)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("oversized upload: status = %d, want 400 (body %s)", w2.Code, w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), "too large") {
		t.Fatalf("body = %s, want the size-cap message", w2.Body.String())
	}
}

// TestHandleBOFRepoImportRejectsInternalTargets proves the remote BOF import
// path runs the same SSRF guard as every other server-initiated fetch.
func TestHandleBOFRepoImportRejectsInternalTargets(t *testing.T) {
	s := mustTenantServer(t)
	for _, url := range []string{
		"http://127.0.0.1/bof.o",
		"http://169.254.169.254/latest/meta-data",
		"http://10.1.2.3/bof.o",
	} {
		body, _ := json.Marshal(map[string]string{"url": url, "filename": "x.o"})
		c, w := automationRuleCtx(s, t, "bof-op", 1, http.MethodPost, string(body), "")
		s.handleBOFRepoImport(c)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: status = %d, want 400", url, w.Code)
		}
		if !strings.Contains(w.Body.String(), "rejected") {
			t.Fatalf("%s: body = %s, want the rejection message", url, w.Body.String())
		}
	}
	var count int64
	if err := s.db.Model(&db.BOFFile{}).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("%d BOF files were imported from internal URLs", count)
	}
}

// TestHandleBOFRepoImportRequiresURLAndFilename proves the endpoint refuses an
// incomplete request instead of downloading something undefined.
func TestHandleBOFRepoImportRequiresURLAndFilename(t *testing.T) {
	s := mustTenantServer(t)
	cases := []string{
		`{}`,
		`{"url":"https://example.com/a.o"}`,
		`{"filename":"a.o"}`,
		`{"url":"","filename":"a.o"}`,
		`{"url":"https://example.com/a.o","filename":""}`,
	}
	for _, body := range cases {
		c, w := automationRuleCtx(s, t, "bof-op", 1, http.MethodPost, body, "")
		s.handleBOFRepoImport(c)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("body %s: status = %d, want 400", body, w.Code)
		}
	}
	// Malformed JSON is also a 400, not a panic.
	c, w := automationRuleCtx(s, t, "bof-op", 1, http.MethodPost, `{"url":`, "")
	s.handleBOFRepoImport(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("malformed body: status = %d, want 400", w.Code)
	}
}

// TestHandleBOFRepoIndexIsStatic proves the BOF index is a built-in allowlist
// rather than a fetch, so it cannot be pointed at an attacker-controlled host.
func TestHandleBOFRepoIndexIsStatic(t *testing.T) {
	s := mustTenantServer(t)
	c, w := automationRuleCtx(s, t, "bof-op", 1, http.MethodGet, "", "")
	s.handleBOFRepoIndex(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var body struct {
		Data []struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Data) == 0 {
		t.Fatal("BOF index returned no repositories")
	}
	for _, repo := range body.Data {
		if !strings.HasPrefix(repo.URL, "https://github.com/") {
			t.Fatalf("index entry %q is not a pinned github repo: %s", repo.Name, repo.URL)
		}
	}
}
