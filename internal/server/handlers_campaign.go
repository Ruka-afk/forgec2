package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/util"
	"github.com/forgec2/forgec2/pkg/protocol"
	"github.com/gin-gonic/gin"
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
		ID:          util.NewString(),
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
	phaseDone := map[string]int{}
	phaseFailed := map[string]int{}
	for _, t := range tasks {
		p := taskPhase(t.Type)
		phaseCounts[p]++
		switch t.Status {
		case "completed":
			phaseDone[p]++
		case "failed":
			phaseFailed[p]++
		}
	}

	phases := make([]map[string]interface{}, 0, len(PHASE_ORDER))
	for _, p := range PHASE_ORDER {
		// A phase is only "completed" when a task of that phase actually
		// completed; a phase with only failed/pending tasks must not be
		// advertised as done.
		status := "pending"
		if phaseDone[p] > 0 {
			status = "completed"
		} else if phaseCounts[p] > 0 && phaseFailed[p] == phaseCounts[p] {
			status = "failed"
		} else if phaseCounts[p] > 0 {
			status = "in_progress"
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

// ── Kill chain templates ────────────────────────────────────────────────────
// Built-in templates are the source of truth for both the template listing and
// the kill-chain seeding endpoint. Every task_type referenced by a template is
// a real agent task (validated at seed time), and the phase label is advisory:
// after seeding, the campaign timeline derives phases from the actual task
// types via taskPhase().

type killChainStep struct {
	Phase    string            `json:"phase"`
	TaskType string            `json:"task_type"`
	Params   map[string]string `json:"params"`
	WaitTime int               `json:"wait_time"`
}

type killChainTemplate struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Steps       []killChainStep `json:"steps"`
}

func builtinKillChainTemplates() []killChainTemplate {
	return []killChainTemplate{
		{
			Name:        "Standard Intrusion",
			Description: "Recon through impact kill chain (uses safe default steps; replace with your own operations where offensive action is intended).",
			Steps: []killChainStep{
				{Phase: "Reconnaissance", TaskType: "ps", WaitTime: 0},
				{Phase: "Initial Access", TaskType: "shell", Params: map[string]string{"command": "whoami"}, WaitTime: 30},
				{Phase: "Execution", TaskType: "shell", Params: map[string]string{"command": "whoami"}, WaitTime: 0},
				{Phase: "Persistence", TaskType: "persistence_list", WaitTime: 0},
				{Phase: "Credential Access", TaskType: "keylogger_start", WaitTime: 0},
				{Phase: "Lateral Movement", TaskType: "lateral_list", WaitTime: 60},
				{Phase: "Impact", TaskType: "shell", Params: map[string]string{"command": "whoami"}, WaitTime: 0},
			},
		},
		{
			Name:        "Stealth Operations",
			Description: "Low-and-slow discovery focused chain.",
			Steps: []killChainStep{
				{Phase: "Discovery", TaskType: "ps", WaitTime: 120},
				{Phase: "Discovery", TaskType: "ls", WaitTime: 120},
				{Phase: "Defense Evasion", TaskType: "av", WaitTime: 60},
				{Phase: "Command and Control", TaskType: "beacon_now", WaitTime: 0},
			},
		},
	}
}

// splitStepParams maps a template step's params onto the task columns the agent
// dispatcher reads (command/path/data). Unknown keys are ignored so templates
// stay readable without naming internal columns.
func splitStepParams(params map[string]string) (command, path, data string) {
	if v, ok := params["command"]; ok {
		command = v
	}
	if v, ok := params["path"]; ok {
		path = v
	}
	if v, ok := params["data"]; ok {
		data = v
	}
	return command, path, data
}

func findKillChainTemplate(name string) *killChainTemplate {
	for i := range builtinKillChainTemplates() {
		if builtinKillChainTemplates()[i].Name == name {
			return &builtinKillChainTemplates()[i]
		}
	}
	return nil
}

// handleCampaignKillChain seeds a campaign's kill chain: it creates one real
// pending task per template step per campaign agent, so the campaign timeline
// and phase stats reflect actual queued operations. Previously this endpoint
// merely acknowledged the request and created nothing.
// POST /v1/campaigns/:id/killchain
func (s *Server) handleCampaignKillChain(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Template string `json:"template"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Template == "" {
		respondError(c, http.StatusBadRequest, `request body requires {"template": "<template name>"}`)
		return
	}

	var campaign db.Campaign
	if !s.findOrFail(c, &campaign, id, "campaign") {
		return
	}

	var agents []db.Implant
	if err := s.db.Model(&campaign).Association("Agents").Find(&agents); err != nil {
		slog.Error("Failed to load campaign agents", "err", err)
		respondError(c, http.StatusInternalServerError, "failed to load campaign agents")
		return
	}
	if len(agents) == 0 {
		respondError(c, http.StatusBadRequest, "campaign has no agents: add agents to the campaign before seeding its kill chain")
		return
	}

	tpl := findKillChainTemplate(req.Template)
	if tpl == nil {
		respondError(c, http.StatusBadRequest, "unknown kill chain template: "+req.Template)
		return
	}

	// Reject templates referencing task types no agent can execute; seeding a
	// ghost task type would fabricate queue entries that can never run.
	for _, st := range tpl.Steps {
		if !IsKnownTaskType(st.TaskType) && !protocol.ValidTaskType(st.TaskType) {
			respondError(c, http.StatusBadRequest,
				fmt.Sprintf("template %q step references unknown task type %q", tpl.Name, st.TaskType))
			return
		}
	}

	operator := "operator"
	if u, ok := c.Get("user"); ok {
		if s, ok := u.(string); ok && s != "" {
			operator = s
		}
	}

	// Respect the same per-agent pending ceiling and status resolution as the
	// single/batch task paths, so seeding can't silently bypass queue limits.
	var tasks []db.Task
	skippedAgents := 0
	s.agentPendingTasksMu.Lock()
	for _, a := range agents {
		if s.agentPendingTasks[a.ID]+len(tpl.Steps) > MaxPendingTasksPerAgent {
			skippedAgents++
			continue
		}
		for _, st := range tpl.Steps {
			command, path, data := splitStepParams(st.Params)
			if len(command) > MaxCommandLength {
				continue
			}
			tasks = append(tasks, db.Task{
				AgentID:   a.ID,
				Type:      st.TaskType,
				Command:   command,
				Path:      path,
				Data:      data,
				Status:    s.resolveInitialTaskStatus(st.TaskType),
				CreatedBy: operator,
			})
			s.agentPendingTasks[a.ID]++
		}
	}
	s.agentPendingTasksMu.Unlock()

	if len(tasks) == 0 {
		respondError(c, http.StatusBadRequest, "no tasks created: all campaign agents are at the pending-task ceiling")
		return
	}
	if err := s.db.CreateInBatches(tasks, 100).Error; err != nil {
		for i := range tasks {
			s.decPendingTasks(tasks[i].AgentID)
		}
		slog.Error("Failed to seed campaign kill chain", "err", err)
		respondError(c, http.StatusInternalServerError, "failed to create tasks")
		return
	}
	for _, t := range tasks {
		s.broadcastTaskUpdate(t.AgentID, t)
	}

	slog.Info("Campaign kill chain seeded",
		"campaign_id", id, "template", tpl.Name,
		"tasks", len(tasks), "agents", len(agents), "skipped", skippedAgents)
	respond(c, gin.H{
		"success":        true,
		"campaign_id":    id,
		"template":       tpl.Name,
		"tasks_created":  len(tasks),
		"agents":         len(agents),
		"agents_skipped": skippedAgents,
	})
}

// handleMitreTemplates returns the built-in kill chain templates.
// GET /mitre/templates
func (s *Server) handleMitreTemplates(c *gin.Context) {
	respond(c, gin.H{"success": true, "data": builtinKillChainTemplates()})
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
