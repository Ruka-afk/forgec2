package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"gorm.io/gorm"
)

func newJWTTestServer(t *testing.T, database *gorm.DB, cfg *config.Config) *Server {
	t.Helper()
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	return &Server{
		db:                database,
		cfg:               cfg,
		ctx:               context.Background(),
		agentPendingTasks: make(map[string]int),
		metrics:           &MetricsCollector{TasksTotal: prometheus.NewCounter(prometheus.CounterOpts{})},
	}
}

func jwtPostContext() (*httptest.ResponseRecorder, *gin.Context) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(http.MethodPost, "/api/settings/jwt/regenerate", strings.NewReader(""))
	c.Request = req
	c.Set("user_role", "operator")
	return w, c
}

func TestHandleRegenerateJWT_RejectsWithoutExplicitLootKey(t *testing.T) {
	database := testutil.SetupTestDB(t)
	cfg := config.DefaultConfig()
	cfg.Crypto.LootKey = ""
	cfg.Server.JWTSecret = "old-jwt-secret-that-is-32-bytes-ok!!"
	s := newJWTTestServer(t, database, cfg)

	// Persist a credential encrypted under the old (derived) loot key.
	crypto.InitLootEncryption(cfg.Server.JWTSecret, cfg.Crypto.LootKey)
	enc, err := crypto.EncryptLoot("credential-value")
	if err != nil {
		t.Fatalf("EncryptLoot: %v", err)
	}
	if err := database.Create(&db.CredentialEntry{
		AgentID:  "agent-1",
		Username: "admin",
		Password: enc,
		Source:   "manual",
	}).Error; err != nil {
		t.Fatalf("seed credential: %v", err)
	}

	w, c := jwtPostContext()
	s.handleRegenerateJWT(c)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict without explicit loot_key, got %d body=%s", w.Code, w.Body.String())
	}

	// JWT secret must be unchanged, and the credential must still decrypt.
	if s.cfg.Server.JWTSecret != "old-jwt-secret-that-is-32-bytes-ok!!" {
		t.Fatalf("JWT secret was mutated despite rejection: %q", s.cfg.Server.JWTSecret)
	}
	var cred db.CredentialEntry
	if err := database.First(&cred).Error; err != nil {
		t.Fatalf("load credential: %v", err)
	}
	plain, err := crypto.DecryptLoot(cred.Password)
	if err != nil || plain != "credential-value" {
		t.Fatalf("credential unreadable after rejected rotation: plain=%q err=%v", plain, err)
	}
}

func TestHandleRegenerateJWT_RotatesAndReencryptsAllTOTP(t *testing.T) {
	database := testutil.SetupTestDB(t)
	cfg := config.DefaultConfig()
	cfg.Crypto.LootKey = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
	cfg.Server.JWTSecret = "old-jwt-secret-that-is-32-bytes-ok!!"
	s := newJWTTestServer(t, database, cfg)
	s.configPath = t.TempDir() + "/config.yaml"

	// Explicit loot key stays stable across JWT rotation.
	crypto.InitLootEncryption(cfg.Server.JWTSecret, cfg.Crypto.LootKey)

	// Seed 510 users with TOTP secrets (exceeds the old hard Limit(500)).
	const total = 510
	for i := 0; i < total; i++ {
		u := db.User{
			Username:     "user-" + strconv.Itoa(i),
			PasswordHash: "hash",
			Role:         "operator",
		}
		// User TOTPSecret stored encrypted with old JWT secret.
		enc, err := encryptSecret("secret-value-"+strconv.Itoa(i), cfg.Server.JWTSecret)
		if err != nil {
			t.Fatalf("encryptSecret: %v", err)
		}
		u.TOTPSecret = enc
		if err := database.Create(&u).Error; err != nil {
			t.Fatalf("seed user %d: %v", i, err)
		}
	}

	oldSecret := cfg.Server.JWTSecret
	w, c := jwtPostContext()
	s.handleRegenerateJWT(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	newSecret := s.cfg.Server.JWTSecret
	if newSecret == oldSecret {
		t.Fatal("JWT secret did not rotate")
	}

	// All 510 TOTP secrets must be decryptable with the NEW secret.
	var count int64
	if err := database.Model(&db.User{}).Count(&count).Error; err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != total {
		t.Fatalf("expected %d users, got %d", total, count)
	}
	var users []db.User
	if err := database.Find(&users).Error; err != nil {
		t.Fatalf("find users: %v", err)
	}
	for _, u := range users {
		plain, err := decryptSecret(u.TOTPSecret, newSecret)
		if err != nil {
			t.Fatalf("user %s TOTP not decryptable with new secret: %v", u.Username, err)
		}
		if !strings.HasPrefix(plain, "secret-value-") {
			t.Fatalf("user %s: unexpected plaintext %q", u.Username, plain)
		}
	}

	// Old-secret decryption must now fail (proves re-encryption happened).
	first := users[0]
	if _, err := decryptSecret(first.TOTPSecret, oldSecret); err == nil {
		t.Fatal("old-secret decryption should fail after re-encryption")
	}
}

func TestHandleRegenerateJWT_PersistsAndReinitializes(t *testing.T) {
	database := testutil.SetupTestDB(t)
	cfg := config.DefaultConfig()
	cfg.Crypto.LootKey = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
	cfg.Server.JWTSecret = "old-jwt-secret-that-is-32-bytes-ok!!"
	s := newJWTTestServer(t, database, cfg)
	s.configPath = t.TempDir() + "/config.yaml"

	crypto.InitLootEncryption(cfg.Server.JWTSecret, cfg.Crypto.LootKey)

	// Encrypt something before rotation; loot key must remain usable after.
	before, err := crypto.EncryptLoot("stable-loot")
	if err != nil {
		t.Fatalf("EncryptLoot: %v", err)
	}

	w, c := jwtPostContext()
	s.handleRegenerateJWT(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	// Config must have been saved with the new secret on disk.
	loaded, err := config.Load(s.configPath)
	if err != nil {
		t.Fatalf("load saved config: %v", err)
	}
	if loaded.Server.JWTSecret != s.cfg.Server.JWTSecret {
		t.Fatalf("config on disk does not match runtime secret")
	}

	// Loot ciphertext encrypted before rotation must decrypt (explicit key stable).
	plain, err := crypto.DecryptLoot(before)
	if err != nil || plain != "stable-loot" {
		t.Fatalf("loot lost after rotation: plain=%q err=%v", plain, err)
	}
}
