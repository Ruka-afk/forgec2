package server

import (
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
)

func newRegSecretServer(t *testing.T, key []byte) *Server {
	t.Helper()
	database := testutil.SetupTestDB(t)
	return &Server{
		db:         database,
		regSecrets: crypto.NewRegSecretStore(key),
	}
}

func TestEnsureV3RegSecretCreatesAndClearsKey(t *testing.T) {
	s := newRegSecretServer(t, make([]byte, 32))
	const master = "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"

	id, secret, cleared, err := s.ensureV3RegSecret(master, 0)
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

// TestEnsureV3RegSecretEmptyKeyFails pins the P2-4 fix: an empty
// server.beacon_key must REFUSE generation — processAuthFrame rejects any
// frame without a SecretID, so a passthrough produced implants that could
// never register while the build endpoints returned success.
func TestEnsureV3RegSecretEmptyKeyPassthrough(t *testing.T) {
	s := newRegSecretServer(t, make([]byte, 32))
	id, secret, cleared, err := s.ensureV3RegSecret("", 0)
	if err == nil {
		t.Fatal("empty beacon_key must fail generation (dead payload otherwise)")
	}
	if id != "" || secret != "" || cleared != "" {
		t.Fatalf("no secret material should be returned on failure, got id=%q secret=%q cleared=%q", id, secret, cleared)
	}
}

func TestEnsureV3RegSecretNilStoreFailsSafe(t *testing.T) {
	s := &Server{db: testutil.SetupTestDB(t), regSecrets: nil}
	_, _, _, err := s.ensureV3RegSecret("aabb", 0)
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

// TestRegSecretForAuthEnforcesOrphanTTL pins the difference between the hourly
// sweep and the actual security boundary: an unbound build secret older than
// server.reg_secret_orphan_ttl_hours must be refused at authentication, even
// before the sweep has run. A secret already bound to a deployed agent is that
// agent's long-term credential and must keep working.
func TestRegSecretForAuthEnforcesOrphanTTL(t *testing.T) {
	s := newRegSecretServer(t, make([]byte, 32))
	const master = "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
	id, _, _, err := s.ensureV3RegSecret(master, 0)
	if err != nil {
		t.Fatalf("ensureV3RegSecret: %v", err)
	}

	// Fresh unbound secret: accepted (normal first registration).
	if _, ok := s.regSecretForAuth(id, "agent-ttl"); !ok {
		t.Fatal("fresh unbound secret must authenticate")
	}

	age := func(d time.Duration) {
		t.Helper()
		if err := s.db.Model(&db.RegSecret{}).Where("id = ?", id).
			UpdateColumn("created_at", time.Now().Add(-d)).Error; err != nil {
			t.Fatalf("age secret: %v", err)
		}
	}

	// Older than the built-in default: rejected at auth.
	age(defaultRegSecretOrphanTTL + time.Minute)
	if _, ok := s.regSecretForAuth(id, "agent-ttl"); ok {
		t.Fatal("expired unbound secret must be rejected at authentication")
	}

	// The configured TTL is honoured (live-read): 2h old passes 3h, fails 1h.
	s.cfg = config.DefaultConfig()
	s.cfg.Server.RegSecretOrphanTTLHours = 3
	age(2 * time.Hour)
	if _, ok := s.regSecretForAuth(id, "agent-ttl"); !ok {
		t.Fatal("unbound secret inside the configured TTL must authenticate")
	}
	s.cfg.Server.RegSecretOrphanTTLHours = 1
	if _, ok := s.regSecretForAuth(id, "agent-ttl"); ok {
		t.Fatal("unbound secret outside the configured TTL must be rejected")
	}

	// Bound secrets never expire: the agent row may outlive any orphan TTL.
	s.bindRegSecret(id, "agent-ttl")
	age(30 * 24 * time.Hour)
	if _, ok := s.regSecretForAuth(id, "agent-ttl"); !ok {
		t.Fatal("a bound secret must never expire")
	}
}
