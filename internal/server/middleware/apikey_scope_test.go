package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// authWithKey runs AuthRequired with the given key header + remote addr and
// returns the (possibly aborted) context for downstream middleware tests.
func authWithKey(t *testing.T, database *gorm.DB, key, remoteAddr string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/test", nil)
	c.Request.Header.Set("X-API-Key", key)
	c.Request.RemoteAddr = remoteAddr
	AuthRequired(database)(c)
	return c, w
}

func TestScopedKeyEnforcement(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := setupAPITestDB(t)
	cfg := config.DefaultConfig()
	cfg.Server.JWTSecret = "test-secret-for-apikey-auth-32char!"
	if err := InitJWTSecret(cfg, ""); err != nil {
		t.Fatalf("InitJWTSecret: %v", err)
	}
	database.Create(&db.ApiKey{
		KeyHash:      sha256Of("scoped-key-1"),
		Prefix:       "scoped",
		UserID:       1,
		Active:       true,
		Scopes:       "agents.read,tasks.read",
		AllowedCIDRs: "10.0.0.0/8",
	})
	database.Create(&db.ApiKey{
		KeyHash: sha256Of("full-key-1"),
		Prefix:  "full",
		UserID:  1,
		Active:  true,
	})

	// In-scope permission passes.
	c, w := authWithKey(t, database, "scoped-key-1", "10.1.2.3:4444")
	if w.Code == http.StatusUnauthorized || w.Code == http.StatusForbidden {
		t.Fatalf("scoped key auth failed: %d %s", w.Code, w.Body.String())
	}
	RequirePermission(db.PermAgentsRead)(c)
	if c.IsAborted() {
		t.Fatalf("in-scope perm denied: %d %s", w.Code, w.Body.String())
	}

	// Out-of-scope permission denied even though the owner could do it.
	c2, _ := authWithKey(t, database, "scoped-key-1", "10.1.2.3:4444")
	RequirePermission(db.PermAgentsWrite)(c2)
	if !c2.IsAborted() {
		t.Fatalf("out-of-scope perm allowed")
	}

	// Scoped keys never satisfy role gates.
	c3, _ := authWithKey(t, database, "scoped-key-1", "10.1.2.3:4444")
	RequireRole(db.RoleAdmin)(c3)
	if !c3.IsAborted() {
		t.Fatalf("scoped key passed role gate")
	}

	// CIDR outsider rejected at auth.
	_, w4 := authWithKey(t, database, "scoped-key-1", "192.168.1.9:4444")
	if w4.Code != http.StatusUnauthorized {
		t.Fatalf("CIDR outsider got %d, want 401", w4.Code)
	}

	// Legacy full key: scopes read back unconstrained (stored typed-nil
	// map), so role logic decides exactly as before.
	c5, _ := authWithKey(t, database, "full-key-1", "192.168.1.9:4444")
	if v, ok := c5.Get("api_key_scopes"); ok {
		if mm, isMap := v.(map[string]bool); !isMap || mm != nil {
			t.Fatalf("full key scopes misread: %#v", v)
		}
	}
}
