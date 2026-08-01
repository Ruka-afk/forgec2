package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// handleOpsecHistory returns the OPSEC evaluation history.
// GET /opsec/history
func (s *Server) handleOpsecHistory(c *gin.Context) {
	p := parsePagination(c, 50, 200)
	var total int64
	if err := s.db.Model(&db.OpsecHistory{}).Count(&total).Error; err != nil {
		slog.Error("Failed to count OPSEC history", "err", err)
	}
	var history []db.OpsecHistory
	if err := s.db.Order("created_at desc").Offset(p.Offset).Limit(p.PageSize).Find(&history).Error; err != nil {
		slog.Error("Failed to query OPSEC history", "err", err)
	}
	respond(c, gin.H{"history": history, "total": total, "page": p.Page, "page_size": p.PageSize})
}

// handleOpsecRuleCreate persists a new OPSEC rule and injects it into the engine.
// POST /opsec/rules
func (s *Server) handleOpsecRuleCreate(c *gin.Context) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		RiskLevel   int    `json:"risk_level"`
		Action      int    `json:"default_action"`
		CheckType   string `json:"check_type"`
		TaskTypes   string `json:"task_types"`
		Enabled     *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErrorSafe(c, http.StatusBadRequest, err, "")
		return
	}

	if req.Name == "" {
		respondError(c, http.StatusBadRequest, "name is required")
		return
	}
	if req.RiskLevel < 1 || req.RiskLevel > 4 {
		respondError(c, http.StatusBadRequest, "risk_level must be 1-4")
		return
	}
	if req.Action < 0 || req.Action > 2 {
		respondError(c, http.StatusBadRequest, "default_action must be 0 (block), 1 (warn), or 2 (bypass)")
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	rule := db.OpsecRule{
		Name:          req.Name,
		Description:   req.Description,
		RiskLevel:     req.RiskLevel,
		DefaultAction: req.Action,
		CheckType:     req.CheckType,
		TaskTypes:     req.TaskTypes,
		Enabled:       enabled,
		CreatedBy:     s.currentUsername(c),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Check for duplicate name
	var existing db.OpsecRule
	if s.db.Where("name = ?", req.Name).First(&existing).Error == nil {
		respondError(c, http.StatusConflict, "rule with this name already exists")
		return
	}

	if err := s.db.Create(&rule).Error; err != nil {
		slog.Error("Failed to create OPSEC rule", "name", req.Name, "err", err)
		respondError(c, http.StatusInternalServerError, "failed to create rule")
		return
	}

	s.LogAuditRecord(c, "opsec_rule_create", "opsec_rule", req.Name, fmt.Sprintf("Created OPSEC rule: %s (risk=%d, action=%d)", req.Name, req.RiskLevel, req.Action), true, nil)
	respond(c, gin.H{"success": true, "rule": rule})
}

// handleOpsecRuleDelete removes an OPSEC rule by name.
// DELETE /opsec/rules/:name
func (s *Server) handleOpsecRuleDelete(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		respondError(c, http.StatusBadRequest, "rule name is required")
		return
	}

	result := s.db.Where("name = ?", name).Delete(&db.OpsecRule{})
	if result.Error != nil {
		slog.Error("Failed to delete OPSEC rule", "name", name, "err", result.Error)
		respondError(c, http.StatusInternalServerError, "failed to delete rule")
		return
	}
	if result.RowsAffected == 0 {
		respondError(c, http.StatusNotFound, "rule not found")
		return
	}

	s.LogAuditRecord(c, "opsec_rule_delete", "opsec_rule", name, fmt.Sprintf("Deleted OPSEC rule: %s", name), true, nil)
	respond(c, gin.H{"success": true})
}

// handleOpsecRulesList returns all persisted OPSEC rules.
// GET /api/opsec/rules (or /opsec/rules)
func (s *Server) handleOpsecRulesList(c *gin.Context) {
	var rules []db.OpsecRule
	if err := s.db.Order("risk_level desc, name").Limit(200).Find(&rules).Error; err != nil {
		slog.Error("Failed to list OPSEC rules", "err", err)
	}
	respond(c, gin.H{"rules": rules})
}
