package server

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// ── AI Chat Page ──────────────────────────────────────────────────────────

// aiConfigPublicView returns a redacted view of the AI configuration safe to
// send to any authenticated user. The provider API key is NEVER included
// (S1): only non-secret display fields are exposed.
func (s *Server) aiConfigPublicView() gin.H {
	s.configMu.RLock()
	defer s.configMu.RUnlock()
	return gin.H{
		"enabled":          s.cfg.AI.Enabled,
		"has_api_key":      s.cfg.AI.APIKey != "",
		"provider":         s.cfg.AI.Provider,
		"model":            s.cfg.AI.Model,
		"endpoint":         s.cfg.AI.Endpoint,
		"system_prompt":    s.cfg.AI.SystemPrompt,
		"allow_execute":    s.cfg.AI.AllowExecute,
		"engagement_notes": s.cfg.AI.EngagementNotes,
	}
}

func (s *Server) aiExecutionEnabled() bool {
	if s.cfg == nil {
		return false
	}
	s.configMu.RLock()
	defer s.configMu.RUnlock()
	return s.cfg.AI.AllowExecute
}

func (s *Server) handleAIPage(c *gin.Context) {
	stats := s.getNavStats(c)
	publicConfig := s.aiConfigPublicView()
	aiEnabled, _ := publicConfig["enabled"].(bool)
	aiHasAPIKey, _ := publicConfig["has_api_key"].(bool)
	data := gin.H{
		"Title":      "ForgeC2 - AI Assistant",
		"ActiveNav":  "ai",
		"IsFullPage": true,
		// Never embed the raw AIConfig: it carries the provider API key. Send a
		// redacted view so the key is never exposed to any authenticated user
		// (including non-admin operators) via the JSON page data (S1).
		"AIConfig":     publicConfig,
		"AIConfigured": aiEnabled && aiHasAPIKey,
		"AIHasAPIKey":  aiHasAPIKey,
	}
	s.addUserToData(c, data)
	for k, v := range stats {
		data[k] = v
	}
	slog.Info("AI page rendered", "enabled", aiEnabled, "provider", publicConfig["provider"])
	s.renderPageOrJSON(c, data)
}

// ── AI Config Save ────────────────────────────────────────────────────────

func (s *Server) handleAIConfig(c *gin.Context) {
	if !s.requireAdmin(c) {
		return
	}
	var req struct {
		Enabled         bool   `json:"enabled"`
		Provider        string `json:"provider"`
		APIKey          string `json:"api_key"`
		Model           string `json:"model"`
		Endpoint        string `json:"endpoint"`
		SystemPrompt    string `json:"system_prompt"`
		EngagementNotes string `json:"engagement_notes"`
		AllowExecute    bool   `json:"allow_execute"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, AIConfigRequestMaxBytes)
	if err := c.ShouldBindJSON(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			respondError(c, http.StatusRequestEntityTooLarge, "AI configuration request too large")
			return
		}
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}

	req.Provider = strings.ToLower(strings.TrimSpace(req.Provider))
	req.APIKey = strings.TrimSpace(req.APIKey)
	req.Model = strings.TrimSpace(req.Model)
	req.Endpoint = strings.TrimSpace(req.Endpoint)
	allowedProviders := map[string]bool{
		"deepseek": true, "openai": true, "claude": true, "qianwen": true,
		"zhipu": true, "longcat": true, "custom": true,
	}
	if !allowedProviders[req.Provider] {
		respondError(c, http.StatusBadRequest, "unsupported AI provider")
		return
	}
	if req.Model == "" {
		req.Model = aiDefaultModel(req.Provider)
	}
	if len(req.APIKey) > 16*1024 || len(req.Model) > 200 || len(req.Endpoint) > 2048 || len(req.SystemPrompt) > 16*1024 {
		respondError(c, http.StatusBadRequest, "AI configuration field too large")
		return
	}
	if strings.ContainsAny(req.APIKey, "\r\n") {
		respondError(c, http.StatusBadRequest, "API key contains invalid characters")
		return
	}
	if len(req.EngagementNotes) > aiMaxNotesLen {
		respondError(c, http.StatusBadRequest, fmt.Sprintf("engagement notes too large (max %d chars)", aiMaxNotesLen))
		return
	}
	if req.Endpoint != "" {
		if err := validateExternalURL(req.Endpoint); err != nil {
			respondError(c, http.StatusBadRequest, "AI endpoint is invalid or blocked")
			return
		}
	}

	s.configMu.RLock()
	hasExistingAPIKey := strings.TrimSpace(s.cfg.AI.APIKey) != ""
	s.configMu.RUnlock()
	if req.Enabled && req.APIKey == "" && !hasExistingAPIKey {
		respondError(c, http.StatusBadRequest, "API key is required when AI is enabled")
		return
	}

	slog.Info("AI config save request", "enabled", req.Enabled, "provider", req.Provider, "model", req.Model)
	s.configMu.Lock()
	previousAI := s.cfg.AI
	nextAI := previousAI
	nextAI.Enabled = req.Enabled
	nextAI.Provider = req.Provider
	if req.APIKey != "" {
		nextAI.APIKey = req.APIKey
	}
	nextAI.Model = req.Model
	nextAI.Endpoint = req.Endpoint
	nextAI.SystemPrompt = req.SystemPrompt
	nextAI.AllowExecute = req.AllowExecute
	nextAI.EngagementNotes = req.EngagementNotes
	s.cfg.AI = nextAI
	s.configMu.Unlock()
	configPath := s.configPath
	if configPath == "" {
		configPath = "config.yaml"
	}
	if err := s.cfg.Save(configPath); err != nil {
		// A failed disk write must not leave the running server on a config the
		// operator was explicitly told did not save.
		s.configMu.Lock()
		if s.cfg.AI == nextAI {
			s.cfg.AI = previousAI
		}
		s.configMu.Unlock()
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "AI operation"))
		return
	}
	username, _ := c.Get("user")
	slog.Info("AI config saved", "user", username, "enabled", req.Enabled, "provider", req.Provider)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "AI config saved"})
}

func aiDefaultModel(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openai", "custom":
		return "gpt-4o-mini"
	case "claude":
		return "claude-3-5-sonnet-latest"
	case "qianwen":
		return "qwen-plus"
	case "zhipu":
		return "glm-4-flash"
	case "longcat":
		return "LongCat-Flash-Chat"
	default:
		return "deepseek-chat"
	}
}

// ── SSE Chat (streaming) ──────────────────────────────────────────────────

func (s *Server) handleAIChat(c *gin.Context) {
	s.configMu.RLock()
	aiEnabled := s.cfg.AI.Enabled
	aiAPIKey := s.cfg.AI.APIKey
	aiProvider := s.cfg.AI.Provider
	aiModel := s.cfg.AI.Model
	aiEndpoint := s.cfg.AI.Endpoint
	if aiEndpoint == "" {
		aiEndpoint = s.cfg.AIEndpoint()
	}
	aiSystemPrompt := s.cfg.AI.SystemPrompt
	aiEngagementNotes := s.cfg.AI.EngagementNotes
	aiMaxConversationTurns := s.cfg.AI.MaxConversationTurns
	aiMaxToolRounds := s.cfg.AI.MaxToolRounds
	aiMaxDuplicateToolCalls := s.cfg.AI.MaxDuplicateToolCalls
	s.configMu.RUnlock()
	if !aiEnabled || aiAPIKey == "" {
		slog.Warn("AI chat blocked", "enabled", aiEnabled, "provider", aiProvider)
		respondError(c, http.StatusBadRequest, "AI not configured. Set api_key in AI settings.")
		return
	}

	var req struct {
		Messages []chatMessage `json:"messages"`
		Context  struct {
			Page    string `json:"page"`
			AgentID string `json:"agent_id"`
		} `json:"context"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, AIChatRequestMaxBytes)
	if err := c.ShouldBindJSON(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			respondError(c, http.StatusRequestEntityTooLarge, "AI chat request too large")
			return
		}
		respondError(c, http.StatusBadRequest, "messages required")
		return
	}
	if len(req.Messages) == 0 {
		respondError(c, http.StatusBadRequest, "messages required")
		return
	}
	if len(req.Messages) > AIChatMaxMessages {
		respondError(c, http.StatusBadRequest, "too many messages")
		return
	}
	for _, message := range req.Messages {
		if message.Role != "user" && message.Role != "assistant" {
			respondError(c, http.StatusBadRequest, "invalid message role")
			return
		}
	}
	principal := aiPrincipal{}
	if s.db != nil {
		var ok bool
		principal, ok = s.currentAIPrincipal(c)
		if !ok {
			respondError(c, http.StatusForbidden, "AI use permission required")
			return
		}
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	clearHTTPWriteDeadline(c.Writer)
	flusher, _ := c.Writer.(http.Flusher)

	model := aiModel
	if model == "" {
		model = aiDefaultModel(aiProvider)
	}

	sysPrompt := effectiveAISystemPrompt(aiSystemPrompt)
	if principal.TenantID == 0 {
		if notes := strings.TrimSpace(aiEngagementNotes); notes != "" {
			sysPrompt += "\n\n## Engagement memory (operator-maintained, persistent)\n" + notes
		}
	}
	if principal.TenantID == 0 {
		if snap := s.buildSituationSnapshot(); snap != "" {
			sysPrompt += "\n\n" + snap
		}
	}

	// Operator context: which page/agent the human is looking at right now.
	// Tools fall back to this agent when the model omits agent_id, so "dump
	// creds on this machine" works without pasting identifiers.
	reqCtx := &aiReqCtx{Principal: principal}
	if ctxAgent := strings.TrimSpace(req.Context.AgentID); ctxAgent != "" {
		if aid := s.resolveAIAgentID(reqCtx, ctxAgent); aid != "" {
			reqCtx.DefaultAgentID = aid
			var agent db.Implant
			if err := s.db.Where("id = ?", aid).First(&agent).Error; err == nil {
				label := agent.Hostname
				if label == "" {
					label = aid
				}
				sysPrompt += fmt.Sprintf("\n\n## Operator context\nOperator is currently viewing agent \"%s\" (id: %s, os: %s, user: %s). When they say \"this machine\", \"this host\" or \"the target\" without naming one, they mean THIS agent.", label, aid, agent.OS, agent.Username)
			} else {
				sysPrompt += "\n\n## Operator context\nOperator is currently viewing agent id " + aid + ". References to \"this machine\" mean that agent."
			}
		}
	}
	if page := strings.TrimSpace(req.Context.Page); page != "" {
		sysPrompt += "\nOperator's current console page: " + truncateStr(page, 120)
	}

	userMessages := trimConversationHistory(req.Messages)

	events := s.converse(model, sysPrompt, userMessages, c.Request.Context(), reqCtx, aiConversationOptions{
		provider:              aiProvider,
		endpoint:              aiEndpoint,
		apiKey:                aiAPIKey,
		maxConversationTurns:  aiMaxConversationTurns,
		maxToolRounds:         aiMaxToolRounds,
		maxDuplicateToolCalls: aiMaxDuplicateToolCalls,
	})
	heartbeat := time.NewTicker(AIStreamHeartbeatInterval)
	defer heartbeat.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-heartbeat.C:
			// Explicit ping events keep reverse proxies and the browser watchdog
			// alive while a provider is still thinking before its first token.
			s.writeSSE(c, flusher, "ping", strconv.FormatInt(time.Now().Unix(), 10))
		case evt, ok := <-events:
			if !ok {
				return
			}
			s.writeSSE(c, flusher, evt.Type, evt.Data)
		}
	}
}

// aiMaxNotesLen bounds the persistent engagement memory so a runaway tool
// loop cannot balloon the system prompt.
const aiMaxNotesLen = 8000

// aiMaxContextChars bounds the conversation payload sent to the provider
// (~4 chars/token heuristic). Oldest turns beyond the budget are collapsed.
const aiMaxContextChars = 48000

const aiMaxContextMessageChars = 16000

// trimConversationHistory collapses the oldest turns when the conversation
// grows past the context budget: everything except the system prompt and the
// most recent messages is replaced by a single deterministic summary marker.
// This keeps long engagements usable without an extra summarization API call.
func trimConversationHistory(msgs []chatMessage) []chatMessage {
	normalized := make([]chatMessage, len(msgs))
	for i, message := range msgs {
		if len(message.Content) > aiMaxContextMessageChars {
			message.Content = truncateStr(message.Content, aiMaxContextMessageChars)
		}
		normalized[i] = message
	}
	msgs = normalized
	total := 0
	for _, m := range msgs {
		total += len(m.Content)
	}
	if total <= aiMaxContextChars {
		return msgs
	}
	// Walk from the newest backwards accumulating until over half the budget;
	// everything older is dropped into a trim notice.
	keepFrom := len(msgs)
	kept := 0
	for i := len(msgs) - 1; i >= 0; i-- {
		kept += len(msgs[i].Content)
		keepFrom = i
		if kept > aiMaxContextChars/2 {
			break
		}
	}
	if keepFrom == 0 {
		return msgs
	}
	trimmed := msgs[keepFrom:]
	// "user" role keeps provider compatibility: the Claude converter folds
	// every system-role message into the top-level prompt, which would
	// clobber the real system prompt.
	notice := chatMessage{
		Role: "user",
		Content: fmt.Sprintf(
			"[System] Earlier conversation trimmed for context budget: %d messages removed. Key facts from those turns are no longer visible — ask the operator if needed.",
			keepFrom),
	}
	out := make([]chatMessage, 0, len(trimmed)+1)
	out = append(out, notice)
	out = append(out, trimmed...)
	return out
}

func (s *Server) handleAIPendingTasks(c *gin.Context) {
	var tasks []db.Task
	if err := s.db.Where("status = ? AND created_by = ?", "pending_approval", "ai").Order("created_at desc").Limit(20).Find(&tasks).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "query failed")
		return
	}
	type pendingTask struct {
		ID        uint   `json:"id"`
		AgentID   string `json:"agent_id"`
		Hostname  string `json:"hostname"`
		Command   string `json:"command"`
		Type      string `json:"type"`
		CreatedAt string `json:"created_at"`
	}
	var out []pendingTask
	for _, t := range tasks {
		hostname := ""
		var ag db.Implant
		if err := s.db.Select("hostname").Where("id = ?", t.AgentID).First(&ag).Error; err == nil {
			hostname = ag.Hostname
		}
		out = append(out, pendingTask{
			ID: t.ID, AgentID: t.AgentID, Hostname: hostname,
			Command: truncateStr(t.Command, 200), Type: t.Type,
			CreatedAt: t.CreatedAt.Format(time.RFC3339),
		})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "tasks": out})
}

type aiToolLimits struct {
	maxConversationTurns  int
	maxToolRounds         int
	maxDuplicateToolCalls int
}

func resolveAIToolLimits(maxConversationTurns, maxToolRounds, maxDuplicateToolCalls int) aiToolLimits {
	// 0 in config means "unlimited", but an unbounded tool loop never
	// closes the SSE stream and the composer stays stuck on loading.
	// Apply a hard safety ceiling so the conversation always terminates.
	if maxConversationTurns <= 0 {
		maxConversationTurns = AISafetyMaxTurns
	}
	if maxToolRounds <= 0 {
		maxToolRounds = AISafetyMaxToolRounds
	}
	if maxDuplicateToolCalls <= 0 {
		maxDuplicateToolCalls = AISafetyMaxDuplicateTools
	}
	return aiToolLimits{
		maxConversationTurns:  maxConversationTurns,
		maxToolRounds:         maxToolRounds,
		maxDuplicateToolCalls: maxDuplicateToolCalls,
	}
}

// clearHTTPWriteDeadline drops the server WriteTimeout for a long-lived SSE
// response. gin.Context.Writer wraps the net/http ResponseWriter, so we unwrap
// a few layers until SetWriteDeadline sticks.
func clearHTTPWriteDeadline(w http.ResponseWriter) {
	for i := 0; i < 6 && w != nil; i++ {
		if err := http.NewResponseController(w).SetWriteDeadline(time.Time{}); err == nil {
			return
		}
		u, ok := w.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			return
		}
		w = u.Unwrap()
	}
}

// aiUsage carries provider token accounting for one conversation.
type aiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// aiReqCtx carries per-request context through the tool loop: the agent the
// operator is currently viewing, used as fallback target when a tool call
// omits agent_id ("run mimikatz on this box" while looking at that box).
type aiReqCtx struct {
	DefaultAgentID         string
	Principal              aiPrincipal
	SessionID              uint
	RunID                  string
	AllowLowRiskWrites     bool
	ApprovedIntent         bool
	KnowledgeCollectionIDs []uint
	DisableTools           bool
}

type aiConversationOptions struct {
	provider              string
	endpoint              string
	apiKey                string
	maxConversationTurns  int
	maxToolRounds         int
	maxDuplicateToolCalls int
}
