package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func seedCollabUser(t *testing.T, s *Server, username, role string) uint {
	t.Helper()
	u := db.User{Username: username, Role: role, IsActive: true}
	if err := s.db.Create(&u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return u.ID
}

func TestCollabLockExpiryAndOwnership(t *testing.T) {
	s := newTasksTestServer(t)
	seedAgent(t, s, "agent-lock")
	alice := seedCollabUser(t, s, "alice", "operator")
	_ = alice
	bob := seedCollabUser(t, s, "bob", "operator")
	_ = bob
	seedCollabUser(t, s, "root", "admin")

	// Alice locks.
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/", nil)
	c.Params = gin.Params{{Key: "id", Value: "agent-lock"}}
	c.Set("user", "alice")
	c.Set("user_role", "operator")
	// getAgentOrFail needs tenant scope; implant has zero tenant. Seed tenant-visible agent instead.
	s.handleCollabLock(c)
	if w.Code != http.StatusOK {
		t.Fatalf("alice lock: %d %s", w.Code, w.Body.String())
	}

	// Bob takes over without force -> 409.
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request, _ = http.NewRequest(http.MethodPost, "/", nil)
	c2.Params = gin.Params{{Key: "id", Value: "agent-lock"}}
	c2.Set("user", "bob")
	c2.Set("user_role", "operator")
	s.handleCollabLock(c2)
	if w2.Code != http.StatusConflict {
		t.Fatalf("bob takeover without force: got %d, want 409", w2.Code)
	}

	// Bob cannot unlock Alice's live lock -> 403.
	w3 := httptest.NewRecorder()
	c3, _ := gin.CreateTestContext(w3)
	c3.Request, _ = http.NewRequest(http.MethodPost, "/", nil)
	c3.Params = gin.Params{{Key: "id", Value: "agent-lock"}}
	c3.Set("user", "bob")
	c3.Set("user_role", "operator")
	s.handleCollabUnlock(c3)
	if w3.Code != http.StatusForbidden {
		t.Fatalf("bob unlock of alice lock: got %d, want 403", w3.Code)
	}

	// Expire the lock manually; Bob can now take it.
	past := time.Now().Add(-time.Hour)
	if err := s.db.Model(&db.AgentLock{}).Where("agent_id = ?", "agent-lock").Update("expires_at", past).Error; err != nil {
		t.Fatalf("expire lock: %v", err)
	}
	w4 := httptest.NewRecorder()
	c4, _ := gin.CreateTestContext(w4)
	c4.Request, _ = http.NewRequest(http.MethodPost, "/", nil)
	c4.Params = gin.Params{{Key: "id", Value: "agent-lock"}}
	c4.Set("user", "bob")
	c4.Set("user_role", "operator")
	s.handleCollabLock(c4)
	if w4.Code != http.StatusOK {
		t.Fatalf("bob lock after expiry: got %d %s", w4.Code, w4.Body.String())
	}
}

func TestCollabTaskClaimSeparatedFromBeaconClaim(t *testing.T) {
	s := newTasksTestServer(t)
	seedAgent(t, s, "agent-claim")
	seedCollabUser(t, s, "carol", "operator")

	// Beacon-style claim state as dispatch would leave it.
	task := seedTask(t, s, "agent-claim", "shell", "whoami", "running")
	s.db.Model(&task).Updates(map[string]interface{}{"claimed_by": "agent-claim"})

	// Operator claim must NOT touch the beacon claim columns.
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/tasks/1/claim", nil)
	c.Params = gin.Params{{Key: "taskId", Value: "1"}}
	c.Set("user", "carol")
	c.Set("user_role", "operator")
	s.handleCollabClaimTask(c)
	if w.Code != http.StatusOK {
		t.Fatalf("claim: got %d %s", w.Code, w.Body.String())
	}

	var reloaded db.Task
	s.db.First(&reloaded, task.ID)
	if reloaded.ClaimedBy != "agent-claim" {
		t.Fatalf("beacon claim must survive operator actions, got %q", reloaded.ClaimedBy)
	}
	if reloaded.OperatorClaimedBy != "carol" {
		t.Fatalf("operator claim not recorded, got %q", reloaded.OperatorClaimedBy)
	}

	// Another operator cannot steal the claim; release by non-holder fails.
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request, _ = http.NewRequest(http.MethodPost, "/tasks/1/claim", nil)
	c2.Params = gin.Params{{Key: "taskId", Value: "1"}}
	c2.Set("user", "dave")
	c2.Set("user_role", "operator")
	s.handleCollabClaimTask(c2)
	if w2.Code != http.StatusConflict {
		t.Fatalf("claim steal: got %d, want 409", w2.Code)
	}
}

func TestCreateTaskRespectsCollabLock(t *testing.T) {
	s := newTasksTestServer(t)
	seedAgent(t, s, "agent-gate")
	aliceID := seedCollabUser(t, s, "gate-alice", "operator")
	bobID := seedCollabUser(t, s, "gate-bob", "operator")

	now := time.Now()
	if err := s.db.Create(&db.AgentLock{AgentID: "agent-gate", LockedBy: "gate-alice", LockedAt: now, ExpiresAt: now.Add(15 * time.Minute)}).Error; err != nil {
		t.Fatalf("seed lock: %v", err)
	}

	// Holder writes freely.
	if _, err := s.createTask("agent-gate", "shell", "whoami", "", "", "", 0, 0, WithCaller(aliceID)); err != nil {
		t.Fatalf("holder createTask: %v", err)
	}
	// Other operator blocked.
	if _, err := s.createTask("agent-gate", "shell", "whoami", "", "", "", 0, 0, WithCaller(bobID)); err == nil {
		t.Fatal("non-holder createTask must be rejected")
	} else if !isConflictError(err) {
		t.Fatalf("expected conflict error, got %v", err)
	}
	// System paths (no caller) bypass UI locks.
	if _, err := s.createTask("agent-gate", "shell", "whoami", "", "", "", 0, 0); err != nil {
		t.Fatalf("system createTask must bypass UI lock: %v", err)
	}
	// Explicit admin force with reason passes.
	if _, err := s.createTask("agent-gate", "shell", "whoami", "", "", "", 0, 0, WithCaller(bobID), WithForceWrite("test override")); err != nil {
		t.Fatalf("forced createTask: %v", err)
	}
}
