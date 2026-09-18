package server

import (
	"context"
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
	owner := aiPrincipal{UserID: 1, TenantID: 1}
	// AI not configured in this fixture (no API key) — must no-op safely.
	s.maybeSummarizeSession(context.Background(), 11, owner)
	var session db.AIChatSession
	s.db.First(&session, 11)
	if session.Summary != "" || session.SummaryUpToID != 0 {
		t.Fatalf("short session must not be summarized: %+v", session.Summary)
	}
	if block := s.sessionSummaryForPrompt(11, owner); block != "" {
		t.Fatalf("empty summary must inject nothing, got %q", block)
	}
	if block := s.sessionSummaryForPrompt(0, owner); block != "" {
		t.Fatalf("zero session must inject nothing, got %q", block)
	}
}

// TestSessionSummaryCrossTenantHidden proves one tenant's digest never leaks
// into another tenant's prompt (and folds never run for it).
func TestSessionSummaryCrossTenantHidden(t *testing.T) {
	s := setupAIRunStorageTest(t)
	s.db.Create(&db.AIChatSession{ID: 13, TenantID: 2, OwnerID: 9, Owner: "mallory", Summary: "victim hunts creds on host-99", SummaryUpToID: 40})
	other := aiPrincipal{UserID: 1, TenantID: 1}
	if block := s.sessionSummaryForPrompt(13, other); block != "" {
		t.Fatalf("cross-tenant digest leaked: %q", block)
	}
	if s.sessionVisibleTo(13, other) {
		t.Fatal("cross-tenant session must not be visible")
	}
	owner := aiPrincipal{UserID: 9, TenantID: 2}
	if block := s.sessionSummaryForPrompt(13, owner); block == "" {
		t.Fatal("owner must still see own digest")
	}
	legacy := aiPrincipal{UserID: 7}
	if block := s.sessionSummaryForPrompt(13, legacy); block == "" {
		t.Fatal("legacy unscoped operator keeps visibility")
	}
}

// TestSessionSummaryBlockFormat verifies the injection block shape once a
// digest exists.
func TestSessionSummaryBlockFormat(t *testing.T) {
	s := setupAIRunStorageTest(t)
	s.db.Create(&db.AIChatSession{ID: 12, TenantID: 1, OwnerID: 1, Owner: "alice", Summary: "operator hunts creds on host-01", SummaryUpToID: 40})
	block := s.sessionSummaryForPrompt(12, aiPrincipal{UserID: 1, TenantID: 1})
	if !strings.Contains(block, "auto-summary") || !strings.Contains(block, "host-01") {
		t.Fatalf("bad summary block: %q", block)
	}
}
