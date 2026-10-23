package server

import (
	"net/http"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// requireExportScope enforces the API-key scope gate for bulk secret exports
// (credential / task / handover / report bundles). Interactive session callers
// pass; non-interactive API-key callers must hold the bulk_export scope.
//
// Returns true when the export may proceed. On false the handler must
// return immediately (response already written).
func (s *Server) requireExportScope(c *gin.Context, action, resource string) bool {
	if c.GetBool("auth_via_api_key") {
		// Non-interactive callers cannot answer an interactive challenge:
		// bulk export requires an explicit bulk_export scope on the key
		// instead. Legacy full-access keys (nil set) keep working; scoped
		// keys without it are denied here even if they hold the read perm.
		if v, ok := c.Get("api_key_scopes"); ok {
			if set, ok := v.(map[string]bool); ok && set != nil && !set[db.PermBulkExport] {
				s.LogAuditRecord(c, action, resource, "", "export blocked: API key lacks bulk_export scope", false, nil)
				c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "API key lacks bulk_export scope"})
				return false
			}
		}
		return true
	}
	return true
}
