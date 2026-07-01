package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestNewRateLimiter(t *testing.T) {
	rl := NewRateLimiter(2, 100*time.Millisecond)
	if rl == nil {
		t.Fatal("NewRateLimiter returned nil")
	}
	if rl.limit != 2 {
		t.Errorf("expected limit 2, got %d", rl.limit)
	}
}

func TestRateLimiterBasic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rl := NewRateLimiter(2, 1*time.Minute)

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
	rl := NewRateLimiter(1, 1*time.Minute)

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
	rl := NewRateLimiter(1, 50*time.Millisecond)

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


