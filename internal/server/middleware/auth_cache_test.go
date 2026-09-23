package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// authedRequest runs one AuthRequired pass with the given session cookie and
// returns the recorder code. Non-API browser paths redirect (302) on failure.
func authedRequest(t *testing.T, database *gorm.DB, token string) int {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/dashboard", nil)
	c.Request.AddCookie(&http.Cookie{Name: "forgec2_session", Value: token})
	AuthRequired(database)(c)
	return w.Code
}

// TestInvalidateUserCacheEndsStaleSession proves disable + cache eviction
// takes effect on the very next request. Without eviction the 5-minute
// shared cache keeps serving the pre-disable record (documented stale
// window); InvalidateUserCache closes it. Unique IDs avoid collisions with
// the package-level shared cache used by other tests.
func TestInvalidateUserCacheEndsStaleSession(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Server.JWTSecret = "test-jwt-secret-for-cache-invalidation-123"
	if err := InitJWTSecret(cfg, ""); err != nil {
		t.Fatalf("InitJWTSecret() error = %v", err)
	}
	gin.SetMode(gin.TestMode)

	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	database.AutoMigrate(&db.User{}, &db.UserSession{})

	user := db.User{ID: 9001, Username: "cache-victim", Role: "user", IsActive: true}
	if err := database.Create(&user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	token, err := GenerateToken(user, false, 24)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	database.Create(&db.UserSession{UserID: user.ID, TokenHash: TokenHash(token)})

	// First pass populates the shared cache.
	if code := authedRequest(t, database, token); code != http.StatusOK {
		t.Fatalf("first pass got %d, want 200", code)
	}

	// Admin disables the account in the DB. The cached record is now stale.
	if err := database.Model(&db.User{}).Where("id = ?", user.ID).Update("is_active", false).Error; err != nil {
		t.Fatalf("disable: %v", err)
	}
	if code := authedRequest(t, database, token); code != http.StatusOK {
		t.Fatalf("stale cache should still serve (precondition), got %d", code)
	}

	// This is what handleToggleUser does on disable: evict.
	InvalidateUserCache(user.ID)
	if code := authedRequest(t, database, token); code == http.StatusOK {
		t.Fatal("disabled user still authenticated after InvalidateUserCache")
	}
}

// TestInvalidateUserCacheForceLogout proves a ForceLogoutAt bump only takes
// effect immediately when paired with eviction: the cached record carries
// the old (zero) ForceLogoutAt and would otherwise pass the check until TTL.
func TestInvalidateUserCacheForceLogout(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Server.JWTSecret = "test-jwt-secret-for-cache-invalidation-123"
	if err := InitJWTSecret(cfg, ""); err != nil {
		t.Fatalf("InitJWTSecret() error = %v", err)
	}
	gin.SetMode(gin.TestMode)

	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	database.AutoMigrate(&db.User{}, &db.UserSession{})

	user := db.User{ID: 9002, Username: "force-victim", Role: "user", IsActive: true}
	if err := database.Create(&user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	token, err := GenerateToken(user, false, 24)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	database.Create(&db.UserSession{UserID: user.ID, TokenHash: TokenHash(token)})

	if code := authedRequest(t, database, token); code != http.StatusOK {
		t.Fatalf("first pass got %d, want 200", code)
	}

	// Force-logout bump in DB alone does not evict the cached record.
	if err := database.Model(&db.User{}).Where("id = ?", user.ID).Update("force_logout_at", time.Now().Add(time.Hour)).Error; err != nil {
		t.Fatalf("bump: %v", err)
	}
	if code := authedRequest(t, database, token); code != http.StatusOK {
		t.Fatalf("stale cache should still serve (precondition), got %d", code)
	}

	InvalidateUserCache(user.ID)
	if code := authedRequest(t, database, token); code == http.StatusOK {
		t.Fatal("force-logged-out user still authenticated after InvalidateUserCache")
	}
}
