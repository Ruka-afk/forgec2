package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type scheduledReportRequest struct {
	Name          string `json:"name"`
	Schedule      string `json:"schedule"`
	Format        string `json:"format"`
	IncludeAgents bool   `json:"include_agents"`
	IncludeTasks  bool   `json:"include_tasks"`
	IncludeCreds  bool   `json:"include_creds"`
	IncludeAudit  bool   `json:"include_audit"`
	DeliveryType  string `json:"delivery_type"`
	DeliveryTo    string `json:"delivery_to"`
	Enabled       bool   `json:"enabled"`
}

// handleScheduledReportList lists scheduled report jobs.
func (s *Server) handleScheduledReportList(c *gin.Context) {
	var reports []db.ScheduledReport
	s.listAll(c, &reports, "created_at desc")
}

// handleScheduledReportCreate adds a new scheduled report.
func (s *Server) handleScheduledReportCreate(c *gin.Context) {
	var req scheduledReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid body")
		return
	}
	if req.Name == "" || req.Schedule == "" {
		respondError(c, http.StatusBadRequest, "name and schedule required")
		return
	}
	if req.Format == "" {
		req.Format = "html"
	}

	now := time.Now()
	rep := db.ScheduledReport{
		ID:            uuid.New().String(),
		Name:          req.Name,
		Enabled:       req.Enabled,
		Schedule:      req.Schedule,
		Format:        req.Format,
		IncludeAgents: req.IncludeAgents,
		IncludeTasks:  req.IncludeTasks,
		IncludeCreds:  req.IncludeCreds,
		IncludeAudit:  req.IncludeAudit,
		DeliveryType:  req.DeliveryType,
		DeliveryTo:    req.DeliveryTo,
		NextRun:       now,
		CreatedBy:     s.currentUsername(c),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.db.Create(&rep).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Scheduled report create"))
		return
	}
	respond(c, gin.H{"success": true, "id": rep.ID})
}

// handleScheduledReportUpdate edits a scheduled report.
func (s *Server) handleScheduledReportUpdate(c *gin.Context) {
	id := c.Param("id")
	var rep db.ScheduledReport
	if !s.findOrFail(c, &rep, id, "report") {
		return
	}
	var req scheduledReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid body")
		return
	}
	if req.Name != "" {
		rep.Name = req.Name
	}
	if req.Schedule != "" {
		rep.Schedule = req.Schedule
	}
	if req.Format != "" {
		rep.Format = req.Format
	}
	if req.DeliveryType != "" {
		rep.DeliveryType = req.DeliveryType
	}
	if req.DeliveryTo != "" {
		rep.DeliveryTo = req.DeliveryTo
	}
	rep.Enabled = req.Enabled
	rep.IncludeAgents = req.IncludeAgents
	rep.IncludeTasks = req.IncludeTasks
	rep.IncludeCreds = req.IncludeCreds
	rep.IncludeAudit = req.IncludeAudit
	rep.UpdatedAt = time.Now()
	if err := s.db.Save(&rep).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to update report")
		return
	}
	respond(c, gin.H{"success": true})
}

// handleScheduledReportDelete removes a scheduled report.
func (s *Server) handleScheduledReportDelete(c *gin.Context) {
	id := c.Param("id")
	if err := s.db.Delete(&db.ScheduledReport{}, "id = ?", id).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Scheduled report delete"))
		return
	}
	respond(c, gin.H{"success": true})
}

// handleScheduledReportToggle toggles the enabled state of a scheduled report.
func (s *Server) handleScheduledReportToggle(c *gin.Context) {
	id := c.Param("id")
	var rep db.ScheduledReport
	if !s.findOrFail(c, &rep, id, "report") {
		return
	}
	rep.Enabled = !rep.Enabled
	rep.UpdatedAt = time.Now()
	if err := s.db.Save(&rep).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to toggle report")
		return
	}
	s.LogAuditRecord(c, "scheduled_report_toggle", "scheduled_report", id,
		fmt.Sprintf("Scheduled report %s toggled to %v", rep.Name, rep.Enabled), true, nil)
	respond(c, gin.H{"success": true, "enabled": rep.Enabled})
}
