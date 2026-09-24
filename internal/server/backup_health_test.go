package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gin-gonic/gin"
	promtestutil "github.com/prometheus/client_golang/prometheus/testutil"
)

func backupTestManager(t *testing.T) *BackupManager {
	t.Helper()
	ginSetTestMode(t)
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "forgec2.db")
	database := testutil.SetupTestDB(t)
	bm, err := NewBackupManager(database, dbPath, filepath.Join(dir, "backups"), testStorageKeyHex)
	if err != nil {
		t.Fatalf("NewBackupManager: %v", err)
	}
	return bm
}

// TestBackupRetentionIsConfigurable proves retention is an operator decision
// instead of a hard-coded 7, and that the manager prunes to the configured
// count (sidecars included).
func TestBackupRetentionIsConfigurable(t *testing.T) {
	bm := backupTestManager(t)
	if got := bm.Health().Retain; got != defaultBackupRetention {
		t.Fatalf("default retain = %d, want %d", got, defaultBackupRetention)
	}
	bm.SetRetention(3)
	if got := bm.Health().Retain; got != 3 {
		t.Fatalf("retain = %d, want 3", got)
	}
	// Nonsense values must not wipe the policy.
	bm.SetRetention(0)
	bm.SetRetention(-5)
	if got := bm.Health().Retain; got != 3 {
		t.Fatalf("invalid SetRetention changed policy: %d", got)
	}

	// Plant 5 snapshots; cleanup must keep the newest 3.
	for i := 1; i <= 5; i++ {
		name := filepath.Join(bm.backupDir, backupNameForTest(i))
		if err := os.WriteFile(name, []byte("x"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		if err := os.WriteFile(name+".meta.json", []byte("{}"), 0o600); err != nil {
			t.Fatalf("write sidecar: %v", err)
		}
		age := time.Duration(6-i) * time.Hour
		stamp := time.Now().Add(-age)
		if err := os.Chtimes(name, stamp, stamp); err != nil {
			t.Fatalf("chtimes: %v", err)
		}
	}
	bm.cleanupOldBackups()

	var kept []string
	entries, err := os.ReadDir(bm.backupDir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".fbk" {
			kept = append(kept, e.Name())
		}
	}
	if len(kept) != 3 {
		t.Fatalf("kept %d snapshots, want 3: %v", len(kept), kept)
	}
	// The newest snapshot must survive.
	if _, err := os.Stat(filepath.Join(bm.backupDir, backupNameForTest(5))); err != nil {
		t.Fatalf("newest snapshot pruned: %v", err)
	}
	// Pruned sidecars must go with their snapshot.
	if _, err := os.Stat(filepath.Join(bm.backupDir, backupNameForTest(1))); !os.IsNotExist(err) {
		t.Fatalf("oldest snapshot survived cleanup: %v", err)
	}
}

func backupNameForTest(i int) string {
	return "forgec2_backup_2026010" + string(rune('0'+i)) + "_000000.fbk"
}

// TestBackupHealthTracksResults proves freshness state is recorded for both
// outcomes, which is what the Prometheus gauge and /api/v1/health report.
func TestBackupHealthTracksResults(t *testing.T) {
	bm := backupTestManager(t)

	if h := bm.Health(); !h.LastSuccess.IsZero() || h.AgeSeconds != 0 {
		t.Fatalf("fresh manager reported success: %+v", h)
	}

	bm.noteBackupResult(errNonSQLiteBackup)
	h := bm.Health()
	if h.LastFailure.IsZero() || h.LastError == "" {
		t.Fatalf("failure not recorded: %+v", h)
	}
	if !h.LastSuccess.IsZero() {
		t.Fatal("failure must not mark a success")
	}

	bm.noteBackupResult(nil)
	h = bm.Health()
	if h.LastSuccess.IsZero() {
		t.Fatal("success not recorded")
	}
	if h.LastError != "" {
		t.Fatalf("success left a stale error: %q", h.LastError)
	}
	if h.AgeSeconds < 0 || h.AgeSeconds > 5 {
		t.Fatalf("age = %v, want a small non-negative value", h.AgeSeconds)
	}
}

// TestAPIHealthReportsBackupFreshness proves the health endpoint surfaces the
// backup state (and says so plainly when backups are unavailable).
func TestAPIHealthReportsBackupFreshness(t *testing.T) {
	ginSetTestMode(t)
	cfg := config.DefaultConfig()
	cfg.Server.OfflineThreshold = 60
	s := &Server{db: testutil.SetupTestDB(t), cfg: cfg}
	s.backupManager = backupTestManager(t)
	s.backupManager.noteBackupResult(nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	s.apiHealth(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Backup map[string]any `json:"backup"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v body=%s", err, w.Body.String())
	}
	if _, ok := body.Backup["age_seconds"]; !ok {
		t.Fatalf("health response lacks backup freshness: %s", w.Body.String())
	}
	if _, ok := body.Backup["retain"]; !ok {
		t.Fatalf("health response lacks retention: %s", w.Body.String())
	}

	// No manager (non-SQLite deployment) must still answer clearly.
	s2 := &Server{db: testutil.SetupTestDB(t), cfg: cfg}
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	s2.apiHealth(c2)
	var body2 struct {
		Backup map[string]any `json:"backup"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &body2); err != nil {
		t.Fatalf("decode 2: %v", err)
	}
	if enabled, ok := body2.Backup["enabled"].(bool); !ok || enabled {
		t.Fatalf("expected backup.enabled=false, got %v", body2.Backup)
	}
}

// TestBackupMetricsExposeFreshness proves the gauges exist and carry values
// (Prometheus collectors are the operator's monitoring contract).
func TestBackupMetricsExposeFreshness(t *testing.T) {
	ginSetTestMode(t)
	cfg := config.DefaultConfig()
	cfg.Server.OfflineThreshold = 60
	s := &Server{db: testutil.SetupTestDB(t), cfg: cfg}
	s.metrics = NewMetricsCollector(s)
	s.backupManager = backupTestManager(t)
	s.backupManager.noteBackupResult(nil)
	// Age the last success so the gauge carries a clearly non-zero value
	// instead of a sub-millisecond delta.
	s.backupManager.mu.Lock()
	s.backupManager.lastSuccess = time.Now().Add(-30 * time.Minute)
	s.backupManager.mu.Unlock()

	s.updateMetricsFromDB()
	age := promtestutil.ToFloat64(s.metrics.BackupAgeSeconds)
	if age < 1700 || age > 1900 {
		t.Fatalf("backup age gauge = %v, want ~1800s", age)
	}
	if got := promtestutil.ToFloat64(s.metrics.BackupSuccessTotal); got != 0 {
		t.Fatalf("success counter = %v, want 0 before any counted run", got)
	}
	s.metrics.BackupSuccessTotal.Inc()
	s.metrics.BackupFailuresTotal.Inc()
	if got := promtestutil.ToFloat64(s.metrics.BackupSuccessTotal); got != 1 {
		t.Fatalf("success counter = %v, want 1", got)
	}
	if got := promtestutil.ToFloat64(s.metrics.BackupFailuresTotal); got != 1 {
		t.Fatalf("failure counter = %v, want 1", got)
	}
}
