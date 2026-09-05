package server

import (
	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// tenantIDContextKey caches the resolved tenant ID for the request so repeated
// tenantScope calls (e.g. fan-out detail queries) cost one user lookup total.
const tenantIDContextKey = "forgec2_tenant_id"

// currentTenantID returns the tenant ID for the authenticated operator, or 0 if
// the user is unscoped (legacy / pre-multi-tenant). The result is cached in
// the gin context: tenantScope-heavy handlers previously paid one SELECT per
// call (4-5 per agent-detail page).
func (s *Server) currentTenantID(c *gin.Context) uint {
	if v, ok := c.Get(tenantIDContextKey); ok {
		if tid, ok := v.(uint); ok {
			return tid
		}
	}
	tid := s.lookupTenantID(c)
	c.Set(tenantIDContextKey, tid)
	return tid
}

func (s *Server) lookupTenantID(c *gin.Context) uint {
	if u, ok := c.Get("user"); ok {
		if name, ok := u.(string); ok && name != "" {
			var user db.User
			if err := s.db.Select("tenant_id").Where("username = ?", name).First(&user).Error; err == nil {
				return user.TenantID
			}
		}
	}
	return 0
}

// tenantScope restricts a query to the caller's tenant. A tenant ID of 0
// (legacy / unscoped operators) is intentionally NOT restricted, preserving
// pre-multi-tenant behavior until operators are explicitly assigned tenants.
func (s *Server) tenantScope(query *gorm.DB, c *gin.Context) *gorm.DB {
	if tid := s.currentTenantID(c); tid != 0 {
		return query.Where("tenant_id = ?", tid)
	}
	return query
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
