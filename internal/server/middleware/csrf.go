package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

const csrfCookieName = "forgec2_csrf"
const csrfHeaderName = "X-CSRF-Token"

// CSRFProtect generates a CSRF token on GET requests and validates it on
// mutating requests (POST, PUT, DELETE, PATCH). The token is delivered via
// a cookie that JavaScript can read (HttpOnly=false) and must be echoed back
// in the X-CSRF-Token header. This double-submit pattern defeats CSRF because
// an attacker cannot read cookies from a cross-origin page.
func CSRFProtect() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method

		// Only protect state-changing methods
		if method != "GET" && method != "HEAD" && method != "OPTIONS" {
			headerToken := c.GetHeader(csrfHeaderName)
			cookieToken, _ := c.Cookie(csrfCookieName)

			if cookieToken == "" || headerToken == "" || headerToken != cookieToken {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"success": false,
					"error":   "Missing or invalid CSRF token",
				})
				return
			}
		}

		// On every GET/HEAD, rotate the CSRF token to prevent stale cookie injection
		if method == "GET" || method == "HEAD" {
			token, err := generateCSRFToken()
			if err != nil {
				slog.Error("Failed to generate CSRF token", "error", err)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"error":   "Internal server error",
				})
				return
			}
			SetCookieWithSameSite(c, csrfCookieName, token, 0, "/", CookieSecure, false, http.SameSiteLaxMode)
		}

		c.Next()
	}
}

func generateCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("CRNG failed for CSRF token: %w", err)
	}
	return hex.EncodeToString(b), nil
}
