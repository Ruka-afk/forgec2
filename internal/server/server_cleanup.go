package server

import (
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"gorm.io/gorm"
)

// ── Retention cleanup + agent status thresholds ───────────────────────────

// cleanupOldData removes old completed/failed tasks and old files (screenshots + uploads) to prevent bloat
func (s *Server) cleanupOldData() {
	retention := s.cfg.Server.CleanupRetentionDays
	if retention < 1 {
		retention = DefaultCleanupRetentionDays
	}
	cutoff := time.Now().AddDate(0, 0, -retention)

	// delete old tasks
	if err := s.db.WithContext(s.ctx).Where("created_at < ? AND status IN ?", cutoff, []string{"completed", "failed"}).Delete(&db.Task{}).Error; err != nil {
		slog.Error("Cleanup tasks failed", "err", err)
	}

	// delete old system metrics (monitoring persists one row every 30s; without
	// retention the table grows ~2.9k rows/day and bloats the SQLite database)
	if err := s.db.WithContext(s.ctx).Where("created_at < ?", cutoff).Delete(&db.SystemMetric{}).Error; err != nil {
		slog.Error("Cleanup system metrics failed", "err", err)
	}

	// Periodic SQLite maintenance: VACUUM and ANALYZE to prevent bloat and
	// keep query planner statistics fresh. Only runs if the DB is SQLite.
	if sqlDB, err := s.db.DB(); err == nil {
		if err := sqlDB.Ping(); err == nil {
			dbName := s.cfg.Database.Driver
			if dbName == "" || dbName == "sqlite" {
				s.wg.Add(1)
				go func() {
					defer s.wg.Done()
					if _, err := sqlDB.Exec("PRAGMA optimize"); err != nil {
						slog.Debug("SQLite ANALYZE failed", "err", err)
					}
				}()
			}
		}
	}

	s.cleanupGhostAgents()

	dataDir := s.cfg.Server.DataDir
	if dataDir == "" {
		dataDir = "data"
	}

	// Clean old screenshots
	s.cleanOldFiles(filepath.Join(dataDir, "screenshots"), cutoff)

	// Clean old uploads (exfil files)
	s.cleanOldFiles(filepath.Join(dataDir, "uploads"), cutoff)

	// Clean old agent binaries
	s.cleanOldFiles(filepath.Join(dataDir, "agents"), cutoff)

	s.cleanupStaleMapEntries()

	slog.Info("Old data cleanup completed")
}

func (s *Server) cleanupStaleMapEntries() {
	cutoff := time.Now().Add(-StaleMapCleanupAge)
	cleaned := 0

	// Clean stale agentStatusCooldown entries
	s.agentStatusCooldownMu.Lock()
	for k, v := range s.agentStatusCooldown {
		if v.Before(cutoff) {
			delete(s.agentStatusCooldown, k)
			cleaned++
		}
	}
	s.agentStatusCooldownMu.Unlock()

	// Clean stale screenMonitorImplants entries
	s.screenMonitorMu.Lock()
	for k, v := range s.screenMonitorImplants {
		if v.Before(cutoff) {
			delete(s.screenMonitorImplants, k)
			cleaned++
		}
	}
	s.screenMonitorMu.Unlock()

	// Live frames are kept in memory only as an HTTP fallback for consoles
	// whose WebSocket briefly reconnects. Bound their lifetime with the same
	// stale-map cleanup used by monitor registrations.
	s.screenFrameMu.Lock()
	for k, frame := range s.screenFrames {
		if frame.capturedAt.Before(cutoff) {
			delete(s.screenFrames, k)
			cleaned++
		}
	}
	s.screenFrameMu.Unlock()

	// Clean empty extC2TaskQueue entries
	s.extC2TaskMu.Lock()
	for k, v := range s.extC2TaskQueue {
		if len(v) == 0 {
			delete(s.extC2TaskQueue, k)
			cleaned++
		}
	}
	s.extC2TaskMu.Unlock()

	if cleaned > 0 {
		slog.Info("Cleaned stale map entries", "count", cleaned)
	}
}

func (s *Server) offlineThreshold() time.Duration {
	if s.cfg == nil {
		return time.Duration(DefaultOfflineThresholdSec) * time.Second
	}
	d := s.cfg.Server.OfflineThreshold
	if d < 1 {
		slog.Warn("OfflineThreshold is invalid, using default", "configured", d, "default", DefaultOfflineThresholdSec)
		d = DefaultOfflineThresholdSec
	}
	return time.Duration(d) * time.Second
}

func (s *Server) staleThreshold() time.Duration {
	return s.offlineThreshold() * StaleThresholdMultiplier
}

// AgentStatusInfo holds display info for an agent's status
type AgentStatusInfo struct {
	Status    string // "online", "stale", "offline"
	Label     string // "Online", "Timeout", "Offline"
	DotColor  string // tailwind bg class
	BgColor   string // tailwind bg class
	TextColor string // tailwind text class
	Anim      string // animate-pulse or empty
}

func (s *Server) agentStatus(a db.Implant) AgentStatusInfo {
	since := time.Since(a.LastSeen)
	threshold := s.offlineThreshold()
	switch {
	case since < threshold:
		return AgentStatusInfo{"online", "Online", "bg-emerald-500", "bg-emerald-50", "text-emerald-700", "animate-pulse"}
	case since < s.staleThreshold():
		return AgentStatusInfo{"stale", "Timeout", "bg-amber-500", "bg-amber-50", "text-amber-700", ""}
	default:
		return AgentStatusInfo{"offline", "Offline", "bg-red-500", "bg-red-50", "text-red-700", ""}
	}
}

// cleanupGhostAgents removes invalid or long-dead implant records.
func (s *Server) cleanupGhostAgents() {
	ghostCutoff := time.Now().Add(-GhostAgentCutoff)
	var ghosts []db.Implant
	if err := s.db.Where("(hostname = '' OR hostname IS NULL) AND (ip = '' OR ip IS NULL) AND last_seen < ?", ghostCutoff).Limit(500).Find(&ghosts).Error; err != nil {
		return
	}
	if len(ghosts) > 0 {
		ghostIDs := make([]string, len(ghosts))
		for i, a := range ghosts {
			ghostIDs[i] = a.ID
		}
		if err := s.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("agent_id IN ?", ghostIDs).Delete(&db.Task{}).Error; err != nil {
				return err
			}
			// Hard delete (Unscoped) so auto-cleanup never leaves an invisible
			// soft-deleted tombstone that other views (dashboard stats, etc.)
			// would still count. Consistent with manual beacon hard-delete.
			return tx.Unscoped().Where("id IN ?", ghostIDs).Delete(&db.Implant{}).Error
		}); err != nil {
			slog.Error("Failed to remove ghost agents", "err", err)
		} else {
			slog.Info("Removed ghost agents", "count", len(ghosts))
		}
	}

	retention := s.cfg.Server.CleanupRetentionDays
	if retention < 1 {
		retention = DefaultCleanupRetentionDays
	}
	offlineCutoff := time.Now().AddDate(0, 0, -retention)
	var stale []db.Implant
	if err := s.db.Where("last_seen < ?", offlineCutoff).Limit(500).Find(&stale).Error; err != nil {
		return
	}
	if len(stale) > 0 {
		idStrs := make([]string, len(stale))
		for i, a := range stale {
			idStrs[i] = a.ID
		}
		slog.Info("Removing stale offline agents", "count", len(stale), "age_days", retention)
		if err := s.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("agent_id IN ?", idStrs).Delete(&db.Task{}).Error; err != nil {
				return err
			}
			// Hard delete, consistent with the ghost pass above and manual deletes.
			return tx.Unscoped().Where("id IN ?", idStrs).Delete(&db.Implant{}).Error
		}); err != nil {
			slog.Error("Failed to remove stale agents", "err", err)
		}
	}
}

// cleanOldFiles recursively removes files older than cutoff in the given dir
func (s *Server) cleanOldFiles(dir string, cutoff time.Time) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("Failed to read cleanup directory", "dir", dir, "err", err)
		}
		return
	}
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if e.IsDir() {
			s.cleanOldFiles(path, cutoff)
			remaining, _ := os.ReadDir(path)
			if len(remaining) == 0 {
				if err := os.Remove(path); err != nil {
					slog.Debug("Failed to remove empty directory", "path", path, "err", err)
				}
			}
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(path); err != nil {
				slog.Debug("Failed to remove old file", "path", path, "err", err)
			}
		}
	}
}
