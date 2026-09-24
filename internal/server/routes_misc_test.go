package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gin-gonic/gin"
)

// TestScreenshotsRouteOperatorPlaneGuard proves the /screenshots blob path
// re-applies the operator CIDR allowlist — it is registered outside the auth
// group, so AuthRequired alone used to leave it open to non-operator networks.
func TestScreenshotsRouteOperatorPlaneGuard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := testutil.SetupTestDB(t)
	cfg := config.DefaultConfig()
	cfg.Server.JWTSecret = "test-secret-for-shot-plane-32chars!"
	cfg.Server.OperatorAllowedCIDRs = []string{"10.0.0.0/8"}
	setServerTestKeys(cfg)
	crypto.InitLootEncryption(cfg.Crypto.LootKey)

	s := New(cfg, database)
	s.SetupRoutes()

	req := func(remote string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/screenshots/agent-x/shot.png", nil)
		r.RemoteAddr = remote + ":12345"
		s.router.ServeHTTP(w, r)
		return w
	}

	if w := req("192.168.1.9"); w.Code != http.StatusForbidden {
		t.Fatalf("outsider screenshot: got %d, want 403; body=%s", w.Code, w.Body.String())
	}

	// In-CIDR traffic passes the plane guard (auth may still 401).
	if w := req("10.9.9.9"); w.Code == http.StatusForbidden {
		t.Fatalf("operator CIDR member denied by plane guard: body=%s", w.Body.String())
	}
}

// TestRESTAPIOperatorPlaneGuard proves the authenticated /api/v1 surface is
// covered by operator_allowed_cidrs (it used to bypass the guard entirely),
// while /api/v1/health stays reachable for out-of-network probes.
func TestRESTAPIOperatorPlaneGuard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := testutil.SetupTestDB(t)
	cfg := config.DefaultConfig()
	cfg.Server.JWTSecret = "test-secret-rest-plane-32chars!"
	cfg.Server.OperatorAllowedCIDRs = []string{"10.0.0.0/8"}
	setServerTestKeys(cfg)
	crypto.InitLootEncryption(cfg.Crypto.LootKey)

	s := New(cfg, database)
	s.SetupRoutes()

	do := func(path, remote string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.RemoteAddr = remote + ":12345"
		s.router.ServeHTTP(w, r)
		return w
	}

	if w := do("/api/v1/agents", "192.168.1.9"); w.Code != http.StatusForbidden {
		t.Fatalf("outsider /api/v1/agents: got %d, want 403; body=%s", w.Code, w.Body.String())
	}
	if w := do("/api/v1/audit", "192.168.1.9"); w.Code != http.StatusForbidden {
		t.Fatalf("outsider /api/v1/audit: got %d, want 403; body=%s", w.Code, w.Body.String())
	}
	// Health probe: guarded off, so it answers auth (401) instead of 403.
	if w := do("/api/v1/health", "192.168.1.9"); w.Code == http.StatusForbidden {
		t.Fatalf("exempt /api/v1/health blocked by operator plane: body=%s", w.Body.String())
	}
	// In-CIDR traffic passes the guard and reaches auth.
	if w := do("/api/v1/agents", "10.9.9.9"); w.Code == http.StatusForbidden {
		t.Fatalf("operator CIDR member denied by plane guard: body=%s", w.Body.String())
	}
}
