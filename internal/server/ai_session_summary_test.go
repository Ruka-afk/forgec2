package server

import (
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
)

// TestSessionSummaryBelowThresholdNoop verifies short sessions are never
// folded and inject nothing into the prompt.
func TestSessionSummaryBelowThresholdNoop(t *testing.T) {
	s := setupAIRunStorageTest(t)
	s.db.Create(&db.AIChatSession{ID: 11, TenantID: 1, OwnerID: 1, Owner: "alice"})
	for i := 0; i < 5; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		s.db.Create(&db.AIChatMessage{SessionID: 11, Role: role, Content: "hello"})
	}
	// AI not configured in this fixture (no API key) — must no-op safely.
	s.maybeSummarizeSession(11)
	var session db.AIChatSession
	s.db.First(&session, 11)
	if session.Summary != "" || session.SummaryUpToID != 0 {
		t.Fatalf("short session must not be summarized: %+v", session.Summary)
	}
	if block := s.sessionSummaryForPrompt(11); block != "" {
		t.Fatalf("empty summary must inject nothing, got %q", block)
	}
	if block := s.sessionSummaryForPrompt(0); block != "" {
		t.Fatalf("zero session must inject nothing, got %q", block)
	}
}

// TestSessionSummaryBlockFormat verifies the injection block shape once a
// digest exists.
func TestSessionSummaryBlockFormat(t *testing.T) {
	s := setupAIRunStorageTest(t)
	s.db.Create(&db.AIChatSession{ID: 12, TenantID: 1, OwnerID: 1, Owner: "alice", Summary: "operator hunts creds on host-01", SummaryUpToID: 40})
	block := s.sessionSummaryForPrompt(12)
	if !strings.Contains(block, "auto-summary") || !strings.Contains(block, "host-01") {
		t.Fatalf("bad summary block: %q", block)
	}
}
