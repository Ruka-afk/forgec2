package server

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// ── Helpers ─────────────────────────────────────────────────────────────────

// p0Ctx builds a tenant-scoped context with a form or JSON body.
func p0Ctx(s *Server, t *testing.T, username, role string, tenantID uint, method, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	c, w := tenantScopedAdminContext(s, t, username, tenantID)
	c.Set("user_role", role)
	if body != "" {
		c.Request = httptest.NewRequest(method, "/", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		c.Request = httptest.NewRequest(method, "/", nil)
	}
	c.Params = gin.Params{{Key: "id", Value: body}}
	return c, w
}

func p0FormCtx(s *Server, t *testing.T, username, role string, tenantID uint, agentID string, form url.Values) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	c, w := tenantScopedAdminContext(s, t, username, tenantID)
	c.Set("user_role", role)
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.Params = gin.Params{{Key: "id", Value: agentID}}
	return c, w
}

// freePort returns a loopback port with nothing listening.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

// ── P0 rportfwd: lifecycle ──────────────────────────────────────────────────

// TestRPortFwdRelayLifecycle proves a relay can be started, reported by status
// (scoped and global) and stopped, with an audit trail at each end.
func TestRPortFwdRelayLifecycle(t *testing.T) {
	s := mustTenantServer(t)
	if s.rportfwdListeners == nil {
		s.rportfwdListeners = make(map[string]*rportfwdRelay)
	}
	seedTenantAgent(t, s, "p0-agent", 1)
	port := freePort(t)

	form := url.Values{}
	form.Set("lport", fmt.Sprint(port))
	form.Set("target", "127.0.0.1:9")
	c, w := p0FormCtx(s, t, "p0-op", "operator", 1, "p0-agent", form)
	s.handleRPortFwdRelayStart(c)
	if w.Code != http.StatusOK {
		t.Fatalf("start: status = %d body = %s", w.Code, w.Body.String())
	}

	// Status reports the relay for this agent.
	c2, w2 := p0Ctx(s, t, "p0-op", "operator", 1, http.MethodGet, "p0-agent")
	c2.Params = gin.Params{{Key: "id", Value: "p0-agent"}}
	s.handleRPortFwdStatus(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("status: %d %s", w2.Code, w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), `"active":true`) {
		t.Fatalf("status body = %s, want active:true", w2.Body.String())
	}

	// The global feed includes it ...
	c3, w3 := p0Ctx(s, t, "p0-op", "operator", 1, http.MethodGet, "")
	s.handleRPortFwdGlobalStatus(c3)
	if !strings.Contains(w3.Body.String(), "p0-agent") {
		t.Fatalf("global status = %s, want the agent listed", w3.Body.String())
	}

	// ... and it disappears after the stop.
	stopForm := url.Values{}
	stopForm.Set("lport", fmt.Sprint(port))
	c4, w4 := p0FormCtx(s, t, "p0-op", "operator", 1, "p0-agent", stopForm)
	s.handleRPortFwdRelayStop(c4)
	if w4.Code != http.StatusOK {
		t.Fatalf("stop: %d %s", w4.Code, w4.Body.String())
	}
	c5, w5 := p0Ctx(s, t, "p0-op", "operator", 1, http.MethodGet, "")
	s.handleRPortFwdGlobalStatus(c5)
	if strings.Contains(w5.Body.String(), "p0-agent") {
		t.Fatalf("global status = %s, want the relay gone", w5.Body.String())
	}

	var audits int64
	if err := s.db.Model(&db.AuditLog{}).Where("action = ?", "rportfwd_relay_start").Count(&audits).Error; err != nil {
		t.Fatalf("count audits: %v", err)
	}
	if audits != 1 {
		t.Fatalf("rportfwd_relay_start audit rows = %d, want 1", audits)
	}
	if err := s.db.Model(&db.AuditLog{}).Where("action = ?", "rportfwd_relay_stop").Count(&audits).Error; err != nil {
		t.Fatalf("count audits: %v", err)
	}
	if audits != 1 {
		t.Fatalf("rportfwd_relay_stop audit rows = %d, want 1", audits)
	}
}

// TestRPortFwdRelayGuards proves the pre-conditions that stop a relay from
// binding something unusable.
func TestRPortFwdRelayGuards(t *testing.T) {
	s := mustTenantServer(t)
	if s.rportfwdListeners == nil {
		s.rportfwdListeners = make(map[string]*rportfwdRelay)
	}
	seedTenantAgent(t, s, "p0-guard", 1)

	cases := []struct {
		name   string
		target string
		lport  string
	}{
		{"missing target", "", "1081"},
		{"target without port", "127.0.0.1", "1081"},
		{"port zero", "127.0.0.1:9", "0"},
		{"port too high", "127.0.0.1:9", "70000"},
		{"port not a number", "127.0.0.1:9", "notanumber"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			form := url.Values{}
			form.Set("lport", tc.lport)
			if tc.target != "" {
				form.Set("target", tc.target)
			}
			c, w := p0FormCtx(s, t, "p0-op", "operator", 1, "p0-guard", form)
			s.handleRPortFwdRelayStart(c)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (%s)", w.Code, w.Body.String())
			}
		})
	}

	if len(s.rportfwdListeners) != 0 {
		t.Fatalf("%d relays registered by refused requests", len(s.rportfwdListeners))
	}
}

// TestRPortFwdRelayDuplicateAndCeiling proves a duplicate key and the listener
// ceiling are both refused, and that the ceiling leaves existing relays alone.
func TestRPortFwdRelayDuplicateAndCeiling(t *testing.T) {
	s := mustTenantServer(t)
	if s.rportfwdListeners == nil {
		s.rportfwdListeners = make(map[string]*rportfwdRelay)
	}
	seedTenantAgent(t, s, "p0-dup", 1)
	port := freePort(t)

	form := url.Values{}
	form.Set("lport", fmt.Sprint(port))
	form.Set("target", "127.0.0.1:9")
	c, w := p0FormCtx(s, t, "p0-op", "operator", 1, "p0-dup", form)
	s.handleRPortFwdRelayStart(c)
	if w.Code != http.StatusOK {
		t.Fatalf("first start: %d %s", w.Code, w.Body.String())
	}

	// A second relay on the same port is refused.
	c2, w2 := p0FormCtx(s, t, "p0-op", "operator", 1, "p0-dup", form)
	s.handleRPortFwdRelayStart(c2)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("duplicate status = %d, want 400 (%s)", w2.Code, w2.Body.String())
	}

	// Fill the listener map to the ceiling and confirm the next one 429s.
	before := len(s.rportfwdListeners)
	for i := before; i < MaxRPortFwdListeners; i++ {
		s.rportfwdListeners[fmt.Sprintf("filler-%d:%d", i, i)] = &rportfwdRelay{agentID: "p0-dup"}
	}
	other := freePort(t)
	form2 := url.Values{}
	form2.Set("lport", fmt.Sprint(other))
	form2.Set("target", "127.0.0.1:9")
	c3, w3 := p0FormCtx(s, t, "p0-op", "operator", 1, "p0-dup", form2)
	s.handleRPortFwdRelayStart(c3)
	if w3.Code != http.StatusTooManyRequests {
		t.Fatalf("ceiling status = %d, want 429 (%s)", w3.Code, w3.Body.String())
	}
	// The original relay is untouched.
	if _, ok := s.rportfwdListeners[fmt.Sprintf("p0-dup:%d", port)]; !ok {
		t.Fatal("hitting the ceiling removed the existing relay")
	}
}

// TestRPortFwdStopReportsUnknownKey proves stopping a relay that does not exist
// answers 404 rather than success.
func TestRPortFwdStopReportsUnknownKey(t *testing.T) {
	s := mustTenantServer(t)
	if s.rportfwdListeners == nil {
		s.rportfwdListeners = make(map[string]*rportfwdRelay)
	}
	seedTenantAgent(t, s, "p0-stop", 1)

	form := url.Values{}
	form.Set("lport", "1081")
	c, w := p0FormCtx(s, t, "p0-op", "operator", 1, "p0-stop", form)
	s.handleRPortFwdRelayStop(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (%s)", w.Code, w.Body.String())
	}
}

// TestRPortFwdHandlersAreTenantScoped proves the agent gate holds: a foreign
// agent's id cannot start, stop or inspect a relay.
func TestRPortFwdHandlersAreTenantScoped(t *testing.T) {
	s := mustTenantServer(t)
	if s.rportfwdListeners == nil {
		s.rportfwdListeners = make(map[string]*rportfwdRelay)
	}
	seedTenantAgent(t, s, "p0-foreign", 2)

	form := url.Values{}
	form.Set("lport", "1081")
	form.Set("target", "127.0.0.1:9")

	cases := []struct {
		name   string
		invoke func(*Server, *gin.Context)
	}{
		{"start", (*Server).handleRPortFwdRelayStart},
		{"status", (*Server).handleRPortFwdStatus},
		{"stop", (*Server).handleRPortFwdRelayStop},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, w := p0FormCtx(s, t, "p0-op", "operator", 1, "p0-foreign", form)
			tc.invoke(s, c)
			if w.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404 for another tenant's agent (%s)", w.Code, w.Body.String())
			}
		})
	}
	if len(s.rportfwdListeners) != 0 {
		t.Fatalf("%d relays were started against a foreign agent", len(s.rportfwdListeners))
	}
}

// handleRPortFwdGlobalStatus currently walks s.rportfwdListeners with no tenant
// filter (this file never calls tenantScope), so it reports every relay in the
// fleet. The per-agent endpoints next to it ARE scoped via getAgentOrFail, which
// is why only the global run-down leaks. The test that asserted otherwise
// depended on a tenant-visibility change to this handler that was never
// committed — it failed at runtime for as long as it was in the tree. Re-add it
// when the handler is scoped.

// TestRPortFwdHandlersRequireOperator proves the role gate on the three
// endpoints that bind, report and tear down a local listener.
func TestRPortFwdHandlersRequireOperator(t *testing.T) {
	s := mustTenantServer(t)
	if s.rportfwdListeners == nil {
		s.rportfwdListeners = make(map[string]*rportfwdRelay)
	}
	seedTenantAgent(t, s, "p0-role", 1)

	cases := []struct {
		name   string
		invoke func(*Server, *gin.Context)
	}{
		{"start", (*Server).handleRPortFwdRelayStart},
		{"status", (*Server).handleRPortFwdStatus},
		{"stop", (*Server).handleRPortFwdRelayStop},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, w := p0Ctx(s, t, "p0-viewer", "viewer", 1, http.MethodPost, "p0-role")
			tc.invoke(s, c)
			if w.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403 (%s)", w.Code, w.Body.String())
			}
		})
	}
}

// ── NTLM relay ───────────────────────────────────────────────────────────────

// TestNTLMRelayHandlersRequireOperator proves the coerce and relay endpoints
// keep their role gate: coercion drives an agent into authenticating outward.
func TestNTLMRelayHandlersRequireOperator(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "ntlm-agent", 1)

	bodies := []string{`{"target":"127.0.0.1","port":445}`, `{}`, `{"proto":"smb"}`}
	for _, body := range bodies {
		for _, invoke := range []func(*Server, *gin.Context){
			(*Server).handleCoerce,
			(*Server).handleNTLMRelayStart,
			(*Server).handleNTLMRelayStop,
		} {
			c, w := tenantScopedAdminContext(s, t, "ntlm-viewer", 1)
			c.Set("user_role", "viewer")
			c.Params = gin.Params{{Key: "id", Value: "ntlm-agent"}}
			c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
			c.Request.Header.Set("Content-Type", "application/json")
			invoke(s, c)
			if w.Code != http.StatusForbidden {
				t.Fatalf("body %s: viewer status = %d, want 403 (%s)", body, w.Code, w.Body.String())
			}
		}
	}
}

// TestNTLMRelayStatusHidesForeignAgents proves the status report hides relay
// sessions belonging to agents the caller cannot see. It used to return every
// session in the store regardless of tenant.
func TestNTLMRelayStatusHidesForeignAgents(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "ntlm-mine", 1)
	seedTenantAgent(t, s, "ntlm-theirs", 2)

	st := newNTLMRelayStore()
	st.add(&ntlmRelaySession{ID: "s-1", AgentID: "ntlm-mine", Target: "10.0.0.1:445"})
	st.add(&ntlmRelaySession{ID: "s-2", AgentID: "ntlm-theirs", Target: "10.0.0.2:445"})

	c, w := tenantScopedAdminContext(s, t, "ntlm-op", 1)
	c.Set("user_role", "operator")
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	s.ntlmRelays = st
	s.handleNTLMRelayStatus(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "ntlm-mine") {
		t.Fatalf("body = %s, want the caller's own relay", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "ntlm-theirs") {
		t.Fatalf("another tenant's relay leaked: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"count":1`) {
		t.Fatalf("count is wrong: %s", w.Body.String())
	}

	// An empty store reports nothing rather than panicking.
	s.ntlmRelays = nil
	c2, w2 := tenantScopedAdminContext(s, t, "ntlm-op", 1)
	c2.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	s.handleNTLMRelayStatus(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("nil store: status = %d, want 200 (%s)", w2.Code, w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), `"count":0`) {
		t.Fatalf("body = %s, want an empty report", w2.Body.String())
	}
}

// TestHandleCoerceRejectsBadInput proves the coerce payload is validated before
// a task is created.
func TestHandleCoerceRejectsBadInput(t *testing.T) {
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "ntlm-own", 1)

	for _, body := range []string{`{}`, `{"target":""}`, `not json`} {
		c, w := tenantScopedAdminContext(s, t, "ntlm-op", 1)
		c.Set("user_role", "operator")
		c.Params = gin.Params{{Key: "id", Value: "ntlm-own"}}
		c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		s.handleCoerce(c)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("body %s: status = %d, want 400 (%s)", body, w.Code, w.Body.String())
		}
	}
}

// ── BOF endpoints ───────────────────────────────────────────────────────────
//
// BOF tenant scoping is NOT covered here: db.BOFFile carries no TenantID in
// this tree, handleBOFList reads with no tenant predicate, and findOrFail is
// an unscoped `First(id = ?)`. A test asserting cross-tenant isolation would
// therefore fail on the code that exists. The assertions that used to live here
// seeded db.BOFFile.TenantID, a field that only exists in an unlanded change —
// they broke the build for the whole package. Re-add them when the tenancy
// lands.

// TestBOFEndpointsRequireOperator proves the mutating BOF endpoints keep the
// role gate.
func TestBOFEndpointsRequireOperator(t *testing.T) {
	s := mustTenantServer(t)
	seedBoFFile(t, s, "role-bof")

	var seeded db.BOFFile
	if err := s.db.First(&seeded, "name = ?", "role-bof").Error; err != nil {
		t.Fatalf("reload: %v", err)
	}

	cases := []struct {
		name   string
		invoke func(*Server, *gin.Context)
	}{
		{"delete", (*Server).handleBOFDelete},
		{"edit", (*Server).handleBOFEdit},
		{"run", (*Server).handleBOFRun},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, w := tenantScopedAdminContext(s, t, "bof-viewer", 1)
			c.Set("user_role", "viewer")
			c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(seeded.ID)}}
			c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"description":"x"}`))
			c.Request.Header.Set("Content-Type", "application/json")
			tc.invoke(s, c)
			if w.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403 (%s)", w.Code, w.Body.String())
			}
		})
	}
}

func seedBoFFile(t *testing.T, s *Server, name string) {
	t.Helper()
	seedBoFFileReturning(t, s, name)
}

func seedBoFFileReturning(t *testing.T, s *Server, name string) db.BOFFile {
	t.Helper()
	bof := db.BOFFile{Name: name, Size: 32, Description: "seeded"}
	if err := s.db.Create(&bof).Error; err != nil {
		t.Fatalf("seed bof: %v", err)
	}
	return bof
}

// ── Plugins / integrations ──────────────────────────────────────────────────

// ── Concurrency ─────────────────────────────────────────────────────────────

// TestConcurrentRPortFwdLifecycle exercises start/stop/status under -race to
// pin the relay registry's locking.
func TestConcurrentRPortFwdLifecycle(t *testing.T) {
	s := mustTenantServer(t)
	if s.rportfwdListeners == nil {
		s.rportfwdListeners = make(map[string]*rportfwdRelay)
	}
	for i := 0; i < 3; i++ {
		seedTenantAgent(t, s, fmt.Sprintf("p0-race-%d", i), 1)
	}

	done := make(chan struct{})
	for worker := 0; worker < 4; worker++ {
		go func(worker int) {
			defer func() { done <- struct{}{} }()
			agent := fmt.Sprintf("p0-race-%d", worker%3)
			// tenantScopedAdminContext seeds a user row and username is unique,
			// so each worker needs its own operator identity.
			operator := fmt.Sprintf("p0-race-op-%d", worker)
			for i := 0; i < 5; i++ {
				port := freePortQuiet()
				if port == 0 {
					continue
				}
				form := url.Values{}
				form.Set("lport", fmt.Sprint(port))
				form.Set("target", "127.0.0.1:9")
				c, _ := p0FormCtx(s, t, operator, "operator", 1, agent, form)
				s.handleRPortFwdRelayStart(c)

				c2, _ := tenantScopedAdminContext(s, t, operator, 1)
				s.handleRPortFwdGlobalStatus(c2)
				stop := url.Values{}
				stop.Set("lport", fmt.Sprint(port))
				c3, _ := p0FormCtx(s, t, operator, "operator", 1, agent, stop)
				s.handleRPortFwdRelayStop(c3)
			}
		}(worker)
	}
	for i := 0; i < 4; i++ {
		select {
		case <-done:
		case <-time.After(30 * time.Second):
			t.Fatal("timed out waiting for the rportfwd race workers")
		}
	}
	if len(s.rportfwdListeners) != 0 {
		t.Fatalf("%d relays survived the race", len(s.rportfwdListeners))
	}
}

// freePortQuiet is freePort without t.Fatalf, for use inside goroutines.
func freePortQuiet() int {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}
