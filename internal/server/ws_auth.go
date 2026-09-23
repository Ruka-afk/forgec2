package server

import (
	"net/http"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

// wsAuthInterval is how often a live operator/notification WebSocket re-checks
// that its session is still valid (force-logout, disable, revoke). Without
// this, AuthRequired rejections on HTTP never reach an already-upgraded socket.
const wsAuthInterval = 30 * time.Second

// authenticateOperatorWS validates the forgec2_session cookie for operator
// WebSocket endpoints with the same checks as AuthRequired: parseable token,
// live (non-revoked) session row, user still active, and force-logout not
// after token IssuedAt. On failure it writes the JSON error and returns false.
func (s *Server) authenticateOperatorWS(c *gin.Context) (*middleware.Claims, string, bool) {
	tokenStr, err := c.Cookie("forgec2_session")
	if err != nil || tokenStr == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "no session token"})
		return nil, "", false
	}
	claims, err := middleware.ParseToken(tokenStr)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "invalid token"})
		return nil, "", false
	}
	revoked, err := s.isSessionRevoked(tokenStr)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "auth_unavailable"})
		return nil, "", false
	}
	if revoked {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "session_revoked"})
		return nil, "", false
	}
	if !s.wsSessionUserOK(claims) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "session_expired"})
		return nil, "", false
	}
	return claims, tokenStr, true
}

// wsSessionUserOK re-loads the token's user and applies the AuthRequired
// account checks (IsActive + force-logout vs IssuedAt). A DB error fails
// closed so a wedged database cannot keep a disabled session alive.
func (s *Server) wsSessionUserOK(claims *middleware.Claims) bool {
	if claims == nil {
		return false
	}
	var user db.User
	if err := s.db.Where("id = ?", claims.UserID).First(&user).Error; err != nil {
		return false
	}
	if !user.IsActive {
		return false
	}
	if !user.ForceLogoutAt.IsZero() && claims.IssuedAt != nil &&
		user.ForceLogoutAt.After(claims.IssuedAt.Time) {
		return false
	}
	return true
}

// startWSAuthRecheck periodically re-validates a live operator WS session and
// closes the connection when the account is disabled, force-logged-out, or the
// session row is revoked. done must be closed when the read loop exits.
func (s *Server) startWSAuthRecheck(tokenStr string, claims *middleware.Claims, done <-chan struct{}, closeFn func()) {
	if claims == nil || tokenStr == "" || closeFn == nil {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(wsAuthInterval)
		defer ticker.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-done:
				return
			case <-ticker.C:
				if !s.wsSessionStillValid(tokenStr, claims) {
					closeFn()
					return
				}
			}
		}
	}()
}

// wsSessionStillValid is the periodic form of the connect-time auth checks.
func (s *Server) wsSessionStillValid(tokenStr string, claims *middleware.Claims) bool {
	revoked, err := s.isSessionRevoked(tokenStr)
	if err != nil || revoked {
		return false
	}
	return s.wsSessionUserOK(claims)
}
