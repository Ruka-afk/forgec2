package server

import (
	"net/http"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/scripting"
	"github.com/gin-gonic/gin"
)

// handleScriptingPage renders the scripting console page
func (s *Server) handleScriptingPage(c *gin.Context) {
	stats := s.getNavStats()
	data := gin.H{
		"Title":     "ForgeC2 - Scripting Console",
		"ActiveNav": "scripting",
		"Stats":     stats,
		"Scripts":   scripting.GetEngine().ListScripts(),
	}
	for k, v := range stats {
		data[k] = v
	}
	s.renderPageOrJSON(c, data)
}

// handleAPIGetScripts returns all scripts
func (s *Server) handleAPIGetScripts(c *gin.Context) {
	scripts := scripting.GetEngine().ListScripts()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": scripts})
}

// handleAPISaveScript saves a new or updated script
func (s *Server) handleAPISaveScript(c *gin.Context) {
	var req struct {
		ID          string `json:"id"`
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		Code        string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid request"})
		return
	}

	engine := scripting.GetEngine()
	if req.ID == "" {
		req.ID = generateID()
	}

	script := &scripting.Script{
		ID:          req.ID,
		Name:        req.Name,
		Description: req.Description,
		Code:        req.Code,
	}
	engine.SaveScript(script)

	s.LogAuditRecord(c, "save_script", "scripting", req.ID, "Script saved: "+req.Name, true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": script})
}

// handleAPIDeleteScript deletes a script
func (s *Server) handleAPIDeleteScript(c *gin.Context) {
	id := c.Param("id")
	if scripting.GetEngine().DeleteScript(id) {
		s.LogAuditRecord(c, "delete_script", "scripting", id, "Script deleted", true, nil)
		c.JSON(http.StatusOK, gin.H{"success": true})
	} else {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "script not found"})
	}
}

// handleAPIExecuteScript executes a script
func (s *Server) handleAPIExecuteScript(c *gin.Context) {
	var req struct {
		ScriptID string                 `json:"script_id"`
		Code     string                 `json:"code"`
		Context  map[string]interface{} `json:"context"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid request"})
		return
	}

	// Build context from database
	context := map[string]interface{}{
		"agents":    []interface{}{},
		"tasks":     []interface{}{},
		"credentials": []interface{}{},
	}

	var agents []db.Implant
	s.db.Select("id, hostname, ip, os, status").Limit(100).Find(&agents)
	context["agents"] = agents

	var tasks []db.Task
	s.db.Select("id, agent_id, type, command, status").Order("created_at desc").Limit(50).Find(&tasks)
	context["tasks"] = tasks

	var creds []db.CredentialEntry
	s.db.Select("agent_id, domain, username, type, source").Limit(50).Find(&creds)
	context["credentials"] = creds

	engine := scripting.GetEngine()
	var result scripting.ExecutionResult

	if req.ScriptID != "" {
		result = engine.Execute(req.ScriptID, context)
	} else if req.Code != "" {
		result = engine.ExecuteCode(req.Code, context)
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "no script_id or code provided"})
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

func generateID() string {
	return "script_" + time.Now().Format("20060102150405")
}
