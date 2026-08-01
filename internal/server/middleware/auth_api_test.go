package middleware

import (
	"crypto/sha256"
	"fmt"
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

func setupAPITestDB(t *testing.T) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.AutoMigrate(&db.User{}, &db.ApiKey{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.Create(&db.User{
		Username: "apiuser",
		Role:     "user",
		IsActive: true,
	})
	return database
}

func TestAuthRequired_APIKeyValid(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := setupAPITestDB(t)
	cfg := config.DefaultConfig()
	cfg.Server.JWTSecret = "test-secret-for-apikey-auth-32char!"
	if err := InitJWTSecret(cfg, ""); err != nil {
		t.Fatalf("InitJWTSecret: %v", err)
	}

	keyHash := sha256Of("valid-api-key-12345")
	database.Create(&db.ApiKey{
		KeyHash: keyHash,
		Prefix:  "valid",
		UserID:  1,
		Active:  true,
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/test", nil)
	c.Request.Header.Set("X-API-Key", "valid-api-key-12345")

	AuthRequired(database)(c)

	if w.Code == http.StatusUnauthorized || w.Code == http.StatusForbidden {
		t.Errorf("valid API key should pass auth, got %d; body=%s", w.Code, w.Body.String())
	}
	if !c.IsAborted() {
		userID, _ := c.Get("user_id")
		if userID == nil {
			t.Error("user_id should be set in context")
		}
		role, _ := c.Get("user_role")
		if role != "user" {
			t.Errorf("expected role=user, got %v", role)
		}
	}
}

func TestAuthRequired_APIKeyInvalid(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := setupAPITestDB(t)
	cfg := config.DefaultConfig()
	cfg.Server.JWTSecret = "test-secret-for-apikey-auth-32char!"
	if err := InitJWTSecret(cfg, ""); err != nil {
		t.Fatalf("InitJWTSecret: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/test", nil)
	c.Request.Header.Set("X-API-Key", "non-existent-key")

	AuthRequired(database)(c)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid API key, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestAuthRequired_APIKeyExpired(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := setupAPITestDB(t)
	cfg := config.DefaultConfig()
	cfg.Server.JWTSecret = "test-secret-for-apikey-auth-32char!"
	if err := InitJWTSecret(cfg, ""); err != nil {
		t.Fatalf("InitJWTSecret: %v", err)
	}

	keyHash := sha256Of("expired-api-key")
	database.Create(&db.ApiKey{
		KeyHash:   keyHash,
		Prefix:    "exp",
		UserID:    1,
		Active:    true,
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/test", nil)
	c.Request.Header.Set("X-API-Key", "expired-api-key")

	AuthRequired(database)(c)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for expired API key, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestAuthRequired_APIKeyInactiveUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := setupAPITestDB(t)
	cfg := config.DefaultConfig()
	cfg.Server.JWTSecret = "test-secret-for-apikey-auth-32char!"
	if err := InitJWTSecret(cfg, ""); err != nil {
		t.Fatalf("InitJWTSecret: %v", err)
	}

	// Create inactive user
	database.Create(&db.User{
		Username: "inactive-user",
		Role:     "user",
		IsActive: false,
	})
	keyHash := sha256Of("inactive-user-key")
	// Find the inactive user's ID
	var inactiveUser db.User
	database.Where("username = ?", "inactive-user").First(&inactiveUser)

	database.Create(&db.ApiKey{
		KeyHash: keyHash,
		Prefix:  "inu",
		UserID:  inactiveUser.ID,
		Active:  true,
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/test", nil)
	c.Request.Header.Set("X-API-Key", "inactive-user-key")

	AuthRequired(database)(c)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for inactive user's API key, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestAuthRequired_SessionCookieMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := setupAPITestDB(t)
	cfg := config.DefaultConfig()
	cfg.Server.JWTSecret = "test-secret-for-apikey-auth-32char!"
	if err := InitJWTSecret(cfg, ""); err != nil {
		t.Fatalf("InitJWTSecret: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/test", nil)

	AuthRequired(database)(c)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 when no session cookie, got %d; body=%s", w.Code, w.Body.String())
	}
}

func sha256Of(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h)
}
