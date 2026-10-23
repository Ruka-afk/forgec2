package server

import (
	"bytes"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"gorm.io/gorm"
)

func newCleanupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gdb, err := db.InitDBWithDriver("", "", filepath.Join(t.TempDir(), "test.db"), slog.LevelWarn, 10, 5, time.Minute)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := gdb.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return gdb
}

func TestCleanupOldDataPurgesStaleSystemMetrics(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.CleanupRetentionDays = 30
	cfg.Server.DataDir = t.TempDir()
	s := &Server{db: newCleanupTestDB(t), cfg: cfg}

	now := time.Now()
	stale := db.SystemMetric{CPULoad: 1, CreatedAt: now.AddDate(0, 0, -120)}
	fresh := db.SystemMetric{CPULoad: 2, CreatedAt: now.Add(-time.Minute)}
	if err := s.db.Create(&stale).Error; err != nil {
		t.Fatalf("seed stale metric: %v", err)
	}
	if err := s.db.Create(&fresh).Error; err != nil {
		t.Fatalf("seed fresh metric: %v", err)
	}

	s.cleanupOldData()

	var remaining int64
	if err := s.db.Model(&db.SystemMetric{}).Count(&remaining).Error; err != nil {
		t.Fatalf("count metrics: %v", err)
	}
	if remaining != 1 {
		t.Fatalf("expected 1 metric to survive cleanup, got %d", remaining)
	}

	var kept db.SystemMetric
	if err := s.db.Order("id").First(&kept).Error; err != nil {
		t.Fatalf("read surviving metric: %v", err)
	}
	if int(kept.CPULoad) != 2 {
		t.Errorf("expected fresh metric to survive, got CPULoad=%v", kept.CPULoad)
	}
}

func TestCleanupOldDataPurgesTerminalTasks(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.CleanupRetentionDays = 30
	cfg.Server.DataDir = t.TempDir()
	s := &Server{db: newCleanupTestDB(t), cfg: cfg}

	if err := s.db.Create(&db.Implant{ID: "a", LastSeen: time.Now()}).Error; err != nil {
		t.Fatalf("seed agent: %v", err)
	}
	old := time.Now().AddDate(0, 0, -120)
	fresh := time.Now().Add(-time.Minute)
	seed := []db.Task{
		{AgentID: "a", Type: "shell", Status: "cancelled", CreatedAt: old},
		{AgentID: "a", Type: "shell", Status: "sent", CreatedAt: old},
		{AgentID: "a", Type: "shell", Status: "completed", CreatedAt: old},
		{AgentID: "a", Type: "shell", Status: "cancelled", CreatedAt: fresh},
		{AgentID: "a", Type: "shell", Status: "pending", CreatedAt: old},
	}
	for i := range seed {
		if err := s.db.Create(&seed[i]).Error; err != nil {
			t.Fatalf("seed task: %v", err)
		}
	}

	s.cleanupOldData()

	var remaining []db.Task
	if err := s.db.Find(&remaining).Error; err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	if len(remaining) != 2 {
		t.Fatalf("remaining=%d, want 2 (fresh cancelled + old pending)", len(remaining))
	}
	for _, task := range remaining {
		if task.Status != "cancelled" && task.Status != "pending" {
			t.Fatalf("unexpected survivor: %+v", task)
		}
	}
}

func TestSystemMetricsCreatedAtIndexApplied(t *testing.T) {
	gdb := newCleanupTestDB(t)

	rows, err := gdb.Raw("PRAGMA index_list('system_metrics')").Rows()
	if err != nil {
		t.Fatalf("query index list: %v", err)
	}
	defer rows.Close()

	found := false
	for rows.Next() {
		var seq int
		var name string
		var unique int
		var origin string
		var partial int
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			t.Fatalf("scan index row: %v", err)
		}
		if name == "idx_system_metrics_created_at" {
			found = true
			break
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate index rows: %v", err)
	}
	if !found {
		t.Fatal("expected idx_system_metrics_created_at index to exist")
	}
}

// TestOfflineThresholdWarnIsRateLimited pins the log-volume fix: the getter
// runs on every agent list, dashboard poll, metrics scrape and status
// recompute, so an unthrottled Warn drowned a misconfigured server in its own
// diagnostic (thousands of lines per minute). One warning per gap, and the
// default is still returned.
func TestOfflineThresholdWarnIsRateLimited(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	cfg := &config.Config{}
	cfg.Server.OfflineThreshold = 0 // invalid → warn path
	s := &Server{cfg: cfg}

	const calls = 5000
	for i := 0; i < calls; i++ {
		if got := s.offlineThreshold(); got != time.Duration(DefaultOfflineThresholdSec)*time.Second {
			t.Fatalf("offlineThreshold() = %v, want the default", got)
		}
	}

	warns := bytes.Count(buf.Bytes(), []byte("OfflineThreshold is invalid"))
	if warns != 1 {
		t.Fatalf("got %d warnings across %d calls, want exactly 1 (throttled)", warns, calls)
	}
}
