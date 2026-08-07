package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestNewRateLimiter(t *testing.T) {
	rl := NewRateLimiter(context.Background(), 2, 100*time.Millisecond)
	if rl == nil {
		t.Fatal("NewRateLimiter returned nil")
	}
	if rl.limit != 2 {
		t.Errorf("expected limit 2, got %d", rl.limit)
	}
}

func TestRateLimiterBasic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rl := NewRateLimiter(context.Background(), 2, 1*time.Minute)

	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("POST", "/", nil)
		c.Request.RemoteAddr = "10.0.0.1:12345"

		rl.Limit()(c)
		if w.Code != http.StatusOK {
			t.Errorf("request %d should be allowed, got %d", i+1, w.Code)
		}
	}

	// Third request should be rate limited
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/", nil)
	c.Request.RemoteAddr = "10.0.0.1:12345"

	rl.Limit()(c)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("third request should be rate limited, got %d", w.Code)
	}
}

func TestRateLimiterDifferentIPs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rl := NewRateLimiter(context.Background(), 1, 1*time.Minute)

	for _, ip := range []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("POST", "/", nil)
		c.Request.RemoteAddr = ip + ":12345"

		rl.Limit()(c)
		if w.Code != http.StatusOK {
			t.Errorf("request from %s should be allowed, got %d", ip, w.Code)
		}
	}
}

func TestRateLimiterWindowReset(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rl := NewRateLimiter(context.Background(), 1, 50*time.Millisecond)

	// First request - allowed
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/", nil)
	c.Request.RemoteAddr = "10.0.0.1:12345"
	rl.Limit()(c)
	if w.Code != http.StatusOK {
		t.Error("first request should be allowed")
	}

	// Second request - rate limited
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/", nil)
	c.Request.RemoteAddr = "10.0.0.1:12345"
	rl.Limit()(c)
	if w.Code != http.StatusTooManyRequests {
		t.Error("second request should be rate limited")
	}

	// Wait for window to reset
	time.Sleep(60 * time.Millisecond)

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/", nil)
	c.Request.RemoteAddr = "10.0.0.1:12345"
	rl.Limit()(c)
	if w.Code != http.StatusOK {
		t.Error("request after window reset should be allowed")
	}
}

func TestCacheControl(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		method     string
		maxAge     int
		wantHeader bool
	}{
		{"GET 200 sets cache", "GET", 30, true},
		{"POST does not cache", "POST", 30, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(tt.method, "/test", nil)
			c.Writer.WriteHeader(200)
			c.Writer.Write([]byte("test body"))

			middleware := CacheControl(tt.maxAge)
			middleware(c)

			cc := w.Header().Get("Cache-Control")
			if tt.wantHeader && cc == "" {
				t.Error("expected Cache-Control header, got empty")
			}
			if !tt.wantHeader && cc != "" {
				t.Errorf("unexpected Cache-Control header: %s", cc)
			}
			if tt.wantHeader && cc != "public, max-age=30" {
				t.Errorf("Cache-Control = %q, want %q", cc, "public, max-age=30")
			}
		})
	}
}

func TestRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test", nil)

	RequestID()(c)

	id := w.Header().Get("X-Request-ID")
	if id == "" {
		t.Error("expected X-Request-ID header, got empty")
	}
	if len(id) != 32 {
		t.Errorf("X-Request-ID length = %d, want 32", len(id))
	}
}

func TestRequestIDReuseClientValue(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test", nil)
	c.Request.Header.Set("X-Request-ID", "my-custom-id")

	RequestID()(c)

	id := w.Header().Get("X-Request-ID")
	if id != "my-custom-id" {
		t.Errorf("X-Request-ID = %q, want %q", id, "my-custom-id")
	}
}

func TestSecurityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test", nil)

	SecurityHeaders(false)(c)

	checks := map[string]string{
		"X-Content-Type-Options":         "nosniff",
		"X-Frame-Options":                "SAMEORIGIN",
		"Referrer-Policy":                "same-origin",
		"Cross-Origin-Opener-Policy":     "same-origin",
		"Cross-Origin-Resource-Policy":   "same-origin",
	}
	for header, want := range checks {
		got := w.Header().Get(header)
		if got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
}

func TestNoCache(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("GET", "/test", nil)

	NoCache()(c)

	if cc := w.Header().Get("Cache-Control"); cc != "no-cache, no-store, must-revalidate" {
		t.Errorf("Cache-Control = %q", cc)
	}
}
