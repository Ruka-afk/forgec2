package server

import (
	"context"
	"net/http"
	"net/http/httptest"
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

func TestHandleRegenerateJWT_IndependentKeysSurviveRotation(t *testing.T) {
	database := testutil.SetupTestDB(t)
	cfg := config.DefaultConfig()
	setServerTestKeys(cfg)
	cfg.Server.JWTSecret = "old-jwt-secret-that-is-32-bytes-ok!!"
	s := newJWTTestServer(t, database, cfg)
	s.configPath = t.TempDir() + "/config.yaml"

	crypto.InitLootEncryption(cfg.Crypto.LootKey)

	// Persist a credential encrypted under the (independent) loot key.
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

	oldSecret := cfg.Server.JWTSecret
	w, c := jwtPostContext()
	s.handleRegenerateJWT(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (rotation must not require loot coupling), got %d body=%s", w.Code, w.Body.String())
	}
	if s.cfg.Server.JWTSecret == oldSecret {
		t.Fatal("JWT secret did not rotate")
	}

	// The credential encrypted with the stable loot key must still decrypt
	// even though the JWT secret changed.
	var cred db.CredentialEntry
	if err := database.First(&cred).Error; err != nil {
		t.Fatalf("load credential: %v", err)
	}
	plain, err := crypto.DecryptLoot(cred.Password)
	if err != nil || plain != "credential-value" {
		t.Fatalf("credential unreadable after rotation: plain=%q err=%v", plain, err)
	}
}

func TestHandleRegenerateJWT_PersistsAndReinitializes(t *testing.T) {
	database := testutil.SetupTestDB(t)
	cfg := config.DefaultConfig()
	setServerTestKeys(cfg)
	cfg.Server.JWTSecret = "old-jwt-secret-that-is-32-bytes-ok!!"
	s := newJWTTestServer(t, database, cfg)
	s.configPath = t.TempDir() + "/config.yaml"

	crypto.InitLootEncryption(cfg.Crypto.LootKey)

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

	// Loot ciphertext encrypted before rotation must decrypt (independent key stable).
	plain, err := crypto.DecryptLoot(before)
	if err != nil || plain != "stable-loot" {
		t.Fatalf("loot lost after rotation: plain=%q err=%v", plain, err)
	}
}
