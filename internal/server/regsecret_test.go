package server

import (
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
)

func newRegSecretServer(t *testing.T, key []byte) *Server {
	t.Helper()
	database := testutil.SetupTestDB(t)
	return &Server{
		db:        database,
		regSecrets: crypto.NewRegSecretStore(key),
	}
}

func TestEnsureV3RegSecretCreatesAndClearsKey(t *testing.T) {
	s := newRegSecretServer(t, make([]byte, 32))
	const master = "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"

	id, secret, cleared, err := s.ensureV3RegSecret(master)
	if err != nil {
		t.Fatalf("ensureV3RegSecret: %v", err)
	}
	if id == "" || secret == "" {
		t.Fatalf("expected non-empty id/secret, got id=%q secret=%q", id, secret)
	}
	if cleared != "" {
		t.Fatalf("beacon master key must be cleared for v3 builds, got %q", cleared)
	}

	// The persisted secret must be recoverable by its public id.
	if got := s.regSecretByID(id); len(got) != 32 {
		t.Fatalf("regSecretByID returned %d bytes, want 32", len(got))
	}

	// No RegSecret row should remain for a never-registered secret
	// (cleanup is exercised elsewhere); at minimum it was created.
	var count int64
	if err := s.db.Model(&db.RegSecret{}).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 reg secret row, got %d", count)
	}
}

func TestEnsureV3RegSecretEmptyKeyPassthrough(t *testing.T) {
	s := newRegSecretServer(t, make([]byte, 32))
	id, secret, cleared, err := s.ensureV3RegSecret("")
	if err != nil {
		t.Fatalf("ensureV3RegSecret: %v", err)
	}
	if id != "" || secret != "" || cleared != "" {
		t.Fatalf("empty master key must be returned unchanged, got id=%q secret=%q cleared=%q", id, secret, cleared)
	}
}

func TestEnsureV3RegSecretNilStoreFailsSafe(t *testing.T) {
	s := &Server{db: testutil.SetupTestDB(t), regSecrets: nil}
	_, _, _, err := s.ensureV3RegSecret("aabb")
	if err == nil {
		t.Fatal("expected error when reg secret store is uninitialized")
	}
}

// TestCleanupOrphanedRegSecrets verifies the sweep deletes unbound secrets
// while preserving bound ones. A failed build leaves an unbound secret row that
// must eventually be reclaimed.
func TestCleanupOrphanedRegSecrets(t *testing.T) {
	s := newRegSecretServer(t, make([]byte, 32))

	orphan := db.RegSecret{ID: "orphan", SecretEnc: "x", Bound: false, CreatedAt: time.Now().Add(-time.Hour)}
	bound := db.RegSecret{ID: "bound", SecretEnc: "y", Bound: true, AgentID: "agent-1"}
	if err := s.db.Create(&orphan).Error; err != nil {
		t.Fatalf("create orphan: %v", err)
	}
	if err := s.db.Create(&bound).Error; err != nil {
		t.Fatalf("create bound: %v", err)
	}

	// ttl=0 => cutoff=now, so every existing unbound row is eligible for deletion.
	s.cleanupOrphanedRegSecrets(0)

	var orphanCount, boundCount int64
	s.db.Model(&db.RegSecret{}).Where("id = ?", "orphan").Count(&orphanCount)
	s.db.Model(&db.RegSecret{}).Where("id = ?", "bound").Count(&boundCount)
	if orphanCount != 0 {
		t.Fatalf("orphaned (unbound) secret should be deleted, count=%d", orphanCount)
	}
	if boundCount != 1 {
		t.Fatalf("bound secret must be preserved, count=%d", boundCount)
	}
}
