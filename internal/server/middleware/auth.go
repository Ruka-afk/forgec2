package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var jwtSecret []byte

// respondError sends a consistent {success: false, error: msg} envelope.
func respondError(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"success": false, "error": msg})
}

// CookieSecure controls the Secure flag on all cookies (set by InitJWTSecret from config)
var CookieSecure bool

// CookieDomain controls the Domain attribute on all cookies (set by InitJWTSecret from config)
var CookieDomain string

const (
	JWTExpiry     = 24 * time.Hour
	JWTLongExpiry = 7 * 24 * time.Hour // "remember me"
)

// Claims for JWT
type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// InitJWTSecret initializes the JWT secret and cookie secure flag from config
func InitJWTSecret(cfg *config.Config) {
	secret := cfg.Server.JWTSecret
	if secret == "" {
		panic("JWT secret is empty after config load - security misconfiguration")
	}
	jwtSecret = []byte(secret)
	CookieSecure = cfg.Server.TLSEnabled
	CookieDomain = cfg.Server.CookieDomain
}

// SetCookieWithSameSite sets a cookie with SameSite attribute.
// Gin's SetCookie does not support SameSite, so we use http.SetCookie directly.
func SetCookieWithSameSite(c *gin.Context, name, value string, maxAge int, path string, secure, httpOnly bool, sameSite http.SameSite) {
	domain := CookieDomain
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    value,
		MaxAge:   maxAge,
		Path:     path,
		Domain:   domain,
		Secure:   secure,
		HttpOnly: httpOnly,
		SameSite: sameSite,
	})
}

func isWebSocketUpgrade(c *gin.Context) bool {
	return strings.EqualFold(c.GetHeader("Upgrade"), "websocket") ||
		strings.Contains(strings.ToLower(c.GetHeader("Connection")), "upgrade")
}

func authFail(c *gin.Context, logMsg string, args ...any) {
	if len(args) > 0 {
		slog.Warn(logMsg, args...)
	} else {
		slog.Debug(logMsg)
	}
	if isWebSocketUpgrade(c) {
		respondError(c, http.StatusUnauthorized, "unauthorized")
	} else {
		c.Redirect(http.StatusFound, "/login")
	}
	c.Abort()
}

// AuthRequired middleware for web UI - validates JWT + DB user active
func AuthRequired(database *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr, err := c.Cookie("forgec2_session")
		if err != nil {
			tokenStr = c.Query("token")
		}
		if tokenStr == "" {
			authFail(c, "Auth failed: no session token", "path", c.Request.URL.Path, "ip", c.ClientIP())
			return
		}

		token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return jwtSecret, nil
		})
		if err != nil || !token.Valid {
			SetCookieWithSameSite(c, "forgec2_session", "", -1, "/", CookieSecure, true, http.SameSiteLaxMode)
			authFail(c, "Auth failed: invalid token", "path", c.Request.URL.Path, "ip", c.ClientIP(), "err", err)
			return
		}

		claims, ok := token.Claims.(*Claims)
		if !ok {
			SetCookieWithSameSite(c, "forgec2_session", "", -1, "/", CookieSecure, true, http.SameSiteLaxMode)
			authFail(c, "Auth failed: invalid claims", "path", c.Request.URL.Path)
			return
		}

		// Verify user still exists and is active
		var user db.User
		if database.Where("id = ? AND is_active = ?", claims.UserID, true).First(&user).Error != nil {
			SetCookieWithSameSite(c, "forgec2_session", "", -1, "/", CookieSecure, true, http.SameSiteLaxMode)
			authFail(c, "Auth failed: user not found or inactive", "user_id", claims.UserID, "username", claims.Username)
			return
		}

		// Force-logout check: if user's ForceLogoutAt > token IssuedAt, session was invalidated
		if !user.ForceLogoutAt.IsZero() && claims.IssuedAt != nil {
			if user.ForceLogoutAt.After(claims.IssuedAt.Time) {
				SetCookieWithSameSite(c, "forgec2_session", "", -1, "/", CookieSecure, true, http.SameSiteLaxMode)
				if isWebSocketUpgrade(c) {
					respondError(c, http.StatusUnauthorized, "session_expired")
				} else {
					c.Redirect(http.StatusFound, "/login?error=session_expired")
				}
				c.Abort()
				return
			}
		}

		// Set user info in context
		c.Set("user_id", user.ID)
		c.Set("user", user.Username)
		c.Set("user_role", user.Role)
		c.Next()
	}
}

// GenerateToken creates a JWT for the session
func GenerateToken(user db.User, rememberMe bool, sessionMaxAgeHours int) (string, error) {
	expiry := time.Duration(sessionMaxAgeHours) * time.Hour
	if expiry <= 0 || expiry > JWTLongExpiry {
		expiry = JWTExpiry
	}
	if rememberMe {
		expiry = JWTLongExpiry
	}
	claims := &Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// CheckPassword compares hash
func CheckPassword(hash, password string) bool {
	if hash == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// HashPassword for storage
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

// RequireRole returns middleware that restricts access to specified roles
func RequireRole(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("user_role")
		if !exists {
			respondError(c, http.StatusForbidden, "access denied")
			c.Abort()
			return
		}
		roleStr, ok := role.(string)
		if !ok {
			respondError(c, http.StatusForbidden, "access denied")
			c.Abort()
			return
		}
		for _, allowed := range allowedRoles {
			if roleStr == allowed {
				c.Next()
				return
			}
		}
		// "admin" overrides all role restrictions
		if roleStr == "admin" {
			c.Next()
			return
		}
		respondError(c, http.StatusForbidden, "insufficient permissions")
		c.Abort()
	}
}

func RequirePermission(permissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("user_role")
		if !exists {
			respondError(c, http.StatusForbidden, "access denied")
			c.Abort()
			return
		}
		roleStr, ok := role.(string)
		if !ok {
			respondError(c, http.StatusForbidden, "access denied")
			c.Abort()
			return
		}
		if roleStr == "admin" {
			c.Next()
			return
		}
		for _, perm := range permissions {
			if db.RoleHasPermission(roleStr, perm) {
				c.Next()
				return
			}
		}
		respondError(c, http.StatusForbidden, "insufficient permissions")
		c.Abort()
	}
}

func RequireAllPermissions(permissions ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("user_role")
		if !exists {
			respondError(c, http.StatusForbidden, "access denied")
			c.Abort()
			return
		}
		roleStr, ok := role.(string)
		if !ok {
			respondError(c, http.StatusForbidden, "access denied")
			c.Abort()
			return
		}
		if roleStr == "admin" {
			c.Next()
			return
		}
		for _, perm := range permissions {
			if !db.RoleHasPermission(roleStr, perm) {
				respondError(c, http.StatusForbidden, "insufficient permissions")
				c.Abort()
				return
			}
		}
		c.Next()
	}
}
