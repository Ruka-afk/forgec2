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
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// 鈹€鈹€ AI Chat Page 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€

// aiConfigPublicView returns a redacted view of the AI configuration safe to
// send to any authenticated user. The provider API key is NEVER included
// (S1): only non-secret display fields are exposed.
func (s *Server) aiConfigPublicView() gin.H {
	return gin.H{
		"enabled":       s.cfg.AI.Enabled,
		"provider":      s.cfg.AI.Provider,
		"model":         s.cfg.AI.Model,
		"endpoint":      s.cfg.AI.Endpoint,
		"system_prompt": s.cfg.AI.SystemPrompt,
		"allow_execute": s.cfg.AI.AllowExecute,
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
		Enabled      bool   `json:"enabled"`
		Provider     string `json:"provider"`
		APIKey       string `json:"api_key"`
		Model        string `json:"model"`
		Endpoint     string `json:"endpoint"`
		SystemPrompt string `json:"system_prompt"`
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

	events := s.converse(model, s.cfg.AI.SystemPrompt, req.Messages, c.Request.Context())
	for evt := range events {
		s.writeSSE(c, flusher, evt.Type, evt.Data)
	}
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

// converse runs the LLM conversation loop with tool calling, returning SSE events
func (s *Server) converse(model, systemPrompt string, userMessages []chatMessage, ctx context.Context) <-chan sseEvent {
	ch := make(chan sseEvent, 10)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("recovered from panic", "err", r, "stack", string(debug.Stack()))
			}
		}()
		defer close(ch)

		// Send immediate thinking indicator
		ch <- sseEvent{"thinking", ""}

		messages := make([]chatMessage, 0, len(userMessages)+1)
		messages = append(messages, chatMessage{Role: "system", Content: systemPrompt})
		messages = append(messages, userMessages...)
		tools := buildTools()

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
				ch <- sseEvent{"error", "AI request failed"}
				return
			}

			var toolCalls []toolCall
			var content, finishReason string
			if s.cfg.AI.Provider == "claude" {
				toolCalls, content, finishReason = s.parseClaudeStream(resp, ch)
			} else {
				toolCalls, content, _, finishReason = s.parseStreamChunks(resp, ch)
			}
			resp.Body.Close()

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
					result := s.executeTool(tc.Function.Name, tc.Function.Arguments)
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
// and accumulates tool calls. Returns collected tool calls, full content, full reasoning, and finish reason.
func (s *Server) parseStreamChunks(resp *http.Response, ch chan<- sseEvent) (toolCalls []toolCall, content, reasoning, finishReason string) {
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
				}
				if err := json.Unmarshal([]byte(jsonStr), &chunk); err != nil {
					continue
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
		urlStr = baseURL + "/messages"
		payload = s.buildClaudeRequest(payload)
	} else {
		urlStr = baseURL + "/chat/completions"
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

func buildTools() []toolDef {
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
				Description: "Execute a command on the specified agent. The command is queued and requires a human operator to approve it in the ForgeC2 Tasks page before the agent executes it. Returns the task id and status pending_approval; use get_agent_tasks later to check the result.",
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
							"description": "Reserved. AI-generated commands always require operator approval.",
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
	}
}

// 鈹€鈹€ Tool Execution 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€

func (s *Server) executeTool(name string, argsJSON string) string {
	var args map[string]string
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return fmt.Sprintf(`{"error":"invalid arguments JSON: %s"}`, err.Error())
	}

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
		task := db.Task{
			AgentID: aid, Type: "shell", Command: ecArgs.Command,
			Shell: shell, Status: TaskStatusPendingApproval, CreatedBy: "ai",
		}
		if err := s.db.Create(&task).Error; err != nil {
			return `{"error":"failed to create task"}`
		}
		b, ok := marshalJSONSafe(map[string]interface{}{
			"task_id": task.ID,
			"status":  "pending_approval",
			"message": "Command created but requires operator approval before the agent executes it. Approve it in the Tasks page.",
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

	default:
		return `{"error":"unknown tool"}`
	}
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
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
		Model    string        `json:"model"`
		Messages []chatMessage `json:"messages"`
		Stream   bool          `json:"stream"`
		Tools    []toolDef     `json:"tools,omitempty"`
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

	claudeReq := map[string]interface{}{
		"model":      req.Model,
		"messages":   claudeMessages,
		"max_tokens": ClaudeMaxTokens,
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

func (s *Server) parseClaudeStream(resp *http.Response, ch chan<- sseEvent) (toolCalls []toolCall, content, finishReason string) {
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
				}
				if err := json.Unmarshal([]byte(jsonStr), &event); err != nil {
					continue
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
