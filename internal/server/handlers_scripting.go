package server

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/scripting"
	"github.com/gin-gonic/gin"
)

func (s *Server) loadScriptsFromDB() {
	var rows []db.Script
	if err := s.db.Where("enabled = ?", true).Find(&rows).Error; err != nil {
		return
	}
	engine := scripting.GetEngine()
	for _, row := range rows {
		_ = engine.LoadScript(row.ID, row.Name, row.Code)
	}
}

func (s *Server) handleScriptingPage(c *gin.Context) {
	stats := s.getNavStats()
	data := gin.H{
		"Title":     "ForgeC2 - Scripting Console",
		"ActiveNav": "scripting",
		"Stats":     stats,
		"Scripts":   s.listScriptsFromDB(),
	}
	for k, v := range stats {
		data[k] = v
	}
	s.renderPageOrJSON(c, data)
}

func (s *Server) listScriptsFromDB() []scripting.Script {
	var rows []db.Script
	s.db.Order("updated_at desc").Limit(200).Find(&rows)
	out := make([]scripting.Script, 0, len(rows))
	for _, row := range rows {
		out = append(out, scripting.Script{
			ID:          strconv.FormatUint(uint64(row.ID), 10),
			Name:        row.Name,
			Description: row.Description,
			Code:        row.Code,
			CreatedAt:   row.CreatedAt,
			UpdatedAt:   row.UpdatedAt,
			RunCount:    row.RunCount,
			LastRun:     row.LastRun,
		})
	}
	return out
}

func (s *Server) handleAPIGetScripts(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": s.listScriptsFromDB()})
}

func (s *Server) handleAPISaveScript(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	var req struct {
		ID          string `json:"id"`
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		Code        string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}

	var row db.Script
	now := time.Now()
	if req.ID != "" {
		if id, err := strconv.ParseUint(req.ID, 10, 64); err == nil {
			if err := s.db.First(&row, id).Error; err == nil {
				row.Name = req.Name
				row.Description = req.Description
				row.Code = req.Code
				row.UpdatedAt = now
				if err := s.db.Save(&row).Error; err != nil {
					respondError(c, http.StatusInternalServerError, sanitizeError(err, "script"))
					return
				}
			}
		}
	}
	if row.ID == 0 {
		row = db.Script{
			Name:        req.Name,
			Description: req.Description,
			Code:        req.Code,
			Enabled:     true,
			CreatedBy:   c.GetString("username"),
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := s.db.Create(&row).Error; err != nil {
			respondError(c, http.StatusInternalServerError, sanitizeError(err, "script"))
			return
		}
	}

	_ = scripting.GetEngine().LoadScript(row.ID, row.Name, row.Code)

	script := scripting.Script{
		ID:          strconv.FormatUint(uint64(row.ID), 10),
		Name:        row.Name,
		Description: row.Description,
		Code:        row.Code,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
		RunCount:    row.RunCount,
		LastRun:     row.LastRun,
	}
	s.LogAuditRecord(c, "save_script", "scripting", script.ID, "Script saved: "+req.Name, true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": script})
}

func (s *Server) handleAPIDeleteScript(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	uid, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid script id")
		return
	}
	var row db.Script
	if err := s.db.First(&row, uid).Error; err != nil {
		respondError(c, http.StatusNotFound, "script not found")
		return
	}
	if err := s.db.Delete(&row).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "script"))
		return
	}
	scripting.GetEngine().UnloadScript(row.Name)
	s.LogAuditRecord(c, "delete_script", "scripting", id, "Script deleted", true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleAPIExecuteScript(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	var req struct {
		ScriptID string                 `json:"script_id"`
		Code     string                 `json:"code"`
		Context  map[string]interface{} `json:"context"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}

	context := map[string]interface{}{
		"agents":      []interface{}{},
		"tasks":       []interface{}{},
		"credentials": []interface{}{},
	}

	var agents []db.Implant
	if err := s.db.Select("id, hostname, ip, os, status").Limit(100).Find(&agents).Error; err != nil {
		slog.Error("Scripting: failed to query agents", "err", err)
	}
	context["agents"] = agents

	var tasks []db.Task
	if err := s.db.Select("id, agent_id, type, command, status").Order("created_at desc").Limit(50).Find(&tasks).Error; err != nil {
		slog.Error("Scripting: failed to query tasks", "err", err)
	}
	context["tasks"] = tasks

	var creds []db.CredentialEntry
	if err := s.db.Select("agent_id, domain, username, type, source").Limit(50).Find(&creds).Error; err != nil {
		slog.Error("Scripting: failed to query credentials", "err", err)
	}
	context["credentials"] = creds

	engine := scripting.GetEngine()
	var result scripting.ExecutionResult
	caller := scripting.Caller{
		Username: c.GetString("username"),
		Role:     c.GetString("user_role"),
	}

	if req.ScriptID != "" {
		if uid, err := strconv.ParseUint(req.ScriptID, 10, 64); err == nil {
			var row db.Script
			if err := s.db.First(&row, uid).Error; err == nil {
				_ = engine.LoadScript(row.ID, row.Name, row.Code)
				row.RunCount++
				row.LastRun = time.Now()
				if err := s.db.Save(&row).Error; err != nil {
					slog.Error("Failed to update script run count", "script_id", uid, "err", err)
				}
			}
		}
		result = engine.Execute(req.ScriptID, context, caller)
	} else if req.Code != "" {
		result = engine.ExecuteCode(req.Code, context, caller)
	} else {
		respondError(c, http.StatusBadRequest, "no script_id or code provided")
		return
	}

	s.LogAuditRecord(c, "execute_script", "scripting", req.ScriptID, "Script executed", result.Success, nil)
	c.JSON(http.StatusOK, gin.H{"success": true, "result": result})
}

func (s *Server) handleAPIScriptsHistory(c *gin.Context) {
	var tasks []db.Task
	s.db.Where("type LIKE ?", "script_execute%").
		Order("created_at desc").
		Limit(50).
		Find(&tasks)

	type historyEntry struct {
		ID        uint      `json:"id"`
		AgentID   string    `json:"agent_id"`
		Type      string    `json:"type"`
		Command   string    `json:"command"`
		Status    string    `json:"status"`
		Result    string    `json:"result"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}

	entries := make([]historyEntry, len(tasks))
	for i, t := range tasks {
		entries[i] = historyEntry{
			ID:        t.ID,
			AgentID:   t.AgentID,
			Type:      t.Type,
			Command:   t.Command,
			Status:    t.Status,
			Result:    t.Result,
			CreatedAt: t.CreatedAt,
			UpdatedAt: t.UpdatedAt,
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "history": entries})
}
