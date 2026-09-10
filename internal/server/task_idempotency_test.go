package server

import (
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
)

// TestCreateTaskIdempotencyKey verifies duplicate creation with the same key
// returns the live task instead of inserting a second row, terminal tasks
// free the key, and keys are scoped per agent.
func TestCreateTaskIdempotencyKey(t *testing.T) {
	s := newTasksTestServer(t)
	seedAgent(t, s, "agent-1")
	seedAgent(t, s, "agent-2")

	first, err := s.createTask("agent-1", "shell", "whoami", "", "", "", 0, 0, WithIdempotencyKey("op-1"))
	if err != nil {
		t.Fatalf("createTask: %v", err)
	}
	dup, err := s.createTask("agent-1", "shell", "whoami", "", "", "", 0, 0, WithIdempotencyKey("op-1"))
	if err != nil {
		t.Fatalf("duplicate createTask: %v", err)
	}
	if dup.ID != first.ID {
		t.Fatalf("duplicate key returned new task %d, want %d", dup.ID, first.ID)
	}
	var count int64
	s.db.Model(&db.Task{}).Where("agent_id = ?", "agent-1").Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 task row, got %d", count)
	}

	// Same key on another agent is independent.
	other, err := s.createTask("agent-2", "shell", "whoami", "", "", "", 0, 0, WithIdempotencyKey("op-1"))
	if err != nil {
		t.Fatalf("other agent createTask: %v", err)
	}
	if other.ID == first.ID {
		t.Fatal("key leaked across agents")
	}

	// Terminal task frees the key for reuse.
	s.db.Model(&db.Task{}).Where("id = ?", first.ID).Update("status", "completed")
	fresh, err := s.createTask("agent-1", "shell", "whoami", "", "", "", 0, 0, WithIdempotencyKey("op-1"))
	if err != nil {
		t.Fatalf("recreate after terminal: %v", err)
	}
	if fresh.ID == first.ID {
		t.Fatal("terminal task must not block key reuse")
	}

	// Empty key disables dedup entirely.
	a, _ := s.createTask("agent-1", "shell", "a", "", "", "", 0, 0)
	b, _ := s.createTask("agent-1", "shell", "a", "", "", "", 0, 0)
	if a.ID == b.ID {
		t.Fatal("empty keys must not dedup")
	}

	// Oversized key rejected.
	if _, err := s.createTask("agent-1", "shell", "x", "", "", "", 0, 0, WithIdempotencyKey(strings.Repeat("k", 65))); err == nil {
		t.Fatal("expected error for oversized idempotency key")
	}
}
