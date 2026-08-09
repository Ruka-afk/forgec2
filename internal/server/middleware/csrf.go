package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/gin-gonic/gin"
)

const csrfCookieName = "forgec2_csrf"
const csrfHeaderName = "X-CSRF-Token"

// csrfSecret holds the dedicated CSRF binding key (crypto.csrf_key). It is
// deliberately independent of the JWT secret so session-binder tokens cannot be
// forged with a leaked JWT secret (and vice versa).
var csrfSecret atomic.Value // []byte

// InitCSRFSecret stores the CSRF binding key from config. It must be called
// during server startup (and on config reload when crypto.csrf_key changes)
// before any requests are served.
func InitCSRFSecret(cfg *config.Config) error {
	keyHex := cfg.Crypto.CsrfKey
	if keyHex == "" {
		return errors.New("crypto.csrf_key is required (set a 64-character hex key; the legacy reuse of the JWT secret was removed)")
	}
	if len(keyHex) != 64 {
		return fmt.Errorf("crypto.csrf_key must be a 64-character hex string (32 bytes), got %d chars", len(keyHex))
	}
	b, err := hex.DecodeString(keyHex)
	if err != nil || len(b) != 32 {
		return fmt.Errorf("crypto.csrf_key must be a valid 64-character hex string (32 bytes)")
	}
	csrfSecret.Store(b)
	return nil
}

// CSRFProtect generates a CSRF token on GET requests and validates it on
// mutating requests (POST, PUT, DELETE, PATCH). The token is delivered via
// a cookie that JavaScript can read (HttpOnly=false) and must be echoed back
// in the X-CSRF-Token header. This double-submit pattern defeats CSRF because
// an attacker cannot read cookies from a cross-origin page.
//
// The CSRF token is derived from the session token via HMAC with the dedicated
// crypto.csrf_key, binding it to the current session so a token stolen from one
// session cannot be reused in another.
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
			secret := csrfSecret.Load().([]byte)
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
				secret := csrfSecret.Load().([]byte)
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
