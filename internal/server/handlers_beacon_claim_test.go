package server

import (
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
)

func TestFetchPendingTasksClaimsOnlyPendingTasks(t *testing.T) {
	database := testutil.SetupTestDB(t)
	s := &Server{db: database}
	agentID := "agent-1"
	now := time.Now()

	oldRunning := db.Task{AgentID: agentID, Type: "old", Status: "running", Priority: 3, CreatedAt: now.Add(-time.Minute)}
	pendingLow := db.Task{AgentID: agentID, Type: "low", Status: "pending", Priority: 0, CreatedAt: now}
	pendingHigh := db.Task{AgentID: agentID, Type: "high", Status: "pending", Priority: 3, CreatedAt: now.Add(time.Second)}
	for _, task := range []*db.Task{&oldRunning, &pendingLow, &pendingHigh} {
		if err := database.Create(task).Error; err != nil {
			t.Fatalf("create task: %v", err)
		}
	}

	claimed := s.fetchPendingTasks(agentID)
	if len(claimed) != 2 {
		t.Fatalf("expected 2 newly claimed tasks, got %d", len(claimed))
	}
	if claimed[0].ID != pendingHigh.ID || claimed[1].ID != pendingLow.ID {
		t.Fatalf("unexpected claimed task order: %+v", claimed)
	}

	var persistedOld db.Task
	if err := database.First(&persistedOld, oldRunning.ID).Error; err != nil {
		t.Fatalf("load old running task: %v", err)
	}
	if persistedOld.ClaimedBy != "" {
		t.Fatalf("existing running task was changed: %+v", persistedOld)
	}

	var persistedClaimed []db.Task
	if err := database.Where("id IN ?", []uint{pendingHigh.ID, pendingLow.ID}).Find(&persistedClaimed).Error; err != nil {
		t.Fatalf("load claimed tasks: %v", err)
	}
	for _, claimedTask := range persistedClaimed {
		if claimedTask.Status != "running" || claimedTask.ClaimedBy != agentID || claimedTask.ClaimedAt.IsZero() {
			t.Fatalf("task was not recorded as claimed: %+v", claimedTask)
		}
	}
}

func TestFetchPendingTasksRespectsCapacity(t *testing.T) {
	database := testutil.SetupTestDB(t)
	s := &Server{db: database}
	agentID := "agent-capacity"
	for i := 0; i < 2; i++ {
		if err := database.Create(&db.Task{AgentID: agentID, Type: "shell", Status: "pending"}).Error; err != nil {
			t.Fatalf("create task: %v", err)
		}
	}
	if tasks := s.fetchPendingTasks(agentID, 0); len(tasks) != 0 {
		t.Fatalf("expected no tasks at zero capacity, got %d", len(tasks))
	}
	if tasks := s.fetchPendingTasks(agentID, 1); len(tasks) != 1 {
		t.Fatalf("expected one task at capacity one, got %d", len(tasks))
	}
}

func TestTaskAcknowledgementPreventsStaleRequeue(t *testing.T) {
	database := testutil.SetupTestDB(t)
	s := &Server{db: database}
	agentID := "agent-ack"
	claimedAt := time.Now().Add(-StaleRunningTaskTimeout - time.Minute)
	acknowledgedAt := claimedAt.Add(time.Minute)

	unacknowledged := db.Task{AgentID: agentID, Type: "unacknowledged", Status: "running", ClaimedBy: agentID, ClaimedAt: claimedAt}
	acknowledged := db.Task{AgentID: agentID, Type: "acknowledged", Status: "running", ClaimedBy: agentID, ClaimedAt: claimedAt, AcknowledgedAt: &acknowledgedAt}
	fresh := db.Task{AgentID: agentID, Type: "fresh", Status: "running", ClaimedBy: agentID, ClaimedAt: time.Now()}
	for _, task := range []*db.Task{&unacknowledged, &acknowledged, &fresh} {
		if err := database.Create(task).Error; err != nil {
			t.Fatalf("create task: %v", err)
		}
	}
	s.processTaskAcknowledgements(agentID, []uint{fresh.ID, fresh.ID}, time.Now())
	if err := database.First(&fresh, fresh.ID).Error; err != nil {
		t.Fatalf("load acknowledged task: %v", err)
	}
	if fresh.AcknowledgedAt == nil {
		t.Fatal("running task acknowledgement was not recorded")
	}

	s.requeueStaleTasks()

	var reloadedUnacknowledged, reloadedAcknowledged db.Task
	if err := database.First(&reloadedUnacknowledged, unacknowledged.ID).Error; err != nil {
		t.Fatalf("load unacknowledged task: %v", err)
	}
	if err := database.First(&reloadedAcknowledged, acknowledged.ID).Error; err != nil {
		t.Fatalf("load acknowledged task: %v", err)
	}
	if reloadedUnacknowledged.Status != "pending" || reloadedUnacknowledged.ClaimedBy != "" || !reloadedUnacknowledged.ClaimedAt.IsZero() {
		t.Fatalf("unacknowledged task was not released: %+v", reloadedUnacknowledged)
	}
	if reloadedAcknowledged.Status != "running" || reloadedAcknowledged.AcknowledgedAt == nil {
		t.Fatalf("acknowledged task was requeued: %+v", reloadedAcknowledged)
	}

	s.processTaskAcknowledgements(agentID, []uint{unacknowledged.ID, unacknowledged.ID}, time.Now())
	if err := database.First(&reloadedUnacknowledged, unacknowledged.ID).Error; err != nil {
		t.Fatalf("reload task: %v", err)
	}
	if reloadedUnacknowledged.AcknowledgedAt != nil {
		t.Fatal("pending task acknowledgement should be ignored")
	}
}
