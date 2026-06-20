package middleware

import (
	"crypto/rand"
	"encoding/hex"
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
			// For form submissions, redirect back with error instead of JSON
			if c.Request.Method == "POST" && c.Request.Header.Get("Content-Type") == "application/x-www-form-urlencoded" {
				c.Redirect(http.StatusFound, "/login?error=rate_limited")
				c.Abort()
				return
			}
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

// GenerateCSRFToken creates a random CSRF token
func GenerateCSRFToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// CSRFProtection middleware using double-submit cookie pattern
func CSRFProtection() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == "GET" || c.Request.Method == "HEAD" {
			// Only generate CSRF token if not already present
			_, err := c.Cookie("csrf_token")
			if err != nil {
				// No CSRF cookie, generate one
				token := GenerateCSRFToken()
				c.SetCookie("csrf_token", token, CookieMaxAge, "/", "", CookieSecure, false)
				c.Set("csrf_token_value", token)
			} else {
				// CSRF cookie exists, use it
				token, _ := c.Cookie("csrf_token")
				c.Set("csrf_token_value", token)
			}
			c.Next()
			return
		}
		cookie, err := c.Cookie("csrf_token")
		if err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "CSRF cookie not found"})
			return
		}
		// Check X-CSRF-Token header first (for AJAX/HTMX), then form field (for regular forms)
		token := c.GetHeader("X-CSRF-Token")
		if token == "" {
			token = c.PostForm("csrf_token")
		}
		if token == "" || token != cookie {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "CSRF token mismatch or missing"})
			return
		}
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
		c.Header("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-inline' https://cdn.tailwindcss.com https://unpkg.com; "+
				"style-src 'self' 'unsafe-inline' https://cdnjs.cloudflare.com; "+
				"img-src 'self' data:; "+
				"font-src 'self' https://cdnjs.cloudflare.com; "+
				"connect-src 'self' ws: wss:; "+
				"form-action 'self'")
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
