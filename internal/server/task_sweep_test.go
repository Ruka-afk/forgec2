package server

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gin-gonic/gin"
)

func newSweepTestServer(t *testing.T) *Server {
	t.Helper()
	return &Server{db: testutil.SetupTestDB(t), agentPendingTasks: make(map[string]int)}
}

func pendingCount(s *Server, agentID string) int {
	s.agentPendingTasksMu.Lock()
	defer s.agentPendingTasksMu.Unlock()
	return s.agentPendingTasks[agentID]
}

// TestSweepExhaustedDecsCounter proves failing delivery-exhausted tasks
// releases their pending slots (previously the in-memory counter leaked
// until the 5-minute reconcile and falsely held MaxPendingTasksPerAgent).
func TestSweepExhaustedDecsCounter(t *testing.T) {
	s := newSweepTestServer(t)
	const agentID = "sweep-exhausted"
	s.agentPendingTasks[agentID] = 1
	task := db.Task{AgentID: agentID, Type: "shell", Status: "running",
		ClaimedBy: agentID, ClaimedAt: time.Now().Add(-2 * StaleRunningTaskTimeout),
		DeliveryAttempts: 3}
	if err := s.db.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}

	s.requeueStaleTasks()

	var reloaded db.Task
	if err := s.db.First(&reloaded, task.ID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if reloaded.Status != "failed" {
		t.Fatalf("exhausted task status=%q, want failed", reloaded.Status)
	}
	if n := pendingCount(s, agentID); n != 0 {
		t.Fatalf("pending counter=%d, want 0 after sweep-fail", n)
	}
}

// TestSweepAckedFailDecsCounter proves failing acked-but-resultless tasks
// releases their pending slots too.
func TestSweepAckedFailDecsCounter(t *testing.T) {
	s := newSweepTestServer(t)
	const agentID = "sweep-acked"
	s.agentPendingTasks[agentID] = 1
	ackedAt := time.Now().Add(-2 * AckedTaskResultTimeout)
	task := db.Task{AgentID: agentID, Type: "shell", Status: "running",
		ClaimedBy: agentID, ClaimedAt: ackedAt.Add(-time.Minute), AcknowledgedAt: &ackedAt}
	if err := s.db.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}

	s.failStaleAcknowledgedTasks()

	var reloaded db.Task
	if err := s.db.First(&reloaded, task.ID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if reloaded.Status != "failed" {
		t.Fatalf("acked-stale task status=%q, want failed", reloaded.Status)
	}
	if n := pendingCount(s, agentID); n != 0 {
		t.Fatalf("pending counter=%d, want 0 after sweep-fail", n)
	}
}

// TestSweepRequeueKeepsCounter proves the retry path (running→pending)
// keeps the slot: the task still occupies backlog until it finals.
func TestSweepRequeueKeepsCounter(t *testing.T) {
	s := newSweepTestServer(t)
	const agentID = "sweep-requeue"
	s.agentPendingTasks[agentID] = 1
	task := db.Task{AgentID: agentID, Type: "shell", Status: "running",
		ClaimedBy: agentID, ClaimedAt: time.Now().Add(-2 * StaleRunningTaskTimeout)}
	if err := s.db.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}

	s.requeueStaleTasks()

	var reloaded db.Task
	if err := s.db.First(&reloaded, task.ID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if reloaded.Status != "pending" {
		t.Fatalf("stale task status=%q, want pending", reloaded.Status)
	}
	if n := pendingCount(s, agentID); n != 1 {
		t.Fatalf("pending counter=%d, want 1 after requeue", n)
	}
}

// TestFlippedSetFiltersToActualStatus proves sweep side effects calibrate to
// rows actually flipped: only IDs currently carrying the terminal status are
// returned, so a concurrent transition is never broadcast or double-dec'd.
func TestFlippedSetFiltersToActualStatus(t *testing.T) {
	s := newSweepTestServer(t)
	mk := func(status string) uint {
		task := db.Task{AgentID: "flip", Type: "shell", Status: status}
		if err := s.db.Create(&task).Error; err != nil {
			t.Fatalf("seed task: %v", err)
		}
		return task.ID
	}
	failed1, failed2, running := mk("failed"), mk("failed"), mk("running")

	got, ok := s.flippedSet([]uint{failed1, failed2, running}, "failed")
	if !ok {
		t.Fatal("re-read ok=false on healthy DB")
	}
	if len(got) != 2 || !got[failed1] || !got[failed2] {
		t.Fatalf("flipped set=%v, want exactly the 2 failed rows", got)
	}

	empty, ok := s.flippedSet(nil, "failed")
	if !ok || len(empty) != 0 {
		t.Fatalf("empty input: ok=%v len=%d", ok, len(empty))
	}
}

// TestSweepSkipsCompletedRace proves a result landing between the sweep
// SELECT and UPDATE is neither rebroadcast as failed nor double-dec'd: the
// guarded UPDATE skips it and calibration drops it from side effects.
func TestSweepSkipsCompletedRace(t *testing.T) {
	s := newSweepTestServer(t)
	const agentID = "sweep-race"
	s.agentPendingTasks[agentID] = 1
	task := db.Task{AgentID: agentID, Type: "shell", Status: "running",
		ClaimedBy: agentID, ClaimedAt: time.Now().Add(-2 * StaleRunningTaskTimeout),
		DeliveryAttempts: 3}
	if err := s.db.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}
	// A result lands after the sweep's SELECT window would have read it:
	// flip to completed up front (same end state as winning the race).
	if err := s.db.Model(&db.Task{}).Where("id = ?", task.ID).Update("status", "completed").Error; err != nil {
		t.Fatalf("flip task: %v", err)
	}

	s.requeueStaleTasks()

	var reloaded db.Task
	if err := s.db.First(&reloaded, task.ID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if reloaded.Status != "completed" {
		t.Fatalf("status=%q, want completed (untouched by sweep)", reloaded.Status)
	}
	if n := pendingCount(s, agentID); n != 1 {
		t.Fatalf("pending counter=%d, want 1 (no phantom dec)", n)
	}
}

// TestSweepRetiresSentZombie proves legacy "sent" rows (nothing creates or
// claims sent anymore) are failed, never requeued: resurrecting them would
// redeliver stale commands to live agents.
func TestSweepRetiresSentZombie(t *testing.T) {
	s := newSweepTestServer(t)
	const agentID = "sweep-sent"
	s.agentPendingTasks[agentID] = 1
	task := db.Task{AgentID: agentID, Type: "shell", Status: "sent",
		CreatedAt: time.Now().Add(-2 * StaleRunningTaskTimeout)}
	if err := s.db.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}

	s.requeueStaleTasks()

	var reloaded db.Task
	if err := s.db.First(&reloaded, task.ID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if reloaded.Status != "failed" {
		t.Fatalf("sent zombie status=%q, want failed", reloaded.Status)
	}
	if n := pendingCount(s, agentID); n != 0 {
		t.Fatalf("pending counter=%d, want 0 after retire", n)
	}
}

// TestSentTaskCancellable proves the operator can cancel a legacy sent row
// instead of it sitting permanently un-cancellable.
func TestSentTaskCancellable(t *testing.T) {
	database := testutil.SetupTestDB(t)
	s := &Server{db: database, agentPendingTasks: map[string]int{"sent-cancel": 1}}
	if err := database.Create(&db.Implant{ID: "sent-cancel"}).Error; err != nil {
		t.Fatalf("seed agent: %v", err)
	}
	task := db.Task{AgentID: "sent-cancel", Type: "shell", Command: "sleep 1", Status: "sent"}
	if err := database.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/", nil)
	c.Params = gin.Params{{Key: "id", Value: "sent-cancel"}, {Key: "taskId", Value: strconv.FormatUint(uint64(task.ID), 10)}}
	c.Set("user", "tester")

	s.handleCancelTask(c)

	if w.Code != http.StatusOK {
		t.Fatalf("cancel status=%d body=%s", w.Code, w.Body.String())
	}
	var reloaded db.Task
	if err := database.First(&reloaded, task.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Status != "cancelled" {
		t.Fatalf("status=%q, want cancelled", reloaded.Status)
	}
}
