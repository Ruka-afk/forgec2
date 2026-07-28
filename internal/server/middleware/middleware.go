package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter holds rate limiting data
type RateLimiter struct {
	visitors map[string]*visitor
	mu       sync.Mutex
	limit    int           // requests per window
	window   time.Duration // time window
	stop     chan struct{}
}

type visitor struct {
	timestamp time.Time
	count     int
}

// NewRateLimiter creates a new rate limiter with periodic cleanup that stops on context cancellation
func NewRateLimiter(ctx context.Context, limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		limit:    limit,
		window:   window,
		stop:     make(chan struct{}),
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("Rate limiter cleanup panicked", "recover", r)
			}
		}()
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-rl.stop:
				return
			case <-ticker.C:
				rl.mu.Lock()
				now := time.Now()
				for ip, v := range rl.visitors {
					if now.Sub(v.timestamp) > rl.window*2 {
						delete(rl.visitors, ip)
					}
				}
				rl.mu.Unlock()
			}
		}
	}()
	return rl
}

// Stop terminates the cleanup goroutine.
func (rl *RateLimiter) Stop() {
	close(rl.stop)
}

// Limit returns a middleware handler for rate limiting
func (rl *RateLimiter) Limit() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if ip == "" {
			ip = "unknown"
		}

		rl.mu.Lock()
		v, exists := rl.visitors[ip]
		now := time.Now()

		if !exists {
			rl.visitors[ip] = &visitor{timestamp: now, count: 1}
			rl.mu.Unlock()
			c.Next()
			return
		}

		// Reset if window has passed
		if now.Sub(v.timestamp) > rl.window {
			v.timestamp = now
			v.count = 1
			rl.mu.Unlock()
			c.Next()
			return
		}

		// Check limit
		if v.count >= rl.limit {
			rl.mu.Unlock()
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate_limit_exceeded",
				"message":     "Too many requests, please try again later",
				"retry_after": int(rl.window.Seconds()),
			})
			c.Abort()
			return
		}

		v.count++
		rl.mu.Unlock()
		c.Next()
	}
}

// ErrorHandler middleware for unified error handling
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Check if there are any errors
		if len(c.Errors) > 0 {
			lastError := c.Errors.Last()

			// Log the real error server-side; never leak internal details to clients
			slog.Error("Handler error", "path", c.Request.URL.Path, "method", c.Request.Method, "error", lastError.Error())

			// Determine status code
			statusCode := http.StatusInternalServerError
			if c.Writer.Status() != http.StatusOK && c.Writer.Status() != http.StatusInternalServerError {
				statusCode = c.Writer.Status()
			}

			// Return JSON error response with generic message
			c.JSON(statusCode, gin.H{
				"success": false,
				"error":   "internal_error",
				"message": "An internal error occurred",
			})
		}
	}
}

// SecurityHeaders adds security headers to responses
func SecurityHeaders(tlsEnabled bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "SAMEORIGIN")
		c.Header("Referrer-Policy", "same-origin")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; img-src 'self' data: blob:; font-src 'self' data: https://fonts.gstatic.com; connect-src 'self' ws: wss:; frame-ancestors 'none'")
		if tlsEnabled {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}

// RequestLogger logs all incoming requests
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		slog.Info("Request received", "method", c.Request.Method, "path", c.Request.URL.Path, "ip", c.ClientIP())
		c.Next()
	}
}

// NoCache adds cache-control headers to prevent caching
func NoCache() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		c.Next()
	}
}

// RequestBodyLimit wraps the request body with http.MaxBytesReader to reject
// oversized payloads before they are buffered into memory. File upload routes
// (multipart) should skip this middleware since they have their own size checks.
func RequestBodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}

// CORS returns a middleware that sets CORS headers based on the provided allowed origins.
// When allowedOrigins is empty, only localhost origins are permitted (safe default).
func CORS(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin == "" {
			c.Next()
			return
		}

		allowed := false
		if len(allowedOrigins) == 0 {
			// Default: only localhost origins
			if origin == "http://localhost" || origin == "http://127.0.0.1" ||
				origin == "http://localhost:8000" || origin == "http://127.0.0.1:8000" ||
				origin == "https://localhost" || origin == "https://127.0.0.1" ||
				origin == "https://localhost:8000" || origin == "https://127.0.0.1:8000" ||
				strings.HasPrefix(origin, "http://localhost:") || strings.HasPrefix(origin, "http://127.0.0.1:") {
				allowed = true
			}
		} else {
			for _, o := range allowedOrigins {
				if o == origin {
					allowed = true
					break
				}
			}
		}
		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		if allowed {
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// InFlightTracker counts active requests and blocks until all complete.
type InFlightTracker struct {
	wg sync.WaitGroup
}

// NewInFlightTracker creates a tracker for in-flight HTTP requests.
func NewInFlightTracker() *InFlightTracker {
	return &InFlightTracker{}
}

// Middleware increments the counter on entry and decrements on exit.
func (t *InFlightTracker) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		t.wg.Add(1)
		defer t.wg.Done()
		c.Next()
	}
}

// Wait blocks until all in-flight requests have completed.
func (t *InFlightTracker) Wait() {
	t.wg.Wait()
}

// CacheControl returns a middleware that sets cache-control headers for
// cacheable GET responses. Only caches responses with 200 status and a
// non-empty body. The maxAge parameter controls how long the response
// can be cached (in seconds).
func CacheControl(maxAge int) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if c.Request.Method == "GET" && c.Writer.Status() == 200 && c.Writer.Size() > 0 {
			c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d", maxAge))
		}
	}
}

// RequestID generates a unique request ID for each incoming request.
// If the client sends an X-Request-ID header, it is reused; otherwise a
// random 16-byte hex string is generated. The ID is set in the response
// header and stored in the Gin context under "request_id".
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" {
			b := make([]byte, 16)
			if _, err := rand.Read(b); err == nil {
				id = hex.EncodeToString(b)
			} else {
				id = "unknown"
			}
		}
		c.Set("request_id", id)
		c.Header("X-Request-ID", id)
		c.Next()
	}
}
