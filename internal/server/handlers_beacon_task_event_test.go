package server

import (
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
)

// waitForEvent waits up to 2s for the handler to receive an event of the given
// type, returning it via the channel.
func waitForEvent(t *testing.T, evtCh chan Event, want EventType) Event {
	t.Helper()
	select {
	case evt := <-evtCh:
		if evt.Type != want {
			t.Fatalf("expected event %s, got %s", want, evt.Type)
		}
		return evt
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for event %s", want)
		return Event{}
	}
}

// TestProcessTaskResultsEmitsTaskEvents verifies generic task results now emit
// EventTaskComplete / EventTaskFail, which previously never fired (leaving the
// webhook/email/automation subscribers in server.go dormant).
func TestProcessTaskResultsEmitsTaskEvents(t *testing.T) {
	database := testutil.SetupTestDB(t)
	sm, err := crypto.NewSessionManager()
	if err != nil {
		t.Fatalf("new session manager: %v", err)
	}
	s := &Server{
		db:                    database,
		cfg:                   &config.Config{},
		sessionManager:        sm,
		eventManager:          NewEventManager(database),
		agentPendingTasks:     make(map[string]int),
		beaconDedupCache:      make(map[string]time.Time),
		screenMonitorImplants: make(map[string]time.Time),
	}
	defer s.eventManager.Shutdown()

	uuid := "dddd4444-5555-4333-8444-666666666666"
	now := time.Now()
	agent := db.Implant{ID: uuid, Hostname: "WIN-DC01", IP: "10.0.0.5"}
	if err := database.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}

	okTask := db.Task{AgentID: uuid, Type: "shell", Command: "whoami", Status: "pending"}
	if err := database.Create(&okTask).Error; err != nil {
		t.Fatalf("create ok task: %v", err)
	}
	failTask := db.Task{AgentID: uuid, Type: "shell", Command: "dir", Status: "pending"}
	if err := database.Create(&failTask).Error; err != nil {
		t.Fatalf("create fail task: %v", err)
	}

	completeCh := make(chan Event, 1)
	failCh := make(chan Event, 1)
	s.eventManager.On(EventTaskComplete, func(evt Event) { completeCh <- evt })
	s.eventManager.On(EventTaskFail, func(evt Event) { failCh <- evt })

	s.processTaskResults(agent, []taskResult{
		{TaskID: okTask.ID, Type: "shell", Output: "administrator", Encoding: "", ResultID: "rid-ok"},
		{TaskID: failTask.ID, Type: "shell", Output: "", Error: "access denied", ResultID: "rid-fail"},
	}, uuid, now)

	okEvt := waitForEvent(t, completeCh, EventTaskComplete)
	if okEvt.TaskID != okTask.ID {
		t.Errorf("expected complete event task_id=%d, got %d", okTask.ID, okEvt.TaskID)
	}
	if okEvt.AgentID != uuid {
		t.Errorf("expected complete event agent %s, got %s", uuid, okEvt.AgentID)
	}

	failEvt := waitForEvent(t, failCh, EventTaskFail)
	if failEvt.TaskID != failTask.ID {
		t.Errorf("expected fail event task_id=%d, got %d", failTask.ID, failEvt.TaskID)
	}

	// Persisted statuses must reflect the event.
	var okReloaded db.Task
	if err := database.First(&okReloaded, okTask.ID).Error; err != nil {
		t.Fatalf("reload ok task: %v", err)
	}
	if okReloaded.Status != "completed" {
		t.Errorf("expected ok task status completed, got %q", okReloaded.Status)
	}
	var failReloaded db.Task
	if err := database.First(&failReloaded, failTask.ID).Error; err != nil {
		t.Fatalf("reload fail task: %v", err)
	}
	if failReloaded.Status != "failed" {
		t.Errorf("expected fail task status failed, got %q", failReloaded.Status)
	}
}

// TestSIEMRulesMatchEmittedActions pins the correlation-rule actions to the
// actions actually produced by the audit/event pipeline so rules are not dead.
func TestSIEMRulesMatchEmittedActions(t *testing.T) {
	ec := NewEventCorrelator()
	want := map[string]bool{
		"login_failed":      true,
		"implant_checkin":   true,
		"credential_ingest": true,
		"cancel_task":       true,
		"agent_elevated":    true,
	}
	for _, rule := range ec.rules {
		if !want[rule.Action] {
			t.Errorf("rule %q references action %q which is never emitted by the pipeline", rule.Name, rule.Action)
		}
	}
}
