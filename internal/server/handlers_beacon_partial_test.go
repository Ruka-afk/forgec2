package server

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
)

// partialTestServer builds a minimal Server able to run processTaskResults.
func partialTestServer(t *testing.T) *Server {
	t.Helper()
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	s := &Server{db: database, eventManager: NewEventManager(database)}
	s.agentPendingTasks = make(map[string]int)
	s.beaconDedupCache = make(map[string]time.Time)
	return s
}

// seedRunningTask creates a claimed/running task owned by the test agent.
func seedRunningTask(t *testing.T, s *Server, agentID string) db.Task {
	t.Helper()
	task := db.Task{AgentID: agentID, Type: "shell", Command: "long-op", Status: "running"}
	if err := s.db.Create(&task).Error; err != nil {
		t.Fatalf("seed running task: %v", err)
	}
	return task
}

func partialChunk(taskID uint, body string) taskResult {
	return taskResult{TaskID: taskID, Type: "shell", Output: body, Partial: true,
		ResultID: fmt.Sprintf("%d-p-%s", taskID, body[:7])}
}

// TestPartialResultsAppendWhileRunning verifies the streaming contract:
// chunks accumulate into a capped tail, status stays "running", and the
// pending counter is untouched (the task is not finalised by a chunk).
func TestPartialResultsAppendWhileRunning(t *testing.T) {
	s := partialTestServer(t)
	const uuid = "partial-agent-1"
	if err := s.db.Create(&db.Implant{ID: uuid, Status: "online"}).Error; err != nil {
		t.Fatalf("seed implant: %v", err)
	}
	task := seedRunningTask(t, s, uuid)
	before := s.agentPendingTasks[uuid]

	// Bodies deliberately contain characters invalid in base64 so the server's
	// transparent decoder cannot mangle them.
	chunkA := "delta-A##" + strings.Repeat("~", 100)
	chunkB := "delta-B##" + strings.Repeat("^", 100)
	s.processTaskResults(db.Implant{ID: uuid}, []taskResult{
		partialChunk(task.ID, chunkA),
		partialChunk(task.ID, chunkB),
	}, uuid, time.Now())

	var got db.Task
	if err := s.db.First(&got, task.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Status != "running" {
		t.Fatalf("status = %q after partials, want running", got.Status)
	}
	for _, want := range []string{chunkA, chunkB} {
		if !strings.Contains(got.Result, want) {
			t.Fatalf("stored tail missing a delivered chunk\nwant=%q\nstored=%q", want, got.Result)
		}
	}
	if after := s.agentPendingTasks[uuid]; after != before {
		t.Fatalf("pending counter changed on partial: %d -> %d", before, after)
	}
}

// TestFinalResultOverwritesPartialTail pins the terminal semantics: the
// completed result replaces the streamed tail wholesale.
func TestFinalResultOverwritesPartialTail(t *testing.T) {
	s := partialTestServer(t)
	const uuid = "partial-agent-2"
	if err := s.db.Create(&db.Implant{ID: uuid, Status: "online"}).Error; err != nil {
		t.Fatalf("seed implant: %v", err)
	}
	task := seedRunningTask(t, s, uuid)

	s.processTaskResults(db.Implant{ID: uuid}, []taskResult{
		partialChunk(task.ID, "STREAMTAIL"),
	}, uuid, time.Now())
	s.processTaskResults(db.Implant{ID: uuid}, []taskResult{
		{TaskID: task.ID, Type: "shell", Output: "FINAL"},
	}, uuid, time.Now())

	var got db.Task
	if err := s.db.First(&got, task.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Status != "completed" {
		t.Fatalf("status = %q, want completed", got.Status)
	}
	if strings.Contains(got.Result, "STREAMTAIL") || !strings.Contains(got.Result, "FINAL") {
		t.Fatalf("final result did not overwrite tail: %q", got.Result)
	}
}

// TestPartialTailCap bounds DB growth: a monster chunk collapses to the cap.
func TestPartialTailCap(t *testing.T) {
	s := partialTestServer(t)
	const uuid = "partial-agent-3"
	if err := s.db.Create(&db.Implant{ID: uuid, Status: "online"}).Error; err != nil {
		t.Fatalf("seed implant: %v", err)
	}
	task := seedRunningTask(t, s, uuid)

	big := strings.Repeat("X", taskResultTailCap*3)
	s.processTaskResults(db.Implant{ID: uuid}, []taskResult{
		partialChunk(task.ID, big),
	}, uuid, time.Now())

	var got db.Task
	if err := s.db.First(&got, task.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(got.Result) > taskResultTailCap {
		t.Fatalf("tail len %d exceeds cap %d", len(got.Result), taskResultTailCap)
	}
}
