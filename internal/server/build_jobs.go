package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
		ID:         uuid.New().String(),
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
						os.Remove(job.Output)
					}
					delete(s.buildJobs, id)
				}
			}
			s.buildJobsMu.Unlock()
		case <-s.ctx.Done():
			return
		}
	}
}

// handleBuildStatus returns the current status of an async build.
func (s *Server) handleBuildStatus(c *gin.Context) {
	buildID := c.Param("id")
	s.buildJobsMu.RLock()
	job, ok := s.buildJobs[buildID]
	s.buildJobsMu.RUnlock()
	if !ok {
		respondError(c, http.StatusNotFound, "Build not found")
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
	c.JSON(http.StatusOK, resp)
}

// handleBuildDownload serves the completed build file.
func (s *Server) handleBuildDownload(c *gin.Context) {
	buildID := c.Param("id")
	s.buildJobsMu.RLock()
	job, ok := s.buildJobs[buildID]
	s.buildJobsMu.RUnlock()
	if !ok {
		respondError(c, http.StatusNotFound, "Build not found")
		return
	}
	if job.Status != "completed" {
		respondError(c, http.StatusNotFound, "Build not ready")
		return
	}
	cleanPath := filepath.Clean(job.Output)
	if _, err := os.Stat(cleanPath); err != nil {
		respondError(c, http.StatusNotFound, "Build output file not found")
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, sanitizeFilename(filepath.Base(cleanPath))))
	c.File(cleanPath)
}

// handleBuildList returns all active build jobs for the caller.
func (s *Server) handleBuildList(c *gin.Context) {
	s.buildJobsMu.RLock()
	jobs := make([]*BuildJob, 0, len(s.buildJobs))
	for _, j := range s.buildJobs {
		jobs = append(jobs, j)
	}
	s.buildJobsMu.RUnlock()

	type jobResp struct {
		ID          string    `json:"id"`
		Status      string    `json:"status"`
		Platform    string    `json:"platform"`
		Format      string    `json:"format"`
		Filename    string    `json:"filename"`
		Error       string    `json:"error,omitempty"`
		CreatedAt   time.Time `json:"created_at"`
		CompletedAt time.Time `json:"completed_at,omitempty"`
	}
	resp := make([]jobResp, 0, len(jobs))
	for _, j := range jobs {
		resp = append(resp, jobResp{
			ID:          j.ID,
			Status:      j.Status,
			Platform:    j.Platform,
			Format:      j.Format,
			Filename:    j.Filename,
			Error:       j.Error,
			CreatedAt:   j.CreatedAt,
			CompletedAt: j.CompletedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "builds": resp})
}

// isBuildRunning returns true if there are active builds.
func (s *Server) isBuildRunning() bool {
	s.buildJobsMu.RLock()
	defer s.buildJobsMu.RUnlock()
	for _, j := range s.buildJobs {
		if j.Status == "building" {
			return true
		}
	}
	return false
}

// runBuildAndUpdateJob is a helper to run a build function and update the job.
func (s *Server) runBuildAndUpdateJob(job *BuildJob, buildFn func() (string, error), platform, format, c2URL string, listenerID uint, filename string) {
	outPath, err := buildFn()
	s.completeBuildJob(job, outPath, err)
	if err != nil {
		s.logBuild(platform, format, c2URL, listenerID, filename, "failed", err.Error(), "")
		slog.Error("Async build failed", "build_id", job.ID, "platform", platform, "format", format, "error", err)
	} else {
		s.logBuild(platform, format, c2URL, listenerID, filename, "success", "", outPath)
		slog.Info("Async build completed", "build_id", job.ID, "platform", platform, "format", format, "output", outPath)
	}
	// Broadcast build status via WebSocket
	if s.wsHub != nil {
		msg, _ := json.Marshal(gin.H{
			"type":     "build_complete",
			"build_id": job.ID,
			"status":   job.Status,
			"platform": platform,
			"format":   format,
		})
		s.wsHub.Broadcast(msg)
	}
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

// validateAndResolveListener validates the request and resolves the listener.
// Returns the resolved C2URL, Protocol, DNSDomain, DNSServer, and whether it's P2P/DNS.
func (s *Server) validateAndResolveListener(listenerID uint, p2pMode, dnsDomain, dnsServer, protocol string) (c2URL, resolvedProtocol, resolvedDNSDomain, resolvedDNSServer string, isP2P, isDNS bool, err error) {
	isP2P = p2pMode == "parent" || p2pMode == "child"
	isDNS = dnsDomain != "" || dnsServer != ""

	if !isP2P && !isDNS && listenerID == 0 {
		return "", "", "", "", false, false, fmt.Errorf("listener or DNS domain is required")
	}

	if !isP2P && !isDNS {
		resolved, err := s.resolveListener(listenerID)
		if err != nil {
			return "", "", "", "", false, false, fmt.Errorf("invalid listener configuration")
		}
		c2URL = resolved.C2URL
		resolvedProtocol = resolved.Protocol
		if resolved.DNSDomain != "" {
			resolvedDNSDomain = resolved.DNSDomain
			if dnsServer == "" {
				resolvedDNSServer = resolved.DNSServer
			}
		}
	} else if isDNS && protocol == "" {
		resolvedProtocol = "dns"
	}

	return c2URL, resolvedProtocol, resolvedDNSDomain, resolvedDNSServer, isP2P, isDNS, nil
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
	if strings.TrimSpace(arch) == "" {
		return "amd64"
	}
	return strings.TrimSpace(arch)
}
