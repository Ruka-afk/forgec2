package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleAPIWorkflows(c *gin.Context) {
	p := parsePagination(c, 50, 200)
	var total int64
	if err := s.db.Model(&db.Workflow{}).Count(&total).Error; err != nil {
		slog.Error("Failed to count workflows", "err", err)
	}
	var workflows []db.Workflow
	if err := s.db.Preload("Steps").Order("created_at desc").Offset(p.Offset).Limit(p.PageSize).Find(&workflows).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to load workflows")
		return
	}
	respond(c, gin.H{"workflows": workflows, "total": total, "page": p.Page, "page_size": p.PageSize})
}

func (s *Server) handleAPIWorkflowsDetail(c *gin.Context) {
	var wf db.Workflow
	if err := s.db.Preload("Steps").First(&wf, "id = ?", c.Param("id")).Error; err != nil {
		respondError(c, http.StatusNotFound, "workflow not found")
		return
	}
	respond(c, gin.H{"workflow": wf})
}

func (s *Server) handleAPIWorkflowsToggle(c *gin.Context) {
	var wf db.Workflow
	if !s.findOrFail(c, &wf, c.Param("id"), "workflow") {
		return
	}
	wf.Enabled = !wf.Enabled
	if err := s.db.Save(&wf).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to update workflow")
		return
	}
	respond(c, gin.H{"success": true, "enabled": wf.Enabled})
	s.LogAuditRecord(c, "workflow_toggle", "workflow", wf.ID, fmt.Sprintf("Workflow %s", map[bool]string{true: "enabled", false: "disabled"}[wf.Enabled]), true, nil)
}

func (s *Server) handleAPIWorkflowsExecute(c *gin.Context) {
	var wf db.Workflow
	if err := s.db.Preload("Steps").First(&wf, "id = ?", c.Param("id")).Error; err != nil {
		respondError(c, http.StatusNotFound, "workflow not found")
		return
	}

	var targetAgents []db.Implant
	switch wf.ScopeType {
	case "tags", "groups", "agents":
		if wf.ScopeIDs == "" {
			respond(c, gin.H{"success": true, "task_count": 0, "agents_count": 0})
			return
		}
		var ids []string
		if err := json.Unmarshal([]byte(wf.ScopeIDs), &ids); err != nil {
			respondError(c, http.StatusInternalServerError, "invalid scope_ids")
			return
		}
	if wf.ScopeType == "agents" {
		if err := s.db.Where("id IN ?", ids).Limit(AgentQueryLimit).Find(&targetAgents).Error; err != nil {
			slog.Error("Workflow: failed to query target agents by IDs", "err", err)
		}
	} else if wf.ScopeType == "tags" {
		if err := s.db.Distinct("implants.*").Joins("JOIN agent_tag_assignments ON agent_tag_assignments.implant_id = implants.id").
			Where("agent_tag_assignments.agent_tag_id IN ?", ids).Limit(AgentQueryLimit).Find(&targetAgents).Error; err != nil {
			slog.Error("Workflow: failed to query target agents by tags", "err", err)
		}
	} else {
		if err := s.db.Distinct("implants.*").Joins("JOIN agent_group_assignments ON agent_group_assignments.implant_id = implants.id").
			Where("agent_group_assignments.agent_group_id IN ?", ids).Limit(AgentQueryLimit).Find(&targetAgents).Error; err != nil {
			slog.Error("Workflow: failed to query target agents by groups", "err", err)
		}
	}
default:
	if err := s.db.Where("last_seen > ?", time.Now().Add(-30*time.Minute)).Limit(AgentQueryLimit).Find(&targetAgents).Error; err != nil {
		slog.Error("Workflow: failed to query recent agents", "err", err)
	}
	}

	agentIDs := make([]string, len(targetAgents))
	for i, a := range targetAgents {
		agentIDs[i] = a.ID
	}

	// Use workflow engine for conditional execution
	engine := NewWorkflowEngine(s)
	taskCount, agentsCount, err := engine.ExecuteWorkflow(wf, agentIDs)
	if err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Workflow execution"))
		return
	}

	respond(c, gin.H{"success": true, "task_count": taskCount, "agents_count": agentsCount})
	s.LogAuditRecord(c, "workflow_execute", "workflow", wf.ID, fmt.Sprintf("Executed workflow on %d agent(s), created %d task(s)", agentsCount, taskCount), true, nil)
}

func (s *Server) handleAPICreateWorkflow(c *gin.Context) {
	var req struct {
		Name        string            `json:"name"`
		Description string            `json:"description"`
		ScopeType   string            `json:"scope_type"`
		ScopeIDs    string            `json:"scope_ids"`
		Steps       []db.WorkflowStep `json:"steps"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Name == "" {
		respondError(c, http.StatusBadRequest, "name is required")
		return
	}
	if req.ScopeType == "" {
		req.ScopeType = "all"
	}
	wf := db.Workflow{
		ID:          fmt.Sprintf("%d", time.Now().UnixNano()),
		Name:        req.Name,
		Description: req.Description,
		Enabled:     true,
		ScopeType:   req.ScopeType,
		ScopeIDs:    req.ScopeIDs,
		CreatedBy:   s.currentUsername(c),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := s.db.Create(&wf).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Workflow operation"))
		return
	}
	for i, step := range req.Steps {
		step.WorkflowID = wf.ID
		step.StepOrder = i + 1
		if step.Shell == "" {
			step.Shell = "cmd"
		}
		if step.TimeoutSec == 0 {
			step.TimeoutSec = 60
		}
		step.CreatedAt = time.Now()
		if err := s.db.Create(&step).Error; err != nil {
			slog.Error("Failed to create workflow step", "workflow_id", wf.ID, "step", i+1, "error", err)
		}
	}
	respond(c, gin.H{"success": true, "id": wf.ID})
}

func (s *Server) handleAPIUpdateWorkflow(c *gin.Context) {
	id := c.Param("id")
	var wf db.Workflow
	if !s.findOrFail(c, &wf, id, "workflow") {
		return
	}
	var req struct {
		Name        string            `json:"name"`
		Description string            `json:"description"`
		ScopeType   string            `json:"scope_type"`
		ScopeIDs    string            `json:"scope_ids"`
		Enabled     *bool             `json:"enabled"`
		Steps       []db.WorkflowStep `json:"steps"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Name != "" {
		wf.Name = req.Name
	}
	if req.Description != "" {
		wf.Description = req.Description
	}
	if req.ScopeType != "" {
		wf.ScopeType = req.ScopeType
	}
	wf.ScopeIDs = req.ScopeIDs
	if req.Enabled != nil {
		wf.Enabled = *req.Enabled
	}
	wf.UpdatedAt = time.Now()
	if err := s.db.Save(&wf).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Workflow operation"))
		return
	}

	if req.Steps != nil {
		if err := s.db.Where("workflow_id = ?", id).Delete(&db.WorkflowStep{}).Error; err != nil {
			respondError(c, http.StatusInternalServerError, "failed to delete old workflow steps")
			return
		}
		for i, step := range req.Steps {
			step.WorkflowID = wf.ID
			step.StepOrder = i + 1
			if step.Shell == "" {
				step.Shell = "cmd"
			}
			if step.TimeoutSec == 0 {
				step.TimeoutSec = 60
			}
			step.CreatedAt = time.Now()
			if err := s.db.Create(&step).Error; err != nil {
				slog.Error("Failed to create workflow step", "workflow_id", wf.ID, "step", i+1, "error", err)
			}
		}
	}
	respond(c, gin.H{"success": true})
}

func (s *Server) handleListWorkflowExecutions(c *gin.Context) {
	workflowID := c.Param("id")
	var executions []db.WorkflowExecution
	if err := s.db.Where("workflow_id = ?", workflowID).Order("started_at desc").Limit(50).Find(&executions).Error; err != nil {
		slog.Error("Failed to list workflow executions", "err", err)
	}
	respond(c, gin.H{"executions": executions})
}

func (s *Server) handleGetWorkflowExecution(c *gin.Context) {
	executionID := c.Param("executionId")
	var exec db.WorkflowExecution
	if !s.findOrFail(c, &exec, executionID, "execution") {
		return
	}
	var logs []db.WorkflowStepLog
	if err := s.db.Where("execution_id = ?", exec.ID).Order("step_order").Find(&logs).Error; err != nil {
		slog.Error("Failed to query workflow step logs", "err", err)
	}
	respond(c, gin.H{"execution": exec, "logs": logs})
}

func (s *Server) handleAPIDeleteWorkflow(c *gin.Context) {
	id := c.Param("id")
	if err := s.db.Where("workflow_id = ?", id).Delete(&db.WorkflowStep{}).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to delete workflow steps")
		return
	}
	if err := s.db.Delete(&db.Workflow{}, "id = ?", id).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Workflow operation"))
		return
	}
	s.LogAuditRecord(c, "delete_workflow", "workflow", id, "Workflow deleted", true, nil)
	respond(c, gin.H{"success": true})
}

func (s *Server) handleAPIGroups(c *gin.Context) {
	var groups []db.AgentGroup
	if err := s.db.Limit(200).Find(&groups).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to load groups")
		return
	}
	type enrichedGroup struct {
		db.AgentGroup
		AgentCount int `json:"agent_count"`
		ChildCount int `json:"child_count"`
	}
	if len(groups) == 0 {
		respond(c, gin.H{"groups": []enrichedGroup{}})
		return
	}
	groupIDs := make([]string, len(groups))
	for i, g := range groups {
		groupIDs[i] = g.ID
	}

	type agentCountRow struct {
		GroupID string
		Count   int64
	}
	var agentCounts []agentCountRow
	if err := s.db.Model(&db.Implant{}).Select("agent_group_assignments.agent_group_id as group_id, COUNT(*) as count").
		Joins("JOIN agent_group_assignments ON agent_group_assignments.implant_id = implants.id").
		Where("agent_group_assignments.agent_group_id IN ?", groupIDs).
		Group("agent_group_assignments.agent_group_id").Find(&agentCounts).Error; err != nil {
		slog.Error("Failed to query agent counts by group", "err", err)
	}
	agentCountMap := make(map[string]int64, len(agentCounts))
	for _, row := range agentCounts {
		agentCountMap[row.GroupID] = row.Count
	}

	type childCountRow struct {
		ParentID string
		Count    int64
	}
	var childCounts []childCountRow
	if err := s.db.Model(&db.AgentGroup{}).Select("parent_id, COUNT(*) as count").
		Where("parent_id IN ?", groupIDs).
		Group("parent_id").Find(&childCounts).Error; err != nil {
		slog.Error("Failed to query child counts by group", "err", err)
	}
	childCountMap := make(map[string]int64, len(childCounts))
	for _, row := range childCounts {
		childCountMap[row.ParentID] = row.Count
	}

	result := make([]enrichedGroup, len(groups))
	for i, g := range groups {
		result[i] = enrichedGroup{
			AgentGroup: g,
			AgentCount: int(agentCountMap[g.ID]),
			ChildCount: int(childCountMap[g.ID]),
		}
	}
	respond(c, gin.H{"groups": result})
}

func (s *Server) handleAPICreateGroup(c *gin.Context) {
	var req struct {
		Name        string  `json:"name"`
		Description string  `json:"description"`
		Color       string  `json:"color"`
		ParentID    *string `json:"parent_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Name == "" {
		respondError(c, http.StatusBadRequest, "name is required")
		return
	}
	if req.Color == "" {
		req.Color = "#2ecc71"
	}
	group := db.AgentGroup{
		ID:          fmt.Sprintf("%d", time.Now().UnixNano()),
		Name:        req.Name,
		Description: req.Description,
		Color:       req.Color,
		ParentID:    req.ParentID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := s.db.Create(&group).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Workflow operation"))
		return
	}
	respond(c, gin.H{"success": true, "id": group.ID})
}

func (s *Server) handleAPIUpdateGroup(c *gin.Context) {
	id := c.Param("id")
	var group db.AgentGroup
	if !s.findOrFail(c, &group, id, "group") {
		return
	}
	var req struct {
		Name        string  `json:"name"`
		Description string  `json:"description"`
		Color       string  `json:"color"`
		ParentID    *string `json:"parent_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Name != "" {
		group.Name = req.Name
	}
	if req.Description != "" {
		group.Description = req.Description
	}
	if req.Color != "" {
		group.Color = req.Color
	}
	group.ParentID = req.ParentID
	group.UpdatedAt = time.Now()
	if err := s.db.Save(&group).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to update group")
		return
	}
	respond(c, gin.H{"success": true})
}

func (s *Server) handleAPIDeleteGroup(c *gin.Context) {
	id := c.Param("id")
	var agentCount int64
	if err := s.db.Model(&db.Implant{}).Joins("JOIN agent_group_assignments ON agent_group_assignments.implant_id = implants.id").
		Where("agent_group_assignments.agent_group_id = ?", id).Count(&agentCount).Error; err != nil {
		slog.Error("Failed to count group agents", "group_id", id, "err", err)
		respondError(c, http.StatusInternalServerError, "failed to check agent count")
		return
	}
	if agentCount > 0 {
		respondError(c, http.StatusBadRequest, "cannot delete group with agents — reassign agents first")
		return
	}
	var childCount int64
	if err := s.db.Model(&db.AgentGroup{}).Where("parent_id = ?", id).Count(&childCount).Error; err != nil {
		slog.Error("Failed to count child groups", "parent_id", id, "err", err)
		respondError(c, http.StatusInternalServerError, "failed to check child group count")
		return
	}
	if childCount > 0 {
		respondError(c, http.StatusBadRequest, "cannot delete group with child groups — remove children first")
		return
	}
	if err := s.db.Delete(&db.AgentGroup{}, "id = ?", id).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to delete group")
		return
	}
	s.LogAuditRecord(c, "delete_group", "group", id, "Agent group deleted", true, nil)
	respond(c, gin.H{"success": true})
}
