package middleware

import (
	"log/slog"
	"net/http"
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
}

type visitor struct {
	timestamp time.Time
	count     int
}

// NewRateLimiter creates a new rate limiter with periodic cleanup
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		limit:    limit,
		window:   window,
	}
	// Periodic cleanup of stale entries every 5 minutes
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		for range ticker.C {
			rl.mu.Lock()
			now := time.Now()
			for ip, v := range rl.visitors {
				if now.Sub(v.timestamp) > rl.window*2 {
					delete(rl.visitors, ip)
				}
			}
			rl.mu.Unlock()
		}
	}()
	return rl
}

// Limit returns a middleware handler for rate limiting
func (rl *RateLimiter) Limit() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if ip == "" {
			ip = "unknown"
		}

		rl.mu.Lock()
		defer rl.mu.Unlock()

		v, exists := rl.visitors[ip]
		now := time.Now()

		if !exists {
			rl.visitors[ip] = &visitor{timestamp: now, count: 1}
			c.Next()
			return
		}

		// Reset if window has passed
		if now.Sub(v.timestamp) > rl.window {
			v.timestamp = now
			v.count = 1
			c.Next()
			return
		}

		// Check limit
		if v.count >= rl.limit {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate_limit_exceeded",
				"message":     "Too many requests, please try again later",
				"retry_after": int(rl.window.Seconds()),
			})
			c.Abort()
			return
		}

		v.count++
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

			// Determine status code
			statusCode := http.StatusInternalServerError
			if c.Writer.Status() != http.StatusOK && c.Writer.Status() != http.StatusInternalServerError {
				statusCode = c.Writer.Status()
			}

			// Return JSON error response
			c.JSON(statusCode, gin.H{
				"error":   "internal_error",
				"message": lastError.Error(),
			})
		}
	}
}

// Recovery middleware for panic recovery
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				// Log the panic
				// In production, you would want to log this to a service like Sentry

				c.JSON(http.StatusInternalServerError, gin.H{
					"error":   "server_error",
					"message": "An unexpected error occurred",
				})
			}
		}()
		c.Next()
	}
}

// SecurityHeaders adds security headers to responses
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
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

// StaticCache adds long-term cache headers for static assets
func StaticCache() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		if isStaticAsset(path) {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
			c.Header("X-Content-Type-Options", "nosniff")
		}

		c.Next()
	}
}

func isStaticAsset(path string) bool {
	staticExtensions := map[string]bool{
		".css":   true,
		".js":    true,
		".png":   true,
		".jpg":   true,
		".jpeg":  true,
		".gif":   true,
		".svg":   true,
		".ico":   true,
		".woff":  true,
		".woff2": true,
		".ttf":   true,
		".eot":   true,
		".webp":  true,
	}

	for ext := range staticExtensions {
		if len(path) > len(ext) && path[len(path)-len(ext):] == ext {
			return true
		}
	}
	return false
}
