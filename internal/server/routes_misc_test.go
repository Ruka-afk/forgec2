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
