package middleware

import (
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

const (
	JWTExpiry    = 24 * time.Hour
	JWTLongExpiry = 7 * 24 * time.Hour // "remember me"
	CookieMaxAge = 86400
)

// Claims for JWT
type Claims struct {
	UserID      uint   `json:"user_id"`
	Username    string `json:"username"`
	Role        string `json:"role"`
	ClientIP    string `json:"client_ip"`
	jwt.RegisteredClaims
}

// InitJWTSecret initializes the JWT secret from config
func InitJWTSecret(cfg *config.Config) {
	secret := cfg.Server.JWTSecret
	if secret == "" {
		panic("JWT secret is empty after config load - security misconfiguration")
	}
	jwtSecret = []byte(secret)
}

func clientIP(c *gin.Context) string {
	ip := c.ClientIP()
	if ip == "" || ip == "::1" {
		ip = c.Request.Header.Get("X-Forwarded-For")
	}
	if ip == "" {
		ip = c.Request.RemoteAddr
	}
	return ip
}

// AuthRequired middleware for web UI - validates JWT + DB user active
func AuthRequired(database *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr, err := c.Cookie("forgec2_session")
		if err != nil {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		})
		if err != nil || !token.Valid {
			c.SetCookie("forgec2_session", "", -1, "/", "", false, true)
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		claims, ok := token.Claims.(*Claims)
		if !ok {
			c.SetCookie("forgec2_session", "", -1, "/", "", false, true)
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		// Validate IP binding
		if claims.ClientIP != "" && claims.ClientIP != clientIP(c) {
			c.SetCookie("forgec2_session", "", -1, "/", "", false, true)
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		// Verify user still exists and is active
		var user db.User
		if database.Where("id = ? AND is_active = ?", claims.UserID, true).First(&user).Error != nil {
			c.SetCookie("forgec2_session", "", -1, "/", "", false, true)
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		// Set user info in context
		c.Set("user_id", user.ID)
		c.Set("user", user.Username)
		c.Set("user_role", user.Role)
		c.Next()
	}
}

// GenerateToken creates a JWT for the session
func GenerateToken(user db.User, clientIP string, rememberMe bool) (string, error) {
	expiry := JWTExpiry
	if rememberMe {
		expiry = JWTLongExpiry
	}
	claims := &Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		ClientIP: clientIP,
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
