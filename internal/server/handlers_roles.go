package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

type roleRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

func parsePermissions(raw string) []string {
	if raw == "" {
		return []string{}
	}
	var perms []string
	if err := json.Unmarshal([]byte(raw), &perms); err != nil {
		return []string{}
	}
	return perms
}

func marshalPermissions(perms []string) string {
	b, _ := json.Marshal(perms)
	return string(b)
}

// handleRolesList returns all custom roles.
// GET /api/roles
func (s *Server) handleRolesList(c *gin.Context) {
	var roles []db.CustomRole
	s.db.Order("created_at desc").Find(&roles)

	type roleResp struct {
		ID          uint     `json:"id"`
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Permissions []string `json:"permissions"`
		CreatedAt   string   `json:"created_at"`
		UpdatedAt   string   `json:"updated_at"`
	}
	out := make([]roleResp, 0, len(roles))
	for _, r := range roles {
		out = append(out, roleResp{
			ID:          r.ID,
			Name:        r.Name,
			Description: r.Description,
			Permissions: parsePermissions(r.Permissions),
			CreatedAt:   r.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   r.UpdatedAt.Format(time.RFC3339),
		})
	}
	respond(c, gin.H{"success": true, "data": out})
}

// handleRolesCreate creates a new custom role (admin only).
// POST /api/roles
func (s *Server) handleRolesCreate(c *gin.Context) {
	if !s.requireAdmin(c) { return }
	var req roleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if req.Name == "" {
		respondError(c, http.StatusBadRequest, "name required")
		return
	}
	newRole := db.CustomRole{
		Name:        req.Name,
		Description: req.Description,
		Permissions: marshalPermissions(req.Permissions),
	}
	if err := s.db.Create(&newRole).Error; err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	respond(c, gin.H{"success": true, "data": newRole})
}

// handleRolesUpdate updates an existing custom role (admin only).
// POST /api/roles/:id
func (s *Server) handleRolesUpdate(c *gin.Context) {
	if !s.requireAdmin(c) { return }
	id := c.Param("id")
	var existingRole db.CustomRole
	if !s.findOrFail(c, &existingRole, id, "role") {
		return
	}
	var req roleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if req.Name != "" {
		existingRole.Name = req.Name
	}
	existingRole.Description = req.Description
	existingRole.Permissions = marshalPermissions(req.Permissions)
	if err := s.db.Save(&existingRole).Error; err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	respond(c, gin.H{"success": true, "data": existingRole})
}

// handleRolesDelete deletes a custom role (admin only, cannot delete builtin roles).
// DELETE /api/roles/:id
func (s *Server) handleRolesDelete(c *gin.Context) {
	if !s.requireAdmin(c) { return }
	id := c.Param("id")
	// Prevent deletion of builtin roles by name
	var r db.CustomRole
	if err := s.db.First(&r, "id = ?", id).Error; err == nil {
		if r.Name == "admin" || r.Name == "user" {
			respondError(c, http.StatusBadRequest, "cannot delete builtin role")
			return
		}
	}
	if err := s.db.Delete(&db.CustomRole{}, "id = ?", id).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Role"))
		return
	}
	respond(c, gin.H{"success": true})
}
