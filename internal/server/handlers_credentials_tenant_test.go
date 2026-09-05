package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func seedTenantVault(t *testing.T, s *Server) (aliceCredID, bobCredID uint) {
	t.Helper()
	users := []db.User{
		{Username: "alice-op", Role: "admin", TenantID: 1, IsActive: true},
		{Username: "bob-op", Role: "admin", TenantID: 2, IsActive: true},
	}
	if err := s.db.Create(&users).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}
	creds := []db.CredentialEntry{
		{AgentID: "agent-a", Domain: "A", Username: "alice-cred", Password: "pw-a", Type: "cleartext", Source: "manual", TenantID: 1},
		{AgentID: "agent-b", Domain: "B", Username: "bob-cred", Password: "pw-b", Type: "cleartext", Source: "manual", TenantID: 2},
	}
	if err := s.db.Create(&creds).Error; err != nil {
		t.Fatalf("seed credentials: %v", err)
	}
	return creds[0].ID, creds[1].ID
}

func tenantCtx(method, target, user string, body string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	c.Request, _ = http.NewRequest(method, target, reader)
	if body != "" {
		c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if user != "" {
		c.Set("user", user)
	}
	c.Set("user_role", "admin")
	return c, w
}

func TestCredentialTenantIsolation_SingleItem(t *testing.T) {
	s := newCredentialsTestServer(t)
	aliceID, bobID := seedTenantVault(t, s)

	// Cross-tenant read → 404.
	c, w := tenantCtx(http.MethodGet, "/x", "alice-op", "")
	c.Params = gin.Params{{Key: "cred_id", Value: fmt.Sprint(bobID)}}
	s.handleGetCredential(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant GET: expected 404, got %d; body=%s", w.Code, w.Body.String())
	}

	// Own-tenant read → 200.
	c, w = tenantCtx(http.MethodGet, "/x", "alice-op", "")
	c.Params = gin.Params{{Key: "cred_id", Value: fmt.Sprint(aliceID)}}
	s.handleGetCredential(c)
	if w.Code != http.StatusOK {
		t.Fatalf("own GET: expected 200, got %d; body=%s", w.Code, w.Body.String())
	}

	// Cross-tenant update → 404, row untouched.
	c, w = tenantCtx(http.MethodPost, "/x", "alice-op", "tags=pwned")
	c.Params = gin.Params{{Key: "cred_id", Value: fmt.Sprint(bobID)}}
	s.handleUpdateCredential(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant UPDATE: expected 404, got %d; body=%s", w.Code, w.Body.String())
	}
	var bob db.CredentialEntry
	if err := s.db.First(&bob, bobID).Error; err != nil {
		t.Fatalf("reload bob cred: %v", err)
	}
	if bob.Tags != "" {
		t.Fatalf("cross-tenant UPDATE mutated row: tags=%q", bob.Tags)
	}

	// Cross-tenant toggle → 404.
	c, w = tenantCtx(http.MethodPost, "/x", "alice-op", "")
	c.Params = gin.Params{{Key: "cred_id", Value: fmt.Sprint(bobID)}}
	s.handleToggleConfirmed(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant TOGGLE: expected 404, got %d; body=%s", w.Code, w.Body.String())
	}

	// Cross-tenant delete → 404, row survives.
	c, w = tenantCtx(http.MethodPost, "/x", "alice-op", "")
	c.Params = gin.Params{{Key: "cred_id", Value: fmt.Sprint(bobID)}}
	s.handleDeleteCredential(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant DELETE: expected 404, got %d; body=%s", w.Code, w.Body.String())
	}
	var count int64
	s.db.Model(&db.CredentialEntry{}).Where("id = ?", bobID).Count(&count)
	if count != 1 {
		t.Fatalf("cross-tenant DELETE removed row: count=%d", count)
	}

	// Legacy unscoped operator (no user in ctx) keeps global visibility.
	c, w = tenantCtx(http.MethodGet, "/x", "", "")
	c.Params = gin.Params{{Key: "cred_id", Value: fmt.Sprint(bobID)}}
	s.handleGetCredential(c)
	if w.Code != http.StatusOK {
		t.Fatalf("legacy GET: expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestCredentialTenantIsolation_Batch(t *testing.T) {
	s := newCredentialsTestServer(t)
	_, bobID := seedTenantVault(t, s)

	// Batch verify must not see (or queue tasks for) another tenant's rows.
	c, w := tenantCtx(http.MethodPost, "/x", "alice-op", "")
	c.Request, _ = http.NewRequest(http.MethodPost, "/x", strings.NewReader(fmt.Sprintf(`{"ids":[%d]}`, bobID)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user", "alice-op")
	c.Set("user_role", "admin")
	s.handleBatchVerifyCredentials(c)
	if w.Code != http.StatusOK {
		t.Fatalf("batch verify: expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), `"queued":1`) {
		t.Fatalf("batch verify queued cross-tenant task: body=%s", w.Body.String())
	}

	// Batch tag must not touch another tenant's rows.
	c, w = tenantCtx(http.MethodPost, "/x", "alice-op", "")
	c.Request, _ = http.NewRequest(http.MethodPost, "/x", strings.NewReader(fmt.Sprintf(`{"ids":[%d],"tags":["x"]}`, bobID)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user", "alice-op")
	c.Set("user_role", "admin")
	s.handleBatchAddTags(c)
	if w.Code != http.StatusOK {
		t.Fatalf("batch tags: expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	var bob db.CredentialEntry
	if err := s.db.First(&bob, bobID).Error; err != nil {
		t.Fatalf("reload bob cred: %v", err)
	}
	if bob.Tags != "" {
		t.Fatalf("batch tags mutated cross-tenant row: tags=%q", bob.Tags)
	}
}
