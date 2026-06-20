package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var jwtSecret []byte

// CookieSecure controls the Secure flag on all cookies (set by InitJWTSecret from config)
var CookieSecure bool

const (
	JWTExpiry     = 24 * time.Hour
	JWTLongExpiry = 7 * 24 * time.Hour // "remember me"
	CookieMaxAge  = 86400
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
}

// AuthRequired middleware for web UI - validates JWT + DB user active
func AuthRequired(database *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr, err := c.Cookie("forgec2_session")
		if err != nil {
			slog.Debug("Auth failed: no session cookie", "path", c.Request.URL.Path, "ip", c.ClientIP())
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		})
		if err != nil || !token.Valid {
			slog.Warn("Auth failed: invalid token", "path", c.Request.URL.Path, "ip", c.ClientIP(), "err", err)
			c.SetCookie("forgec2_session", "", -1, "/", "", CookieSecure, true)
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		claims, ok := token.Claims.(*Claims)
		if !ok {
			slog.Warn("Auth failed: invalid claims", "path", c.Request.URL.Path)
			c.SetCookie("forgec2_session", "", -1, "/", "", CookieSecure, true)
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		// Verify user still exists and is active
		var user db.User
		if database.Where("id = ? AND is_active = ?", claims.UserID, true).First(&user).Error != nil {
			slog.Warn("Auth failed: user not found or inactive", "user_id", claims.UserID, "username", claims.Username)
			c.SetCookie("forgec2_session", "", -1, "/", "", CookieSecure, true)
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		// Force-logout check: if user's ForceLogoutAt > token IssuedAt, session was invalidated
		if !user.ForceLogoutAt.IsZero() && claims.IssuedAt != nil {
			if user.ForceLogoutAt.After(claims.IssuedAt.Time) {
				slog.Warn("Auth failed: force logout",
					"username", user.Username,
					"force_logout_at", user.ForceLogoutAt,
					"token_issued_at", claims.IssuedAt.Time,
					"path", c.Request.URL.Path)
				c.SetCookie("forgec2_session", "", -1, "/", "", CookieSecure, true)
				c.Redirect(http.StatusFound, "/login?error=session_expired")
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
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
			c.Abort()
			return
		}
		roleStr, ok := role.(string)
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
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
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
		c.Abort()
	}
}
