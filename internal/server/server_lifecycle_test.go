package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gin-gonic/gin"
)

const testStorageKeyHex = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"

// setServerTestKeys fills all REQUIRED independent storage keys so tests can
// build a config that passes startup validation / server construction.
func setServerTestKeys(cfg *config.Config) {
	cfg.Crypto.LootKey = testStorageKeyHex
	cfg.Crypto.ExtC2Key = testStorageKeyHex
	cfg.Crypto.BackupKey = testStorageKeyHex
	cfg.Crypto.TotpKey = testStorageKeyHex
	cfg.Crypto.CsrfKey = testStorageKeyHex
}

func TestNewInitializesServer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)
	cfg := config.DefaultConfig()
	cfg.Server.JWTSecret = "test-secret-for-new-test-32char-len!"
	setServerTestKeys(cfg)
	crypto.InitLootEncryption(cfg.Crypto.LootKey)

	srv := New(cfg, db)
	if srv == nil {
		t.Fatal("New() returned nil server")
	}
	if srv.db == nil {
		t.Error("db is nil")
	}
	if srv.cfg == nil {
		t.Error("cfg is nil")
	}
	if srv.eventManager == nil {
		t.Error("eventManager is nil")
	}
	if srv.loginLockout == nil {
		t.Error("loginLockout is nil")
	}
}

func TestShutdownCompletes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)
	cfg := config.DefaultConfig()
	cfg.Server.JWTSecret = "test-secret-for-shutdown-test-32char!"
	setServerTestKeys(cfg)
	crypto.InitLootEncryption(cfg.Crypto.LootKey)

	srv := New(cfg, db)
	srv.SetupRoutes()
	srv.SetStaticFS(nil)

	done := make(chan struct{})
	go func() {
		srv.Shutdown()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Shutdown() did not complete within 10 seconds")
	}
}

func TestHandleHealthCheckWithDBFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)
	s := &Server{db: db, startTime: time.Now()}

	t.Run("healthy DB returns 200", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/health", nil)
		s.handleHealthCheck(c)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d; body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("closed DB returns 503", func(t *testing.T) {
		sqlDB, err := db.DB()
		if err != nil {
			t.Fatalf("get sql.DB: %v", err)
		}
		sqlDB.Close()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/health", nil)
		s.handleHealthCheck(c)
		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("expected 503, got %d; body=%s", w.Code, w.Body.String())
		}
	})
}

func TestHandleReadyCheckWithDBFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)
	s := &Server{db: db, startTime: time.Now()}

	t.Run("healthy DB returns 200", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/ready", nil)
		s.handleReadyCheck(c)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d; body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("closed DB returns 503", func(t *testing.T) {
		sqlDB, err := db.DB()
		if err != nil {
			t.Fatalf("get sql.DB: %v", err)
		}
		sqlDB.Close()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/ready", nil)
		s.handleReadyCheck(c)
		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("expected 503, got %d; body=%s", w.Code, w.Body.String())
		}
	})
}
