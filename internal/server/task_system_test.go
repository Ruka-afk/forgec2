package server

import (
	"testing"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
)

// TestCreateSystemTaskParity proves system-path creation carries the same
// accounting as createTask: cap enforcement, tenant inheritance, approval
// expiry, and counter release on failure.
func TestCreateSystemTaskParity(t *testing.T) {
	database := testutil.SetupTestDB(t)
	s := &Server{db: database, agentPendingTasks: make(map[string]int)}
	const agentID = "sys-parity"
	if err := database.Create(&db.Implant{ID: agentID, TenantID: 7}).Error; err != nil {
		t.Fatalf("seed agent: %v", err)
	}

	task, err := s.createSystemTask(agentID, "shell", "whoami", "cmd.exe", "pending", "ai")
	if err != nil {
		t.Fatalf("createSystemTask: %v", err)
	}
	if task.TenantID != 7 {
		t.Fatalf("tenant=%d, want inherited 7", task.TenantID)
	}
	if task.CreatedBy != "ai" || task.Shell != "cmd.exe" {
		t.Fatalf("fields not carried: %+v", task)
	}
	s.agentPendingTasksMu.Lock()
	n := s.agentPendingTasks[agentID]
	s.agentPendingTasksMu.Unlock()
	if n != 1 {
		t.Fatalf("counter=%d, want 1", n)
	}

	if _, err := s.createSystemTask(agentID, "shell", "x", "", "bogus", "ai"); err == nil {
		t.Fatal("invalid status must be rejected")
	}
	if _, err := s.createSystemTask("", "shell", "x", "", "pending", "ai"); err == nil {
		t.Fatal("empty agent must be rejected")
	}
}

// TestCreateSystemTaskCap proves system paths honor the same backlog cap as
// operators (previously automation inserts bypassed it entirely).
func TestCreateSystemTaskCap(t *testing.T) {
	database := testutil.SetupTestDB(t)
	s := &Server{db: database, agentPendingTasks: map[string]int{"sys-cap": MaxPendingTasksPerAgent}}
	if err := database.Create(&db.Implant{ID: "sys-cap"}).Error; err != nil {
		t.Fatalf("seed agent: %v", err)
	}
	if _, err := s.createSystemTask("sys-cap", "set_sleep", "60,0", "", "pending", "automation"); err == nil {
		t.Fatal("over-cap system task must be refused")
	}
	var count int64
	database.Model(&db.Task{}).Where("agent_id = ?", "sys-cap").Count(&count)
	if count != 0 {
		t.Fatalf("refused task persisted %d rows", count)
	}
}

// TestCreateSystemTaskApprovalExpiry proves pending_approval system tasks get
// an expiry like the operator path (otherwise they linger past the sweep).
func TestCreateSystemTaskApprovalExpiry(t *testing.T) {
	database := testutil.SetupTestDB(t)
	s := &Server{db: database, agentPendingTasks: make(map[string]int)}
	if err := database.Create(&db.Implant{ID: "sys-appr"}).Error; err != nil {
		t.Fatalf("seed agent: %v", err)
	}
	task, err := s.createSystemTask("sys-appr", "shell", "whoami", "", TaskStatusPendingApproval, "ai")
	if err != nil {
		t.Fatalf("createSystemTask: %v", err)
	}
	if task.ApprovalExpiresAt == nil {
		t.Fatal("pending_approval system task needs an expiry")
	}
}

// TestCreateTaskClampsSetSleep proves operator-created set_sleep tasks are
// bounded agent-safe: huge intervals park agents for years, huge jitter
// swings sleeps negative (floored to zero) into a tight beacon loop.
func TestCreateTaskClampsSetSleep(t *testing.T) {
	s := newTasksTestServer(t)
	cases := []struct {
		in, want string
	}{
		{"60,10", "60,10"},
		{"0,0", "1,0"},
		{"-5,-10", "1,0"},
		{"999999999,999999", "86400,100"},
		{"30,500", "30,100"},
		{"not-a-pair", "not-a-pair"},
		{"abc,def", "abc,def"},
	}
	for i, tc := range cases {
		task, err := s.createTask("clamp-agent", "set_sleep", tc.in, "", "", "", 0, 0)
		if err != nil {
			t.Fatalf("case %d (%q): %v", i, tc.in, err)
		}
		if task.Command != tc.want {
			t.Errorf("case %d (%q): command=%q, want %q", i, tc.in, task.Command, tc.want)
		}
	}
}
