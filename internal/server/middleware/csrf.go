package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
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
//
// The CSRF token is derived from the session token via HMAC, binding it to
// the current session so a token stolen from one session cannot be reused
// in another.
func CSRFProtect() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method

		// Only protect state-changing methods
		if method != "GET" && method != "HEAD" && method != "OPTIONS" {
			// Requests authenticated via API key (X-API-Key) carry no session
			// cookie: the secret bearer header cannot be attached cross-origin,
			// so CSRF is not applicable.
			if c.GetBool("auth_via_api_key") {
				c.Next()
				return
			}
			headerToken := c.GetHeader(csrfHeaderName)
			sessionToken, err := c.Cookie("forgec2_session")
			if sessionToken == "" || err != nil {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"success": false,
					"error":   "Missing or invalid CSRF token",
				})
				return
			}
			secret := jwtSecret.Load().([]byte)
			expected := deriveCSRFToken(sessionToken, secret)
			if headerToken == "" || headerToken != expected {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"success": false,
					"error":   "Missing or invalid CSRF token",
				})
				return
			}
		}

		// On every GET/HEAD, rotate the CSRF token to prevent stale cookie injection
		if method == "GET" || method == "HEAD" {
			sessionToken, err := c.Cookie("forgec2_session")
			if err == nil && sessionToken != "" {
				secret := jwtSecret.Load().([]byte)
				token := deriveCSRFToken(sessionToken, secret)
				SetCookieWithSameSite(c, csrfCookieName, token, 0, "/", CookieSecure, false, http.SameSiteLaxMode)
			}
		}

		c.Next()
	}
}

func deriveCSRFToken(sessionToken string, secret []byte) string {
	h := sha256.Sum256([]byte(sessionToken))
	mac := hmac.New(sha256.New, secret)
	mac.Write(h[:])
	return hex.EncodeToString(mac.Sum(nil))
}
