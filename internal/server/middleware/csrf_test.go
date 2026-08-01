package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/gin-gonic/gin"
)

func sessionCookie(sessionToken string) *http.Cookie {
	return &http.Cookie{Name: "forgec2_session", Value: sessionToken}
}

func validCSRFToken(sessionToken, jwtSecret string) string {
	return deriveCSRFToken(sessionToken, []byte(jwtSecret))
}

func initJWTSecret(t *testing.T, secret string) {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.Server.JWTSecret = secret
	if err := InitJWTSecret(cfg, ""); err != nil {
		t.Fatalf("InitJWTSecret: %v", err)
	}
}

func TestCSRFProtect_GET_SetsCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	initJWTSecret(t, "test-secret-key-32chars-long!!!!")

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
	initJWTSecret(t, "test-secret-key-32chars-long!!!!")

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
	initJWTSecret(t, "test-secret-key-32chars-long!!!!")

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
	const jwtSecret = "test-secret-key-32chars-long!!!!"
	const sessionToken = "my-session-token"
	initJWTSecret(t, jwtSecret)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/test", nil)
	c.Request.Header.Set(csrfHeaderName, validCSRFToken(sessionToken, jwtSecret))
	c.Request.AddCookie(sessionCookie(sessionToken))

	CSRFProtect()(c)

	if w.Code == http.StatusForbidden {
		t.Error("valid CSRF token should not be rejected")
	}
	if !c.IsAborted() {
		// should proceed to next handler
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
	initJWTSecret(t, "test-secret-key-32chars-long!!!!")

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
