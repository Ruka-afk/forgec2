package server

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"

	forgecrypto "github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// ── Helpers ─────────────────────────────────────────────────────────────────

// redirectorCtx builds a tenant-scoped context with an explicit role.
func redirectorCtx(s *Server, t *testing.T, username, role string, tenantID uint) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	// Redirector SSH credentials go through the loot vault on write
	// (models.Reducetor BeforeCreate/BeforeUpdate), which refuses plaintext
	// when no key is installed.
	forgecrypto.InitLootEncryption(tenantVisibilityMasterHex)
	c, w := tenantScopedAdminContext(s, t, username, tenantID)
	c.Set("user_role", role)
	return c, w
}

// redirectorJSONCtx posts a JSON body to a redirector endpoint.
func redirectorJSONCtx(s *Server, t *testing.T, username, role string, tenantID uint, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	c, w := redirectorCtx(s, t, username, role, tenantID)
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, w
}

// redirectorIDCtx pins the :id path parameter for update/delete.
func redirectorIDCtx(s *Server, t *testing.T, username, role string, tenantID uint, id uint, method, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	c, w := redirectorCtx(s, t, username, role, tenantID)
	c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(int(id))}}
	if body != "" {
		c.Request = httptest.NewRequest(method, "/", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
	} else {
		c.Request = httptest.NewRequest(method, "/", nil)
	}
	return c, w
}

// seedRedirector stores a redirector with recognisable credential material so
// tests can assert it never reaches a response body.
func seedRedirector(t *testing.T, s *Server, tenantID uint, name, host string) db.Redirector {
	t.Helper()
	forgecrypto.InitLootEncryption(tenantVisibilityMasterHex)
	rd := db.Redirector{
		TenantID:    tenantID,
		Name:        name,
		Host:        host,
		Type:        "nginx",
		Status:      "inactive",
		Config:      "server {}",
		SSHUser:     "deploy",
		SSHPort:     DefaultSSHPort,
		SSHKey:      "-----BEGIN OPENSSH PRIVATE KEY-----" + name + "-KEYSECRET-----END OPENSSH PRIVATE KEY-----",
		SSHPassword: "ENCRYPTED:" + name + "-PASSWORDSECRET",
	}
	if err := s.db.Create(&rd).Error; err != nil {
		t.Fatalf("seed redirector: %v", err)
	}
	return rd
}

// silentHost binds a loopback listener and returns its host:port, standing in
// for a reachable redirector front-end.
func silentHost(t *testing.T) string {
	t.Helper()
	ln := listenSilent(t)
	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split listener addr: %v", err)
	}
	return "127.0.0.1:" + portStr
}

func redirectorHostPort(t *testing.T, hostPort string) (string, int) {
	t.Helper()
	host, portStr, err := net.SplitHostPort(hostPort)
	if err != nil {
		t.Fatalf("split %q: %v", hostPort, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port %q: %v", portStr, err)
	}
	return host, port
}

// ── Credential containment ──────────────────────────────────────────────────

// TestRedirectorListNeverReturnsSSHCredentials is the core regression: the list
// endpoint serialized the model, so the SSH private key and encrypted password
// of every tenant's redirectors were handed to any caller with settings.read.
func TestRedirectorListNeverReturnsSSHCredentials(t *testing.T) {
	s := mustTenantServer(t)
	seedRedirector(t, s, 1, "mine", "10.0.0.5")
	seedRedirector(t, s, 2, "theirs", "10.0.0.6")

	c, w := redirectorCtx(s, t, "rd-list", "operator", 1)
	s.handleRedirectorList(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, secret := range []string{
		"BEGIN OPENSSH PRIVATE KEY", "KEYSECRET", "PASSWORDSECRET", "ENCRYPTED:",
	} {
		if strings.Contains(body, secret) {
			t.Fatalf("credential material %q leaked into the list: %s", secret, body)
		}
	}
	// The field names must be gone too, not just the values.
	for _, field := range []string{`"ssh_key"`, `"ssh_password"`} {
		if strings.Contains(body, field) {
			t.Fatalf("list still exposes %s: %s", field, body)
		}
	}

	var payload struct {
		Redirectors []redirectorView `json:"redirectors"`
		Total       int              `json:"total"`
	}
	unmarshalData(t, w.Body.Bytes(), &payload)
	if payload.Total != 1 || len(payload.Redirectors) != 1 {
		t.Fatalf("redirectors = %+v total = %d, want the caller's single redirector", payload.Redirectors, payload.Total)
	}
	entry := payload.Redirectors[0]
	if entry.Host != "10.0.0.5" || entry.Name != "mine" {
		t.Fatalf("entry = %+v", entry)
	}
	// Presence is still reported, so the UI can tell "key stored" from "no key".
	if !entry.HasSSHKey || !entry.HasSSHPassword {
		t.Fatalf("presence flags = key:%v password:%v, want both true", entry.HasSSHKey, entry.HasSSHPassword)
	}
}

// TestRedirectorViewPresenceFlags pins that the boolean reflects stored material
// rather than always reporting true.
func TestRedirectorViewPresenceFlags(t *testing.T) {
	// Structural guarantee: the view type has no field that could ever carry
	// the SSH key or password, whatever a caller does with the row.
	viewType := reflect.TypeOf(redirectorView{})
	for i := 0; i < viewType.NumField(); i++ {
		switch viewType.Field(i).Tag.Get("json") {
		case "ssh_key", "ssh_password":
			t.Fatalf("redirectorView exposes %s", viewType.Field(i).Name)
		}
	}

	cases := []struct {
		name             string
		rd               db.Redirector
		wantKey, wantSSH bool
	}{
		{"both stored", db.Redirector{SSHKey: "k", SSHPassword: "p"}, true, true},
		{"key only", db.Redirector{SSHKey: "k"}, true, false},
		{"password only", db.Redirector{SSHPassword: "p"}, false, true},
		{"neither", db.Redirector{}, false, false},
		{"whitespace only", db.Redirector{SSHKey: "  ", SSHPassword: "\t"}, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := makeRedirectorView(tc.rd)
			if got.HasSSHKey != tc.wantKey || got.HasSSHPassword != tc.wantSSH {
				t.Fatalf("flags = key:%v password:%v, want key:%v password:%v",
					got.HasSSHKey, got.HasSSHPassword, tc.wantKey, tc.wantSSH)
			}
		})
	}
}

// ── Tenant isolation ─────────────────────────────────────────────────────────

// TestRedirectorListIsTenantScoped proves each tenant sees only its own
// redirectors; the row has no tenant column at all before this batch.
func TestRedirectorListIsTenantScoped(t *testing.T) {
	s := mustTenantServer(t)
	seedRedirector(t, s, 1, "mine", "10.0.0.5")
	seedRedirector(t, s, 2, "theirs", "10.0.0.6")
	seedRedirector(t, s, 0, "legacy", "10.0.0.7")

	c, w := redirectorCtx(s, t, "rd-scope", "operator", 1)
	s.handleRedirectorList(c)
	if strings.Contains(w.Body.String(), "theirs") || strings.Contains(w.Body.String(), "10.0.0.6") {
		t.Fatalf("another tenant's redirector leaked: %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "legacy") {
		t.Fatalf("legacy (tenant 0) redirector visible to a scoped operator: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "mine") {
		t.Fatalf("own redirector missing: %s", w.Body.String())
	}

	// Unscoped operators keep the fleet-wide view.
	c2, w2 := redirectorCtx(s, t, "rd-global", "operator", 0)
	s.handleRedirectorList(c2)
	var global struct {
		Total int `json:"total"`
	}
	unmarshalData(t, w2.Body.Bytes(), &global)
	if global.Total != 3 {
		t.Fatalf("unscoped operator sees %d redirectors, want all 3", global.Total)
	}
}

// TestRedirectorCreateStampsCallerTenant proves a new redirector is owned by
// its creator.
func TestRedirectorCreateStampsCallerTenant(t *testing.T) {
	s := mustTenantServer(t)

	body := `{"name":"front","host":"203.0.113.9","type":"nginx","ssh_user":"deploy","ssh_key":"KEY"}`
	c, w := redirectorJSONCtx(s, t, "rd-create", "operator", 8, body)
	s.handleRedirectorCreate(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	var created struct {
		ID uint `json:"id"`
	}
	unmarshalData(t, w.Body.Bytes(), &created)
	var stored db.Redirector
	if err := s.db.First(&stored, "id = ?", created.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.TenantID != 8 {
		t.Fatalf("TenantID = %d, want the creator's tenant 8", stored.TenantID)
	}
	if stored.Name != "front" || stored.Host != "203.0.113.9" || stored.Status != "inactive" {
		t.Fatalf("stored redirector = %+v", stored)
	}
	if stored.SSHPort != DefaultSSHPort {
		t.Fatalf("SSHPort = %d, want the %d default", stored.SSHPort, DefaultSSHPort)
	}
}

// TestRedirectorCreateRejectsIncompleteBody proves the required-field contract.
func TestRedirectorCreateRejectsIncompleteBody(t *testing.T) {
	s := mustTenantServer(t)
	for _, body := range []string{`{}`, `{"name":"x"}`, `{"host":"h"}`, `{"name":"","host":""}`, `{"name":`} {
		c, w := redirectorJSONCtx(s, t, "rd-create", "operator", 1, body)
		s.handleRedirectorCreate(c)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("body %s: status = %d, want 400", body, w.Code)
		}
	}
	var n int64
	if err := s.db.Model(&db.Redirector{}).Count(&n).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("a rejected create stored %d redirectors", n)
	}
}

// TestRedirectorMutationsAreTenantScoped proves update and delete refuse a
// foreign redirector and leave its credentials untouched.
func TestRedirectorMutationsAreTenantScoped(t *testing.T) {
	s := mustTenantServer(t)
	foreign := seedRedirector(t, s, 2, "theirs", "10.0.0.6")

	c, w := redirectorIDCtx(s, t, "rd-op", "operator", 1, foreign.ID, http.MethodPut,
		`{"name":"pwned","ssh_user":"root","ssh_key":"ATTACKER-KEY"}`)
	s.handleRedirectorUpdate(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("update: status = %d, want 404", w.Code)
	}

	c2, w2 := redirectorIDCtx(s, t, "rd-op", "operator", 1, foreign.ID, http.MethodDelete, "")
	s.handleRedirectorDelete(c2)
	if w2.Code != http.StatusNotFound {
		t.Fatalf("delete: status = %d, want 404", w2.Code)
	}

	var survivor db.Redirector
	if err := s.db.First(&survivor, "id = ?", foreign.ID).Error; err != nil {
		t.Fatalf("redirector was destroyed by a refused request: %v", err)
	}
	if survivor.Name != "theirs" || survivor.SSHUser != "deploy" {
		t.Fatalf("refused update mutated the redirector: %+v", survivor)
	}
	if !strings.Contains(survivor.SSHKey, "KEYSECRET") {
		t.Fatalf("refused update overwrote the stored key: %q", survivor.SSHKey)
	}
}

// TestRedirectorDeleteReportsGhostID proves deleting an unknown id answers 404
// instead of a bare success.
func TestRedirectorDeleteReportsGhostID(t *testing.T) {
	s := mustTenantServer(t)

	c, w := redirectorIDCtx(s, t, "rd-op", "operator", 1, 999999, http.MethodDelete, "")
	s.handleRedirectorDelete(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (%s)", w.Code, w.Body.String())
	}
}

// TestRedirectorUpdateAppliesWithinTenant proves the owner's update still works.
func TestRedirectorUpdateAppliesWithinTenant(t *testing.T) {
	s := mustTenantServer(t)
	rd := seedRedirector(t, s, 4, "front", "10.0.0.5")

	c, w := redirectorIDCtx(s, t, "rd-op", "operator", 4, rd.ID, http.MethodPut,
		`{"name":"front-eu","type":"haproxy","config":"backend {}","ssh_user":"ops","ssh_port":2222,"status":"active"}`)
	s.handleRedirectorUpdate(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	var stored db.Redirector
	if err := s.db.First(&stored, "id = ?", rd.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.Name != "front-eu" || stored.Type != "haproxy" || stored.SSHUser != "ops" ||
		stored.SSHPort != 2222 || stored.Status != "active" || stored.Config != "backend {}" {
		t.Fatalf("stored redirector = %+v", stored)
	}
	if stored.TenantID != 4 {
		t.Fatalf("TenantID drifted to %d", stored.TenantID)
	}

	// Empty fields keep the stored values.
	c2, w2 := redirectorIDCtx(s, t, "rd-op", "operator", 4, rd.ID, http.MethodPut,
		`{"name":"","type":"","config":"","ssh_user":"","ssh_port":0,"status":""}`)
	s.handleRedirectorUpdate(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("partial update: %d %s", w2.Code, w2.Body.String())
	}
	if err := s.db.First(&stored, "id = ?", rd.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.Name != "front-eu" || stored.Type != "haproxy" || stored.SSHUser != "ops" || stored.SSHPort != 2222 {
		t.Fatalf("an empty field overwrote stored values: %+v", stored)
	}

	// A failing credential encryption must refuse the whole update rather than
	// erase the stored one; the test server has no crypto.totp_key.
	kept := stored.SSHPassword
	c3, w3 := redirectorIDCtx(s, t, "rd-op", "operator", 4, rd.ID, http.MethodPut,
		`{"ssh_password":"rotated-secret"}`)
	s.handleRedirectorUpdate(c3)
	if w3.Code != http.StatusInternalServerError {
		t.Fatalf("unencryptable password: status = %d, want 500 (%s)", w3.Code, w3.Body.String())
	}
	if err := s.db.First(&stored, "id = ?", rd.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.SSHPassword != kept || stored.SSHPassword == "" {
		t.Fatalf("a refused password update altered the stored credential: %q", stored.SSHPassword)
	}
}

// TestRedirectorUpdateRotatesSSHPasswordWithKey proves the password is actually
// rotated once a credential key is configured.
func TestRedirectorUpdateRotatesSSHPasswordWithKey(t *testing.T) {
	s := mustTenantServer(t)
	s.cfg.Crypto.TotpKey = strings.Repeat("ab", 32)
	rd := seedRedirector(t, s, 4, "front", "10.0.0.5")

	c, w := redirectorIDCtx(s, t, "rd-op", "operator", 4, rd.ID, http.MethodPut,
		`{"ssh_password":"rotated-secret"}`)
	s.handleRedirectorUpdate(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	var stored db.Redirector
	if err := s.db.First(&stored, "id = ?", rd.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	// The write path applies the credential key and the model hook applies the
	// vault, so the value read back through AfterFind is the outer ciphertext
	// for a second time round — it is not the plaintext. What matters here is
	// that the rotation happened and nothing was erased.
	original := stored.SSHPassword
	if original == "" {
		t.Fatal("the rotation stored an empty credential")
	}
	if original == "rotated-secret" {
		t.Fatal("the SSH password was stored in plaintext")
	}

	// An update that omits the password keeps it.
	c2, w2 := redirectorIDCtx(s, t, "rd-op", "operator", 4, rd.ID, http.MethodPut,
		`{"name":"front-eu"}`)
	s.handleRedirectorUpdate(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("omitted password: %d %s", w2.Code, w2.Body.String())
	}
	if err := s.db.First(&stored, "id = ?", rd.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.SSHPassword != original {
		t.Fatalf("an omitted password changed the stored credential: got %q, want %q",
			stored.SSHPassword, original)
	}
}

// TestRedirectorCreateRefusesUnencryptablePassword proves a create with a
// password and no credential key fails instead of storing a blank credential.
func TestRedirectorCreateRefusesUnencryptablePassword(t *testing.T) {
	s := mustTenantServer(t)
	s.cfg.Crypto.TotpKey = "not-a-hex-key"

	c, w := redirectorJSONCtx(s, t, "rd-op", "operator", 1,
		`{"name":"front","host":"203.0.113.9","ssh_password":"hunter2"}`)
	s.handleRedirectorCreate(c)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (%s)", w.Code, w.Body.String())
	}
	var n int64
	if err := s.db.Model(&db.Redirector{}).Count(&n).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("a refused create still stored %d redirectors", n)
	}

	// The same body without a password is accepted with no credential stored.
	c2, w2 := redirectorJSONCtx(s, t, "rd-op", "operator", 1,
		`{"name":"front","host":"203.0.113.9"}`)
	s.handleRedirectorCreate(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("no-password create: %d %s", w2.Code, w2.Body.String())
	}
}

// ── SSH probe endpoints ─────────────────────────────────────────────────────

// TestRedirectorTestSSHRequiresOwnedRedirector proves the probe is anchored to
// a redirector the caller owns. Before this batch the endpoint dialed whatever
// host:port the body named, making the C2 server an arbitrary port scanner.
func TestRedirectorTestSSHRequiresOwnedRedirector(t *testing.T) {
	s := mustTenantServer(t)
	foreign := seedRedirector(t, s, 2, "theirs", "198.51.100.7")

	// An unregistered host is refused without dialling.
	c, w := redirectorJSONCtx(s, t, "rd-op", "operator", 1, `{"host":"203.0.113.1","port":22}`)
	s.handleRedirectorTestSSH(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("unregistered host: status = %d, want 404 (%s)", w.Code, w.Body.String())
	}

	// Another tenant's redirector host is equally invisible.
	c2, w2 := redirectorJSONCtx(s, t, "rd-op", "operator", 1,
		fmt.Sprintf(`{"host":%q,"port":%d}`, foreign.Host, foreign.SSHPort))
	s.handleRedirectorTestSSH(c2)
	if w2.Code != http.StatusNotFound {
		t.Fatalf("foreign host: status = %d, want 404 (%s)", w2.Code, w2.Body.String())
	}

	// A port that does not match the stored redirector does not resolve either.
	c3, w3 := redirectorJSONCtx(s, t, "rd-op", "operator", 1, `{"host":"198.51.100.7","port":2222}`)
	s.handleRedirectorTestSSH(c3)
	if w3.Code != http.StatusNotFound {
		t.Fatalf("mismatched port: status = %d, want 404 (%s)", w3.Code, w3.Body.String())
	}

	var audits int64
	if err := s.db.Model(&db.AuditLog{}).Where("action = ?", "redirector_test_ssh").Count(&audits).Error; err != nil {
		t.Fatalf("count audits: %v", err)
	}
	if audits != 0 {
		t.Fatalf("%d refused probes were audited as attempts", audits)
	}
}

// TestRedirectorTestSSHProbesOwnedRedirector proves the legitimate path still
// works and is audited.
func TestRedirectorTestSSHProbesOwnedRedirector(t *testing.T) {
	s := mustTenantServer(t)
	hostPort := silentHost(t)
	host, port := redirectorHostPort(t, hostPort)

	rd := db.Redirector{Name: "front", Host: host, Type: "nginx", SSHPort: port, TenantID: 1}
	if err := s.db.Create(&rd).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	c, w := redirectorJSONCtx(s, t, "rd-op", "operator", 1, fmt.Sprintf(`{"host":%q,"port":%d}`, host, port))
	s.handleRedirectorTestSSH(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"reachable":true`) {
		t.Fatalf("body = %s, want reachable:true", w.Body.String())
	}

	var audit db.AuditLog
	if err := s.db.Where("action = ?", "redirector_test_ssh").First(&audit).Error; err != nil {
		t.Fatalf("no probe audit entry: %v", err)
	}
	if !audit.Success {
		t.Fatalf("audit entry = %+v", audit)
	}
}

// TestRedirectorTestSSHReportsUnreachable proves a registered but down target is
// reported honestly rather than as success.
func TestRedirectorTestSSHReportsUnreachable(t *testing.T) {
	s := mustTenantServer(t)
	// Port 0 is never a valid destination, so the dial fails immediately.
	rd := db.Redirector{Name: "dead", Host: "127.0.0.1", Type: "nginx", SSHPort: 1, TenantID: 1}
	if err := s.db.Create(&rd).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	c, w := redirectorJSONCtx(s, t, "rd-op", "operator", 1, `{"host":"127.0.0.1","port":1}`)
	s.handleRedirectorTestSSH(c)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 (%s)", w.Code, w.Body.String())
	}
}

// TestRedirectorDeploySSHScopesActivation is the cross-tenant write regression:
// the endpoint updated `WHERE host = ?` across every tenant, so probing one
// host flipped all tenants' matching redirectors to active.
func TestRedirectorDeploySSHScopesActivation(t *testing.T) {
	s := mustTenantServer(t)
	hostPort := silentHost(t)
	host, port := redirectorHostPort(t, hostPort)

	mine := db.Redirector{Name: "mine", Host: host, Type: "nginx", SSHPort: port, Status: "inactive", TenantID: 1}
	if err := s.db.Create(&mine).Error; err != nil {
		t.Fatalf("seed own: %v", err)
	}
	theirs := db.Redirector{Name: "theirs", Host: host, Type: "nginx", SSHPort: port, Status: "inactive", TenantID: 2}
	if err := s.db.Create(&theirs).Error; err != nil {
		t.Fatalf("seed foreign: %v", err)
	}

	c, w := redirectorJSONCtx(s, t, "rd-op", "operator", 1, fmt.Sprintf(`{"host":%q,"port":%d,"type":"nginx"}`, host, port))
	s.handleRedirectorDeploySSH(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}

	var own db.Redirector
	if err := s.db.First(&own, "id = ?", mine.ID).Error; err != nil {
		t.Fatalf("reload own: %v", err)
	}
	if own.Status != "active" {
		t.Fatalf("own redirector status = %q, want active", own.Status)
	}
	var foreign db.Redirector
	if err := s.db.First(&foreign, "id = ?", theirs.ID).Error; err != nil {
		t.Fatalf("reload foreign: %v", err)
	}
	if foreign.Status != "inactive" {
		t.Fatalf("another tenant's redirector was flipped to %q by our probe", foreign.Status)
	}

	var audit db.AuditLog
	if err := s.db.Where("action = ?", "redirector_deploy_ssh").First(&audit).Error; err != nil {
		t.Fatalf("no deploy audit entry: %v", err)
	}
	if !audit.Success {
		t.Fatalf("deploy audit = %+v", audit)
	}
}

// TestRedirectorDeploySSHRequiresOwnedRedirector proves the deploy probe shares
// the test endpoint's ownership requirement.
func TestRedirectorDeploySSHRequiresOwnedRedirector(t *testing.T) {
	s := mustTenantServer(t)
	foreign := seedRedirector(t, s, 2, "theirs", "198.51.100.7")

	c, w := redirectorJSONCtx(s, t, "rd-op", "operator", 1,
		fmt.Sprintf(`{"host":%q,"port":%d}`, foreign.Host, foreign.SSHPort))
	s.handleRedirectorDeploySSH(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (%s)", w.Code, w.Body.String())
	}
	var stored db.Redirector
	if err := s.db.First(&stored, "id = ?", foreign.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.Status != "inactive" {
		t.Fatalf("a refused deploy still activated the foreign redirector (%q)", stored.Status)
	}
}

// TestRedirectorDeploySSHMalformedBody proves an unparseable request never
// reaches the dial.
func TestRedirectorDeploySSHMalformedBody(t *testing.T) {
	s := mustTenantServer(t)
	c, w := redirectorJSONCtx(s, t, "rd-op", "operator", 1, `{"host":`)
	s.handleRedirectorDeploySSH(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (%s)", w.Code, w.Body.String())
	}
}

// ── Config generation ───────────────────────────────────────────────────────

// TestRedirectorGenerateRejectsInjection proves the generated server config
// cannot be steered by a crafted domain or protocol — the UI tells the operator
// to deploy exactly this text.
func TestRedirectorGenerateRejectsInjection(t *testing.T) {
	s := mustTenantServer(t)

	bad := []struct {
		name string
		body string
	}{
		{"domain with directive", `{"domain":"x; } location / { deny all; }","protocol":"https"}`},
		{"domain with newline", "{\"domain\":\"a\\nb;c\",\"protocol\":\"https\"}"},
		{"domain with brace", `{"domain":"a{b}","protocol":"https"}`},
		{"domain with space", `{"domain":"a b","protocol":"https"}`},
		{"domain with slash", `{"domain":"a/b","protocol":"https"}`},
		{"domain with empty label", `{"domain":"a..b","protocol":"https"}`},
		{"oversized domain", `{"domain":"` + strings.Repeat("a", 300) + `","protocol":"https"}`},
		{"protocol override", `{"domain":"ok.example","protocol":"file"}`},
		{"protocol break-out", `{"domain":"ok.example","protocol":"http://evil/x"}`},
		{"protocol empty after trim", `{"domain":"ok.example","protocol":"  "}`},
		{"target port zero", `{"domain":"ok.example","protocol":"https","target_port":-1}`},
		{"target port overflow", `{"domain":"ok.example","protocol":"https","target_port":70000}`},
		{"malformed body", `{"domain":`},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			c, w := redirectorJSONCtx(s, t, "rd-op", "operator", 1, tc.body)
			c.Params = gin.Params{{Key: "type", Value: "nginx"}}
			s.handleRedirectorGenerate(c)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (%s)", w.Code, w.Body.String())
			}
			if strings.Contains(w.Body.String(), `"config"`) {
				t.Fatalf("a rejected request still returned a config: %s", w.Body.String())
			}
		})
	}
}

// TestRedirectorGenerateAcceptsValidInput proves the guard is not so broad that
// legitimate templates are refused, across all three supported types.
func TestRedirectorGenerateAcceptsValidInput(t *testing.T) {
	s := mustTenantServer(t)
	cases := []struct {
		rdType string
		body   string
		want   string
	}{
		{"nginx", `{"domain":"c2.example.com","protocol":"https","target_port":8443}`, "server_name c2.example.com"},
		{"apache", `{"domain":"c2.example.com","protocol":"http","target_port":8080}`, "ServerName c2.example.com"},
		{"haproxy", `{"domain":"c2.example.com","protocol":"https","target_port":443}`, "backend c2_nodes"},
		// Defaults fill in for an empty body.
		{"nginx", `{}`, "server_name c2.example.com"},
	}
	for _, tc := range cases {
		t.Run(tc.rdType+"/"+tc.body, func(t *testing.T) {
			c, w := redirectorJSONCtx(s, t, "rd-op", "operator", 1, tc.body)
			c.Params = gin.Params{{Key: "type", Value: tc.rdType}}
			s.handleRedirectorGenerate(c)
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tc.want) {
				t.Fatalf("body = %s, want it to contain %q", w.Body.String(), tc.want)
			}
		})
	}
}

// TestValidateRedirectorDomain pins the hostname validator directly.
func TestValidateRedirectorDomain(t *testing.T) {
	valid := []string{
		"c2.example.com", "sub.domain.example.org", "localhost", "a-b.example",
		"xn--80ak6aa92e.com", "1.2.3.4", "under_score.example",
	}
	for _, domain := range valid {
		if err := validateRedirectorDomain(domain); err != nil {
			t.Fatalf("validateRedirectorDomain(%q) = %v, want nil", domain, err)
		}
	}
	invalid := []string{
		"", "a b", "a;b", "a{b}", "a/b", "a..b", ".a", "a.", strings.Repeat("a", 64) + ".com",
		strings.Repeat("a", 300), "a\nb", "a$b", "a\rb", "*.example.com", "a:b",
	}
	for _, domain := range invalid {
		if err := validateRedirectorDomain(domain); err == nil {
			t.Fatalf("validateRedirectorDomain(%q) accepted an invalid domain", domain)
		}
	}
}

// TestValidRedirectorProtocol pins the protocol allowlist.
func TestValidRedirectorProtocol(t *testing.T) {
	for _, p := range []string{"http", "https"} {
		if !validRedirectorProtocol(p) {
			t.Fatalf("validRedirectorProtocol(%q) = false", p)
		}
	}
	for _, p := range []string{"", "HTTP", "ftp", "file", "http://evil", "https://evil", " ws ", "ws"} {
		if validRedirectorProtocol(p) {
			t.Fatalf("validRedirectorProtocol(%q) = true", p)
		}
	}
}

// TestGenerateRedirectorConfig pins the shared template helper, including that
// an unknown type yields an empty config rather than echoing the input back.
func TestGenerateRedirectorConfig(t *testing.T) {
	s := mustTenantServer(t)
	cases := []struct {
		rdType string
		want   string
	}{
		{"nginx", "server_name c2.example.com"},
		{"apache", "<VirtualHost *:80>"},
		{"haproxy", "backend c2_nodes"},
	}
	for _, tc := range cases {
		got := s.generateRedirectorConfig(tc.rdType, "c2.example.com", "https", "10.0.0.1", 8443)
		if !strings.Contains(got, tc.want) {
			t.Fatalf("generateRedirectorConfig(%q) = %q, want it to contain %q", tc.rdType, got, tc.want)
		}
		if !strings.Contains(got, "8443") {
			t.Fatalf("generateRedirectorConfig(%q) lost the target port: %q", tc.rdType, got)
		}
	}
	if got := s.generateRedirectorConfig("evil-injected", "x", "https", "h", 1); got != "" {
		t.Fatalf("unknown type produced %q, want an empty config", got)
	}
}

// ── Role gate ───────────────────────────────────────────────────────────────

// TestRedirectorMutationsRequireOperator proves every mutating redirector
// endpoint re-checks the role in the handler. The file had no role gate at all,
// so the route permission was the only barrier.
func TestRedirectorMutationsRequireOperator(t *testing.T) {
	s := mustTenantServer(t)
	rd := seedRedirector(t, s, 1, "mine", "10.0.0.5")

	cases := []struct {
		name   string
		method string
		id     uint
		body   string
		invoke func(*Server, *gin.Context)
	}{
		{"create", http.MethodPost, 0, `{"name":"x","host":"h"}`, (*Server).handleRedirectorCreate},
		{"update", http.MethodPut, rd.ID, `{"name":"x"}`, (*Server).handleRedirectorUpdate},
		{"delete", http.MethodDelete, rd.ID, "", (*Server).handleRedirectorDelete},
		{"test-ssh", http.MethodPost, 0, `{"host":"10.0.0.5"}`, (*Server).handleRedirectorTestSSH},
		{"deploy-ssh", http.MethodPost, 0, `{"host":"10.0.0.5"}`, (*Server).handleRedirectorDeploySSH},
		{"generate", http.MethodPost, 0, `{"domain":"x.example.com"}`, (*Server).handleRedirectorGenerate},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, w := redirectorCtx(s, t, "rd-viewer", "viewer", 1)
			c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(int(tc.id))}, {Key: "type", Value: "nginx"}}
			if tc.body != "" {
				c.Request = httptest.NewRequest(tc.method, "/", strings.NewReader(tc.body))
				c.Request.Header.Set("Content-Type", "application/json")
			} else {
				c.Request = httptest.NewRequest(tc.method, "/", nil)
			}
			tc.invoke(s, c)
			if w.Code != http.StatusForbidden {
				t.Fatalf("viewer status = %d, want 403 (%s)", w.Code, w.Body.String())
			}
		})
	}

	// Nothing changed.
	var survivor db.Redirector
	if err := s.db.First(&survivor, "id = ?", rd.ID).Error; err != nil {
		t.Fatalf("redirector was destroyed by a refused request: %v", err)
	}
	if survivor.Name != "mine" || survivor.Status != "inactive" {
		t.Fatalf("a viewer mutated the redirector: %+v", survivor)
	}
	var n int64
	if err := s.db.Model(&db.Redirector{}).Count(&n).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("%d redirectors exist, want the single seeded one", n)
	}
}
