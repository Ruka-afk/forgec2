package server

import (
	"strings"
	"sync"
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

// TestCreateTaskIdempotencyConcurrent hammers the same key from goroutines:
// the SELECT-then-INSERT race must collapse via the partial unique index
// into exactly one row, with every caller receiving the winner.
// NOTE: test DBs run AutoMigrate only (no index migrations), so the
// production partial unique index is created explicitly here.
func TestCreateTaskIdempotencyConcurrent(t *testing.T) {
	s := newTasksTestServer(t)
	seedAgent(t, s, "agent-race")
	if err := s.db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS uidx_tasks_idem_live ON tasks(agent_id, idempotency_key) WHERE status IN ('pending','pending_approval','running') AND idempotency_key <> ''").Error; err != nil {
		t.Fatalf("create partial unique index: %v", err)
	}

	const racers = 16
	ids := make([]uint, racers)
	errs := make([]error, racers)
	var wg sync.WaitGroup
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			task, err := s.createTask("agent-race", "shell", "whoami", "", "", "", 0, 0, WithIdempotencyKey("op-race"))
			if err == nil {
				ids[i] = task.ID
			}
			errs[i] = err
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("racer %d: %v", i, err)
		}
		if ids[i] != ids[0] {
			t.Fatalf("racer %d got task %d, want %d", i, ids[i], ids[0])
		}
	}
	var count int64
	s.db.Model(&db.Task{}).Where("agent_id = ?", "agent-race").Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 task row after race, got %d", count)
	}
}
