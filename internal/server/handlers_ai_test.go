package server

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

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
	if limits.maxConversationTurns != 0 || limits.maxToolRounds != 0 || limits.maxDuplicateToolCalls != 0 {
		t.Fatalf("zero config should mean unlimited: %+v", limits)
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
