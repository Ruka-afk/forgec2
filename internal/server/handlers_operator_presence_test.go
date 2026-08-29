package server

import (
	"testing"
	"time"
)

// newPresenceSession builds a WSOperatorSession suitable for presence tests
// (lastSeenNano is atomic, so it must be set after construction).
func newPresenceSession(userID uint, username string, agentView string, lastSeen time.Time) *WSOperatorSession {
	s := &WSOperatorSession{
		UserID:    userID,
		Username:  username,
		AgentView: agentView,
	}
	s.lastSeenNano.Store(lastSeen.UnixNano())
	return s
}

// TestActiveOperatorsForAgent verifies that the tracker correctly identifies
// operators viewing a specific agent within the heartbeat window.
func TestActiveOperatorsForAgent(t *testing.T) {
	tr := &operatorSessionTracker{sessions: make(map[uint]*WSOperatorSession)}
	now := time.Now()

	// alice is viewing agent-1, recently active
	tr.sessions[1] = newPresenceSession(1, "alice", "agent-1", now)
	// bob is viewing agent-1, but stale (beyond heartbeat window)
	tr.sessions[2] = newPresenceSession(2, "bob", "agent-1", now.Add(-2*operatorHeartbeatTimeout))
	// carol is viewing agent-2, recently active
	tr.sessions[3] = newPresenceSession(3, "carol", "agent-2", now)

	// Query for agent-1, excluding alice (user 1) — only active operators, excluding self
	others := tr.ActiveOperatorsForAgent("agent-1", 1)
	if len(others) != 0 {
		t.Fatalf("expected 0 active operators for agent-1 (excluding alice), got %v", others)
	}

	// Query for agent-1, excluding nobody — bob is stale, should not appear
	others = tr.ActiveOperatorsForAgent("agent-1", 0)
	if len(others) != 1 || others[0] != "alice" {
		t.Fatalf("expected [alice] for agent-1, got %v", others)
	}

	// Query for agent-2, excluding carol — should be empty
	others = tr.ActiveOperatorsForAgent("agent-2", 3)
	if len(others) != 0 {
		t.Fatalf("expected 0 for agent-2 (excluding carol), got %v", others)
	}

	// Query for unknown agent — empty
	others = tr.ActiveOperatorsForAgent("agent-999", 0)
	if len(others) != 0 {
		t.Fatalf("expected 0 for unknown agent, got %v", others)
	}
}

// TestActiveOperatorCount verifies counting of recent operators.
func TestActiveOperatorCount(t *testing.T) {
	tr := &operatorSessionTracker{sessions: make(map[uint]*WSOperatorSession)}
	now := time.Now()

	tr.sessions[1] = newPresenceSession(1, "", "", now)
	tr.sessions[2] = newPresenceSession(2, "", "", now.Add(-2*operatorHeartbeatTimeout))
	tr.sessions[3] = newPresenceSession(3, "", "", now)

	if count := tr.ActiveOperatorCount(); count != 2 {
		t.Fatalf("expected 2 active, got %d", count)
	}
}

// TestOperatorPresenceSnapshot verifies the snapshot includes only active operators.
func TestOperatorPresenceSnapshot(t *testing.T) {
	tr := &operatorSessionTracker{sessions: make(map[uint]*WSOperatorSession)}
	now := time.Now()

	tr.sessions[1] = newPresenceSession(1, "alice", "agent-1", now)
	tr.sessions[2] = newPresenceSession(2, "bob", "", now)
	tr.sessions[3] = newPresenceSession(3, "carol", "agent-2", now.Add(-2*operatorHeartbeatTimeout))

	snap := tr.OperatorPresenceSnapshot()
	if len(snap) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(snap))
	}
	// Verify alice has agent_id
	for _, entry := range snap {
		user, _ := entry["user"].(string)
		if user == "alice" {
			if aid, _ := entry["agent_id"].(string); aid != "agent-1" {
				t.Fatalf("alice agent_id: expected agent-1, got %q", aid)
			}
		} else if user == "bob" {
			if _, ok := entry["agent_id"]; ok {
				t.Fatal("bob should not have agent_id")
			}
		} else {
			t.Fatalf("unexpected user: %q", user)
		}
	}
}

// TestJoinUsernames verifies the comma-joining helper.
func TestJoinUsernames(t *testing.T) {
	if r := joinUsernames(nil); r != "" {
		t.Fatalf("nil: got %q", r)
	}
	if r := joinUsernames([]string{"alice"}); r != "alice" {
		t.Fatalf("1: got %q", r)
	}
	if r := joinUsernames([]string{"alice", "bob"}); r != "alice and bob" {
		t.Fatalf("2: got %q", r)
	}
	if r := joinUsernames([]string{"alice", "bob", "carol"}); r != "alice and 2 others" {
		t.Fatalf("3: got %q", r)
	}
}

// TestSoftLockBlocksTask verifies that createTask rejects when another operator
// is actively viewing the same agent.
func TestSoftLockBlocksTask(t *testing.T) {
	s := newTasksTestServer(t)

	// Set up a fake operator session viewing agent-lock-test
	s.operatorSessions = &operatorSessionTracker{sessions: make(map[uint]*WSOperatorSession)}
	s.operatorSessions.sessions[99] = newPresenceSession(99, "other_operator", "agent-lock-test", time.Now())

	// Create task without caller — should succeed (no soft-lock check)
	task, err := s.createTask("agent-lock-test", "shell", "whoami", "", "", "", 0, 0)
	if err != nil {
		t.Fatalf("no-caller createTask should succeed: %v", err)
	}
	_ = task

	// Create task with a different caller (user 1) — should be blocked
	_, err = s.createTask("agent-lock-test", "shell", "whoami", "", "", "", 0, 0, WithCaller(1))
	if err == nil {
		t.Fatal("expected soft-lock error, got nil")
	}
	if !isConflictError(err) {
		t.Fatalf("expected conflict error, got: %v", err)
	}

	// Create task with the same operator (user 99) — should succeed (excluded)
	task, err = s.createTask("agent-lock-test", "shell", "whoami", "", "", "", 0, 0, WithCaller(99))
	if err != nil {
		t.Fatalf("same-operator createTask should succeed: %v", err)
	}
	_ = task
}

// TestSoftLockSkippedForStaleOperator verifies that a stale operator (beyond
// heartbeat timeout) does not trigger the soft-lock.
func TestSoftLockSkippedForStaleOperator(t *testing.T) {
	s := newTasksTestServer(t)

	s.operatorSessions = &operatorSessionTracker{sessions: make(map[uint]*WSOperatorSession)}
	s.operatorSessions.sessions[99] = newPresenceSession(99, "stale_operator", "agent-stale-test",
		time.Now().Add(-2*operatorHeartbeatTimeout)) // stale

	// Should succeed — stale operator doesn't block
	task, err := s.createTask("agent-stale-test", "shell", "whoami", "", "", "", 0, 0, WithCaller(1))
	if err != nil {
		t.Fatalf("stale operator should not block: %v", err)
	}
	_ = task
}
