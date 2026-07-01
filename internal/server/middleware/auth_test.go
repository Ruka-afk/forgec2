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
	InitJWTSecret(cfg)

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

	t.Run("remember me session", func(t *testing.T) {
		user := db.User{ID: 2, Username: "operator", Role: "operator", IsActive: true}
		token, err := GenerateToken(user, true, 24)
		if err != nil {
			t.Fatalf("GenerateToken() error = %v", err)
		}
		if token == "" {
			t.Fatal("GenerateToken() returned empty token for remember me")
		}
	})

	t.Run("invalid max age falls back to default", func(t *testing.T) {
		user := db.User{ID: 3, Username: "viewer", Role: "viewer", IsActive: true}
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
	InitJWTSecret(cfg)

	gin.SetMode(gin.TestMode)

	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	testDB.AutoMigrate(&db.User{})

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

	t.Run("valid cookie", func(t *testing.T) {
		user := db.User{ID: 1, Username: "admin", Role: "admin", IsActive: true}
		testDB.Create(&user)

		token, _ := GenerateToken(user, false, 24)

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
		user := db.User{ID: 10, Username: "inactive", Role: "viewer", IsActive: false}
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
		user := db.User{ID: 999, Username: "nonexistent", Role: "viewer", IsActive: true}

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
		user := db.User{ID: 20, Username: "forcelogout", Role: "operator", IsActive: true, ForceLogoutAt: time.Now().Add(1 * time.Hour)}
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
		user := db.User{ID: 30, Username: "testuser", Role: "operator", IsActive: true}
		testDB.Create(&user)

		token, _ := GenerateToken(user, false, 24)

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
	cfg.Server.JWTSecret = "my-custom-secret-key-for-test"

	InitJWTSecret(cfg)

	if string(jwtSecret) != "my-custom-secret-key-for-test" {
		t.Error("jwtSecret was not initialized from config")
	}

	if CookieSecure != cfg.Server.TLSEnabled {
		t.Error("CookieSecure should match TLSEnabled")
	}

	cfg2 := config.DefaultConfig()
	cfg2.Server.JWTSecret = ""
	defer func() {
		if r := recover(); r == nil {
			t.Error("InitJWTSecret should panic on empty secret")
		}
	}()
	InitJWTSecret(cfg2)
}

func TestRequireRole(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("admin bypasses all role checks", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_role", "admin")

		RequireRole("viewer")(c)

		if w.Code == http.StatusForbidden {
			t.Error("admin should bypass role restrictions")
		}
	})

	t.Run("role in allowed list", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_role", "operator")

		RequireRole("operator", "admin")(c)

		if w.Code == http.StatusForbidden {
			t.Error("operator should be allowed when in allowed list")
		}
	})

	t.Run("role not in allowed list", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_role", "viewer")

		RequireRole("operator", "admin")(c)

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

	t.Run("operator has agents.write permission", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_role", "operator")

		RequirePermission("agents.write")(c)

		if w.Code == http.StatusForbidden {
			t.Error("operator should have agents.write permission")
		}
	})

	t.Run("viewer does not have agents.write permission", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_role", "viewer")

		RequirePermission("agents.write")(c)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", w.Code)
		}
	})

	t.Run("viewer has agents.read permission", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_role", "viewer")

		RequirePermission("agents.read")(c)

		if w.Code == http.StatusForbidden {
			t.Error("viewer should have agents.read permission")
		}
	})

	t.Run("guest has limited permissions", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_role", "guest")

		RequirePermission("audit.read")(c)

		if w.Code != http.StatusForbidden {
			t.Errorf("guest should not have audit.read permission, got %d", w.Code)
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

	t.Run("operator has all agent permissions", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_role", "operator")

		RequireAllPermissions("agents.read", "agents.write", "agents.delete")(c)

		if w.Code == http.StatusForbidden {
			t.Error("operator should have all agent permissions")
		}
	})

	t.Run("viewer missing write permission", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_role", "viewer")

		RequireAllPermissions("agents.read", "agents.write")(c)

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
