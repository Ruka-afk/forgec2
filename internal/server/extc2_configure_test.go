package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gin-gonic/gin"
)

// newConfigureExtC2TestServer builds a server with a real DB and the runner
// map initialized (the handlers touch both).
func newConfigureExtC2TestServer(t *testing.T, cfg *config.Config) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	// The BotToken column encrypts on write (fail-closed vault hook), so
	// the encryption primitives must be initialized like server boot does.
	crypto.InitLootEncryption(testStorageKeyHex)
	crypto.InitExtC2Encryption("0123456789abcdef0123456789abcdef")
	database := testutil.SetupTestDB(t)
	// SetupTestDB only AutoMigrates; the (type, channel_id) unique index
	// comes from indexMigrations in production, so create it here for the
	// upsert ON CONFLICT target.
	if err := database.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_extc2_type_channel ON extc2_channels(type, channel_id)").Error; err != nil {
		t.Fatalf("create unique index: %v", err)
	}
	return &Server{
		db:             database,
		cfg:            cfg,
		extC2Runners:   make(map[string]extC2Runner),
		extC2Channels:  make(map[string]*extC2WSChannel),
		extC2TaskQueue: make(map[string][]extC2Task),
		extC2Notify:    make(map[string]chan struct{}),
	}
}

// fakeExtC2Runner records Stop calls for the runner-registry assertions.
type fakeExtC2Runner struct {
	stopped  int
	stopChan chan struct{}
}

func (f *fakeExtC2Runner) Stop() {
	f.stopped++
	close(f.stopChan)
}

// TestConfigureDiscordC2StartFailureLeavesNoRow proves a failed Start leaves
// no configured channel behind: the row used to be written first, so a
// rejected token still advertised a channel that never came up (and
// restore-on-boot retried it forever).
func TestConfigureDiscordC2StartFailureLeavesNoRow(t *testing.T) {
	// Empty crypto.extc2_key makes Discord Start fail closed without any
	// network access.
	s := newConfigureExtC2TestServer(t, &config.Config{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/extc2/discord/configure",
		bytes.NewReader([]byte(`{"bot_token":"t","channel_id":"c1"}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	s.handleConfigureDiscordC2(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s, want 500 on a failed Start", w.Code, w.Body.String())
	}
	var rows int64
	if err := s.db.Model(&db.ExtC2Channel{}).Count(&rows).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if rows != 0 {
		t.Fatalf("failed Start left %d channel rows behind", rows)
	}
	s.extC2ChannelsMu.Lock()
	_, live := s.extC2Runners["extc2-discord-c1"]
	s.extC2ChannelsMu.Unlock()
	if live {
		t.Fatal("failed Start registered a runner")
	}
}

// TestConfigureExtC2UpsertsSameChannel proves reconfiguring the same channel
// refreshes the existing row instead of accumulating one per attempt.
func TestConfigureExtC2UpsertsSameChannel(t *testing.T) {
	// Discord Start requires crypto.extc2_key; with it set Start succeeds
	// without any network call (the poll loop fails to connect and backs
	// off, which Stop() ends when the runner is replaced).
	cfg := &config.Config{}
	cfg.Crypto.ExtC2Key = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	s := newConfigureExtC2TestServer(t, cfg)
	t.Cleanup(func() { s.stopExtC2Runner("extc2-discord-c2") })

	for _, token := range []string{"token-a", "token-b"} {
		body := `{"bot_token":"` + token + `","channel_id":"c2"}`
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/extc2/discord/configure", bytes.NewReader([]byte(body)))
		c.Request.Header.Set("Content-Type", "application/json")
		s.handleConfigureDiscordC2(c)
		if w.Code != http.StatusOK {
			t.Fatalf("configure(%s): status=%d body=%s", token, w.Code, w.Body.String())
		}
	}

	var rows []db.ExtC2Channel
	if err := s.db.Find(&rows).Error; err != nil {
		t.Fatalf("find: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("reconfigure accumulated %d rows, want 1", len(rows))
	}
	if rows[0].BotToken != "token-b" {
		t.Fatalf("token=%q, want the refreshed token-b", rows[0].BotToken)
	}
}

// TestRegisterExtC2RunnerStopsPrevious proves re-registering the same runner
// key stops the poller it replaces (the old one kept reconnecting with the
// stale token until restart).
func TestRegisterExtC2RunnerStopsPrevious(t *testing.T) {
	s := newConfigureExtC2TestServer(t, &config.Config{})

	first := &fakeExtC2Runner{stopChan: make(chan struct{})}
	second := &fakeExtC2Runner{stopChan: make(chan struct{})}
	s.registerExtC2Runner("extc2-discord-x", first)
	s.registerExtC2Runner("extc2-discord-x", second)

	if first.stopped != 1 {
		t.Fatalf("previous runner Stop count=%d, want 1", first.stopped)
	}
	if second.stopped != 0 {
		t.Fatalf("new runner was stopped: count=%d, want 0", second.stopped)
	}
	s.extC2ChannelsMu.Lock()
	current := s.extC2Runners["extc2-discord-x"]
	s.extC2ChannelsMu.Unlock()
	if current != second {
		t.Fatal("runner map does not hold the newest runner")
	}
}

// TestExtC2ChannelUniqueIndexMigration proves the unique constraint exists
// and legacy duplicates are collapsed before it is created.
func TestExtC2ChannelUniqueIndexMigration(t *testing.T) {
	database := testutil.SetupTestDB(t)
	// SetupTestDB already created the table via AutoMigrate; seed legacy
	// duplicates for the same (type, channel_id) pair.
	stmts := []string{
		`DELETE FROM extc2_channels`,
		`INSERT INTO extc2_channels (id, type, bot_token, channel_id) VALUES (1, 'discord', 'old-token', 'c1')`,
		`INSERT INTO extc2_channels (id, type, bot_token, channel_id) VALUES (2, 'discord', 'new-token', 'c1')`,
	}
	for _, stmt := range stmts {
		if err := database.Exec(stmt).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	migIndex := -1
	for i, m := range db.Migrations {
		if m.ID == "2026-10-10-extc2-channel-unique" {
			migIndex = i
			break
		}
	}
	if migIndex < 0 {
		t.Fatal("extc2-channel-unique migration not found")
	}
	if err := db.Migrations[migIndex].Migrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	var rows []db.ExtC2Channel
	if err := database.Find(&rows).Error; err != nil {
		t.Fatalf("find: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != 1 {
		t.Fatalf("dedupe kept %+v, want only the lowest id", rows)
	}
	if !database.Migrator().HasIndex(&db.ExtC2Channel{}, "idx_extc2_type_channel") {
		t.Fatal("missing unique index idx_extc2_type_channel")
	}
}
