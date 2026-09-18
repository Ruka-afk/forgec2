package server

import (
	"net/http"
	"strings"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/server/totp"
	"github.com/gin-gonic/gin"
)

const (
	// exportTOTPHeader is the preferred transport for the step-up code
	// (keeps one-time codes out of access logs).
	exportTOTPHeader = "X-TOTP-Code"
	// exportTOTPQuery is the fallback transport for plain anchor/GET
	// downloads. TOTP codes are single-use and 30s-boxed, so a logged
	// code is already dead by the time anyone reads it.
	exportTOTPQuery = "totp_code"
)

// requireExportStepUp enforces step-up authentication for bulk secret
// exports (credential / task / handover / report bundles). Operators with
// TOTP enrolled must present a fresh code; everyone else passes with the
// caller's own audit row. API-key callers are exempt — they are
// non-interactive (key scoping is tracked separately) — but stay
// audit-logged by the export handler itself.
//
// Returns true when the export may proceed. On false the handler must
// return immediately (response already written).
func (s *Server) requireExportStepUp(c *gin.Context, action, resource string) bool {
	if c.GetBool("auth_via_api_key") {
		// Non-interactive callers cannot do TOTP step-up: bulk export
		// requires an explicit bulk_export scope on the key instead.
		// Legacy full-access keys (nil set) keep working; scoped keys
		// without it are denied here even if they hold the read perm.
		if v, ok := c.Get("api_key_scopes"); ok {
			if set, ok := v.(map[string]bool); ok && set != nil && !set[db.PermBulkExport] {
				s.LogAuditRecord(c, action, resource, "", "export blocked: API key lacks bulk_export scope", false, nil)
				c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "API key lacks bulk_export scope"})
				return false
			}
		}
		return true
	}
	username := s.currentUsername(c)
	var user db.User
	if username == "" {
		return true
	}
	if err := s.db.Select("totp_secret").Where("username = ?", username).First(&user).Error; err != nil {
		return true
	}
	if user.TOTPSecret == "" {
		return true
	}
	code := strings.TrimSpace(c.GetHeader(exportTOTPHeader))
	if code == "" {
		code = strings.TrimSpace(c.Query(exportTOTPQuery))
	}
	if code == "" {
		s.LogAuditRecord(c, action, resource, "", "export blocked: TOTP step-up required", false, nil)
		// 403, not 401: the operator IS authenticated, only the step-up
		// factor is missing. The frontend treats any 401 as session expiry
		// and redirects to /login — a step-up challenge must never do that.
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "TOTP verification required for export", "totp_required": true})
		return false
	}
	secret, err := decryptSecret(user.TOTPSecret, s.cfg.Crypto.TotpKey)
	if err != nil || !totp.VerifyCode(secret, code) {
		s.LogAuditRecord(c, action, resource, "", "export blocked: invalid TOTP code", false, nil)
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "invalid TOTP code", "totp_required": true})
		return false
	}
	return true
}
