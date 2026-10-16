package server

import (
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
)

// TestWorkflowJumpBudgetBoundsSelfJumps proves a step whose branch target is
// itself cannot spin forever. The per-agent jump budget used to be declared
// inside the step loop, so every outer iteration reset it and the bound never
// fired: a self-jumping workflow re-created tasks indefinitely (P2).
//
// The engine runs in a goroutine so a regression fails fast on the timeout
// instead of hanging the package until `go test -timeout` kills it.
func TestWorkflowJumpBudgetBoundsSelfJumps(t *testing.T) {
	s := newTasksTestServer(t)
	s.cfg = &config.Config{}

	wf := db.Workflow{ID: "wf-self-jump", Name: "self jump", Enabled: true, ScopeType: "all"}
	if err := s.db.Create(&wf).Error; err != nil {
		t.Fatalf("create workflow: %v", err)
	}
	step := db.WorkflowStep{
		WorkflowID: wf.ID,
		StepOrder:  1,
		TaskType:   "shell",
		Command:    "whoami",
		Shell:      "cmd",
		// One second keeps the test quick; the queued task is never completed
		// by an agent, so the step result stays empty and the condition
		// routes to on_failure.
		TimeoutSec: 1,
		Condition:  "not_empty",
		OnFailure:  "1", // jump back to this same step
	}
	if err := s.db.Create(&step).Error; err != nil {
		t.Fatalf("create step: %v", err)
	}
	// The step "fails" by timeout in this test and stop_on_failure would abort
	// before the branch is evaluated. GORM drops the zero value for fields
	// that declare a DB default, so force the column explicitly.
	if err := s.db.Model(&step).Update("stop_on_failure", false).Error; err != nil {
		t.Fatalf("disable stop_on_failure: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _, _ = NewWorkflowEngine(s).ExecuteWorkflow(wf, []string{"agent-jump"})
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("self-jumping workflow never finished: the jump budget is not bounded")
	}

	var count int64
	if err := s.db.Model(&db.Task{}).Where("agent_id = ?", "agent-jump").Count(&count).Error; err != nil {
		t.Fatalf("count tasks: %v", err)
	}
	// Budget = len(steps) = 1: the step runs once, spends the jump on itself,
	// runs a second time, and the next jump is refused.
	if count != 2 {
		t.Fatalf("self-jumping step created %d tasks, want 2 (initial run + one bounded jump)", count)
	}
}
