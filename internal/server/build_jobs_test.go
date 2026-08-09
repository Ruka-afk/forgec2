package server

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// newBuildTestServer builds a minimal Server with the build queue primed so
// jobs can be submitted and executed synchronously by the worker goroutine.
func newBuildTestServer(t *testing.T) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := config.DefaultConfig()
	cfg.Server.DataDir = t.TempDir()
	s := &Server{
		db:               newContractDB(t),
		cfg:              cfg,
		ctx:              context.Background(),
		buildJobs:        make(map[string]*BuildJob),
		buildJobsMu:      sync.RWMutex{},
		wsClients:        make(map[*websocket.Conn]*wsClientConn),
		wsMutex:          sync.RWMutex{},
		operatorSessions: &operatorSessionTracker{sessions: make(map[uint]*WSOperatorSession)},
	}
	return s
}

// waitForBuildJob waits for the job to leave the "building" state.
func waitForBuildJob(t *testing.T, s *Server, jobID string, timeout time.Duration) *BuildJob {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		s.buildJobsMu.RLock()
		job, ok := s.buildJobs[jobID]
		s.buildJobsMu.RUnlock()
		if !ok {
			t.Fatalf("job %s not found", jobID)
		}
		if job.Status != "building" {
			return job
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("job %s still building after %v", jobID, timeout)
	return nil
}

// TestBuildQueueSurvivesPanickingBuild verifies that a panicking buildFn
// marks the job failed AND the worker goroutine keeps processing the next
// queued build instead of dying with the panic.
func TestBuildQueueSurvivesPanickingBuild(t *testing.T) {
	s := newBuildTestServer(t)

	panicJob := s.startBuildJob("windows", "exe", "http://127.0.0.1:8080", 1, "panic.exe")
	if !s.submitBuild(panicJob, func() (string, error) {
		panic("toolchain exploded")
	}, "windows", "exe", "http://127.0.0.1:8080", 1, "panic.exe") {
		t.Fatal("queue should accept the panicking build")
	}

	okJob := s.startBuildJob("linux", "elf", "http://127.0.0.1:8080", 1, "ok.elf")
	output := filepath.Join(s.extractAgentsDir(), "ok.elf")
	if !s.submitBuild(okJob, func() (string, error) { return output, nil }, "linux", "elf", "http://127.0.0.1:8080", 1, "ok.elf") {
		t.Fatal("queue should accept the follow-up build")
	}

	donePanic := waitForBuildJob(t, s, panicJob.ID, 5*time.Second)
	if donePanic.Status != "failed" {
		t.Errorf("panic job status = %q, want failed", donePanic.Status)
	}
	if donePanic.Error == "" {
		t.Error("panic job should carry an error message")
	}

	doneOK := waitForBuildJob(t, s, okJob.ID, 5*time.Second)
	if doneOK.Status != "completed" {
		t.Errorf("follow-up job status = %q, want completed (queue must survive panic), error=%q", doneOK.Status, doneOK.Error)
	}
	if doneOK.Output == "" {
		t.Errorf("follow-up job should record output path")
	}
}

// TestBuildQueuePanicFailurePersistsLogEntry verifies the recover path also
// writes a BuildLog row so failed builds remain auditable.
func TestBuildQueuePanicWritesBuildLog(t *testing.T) {
	s := newBuildTestServer(t)

	job := s.startBuildJob("windows", "exe", "http://127.0.0.1:8080", 1, "panic.exe")
	if !s.submitBuild(job, func() (string, error) {
		panic("boom")
	}, "windows", "exe", "http://127.0.0.1:8080", 1, "panic.exe") {
		t.Fatal("build queue should accept job")
	}
	waitForBuildJob(t, s, job.ID, 5*time.Second)

	var count int64
	if err := s.db.Model(&db.BuildLog{}).Where("status = ? AND filename = ?", "failed", "panic.exe").Count(&count).Error; err != nil {
		t.Fatalf("count build logs: %v", err)
	}
	if count == 0 {
		t.Error("expected a failed BuildLog entry after a panicking build")
	}
}