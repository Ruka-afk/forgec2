package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
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

func TestCheckPassword(t *testing.T) {
	t.Run("empty hash", func(t *testing.T) {
		if CheckPassword("", "password") {
			t.Error("CheckPassword() should return false for empty hash")
		}
	})

	t.Run("default admin hash", func(t *testing.T) {
		defaultAdminHash := "$2a$10$E40B4XhFn4P2qRk60otaFOs61NHuKnB34OS6NfKKGHakYO8CsvoU2"
		if !CheckPassword(defaultAdminHash, "admin") {
			t.Error("CheckPassword() should return true for default admin hash")
		}
		t.Logf("Default admin hash verified: %s", defaultAdminHash)
	})

	t.Run("correct password", func(t *testing.T) {
		hash, _ := HashPassword("correct-password")
		if !CheckPassword(hash, "correct-password") {
			t.Error("CheckPassword() should return true for correct password")
		}
	})

	t.Run("wrong password", func(t *testing.T) {
		hash, _ := HashPassword("correct-password")
		if CheckPassword(hash, "wrong-password") {
			t.Error("CheckPassword() should return false for wrong password")
		}
	})

	t.Run("empty password", func(t *testing.T) {
		hash, _ := HashPassword("correct-password")
		if CheckPassword(hash, "") {
			t.Error("CheckPassword() should return false for empty password")
		}
	})
}

func TestGenerateToken(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Server.JWTSecret = "test-jwt-secret-for-testing-12345"
	if err := InitJWTSecret(cfg, ""); err != nil {
		t.Fatalf("InitJWTSecret() error = %v", err)
	}

	t.Run("normal session", func(t *testing.T) {
		user := db.User{ID: 1, Username: "admin", Role: "admin", IsActive: true, LastLogin: time.Now()}
		token, err := GenerateToken(user, false, 24)
		if err != nil {
			t.Fatalf("GenerateToken() error = %v", err)
		}
		if token == "" {
			t.Fatal("GenerateToken() returned empty token")
		}
	})

	t.Run("user session", func(t *testing.T) {
		user := db.User{ID: 2, Username: "testuser", Role: "user", IsActive: true}
		token, err := GenerateToken(user, true, 24)
		if err != nil {
			t.Fatalf("GenerateToken() error = %v", err)
		}
		if token == "" {
			t.Fatal("GenerateToken() returned empty token for user")
		}
	})

	t.Run("invalid max age falls back to default", func(t *testing.T) {
		user := db.User{ID: 3, Username: "user2", Role: "user", IsActive: true}
		token, err := GenerateToken(user, false, 0)
		if err != nil {
			t.Fatalf("GenerateToken() error = %v", err)
		}
		if token == "" {
			t.Fatal("GenerateToken() returned empty token with invalid max age")
		}
	})
}

func TestAuthRequired(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Server.JWTSecret = "test-jwt-secret-for-auth-test"
	if err := InitJWTSecret(cfg, ""); err != nil {
		t.Fatalf("InitJWTSecret() error = %v", err)
	}

	gin.SetMode(gin.TestMode)

	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	testDB.AutoMigrate(&db.User{}, &db.UserSession{})

	t.Run("no cookie", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("GET", "/dashboard", nil)

		AuthRequired(testDB)(c)

		if w.Code != http.StatusFound {
			t.Errorf("expected redirect, got %d", w.Code)
		}
	})

	t.Run("invalid cookie", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("GET", "/dashboard", nil)
		c.Request.AddCookie(&http.Cookie{Name: "forgec2_session", Value: "invalid-token"})

		AuthRequired(testDB)(c)

		if w.Code != http.StatusFound {
			t.Errorf("expected redirect, got %d", w.Code)
		}
	})

	t.Run("api path returns JSON 401 without Accept header", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("GET", "/api/modules", nil)

		AuthRequired(testDB)(c)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for /api/* unauthenticated, got %d body=%s", w.Code, w.Body.String())
		}
		ct := w.Header().Get("Content-Type")
		if !strings.Contains(ct, "application/json") {
			t.Fatalf("expected JSON content-type, got %q body=%s", ct, w.Body.String())
		}
	})

	t.Run("accept json returns 401 not redirect", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("GET", "/agents", nil)
		c.Request.Header.Set("Accept", "application/json")

		AuthRequired(testDB)(c)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("valid cookie", func(t *testing.T) {
		user := db.User{ID: 1, Username: "admin", Role: "admin", IsActive: true}
		testDB.Create(&user)

		token, _ := GenerateToken(user, false, 24)
		testDB.Create(&db.UserSession{UserID: user.ID, TokenHash: TokenHash(token)})

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("GET", "/dashboard", nil)
		c.Request.AddCookie(&http.Cookie{Name: "forgec2_session", Value: token})

		AuthRequired(testDB)(c)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})

	t.Run("inactive user", func(t *testing.T) {
		user := db.User{ID: 10, Username: "inactive", Role: "user", IsActive: false}
		testDB.Create(&user)

		token, _ := GenerateToken(user, false, 24)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("GET", "/dashboard", nil)
		c.Request.AddCookie(&http.Cookie{Name: "forgec2_session", Value: token})

		AuthRequired(testDB)(c)

		if w.Code != http.StatusFound {
			t.Errorf("expected redirect for inactive user, got %d", w.Code)
		}
	})

	t.Run("user not found in db", func(t *testing.T) {
		user := db.User{ID: 999, Username: "nonexistent", Role: "user", IsActive: true}

		token, _ := GenerateToken(user, false, 24)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("GET", "/dashboard", nil)
		c.Request.AddCookie(&http.Cookie{Name: "forgec2_session", Value: token})

		AuthRequired(testDB)(c)

		if w.Code != http.StatusFound {
			t.Errorf("expected redirect for non-existent user, got %d", w.Code)
		}
	})

	t.Run("force logout invalidates session", func(t *testing.T) {
		user := db.User{ID: 20, Username: "forcelogout", Role: "user", IsActive: true, ForceLogoutAt: time.Now().Add(1 * time.Hour)}
		testDB.Create(&user)

		token, _ := GenerateToken(user, false, 24)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("GET", "/dashboard", nil)
		c.Request.AddCookie(&http.Cookie{Name: "forgec2_session", Value: token})

		AuthRequired(testDB)(c)

		if w.Code != http.StatusFound {
			t.Errorf("expected redirect for force logout, got %d", w.Code)
		}
	})

	t.Run("context contains user info", func(t *testing.T) {
		user := db.User{ID: 30, Username: "testuser", Role: "user", IsActive: true}
		testDB.Create(&user)

		token, _ := GenerateToken(user, false, 24)
		testDB.Create(&db.UserSession{UserID: user.ID, TokenHash: TokenHash(token)})

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("GET", "/dashboard", nil)
		c.Request.AddCookie(&http.Cookie{Name: "forgec2_session", Value: token})

		AuthRequired(testDB)(c)

		if userID, exists := c.Get("user_id"); !exists || userID != user.ID {
			t.Errorf("context user_id not set correctly, got %v", userID)
		}
		if username, exists := c.Get("user"); !exists || username != user.Username {
			t.Errorf("context user not set correctly, got %v", username)
		}
		if role, exists := c.Get("user_role"); !exists || role != user.Role {
			t.Errorf("context user_role not set correctly, got %v", role)
		}
	})
}

func TestInitJWTSecret(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Server.JWTSecret = "my-custom-secret-key-for-test-32chars"

	if err := InitJWTSecret(cfg, ""); err != nil {
		t.Fatalf("InitJWTSecret() error = %v", err)
	}

	if string(jwtSecret.Load().([]byte)) != "my-custom-secret-key-for-test-32chars" {
		t.Error("jwtSecret was not initialized from config")
	}

	if CookieSecure != cfg.Server.TLSEnabled {
		t.Error("CookieSecure should match TLSEnabled")
	}

	cfg2 := config.DefaultConfig()
	cfg2.Server.JWTSecret = ""
	// Empty secret auto-generates a new one; should not error
	if err := InitJWTSecret(cfg2, ""); err != nil {
		t.Fatalf("InitJWTSecret() with empty secret should auto-generate, got error = %v", err)
	}
	if len(cfg2.Server.JWTSecret) < 32 {
		t.Error("auto-generated JWT secret should be at least 32 chars")
	}
}

func TestRequireRole(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("admin bypasses all role checks", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_role", "admin")

		RequireRole("user")(c)

		if w.Code == http.StatusForbidden {
			t.Error("admin should bypass role restrictions")
		}
	})

	t.Run("role in allowed list", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_role", "user")

		RequireRole("user", "admin")(c)

		if w.Code == http.StatusForbidden {
			t.Error("user should be allowed when in allowed list")
		}
	})

	t.Run("role not in allowed list", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_role", "user")

		RequireRole("admin")(c)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", w.Code)
		}
	})

	t.Run("no role in context", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		RequireRole("admin")(c)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", w.Code)
		}
	})

	t.Run("invalid role type", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_role", 123)

		RequireRole("admin")(c)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", w.Code)
		}
	})
}

func TestRequirePermission(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("admin bypasses all permission checks", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_role", "admin")

		RequirePermission("agents.read")(c)

		if w.Code == http.StatusForbidden {
			t.Error("admin should bypass permission restrictions")
		}
	})

	t.Run("user has agents.write permission", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_role", "user")

		RequirePermission("agents.write")(c)

		if w.Code == http.StatusForbidden {
			t.Error("user should have agents.write permission")
		}
	})

	t.Run("user has any matching agent permission (OR logic)", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_role", "user")

		RequirePermission("agents.read", "agents.write", "agents.delete")(c)

		if w.Code == http.StatusForbidden {
			t.Error("user should have at least one agent permission (agents.read or agents.write)")
		}
	})

	t.Run("no role in context", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		RequirePermission("agents.read")(c)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", w.Code)
		}
	})
}

func TestRequireAllPermissions(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("admin bypasses all permission checks", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_role", "admin")

		RequireAllPermissions("agents.read", "agents.write", "agents.delete")(c)

		if w.Code == http.StatusForbidden {
			t.Error("admin should bypass all permission restrictions")
		}
	})

	t.Run("user has agents.read and agents.write", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_role", "user")

		RequireAllPermissions("agents.read", "agents.write")(c)

		if w.Code == http.StatusForbidden {
			t.Error("user should have agents.read and agents.write permissions")
		}
	})

	t.Run("user lacks agents.delete", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_role", "user")

		RequireAllPermissions("agents.read", "agents.write", "agents.delete")(c)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 for missing agents.delete, got %d", w.Code)
		}
	})

	t.Run("user missing permission", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_role", "user")

		RequireAllPermissions("users.write", "users.delete")(c)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", w.Code)
		}
	})

	t.Run("no role in context", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		RequireAllPermissions("agents.read")(c)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", w.Code)
		}
	})
}
