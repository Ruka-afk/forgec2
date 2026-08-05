package middleware

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const minJWTSecretLen = 32

var jwtSecret atomic.Value

// trustedProxyIPs holds the operator-configured proxy IPs/CIDRs whose
// X-Forwarded-* headers are trusted (set via SetTrustedProxyIPs).
var trustedProxyIPs atomic.Value // []netip.Prefix

// SetTrustedProxyIPs configures the reverse proxies whose X-Forwarded-Proto
// header IsSecureConnection may trust. An empty list disables header trust.
func SetTrustedProxyIPs(proxies []string) {
	prefixes := make([]netip.Prefix, 0, len(proxies))
	for _, p := range proxies {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if addr, err := netip.ParseAddr(p); err == nil {
			prefixes = append(prefixes, netip.PrefixFrom(addr, addr.BitLen()))
			continue
		}
		if pref, err := netip.ParsePrefix(p); err == nil {
			prefixes = append(prefixes, pref.Masked())
		}
	}
	trustedProxyIPs.Store(prefixes)
}

func peerAddr(c *gin.Context) netip.Addr {
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		host = c.Request.RemoteAddr
	}
	if addr, err := netip.ParseAddr(strings.TrimSpace(host)); err == nil {
		return addr.Unmap()
	}
	return netip.Addr{}
}

func isTrustedPeer(c *gin.Context) bool {
	prefixes, _ := trustedProxyIPs.Load().([]netip.Prefix)
	if len(prefixes) == 0 {
		return false
	}
	addr := peerAddr(c)
	if !addr.IsValid() {
		return false
	}
	for _, pref := range prefixes {
		if pref.Contains(addr) {
			return true
		}
	}
	return false
}

// apiKeyRateLimiter provides simple per-IP rate limiting for API key auth.
type apiKeyRateLimiter struct {
	mu      sync.Mutex
	entries map[string]*apiKeyLimitEntry
}

type apiKeyLimitEntry struct {
	attempts    int
	windowStart time.Time
}

const (
	apiKeyMaxAttempts = 10
	apiKeyWindowSec   = 60
	apiKeyLockoutSec  = 300
)

func (r *apiKeyRateLimiter) isLocked(ip string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.entries[ip]
	if !ok {
		return false
	}
	if !entry.windowStart.IsZero() && time.Since(entry.windowStart) > time.Duration(apiKeyLockoutSec)*time.Second {
		delete(r.entries, ip)
		return false
	}
	return entry.attempts >= apiKeyMaxAttempts
}

func (r *apiKeyRateLimiter) recordFailure(ip string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.entries[ip]
	if !ok {
		if len(r.entries) >= 10000 {
			for k := range r.entries {
				delete(r.entries, k)
				break
			}
		}
		entry = &apiKeyLimitEntry{windowStart: time.Now()}
		r.entries[ip] = entry
	}
	entry.attempts++
	entry.windowStart = time.Now()
}

var globalAPIKeyLimiter = &apiKeyRateLimiter{
	entries: make(map[string]*apiKeyLimitEntry),
}

// respondError sends a consistent {success: false, error: msg} envelope.
func respondError(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"success": false, "error": msg})
}

// CookieSecure controls the Secure flag on all cookies (set by InitJWTSecret from config)
var CookieSecure bool

// CookieDomain controls the Domain attribute on all cookies (set by InitJWTSecret from config)
var CookieDomain string

// RequireTLSForAuth when true, prevents session cookies from being issued
// unless the connection is TLS (X-Forwarded-Proto: https or direct TLS).
var RequireTLSForAuth bool

const (
	JWTExpiry     = 24 * time.Hour
	JWTLongExpiry = 7 * 24 * time.Hour // "remember me"
	userCacheTTL  = 5 * time.Minute
)

// Claims for JWT
type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// InitJWTSecret initializes the JWT secret and cookie secure flag from config.
// If the secret is shorter than 32 characters, it auto-generates a secure random
// secret and logs a warning.
func InitJWTSecret(cfg *config.Config, configPath string) error {
	secret := cfg.Server.JWTSecret
	if secret == "" {
		slog.Warn("JWT secret is empty, auto-generating a secure random secret")
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return fmt.Errorf("JWT secret is empty and cannot generate random secret: %w", err)
		}
		cfg.Server.JWTSecret = hex.EncodeToString(b)
		if configPath != "" {
			if err := cfg.Save(configPath); err != nil {
				slog.Error("Failed to save auto-generated JWT secret to config", "error", err)
			}
		}
		secret = cfg.Server.JWTSecret
	}
	if len(secret) < minJWTSecretLen {
		slog.Warn("JWT secret is too short, auto-generating a secure random secret",
			"length", len(secret), "min_required", minJWTSecretLen)
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return fmt.Errorf("JWT secret is too short and cannot generate random secret: %w", err)
		}
		cfg.Server.JWTSecret = hex.EncodeToString(b)
		if configPath != "" {
			if err := cfg.Save(configPath); err != nil {
				slog.Error("Failed to save auto-generated JWT secret to config", "error", err)
			}
		}
		secret = cfg.Server.JWTSecret
	}
	jwtSecret.Store([]byte(secret))
	CookieSecure = cfg.Server.TLSEnabled
	CookieDomain = cfg.Server.CookieDomain
	RequireTLSForAuth = cfg.Server.RequireTLSForAuth
	if !CookieSecure {
		if RequireTLSForAuth {
			slog.Warn("Cookie Secure flag disabled but RequireTLSForAuth=true — session cookies will NOT be set on non-TLS connections")
		} else {
			slog.Warn("Cookie Secure flag is disabled (TLS not enabled) — session cookies will be sent over plain HTTP")
		}
	}
	return nil
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

// IsSecureConnection returns true if the request arrived over TLS, either
// directly or via a trusted reverse proxy that sets X-Forwarded-Proto: https.
// The X-Forwarded-Proto header is only honored when the direct peer is a
// configured trusted proxy; otherwise any client could spoof it on a plaintext
// connection.
func IsSecureConnection(c *gin.Context) bool {
	if c.Request.TLS != nil {
		return true
	}
	if !isTrustedPeer(c) {
		return false
	}
	return strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
}

// wantsJSONAuth prefers a JSON 401 over HTML redirect for API clients and API paths.
// Browser navigations (text/html Accept, page paths) still redirect to /login.
func wantsJSONAuth(c *gin.Context) bool {
	if isWebSocketUpgrade(c) {
		return true
	}
	accept := c.GetHeader("Accept")
	if strings.Contains(accept, "application/json") {
		return true
	}
	if strings.Contains(c.GetHeader("Content-Type"), "application/json") {
		return true
	}
	if c.GetHeader("X-Requested-With") == "XMLHttpRequest" {
		return true
	}
	if c.GetHeader("X-CSRF-Token") != "" {
		return true
	}
	path := c.Request.URL.Path
	if strings.HasPrefix(path, "/api") ||
		strings.HasPrefix(path, "/ws") ||
		strings.HasPrefix(path, "/extc2") ||
		strings.HasPrefix(path, "/admin/") ||
		strings.HasPrefix(path, "/debug/") {
		return true
	}
	return false
}

func authFail(c *gin.Context, logMsg string, args ...any) {
	if len(args) > 0 {
		slog.Warn(logMsg, args...)
	} else {
		slog.Debug(logMsg)
	}
	if wantsJSONAuth(c) {
		respondError(c, http.StatusUnauthorized, "unauthorized")
	} else {
		c.Redirect(http.StatusFound, "/login")
	}
	c.Abort()
}

// AuthRequired middleware for web UI - validates JWT + DB user active
func AuthRequired(database *gorm.DB) gin.HandlerFunc {
	type cacheEntry struct {
		user      db.User
		expiresAt time.Time
	}
	var cacheMu sync.RWMutex
	userCache := make(map[uint]cacheEntry)

	return func(c *gin.Context) {
		// API key authentication (X-API-Key header)
		if apiKey := c.GetHeader("X-API-Key"); apiKey != "" {
			clientIP := c.ClientIP()
			if globalAPIKeyLimiter.isLocked(clientIP) {
				authFail(c, "Auth failed: API key rate limit exceeded", "path", c.Request.URL.Path, "ip", clientIP)
				return
			}
			h := sha256.Sum256([]byte(apiKey))
			hash := fmt.Sprintf("%x", h)
			var ak db.ApiKey
			if database.Where("key_hash = ? AND active = ?", hash, true).First(&ak).Error == nil {
				if ak.ExpiresAt.IsZero() || time.Now().Before(ak.ExpiresAt) {
					var user db.User
					if database.Where("id = ? AND is_active = ?", ak.UserID, true).First(&user).Error == nil {
						database.Model(&ak).Update("last_used", time.Now())
						c.Set("user_id", user.ID)
						c.Set("user", user.Username)
						c.Set("user_role", user.Role)
						// API-key requests carry no session cookie; CSRFProtect
						// skips validation for them (the bearer header itself
						// is not attachable cross-origin).
						c.Set("auth_via_api_key", true)
						c.Next()
						return
					}
				}
			}
			globalAPIKeyLimiter.recordFailure(clientIP)
			authFail(c, "Auth failed: invalid API key", "path", c.Request.URL.Path, "ip", clientIP)
			return
		}

		tokenStr, err := c.Cookie("forgec2_session")
		if err != nil {
			authFail(c, "Auth failed: no session token", "path", c.Request.URL.Path, "ip", c.ClientIP())
			return
		}
		if tokenStr == "" {
			authFail(c, "Auth failed: no session token", "path", c.Request.URL.Path, "ip", c.ClientIP())
			return
		}

		token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return jwtSecret.Load().([]byte), nil
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

		// Verify user still exists and is active (with short TTL cache)
		var user db.User
		cacheMu.RLock()
		entry, cached := userCache[claims.UserID]
		cacheMu.RUnlock()
		if cached && time.Now().Before(entry.expiresAt) {
			user = entry.user
		} else {
			if database.Where("id = ? AND is_active = ?", claims.UserID, true).First(&user).Error != nil {
				SetCookieWithSameSite(c, "forgec2_session", "", -1, "/", CookieSecure, true, http.SameSiteLaxMode)
				authFail(c, "Auth failed: user not found or inactive", "user_id", claims.UserID, "username", claims.Username)
				return
			}
			cacheMu.Lock()
			userCache[claims.UserID] = cacheEntry{user: user, expiresAt: time.Now().Add(userCacheTTL)}
			cacheMu.Unlock()
		}

		// Force-logout check: if user's ForceLogoutAt > token IssuedAt, session was invalidated
		if !user.ForceLogoutAt.IsZero() && claims.IssuedAt != nil {
			if user.ForceLogoutAt.After(claims.IssuedAt.Time) {
				SetCookieWithSameSite(c, "forgec2_session", "", -1, "/", CookieSecure, true, http.SameSiteLaxMode)
				if wantsJSONAuth(c) {
					respondError(c, http.StatusUnauthorized, "session_expired")
				} else {
					c.Redirect(http.StatusFound, "/login?error=session_expired")
				}
				c.Abort()
				return
			}
		}

		// Per-session revocation check
		if isSessionRevoked(database, tokenStr) {
			SetCookieWithSameSite(c, "forgec2_session", "", -1, "/", CookieSecure, true, http.SameSiteLaxMode)
			if wantsJSONAuth(c) {
				respondError(c, http.StatusUnauthorized, "session_revoked")
			} else {
				c.Redirect(http.StatusFound, "/login?error=session_revoked")
			}
			c.Abort()
			return
		}

		// Set user info in context
		c.Set("user_id", user.ID)
		c.Set("user", user.Username)
		c.Set("user_role", user.Role)
		if claims.ExpiresAt != nil {
			c.Set("session_exp", claims.ExpiresAt.Time)
		}
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
	return token.SignedString(jwtSecret.Load().([]byte))
}

// CheckPassword compares hash
func CheckPassword(hash, password string) bool {
	if hash == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// BcryptCost is the bcrypt hash cost used for password hashing.
// Set via SetBcryptCost at startup from config.
var BcryptCost = 12

// SetBcryptCost sets the bcrypt cost for password hashing (clamped to [4, 31]).
func SetBcryptCost(cost int) {
	if cost >= 4 && cost <= 31 {
		BcryptCost = cost
	}
}

// HashPassword for storage
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
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

// ParseToken validates a JWT token string and returns the claims.
func ParseToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret.Load().([]byte), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}

func TokenHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", h)
}

func isSessionRevoked(database *gorm.DB, tokenStr string) bool {
	hash := TokenHash(tokenStr)
	var count int64
	if err := database.Table("user_sessions").
		Where("token_hash = ? AND revoked_at > ?", hash, time.Unix(0, 0)).
		Count(&count).Error; err != nil {
		slog.Warn("Session revocation check failed, allowing access", "err", err)
		return false
	}
	return count > 0
}
