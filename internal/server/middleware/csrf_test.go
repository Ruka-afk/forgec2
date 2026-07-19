package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCSRFProtect_GET_SetsCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/test", nil)

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
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/test", nil)
	c.Request.Header.Set(csrfHeaderName, "wrong-token")
	c.Request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "correct-token"})

	CSRFProtect()(c)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestCSRFProtect_POST_AcceptsValidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/test", nil)
	c.Request.Header.Set(csrfHeaderName, "valid-token")
	c.Request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "valid-token"})

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
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/test", nil)
	c.Request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "existing-token"})

	CSRFProtect()(c)

	cookies := w.Result().Cookies()
	for _, ck := range cookies {
		if ck.Name == csrfCookieName && ck.Value != "existing-token" {
			t.Error("should not overwrite existing CSRF cookie")
		}
	}
}
