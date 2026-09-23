package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func postAPIKey(t *testing.T, s *Server, username, role string, userID uint, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(body)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user", username)
	c.Set("user_role", role)
	c.Set("user_id", userID)
	s.handleCreateAPIKey(c)
	return w
}

// TestCreateAPIKeyValidation proves scope/CIDR inputs are validated and
// creators cannot escalate beyond their own grants.
func TestCreateAPIKeyValidation(t *testing.T) {
	s := mustTenantServer(t)
	var admin db.User
	if err := s.db.Where("username = ?", "ak-admin").First(&admin).Error; err != nil {
		admin = db.User{Username: "ak-admin", Role: "admin", TenantID: 1, IsActive: true}
		if err := s.db.Create(&admin).Error; err != nil {
			t.Fatalf("seed admin: %v", err)
		}
	}
	plain := db.User{Username: "ak-user", Role: "user", TenantID: 1, IsActive: true}
	if err := s.db.Create(&plain).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	if w := postAPIKey(t, s, "ak-admin", "admin", admin.ID, `{"name":"bad-scope","scopes":["root.everything"]}`); w.Code != http.StatusBadRequest {
		t.Fatalf("unknown scope got %d, want 400", w.Code)
	}

	if w := postAPIKey(t, s, "ak-admin", "admin", admin.ID, `{"name":"bad-cidr","allowed_cidrs":"999.1.1.1"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("bad CIDR got %d, want 400", w.Code)
	}

	// Non-admin creator granting users.write (outside the user role) -> 403.
	if w := postAPIKey(t, s, "ak-user", "user", plain.ID, `{"name":"escalate","scopes":["users.write"]}`); w.Code != http.StatusForbidden {
		t.Fatalf("escalation got %d, want 403", w.Code)
	}

	// Valid scoped create: stored normalized (sorted csv) + echoed back.
	w := postAPIKey(t, s, "ak-admin", "admin", admin.ID, `{"name":"scoped-ok","scopes":["tasks.read","agents.read","bulk_export"],"allowed_cidrs":"10.0.0.0/8, 203.0.113.7"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("valid create got %d body=%s, want 200", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"scopes":"agents.read,bulk_export,tasks.read"`) {
		t.Fatalf("scopes not normalized in response: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "203.0.113.7/32") {
		t.Fatalf("CIDR not normalized in response: %s", w.Body.String())
	}
	var stored db.ApiKey
	if err := s.db.Where("name = ?", "scoped-ok").First(&stored).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.Scopes != "agents.read,bulk_export,tasks.read" || stored.AllowedCIDRs != "10.0.0.0/8,203.0.113.7/32" {
		t.Fatalf("stored scopes=%q cidrs=%q", stored.Scopes, stored.AllowedCIDRs)
	}
}

// TestExportStepUpAPIKeyScopes proves API-key callers need bulk_export for
// bulk exports, while legacy full keys stay exempt.
func TestExportStepUpAPIKeyScopes(t *testing.T) {
	s := mustTenantServer(t)

	keyed := func(scopes map[string]bool) (*gin.Context, *httptest.ResponseRecorder) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)
		c.Set("user", "ak-exporter")
		c.Set("user_role", "admin")
		c.Set("auth_via_api_key", true)
		if scopes != nil {
			c.Set("api_key_scopes", scopes)
		}
		return c, w
	}

	c, w := keyed(nil) // legacy full key
	s.handleExportCredentials(c)
	if w.Code != http.StatusOK {
		t.Fatalf("legacy key got %d, want 200", w.Code)
	}

	c, w = keyed(map[string]bool{"credentials.read": true})
	s.handleExportCredentials(c)
	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "bulk_export") {
		t.Fatalf("scoped key without bulk_export got %d body=%s, want 403 bulk_export", w.Code, w.Body.String())
	}

	c, w = keyed(map[string]bool{"credentials.read": true, "bulk_export": true})
	s.handleExportCredentials(c)
	if w.Code != http.StatusOK {
		t.Fatalf("bulk_export key got %d body=%s, want 200", w.Code, w.Body.String())
	}
}

// TestListAPIKeysFlagsLegacy proves the list endpoint flags hygiene risks:
// empty scopes => legacy_full_access, empty CIDRs => unrestricted_source,
// so operators and automation can spot over-privileged keys.
func TestListAPIKeysFlagsLegacy(t *testing.T) {
	s := mustTenantServer(t)
	owner := db.User{Username: "ak-flag-owner", Role: "admin", TenantID: 1, IsActive: true}
	if err := s.db.Create(&owner).Error; err != nil {
		t.Fatalf("seed owner: %v", err)
	}
	seed := []db.ApiKey{
		{UserID: owner.ID, Name: "legacy", KeyHash: "h1", Prefix: "fc2_legacy", Scopes: "", AllowedCIDRs: "", Active: true},
		{UserID: owner.ID, Name: "scoped", KeyHash: "h2", Prefix: "fc2_scoped", Scopes: "agents.read", AllowedCIDRs: "10.0.0.0/8", Active: true},
	}
	for i := range seed {
		if err := s.db.Create(&seed[i]).Error; err != nil {
			t.Fatalf("seed key: %v", err)
		}
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/api-keys", nil)
	c.Set("user_id", owner.ID)
	s.handleListAPIKeys(c)
	if w.Code != http.StatusOK {
		t.Fatalf("list got %d body=%s, want 200", w.Code, w.Body.String())
	}
	var resp struct {
		Success bool `json:"success"`
		Data    []struct {
			Name               string `json:"name"`
			LegacyFullAccess   bool   `json:"legacy_full_access"`
			UnrestrictedSource bool   `json:"unrestricted_source"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	byName := map[string]struct {
		legacy bool
		open   bool
	}{}
	for _, k := range resp.Data {
		byName[k.Name] = struct {
			legacy bool
			open   bool
		}{k.LegacyFullAccess, k.UnrestrictedSource}
	}
	leg, ok := byName["legacy"]
	if !ok || !leg.legacy || !leg.open {
		t.Fatalf("legacy key flags = %+v, want both true (resp=%s)", leg, w.Body.String())
	}
	sc, ok := byName["scoped"]
	if !ok || sc.legacy || sc.open {
		t.Fatalf("scoped key flags = %+v, want both false (resp=%s)", sc, w.Body.String())
	}
}
