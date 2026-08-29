package server

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// newBeaconFinalityTestServer builds the minimal Server that processTaskResults
// and the cancel handler need, mirroring the event test's construction.
func newBeaconFinalityTestServer(t *testing.T) (*Server, *gorm.DB) {
	t.Helper()
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
	t.Cleanup(func() { s.eventManager.Shutdown() })
	return s, database
}

// TestCancelledTaskResultNotApplied pins the P6 finality guarantee: a result
// that arrives for a task the operator already cancelled must not resurrect the
// row (previously the status was unconditionally overwritten to completed).
func TestCancelledTaskResultNotApplied(t *testing.T) {
	s, database := newBeaconFinalityTestServer(t)

	uuid := "dddd4444-5555-4333-8444-666666666666"
	now := time.Now()
	agent := db.Implant{ID: uuid, Hostname: "WIN-DC01", IP: "10.0.0.5"}
	if err := database.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}

	task := db.Task{
		AgentID: uuid,
		Type:    "shell",
		Command: "whoami",
		Status:  "cancelled",
		Error:   "cancelled by operator",
	}
	if err := database.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	s.agentPendingTasks[uuid] = 1

	// A result for the cancelled task arrives (agent had already executed it).
	s.processTaskResults(agent, []taskResult{
		{TaskID: task.ID, Type: "shell", Output: "administrator", Encoding: "base64", ResultID: "rid-late"},
	}, uuid, now)

	var reloaded db.Task
	if err := database.First(&reloaded, task.ID).Error; err != nil {
		t.Fatalf("reload task: %v", err)
	}
	if reloaded.Status != "cancelled" {
		t.Errorf("cancelled task resurrected: status=%q", reloaded.Status)
	}
	if reloaded.Result != "" {
		t.Errorf("cancelled task result overwritten: result=%q", reloaded.Result)
	}
	if reloaded.Error != "cancelled by operator" {
		t.Errorf("cancelled task error overwritten: error=%q", reloaded.Error)
	}
	if reloaded.LastResultID != "" {
		t.Errorf("cancelled task picked up last_result_id=%q", reloaded.LastResultID)
	}
	// The cancelled task's pending slot is NOT released here: terminal-state
	// transitions own the decrement (cancel endpoint / kill-switch disarm /
	// reject all release the slot at flip time, guarded by status conditions).
	// A result arriving for an already-cancelled task is a pure no-op —
	// decrementing again double-counted the release and could zero the
	// per-agent gate early.
	if n := s.agentPendingTasks[uuid]; n != 1 {
		t.Errorf("cancelled result must not touch the pending counter: %d", n)
	}
}

// TestTaskResultDurableDedupAcrossRestart pins the P8 durable idempotency: the
// same agent result re-sent after a server restart (in-memory dedup cache is
// gone) must be dropped based on the persisted LastResultID.
func TestTaskResultDurableDedupAcrossRestart(t *testing.T) {
	s, database := newBeaconFinalityTestServer(t)

	uuid := "eeee5555-5555-4333-8444-777777777777"
	now := time.Now()
	agent := db.Implant{ID: uuid, Hostname: "LINUX-01", IP: "10.0.0.6"}
	if err := database.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	task := db.Task{AgentID: uuid, Type: "shell", Command: "id", Status: "running"}
	if err := database.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	s.agentPendingTasks[uuid] = 1

	result := taskResult{TaskID: task.ID, Type: "shell", Output: "uid=0(root)", ResultID: "rid-4242"}
	s.processTaskResults(agent, []taskResult{result}, uuid, now)

	var first db.Task
	if err := database.First(&first, task.ID).Error; err != nil {
		t.Fatalf("reload task: %v", err)
	}
	if first.Status != "completed" {
		t.Fatalf("expected completed, got %q", first.Status)
	}
	if first.LastResultID != "rid-4242" {
		t.Fatalf("expected last_result_id rid-4242, got %q", first.LastResultID)
	}
	if n := s.agentPendingTasks[uuid]; n != 0 {
		t.Fatalf("pending counter should be 0, got %d", n)
	}

	// Simulate a server restart: the in-memory dedup window is cleared.
	s.resultDedupeCache = make(map[string]time.Time)

	// The agent re-sends the same result (dropped-frame retry). With a mutated
	// payload, a naive re-apply would overwrite the stored output.
	s.processTaskResults(agent, []taskResult{
		{TaskID: task.ID, Type: "shell", Output: "EVIL-OVERWRITE", ResultID: "rid-4242"},
	}, uuid, now)

	var second db.Task
	if err := database.First(&second, task.ID).Error; err != nil {
		t.Fatalf("reload task: %v", err)
	}
	if second.Result != "uid=0(root)" {
		t.Errorf("duplicate result overwrote stored result: %q", second.Result)
	}
	if second.LastResultID != "rid-4242" {
		t.Errorf("duplicate result changed last_result_id: %q", second.LastResultID)
	}
	if n := s.agentPendingTasks[uuid]; n != 0 {
		t.Errorf("duplicate result corrupted pending counter: %d", n)
	}
}

// TestRelayedCancelledTaskResultNotApplied pins finality for P2P child results:
// a cancelled child task must stay cancelled even when the result is relayed
// through its parent.
func TestRelayedCancelledTaskResultNotApplied(t *testing.T) {
	s, database := newBeaconFinalityTestServer(t)

	parent := "aaaa1111-5555-4333-8444-111111111111"
	child := "bbbb2222-5555-4333-8444-222222222222"
	now := time.Now()

	childAgent := db.Implant{ID: child, Hostname: "CHILD-01", IP: "10.0.0.7"}
	if err := database.Create(&childAgent).Error; err != nil {
		t.Fatalf("create child agent: %v", err)
	}
	if err := database.Model(&db.Implant{}).Where("id = ?", child).Update("parent_id", parent).Error; err != nil {
		t.Fatalf("set parent: %v", err)
	}

	task := db.Task{AgentID: child, Type: "shell", Command: "whoami", Status: "cancelled", Error: "cancelled by operator"}
	if err := database.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	s.agentPendingTasks[child] = 1

	s.processRelayedResults([]relayedData{
		{AgentID: child, Results: []taskResult{
			{TaskID: task.ID, Type: "shell", Output: "child\\admin", ResultID: "rid-relayed"},
		}},
	}, parent, now)

	var reloaded db.Task
	if err := database.First(&reloaded, task.ID).Error; err != nil {
		t.Fatalf("reload task: %v", err)
	}
	if reloaded.Status != "cancelled" {
		t.Errorf("relayed result resurrected cancelled task: status=%q", reloaded.Status)
	}
	if reloaded.Result != "" {
		t.Errorf("relayed result overwrote cancelled task result=%q", reloaded.Result)
	}
}

// TestCancelRunningTaskInjectsAbortTask pins the P6 abort wiring: cancelling a
// running task flips it to cancelled AND injects a high-priority "abort" task
// targeting the original task id.
func TestCancelRunningTaskInjectsAbortTask(t *testing.T) {
	s, database := newBeaconFinalityTestServer(t)

	uuid := "cccc3333-5555-4333-8444-333333333333"
	agent := db.Implant{ID: uuid, Hostname: "WS-01", IP: "10.0.0.8"}
	if err := database.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	task := db.Task{AgentID: uuid, Type: "shell", Command: "sleep 3600", Status: "running"}
	if err := database.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/", nil)
	c.Params = gin.Params{{Key: "id", Value: uuid}, {Key: "taskId", Value: strconv.FormatUint(uint64(task.ID), 10)}}
	c.Set("user", "tester")

	s.handleCancelTask(c)

	if w.Code != http.StatusOK {
		t.Fatalf("cancel status=%d body=%s", w.Code, w.Body.String())
	}

	var cancelled db.Task
	if err := database.First(&cancelled, task.ID).Error; err != nil {
		t.Fatalf("reload cancelled task: %v", err)
	}
	if cancelled.Status != "cancelled" {
		t.Fatalf("expected cancelled, got %q", cancelled.Status)
	}

	var abortTasks []db.Task
	if err := database.Where("type = ? AND agent_id = ?", "abort", uuid).Find(&abortTasks).Error; err != nil {
		t.Fatalf("query abort tasks: %v", err)
	}
	if len(abortTasks) != 1 {
		t.Fatalf("expected exactly one abort task, got %d", len(abortTasks))
	}
	abort := abortTasks[0]
	if abort.Command != strconv.FormatUint(uint64(task.ID), 10) {
		t.Errorf("abort command=%q, want %q", abort.Command, strconv.FormatUint(uint64(task.ID), 10))
	}
	if abort.Priority != 3 {
		t.Errorf("abort priority=%d, want 3", abort.Priority)
	}
	if abort.Status != "pending" {
		t.Errorf("abort status=%q, want pending", abort.Status)
	}
	if abort.ClaimedBy != uuid {
		t.Errorf("abort claimed_by=%q, want %q", abort.ClaimedBy, uuid)
	}
}

// TestCancelPendingTaskDoesNotInjectAbort verifies a pending (not yet fetched)
// task is simply cancelled without an abort task, since no execution runs.
func TestCancelPendingTaskDoesNotInjectAbort(t *testing.T) {
	s, database := newBeaconFinalityTestServer(t)

	uuid := "cccc3333-5555-4333-8444-333333333333"
	agent := db.Implant{ID: uuid, Hostname: "WS-01", IP: "10.0.0.8"}
	if err := database.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	task := db.Task{AgentID: uuid, Type: "shell", Command: "whoami", Status: "pending"}
	if err := database.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/", nil)
	c.Params = gin.Params{{Key: "id", Value: uuid}, {Key: "taskId", Value: strconv.FormatUint(uint64(task.ID), 10)}}
	c.Set("user", "tester")

	s.handleCancelTask(c)

	if w.Code != http.StatusOK {
		t.Fatalf("cancel status=%d body=%s", w.Code, w.Body.String())
	}

	var cancelled db.Task
	if err := database.First(&cancelled, task.ID).Error; err != nil {
		t.Fatalf("reload cancelled task: %v", err)
	}
	if cancelled.Status != "cancelled" {
		t.Fatalf("expected cancelled, got %q", cancelled.Status)
	}

	var abortCount int64
	if err := database.Model(&db.Task{}).Where("type = ? AND agent_id = ?", "abort", uuid).Count(&abortCount).Error; err != nil {
		t.Fatalf("count abort tasks: %v", err)
	}
	if abortCount != 0 {
		t.Errorf("pending cancel injected %d abort tasks, want 0", abortCount)
	}
}