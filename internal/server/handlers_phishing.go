package server

import (
	"net/http"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleAPIPhishingTemplates(c *gin.Context) {
	var templates []db.PhishingTemplate
	s.db.Order("created_at desc").Find(&templates)
	respond(c, gin.H{"data": templates})
}

func (s *Server) handleAPIPhishingCampaigns(c *gin.Context) {
	var campaigns []db.PhishingCampaign
	s.db.Order("created_at desc").Find(&campaigns)
	respond(c, gin.H{"data": campaigns})
}

func (s *Server) handleAPIPhishingCaptures(c *gin.Context) {
	var events []db.PhishingEvent
	s.db.Where("event_type = ?", "capture").Order("created_at desc").Limit(500).Find(&events)
	type captureEntry struct {
		ID        uint   `json:"id"`
		Username  string `json:"username"`
		Password  string `json:"password"`
		Domain    string `json:"domain"`
		Source    string `json:"source"`
		Type      string `json:"type"`
		CreatedAt string `json:"created_at"`
	}
	captures := make([]captureEntry, 0, len(events))
	for _, e := range events {
		captures = append(captures, captureEntry{
			ID:        e.ID,
			Username:  e.Email,
			Source:    e.IP,
			Type:      e.EventType,
			CreatedAt: e.CreatedAt.Format(time.RFC3339),
		})
	}
	respond(c, gin.H{"data": captures})
}

func (s *Server) handleAPICreatePhishingTemplate(c *gin.Context) {
	var req struct {
		Name      string `json:"name"`
		Subject   string `json:"subject"`
		Body      string `json:"body"`
		FromName  string `json:"from_name"`
		FromEmail string `json:"from_email"`
		Type      string `json:"type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Name == "" {
		respondError(c, http.StatusBadRequest, "name is required")
		return
	}
	if req.Type == "" {
		req.Type = "html"
	}
	tpl := db.PhishingTemplate{
		Name:      req.Name,
		Subject:   req.Subject,
		Body:      req.Body,
		FromName:  req.FromName,
		FromEmail: req.FromEmail,
		Type:      req.Type,
		CreatedBy: s.currentUsername(c),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := s.db.Create(&tpl).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Phishing operation"))
		return
	}
	respond(c, gin.H{"success": true, "id": tpl.ID})
}

func (s *Server) handleAPIUpdatePhishingTemplate(c *gin.Context) {
	id := c.Param("id")
	var tpl db.PhishingTemplate
	if err := s.db.First(&tpl, "id = ?", id).Error; err != nil {
		respondError(c, http.StatusNotFound, "template not found")
		return
	}
	var req struct {
		Name      string `json:"name"`
		Subject   string `json:"subject"`
		Body      string `json:"body"`
		FromName  string `json:"from_name"`
		FromEmail string `json:"from_email"`
		Type      string `json:"type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Name != "" {
		tpl.Name = req.Name
	}
	if req.Subject != "" {
		tpl.Subject = req.Subject
	}
	if req.Body != "" {
		tpl.Body = req.Body
	}
	if req.FromName != "" {
		tpl.FromName = req.FromName
	}
	if req.FromEmail != "" {
		tpl.FromEmail = req.FromEmail
	}
	if req.Type != "" {
		tpl.Type = req.Type
	}
	tpl.UpdatedAt = time.Now()
	if err := s.db.Save(&tpl).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to update phishing template")
		return
	}
	respond(c, gin.H{"success": true})
}

func (s *Server) handleAPIDeletePhishingTemplate(c *gin.Context) {
	id := c.Param("id")
	if err := s.db.Delete(&db.PhishingTemplate{}, "id = ?", id).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to delete phishing template")
		return
	}
	s.LogAuditRecord(c, "delete_phishing_template", "phishing_template", id, "Phishing template deleted", true, nil)
	respond(c, gin.H{"success": true})
}

func (s *Server) handleAPICreatePhishingCampaign(c *gin.Context) {
	var req struct {
		Name       string `json:"name"`
		TemplateID uint   `json:"template_id"`
		TargetList string `json:"target_list"`
		SMTPHost   string `json:"smtp_host"`
		SMTPPort   int    `json:"smtp_port"`
		SMTPUser   string `json:"smtp_user"`
		SMTPPass   string `json:"smtp_pass"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Name == "" {
		respondError(c, http.StatusBadRequest, "name is required")
		return
	}
	if req.SMTPPort == 0 {
		req.SMTPPort = 587
	}
	encryptedPass, err := encryptSecret(req.SMTPPass, s.cfg.Server.JWTSecret)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to encrypt SMTP password")
		return
	}
	camp := db.PhishingCampaign{
		Name:       req.Name,
		TemplateID: req.TemplateID,
		TargetList: req.TargetList,
		SMTPHost:   req.SMTPHost,
		SMTPPort:   req.SMTPPort,
		SMTPUser:   req.SMTPUser,
		SMTPPass:   encryptedPass,
		Status:     "draft",
		CreatedBy:  s.currentUsername(c),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := s.db.Create(&camp).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Phishing operation"))
		return
	}
	respond(c, gin.H{"success": true, "id": camp.ID})
}

func (s *Server) handleAPILaunchPhishingCampaign(c *gin.Context) {
	id := c.Param("id")
	var camp db.PhishingCampaign
	if err := s.db.First(&camp, "id = ?", id).Error; err != nil {
		respondError(c, http.StatusNotFound, "campaign not found")
		return
	}
	if camp.Status == "running" {
		respondError(c, http.StatusBadRequest, "campaign is already running")
		return
	}
	camp.Status = "running"
	camp.SentCount = 0
	camp.OpenCount = 0
	camp.CredCount = 0
	camp.UpdatedAt = time.Now()
	if err := s.db.Save(&camp).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Phishing operation"))
		return
	}
	respond(c, gin.H{"success": true})
}

func (s *Server) handleAPIStopPhishingCampaign(c *gin.Context) {
	id := c.Param("id")
	var camp db.PhishingCampaign
	if err := s.db.First(&camp, "id = ?", id).Error; err != nil {
		respondError(c, http.StatusNotFound, "campaign not found")
		return
	}
	camp.Status = "completed"
	camp.UpdatedAt = time.Now()
	if err := s.db.Save(&camp).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Phishing operation"))
		return
	}
	respond(c, gin.H{"success": true})
}

func (s *Server) handleAPIDeletePhishingCampaign(c *gin.Context) {
	id := c.Param("id")
	if err := s.db.Delete(&db.PhishingCampaign{}, "id = ?", id).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Phishing operation"))
		return
	}
	s.LogAuditRecord(c, "delete_phishing_campaign", "phishing_campaign", id, "Phishing campaign deleted", true, nil)
	respond(c, gin.H{"success": true})
}
