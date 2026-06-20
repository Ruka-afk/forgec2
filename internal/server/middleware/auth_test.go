package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func TestHashPassword(t *testing.T) {
	hash, err := HashPassword("test-password-123")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if hash == "" {
		t.Fatal("HashPassword() returned empty hash")
	}

	if !CheckPassword(hash, "test-password-123") {
		t.Error("CheckPassword() should return true for correct password")
	}

	if CheckPassword(hash, "wrong-password") {
		t.Error("CheckPassword() should return false for wrong password")
	}
}

func TestGenerateToken(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Server.JWTSecret = "test-jwt-secret-for-testing-12345"
	InitJWTSecret(cfg)

	user := db.User{ID: 1, Username: "admin", Role: "admin", IsActive: true, LastLogin: time.Now()}
	token, err := GenerateToken(user, false, 24)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}
	if token == "" {
		t.Fatal("GenerateToken() returned empty token")
	}
}

func TestAuthRequired(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Server.JWTSecret = "test-jwt-secret-for-auth-test"
	InitJWTSecret(cfg)

	gin.SetMode(gin.TestMode)

	t.Run("no cookie", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("GET", "/dashboard", nil)

		AuthRequired(cfg)(c)

		if w.Code != http.StatusFound {
			t.Errorf("expected redirect, got %d", w.Code)
		}
	})

	t.Run("invalid cookie", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("GET", "/dashboard", nil)
		c.Request.AddCookie(&http.Cookie{Name: "forgec2_session", Value: "invalid-token"})

		AuthRequired(cfg)(c)

		if w.Code != http.StatusFound {
			t.Errorf("expected redirect, got %d", w.Code)
		}
	})

	t.Run("valid cookie", func(t *testing.T) {
		token, _ := GenerateToken(user, false, 24)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("GET", "/dashboard", nil)
		c.Request.AddCookie(&http.Cookie{Name: "forgec2_session", Value: token})
		c.Request.RemoteAddr = "192.168.1.1:12345"

		AuthRequired(cfg)(c)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})
}

func TestInitJWTSecret(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Server.JWTSecret = "my-custom-secret-key-for-test"

	InitJWTSecret(cfg)

	if string(jwtSecret) != "my-custom-secret-key-for-test" {
		t.Error("jwtSecret was not initialized from config")
	}

	// Test empty secret fallback
	cfg2 := config.DefaultConfig()
	cfg2.Server.JWTSecret = ""
	InitJWTSecret(cfg2)

	if len(jwtSecret) == 0 {
		t.Error("jwtSecret should have fallback value")
	}
}
