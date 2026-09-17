package server

import (
	"errors"
	"log/slog"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// tenantIDContextKey caches the resolved tenant ID for the request so repeated
// tenantScope calls (e.g. fan-out detail queries) cost one user lookup total.
const tenantIDContextKey = "forgec2_tenant_id"

// tenantResolution is the cached tenant lookup: tid 0 stays legacy-unscoped,
// while ok=false marks a failed lookup (fail closed, never fail open).
type tenantResolution struct {
	tid uint
	ok  bool
}

// currentTenantID returns the tenant ID for the authenticated operator, or 0 if
// the user is unscoped (legacy / pre-multi-tenant). The result is cached in
// the gin context: tenantScope-heavy handlers previously paid one SELECT per
// call (4-5 per agent-detail page).
func (s *Server) currentTenantID(c *gin.Context) uint {
	tid, _ := s.resolveTenant(c)
	return tid
}

func (s *Server) resolveTenant(c *gin.Context) (uint, bool) {
	if v, ok := c.Get(tenantIDContextKey); ok {
		if r, ok := v.(tenantResolution); ok {
			return r.tid, r.ok
		}
	}
	tid, ok := s.lookupTenantID(c)
	c.Set(tenantIDContextKey, tenantResolution{tid: tid, ok: ok})
	return tid, ok
}

func (s *Server) lookupTenantID(c *gin.Context) (uint, bool) {
	if u, ok := c.Get("user"); ok {
		if name, ok := u.(string); ok && name != "" {
			var user db.User
			err := s.db.Select("tenant_id").Where("username = ?", name).First(&user).Error
			if err == nil {
				return user.TenantID, true
			}
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// Named identity with no user row (hand-built test contexts;
				// production auth always loads the row first): legacy.
				return 0, true
			}
			// Authenticated identity whose lookup errored (DB hiccup): fail
			// closed. Returning legacy 0 here silently expanded a broken
			// lookup into global visibility.
			slog.Warn("Tenant lookup failed, denying scoped query", "user", name, "error", err)
			return 0, false
		}
	}
	// No identity in context (tests, system paths): legacy-unscoped.
	return 0, true
}

// tenantScope restricts a query to the caller's tenant. A tenant ID of 0
// (legacy / unscoped operators) is intentionally NOT restricted, preserving
// pre-multi-tenant behavior until operators are explicitly assigned tenants.
// A FAILED lookup denies everything (Where 1=0) instead of degrading to
// global visibility.
func (s *Server) tenantScope(query *gorm.DB, c *gin.Context) *gorm.DB {
	tid, ok := s.resolveTenant(c)
	if !ok {
		return query.Where("1 = 0")
	}
	if tid != 0 {
		return query.Where("tenant_id = ?", tid)
	}
	return query
}

// tenantIDForUser resolves a username to its tenant ID outside a gin
// context (WebSocket registration). Missing rows and lookup errors both
// yield legacy 0 — the socket layer cannot fail closed here without
// breaking pre-tenant operators, and per-agent fan-out still applies.
func (s *Server) tenantIDForUser(username string) uint {
	var user db.User
	if err := s.db.Select("tenant_id").Where("username = ?", username).First(&user).Error; err != nil {
		return 0
	}
	return user.TenantID
}

// resolveVisibleAgentID resolves an id-or-hostname within the caller's tenant
// scope. The shared resolveAgentID stays unscoped for system paths (beacons,
// sweeps); interactive handlers must use this so one tenant's operator cannot
// pull another tenant's agent profile into AI prompts and pages.
func (s *Server) resolveVisibleAgentID(c *gin.Context, idOrHost string) string {
	var agent db.Implant
	if err := s.tenantScope(s.db, c).Where("id = ? OR hostname = ?", idOrHost, idOrHost).First(&agent).Error; err != nil {
		return ""
	}
	return agent.ID
}

// assignTenantToAgent sets the tenant on a newly created/registered implant so
// it is owned by the calling operator's tenant.
func (s *Server) assignTenantToAgent(c *gin.Context, agentID string) {
	if tid := s.currentTenantID(c); tid != 0 {
		s.db.Model(&db.Implant{}).Where("id = ?", agentID).Update("tenant_id", tid)
	}
}

// defaultTenantID returns the ID of the bootstrap "default" tenant that all
// legacy/freshly-registered assets are assigned to.
func (s *Server) defaultTenantID() uint {
	var t db.Tenant
	if err := s.db.Where("name = ?", db.DefaultTenantName).First(&t).Error; err == nil {
		return t.ID
	}
	return 0
}
