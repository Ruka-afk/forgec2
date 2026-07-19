package middleware

import (
	"crypto/rand"
	"encoding/hex"
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
					"error":   "csrf_validation_failed",
					"message": "Missing or invalid CSRF token",
				})
				return
			}
		}

		// On every GET, ensure a fresh CSRF cookie exists
		if method == "GET" || method == "HEAD" {
			if _, err := c.Cookie(csrfCookieName); err != nil {
				token := generateCSRFToken()
				SetCookieWithSameSite(c, csrfCookieName, token, 0, "/", CookieSecure, false, http.SameSiteLaxMode)
			}
		}

		c.Next()
	}
}

func generateCSRFToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}
