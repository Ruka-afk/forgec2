package server

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// handleCampaignsList returns all campaign-wizard campaigns.
// GET /campaigns
func (s *Server) handleCampaignsList(c *gin.Context) {
	var campaigns []db.Campaign
	if err := s.db.Preload("Agents").Order("created_at desc").Limit(200).Find(&campaigns).Error; err != nil {
		slog.Error("Failed to list campaigns", "err", err)
	}
	respond(c, gin.H{"success": true, "data": campaigns})
}

// handleCampaignGet returns a single campaign with aggregated stats.
// GET /campaigns/:id
func (s *Server) handleCampaignGet(c *gin.Context) {
	id := c.Param("id")
	var campaign db.Campaign
	if !s.findOrFailPreload(c, &campaign, id, "campaign", "Agents") {
		return
	}

	agentIDs := make([]string, 0, len(campaign.Agents))
	for _, a := range campaign.Agents {
		agentIDs = append(agentIDs, a.ID)
	}

	var tasks []db.Task
	if len(agentIDs) > 0 {
		if err := s.db.Where("agent_id IN ?", agentIDs).Limit(CampaignTaskLimit).Find(&tasks).Error; err != nil {
			slog.Error("Failed to query campaign tasks", "err", err)
		}
	}

	stats := computeCampaignStats(campaign.Agents, tasks)
	respond(c, gin.H{"campaign": campaign, "stats": stats})
}

// handleCampaignCreate creates a new campaign.
// POST /api/v1/campaigns
func (s *Server) handleCampaignCreate(c *gin.Context) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, sanitizeError(err, "Campaign operation"))
		return
	}
	if req.Name == "" {
		respondError(c, http.StatusBadRequest, "name required")
		return
	}

	campaign := db.Campaign{
		ID:          uuid.NewString(),
		Name:        req.Name,
		Description: req.Description,
		Status:      "active",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := s.db.Create(&campaign).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create campaign")
		return
	}
	respond(c, gin.H{"success": true, "id": campaign.ID, "campaign": campaign})
}

// handleCampaignUpdate updates a campaign (e.g. status).
// POST /v1/campaigns/:id
func (s *Server) handleCampaignUpdate(c *gin.Context) {
	id := c.Param("id")
	var campaign db.Campaign
	if !s.findOrFail(c, &campaign, id, "campaign") {
		return
	}
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Status      string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, sanitizeError(err, "Campaign operation"))
		return
	}
	if req.Name != "" {
		campaign.Name = req.Name
	}
	if req.Description != "" {
		campaign.Description = req.Description
	}
	if req.Status != "" {
		campaign.Status = req.Status
	}
	campaign.UpdatedAt = time.Now()
	if err := s.db.Save(&campaign).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to update campaign")
		return
	}
	respond(c, gin.H{"success": true, "campaign": campaign})
}

// handleCampaignDelete deletes a campaign and its agent associations.
// DELETE /v1/campaigns/:id
func (s *Server) handleCampaignDelete(c *gin.Context) {
	id := c.Param("id")
	if err := s.db.Delete(&db.Campaign{}, "id = ?", id).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Campaign operation"))
		return
	}
	respond(c, gin.H{"success": true})
}

// handleCampaignMitre returns the MITRE ATT&CK status for a campaign.
// GET /campaigns/:id/mitre
func (s *Server) handleCampaignMitre(c *gin.Context) {
	id := c.Param("id")
	var campaign db.Campaign
	if !s.findOrFailPreload(c, &campaign, id, "campaign", "Agents") {
		return
	}

	agentIDs := make([]string, 0, len(campaign.Agents))
	for _, a := range campaign.Agents {
		agentIDs = append(agentIDs, a.ID)
	}
	var tasks []db.Task
	if len(agentIDs) > 0 {
		if err := s.db.Where("agent_id IN ?", agentIDs).Limit(CampaignTaskLimit).Find(&tasks).Error; err != nil {
			slog.Error("Failed to query campaign MITRE tasks", "err", err)
		}
	}

	phaseCounts := map[string]int{}
	for _, t := range tasks {
		phaseCounts[taskPhase(t.Type)]++
	}

	phases := make([]map[string]interface{}, 0, len(PHASE_ORDER))
	for _, p := range PHASE_ORDER {
		status := "pending"
		if phaseCounts[p] > 0 {
			status = "completed"
		}
		phases = append(phases, map[string]interface{}{
			"phase":      p,
			"status":     status,
			"task_count": phaseCounts[p],
		})
	}

	respond(c, gin.H{
		"success": true,
		"data": map[string]interface{}{
			"campaign_id":   campaign.ID,
			"campaign_name": campaign.Name,
			"phases":        phases,
			"timeline":      buildPhaseTimeline(tasks),
		},
	})
}

// handleCampaignKillChain seeds a campaign's kill chain from a template.
// POST /v1/campaigns/:id/killchain
func (s *Server) handleCampaignKillChain(c *gin.Context) {
	id := c.Param("id")
	var campaign db.Campaign
	if !s.findOrFail(c, &campaign, id, "campaign") {
		return
	}
	// Templates are advisory; we simply acknowledge acceptance.
	respond(c, gin.H{"success": true, "campaign_id": id})
}

// handleMitreTemplates returns the built-in kill chain templates.
// GET /mitre/templates
func (s *Server) handleMitreTemplates(c *gin.Context) {
	templates := []map[string]interface{}{
		{
			"name":        "Standard Intrusion",
			"description": "Recon through impact kill chain",
			"steps": []map[string]interface{}{
				{"phase": "Reconnaissance", "task_type": "ps", "params": map[string]string{}, "wait_time": 0},
				{"phase": "Initial Access", "task_type": "shell", "params": map[string]string{}, "wait_time": 30},
				{"phase": "Execution", "task_type": "shell", "params": map[string]string{}, "wait_time": 0},
				{"phase": "Persistence", "task_type": "reg_add", "params": map[string]string{}, "wait_time": 0},
				{"phase": "Credential Access", "task_type": "keylogger_start", "params": map[string]string{}, "wait_time": 0},
				{"phase": "Lateral Movement", "task_type": "shell", "params": map[string]string{}, "wait_time": 60},
				{"phase": "Impact", "task_type": "shell", "params": map[string]string{}, "wait_time": 0},
			},
		},
		{
			"name":        "Stealth Operations",
			"description": "Low-and-slow discovery focused chain",
			"steps": []map[string]interface{}{
				{"phase": "Discovery", "task_type": "ps", "params": map[string]string{}, "wait_time": 120},
				{"phase": "Discovery", "task_type": "ls", "params": map[string]string{}, "wait_time": 120},
				{"phase": "Defense Evasion", "task_type": "shell", "params": map[string]string{}, "wait_time": 60},
				{"phase": "Command and Control", "task_type": "shell", "params": map[string]string{}, "wait_time": 0},
			},
		},
	}
	respond(c, gin.H{"success": true, "data": templates})
}

// handleMitreTimeline returns the phase timeline for a campaign (or all tasks).
// GET /mitre/timeline?campaign_id=
func (s *Server) handleMitreTimeline(c *gin.Context) {
	campaignID := c.Query("campaign_id")

	var agentIDs []string
	if campaignID != "" {
		var campaign db.Campaign
		if err := s.db.Preload("Agents").First(&campaign, "id = ?", campaignID).Error; err == nil {
			for _, a := range campaign.Agents {
				agentIDs = append(agentIDs, a.ID)
			}
		}
	}

	var tasks []db.Task
	if len(agentIDs) > 0 {
		if err := s.db.Where("agent_id IN ?", agentIDs).Order("created_at asc").Limit(CampaignTaskLimit).Find(&tasks).Error; err != nil {
			slog.Error("Failed to query campaign timeline tasks", "err", err)
		}
	} else {
		if err := s.db.Order("created_at asc").Limit(MITRETimelineLimit).Find(&tasks).Error; err != nil {
			slog.Error("Failed to query MITRE timeline tasks", "err", err)
		}
	}

	respond(c, gin.H{"success": true, "data": buildPhaseTimeline(tasks)})
}

func agentIDList(agents []db.Implant) []string {
	ids := make([]string, 0, len(agents))
	for _, a := range agents {
		ids = append(ids, a.ID)
	}
	return ids
}

func computeCampaignStats(agents []db.Implant, tasks []db.Task) map[string]interface{} {
	totalTasks := len(tasks)
	completed := 0
	failed := 0
	phaseCounts := map[string]int{}
	phaseFirstSeen := map[string]time.Time{}

	for _, t := range tasks {
		switch t.Status {
		case "completed":
			completed++
		case "failed":
			failed++
		}
		p := taskPhase(t.Type)
		phaseCounts[p]++
		if ft, ok := phaseFirstSeen[p]; !ok || t.CreatedAt.Before(ft) {
			phaseFirstSeen[p] = t.CreatedAt
		}
	}

	agentBreakdown := make([]map[string]interface{}, 0, len(agents))
	byAgent := map[string][]db.Task{}
	for _, t := range tasks {
		byAgent[t.AgentID] = append(byAgent[t.AgentID], t)
	}
	for _, a := range agents {
		agentTasks := byAgent[a.ID]
		apc := map[string]int{}
		for _, t := range agentTasks {
			apc[taskPhase(t.Type)]++
		}
		agentBreakdown = append(agentBreakdown, map[string]interface{}{
			"agent_id":   a.ID,
			"hostname":   a.Hostname,
			"username":   a.Username,
			"ip":         a.IP,
			"task_count": len(agentTasks),
			"phases":     apc,
		})
	}

	return map[string]interface{}{
		"total_agents":       len(agents),
		"total_tasks":        totalTasks,
		"completed_tasks":    completed,
		"failed_tasks":       failed,
		"kill_chain_summary": phaseCounts,
		"phase_timeline":     buildPhaseTimeline(tasks),
		"agent_breakdown":    agentBreakdown,
	}
}

func buildPhaseTimeline(tasks []db.Task) []map[string]interface{} {
	firstSeen := map[string]time.Time{}
	counts := map[string]int{}
	for _, t := range tasks {
		p := taskPhase(t.Type)
		counts[p]++
		if ft, ok := firstSeen[p]; !ok || t.CreatedAt.Before(ft) {
			firstSeen[p] = t.CreatedAt
		}
	}
	out := make([]map[string]interface{}, 0, len(firstSeen))
	for p, c := range counts {
		fs := firstSeen[p]
		out = append(out, map[string]interface{}{
			"phase":      p,
			"first_seen": fs.Format(time.RFC3339),
			"task_count": c,
		})
	}
	return out
}

// PHASE_ORDER mirrors the MITRE ATT&CK kill chain used by the frontend.
var PHASE_ORDER = []string{
	"Reconnaissance", "Resource Development", "Initial Access", "Execution",
	"Persistence", "Privilege Escalation", "Defense Evasion", "Credential Access",
	"Discovery", "Lateral Movement", "Collection", "Command and Control",
	"Exfiltration", "Impact",
}

// taskPhase maps a ForgeC2 task type to a MITRE ATT&CK phase.
func taskPhase(taskType string) string {
	t := strings.ToLower(taskType)
	switch {
	case strings.HasPrefix(t, "keylogger"), strings.HasPrefix(t, "clipboard"), t == "screenshot", t == "screen_stream_start", t == "screen_stream_stop":
		return "Collection"
	case strings.HasPrefix(t, "reg_"), t == "elevate", strings.HasPrefix(t, "persist"), t == "schtasks_add":
		return "Persistence"
	case strings.HasPrefix(t, "inject"), t == "shell", strings.HasPrefix(t, "exec"), t == "run":
		return "Execution"
	case strings.HasPrefix(t, "lsass"), strings.HasPrefix(t, "creds"), t == "mimikatz", t == "wdigest", t == "lsa_dump":
		return "Credential Access"
	case t == "ps", t == "ls", t == "find", t == "net", t == "whoami", t == "ipconfig", t == "arp", strings.HasPrefix(t, "discover"):
		return "Discovery"
	case strings.HasPrefix(t, "pivot"), strings.HasPrefix(t, "wmi"), strings.HasPrefix(t, "psexec"), strings.HasPrefix(t, "winrm"), strings.HasPrefix(t, "ssh"):
		return "Lateral Movement"
	case t == "upload", strings.HasPrefix(t, "exfil"), t == "download":
		return "Exfiltration"
	case t == "suspend", t == "resume", t == "killproc", t == "kill":
		return "Defense Evasion"
	case strings.HasPrefix(t, "beacon"), t == "sleep", t == "jitter", t == "profile_rotate":
		return "Command and Control"
	case t == "recon", strings.HasPrefix(t, "portscan"), strings.HasPrefix(t, "scan"):
		return "Reconnaissance"
	default:
		return "Execution"
	}
}
