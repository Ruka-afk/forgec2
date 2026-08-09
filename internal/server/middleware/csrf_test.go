package middleware

import (
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/gin-gonic/gin"
)

const csrfTestKeyHex = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"

func sessionCookie(sessionToken string) *http.Cookie {
	return &http.Cookie{Name: "forgec2_session", Value: sessionToken}
}

func validCSRFToken(sessionToken, keyHex string) string {
	b, err := hex.DecodeString(keyHex)
	if err != nil {
		panic(err)
	}
	return deriveCSRFToken(sessionToken, b)
}

func initSecrets(t *testing.T, jwtSecret string) {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.Server.JWTSecret = jwtSecret
	cfg.Crypto.CsrfKey = csrfTestKeyHex
	if err := InitJWTSecret(cfg, ""); err != nil {
		t.Fatalf("InitJWTSecret: %v", err)
	}
	if err := InitCSRFSecret(cfg); err != nil {
		t.Fatalf("InitCSRFSecret: %v", err)
	}
}

func TestInitCSRFSecret(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Crypto.CsrfKey = ""
	if err := InitCSRFSecret(cfg); err == nil {
		t.Fatal("InitCSRFSecret with empty key should fail")
	}
	cfg.Crypto.CsrfKey = "tooshort"
	if err := InitCSRFSecret(cfg); err == nil {
		t.Fatal("InitCSRFSecret with short key should fail")
	}
	cfg.Crypto.CsrfKey = csrfTestKeyHex
	if err := InitCSRFSecret(cfg); err != nil {
		t.Fatalf("InitCSRFSecret with valid key: %v", err)
	}
}

func TestCSRFProtect_GET_SetsCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initSecrets(t, "test-secret-key-32chars-long!!!!")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/test", nil)
	c.Request.AddCookie(sessionCookie("my-session-token"))

	CSRFProtect()(c)

	cookies := w.Result().Cookies()
	found := false
	for _, ck := range cookies {
		if ck.Name == csrfCookieName {
			found = true
			if ck.Value == "" {
				t.Error("CSRF cookie value is empty")
			}
			if ck.HttpOnly {
				t.Error("CSRF cookie must NOT be HttpOnly (frontend needs to read it)")
			}
		}
	}
	if !found {
		t.Error("GET request should set CSRF cookie")
	}
}

func TestCSRFProtect_POST_RejectsMissingToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initSecrets(t, "test-secret-key-32chars-long!!!!")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/test", nil)

	CSRFProtect()(c)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestCSRFProtect_POST_RejectsMismatchedToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initSecrets(t, "test-secret-key-32chars-long!!!!")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/test", nil)
	c.Request.Header.Set(csrfHeaderName, "wrong-token")
	c.Request.AddCookie(sessionCookie("my-session-token"))

	CSRFProtect()(c)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestCSRFProtect_POST_AcceptsValidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const sessionToken = "my-session-token"
	initSecrets(t, "test-secret-key-32chars-long!!!!")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/test", nil)
	c.Request.Header.Set(csrfHeaderName, validCSRFToken(sessionToken, csrfTestKeyHex))
	c.Request.AddCookie(sessionCookie(sessionToken))

	CSRFProtect()(c)

	if w.Code == http.StatusForbidden {
		t.Error("valid CSRF token should not be rejected")
	}
}

func TestCSRFProtect_TokenBoundToDedicatedKey(t *testing.T) {
	// A token derived with the JWT secret (the legacy behavior) must be
	// rejected now that CSRF uses an independent key.
	gin.SetMode(gin.TestMode)
	const sessionToken = "my-session-token"
	initSecrets(t, "test-secret-key-32chars-long!!!!")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/test", nil)
	legacyToken := deriveCSRFToken(sessionToken, []byte("test-secret-key-32chars-long!!!!"))
	c.Request.Header.Set(csrfHeaderName, legacyToken)
	c.Request.AddCookie(sessionCookie(sessionToken))

	CSRFProtect()(c)

	if w.Code != http.StatusForbidden {
		t.Error("token signed with the JWT secret (not crypto.csrf_key) must be rejected")
	}
}

func TestCSRFProtect_OPTIONS_BypassesCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodOptions, "/api/test", nil)

	CSRFProtect()(c)

	if w.Code == http.StatusForbidden {
		t.Error("OPTIONS should not require CSRF token")
	}
}

func TestCSRFProtect_PreservesExistingCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initSecrets(t, "test-secret-key-32chars-long!!!!")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/test", nil)
	c.Request.AddCookie(sessionCookie("my-session-token"))
	c.Request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "existing-token"})

	CSRFProtect()(c)

	cookies := w.Result().Cookies()
	found := false
	for _, ck := range cookies {
		if ck.Name == csrfCookieName {
			found = true
			if ck.Value == "" {
				t.Error("CSRF cookie value should not be empty")
			}
		}
	}
	if !found {
		t.Error("GET request should set CSRF cookie")
	}
}