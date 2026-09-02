package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type aiRoundTripFunc func(*http.Request) (*http.Response, error)

func (f aiRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestParseStreamChunksForwardsReasoning(t *testing.T) {
	body := strings.Join([]string{
		`data: {"choices":[{"delta":{"reasoning_content":"private chain of thought"},"finish_reason":""}]}`,
		`data: {"choices":[{"delta":{"content":"safe answer"},"finish_reason":"stop"}]}`,
		"",
	}, "\n")
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(body))}
	ch := make(chan sseEvent, 16)

	_, content, reasoning, finishReason, _, err := (&Server{}).parseStreamChunks(resp, ch, context.Background())
	if err != nil {
		t.Fatalf("parseStreamChunks: %v", err)
	}
	close(ch)

	if content != "safe answer" || reasoning != "private chain of thought" || finishReason != "stop" {
		t.Fatalf("unexpected parse result: content=%q reasoning=%q finish=%q", content, reasoning, finishReason)
	}
	var stages []string
	var sawReasoningText bool
	for event := range ch {
		if event.Type == "reasoning" && strings.Contains(event.Data, "private chain of thought") {
			sawReasoningText = true
		}
		if event.Type == "progress" {
			stages = append(stages, event.Data)
		}
	}
	if !sawReasoningText {
		t.Fatal("expected reasoning SSE event with provider thinking text")
	}
	if len(stages) != 2 || !strings.Contains(stages[0], "reasoning") || !strings.Contains(stages[1], "answering") {
		t.Fatalf("unexpected progress stages: %#v", stages)
	}
}

func TestParseStreamChunksOpenRouterReasoningField(t *testing.T) {
	body := strings.Join([]string{
		`data: {"choices":[{"delta":{"reasoning":"Let me think","reasoning_details":[{"type":"reasoning.text","text":"Let me think"}]},"finish_reason":""}]}`,
		`data: {"choices":[{"delta":{"content":null},"finish_reason":"stop"}]}`,
		"",
	}, "\n")
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(body))}
	ch := make(chan sseEvent, 8)
	_, content, reasoning, finish, _, err := (&Server{}).parseStreamChunks(resp, ch, context.Background())
	if err != nil {
		t.Fatalf("parseStreamChunks: %v", err)
	}
	close(ch)
	if content != "" || reasoning != "Let me think" || finish != "stop" {
		t.Fatalf("content=%q reasoning=%q finish=%q", content, reasoning, finish)
	}
}

func TestParseStreamChunksContentPartsArray(t *testing.T) {
	body := `data: {"choices":[{"delta":{"content":[{"type":"text","text":"hello "},{"type":"text","text":"world"}]},"finish_reason":"stop"}]}` + "\n"
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(body))}
	ch := make(chan sseEvent, 8)
	_, content, _, _, _, err := (&Server{}).parseStreamChunks(resp, ch, context.Background())
	if err != nil {
		t.Fatalf("parseStreamChunks: %v", err)
	}
	close(ch)
	if content != "hello world" {
		t.Fatalf("content=%q", content)
	}
}

func TestParseStreamChunksKeepsUnterminatedFinalEvent(t *testing.T) {
	// Several OpenAI-compatible gateways close immediately after their final
	// JSON line. There is deliberately no trailing newline here.
	body := `data:{"choices":[{"delta":{"content":"完整回答"},"finish_reason":"stop"}]}`
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(body))}
	ch := make(chan sseEvent, 8)
	_, content, _, finish, _, err := (&Server{}).parseStreamChunks(resp, ch, context.Background())
	close(ch)
	if err != nil {
		t.Fatalf("parseStreamChunks: %v", err)
	}
	if content != "完整回答" || finish != "stop" {
		t.Fatalf("content=%q finish=%q", content, finish)
	}
}

func TestParseClaudeStreamKeepsUnterminatedFinalEvent(t *testing.T) {
	body := "data:{\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"complete\"}}\n" +
		`data:{"type":"message_stop"}`
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(body))}
	ch := make(chan sseEvent, 8)
	_, content, _, _, _, err := (&Server{}).parseClaudeStream(resp, ch, context.Background())
	close(ch)
	if err != nil {
		t.Fatalf("parseClaudeStream: %v", err)
	}
	if content != "complete" {
		t.Fatalf("content=%q", content)
	}
}

func TestParseStreamChunksRejectsMissingTerminalEvent(t *testing.T) {
	body := "data:{\"choices\":[{\"delta\":{\"content\":\"part\"},\"finish_reason\":\"\"}]}\n" +
		`data:{"choices":[{"delta":{"content":"ial"},"finish_reason":""}]}`
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(body))}
	ch := make(chan sseEvent, 8)
	_, content, _, _, _, err := (&Server{}).parseStreamChunks(resp, ch, context.Background())
	close(ch)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("expected unexpected EOF, got %v", err)
	}
	if content != "partial" {
		t.Fatalf("partial content was lost: %q", content)
	}
	lastText := ""
	for event := range ch {
		if event.Type == "text" {
			lastText = event.Data
		}
	}
	if lastText != "partial" {
		t.Fatalf("last buffered snapshot was not forwarded: %q", lastText)
	}
}

func TestParseClaudeStreamRejectsMissingTerminalEvent(t *testing.T) {
	body := "data:{\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"part\"}}\n" +
		`data:{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"ial"}}`
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(body))}
	ch := make(chan sseEvent, 8)
	_, content, _, _, _, err := (&Server{}).parseClaudeStream(resp, ch, context.Background())
	close(ch)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("expected unexpected EOF, got %v", err)
	}
	if content != "partial" {
		t.Fatalf("partial content was lost: %q", content)
	}
	lastText := ""
	for event := range ch {
		if event.Type == "text" {
			lastText = event.Data
		}
	}
	if lastText != "partial" {
		t.Fatalf("last buffered snapshot was not forwarded: %q", lastText)
	}
}

func TestParseClaudeStreamSurfacesProviderError(t *testing.T) {
	body := `data:{"type":"error","error":{"type":"overloaded_error","message":"provider overloaded"}}`
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(body))}
	ch := make(chan sseEvent, 8)
	_, _, _, _, _, err := (&Server{}).parseClaudeStream(resp, ch, context.Background())
	close(ch)
	if err == nil || !strings.Contains(err.Error(), "provider overloaded") {
		t.Fatalf("expected Claude provider error, got %v", err)
	}
}

func TestAIResponseTruncationPreservesUTF8(t *testing.T) {
	input := strings.Repeat("界", AIResponseTruncLen/3+10)
	got, truncated := appendAIResponseText("", input)
	if !truncated {
		t.Fatal("expected response to be truncated")
	}
	if !utf8.ValidString(got) {
		t.Fatal("truncated response is not valid UTF-8")
	}
	if !strings.HasSuffix(got, "[Response truncated]") {
		t.Fatalf("missing truncation marker: %q", got[len(got)-40:])
	}
}

func TestParseStreamChunksThrottlesCumulativeSnapshots(t *testing.T) {
	var body strings.Builder
	for i := 0; i < 1024; i++ {
		body.WriteString(`data: {"choices":[{"delta":{"content":"x"},"finish_reason":""}]}`)
		body.WriteByte('\n')
	}
	body.WriteString(`data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`)
	body.WriteByte('\n')
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(body.String()))}
	ch := make(chan sseEvent, 32)
	_, content, _, _, _, err := (&Server{}).parseStreamChunks(resp, ch, context.Background())
	if err != nil {
		t.Fatalf("parseStreamChunks: %v", err)
	}
	close(ch)
	textEvents := 0
	for event := range ch {
		if event.Type == "text" {
			textEvents++
		}
	}
	if len(content) != 1024 {
		t.Fatalf("content length=%d, want 1024", len(content))
	}
	if textEvents > 8 {
		t.Fatalf("got %d text snapshots; stream was not throttled", textEvents)
	}
}

func TestParseStreamChunksProviderError(t *testing.T) {
	body := `data: {"error":{"message":"Provider disconnected unexpectedly"},"choices":[{"delta":{"content":""},"finish_reason":"error"}]}` + "\n"
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(body))}
	ch := make(chan sseEvent, 4)
	_, _, _, _, _, err := (&Server{}).parseStreamChunks(resp, ch, context.Background())
	close(ch)
	if err == nil || !strings.Contains(err.Error(), "Provider disconnected") {
		t.Fatalf("expected provider error, got %v", err)
	}
}

func TestSplitThinkBlocks(t *testing.T) {
	think, visible := splitThinkBlocks("<think>step one</think>\n\nfinal answer")
	if think != "step one" || visible != "final answer" {
		t.Fatalf("think=%q visible=%q", think, visible)
	}
	think, visible = splitThinkBlocks("just text")
	if think != "" || visible != "just text" {
		t.Fatalf("plain: think=%q visible=%q", think, visible)
	}
}

func TestAIShouldRequestReasoning(t *testing.T) {
	if !aiShouldRequestReasoning("custom", "https://openrouter.ai/api/v1/chat/completions") {
		t.Fatal("openrouter endpoint should request reasoning")
	}
	if !aiShouldRequestReasoning("deepseek", "") {
		t.Fatal("deepseek should request reasoning")
	}
	if aiShouldRequestReasoning("openai", "https://api.openai.com/v1") {
		t.Fatal("openai chat should not send reasoning.enabled")
	}
}

func TestTaskPollIntervalSeconds(t *testing.T) {
	tests := []struct {
		interval int
		want     int
	}{
		{0, 1},
		{-5, 1},
		{1, 1},
		{10, 10},
		{30, 30},
	}
	for _, tc := range tests {
		if got := taskPollIntervalSeconds(tc.interval); got != tc.want {
			t.Fatalf("taskPollIntervalSeconds(%d) = %d, want %d", tc.interval, got, tc.want)
		}
	}
}

func TestTaskPollSleepDuration(t *testing.T) {
	if got := taskPollSleepDuration(0, 30*time.Second); got != time.Second {
		t.Fatalf("expected 1s poll for realtime agent, got %v", got)
	}
	if got := taskPollSleepDuration(10, 5*time.Second); got != 5*time.Second {
		t.Fatalf("expected remaining cap 5s, got %v", got)
	}
	if got := taskPollSleepDuration(10, 30*time.Second); got != 10*time.Second {
		t.Fatalf("expected 10s poll, got %v", got)
	}
}

func TestIsTaskTerminal(t *testing.T) {
	if !isTaskTerminal("completed") || !isTaskTerminal("failed") {
		t.Fatal("completed/failed should be terminal")
	}
	if isTaskTerminal("pending") || isTaskTerminal("running") {
		t.Fatal("pending/running should not be terminal")
	}
}

func TestParseExecuteCommandArgs_DefaultWait(t *testing.T) {
	args := parseExecuteCommandArgs(`{"agent_id":"a1","command":"whoami"}`)
	if !args.WaitForResult {
		t.Fatal("wait_for_result should default to true")
	}
	if args.AgentID != "a1" || args.Command != "whoami" {
		t.Fatalf("unexpected args: %+v", args)
	}
}

func TestResolveAIToolLimits_UnlimitedByDefault(t *testing.T) {
	limits := resolveAIToolLimits(0, 0, 0)
	if limits.maxConversationTurns != AISafetyMaxTurns || limits.maxToolRounds != AISafetyMaxToolRounds || limits.maxDuplicateToolCalls != AISafetyMaxDuplicateTools {
		t.Fatalf("zero config should apply safety ceilings, got %+v", limits)
	}
}

func TestParseStreamChunksHonorsCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	pr, pw := io.Pipe()
	defer pr.Close()
	defer pw.Close()
	resp := &http.Response{Body: pr}
	ch := make(chan sseEvent, 4)
	done := make(chan error, 1)
	go func() {
		_, _, _, _, _, err := (&Server{}).parseStreamChunks(resp, ch, ctx)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected canceled context error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("parseStreamChunks ignored cancel")
	}
}

func TestResolveAIToolLimits_CustomCaps(t *testing.T) {
	limits := resolveAIToolLimits(10, 8, 3)
	if limits.maxConversationTurns != 10 || limits.maxToolRounds != 8 || limits.maxDuplicateToolCalls != 3 {
		t.Fatalf("unexpected limits: %+v", limits)
	}
}

// TestAIConfigPublicViewRedactsAPIKey ensures the AI page data never exposes
// the provider API key to any authenticated user (S1).
func TestAIConfigPublicViewRedactsAPIKey(t *testing.T) {
	s := &Server{
		ctx: context.Background(),
		cfg: &config.Config{},
	}
	s.cfg.AI.Enabled = true
	s.cfg.AI.Provider = "deepseek"
	s.cfg.AI.Model = "deepseek-chat"
	s.cfg.AI.APIKey = "sk-SUPER-SECRET-KEY-12345"
	s.cfg.AI.Endpoint = "https://api.deepseek.com"

	view := s.aiConfigPublicView()

	if view["api_key"] != nil {
		t.Fatal("api_key must not be present in the public AI config view")
	}
	// Marshal and assert the secret string is nowhere in the payload.
	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if containsStr(string(raw), "sk-SUPER-SECRET-KEY-12345") {
		t.Fatal("API key leaked into AI config view")
	}
	if view["provider"] != "deepseek" || view["model"] != "deepseek-chat" {
		t.Fatalf("non-secret fields should be present: %+v", view)
	}
}

// TestAIDoRequestBlocksInternalEndpoint ensures the AI request path rejects
// internal/metadata/link-local endpoints via the SSRF guard before any
// outbound request carrying the API key is made (S4).
func TestAIDoRequestBlocksInternalEndpoint(t *testing.T) {
	s := &Server{ctx: context.Background(), cfg: &config.Config{}}
	s.cfg.AI.Provider = "openai"
	s.cfg.AI.Endpoint = "http://169.254.169.254"
	s.cfg.AI.APIKey = "sk-test"

	_, err := s.aiDoRequest(context.Background(), []byte(`{}`))
	if err == nil {
		t.Fatal("expected aiDoRequest to block internal/metadata endpoint")
	}
}

func TestParseExecuteCommandArgs_ExplicitWaitFalse(t *testing.T) {
	args := parseExecuteCommandArgs(`{"agent_id":"a1","command":"whoami","wait_for_result":false}`)
	if args.WaitForResult {
		t.Fatal("wait_for_result should be false when explicitly set")
	}
}

func TestEffectiveAISystemPrompt(t *testing.T) {
	if got := effectiveAISystemPrompt(""); !strings.Contains(got, "Never invent") {
		t.Fatalf("blank config should use built-in prompt, got %q", got[:min(80, len(got))])
	}
	if got := effectiveAISystemPrompt("  custom operator prompt  "); got != "  custom operator prompt  " {
		t.Fatalf("custom prompt should be kept, got %q", got)
	}
}

func TestListAgentsFiltersAndOperators(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.AutoMigrate(&db.Implant{}, &db.Alert{}, &db.Task{}, &db.Listener{}, &db.CredentialEntry{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	now := time.Now()
	_ = database.Create(&db.Implant{ID: "a1", Hostname: "WIN-DC01", IP: "10.0.0.10", OS: "windows", Username: "SYSTEM", Status: "online", Elevated: true, LastSeen: now}).Error
	_ = database.Create(&db.Implant{ID: "a2", Hostname: "ubuntu-dev", IP: "10.0.0.11", OS: "linux", Username: "root", Status: "offline", LastSeen: now.Add(-time.Hour)}).Error
	_ = database.Create(&db.Alert{Title: "beacon stale", Severity: "warning", Status: "active", Type: "agent", Message: "a2 missed check-in"}).Error

	s := &Server{db: database, ctx: context.Background(), cfg: &config.Config{}, agentPendingTasks: map[string]int{}}
	s.operatorSessions = &operatorSessionTracker{sessions: make(map[uint]*WSOperatorSession)}
	s.operatorSessions.sessions[1] = newPresenceSession(1, "admin", "a1", now)

	online := s.executeToolSwitch("list_agents", `{"status":"online"}`)
	if !strings.Contains(online, "WIN-DC01") || strings.Contains(online, "ubuntu-dev") {
		t.Fatalf("online filter failed: %s", online)
	}
	search := s.executeToolSwitch("list_agents", `{"query":"ubuntu"}`)
	if !strings.Contains(search, "ubuntu-dev") || strings.Contains(search, "WIN-DC01") {
		t.Fatalf("query filter failed: %s", search)
	}
	ops := s.executeToolSwitch("get_online_operators", `{}`)
	if !strings.Contains(ops, "admin") {
		t.Fatalf("operators empty: %s", ops)
	}
	alerts := s.executeToolSwitch("get_alerts", `{"status":"active"}`)
	if !strings.Contains(alerts, "beacon stale") {
		t.Fatalf("alerts missing: %s", alerts)
	}
	sit := s.executeToolSwitch("get_situation", `{}`)
	if !strings.Contains(sit, `"agents_online":1`) && !strings.Contains(sit, `"agents_online": 1`) {
		t.Fatalf("situation missing online count: %s", sit)
	}
	sleep := s.executeToolSwitch("set_sleep", `{"agent_id":"a1","interval":60,"jitter":10}`)
	if !strings.Contains(sleep, "pending") {
		t.Fatalf("set_sleep failed: %s", sleep)
	}
	elev := s.executeToolSwitch("list_agents", `{"elevated":true}`)
	if !strings.Contains(elev, "WIN-DC01") || strings.Contains(elev, "ubuntu-dev") {
		t.Fatalf("elevated filter failed: %s", elev)
	}
	col := s.executeToolSwitch("queue_collection", `{"agent_id":"a1","action":"screenshot"}`)
	if !strings.Contains(col, "screenshot") {
		t.Fatalf("queue_collection failed: %s", col)
	}
	bad := s.executeToolSwitch("queue_collection", `{"agent_id":"a1","action":"mimikatz"}`)
	if !strings.Contains(bad, "error") {
		t.Fatalf("expected whitelist error, got %s", bad)
	}
}

func TestCompactToolResultJSON(t *testing.T) {
	got := compactToolResultJSON(`{"id":"a1","domain":"","notes":null,"hostname":"dc"}`)
	if strings.Contains(got, "domain") || strings.Contains(got, "notes") {
		t.Fatalf("empty fields not stripped: %s", got)
	}
	if !strings.Contains(got, `"hostname":"dc"`) && !strings.Contains(got, `"hostname": "dc"`) {
		t.Fatalf("kept field missing: %s", got)
	}
}

func TestCompactToolResultJSONKeepsOversizedPayloadValidAndMarkedPartial(t *testing.T) {
	rows := make([]map[string]interface{}, 0, 200)
	for i := 0; i < 200; i++ {
		rows = append(rows, map[string]interface{}{"id": i, "hostname": strings.Repeat("host", 30)})
	}
	raw, _ := json.Marshal(map[string]interface{}{"agents": rows})
	got := compactToolResultJSON(string(raw))
	var parsed struct {
		Meta struct {
			Partial       bool `json:"partial"`
			OriginalBytes int  `json:"original_bytes"`
		} `json:"_meta"`
	}
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("compacted result is invalid JSON: %v\n%s", err, got)
	}
	if !parsed.Meta.Partial || parsed.Meta.OriginalBytes <= len(got) {
		t.Fatalf("missing partial metadata: %#v", parsed.Meta)
	}
	if len(got) > AIToolResultTruncLen*8 {
		t.Fatalf("compacted result exceeds limit: %d", len(got))
	}
}

func TestImplantIsStale(t *testing.T) {
	fresh := db.Implant{Status: "online", LastSeen: time.Now(), CurrentInterval: 10}
	if implantIsStale(fresh) {
		t.Fatal("fresh implant should not be stale")
	}
	old := db.Implant{Status: "online", LastSeen: time.Now().Add(-20 * time.Minute), CurrentInterval: 10}
	if !implantIsStale(old) {
		t.Fatal("missed check-in should be stale")
	}
	offline := db.Implant{Status: "offline", LastSeen: time.Now().Add(-time.Hour)}
	if implantIsStale(offline) {
		t.Fatal("offline is not stale-online")
	}
}

func setupAIWaitTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.AutoMigrate(&db.Implant{}, &db.Task{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.Create(&db.Implant{ID: "agent-1", Hostname: "HOST", CurrentInterval: 0})
	return database
}

func TestWaitForTaskResult_Completed(t *testing.T) {
	database := setupAIWaitTestDB(t)
	task := db.Task{AgentID: "agent-1", Type: "shell", Command: "whoami", Status: "completed", Result: "desktop\\user"}
	if err := database.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	s := &Server{db: database, ctx: context.Background()}
	result := s.waitForTaskResult(task.ID, "agent-1")
	if result == "" || result == `{"error":"task not found"}` {
		t.Fatalf("unexpected result: %s", result)
	}
	if !containsStr(result, `"status":"completed"`) || !containsStr(result, "desktop") {
		t.Fatalf("expected completed result payload, got %s", result)
	}
}

func TestWaitForTaskResult_TimeoutPending(t *testing.T) {
	database := setupAIWaitTestDB(t)
	task := db.Task{AgentID: "agent-1", Type: "shell", Command: "whoami", Status: "pending"}
	if err := database.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	origMax := taskWaitMaxDuration
	origMin := taskPollMinInterval
	taskWaitMaxDuration = 50 * time.Millisecond
	taskPollMinInterval = 10 * time.Millisecond
	defer func() {
		taskWaitMaxDuration = origMax
		taskPollMinInterval = origMin
	}()

	s := &Server{db: database, ctx: context.Background()}
	result := s.waitForTaskResult(task.ID, "agent-1")
	if !containsStr(result, `"status":"pending"`) || !containsStr(result, "wait timeout") {
		t.Fatalf("expected pending timeout payload, got %s", result)
	}
}

func TestAIBuildChatURL(t *testing.T) {
	cases := []struct {
		name     string
		base     string
		provider string
		want     string
	}{
		{"openai base appends chat/completions", "https://api.openai.com/v1", "", "https://api.openai.com/v1/chat/completions"},
		{"deepseek base appends chat/completions", "https://api.deepseek.com/v1", "deepseek", "https://api.deepseek.com/v1/chat/completions"},
		{"zhipu base appends chat/completions", "https://open.bigmodel.cn/api/paas/v4", "zhipu", "https://open.bigmodel.cn/api/paas/v4/chat/completions"},
		{"full openrouter url used as-is", "https://openrouter.ai/api/v1/chat/completions", "custom", "https://openrouter.ai/api/v1/chat/completions"},
		{"claude base appends messages", "https://api.anthropic.com/v1", "claude", "https://api.anthropic.com/v1/messages"},
		{"claude full messages url used as-is", "https://api.anthropic.com/v1/messages", "claude", "https://api.anthropic.com/v1/messages"},
	}
	for _, tc := range cases {
		if got := aiBuildChatURL(tc.base, tc.provider); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestAIEndpointDefaults(t *testing.T) {
	cfg := &config.Config{}
	cases := map[string]string{
		"openai":   "https://api.openai.com/v1",
		"deepseek": "https://api.deepseek.com/v1",
		"zhipu":    "https://open.bigmodel.cn/api/paas/v4",
	}
	for provider, want := range cases {
		cfg.AI.Provider = provider
		if got := cfg.AIEndpoint(); got != want {
			t.Errorf("AIEndpoint(%s): got %q, want %q", provider, got, want)
		}
	}
}

func TestAIDefaultModels(t *testing.T) {
	cases := map[string]string{
		"deepseek": "deepseek-chat",
		"openai":   "gpt-4o-mini",
		"claude":   "claude-3-5-sonnet-latest",
		"qianwen":  "qwen-plus",
		"zhipu":    "glm-4-flash",
		"longcat":  "LongCat-Flash-Chat",
	}
	for provider, want := range cases {
		if got := aiDefaultModel(provider); got != want {
			t.Errorf("aiDefaultModel(%q) = %q, want %q", provider, got, want)
		}
	}
}

func TestAIFlattenError(t *testing.T) {
	if aiFlattenError(nil) != "AI request failed" {
		t.Fatal("nil error should yield generic message")
	}
	got := aiFlattenError(fmt.Errorf("API 402 from https://openrouter.ai/api/v1/chat/completions: insufficient balance"))
	if !containsStr(got, "AI request failed: API 402") {
		t.Fatalf("detail should be surfaced, got %q", got)
	}
	long := aiFlattenError(fmt.Errorf("%s", strings.Repeat("x", AIErrorBodyTruncLen+50)))
	if len(long) > AIErrorBodyTruncLen+len("AI request failed: ")+3 {
		t.Fatalf("error message not trimmed to safe length: %d", len(long))
	}
}

func TestAIDoRequestRetryRebuildsBody(t *testing.T) {
	cfg := &config.Config{}
	cfg.AI.Provider = "custom"
	cfg.AI.Endpoint = "https://1.1.1.1/v1"
	cfg.AI.APIKey = "test-key"

	payload := []byte(`{"model":"test","messages":[{"role":"user","content":"hello"}]}`)
	var bodies []string
	client := &http.Client{Transport: aiRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		bodies = append(bodies, string(body))
		status := http.StatusOK
		responseBody := `{"choices":[]}`
		if len(bodies) == 1 {
			status = http.StatusInternalServerError
			responseBody = `{"error":"retry"}`
		}
		return &http.Response{
			StatusCode: status,
			Body:       io.NopCloser(strings.NewReader(responseBody)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}
	s := &Server{cfg: cfg, httpClient: client}
	resp, err := s.aiDoRequest(context.Background(), payload)
	if err != nil {
		t.Fatalf("aiDoRequest: %v", err)
	}
	resp.Body.Close()
	if len(bodies) != 2 {
		t.Fatalf("request count = %d, want 2", len(bodies))
	}
	for i, body := range bodies {
		if body != string(payload) {
			t.Fatalf("request %d body = %q, want %q", i+1, body, payload)
		}
	}
}

func TestAIDoRequestRetryKeepsProviderSnapshot(t *testing.T) {
	cfg := &config.Config{}
	cfg.AI.Enabled = true
	cfg.AI.Provider = "custom"
	cfg.AI.Endpoint = "https://1.1.1.1/v1"
	cfg.AI.APIKey = "original-key"
	cfg.AI.Model = "original-model"

	var urls, authHeaders []string
	attempt := 0
	s := &Server{cfg: cfg}
	s.httpClient = &http.Client{Transport: aiRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		attempt++
		urls = append(urls, req.URL.String())
		authHeaders = append(authHeaders, req.Header.Get("Authorization"))
		status := http.StatusOK
		body := `{"choices":[]}`
		if attempt == 1 {
			status = http.StatusInternalServerError
			body = `{"error":"retry"}`
			s.configMu.Lock()
			s.cfg.AI.Provider = "claude"
			s.cfg.AI.Endpoint = "https://api.anthropic.com/v1"
			s.cfg.AI.APIKey = "changed-key"
			s.configMu.Unlock()
		}
		return &http.Response{
			StatusCode: status,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}

	resp, err := s.aiDoRequest(context.Background(), []byte(`{"model":"test"}`))
	if err != nil {
		t.Fatalf("aiDoRequest: %v", err)
	}
	resp.Body.Close()
	if len(urls) != 2 || urls[0] != urls[1] {
		t.Fatalf("retry endpoint changed with live config: %#v", urls)
	}
	for _, header := range authHeaders {
		if header != "Bearer original-key" {
			t.Fatalf("retry credential changed with live config: %#v", authHeaders)
		}
	}
}

func TestAIDoRequestCancellationStopsBackoff(t *testing.T) {
	cfg := &config.Config{}
	cfg.AI.Provider = "custom"
	cfg.AI.Endpoint = "https://1.1.1.1/v1"
	cfg.AI.APIKey = "test-key"
	s := &Server{cfg: cfg, httpClient: &http.Client{Transport: aiRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("upstream unavailable")
	})}}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	_, err := s.aiDoRequest(ctx, []byte(`{}`))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("cancelled request waited through backoff: %s", elapsed)
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
