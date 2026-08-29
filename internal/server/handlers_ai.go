package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// 鈹€鈹€ AI Chat Page 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€

// aiConfigPublicView returns a redacted view of the AI configuration safe to
// send to any authenticated user. The provider API key is NEVER included
// (S1): only non-secret display fields are exposed.
func (s *Server) aiConfigPublicView() gin.H {
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

func (s *Server) handleAIPage(c *gin.Context) {
	stats := s.getNavStats(c)
	data := gin.H{
		"Title":        "ForgeC2 - AI Assistant",
		"ActiveNav":    "ai",
		"IsFullPage":   true,
		// Never embed the raw AIConfig: it carries the provider API key. Send a
		// redacted view so the key is never exposed to any authenticated user
		// (including non-admin operators) via the JSON page data (S1).
		"AIConfig":     s.aiConfigPublicView(),
		"AIConfigured": s.cfg.AI.Enabled && s.cfg.AI.APIKey != "",
		"AIHasAPIKey":  s.cfg.AI.APIKey != "",
	}
	s.addUserToData(c, data)
	for k, v := range stats {
		data[k] = v
	}
	slog.Info("AI page rendered", "enabled", s.cfg.AI.Enabled, "provider", s.cfg.AI.Provider)
	s.renderPageOrJSON(c, data)
}

// 鈹€鈹€ AI Config Save 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€

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
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}

	slog.Info("AI config save request", "enabled", req.Enabled, "provider", req.Provider, "model", req.Model)
	s.configMu.Lock()
	s.cfg.AI.Enabled = req.Enabled
	s.cfg.AI.Provider = req.Provider
	if strings.TrimSpace(req.APIKey) != "" {
		s.cfg.AI.APIKey = req.APIKey
	}
	s.cfg.AI.Model = req.Model
	s.cfg.AI.Endpoint = req.Endpoint
	s.cfg.AI.SystemPrompt = req.SystemPrompt
	if len(req.EngagementNotes) <= aiMaxNotesLen {
		s.cfg.AI.EngagementNotes = req.EngagementNotes
	} else {
		s.configMu.Unlock()
		respondError(c, http.StatusBadRequest, fmt.Sprintf("engagement notes too large (max %d chars)", aiMaxNotesLen))
		return
	}
	s.configMu.Unlock()
	configPath := s.configPath
	if configPath == "" {
		configPath = "config.yaml"
	}
	if err := s.cfg.Save(configPath); err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "AI operation"))
		return
	}
	username, _ := c.Get("user")
	slog.Info("AI config saved", "user", username, "enabled", s.cfg.AI.Enabled, "provider", s.cfg.AI.Provider)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "AI config saved"})
}

// 鈹€鈹€ SSE Chat (streaming) 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€

func (s *Server) handleAIChat(c *gin.Context) {
	s.configMu.RLock()
	aiEnabled := s.cfg.AI.Enabled
	aiAPIKey := s.cfg.AI.APIKey
	aiProvider := s.cfg.AI.Provider
	s.configMu.RUnlock()
	if !aiEnabled || aiAPIKey == "" {
		slog.Warn("AI chat blocked", "enabled", aiEnabled, "provider", aiProvider)
		respondError(c, http.StatusBadRequest, "AI not configured. Set api_key in AI settings.")
		return
	}

	// The AI assistant can dispatch commands to agents (execute_command tool),
	// so gate it behind the same permission as manual command execution.
	role, _ := c.Get("user_role")
	roleStr, _ := role.(string)
	if roleStr != "admin" && !db.RoleHasPermission(roleStr, db.PermAgentsWrite) {
		respondError(c, http.StatusForbidden, "AI assistant requires agents.write permission")
		return
	}

	var req struct {
		Messages []chatMessage `json:"messages"`
		Context  struct {
			Page    string `json:"page"`
			AgentID string `json:"agent_id"`
		} `json:"context"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Messages) == 0 {
		respondError(c, http.StatusBadRequest, "messages required")
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	if rc := http.NewResponseController(c.Writer); rc != nil {
		_ = rc.SetWriteDeadline(time.Time{})
	}
	flusher, _ := c.Writer.(http.Flusher)

	model := s.cfg.AI.Model
	if model == "" {
		model = "deepseek-chat"
	}

	sysPrompt := s.cfg.AI.SystemPrompt
	if notes := strings.TrimSpace(s.cfg.AI.EngagementNotes); notes != "" {
		sysPrompt += "\n\n## Engagement memory (operator-maintained, persistent)\n" + notes
	}
	if snap := s.buildSituationSnapshot(); snap != "" {
		sysPrompt += "\n\n" + snap
	}

	// Operator context: which page/agent the human is looking at right now.
	// Tools fall back to this agent when the model omits agent_id, so "dump
	// creds on this machine" works without pasting identifiers.
	reqCtx := &aiReqCtx{}
	if ctxAgent := strings.TrimSpace(req.Context.AgentID); ctxAgent != "" {
		if aid := s.resolveAgentID(ctxAgent); aid != "" {
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

	events := s.converse(model, sysPrompt, userMessages, c.Request.Context(), reqCtx)
	for evt := range events {
		s.writeSSE(c, flusher, evt.Type, evt.Data)
	}
}

// aiMaxNotesLen bounds the persistent engagement memory so a runaway tool
// loop cannot balloon the system prompt.
const aiMaxNotesLen = 8000

// aiMaxContextChars bounds the conversation payload sent to the provider
// (~4 chars/token heuristic). Oldest turns beyond the budget are collapsed.
const aiMaxContextChars = 48000

// trimConversationHistory collapses the oldest turns when the conversation
// grows past the context budget: everything except the system prompt and the
// most recent messages is replaced by a single deterministic summary marker.
// This keeps long engagements usable without an extra summarization API call.
func trimConversationHistory(msgs []chatMessage) []chatMessage {
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
	return aiToolLimits{
		maxConversationTurns:  maxConversationTurns,
		maxToolRounds:         maxToolRounds,
		maxDuplicateToolCalls: maxDuplicateToolCalls,
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
	DefaultAgentID string
}

// converse runs the LLM conversation loop with tool calling, returning SSE events
func (s *Server) converse(model, systemPrompt string, userMessages []chatMessage, ctx context.Context, reqCtx *aiReqCtx) <-chan sseEvent {
	ch := make(chan sseEvent, 10)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("recovered from panic", "err", r, "stack", string(debug.Stack()))
			}
		}()
		defer close(ch)

		var totalUsage aiUsage

		// Send immediate thinking indicator
		ch <- sseEvent{"thinking", ""}

		messages := make([]chatMessage, 0, len(userMessages)+1)
		messages = append(messages, chatMessage{Role: "system", Content: systemPrompt})
		messages = append(messages, userMessages...)
		tools := s.buildTools()

		limits := resolveAIToolLimits(
			s.cfg.AI.MaxConversationTurns,
			s.cfg.AI.MaxToolRounds,
			s.cfg.AI.MaxDuplicateToolCalls,
		)
		toolCallHistory := make(map[string]int) // track tool calls to prevent infinite loops
		consecutiveTools := 0

		for turn := 0; limits.maxConversationTurns == 0 || turn < limits.maxConversationTurns; turn++ {
			// Check if client disconnected
			select {
			case <-ctx.Done():
				return
			default:
			}

			body := chatRequest{
				Model:    model,
				Messages: messages,
				Stream:   true,
				Tools:    tools,
			}
			if turn > 0 {
				body.ToolChoice = "auto"
			}

			payload, ok := marshalJSONSafe(body)
			if !ok {
				ch <- sseEvent{"error", "failed to marshal request"}
				return
			}
			resp, err := s.aiDoRequest(ctx, payload)
			if err != nil {
				ch <- sseEvent{"error", aiFlattenError(err)}
				return
			}

			var toolCalls []toolCall
			var content, finishReason string
			var turnUsage aiUsage
			if s.cfg.AI.Provider == "claude" {
				toolCalls, content, finishReason, turnUsage = s.parseClaudeStream(resp, ch)
			} else {
				toolCalls, content, _, finishReason, turnUsage = s.parseStreamChunks(resp, ch)
			}
			resp.Body.Close()

			// Report per-turn token usage so the operator sees cost accrue.
			totalUsage.PromptTokens += turnUsage.PromptTokens
			totalUsage.CompletionTokens += turnUsage.CompletionTokens
			if ub, ok := marshalJSONSafe(totalUsage); ok {
				ch <- sseEvent{"usage", string(ub)}
			}

			// Safety: cap content length
			if len(content) > AIResponseTruncLen {
				content = content[:AIResponseTruncLen] + "\n\n[Response truncated]"
			}

			if finishReason == "tool_calls" && len(toolCalls) > 0 {
				// Prevent infinite tool loops: same tool+args = skip
				var newCalls []toolCall
				for _, tc := range toolCalls {
					key := tc.Function.Name + ":" + tc.Function.Arguments
					if limits.maxDuplicateToolCalls > 0 && toolCallHistory[key] >= limits.maxDuplicateToolCalls {
						continue
					}
					toolCallHistory[key]++
					newCalls = append(newCalls, tc)
				}
				if len(newCalls) == 0 {
					ch <- sseEvent{"text", "Duplicate tool calls detected, loop terminated."}
					return
				}
				consecutiveTools++
				if limits.maxToolRounds > 0 && consecutiveTools > limits.maxToolRounds {
					ch <- sseEvent{"text", content + "\n\n[Max tool calls reached]"}
					return
				}

				assistMsg := chatMessage{Role: "assistant", Content: content, ToolCalls: newCalls}
				messages = append(messages, assistMsg)
				for _, tc := range newCalls {
					ch <- sseEvent{"tool_start", tc.Function.Name}
					result := s.executeToolCtx(reqCtx, tc.Function.Name, tc.Function.Arguments)
					ch <- sseEvent{"tool", fmt.Sprintf(`{"id":"%s","name":"%s","result":%s}`,
						tc.ID, tc.Function.Name, result)}
					messages = append(messages, chatMessage{
						Role: "tool", ToolCallID: tc.ID, Content: result,
					})
				}
				continue
			}

			// No tool calls 鈥?clear thinking and send content
			ch <- sseEvent{"clear", ""}
			if content != "" {
				ch <- sseEvent{"text", content}
			}
			return
		}
	}()

	return ch
}

// parseStreamChunks reads OpenAI-compatible SSE stream, forwards text/reasoning in real-time,
// and accumulates tool calls. Returns collected tool calls, full content, full reasoning, finish reason, and token usage.
func (s *Server) parseStreamChunks(resp *http.Response, ch chan<- sseEvent) (toolCalls []toolCall, content, reasoning, finishReason string, usage aiUsage) {
	reader := io.Reader(resp.Body)
	buf := make([]byte, AIStreamBufSize)
	var leftover string

	type buildingTool struct {
		Index     int
		ID        string
		Name      string
		Arguments strings.Builder
	}
	var buildingTools []*buildingTool

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			data := leftover + string(buf[:n])
			lines := strings.Split(data, "\n")
			// Last element may be incomplete
			leftover = lines[len(lines)-1]
			lines = lines[:len(lines)-1]

			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" || line == "data: [DONE]" {
					continue
				}
				if !strings.HasPrefix(line, "data: ") {
					continue
				}
				jsonStr := strings.TrimPrefix(line, "data: ")

				var chunk struct {
					Choices []struct {
						Delta struct {
							Content          string `json:"content"`
							ReasoningContent string `json:"reasoning_content"`
							ToolCalls        []struct {
								Index    int    `json:"index"`
								ID       string `json:"id"`
								Type     string `json:"type"`
								Function struct {
									Name      string `json:"name"`
									Arguments string `json:"arguments"`
								} `json:"function"`
							} `json:"tool_calls"`
						} `json:"delta"`
						FinishReason string `json:"finish_reason"`
					} `json:"choices"`
					Usage *struct {
						PromptTokens     int `json:"prompt_tokens"`
						CompletionTokens int `json:"completion_tokens"`
					} `json:"usage"`
				}
				if err := json.Unmarshal([]byte(jsonStr), &chunk); err != nil {
					continue
				}
				if chunk.Usage != nil && (chunk.Usage.PromptTokens > 0 || chunk.Usage.CompletionTokens > 0) {
					usage = aiUsage{
						PromptTokens:     chunk.Usage.PromptTokens,
						CompletionTokens: chunk.Usage.CompletionTokens,
					}
				}
				if len(chunk.Choices) == 0 {
					continue
				}
				delta := chunk.Choices[0].Delta
				fr := chunk.Choices[0].FinishReason

				// Forward reasoning in real-time
				if delta.ReasoningContent != "" {
					reasoning += delta.ReasoningContent
					ch <- sseEvent{"reasoning", delta.ReasoningContent}
				}
				// Forward text in real-time (cap at 8000 chars to prevent runaway generation)
				if delta.Content != "" {
					content += delta.Content
					if len(content) > AIResponseTruncLen {
						content = content[:AIResponseTruncLen] + "\n\n[Response truncated]"
						return
					}
					ch <- sseEvent{"text", content}
				}
				// Accumulate tool calls
				for _, tc := range delta.ToolCalls {
					for len(buildingTools) <= tc.Index {
						buildingTools = append(buildingTools, &buildingTool{Index: len(buildingTools)})
					}
					bt := buildingTools[tc.Index]
					if tc.ID != "" {
						bt.ID = tc.ID
					}
					if tc.Function.Name != "" {
						bt.Name = tc.Function.Name
					}
					bt.Arguments.WriteString(tc.Function.Arguments)
				}

				if fr != "" {
					finishReason = fr
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			ch <- sseEvent{"error", "stream read error"}
			return
		}
	}

	// Convert building tools to tool calls
	for _, bt := range buildingTools {
		if bt.Name != "" {
			toolCalls = append(toolCalls, toolCall{
				ID:   bt.ID,
				Type: "function",
				Function: toolCallFunc{
					Name:      bt.Name,
					Arguments: bt.Arguments.String(),
				},
			})
		}
	}
	return
}

type sseEvent struct {
	Type string
	Data string
}

// aiFlattenError formats an AI request error into a single-line, user-visible
// message. aiDoRequest already wraps useful diagnostics (HTTP status, upstream
// body) in the error, so rather than the generic "AI request failed" we surface
// that detail to the operator's chat view. The API key is never part of these
// errors; any echoed upstream body is trimmed to a read-safe length.
func aiFlattenError(err error) string {
	if err == nil {
		return "AI request failed"
	}
	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		return "AI request failed"
	}
	if len(msg) > AIErrorBodyTruncLen {
		msg = msg[:AIErrorBodyTruncLen] + "..."
	}
	return "AI request failed: " + msg
}

// aiBuildChatURL resolves the final chat endpoint from a configured base URL.
// Providers that use the OpenAI-compatible /chat/completions shape (deepseek,
// openai, qianwen, zhipu, longcat, custom) get the suffix appended; but an
// operator may supply the full chat-completions URL (e.g. OpenRouter examples
// give https://.../api/v1/chat/completions) — appending again would 404, so we
// use it as-is. Claude uses /messages instead.
func aiBuildChatURL(baseURL, provider string) string {
	if provider == "claude" {
		if strings.HasSuffix(baseURL, "/messages") {
			return baseURL
		}
		return baseURL + "/messages"
	}
	if strings.HasSuffix(baseURL, "/chat/completions") {
		return baseURL
	}
	return baseURL + "/chat/completions"
}

func (s *Server) aiDoRequest(ctx context.Context, payload []byte) (*http.Response, error) {
	baseURL := strings.TrimRight(s.cfg.AIEndpoint(), "/")
	hostAndPath := baseURL
	hostAndPath = strings.TrimPrefix(hostAndPath, "https://")
	hostAndPath = strings.TrimPrefix(hostAndPath, "http://")
	if !strings.Contains(hostAndPath, "/") {
		baseURL += "/v1"
	}

	// SSRF guard: resolve the endpoint and reject any loopback/private/
	// link-local/metadata address (incl. DNS-rebinding), matching the outbound
	// fetch protection used elsewhere (S4). Also validates the URL is well-formed.
	if err := validateExternalURL(baseURL); err != nil {
		return nil, fmt.Errorf("AI endpoint blocked: %w", err)
	}

	var urlStr string
	if s.cfg.AI.Provider == "claude" {
		urlStr = aiBuildChatURL(baseURL, "claude")
		payload = s.buildClaudeRequest(payload)
	} else {
		urlStr = aiBuildChatURL(baseURL, "")
	}

	slog.Info("AI API request", "url", urlStr, "model", s.cfg.AI.Model, "provider", s.cfg.AI.Provider)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", urlStr, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if s.cfg.AI.Provider == "claude" {
		httpReq.Header.Set("x-api-key", s.cfg.AI.APIKey)
		httpReq.Header.Set("anthropic-version", "2023-06-01")
	} else {
		httpReq.Header.Set("Authorization", "Bearer "+s.cfg.AI.APIKey)
	}
	if s.cfg.AI.Provider == "deepseek" {
		httpReq.Header.Set("Accept", "application/json")
	}

	backoff := []time.Duration{time.Second, 3 * time.Second, 7 * time.Second}
	var lastErr error
	// ssrfSafeClient re-validates every redirect hop so a public endpoint
	// cannot pivot into internal targets while carrying the API key (S4).
	aiClient := ssrfSafeClient(s.httpClient)
	for attempt := 0; attempt <= aiRetryMax; attempt++ {
		resp, err := aiClient.Do(httpReq)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			if attempt < aiRetryMax {
				slog.Warn("AI API request failed, retrying", "attempt", attempt+1, "error", err)
				time.Sleep(backoff[attempt])
				continue
			}
			return nil, lastErr
		}
		if resp.StatusCode != 200 {
			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr != nil {
				slog.Warn("Failed to read AI error response body", "status", resp.StatusCode, "read_error", readErr)
			}
			bodyStr := string(body)
			if len(bodyStr) > AIErrorBodyTruncLen {
				bodyStr = bodyStr[:AIErrorBodyTruncLen] + "..."
			}
			if attempt < aiRetryMax && (resp.StatusCode == 429 || resp.StatusCode >= 500) {
				slog.Warn("AI API retryable error", "status", resp.StatusCode, "attempt", attempt+1)
				time.Sleep(backoff[attempt])
				continue
			}
			slog.Error("AI API error", "status", resp.StatusCode, "url", urlStr, "body", bodyStr)
			return nil, fmt.Errorf("API %d from %s: %s", resp.StatusCode, urlStr, bodyStr)
		}
		return resp, nil
	}
	return nil, lastErr
}

func (s *Server) writeSSE(c *gin.Context, flusher http.Flusher, event string, data string) {
	fmt.Fprintf(c.Writer, "event: %s\n", event)
	for _, line := range strings.Split(data, "\n") {
		fmt.Fprintf(c.Writer, "data: %s\n", line)
	}
	fmt.Fprintf(c.Writer, "\n")
	if flusher != nil {
		flusher.Flush()
	}
}

// 鈹€鈹€ JSON structures 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€

type chatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type toolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function toolCallFunc `json:"function"`
}

type toolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatRequest struct {
	Model      string        `json:"model"`
	Messages   []chatMessage `json:"messages"`
	Stream     bool          `json:"stream"`
	Tools      []toolDef     `json:"tools,omitempty"`
	ToolChoice interface{}   `json:"tool_choice,omitempty"`
	// Output token cap; 0 = provider default (claude falls back to
	// ClaudeMaxTokens). Set by one-shot helpers, left unset by the
	// streaming conversation loop.
	MaxTokens int `json:"max_tokens,omitempty"`
}

type toolDef struct {
	Type     string      `json:"type"`
	Function toolFuncDef `json:"function"`
}

type toolFuncDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// 鈹€鈹€ Tool Definitions 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€

func (s *Server) buildTools() []toolDef {
	allowExec := s.cfg != nil && s.cfg.AI.AllowExecute
	execDesc := "Execute a command on the specified agent."
	if allowExec {
		execDesc += " When allow_execute is enabled, non-sensitive commands execute immediately (optional wait_for_result up to 60s); sensitive commands (mimikatz/dcsync/secretsdump etc.) always require human approval and return pending_approval."
	} else {
		execDesc += " The command is queued and requires a human operator to approve it before the agent executes it. Returns pending_approval; use get_agent_tasks later to check the result."
	}
	return []toolDef{
		{
			Type: "function",
			Function: toolFuncDef{
				Name:        "list_agents",
				Description: "List all agents, returns ID, hostname, IP, OS, online status",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			Type: "function",
			Function: toolFuncDef{
				Name:        "get_agent_detail",
				Description: "Get agent details including system info, privileges, and task stats",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"agent_id": map[string]string{
							"type":        "string",
							"description": "Agent ID or hostname",
						},
					},
					"required": []string{"agent_id"},
				},
			},
		},
		{
			Type: "function",
			Function: toolFuncDef{
				Name:        "execute_command",
				Description: execDesc,
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"agent_id": map[string]string{
							"type":        "string",
							"description": "Target agent ID or hostname",
						},
						"command": map[string]string{
							"type":        "string",
							"description": "Command to execute (cmd.exe or PowerShell)",
						},
						"shell": map[string]string{
							"type":        "string",
							"description": "Shell type: cmd.exe or powershell.exe",
						},
						"wait_for_result": map[string]interface{}{
							"type":        "boolean",
							"description": "When true and allow_execute is enabled, wait up to 60s for the result. Sensitive commands ignore this flag.",
						},
					},
					"required": []string{"agent_id", "command"},
				},
			},
		},
		{
			Type: "function",
			Function: toolFuncDef{
				Name:        "get_agent_tasks",
				Description: "Get recent task list and results for a specified agent (use when execute_command timed out or wait_for_result was false)",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"agent_id": map[string]string{
							"type":        "string",
							"description": "Agent ID or hostname",
						},
					},
					"required": []string{"agent_id"},
				},
			},
		},
		{
			Type: "function",
			Function: toolFuncDef{
				Name:        "list_listeners",
				Description: "List all configured listeners and their status",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			Type: "function",
			Function: toolFuncDef{
				Name:        "list_credentials",
				Description: "View credential vault summary (without plaintext passwords)",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			Type: "function",
			Function: toolFuncDef{
				Name:        "get_online_operators",
				Description: "View currently online operators",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			Type: "function",
			Function: toolFuncDef{
				Name:        "search_tasks",
				Description: "Search recent tasks by command/result text (e.g. \"mimikatz\", \"360\"). Returns matching tasks with agent, command, status, result snippet.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]string{
							"type":        "string",
							"description": "Search keyword (case-insensitive substring)",
						},
						"limit": map[string]interface{}{
							"type":        "integer",
							"description": "Max results (1-50, default 10)",
						},
					},
					"required": []string{"query"},
				},
			},
		},
		{
			Type: "function",
			Function: toolFuncDef{
				Name:        "get_timeline",
				Description: "Get unified activity timeline for an agent (tasks, screenshots, credentials, status changes) merged by time. Up to 100 most recent events.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"agent_id": map[string]string{
							"type":        "string",
							"description": "Agent ID",
						},
						"limit": map[string]interface{}{
							"type":        "integer",
							"description": "Max events (1-100, default 30)",
						},
					},
					"required": []string{"agent_id"},
				},
			},
		},
		{
			Type: "function",
			Function: toolFuncDef{
				Name:        "query_ioc",
				Description: "Query extracted indicators of compromise (IPs, domains, URLs, file hashes) from recent task results and recon data. Returns top indicators by occurrence.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"days": map[string]interface{}{
							"type":        "integer",
							"description": "Look-back window in days (1-365, default 30)",
						},
						"type": map[string]string{
							"type":        "string",
							"description": "Filter by type: ipv4, domain, url, md5, sha1, sha256 (empty = all)",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: toolFuncDef{
				Name:        "list_macros",
				Description: "List available command macros (recorded command sequences)",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			Type: "function",
			Function: toolFuncDef{
				Name:        "run_macro",
				Description: "Execute a command macro on one or more agents. Each agent runs the macro steps sequentially. Requires allow_execute; otherwise returns pending_approval error.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"macro_id": map[string]interface{}{
							"type":        "integer",
							"description": "Macro ID (preferred)",
						},
						"macro_name": map[string]string{
							"type":        "string",
							"description": "Macro name (alternative to macro_id)",
						},
						"agent_ids": map[string]interface{}{
							"type":        "array",
							"description": "Target agent IDs (max 10)",
							"items":       map[string]string{"type": "string"},
						},
					},
					"required": []string{"agent_ids"},
				},
			},
		},
		{
			Type: "function",
			Function: toolFuncDef{
				Name:        "create_automation_rule",
				Description: "Create an automation rule that triggers on events (agent.checkin, agent.disconnect, task.complete, task.fail, credential.found). The rule's action creates a task or runs a macro automatically. Requires allow_execute; returns the created rule.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]string{
							"type":        "string",
							"description": "Rule name",
						},
						"event_type": map[string]string{
							"type":        "string",
							"description": "Trigger event: agent.checkin, agent.disconnect, task.complete, task.fail, credential.found",
						},
						"command": map[string]string{
							"type":        "string",
							"description": "Shell command to run when triggered (alternative to macro)",
						},
						"macro_id": map[string]interface{}{
							"type":        "integer",
							"description": "Macro ID to run when triggered (alternative to command)",
						},
					},
					"required": []string{"name", "event_type"},
				},
			},
		},
		{
			Type: "function",
			Function: toolFuncDef{
				Name:        "get_attack_surface",
				Description: "Get attack surface for an agent: host profile, credentials nearby, lateral movement history and p2p peers. Use this to recommend the next step in a red-team engagement.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"agent_id": map[string]string{
							"type":        "string",
							"description": "Agent ID or hostname",
						},
					},
					"required": []string{"agent_id"},
				},
			},
		},
		{
			Type: "function",
			Function: toolFuncDef{
				Name:        "get_task_detail",
				Description: "Get the FULL untruncated result/error of one task by ID. Use this to diagnose failed commands or read EDR interception messages that get_agent_tasks truncates.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"task_id": map[string]interface{}{
							"type":        "integer",
							"description": "Task ID",
						},
					},
					"required": []string{"task_id"},
				},
			},
		},
		{
			Type: "function",
			Function: toolFuncDef{
				Name:        "execute_command_bulk",
				Description: "Execute the same command on multiple agents at once (max 20). Without allow_execute every task is queued as pending_approval for human review; with allow_execute non-sensitive commands run immediately. Returns per-agent task ids.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"agent_ids": map[string]interface{}{
							"type":        "array",
							"description": "Target agent IDs or hostnames (max 20)",
							"items":       map[string]string{"type": "string"},
						},
						"command": map[string]string{
							"type":        "string",
							"description": "Command to run on each agent",
						},
						"shell": map[string]string{
							"type":        "string",
							"description": "Shell type: cmd.exe or powershell.exe",
						},
					},
					"required": []string{"agent_ids", "command"},
				},
			},
		},
		{
			Type: "function",
			Function: toolFuncDef{
				Name:        "query_bloodhound",
				Description: "Query the latest BloodHound collection summary (users/computers/groups/sessions/DA counts). Use to answer attack-path questions like 'how do we reach Domain Admin'.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"agent_id": map[string]string{
							"type":        "string",
							"description": "Optional: filter collections by originating agent",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: toolFuncDef{
				Name:        "get_coverage_gaps",
				Description: "Get MITRE ATT&CK coverage gaps: techniques mapped by ForgeC2 but NOT yet exercised in tasks, grouped by tactic. Combine with known host/cred context to recommend next techniques.",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			Type: "function",
			Function: toolFuncDef{
				Name:        "save_engagement_note",
				Description: "Persist a durable engagement note into long-term memory (appended with timestamp). Notes are injected into every future conversation. Use for key findings like 'DC01 allows DCSync from CA-01' or operator constraints. mode=replace overwrites ALL notes.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"note": map[string]string{
							"type":        "string",
							"description": "Note text (keep it short and factual)",
						},
						"mode": map[string]string{
							"type":        "string",
							"description": "\"append\" (default) or \"replace\" (wipes all previous notes)",
						},
					},
					"required": []string{"note"},
				},
			},
		},
		{
			Type: "function",
			Function: toolFuncDef{
				Name:        "list_pending_tasks",
				Description: "List tasks awaiting execution or approval (status pending/pending_approval), optionally filtered by creator. Use this to review the approval queue and recommend which tasks are safe to approve.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"creator": map[string]string{
							"type":        "string",
							"description": "Filter by creator: \"ai\", \"human\" (default: all)",
						},
						"limit": map[string]interface{}{
							"type":        "integer",
							"description": "Max results (1-50, default 20)",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: toolFuncDef{
				Name:        "bulk_task_action",
				Description: "Cancel or approve multiple AI-created pending tasks at once. action=cancel always works for AI-created tasks. action=approve requires allow_execute AND only works on non-sensitive commands - sensitive commands (mimikatz/dcsync etc.) always need a human operator, preserving the two-man rule. Human-created tasks are never touched.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"task_ids": map[string]interface{}{
							"type":        "array",
							"description": "Task IDs to act on (max 50)",
							"items":       map[string]string{"type": "integer"},
						},
						"action": map[string]string{
							"type":        "string",
							"description": "\"approve\" (queue for execution) or \"cancel\" (abort)",
						},
					},
					"required": []string{"task_ids", "action"},
				},
			},
		},
		{
			Type: "function",
			Function: toolFuncDef{
				Name:        "get_screenshot_summary",
				Description: "Summarize screenshot captures within a look-back window: total count, per-agent breakdown, latest captures. Read-only analysis of collection activity.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"agent_id": map[string]string{
							"type":        "string",
							"description": "Optional: filter by agent ID or hostname",
						},
						"days": map[string]interface{}{
							"type":        "integer",
							"description": "Look-back window in days (1-365, default 30)",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: toolFuncDef{
				Name:        "get_keylog_summary",
				Description: "Summarize keylogger dumps within a look-back window: dump counts per agent and high-value entries (passwords, logins, emails detected via keyword scan of captured keystrokes). Read-only.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"agent_id": map[string]string{
							"type":        "string",
							"description": "Optional: filter by agent ID or hostname",
						},
						"days": map[string]interface{}{
							"type":        "integer",
							"description": "Look-back window in days (1-365, default 30)",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: toolFuncDef{
				Name:        "generate_report",
				Description: "Generate a structured engagement report from live data (agents, tasks, credentials, listeners, IOCs, MITRE coverage) and save it to the report library. Scopes: full (everything), executive (summary+findings+recommendations), technical (agents/tasks/creds/network/iocs), coverage (MITRE gaps analysis). Returns the report id; operators view/export it on the Report page.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"scope": map[string]string{
							"type":        "string",
							"description": "full | executive | technical | coverage (default full)",
						},
						"title": map[string]string{
							"type":        "string",
							"description": "Optional report title (default auto-generated with date)",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: toolFuncDef{
				Name:        "create_listener",
				Description: "Create and bind a new C2 listener. Requires allow_execute. Schemes: http, https, tcp, tls, dns, icmp, ssh, h2c, udp, quic. DNS listeners use dns_domain instead of port; ICMP uses icmp_addr. The returned status reflects whether the bind actually succeeded.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]string{
							"type":        "string",
							"description": "Listener name (must be unique)",
						},
						"scheme": map[string]string{
							"type":        "string",
							"description": "http | https | tcp | tls | dns | icmp | ssh | h2c | udp | quic",
						},
						"host": map[string]string{
							"type":        "string",
							"description": "Bind host/IP (e.g. 0.0.0.0)",
						},
						"port": map[string]interface{}{
							"type":        "integer",
							"description": "Bind port 1-65535 (0 for dns/icmp)",
						},
						"dns_domain": map[string]string{
							"type":        "string",
							"description": "DNS domain for dns-scheme listeners",
						},
						"icmp_addr": map[string]string{
							"type":        "string",
							"description": "Listen address for icmp-scheme listeners",
						},
						"notes": map[string]string{
							"type":        "string",
							"description": "Optional notes",
						},
					},
					"required": []string{"scheme"},
				},
			},
		},
		{
			Type: "function",
			Function: toolFuncDef{
				Name:        "update_listener",
				Description: "Update an existing listener: toggle enabled, change bind host/port, or rename. Requires allow_execute. Bind-address changes restart the listener.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"listener_id": map[string]interface{}{
							"type":        "integer",
							"description": "Listener ID",
						},
						"enabled": map[string]interface{}{
							"type":        "boolean",
							"description": "Enable or disable the listener",
						},
						"name": map[string]string{
							"type":        "string",
							"description": "New name",
						},
						"host": map[string]string{
							"type":        "string",
							"description": "New bind host/IP",
						},
						"port": map[string]interface{}{
							"type":        "integer",
							"description": "New bind port 1-65535",
						},
					},
					"required": []string{"listener_id"},
				},
			},
		},
		{
			Type: "function",
			Function: toolFuncDef{
				Name:        "delete_listener",
				Description: "Delete a listener permanently. Requires allow_execute. Refuses if any agent still references the listener.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"listener_id": map[string]interface{}{
							"type":        "integer",
							"description": "Listener ID",
						},
					},
					"required": []string{"listener_id"},
				},
			},
		},
	}
}

// 鈹€鈹€ Tool Execution 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€

func (s *Server) executeToolCtx(reqCtx *aiReqCtx, name string, argsJSON string) string {
	// Tolerant pre-parse: schema-conformant tools carry arrays/numbers/bools
	// (agent_ids, task_ids, port, days...) which cannot decode into
	// map[string]string. encoding/json still fills every string-valued key
	// before reporting the type error, so we ignore the error here — each
	// tool case below re-parses argsJSON into its own typed struct anyway.
	var args map[string]string
	_ = json.Unmarshal([]byte(argsJSON), &args)
	if reqCtx == nil {
		reqCtx = &aiReqCtx{}
	}
	// Context fallback: tools that accept agent_id may omit it when the
	// operator is already focused on an agent in the console. Array-valued
	// keys never land in the partial map; injectDefaultAgent re-parses the
	// full JSON and only injects when the real key is absent/empty.
	if reqCtx.DefaultAgentID != "" {
		for _, key := range []string{"agent_id", "agent_ids"} {
			raw, exists := args[key]
			if !exists || strings.TrimSpace(raw) == "" || raw == "null" || raw == `[]` {
				continue
			}
			return s.executeToolSwitch(name, argsJSON)
		}
		return s.executeToolWithDefaultAgent(name, argsJSON, reqCtx.DefaultAgentID)
	}
	return s.executeToolSwitch(name, argsJSON)
}

// injectDefaultAgent returns argsJSON with the context agent filled in for
// agent-scoped tools (missing/empty agent_id, missing agent_ids for macros).
// Pure so it stays unit-testable without touching tools or the database.
func injectDefaultAgent(name string, argsJSON string, defaultAgent string) string {
	switch name {
	case "execute_command", "get_agent_tasks", "get_timeline", "get_attack_surface",
		"get_agent_detail", "query_bloodhound":
		var orig map[string]interface{}
		if err := json.Unmarshal([]byte(argsJSON), &orig); err == nil {
			cur, _ := orig["agent_id"].(string)
			if strings.TrimSpace(cur) == "" {
				orig["agent_id"] = defaultAgent
				if b, ok := marshalJSONSafe(orig); ok {
					return string(b)
				}
			}
		}
	case "run_macro", "execute_command_bulk":
		// Both read an agent_ids ARRAY; a bare string default would be ignored.
		var orig map[string]interface{}
		if err := json.Unmarshal([]byte(argsJSON), &orig); err == nil {
			if _, has := orig["agent_ids"]; !has {
				orig["agent_ids"] = []string{defaultAgent}
				if b, ok := marshalJSONSafe(orig); ok {
					return string(b)
				}
			}
		}
	}
	return argsJSON
}

// executeToolWithDefaultAgent injects the context agent into the args JSON for
// the agent-scoped tools before dispatching.
func (s *Server) executeToolWithDefaultAgent(name string, argsJSON string, defaultAgent string) string {
	return s.executeToolSwitch(name, injectDefaultAgent(name, argsJSON, defaultAgent))
}

func (s *Server) executeToolSwitch(name string, argsJSON string) string {
	// Tolerant pre-parse (see executeToolCtx): string-valued keys still land
	// in the map even when array/number keys make Unmarshal return an error.
	// Tools that need non-string values re-parse argsJSON themselves.
	var args map[string]string
	_ = json.Unmarshal([]byte(argsJSON), &args)

	switch name {
	case "list_agents":
		var agents []db.Implant
		if err := s.db.Order("last_seen desc").Limit(50).Find(&agents).Error; err != nil {
			slog.Error("AI: failed to list agents", "err", err)
		}
		var out []map[string]interface{}
		for _, a := range agents {
			out = append(out, map[string]interface{}{
				"id": a.ID, "hostname": a.Hostname, "ip": a.IP,
				"os": a.OS, "username": a.Username, "status": a.Status,
				"last_seen": a.LastSeen.Format(time.RFC3339),
			})
		}
		b, ok := marshalJSONSafe(out)
		if !ok {
			return `{"error":"failed to marshal agents"}`
		}
		return string(b)

	case "get_agent_detail":
		aid := s.resolveAgentID(args["agent_id"])
		if aid == "" {
			return `{"error":"agent not found"}`
		}
		var agent db.Implant
		if err := s.db.Where("id = ?", aid).First(&agent).Error; err != nil {
			return `{"error":"agent not found"}`
		}
		var taskCount int64
		if err := s.db.Model(&db.Task{}).Where("agent_id = ?", agent.ID).Count(&taskCount).Error; err != nil {
			slog.Error("Failed to count agent tasks", "err", err)
		}
		type detail struct {
			ID, Hostname, IP, OS, Arch, Username, Domain, Status string
			Integrity                                            string
			PID                                                  int
			Elevated                                             bool
			TaskCount                                            int64
		}
		d := detail{agent.ID, agent.Hostname, agent.IP, agent.OS, agent.Arch, agent.Username, agent.Domain, agent.Status, agent.Integrity, agent.PID, agent.Elevated, taskCount}
		b, ok := marshalJSONSafe(d)
		if !ok {
			return `{"error":"failed to marshal agent detail"}`
		}
		return string(b)

	case "execute_command":
		ecArgs := parseExecuteCommandArgs(argsJSON)
		aid := s.resolveAgentID(ecArgs.AgentID)
		if aid == "" {
			return `{"error":"agent not found"}`
		}
		shell := ecArgs.Shell
		if shell == "" {
			shell = "cmd.exe"
		}
		allowExec := s.cfg != nil && s.cfg.AI.AllowExecute
		sensitive := isSensitiveCommand(ecArgs.Command)
		status := TaskStatusPendingApproval
		// allow_execute is a SUBSET gate: server-wide RequireApproval for
		// approval-flagged task types (resolveInitialTaskStatus) still wins,
		// so ai.allow_execute can never bypass the operator two-man rule.
		if allowExec && !sensitive && s.resolveInitialTaskStatus("shell") == "pending" {
			status = "pending"
		}
		// Enforce the per-agent pending cap + counter parity with the
		// manual dispatch path (dec happens on claim/cancel/reject).
		if err := s.trackPendingTask(aid); err != nil {
			b, _ := marshalJSONSafe(map[string]interface{}{"error": sanitizeError(err, "task")})
			return string(b)
		}
		task := db.Task{
			AgentID: aid, Type: "shell", Command: ecArgs.Command,
			Shell: shell, Status: status, CreatedBy: "ai",
		}
		if err := s.db.Create(&task).Error; err != nil {
			s.decPendingTasks(aid)
			return `{"error":"failed to create task"}`
		}
		if status == "pending" {
			s.broadcastTaskUpdate(aid, task)
		}
		if sensitive && allowExec {
			b, ok := marshalJSONSafe(map[string]interface{}{
				"task_id": task.ID,
				"status":  "pending_approval",
				"message": "Sensitive command requires human approval even with allow_execute enabled. Approve it in the Tasks page.",
				"reason":  "sensitive_command",
			})
			if !ok {
				return `{"error":"failed to marshal task result"}`
			}
			return string(b)
		}
		if status == "pending" && ecArgs.WaitForResult {
			return s.waitForTaskResult(task.ID, aid)
		}
		if status == TaskStatusPendingApproval {
			b, ok := marshalJSONSafe(map[string]interface{}{
				"task_id": task.ID,
				"status":  "pending_approval",
				"message": "Command created but requires operator approval before the agent executes it. Approve it in the Tasks page.",
			})
			if !ok {
				return `{"error":"failed to marshal task result"}`
			}
			return string(b)
		}
		b, ok := marshalJSONSafe(map[string]interface{}{
			"task_id": task.ID,
			"status":  status,
			"message": "Command queued for execution.",
		})
		if !ok {
			return `{"error":"failed to marshal task result"}`
		}
		return string(b)

	case "get_agent_tasks":
		aid := s.resolveAgentID(args["agent_id"])
		if aid == "" {
			return `{"error":"agent not found"}`
		}
		var tasks []db.Task
		if err := s.db.Where("agent_id = ?", aid).Order("created_at desc").Limit(10).Find(&tasks).Error; err != nil {
			slog.Error("AI: failed to query agent tasks", "err", err)
		}
		var out []map[string]interface{}
		for _, t := range tasks {
			r := map[string]interface{}{
				"id": t.ID, "type": t.Type, "command": t.Command,
				"status": t.Status, "created_at": t.CreatedAt.Format(time.RFC3339),
			}
			if t.Result != "" {
				r["result"] = truncateStr(t.Result, AIToolResultTruncLen)
			}
			if t.Error != "" {
				r["error"] = t.Error
			}
			out = append(out, r)
		}
		b, ok := marshalJSONSafe(out)
		if !ok {
			return `{"error":"failed to marshal tasks"}`
		}
		return string(b)

	case "list_listeners":
		var listeners []db.Listener
		if err := s.db.Order("created_at desc").Limit(500).Find(&listeners).Error; err != nil {
			slog.Error("AI: failed to list listeners", "err", err)
		}
		var out []map[string]interface{}
		for _, l := range listeners {
			out = append(out, map[string]interface{}{
				"id": l.ID, "name": l.Name, "type": l.Type,
				"host": l.Host, "port": l.Port, "enabled": l.Enabled,
			})
		}
		b, ok := marshalJSONSafe(out)
		if !ok {
			return `{"error":"failed to marshal listeners"}`
		}
		return string(b)

	case "list_credentials":
		var creds []db.CredentialEntry
		if err := s.db.Order("created_at desc").Limit(100).Find(&creds).Error; err != nil {
			slog.Error("AI: failed to list credentials", "err", err)
		}
		var out []map[string]interface{}
		for _, c := range creds {
			entry := map[string]interface{}{
				"id": c.ID, "domain": c.Domain, "username": c.Username,
				"type": c.Type, "source": c.Source, "has_password": c.Password != "",
				"has_hash": c.Hash != "",
			}
			out = append(out, entry)
		}
		b, ok := marshalJSONSafe(out)
		if !ok {
			return `{"error":"failed to marshal credentials"}`
		}
		return string(b)

	case "get_online_operators":
		b, ok := marshalJSONSafe([]map[string]string{})
		if !ok {
			return `{"error":"failed to marshal operators"}`
		}
		return string(b)

	case "search_tasks":
		var q struct {
			Query string `json:"query"`
			Limit int    `json:"limit"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &q)
		if q.Query == "" {
			q.Query = args["query"]
		}
		if q.Limit <= 0 || q.Limit > 50 {
			q.Limit = 10
		}
		like := "%" + q.Query + "%"
		var tasks []db.Task
		if err := s.db.Where("command LIKE ? OR result LIKE ?", like, like).Order("created_at desc").Limit(q.Limit).Find(&tasks).Error; err != nil {
			return `{"error":"failed to search tasks"}`
		}
		var out []map[string]interface{}
		for _, t := range tasks {
			out = append(out, map[string]interface{}{
				"id": t.ID, "agent_id": t.AgentID, "type": t.Type, "command": truncateStr(t.Command, 200),
				"status": t.Status, "result": truncateStr(t.Result, 500),
			})
		}
		b, ok := marshalJSONSafe(out)
		if !ok {
			return `{"error":"failed to marshal tasks"}`
		}
		return string(b)

	case "get_timeline":
		var p struct {
			AgentID string `json:"agent_id"`
			Limit   int    `json:"limit"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &p)
		if p.AgentID == "" {
			p.AgentID = args["agent_id"]
		}
		if p.AgentID == "" {
			return `{"error":"agent_id required"}`
		}
		if p.Limit <= 0 || p.Limit > 100 {
			p.Limit = 30
		}
		aid2 := s.resolveAgentID(p.AgentID)
		if aid2 == "" {
			return `{"error":"agent not found"}`
		}
		// tasks + recent creds + status events (screenshots are filesystem, skip)
		var tasks2 []db.Task
		s.db.Where("agent_id = ?", aid2).Order("created_at desc").Limit(p.Limit).Find(&tasks2)
		var creds []db.CredentialEntry
		s.db.Where("agent_id = ?", aid2).Order("created_at desc").Limit(10).Find(&creds)
		var events []map[string]interface{}
		for _, t := range tasks2 {
			events = append(events, map[string]interface{}{"kind": "task", "type": t.Type, "command": truncateStr(t.Command, 120), "status": t.Status, "time": t.CreatedAt.Format(time.RFC3339), "result": truncateStr(t.Result, 300)})
		}
		for _, c := range creds {
			events = append(events, map[string]interface{}{"kind": "credential", "username": c.Username, "domain": c.Domain, "type": c.Type, "time": c.CreatedAt.Format(time.RFC3339)})
		}
		b, ok := marshalJSONSafe(events)
		if !ok {
			return `{"error":"failed to marshal timeline"}`
		}
		return string(b)

	case "query_ioc":
		var p struct {
			Days int    `json:"days"`
			Type string `json:"type"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &p)
		if p.Days <= 0 || p.Days > 365 {
			p.Days = 30
		}
		entries, _ := s.extractIOCs(p.Days, false)
		var filtered []iocEntry
		for _, e := range entries {
			if p.Type != "" && e.Type != p.Type {
				continue
			}
			filtered = append(filtered, e)
			if len(filtered) >= 20 {
				break
			}
		}
		b, ok := marshalJSONSafe(filtered)
		if !ok {
			return `{"error":"failed to marshal iocs"}`
		}
		return string(b)

	case "list_macros":
		var macros []db.CommandMacro
		s.db.Order("name").Find(&macros)
		var out []map[string]interface{}
		for _, m := range macros {
			var steps []interface{}
			_ = json.Unmarshal([]byte(m.Steps), &steps)
			out = append(out, map[string]interface{}{"id": m.ID, "name": m.Name, "description": m.Description, "steps": len(steps)})
		}
		b, ok := marshalJSONSafe(out)
		if !ok {
			return `{"error":"failed to marshal macros"}`
		}
		return string(b)

	case "run_macro":
		if s.cfg == nil || !s.cfg.AI.AllowExecute {
			return `{"error":"allow_execute is disabled in AI config; enable it to run macros"}`
		}
		var p struct {
			MacroID   *uint   `json:"macro_id"`
			MacroName string  `json:"macro_name"`
			AgentIDs  []string `json:"agent_ids"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &p)
		if len(p.AgentIDs) == 0 {
			return `{"error":"agent_ids required (max 10)"}`
		}
		if len(p.AgentIDs) > 10 {
			p.AgentIDs = p.AgentIDs[:10]
		}
		var macro db.CommandMacro
		if p.MacroID != nil && *p.MacroID != 0 {
			if err := s.db.First(&macro, *p.MacroID).Error; err != nil {
				return `{"error":"macro not found"}`
			}
		} else if p.MacroName != "" {
			if err := s.db.Where("name = ?", p.MacroName).First(&macro).Error; err != nil {
				return `{"error":"macro not found by name"}`
			}
		} else {
			return `{"error":"macro_id or macro_name required"}`
		}
		dispatched := 0
		for _, aid := range p.AgentIDs {
			resolved := s.resolveAgentID(aid)
			if resolved == "" {
				continue
			}
			if err := s.startMacroRun(&macro, resolved, "ai", true); err == nil {
				dispatched++
			}
		}
		b, ok := marshalJSONSafe(map[string]interface{}{"dispatched": dispatched, "macro": macro.Name})
		if !ok {
			return `{"error":"failed to marshal result"}`
		}
		return string(b)

	case "create_automation_rule":
		if s.cfg == nil || !s.cfg.AI.AllowExecute {
			return `{"error":"allow_execute is disabled; enable it to create automation rules"}`
		}
		var p struct {
			Name      string `json:"name"`
			EventType string `json:"event_type"`
			Command   string `json:"command"`
			MacroID   *uint  `json:"macro_id"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &p)
		if p.Name == "" || p.EventType == "" {
			return `{"error":"name and event_type required (event_type: agent.checkin, agent.disconnect, task.complete, task.fail, credential.found)"}`
		}
		validEvents := map[string]bool{"agent.checkin": true, "agent.disconnect": true, "task.complete": true, "task.fail": true, "credential.found": true}
		if !validEvents[p.EventType] {
			return `{"error":"invalid event_type"}`
		}
		var actionType, params string
		if p.MacroID != nil && *p.MacroID != 0 {
			b2, _ := json.Marshal(map[string]interface{}{"macro_id": *p.MacroID, "stop_on_error": true})
			actionType = "run_macro"
			params = string(b2)
		} else if p.Command != "" {
			b2, _ := json.Marshal(map[string]string{"command": p.Command})
			actionType = "command"
			params = string(b2)
		} else {
			return `{"error":"command or macro_id required"}`
		}
		rule := db.AutomationRule{
			ID:        fmt.Sprintf("ai-%d", time.Now().UnixNano()),
			Name:      p.Name,
			EventType: p.EventType,
			Conditions: "[]",
			Actions:   fmt.Sprintf(`[{"type":"%s","params":%s}]`, actionType, params),
			Enabled:   true,
			CreatedBy: "ai",
		}
		if err := s.db.Create(&rule).Error; err != nil {
			return `{"error":"failed to create rule"}`
		}
		s.invalidateAutomationCache()
		b, ok := marshalJSONSafe(map[string]interface{}{"id": rule.ID, "name": rule.Name, "event_type": rule.EventType})
		if !ok {
			return `{"error":"failed to marshal rule"}`
		}
		return string(b)

	case "get_attack_surface":
		var p struct {
			AgentID string `json:"agent_id"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &p)
		if p.AgentID == "" {
			p.AgentID = args["agent_id"]
		}
		if p.AgentID == "" {
			return `{"error":"agent_id required"}`
		}
		aid := s.resolveAgentID(p.AgentID)
		if aid == "" {
			return `{"error":"agent not found"}`
		}
		var ag db.Implant
		if err := s.db.Where("id = ?", aid).First(&ag).Error; err != nil {
			return `{"error":"agent not found"}`
		}
		var credCount int64
		s.db.Model(&db.CredentialEntry{}).Where("agent_id = ?", aid).Count(&credCount)
		var recentTasks []db.Task
		s.db.Where("agent_id = ?", aid).Order("created_at desc").Limit(5).Find(&recentTasks)
		var recentLateral []db.Task
		s.db.Where("agent_id = ? AND type IN ?", aid, []string{"lateral", "ssh_lateral", "token_steal", "token_make"}).Order("created_at desc").Limit(5).Find(&recentLateral)
		surface := map[string]interface{}{
			"agent": map[string]interface{}{
				"id": aid, "hostname": ag.Hostname, "ip": ag.IP, "os": ag.OS, "arch": ag.Arch,
				"username": ag.Username, "domain": ag.Domain, "integrity": ag.Integrity,
				"elevated": ag.Elevated, "pid": ag.PID, "status": ag.Status, "version": ag.Version,
			},
			"credentials_nearby": credCount,
			"recent_tasks": recentTasks,
			"recent_lateral": recentLateral,
			"hint": "Use this surface to recommend the next step (e.g., lateral movement, cred dumping, persistence). Consider OS, privilege level, and nearby creds.",
		}
		b, ok := marshalJSONSafe(surface)
		if !ok {
			return `{"error":"failed to marshal surface"}`
		}
		return string(b)

	case "get_task_detail":
		var p struct {
			TaskID uint `json:"task_id"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &p)
		if p.TaskID == 0 {
			if v, err := strconv.ParseUint(args["task_id"], 10, 64); err == nil {
				p.TaskID = uint(v)
			}
		}
		if p.TaskID == 0 {
			return `{"error":"numeric task_id required"}`
		}
		var task db.Task
		if err := s.db.First(&task, p.TaskID).Error; err != nil {
			return `{"error":"task not found"}`
		}
		out := map[string]interface{}{
			"id": task.ID, "agent_id": task.AgentID, "type": task.Type,
			"command": task.Command, "status": task.Status,
			"created_at": task.CreatedAt.Format(time.RFC3339),
			"created_by": task.CreatedBy,
		}
		// Full result/error (context-bounded): this is the diagnosis tool —
		// truncation here defeats its purpose.
		const maxDetail = 16 * 1024
		if task.Result != "" {
			out["result"] = truncateStr(task.Result, maxDetail)
		}
		if task.Error != "" {
			out["error"] = truncateStr(task.Error, maxDetail)
		}
		b, ok := marshalJSONSafe(out)
		if !ok {
			return `{"error":"failed to marshal task"}`
		}
		return string(b)

	case "execute_command_bulk":
		var p struct {
			AgentIDs []string `json:"agent_ids"`
			Command  string   `json:"command"`
			Shell    string   `json:"shell"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &p)
		if len(p.AgentIDs) == 0 || strings.TrimSpace(p.Command) == "" {
			return `{"error":"agent_ids and command required (max 20 agents)"}`
		}
		if len(p.AgentIDs) > 20 {
			p.AgentIDs = p.AgentIDs[:20]
		}
		allowExec := s.cfg != nil && s.cfg.AI.AllowExecute
		sensitive := isSensitiveCommand(p.Command)
		status := "pending_approval"
		// Same subset-gate as execute_command: server RequireApproval wins.
		if allowExec && !sensitive && s.resolveInitialTaskStatus("shell") == "pending" {
			status = "pending"
		}
		if p.Shell == "" {
			p.Shell = "cmd.exe"
		}
		type bulkResult struct {
			AgentID string `json:"agent_id"`
			TaskID  uint   `json:"task_id,omitempty"`
			Status  string `json:"status,omitempty"`
			Error   string `json:"error,omitempty"`
		}
		var results []bulkResult
		for _, aid := range p.AgentIDs {
			resolved := s.resolveAgentID(aid)
			if resolved == "" {
				results = append(results, bulkResult{AgentID: aid, Error: "agent not found"})
				continue
			}
			if err := s.trackPendingTask(resolved); err != nil {
				results = append(results, bulkResult{AgentID: resolved, Error: sanitizeError(err, "task")})
				continue
			}
			task := db.Task{
				AgentID: resolved, Type: "shell", Command: p.Command,
				Shell: p.Shell, Status: status, CreatedBy: "ai",
			}
			if err := s.db.Create(&task).Error; err != nil {
				s.decPendingTasks(resolved)
				results = append(results, bulkResult{AgentID: resolved, Error: "create failed"})
				continue
			}
			if status == "pending" {
				s.broadcastTaskUpdate(resolved, task)
			}
			results = append(results, bulkResult{AgentID: resolved, TaskID: task.ID, Status: status})
		}
		b, ok := marshalJSONSafe(map[string]interface{}{
			"status":            status,
			"sensitive":         sensitive,
			"results":           results,
			"pending_tasks_url": "/tasks",
		})
		if !ok {
			return `{"error":"failed to marshal results"}`
		}
		return string(b)

	case "query_bloodhound":
		var p struct {
			AgentID string `json:"agent_id"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &p)
		q := s.db.Model(&db.BloodHoundResult{})
		if p.AgentID != "" {
			q = q.Where("agent_id = ?", s.resolveAgentID(p.AgentID))
		}
		var bh db.BloodHoundResult
		if err := q.Order("id desc").First(&bh).Error; err != nil {
			return `{"error":"no bloodhound collections found. Run a collection from the BloodHound page first."}`
		}
		b, ok := marshalJSONSafe(map[string]interface{}{
			"id":                  bh.ID,
			"agent_id":            bh.AgentID,
			"collection_method":   bh.CollectionMethod,
			"summary":             truncateStr(bh.Summary, AIToolResultTruncLen * 2),
			"user_count":          bh.UserCount,
			"computer_count":      bh.ComputerCount,
			"group_count":         bh.GroupCount,
			"session_count":       bh.SessionCount,
			"domain_admin_count":  bh.DomainAdminCount,
		})
		if !ok {
			return `{"error":"failed to marshal bloodhound data"}`
		}
		return string(b)

	case "get_coverage_gaps":
		usedTypes := map[string]bool{}
		var types []string
		if err := s.db.Table("tasks").Distinct().Pluck("type", &types).Error; err != nil {
			types = nil
		}
		for _, t := range types {
			usedTypes[t] = true
		}
		type gapTactic struct {
			Tactic     string   `json:"tactic"`
			Techniques []string `json:"techniques"`
		}
		var covered, gaps []gapTactic
		gapIndex := map[string]int{}
		for _, grp := range attackTacticMap {
			for _, tech := range grp.Techniques {
				isCovered := false
				for _, tt := range tech.TaskTypes {
					if usedTypes[tt] {
						isCovered = true
						break
					}
				}
				entry := fmt.Sprintf("%s %s (%s)", tech.ID, tech.Name, strings.Join(tech.TaskTypes, ","))
				idx := -1
				target := &covered
				if !isCovered {
					if gi, ok := gapIndex[grp.Tactic]; ok {
						idx = gi
					} else {
						gaps = append(gaps, gapTactic{Tactic: grp.Tactic})
						idx = len(gaps) - 1
						gapIndex[grp.Tactic] = idx
					}
					target = &gaps
				} else {
					found := false
					for i := range covered {
						if covered[i].Tactic == grp.Tactic {
							idx = i
							found = true
							break
						}
					}
					if !found {
						covered = append(covered, gapTactic{Tactic: grp.Tactic})
						idx = len(covered) - 1
					}
				}
				(*target)[idx].Techniques = append((*target)[idx].Techniques, entry)
			}
		}
		b, ok := marshalJSONSafe(map[string]interface{}{
			"covered_tactics": covered,
			"gaps_by_tactic":  gaps,
			"used_task_types": types,
			"hint":            "Recommend gaps that complement existing access: with creds but no persistence, suggest persistence; with discovery but no collection, suggest screen/keylog capture.",
		})
		if !ok {
			return `{"error":"failed to marshal coverage"}`
		}
		return string(b)

	case "save_engagement_note":
		var p struct {
			Note string `json:"note"`
			Mode string `json:"mode"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &p)
		note := strings.TrimSpace(p.Note)
		if note == "" {
			return `{"error":"note required"}`
		}
		s.configMu.Lock()
		current := s.cfg.AI.EngagementNotes
		if p.Mode == "replace" {
			if len(note) > aiMaxNotesLen {
				s.configMu.Unlock()
				return `{"error":"note too large (max 8000 chars in replace mode)"}`
			}
			s.cfg.AI.EngagementNotes = note
		} else {
			stamp := time.Now().UTC().Format("2006-01-02")
			addition := fmt.Sprintf("\n- [%s] %s", stamp, note)
			if len(current)+len(addition) > aiMaxNotesLen {
				s.configMu.Unlock()
				return fmt.Sprintf(`{"error":"engagement memory full (%d chars). Use mode=replace or prune via AI settings."}`, aiMaxNotesLen)
			}
			s.cfg.AI.EngagementNotes = current + addition
		}
		s.configMu.Unlock()
		configPath := s.configPath
		if configPath == "" {
			configPath = "config.yaml"
		}
		if err := s.cfg.Save(configPath); err != nil {
			return `{"error":"failed to persist note: ` + sanitizeError(err, "AI operation") + `"}`
		}
		b, ok := marshalJSONSafe(map[string]interface{}{
			"saved": true, "mode": p.Mode, "total_chars": len(s.cfg.AI.EngagementNotes),
		})
		if !ok {
			return `{"error":"failed to marshal result"}`
		}
		return string(b)

	case "list_pending_tasks":
		var p struct {
			Creator string `json:"creator"`
			Limit   int    `json:"limit"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &p)
		if p.Limit <= 0 || p.Limit > 50 {
			p.Limit = 20
		}
		q := s.db.Where("status IN ?", []string{"pending", TaskStatusPendingApproval})
		switch p.Creator {
		case "ai":
			q = q.Where("created_by = ?", "ai")
		case "human":
			q = q.Where("created_by NOT IN ?", []string{"ai", "automation", "system"})
		}
		var tasks []db.Task
		q.Order("created_at desc").Limit(p.Limit).Find(&tasks)
		var out []map[string]interface{}
		for _, t := range tasks {
			out = append(out, map[string]interface{}{
				"id": t.ID, "agent_id": t.AgentID, "type": t.Type,
				"command":    truncateStr(t.Command, 200),
				"status":     t.Status,
				"created_by": t.CreatedBy,
				"sensitive":  isSensitiveCommand(t.Command),
				"created_at": t.CreatedAt.Format(time.RFC3339),
			})
		}
		b, ok := marshalJSONSafe(out)
		if !ok {
			return `{"error":"failed to marshal tasks"}`
		}
		return string(b)

	case "bulk_task_action":
		var p struct {
			TaskIDs []uint `json:"task_ids"`
			Action  string `json:"action"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &p)
		if len(p.TaskIDs) == 0 {
			return `{"error":"task_ids required (max 50)"}`
		}
		if len(p.TaskIDs) > 50 {
			p.TaskIDs = p.TaskIDs[:50]
		}
		if p.Action != "approve" && p.Action != "cancel" {
			return `{"error":"action must be \"approve\" or \"cancel\""}`
		}
		allowExec := s.cfg != nil && s.cfg.AI.AllowExecute
		var approved, cancelled, skippedHuman, skippedSensitive, skippedOther int
		var audit []auditEntry
		type bulkErr struct {
			TaskID uint   `json:"task_id"`
			Error  string `json:"error"`
		}
		var errs []bulkErr
		for _, tid := range p.TaskIDs {
			var task db.Task
			if err := s.db.First(&task, tid).Error; err != nil {
				errs = append(errs, bulkErr{TaskID: tid, Error: "not found"})
				continue
			}
			if task.Status != "pending" && task.Status != TaskStatusPendingApproval {
				skippedOther++
				continue
			}
			// The AI assistant may only act on its own tasks. Human-created
			// tasks are untouchable here — that preserves the two-man rule.
			if task.CreatedBy != "ai" {
				skippedHuman++
				continue
			}
			if p.Action == "cancel" {
				res := s.db.Model(&db.Task{}).
					Where("id = ? AND status IN ?", tid, []string{"pending", TaskStatusPendingApproval}).
					Updates(map[string]interface{}{"status": "cancelled", "error": "cancelled by AI assistant"})
				if res.Error != nil || res.RowsAffected == 0 {
					errs = append(errs, bulkErr{TaskID: tid, Error: "state changed concurrently"})
					continue
				}
				s.decPendingTasks(task.AgentID)
				cancelled++
				audit = append(audit, auditEntry{
					action: "ai_bulk_cancel_task", resource: "agent_id", agentID: task.AgentID,
					details: fmt.Sprintf("actor=ai Cancelled AI task #%d (%s)", tid, task.Type), success: true,
				})
				fresh := task
				fresh.Status = "cancelled"
				s.broadcastTaskUpdate(task.AgentID, fresh)
			} else { // approve
				// Sensitive commands can NEVER be auto-approved by the AI,
				// even with allow_execute — a human must press the button.
				if !allowExec {
					skippedOther++
					continue
				}
				if isSensitiveCommand(task.Command) {
					skippedSensitive++
					continue
				}
				res := s.db.Model(&db.Task{}).
					Where("id = ? AND status = ?", tid, TaskStatusPendingApproval).
					Updates(map[string]interface{}{"status": "pending", "approved_by": "ai", "approved_at": time.Now()})
				if res.Error != nil || res.RowsAffected == 0 {
					errs = append(errs, bulkErr{TaskID: tid, Error: "state changed concurrently"})
					continue
				}
				approved++
				audit = append(audit, auditEntry{
					action: "ai_bulk_approve_task", resource: "agent_id", agentID: task.AgentID,
					details: fmt.Sprintf("actor=ai Approved AI task #%d (%s)", tid, task.Type), success: true,
				})
				fresh := task
				fresh.Status = "pending"
				s.broadcastTaskUpdate(task.AgentID, fresh)
			}
		}
		// Chain-safe audit trail for every applied bulk action (nil context
		// records actor as system; details carry the ai attribution).
		if len(audit) > 0 {
			s.LogAuditRecords(nil, audit)
		}
		result := map[string]interface{}{
			"action":          p.Action,
			"approved":        approved,
			"cancelled":       cancelled,
			"skipped_human":   skippedHuman,
			"skipped_other":   skippedOther,
			"sensitive_blocked": skippedSensitive,
		}
		if len(errs) > 0 {
			result["errors"] = errs
		}
		b, ok := marshalJSONSafe(result)
		if !ok {
			return `{"error":"failed to marshal result"}`
		}
		return string(b)

	case "get_screenshot_summary":
		var p struct {
			AgentID string `json:"agent_id"`
			Days    int    `json:"days"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &p)
		if p.Days <= 0 || p.Days > 365 {
			p.Days = 30
		}
		since := time.Now().AddDate(0, 0, -p.Days)
		q := s.db.Where("type IN ? AND created_at >= ?", []string{"screenshot", "screenshot_window", "screen_stream_start"}, since)
		if p.AgentID != "" {
			aid := s.resolveAgentID(p.AgentID)
			if aid == "" {
				return `{"error":"agent not found"}`
			}
			q = q.Where("agent_id = ?", aid)
		}
		var tasks []db.Task
		q.Order("created_at desc").Limit(500).Find(&tasks)
		byAgent := map[string]int{}
		type shotItem struct {
			TaskID     uint   `json:"task_id"`
			AgentID    string `json:"agent_id"`
			Status     string `json:"status"`
			Size       int64  `json:"size_bytes"`
			CreatedAt  string `json:"created_at"`
		}
		var recent []shotItem
		for i, t := range tasks {
			byAgent[t.AgentID]++
			if i < 5 {
				recent = append(recent, shotItem{TaskID: t.ID, AgentID: t.AgentID, Status: t.Status, Size: t.TotalBytes, CreatedAt: t.CreatedAt.Format(time.RFC3339)})
			}
		}
		var agentsOut []map[string]interface{}
		for aid, cnt := range byAgent {
			agentsOut = append(agentsOut, map[string]interface{}{"agent_id": aid, "count": cnt})
		}
		b, ok := marshalJSONSafe(map[string]interface{}{
			"window_days":   p.Days,
			"total":         len(tasks),
			"by_agent":      agentsOut,
			"recent":        recent,
			"note":          "Screenshot files are served from the Screenshots page; this is task-level metadata.",
		})
		if !ok {
			return `{"error":"failed to marshal summary"}`
		}
		return string(b)

	case "get_keylog_summary":
		var p struct {
			AgentID string `json:"agent_id"`
			Days    int    `json:"days"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &p)
		if p.Days <= 0 || p.Days > 365 {
			p.Days = 30
		}
		since := time.Now().AddDate(0, 0, -p.Days)
		q := s.db.Where("type = ? AND created_at >= ?", "keylogger_dump", since)
		if p.AgentID != "" {
			aid := s.resolveAgentID(p.AgentID)
			if aid == "" {
				return `{"error":"agent not found"}`
			}
			q = q.Where("agent_id = ?", aid)
		}
		var tasks []db.Task
		q.Order("created_at desc").Limit(200).Find(&tasks)
		byAgent := map[string]int{}
		highValueKeywords := []string{"password", "passwd", "login", "logon", "credential", "token", "@gmail", "@outlook", "@yahoo", "@qq.", "@163.", "banking", "vpn"}
		type kvEntry struct {
			TaskID  uint   `json:"task_id"`
			AgentID string `json:"agent_id"`
			Keyword string `json:"keyword"`
			Context string `json:"context"`
			CreatedAt string `json:"created_at"`
		}
		var hits []kvEntry
		totalBytes := int64(0)
		for _, t := range tasks {
			byAgent[t.AgentID]++
			totalBytes += int64(len(t.Result))
			lower := strings.ToLower(t.Result)
			for _, kw := range highValueKeywords {
				idx := strings.Index(lower, kw)
				if idx < 0 {
					continue
				}
				start := idx - 40
				if start < 0 {
					start = 0
				}
				end := idx + len(kw) + 40
				if end > len(t.Result) {
					end = len(t.Result)
				}
				ctx := t.Result[start:end]
				ctx = strings.ReplaceAll(ctx, "\n", " ")
				hits = append(hits, kvEntry{TaskID: t.ID, AgentID: t.AgentID, Keyword: kw, Context: truncateStr(ctx, 100), CreatedAt: t.CreatedAt.Format(time.RFC3339)})
				break // one hit per dump is enough for triage
			}
			if len(hits) >= 10 {
				break
			}
		}
		var agentsOut []map[string]interface{}
		for aid, cnt := range byAgent {
			agentsOut = append(agentsOut, map[string]interface{}{"agent_id": aid, "dumps": cnt})
		}
		b, ok := marshalJSONSafe(map[string]interface{}{
			"window_days":       p.Days,
			"total_dumps":       len(tasks),
			"captured_bytes":    totalBytes,
			"by_agent":          agentsOut,
			"high_value_hits":   hits,
			"note":              "Full keystroke dumps are on the agent detail page; this is a keyword-triage view.",
		})
		if !ok {
			return `{"error":"failed to marshal summary"}`
		}
		return string(b)

	case "generate_report":
		var p struct {
			Scope string `json:"scope"`
			Title string `json:"title"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &p)
		if p.Scope == "" {
			p.Scope = "full"
		}
		switch p.Scope {
		case "full", "executive", "technical", "coverage":
		default:
			return `{"error":"invalid scope (full|executive|technical|coverage)"}`
		}
		md, sections, err := s.buildAIMarkdownReport(p.Scope)
		if err != nil {
			return `{"error":"failed to build report: ` + sanitizeError(err, "report") + `"}`
		}
		title := strings.TrimSpace(p.Title)
		if title == "" {
			title = fmt.Sprintf("AI %s Report %s", strings.ToUpper(p.Scope[:1])+p.Scope[1:], time.Now().Format("2006-01-02 15:04"))
		}
		sectionsJSON, _ := json.Marshal(sections)
		rep := db.GeneratedReport{
			Name:     title,
			Template: "ai_" + p.Scope,
			Format:   "markdown",
			Content:  md,
			Sections: string(sectionsJSON),
		}
		if err := s.db.Create(&rep).Error; err != nil {
			return `{"error":"failed to save report"}`
		}
		b, ok := marshalJSONSafe(map[string]interface{}{
			"report_id": rep.ID,
			"title":     title,
			"scope":     p.Scope,
			"chars":     len(md),
			"view_url":  "/report",
			"note":      "Saved to the report library. The operator can view/export it on the Report page.",
		})
		if !ok {
			return `{"error":"failed to marshal result"}`
		}
		return string(b)

	case "create_listener":
		if s.cfg == nil || !s.cfg.AI.AllowExecute {
			return `{"error":"allow_execute is disabled in AI config; enable it to manage listeners"}`
		}
		var p struct {
			Name      string `json:"name"`
			Scheme    string `json:"scheme"`
			Host      string `json:"host"`
			Port      int    `json:"port"`
			DNSDomain string `json:"dns_domain"`
			ICMPAddr  string `json:"icmp_addr"`
			Notes     string `json:"notes"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &p)
		scheme := strings.ToLower(strings.TrimSpace(p.Scheme))
		if !validateListenerScheme(scheme) {
			b, _ := marshalJSONSafe(map[string]interface{}{"error": listenerSchemeHint(p.Scheme)})
			return string(b)
		}
		l := db.Listener{
			Name:      strings.TrimSpace(p.Name),
			Scheme:    scheme,
			Host:      strings.TrimSpace(p.Host),
			Port:      p.Port,
			DNSDomain: strings.TrimSpace(p.DNSDomain),
			ICMPAddr:  strings.TrimSpace(p.ICMPAddr),
			Notes:     p.Notes,
			Enabled:   true,
			Status:    "running",
		}
		if l.Name == "" {
			l.Name = fmt.Sprintf("Listener %d", l.Port)
		}
		if l.Host == "" && scheme != "dns" && scheme != "icmp" {
			return `{"error":"host required (e.g. 0.0.0.0)"}`
		}
		if scheme == "dns" && l.DNSDomain == "" {
			return `{"error":"dns_domain required for dns listeners"}`
		}
		if l.Port != 0 && (l.Port < 1 || l.Port > 65535) {
			return `{"error":"port must be between 1 and 65535"}`
		}
		if l.Host != "" && !isValidHost(l.Host) {
			return `{"error":"invalid host address"}`
		}
		normalizeListenerProtocol(&l)
		if err := s.db.Create(&l).Error; err != nil {
			b, _ := marshalJSONSafe(map[string]interface{}{"error": sanitizeError(err, "Listener create")})
			return string(b)
		}
		bound := s.startListenerForRecord(&l, "ai-created")
		if !bound {
			s.db.Model(&l).Update("status", "stopped")
		}
		s.syncListenerProbe(&l)
		s.broadcastListenerUpdate("created", &l)
		bindStatus := "running"
		if !bound {
			bindStatus = "stopped"
		}
		b, ok := marshalJSONSafe(map[string]interface{}{
			"id": l.ID, "name": l.Name, "scheme": l.Scheme,
			"host": l.Host, "port": l.Port,
			"bind_status": bindStatus,
			"note":        "bind_status=stopped means the port could not be bound; check conflicts on the Listeners page.",
		})
		if !ok {
			return `{"error":"failed to marshal result"}`
		}
		return string(b)

	case "update_listener":
		if s.cfg == nil || !s.cfg.AI.AllowExecute {
			return `{"error":"allow_execute is disabled in AI config; enable it to manage listeners"}`
		}
		var p struct {
			ListenerID uint   `json:"listener_id"`
			Enabled    *bool  `json:"enabled"`
			Name       string `json:"name"`
			Host       string `json:"host"`
			Port       int    `json:"port"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &p)
		if p.ListenerID == 0 {
			if v, err := strconv.ParseUint(args["listener_id"], 10, 64); err == nil {
				p.ListenerID = uint(v)
			}
		}
		if p.ListenerID == 0 {
			return `{"error":"numeric listener_id required"}`
		}
		var l db.Listener
		if err := s.db.First(&l, p.ListenerID).Error; err != nil {
			return `{"error":"listener not found"}`
		}
		bindChanged := false
		oldKey := listenerKey(&l)
		if p.Name != "" {
			l.Name = strings.TrimSpace(p.Name)
		}
		if p.Host != "" {
			h := strings.TrimSpace(p.Host)
			if !isValidHost(h) {
				return `{"error":"invalid host address"}`
			}
			l.Host = h
			bindChanged = true
		}
		if p.Port != 0 {
			if p.Port < 1 || p.Port > 65535 {
				return `{"error":"port must be between 1 and 65535"}`
			}
			l.Port = p.Port
			bindChanged = true
		}
		if p.Enabled != nil {
			l.Enabled = *p.Enabled
		}
		if err := s.db.Save(&l).Error; err != nil {
			return `{"error":"failed to save listener"}`
		}
		if bindChanged {
			s.stopExtraListener(oldKey)
		}
		status := l.Status
		if l.Enabled {
			if s.startListenerForRecord(&l, "ai-updated") {
				status = "running"
			} else {
				status = "stopped"
			}
			s.db.Model(&l).Update("status", status)
		} else if p.Enabled != nil && !*p.Enabled {
			s.stopExtraListener(oldKey)
			status = "stopped"
			s.db.Model(&l).Update("status", status)
		}
		s.syncListenerProbe(&l)
		s.broadcastListenerUpdate("updated", &l)
		b, ok := marshalJSONSafe(map[string]interface{}{
			"id": l.ID, "name": l.Name, "enabled": l.Enabled,
			"host": l.Host, "port": l.Port, "bind_status": status,
		})
		if !ok {
			return `{"error":"failed to marshal result"}`
		}
		return string(b)

	case "delete_listener":
		if s.cfg == nil || !s.cfg.AI.AllowExecute {
			return `{"error":"allow_execute is disabled in AI config; enable it to manage listeners"}`
		}
		var p struct {
			ListenerID uint `json:"listener_id"`
		}
		_ = json.Unmarshal([]byte(argsJSON), &p)
		if p.ListenerID == 0 {
			if v, err := strconv.ParseUint(args["listener_id"], 10, 64); err == nil {
				p.ListenerID = uint(v)
			}
		}
		if p.ListenerID == 0 {
			return `{"error":"numeric listener_id required"}`
		}
		var agentCount int64
		s.db.Model(&db.Implant{}).Where("listener_id = ?", p.ListenerID).Count(&agentCount)
		if agentCount > 0 {
			b, _ := marshalJSONSafe(map[string]interface{}{
				"error":       fmt.Sprintf("cannot delete: %d agents still reference this listener", agentCount),
				"agent_count": agentCount,
			})
			return string(b)
		}
		var l db.Listener
		if err := s.db.First(&l, p.ListenerID).Error; err == nil {
			s.stopExtraListener(listenerKey(&l))
			if s.circuitBreaker != nil {
				s.circuitBreaker.UnregisterTarget(listenerTargetID(&l))
			}
		} else {
			return `{"error":"listener not found"}`
		}
		if err := s.db.Delete(&db.Listener{}, p.ListenerID).Error; err != nil {
			return `{"error":"failed to delete listener"}`
		}
		s.broadcastListenerUpdate("deleted", &l)
		b, ok := marshalJSONSafe(map[string]interface{}{"deleted": true, "id": l.ID, "name": l.Name})
		if !ok {
			return `{"error":"failed to marshal result"}`
		}
		return string(b)

	default:
		return `{"error":"unknown tool"}`
	}
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 0 {
		return "..."
	}
	// Back off to a valid rune boundary so multi-byte UTF-8 (Chinese
	// hostnames, CJK task output) is never sliced mid-sequence — encoding/
	// json would otherwise emit U+FFFD mojibake for the tail.
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n] + "..."
}

// ── Situation snapshot (injected as live system context) ──────────────────

func (s *Server) buildSituationSnapshot() string {
	if s.db == nil {
		return ""
	}
	var totalAgents, onlineAgents, pendingTasks, aiPendingTasks, alertCount int64
	var listenersActive int64

	s.db.Model(&db.Implant{}).Count(&totalAgents)
	s.db.Model(&db.Implant{}).Where("status = ?", "online").Count(&onlineAgents)
	s.db.Model(&db.Task{}).Where("status = ?", "pending_approval").Count(&pendingTasks)
	s.db.Model(&db.Task{}).Where("status = ? AND created_by = ?", "pending_approval", "ai").Count(&aiPendingTasks)
	s.db.Model(&db.Alert{}).Where("status = ?", "active").Count(&alertCount)
	s.db.Model(&db.Listener{}).Where("enabled = ?", true).Count(&listenersActive)

	var recentAgents []db.Implant
	s.db.Order("last_seen desc").Limit(3).Find(&recentAgents)

	var sb strings.Builder
	sb.WriteString("## Current situation snapshot (live)\n")
	sb.WriteString(fmt.Sprintf("- Agents: %d total, %d online, %d offline\n", totalAgents, onlineAgents, totalAgents-onlineAgents))
	sb.WriteString(fmt.Sprintf("- Listeners: %d active\n", listenersActive))
	sb.WriteString(fmt.Sprintf("- Tasks pending approval: %d", pendingTasks))
	if aiPendingTasks > 0 {
		sb.WriteString(fmt.Sprintf(" (%d AI-proposed)", aiPendingTasks))
	}
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("- Active alerts: %d\n", alertCount))
	if len(recentAgents) > 0 {
		sb.WriteString("- Recently seen agents:\n")
		for _, a := range recentAgents {
			sb.WriteString(fmt.Sprintf("  - %s (%s, %s, %s, last_seen %s)\n",
				a.ID, a.Hostname, a.OS, a.Status, a.LastSeen.Format(time.RFC3339)))
		}
	}
	return sb.String()
}

// ── Sensitive command guard ───────────────────────────────────────────────

var sensitiveCommandKeywords = map[string]bool{
	"mimikatz":   true,
	"secretsdump": true,
	"dcsync":     true,
	"kerberoast": true,
	"psexec":     true,
	"wce":        true,
	"bloodhound": true,
	"sharphound": true,
}

func isSensitiveCommand(cmd string) bool {
	lower := strings.ToLower(cmd)
	for kw := range sensitiveCommandKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

var (
	taskWaitMaxDuration = AITaskWaitMax
	taskPollMinInterval = AITaskPollMinInterval
)

type executeCommandArgs struct {
	AgentID       string
	Command       string
	Shell         string
	WaitForResult bool
}

func parseExecuteCommandArgs(argsJSON string) executeCommandArgs {
	var raw struct {
		AgentID       string `json:"agent_id"`
		Command       string `json:"command"`
		Shell         string `json:"shell"`
		WaitForResult *bool  `json:"wait_for_result"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &raw); err != nil {
		slog.Error("ai: failed to unmarshal execute command args", "error", err, "args", argsJSON)
	}
	out := executeCommandArgs{
		AgentID:       raw.AgentID,
		Command:       raw.Command,
		Shell:         raw.Shell,
		WaitForResult: true,
	}
	if raw.WaitForResult != nil {
		out.WaitForResult = *raw.WaitForResult
	}
	return out
}

// taskPollIntervalSeconds returns poll interval in seconds: max(agent interval, 1).
func taskPollIntervalSeconds(currentInterval int) int {
	if currentInterval < 1 {
		return 1
	}
	return currentInterval
}

// taskPollSleepDuration computes the next sleep duration respecting min poll and remaining wait budget.
func taskPollSleepDuration(intervalSec int, remaining time.Duration) time.Duration {
	sleep := time.Duration(taskPollIntervalSeconds(intervalSec)) * time.Second
	if sleep < taskPollMinInterval {
		sleep = taskPollMinInterval
	}
	if remaining > 0 && sleep > remaining {
		sleep = remaining
	}
	return sleep
}

func isTaskTerminal(status string) bool {
	return status == "completed" || status == "failed"
}

func marshalTaskResult(task db.Task, extra map[string]interface{}) string {
	result := map[string]interface{}{
		"task_id": task.ID,
		"status":  task.Status,
	}
	for k, v := range extra {
		result[k] = v
	}
	if task.Result != "" {
		result["result"] = truncateStr(task.Result, AITaskResultTruncLen)
	}
	if task.Error != "" {
		result["error"] = task.Error
	}
	b, ok := marshalJSONSafe(result)
	if !ok {
		return `{"error":"failed to marshal task result"}`
	}
	return string(b)
}

func (s *Server) waitForTaskResult(taskID uint, agentID string) string {
	deadline := time.Now().Add(taskWaitMaxDuration)

	var agent db.Implant
	intervalSec := 1
	if err := s.db.Where("id = ?", agentID).First(&agent).Error; err == nil {
		intervalSec = agent.CurrentInterval
	}

	for time.Now().Before(deadline) {
		var task db.Task
		if err := s.db.First(&task, taskID).Error; err != nil {
			return `{"error":"task not found"}`
		}

		if isTaskTerminal(task.Status) {
			return marshalTaskResult(task, nil)
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		time.Sleep(taskPollSleepDuration(intervalSec, remaining))
	}

	var task db.Task
	if err := s.db.First(&task, taskID).Error; err != nil {
		return `{"error":"task not found"}`
	}
	if isTaskTerminal(task.Status) {
		return marshalTaskResult(task, nil)
	}
	return marshalTaskResult(task, map[string]interface{}{
		"message": "Task still pending after wait timeout. Use get_agent_tasks to check later.",
		"waited":  taskWaitMaxDuration.Seconds(),
	})
}

// buildClaudeRequest converts an OpenAI-format JSON payload to Anthropic Claude format
func (s *Server) buildClaudeRequest(openAIPayload []byte) []byte {
	var req struct {
		Model     string        `json:"model"`
		Messages  []chatMessage `json:"messages"`
		Stream    bool          `json:"stream"`
		Tools     []toolDef     `json:"tools,omitempty"`
		MaxTokens int           `json:"max_tokens,omitempty"`
	}
	if err := json.Unmarshal(openAIPayload, &req); err != nil {
		slog.Warn("Failed to parse OpenAI payload for Claude conversion", "error", err)
		return openAIPayload
	}

	// Build Claude-format messages (system is top-level, not a message)
	var claudeMessages []map[string]interface{}
	var systemPrompt string
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			systemPrompt = msg.Content
			continue
		}
		claudeMsg := map[string]interface{}{"role": msg.Role}
		if msg.Content != "" {
			claudeMsg["content"] = msg.Content
		}
		// Convert tool calls to Claude format
		if len(msg.ToolCalls) > 0 {
			var claudeToolUses []map[string]interface{}
			for _, tc := range msg.ToolCalls {
				claudeToolUses = append(claudeToolUses, map[string]interface{}{
					"type":  "tool_use",
					"id":    tc.ID,
					"name":  tc.Function.Name,
					"input": parseJSONMap(tc.Function.Arguments),
				})
			}
			claudeMsg["content"] = claudeToolUses
		}
		// Tool results
		if msg.Role == "tool" {
			claudeMsg["content"] = []map[string]interface{}{{
				"type":        "tool_result",
				"tool_use_id": msg.ToolCallID,
				"content":     msg.Content,
			}}
		}
		claudeMessages = append(claudeMessages, claudeMsg)
	}

	// Claude tools format
	var claudeTools []map[string]interface{}
	for _, t := range req.Tools {
		claudeTools = append(claudeTools, map[string]interface{}{
			"name":         t.Function.Name,
			"description":  t.Function.Description,
			"input_schema": t.Function.Parameters,
		})
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = ClaudeMaxTokens
	}
	claudeReq := map[string]interface{}{
		"model":      req.Model,
		"messages":   claudeMessages,
		"max_tokens": maxTokens,
		"stream":     req.Stream,
	}
	if systemPrompt != "" {
		claudeReq["system"] = systemPrompt
	}
	if len(claudeTools) > 0 {
		claudeReq["tools"] = claudeTools
	}

	b, ok := marshalJSONSafe(claudeReq)
	if !ok {
		return nil
	}
	return b
}

func parseJSONMap(s string) map[string]interface{} {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		slog.Error("parseJSONMap: failed to unmarshal", "error", err)
	}
	if m == nil {
		m = map[string]interface{}{}
	}
	return m
}

func (s *Server) parseClaudeStream(resp *http.Response, ch chan<- sseEvent) (toolCalls []toolCall, content, finishReason string, usage aiUsage) {
	reader := io.Reader(resp.Body)
	buf := make([]byte, AIStreamBufSize)
	var leftover string

	type buildingClaudeTool struct {
		ID        string
		Name      string
		Arguments strings.Builder
	}
	var buildingTools []*buildingClaudeTool

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			data := leftover + string(buf[:n])
			lines := strings.Split(data, "\n")
			leftover = lines[len(lines)-1]
			lines = lines[:len(lines)-1]

			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" || !strings.HasPrefix(line, "data: ") {
					continue
				}
				jsonStr := strings.TrimPrefix(line, "data: ")

				var event struct {
					Type  string `json:"type"`
					Index int    `json:"index"`
					Delta struct {
						Type        string `json:"type"`
						Text        string `json:"text"`
						PartialJSON string `json:"partial_json"`
						StopReason  string `json:"stop_reason"`
					} `json:"delta"`
					ContentBlock struct {
						Type string `json:"type"`
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"content_block"`
					Message *struct {
						Usage *struct {
							InputTokens int `json:"input_tokens"`
						} `json:"usage"`
					} `json:"message"`
					Usage *struct {
						OutputTokens int `json:"output_tokens"`
					} `json:"usage"`
				}
				if err := json.Unmarshal([]byte(jsonStr), &event); err != nil {
					continue
				}
				if event.Message != nil && event.Message.Usage != nil && event.Message.Usage.InputTokens > 0 {
					usage.PromptTokens = event.Message.Usage.InputTokens
				}
				if event.Usage != nil && event.Usage.OutputTokens > 0 {
					usage.CompletionTokens = event.Usage.OutputTokens
				}

				switch event.Type {
				case "content_block_delta":
					if event.Delta.Type == "text_delta" && event.Delta.Text != "" {
						content += event.Delta.Text
						if len(content) > 8000 {
							content = content[:8000] + "\n\n[Response truncated]"
							ch <- sseEvent{"text", content}
							return
						}
						ch <- sseEvent{"text", content}
					} else if event.Delta.Type == "input_json_delta" && event.Delta.PartialJSON != "" {
						for len(buildingTools) <= event.Index {
							buildingTools = append(buildingTools, &buildingClaudeTool{})
						}
						buildingTools[event.Index].Arguments.WriteString(event.Delta.PartialJSON)
					}
				case "content_block_start":
					if event.ContentBlock.Type == "tool_use" {
						for len(buildingTools) <= event.Index {
							buildingTools = append(buildingTools, &buildingClaudeTool{})
						}
						buildingTools[event.Index].ID = event.ContentBlock.ID
						buildingTools[event.Index].Name = event.ContentBlock.Name
					}
				case "message_delta":
					if event.Delta.StopReason != "" {
						finishReason = event.Delta.StopReason
					}
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return
		}
	}

	for _, bt := range buildingTools {
		if bt.Name != "" {
			toolCalls = append(toolCalls, toolCall{
				ID:   bt.ID,
				Type: "function",
				Function: toolCallFunc{
					Name:      bt.Name,
					Arguments: bt.Arguments.String(),
				},
			})
		}
	}
	if len(toolCalls) > 0 {
		finishReason = "tool_calls"
	} else if finishReason == "" {
		finishReason = "stop"
	}
	return
}

func (s *Server) resolveAgentID(idOrHost string) string {
	var agent db.Implant
	if err := s.db.Where("id = ? OR hostname = ?", idOrHost, idOrHost).First(&agent).Error; err != nil {
		return ""
	}
	return agent.ID
}
