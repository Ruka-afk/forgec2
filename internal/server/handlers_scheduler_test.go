package server

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestDispatchDueOneShotTaskClaimsAndCountsOnce(t *testing.T) {
	s := newTasksTestServer(t)
	if err := s.db.AutoMigrate(&db.OneShotTask{}); err != nil {
		t.Fatalf("migrate one-shot tasks: %v", err)
	}
	s.ctx = context.Background()
	agent := db.Implant{ID: "scheduled-agent", Status: "online"}
	if err := s.db.Create(&agent).Error; err != nil {
		t.Fatalf("seed agent: %v", err)
	}
	scheduled := db.OneShotTask{
		AgentID: agent.ID, Type: "shell", Command: "whoami",
		RunAt: time.Now().Add(-time.Minute), Status: "pending",
	}
	if err := s.db.Create(&scheduled).Error; err != nil {
		t.Fatalf("seed one-shot task: %v", err)
	}

	var workers sync.WaitGroup
	workers.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer workers.Done()
			s.dispatchDueOneShotTasks()
		}()
	}
	workers.Wait()

	var taskCount int64
	if err := s.db.Model(&db.Task{}).Where("agent_id = ?", agent.ID).Count(&taskCount).Error; err != nil {
		t.Fatalf("count dispatched tasks: %v", err)
	}
	if taskCount != 1 {
		t.Fatalf("dispatched task count = %d, want 1", taskCount)
	}
	if got := testutil.ToFloat64(s.metrics.TasksTotal); got != 1 {
		t.Fatalf("TasksTotal = %v, want 1", got)
	}
	if err := s.db.First(&scheduled, scheduled.ID).Error; err != nil {
		t.Fatalf("reload scheduled task: %v", err)
	}
	if scheduled.Status != "done" || scheduled.TaskID == 0 {
		t.Fatalf("scheduled task = status %q task_id %d", scheduled.Status, scheduled.TaskID)
	}
}

func TestRecoverClaimedOneShotTasks(t *testing.T) {
	s := newTasksTestServer(t)
	if err := s.db.AutoMigrate(&db.OneShotTask{}); err != nil {
		t.Fatalf("migrate one-shot tasks: %v", err)
	}
	claimed := db.OneShotTask{
		AgentID: "agent", Type: "shell", Command: "hostname",
		RunAt: time.Now(), Status: "dispatching",
	}
	if err := s.db.Create(&claimed).Error; err != nil {
		t.Fatalf("seed claimed task: %v", err)
	}

	s.recoverClaimedOneShotTasks()
	if err := s.db.First(&claimed, claimed.ID).Error; err != nil {
		t.Fatalf("reload claimed task: %v", err)
	}
	if claimed.Status != "pending" {
		t.Fatalf("recovered status = %q, want pending", claimed.Status)
	}
}

func TestCancelledMacroRunIsNotMarkedCompleted(t *testing.T) {
	s := newTasksTestServer(t)
	if err := s.db.AutoMigrate(&db.MacroRun{}); err != nil {
		t.Fatalf("migrate macro runs: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.ctx = ctx
	run := db.MacroRun{
		MacroName: "shutdown-test", AgentID: "agent", Status: "running",
		TotalSteps: 1, Log: "[]", StartedAt: time.Now(),
	}
	if err := s.db.Create(&run).Error; err != nil {
		t.Fatalf("seed macro run: %v", err)
	}

	s.executeMacroRun(run.ID, run.AgentID, []MacroStep{{Command: "whoami"}}, "tester", false)
	if err := s.db.First(&run, run.ID).Error; err != nil {
		t.Fatalf("reload macro run: %v", err)
	}
	if run.Status != "failed" || run.FinishedAt == nil {
		t.Fatalf("cancelled macro status = %q finished_at=%v, want failed terminal state", run.Status, run.FinishedAt)
	}
}
