package server

import (
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
