package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/forgec2/forgec2/pkg/protocol"
	"github.com/gin-gonic/gin"
)

// BuildJob tracks an asynchronous binary build.
type BuildJob struct {
	ID          string
	Status      string // "building", "completed", "failed"
	Output      string // file path on success
	Error       string // error message on failure
	Platform    string // "windows", "linux", "macos"
	Format      string // "exe", "dll", "elf", "binary"
	C2URL       string
	ListenerID  uint
	Filename    string
	CreatedAt   time.Time
	CompletedAt time.Time
}

// startBuildJob creates a build job, stores it, and returns its ID.
func (s *Server) startBuildJob(platform, format, c2URL string, listenerID uint, filename string) *BuildJob {
	job := &BuildJob{
		ID:         protocol.UUIDv7(),
		Status:     "building",
		Platform:   platform,
		Format:     format,
		C2URL:      c2URL,
		ListenerID: listenerID,
		Filename:   filename,
		CreatedAt:  time.Now(),
	}
	s.buildJobsMu.Lock()
	s.buildJobs[job.ID] = job
	s.buildJobsMu.Unlock()
	return job
}

// completeBuildJob marks a build job as completed or failed.
func (s *Server) completeBuildJob(job *BuildJob, outputPath string, err error) {
	s.buildJobsMu.Lock()
	defer s.buildJobsMu.Unlock()
	job.CompletedAt = time.Now()
	if err != nil {
		job.Status = "failed"
		job.Error = sanitizeError(err, "Build failed")
	} else {
		job.Status = "completed"
		job.Output = outputPath
	}
}

// abandonBuildJob removes a job that was never submitted (e.g. queue full), so
// it does not linger as a phantom failed build.
func (s *Server) abandonBuildJob(job *BuildJob) {
	s.buildJobsMu.Lock()
	delete(s.buildJobs, job.ID)
	s.buildJobsMu.Unlock()
}

// cleanupBuildJobs removes completed jobs older than 1 hour.
func (s *Server) cleanupBuildJobs() {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.buildJobsMu.Lock()
			cutoff := time.Now().Add(-1 * time.Hour)
			for id, job := range s.buildJobs {
				if job.Status != "building" && job.CompletedAt.Before(cutoff) {
					if job.Output != "" {
						if err := os.Remove(job.Output); err != nil {
							slog.Error("Build jobs: failed to remove output file", "job_id", id, "err", err)
						}
					}
					delete(s.buildJobs, id)
				}
			}
			s.buildJobsMu.Unlock()

			// Reap one-shot generated files (stager/one-liner outputs) that
			// are not tracked as BuildJobs.
			s.transientArtifactsMu.Lock()
			for p, exp := range s.transientArtifacts {
				if time.Now().After(exp) {
					if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
						slog.Error("transient artifact cleanup failed", "path", p, "err", err)
					}
					delete(s.transientArtifacts, p)
				}
			}
			s.transientArtifactsMu.Unlock()
		case <-s.ctx.Done():
			return
		}
	}
}

// handleBuildStatus returns the current status of an async build.
func (s *Server) handleBuildStatus(c *gin.Context) {
	buildID := c.Param("id")
	// Snapshot all fields under the lock: completeBuildJob mutates the job
	// from the build goroutine, so lock-free reads would race with it.
	s.buildJobsMu.RLock()
	job, ok := s.buildJobs[buildID]
	if !ok {
		s.buildJobsMu.RUnlock()
		respondError(c, http.StatusNotFound, "build not found")
		return
	}
	resp := gin.H{
		"success":    true,
		"build_id":   job.ID,
		"status":     job.Status,
		"platform":   job.Platform,
		"format":     job.Format,
		"created_at": job.CreatedAt,
	}
	if job.Status == "completed" {
		resp["download_url"] = fmt.Sprintf("/generate/builds/%s/download", job.ID)
		resp["completed_at"] = job.CompletedAt
	} else if job.Status == "failed" {
		resp["error"] = job.Error
		resp["completed_at"] = job.CompletedAt
	}
	s.buildJobsMu.RUnlock()
	c.JSON(http.StatusOK, resp)
}

// handleBuildDownload serves the completed build file.
func (s *Server) handleBuildDownload(c *gin.Context) {
	buildID := c.Param("id")
	s.buildJobsMu.RLock()
	job, ok := s.buildJobs[buildID]
	var status, output string
	if ok {
		status = job.Status
		output = job.Output
	}
	s.buildJobsMu.RUnlock()
	if !ok {
		respondError(c, http.StatusNotFound, "build not found")
		return
	}
	if status != "completed" {
		respondError(c, http.StatusNotFound, "build not ready")
		return
	}
	cleanPath := filepath.Clean(output)
	if _, err := os.Stat(cleanPath); err != nil {
		respondError(c, http.StatusNotFound, "build output file not found")
		return
	}
	serveFileSafe(c, cleanPath, s.extractAgentsDir(), sanitizeFilename(filepath.Base(cleanPath)))
}

// handleBuildList returns all active build jobs for the caller.
func (s *Server) handleBuildList(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "builds": s.buildJobSnapshots()})
}

// buildJobSnapshots returns a lock-protected snapshot of every active build
// job in the same JSON shape used by handleBuildList. Copying every field
// under the lock guarantees the build goroutine's mutations cannot race reads.
func (s *Server) buildJobSnapshots() []gin.H {
	s.buildJobsMu.RLock()
	defer s.buildJobsMu.RUnlock()
	resp := make([]gin.H, 0, len(s.buildJobs))
	for _, j := range s.buildJobs {
		entry := gin.H{
			"id":          j.ID,
			"status":      j.Status,
			"platform":    j.Platform,
			"format":      j.Format,
			"filename":    j.Filename,
			"listener_id": j.ListenerID,
			"created_at":  j.CreatedAt,
		}
		if j.Error != "" {
			entry["error"] = j.Error
		}
		if !j.CompletedAt.IsZero() {
			entry["completed_at"] = j.CompletedAt
		}
		resp = append(resp, entry)
	}
	return resp
}

// runBuildAndUpdateJob is a helper to run a build function and update the job.
func (s *Server) runBuildAndUpdateJob(job *BuildJob, buildFn func() (string, error), platform, format, c2URL string, listenerID uint, filename string) {
	// A panic in the toolchain must never kill the worker goroutine (that
	// would silently stop the whole queue). Recover, mark the job failed,
	// and let the worker continue with the next queued build.
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("panic in build: %v", r)
			slog.Error("Async build panicked", "build_id", job.ID, "platform", platform, "format", format, "panic", r)
			s.completeBuildJob(job, "", err)
			s.logBuild(platform, format, c2URL, listenerID, filename, "failed", err.Error(), "")
			s.broadcastOperatorEvent(map[string]interface{}{
				"type":         "build_update",
				"build_id":     job.ID,
				"status":       "failed",
				"platform":     platform,
				"format":       format,
				"error":        err.Error(),
				"completed_at": job.CompletedAt,
			})
		}
	}()
	// The job is registered as "building" already; tell dashboards it has
	// actually started executing (queue wait is over).
	s.broadcastOperatorEvent(map[string]interface{}{
		"type":     "build_update",
		"build_id": job.ID,
		"status":   "building",
		"platform": platform,
		"format":   format,
	})
	outPath, err := buildFn()
	s.completeBuildJob(job, outPath, err)
	if err != nil {
		s.logBuild(platform, format, c2URL, listenerID, filename, "failed", err.Error(), "")
		slog.Error("Async build failed", "build_id", job.ID, "platform", platform, "format", format, "error", err)
	} else {
		s.logBuild(platform, format, c2URL, listenerID, filename, "success", "", outPath)
		slog.Info("Async build completed", "build_id", job.ID, "platform", platform, "format", format, "output", outPath)
	}
	event := map[string]interface{}{
		"type":   "build_update",
		"build_id": job.ID,
		"status": job.Status,
		"platform": platform,
		"format": format,
	}
	if job.Error != "" {
		event["error"] = job.Error
	}
	if !job.CompletedAt.IsZero() {
		event["completed_at"] = job.CompletedAt
	}
	s.broadcastOperatorEvent(event)
}

// Build queue: binary generation runs the Go/garble toolchain, which is
// expensive and must not run unbounded goroutines per request. Jobs are
// enqueued and executed by a small fixed worker pool in FIFO order.
const (
	maxQueuedBuilds     = 32
	buildWorkers        = 2
	maxConcurrentBuilds = 2
)

// queuedBuild is a build job waiting for a worker slot.
type queuedBuild struct {
	job        *BuildJob
	buildFn    func() (string, error)
	platform   string
	format     string
	c2URL      string
	listenerID uint
	filename   string
}

// submitBuild enqueues a build on the serialized worker queue. The job is
// registered with startBuildJob BEFORE submission so the frontend can poll its
// status immediately. Returns false when the queue is full.
func (s *Server) submitBuild(job *BuildJob, fn func() (string, error), platform, format, c2URL string, listenerID uint, filename string) bool {
	s.ensureBuildQueue()
	select {
	case s.buildQueue <- &queuedBuild{job, fn, platform, format, c2URL, listenerID, filename}:
		return true
	default:
		return false
	}
}

// ensureBuildQueue lazily starts the worker pool (safe for Server values
// constructed literally in tests, which never submit builds).
func (s *Server) ensureBuildQueue() {
	s.buildQueueOnce.Do(func() {
		s.buildQueue = make(chan *queuedBuild, maxQueuedBuilds)
		s.buildSem = make(chan struct{}, maxConcurrentBuilds)
		for i := 0; i < buildWorkers; i++ {
			go func() {
				for q := range s.buildQueue {
					s.buildSem <- struct{}{}
					s.runBuildAndUpdateJob(q.job, q.buildFn, q.platform, q.format, q.c2URL, q.listenerID, q.filename)
					<-s.buildSem
				}
			}()
		}
	})
}

// withBuildSlot runs fn while holding a slot from the shared build semaphore,
// bounding concurrent toolchain invocations across BOTH the async queue worker
// and synchronous stager/one-liner/shellcode handlers.
func (s *Server) withBuildSlot(fn func() (string, error)) (string, error) {
	s.ensureBuildQueue()
	s.buildSem <- struct{}{}
	defer func() { <-s.buildSem }()
	return fn()
}

// registerTransientArtifact marks a generated file for automatic cleanup after
// one hour. Used for stager/one-liner outputs that are served once and are not
// tracked as BuildJobs (which cleanupBuildJobs already reaps).
func (s *Server) registerTransientArtifact(path string) {
	if path == "" {
		return
	}
	s.transientArtifactsMu.Lock()
	if s.transientArtifacts == nil {
		s.transientArtifacts = make(map[string]time.Time)
	}
	s.transientArtifacts[path] = time.Now().Add(time.Hour)
	s.transientArtifactsMu.Unlock()
}

// extractAgentsDir returns the absolute path to the agents output directory.
func (s *Server) extractAgentsDir() string {
	agentsDir := filepath.Join(s.cfg.Server.DataDir, "agents")
	if !filepath.IsAbs(agentsDir) {
		if abs, err := filepath.Abs(agentsDir); err == nil {
			agentsDir = abs
		}
	}
	return agentsDir
}

// clampIntervalJitter normalizes interval and jitter values.
func clampIntervalJitter(interval, jitter, beaconTime int) (int, int) {
	if beaconTime > 0 {
		interval = beaconTime
	}
	if interval < 1 {
		interval = 5
	}
	if interval > 86400 {
		interval = 86400
	}
	if jitter < 0 {
		jitter = 0
	}
	if jitter > 100 {
		jitter = 100
	}
	return interval, jitter
}

// parseArchitecture returns the normalized architecture string.
func parseArchitecture(arch string) string {
	arch = strings.TrimSpace(arch)
	if arch == "" {
		return "amd64"
	}
	// Normalize common aliases
	switch arch {
	case "x86_64":
		return "amd64"
	case "aarch64":
		return "arm64"
	case "i386", "x86":
		return "386"
	}
	return arch
}
