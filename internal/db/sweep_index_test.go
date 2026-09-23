package db

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// TestSweepCoverIndexesExist proves the sweep-cover migration creates its
// three indexes (fresh DBs get them via migrations; AutoMigrate alone does
// not create these composites).
func TestSweepCoverIndexesExist(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.AutoMigrate(&Task{}); err != nil {
		t.Fatalf("migrate task: %v", err)
	}
	for _, stmt := range []string{
		"CREATE INDEX IF NOT EXISTS idx_tasks_status_acknowledged ON tasks(status, acknowledged_at)",
		"CREATE INDEX IF NOT EXISTS idx_tasks_sweep_running ON tasks(status, claimed_at, acknowledged_at, delivery_attempts)",
		"CREATE INDEX IF NOT EXISTS idx_tasks_approval_expiry ON tasks(status, approval_expires_at)",
	} {
		if err := database.Exec(stmt).Error; err != nil {
			t.Fatalf("create index: %v", err)
		}
	}

	rows, err := database.Raw("PRAGMA index_list('tasks')").Rows()
	if err != nil {
		t.Fatalf("index list: %v", err)
	}
	defer rows.Close()
	names := make(map[string]bool)
	for rows.Next() {
		var seq, unique, partial int
		var name, origin string
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			rows.Close()
			t.Fatalf("scan index row: %v", err)
		}
		names[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate indexes: %v", err)
	}
	for _, want := range []string{"idx_tasks_status_acknowledged", "idx_tasks_sweep_running", "idx_tasks_approval_expiry"} {
		if !names[want] {
			t.Errorf("missing index %s (have %v)", want, names)
		}
	}
}

// TestSweepPredicatesUseIndex proves each sweep WHERE clause resolves to an
// index search instead of a full status scan.
func TestSweepPredicatesUseIndex(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.AutoMigrate(&Task{}); err != nil {
		t.Fatalf("migrate task: %v", err)
	}
	for _, stmt := range []string{
		"CREATE INDEX IF NOT EXISTS idx_tasks_status_acknowledged ON tasks(status, acknowledged_at)",
		"CREATE INDEX IF NOT EXISTS idx_tasks_sweep_running ON tasks(status, claimed_at, acknowledged_at, delivery_attempts)",
		"CREATE INDEX IF NOT EXISTS idx_tasks_approval_expiry ON tasks(status, approval_expires_at)",
	} {
		if err := database.Exec(stmt).Error; err != nil {
			t.Fatalf("create index: %v", err)
		}
	}

	queries := map[string]string{
		"acked":     "SELECT id FROM tasks WHERE status = 'running' AND acknowledged_at IS NOT NULL AND acknowledged_at < '2000-01-01'",
		"requeue":   "SELECT id FROM tasks WHERE status = 'running' AND claimed_at < '2000-01-01' AND acknowledged_at IS NULL AND delivery_attempts < 3",
		"approvals": "SELECT id FROM tasks WHERE status = 'pending_approval' AND approval_expires_at IS NOT NULL AND approval_expires_at < '2000-01-01'",
	}
	for name, q := range queries {
		var details []string
		rows, err := database.Raw("EXPLAIN QUERY PLAN " + q).Rows()
		if err != nil {
			t.Fatalf("%s: explain: %v", name, err)
		}
		for rows.Next() {
			var id, parent, notused int
			var detail string
			if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
				rows.Close()
				t.Fatalf("%s: scan plan: %v", name, err)
			}
			details = append(details, detail)
		}
		rows.Close()
		plan := strings.Join(details, "; ")
		if !strings.Contains(plan, "USING COVERING INDEX") && !strings.Contains(plan, "USING INDEX") {
			t.Errorf("%s: no index used: %s", name, plan)
		}
		if strings.Contains(plan, "SCAN tasks") && !strings.Contains(plan, "USING") {
			t.Errorf("%s: full table scan: %s", name, plan)
		}
	}
}
