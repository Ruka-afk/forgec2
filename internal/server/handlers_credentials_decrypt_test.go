package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gin-gonic/gin"
)

// TestCredentialListReturnsDecryptedSecrets proves the vault list endpoint
// honors its contract: the AfterFind hook must decrypt Password/Hash, and
// a future "decryption storm" optimization (SkipHooks on this path) would
// silently ship ciphertext to the masked UI fields. A 5000-row benchmark
// showed decryption is not the bottleneck there (AES-GCM ~0.4% of read
// time), so there is nothing to win by breaking this.
func TestCredentialListReturnsDecryptedSecrets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	crypto.InitLootEncryption(testStorageKeyHex)
	crypto.InitExtC2Encryption("0123456789abcdef0123456789abcdef")
	s := &Server{db: testutil.SetupTestDB(t), cfg: &config.Config{}}

	entry := db.CredentialEntry{
		AgentID: "a1", Domain: "corp.local", Username: "svc",
		Password: "S3cret!", Hash: "aad3b435b51404ee", Type: "cleartext", Source: "test",
	}
	if err := s.db.Create(&entry).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	s.handleCredentialsPage(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{"S3cret!", "aad3b435b51404ee"} {
		if !strings.Contains(body, want) {
			t.Fatalf("response is missing decrypted %q; the list contract is broken", want)
		}
	}
}

// TestCredentialExportFailsLoudly proves a failed export query answers 500
// instead of streaming an empty CSV that reads as "the vault is empty".
func TestCredentialExportFailsLoudly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	crypto.InitLootEncryption(testStorageKeyHex)
	crypto.InitExtC2Encryption("0123456789abcdef0123456789abcdef")
	s := &Server{db: testutil.SetupTestDB(t), cfg: &config.Config{}}

	// requireExportStepUp needs an operator identity; seed one.
	if err := s.db.Create(&db.User{Username: "exp-admin", Role: "admin", IsActive: true, TenantID: 0}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	sqlDB, err := s.db.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Set("user", "exp-admin")
	c.Set("user_role", "admin")
	s.handleExportCredentials(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s, want 500 (an empty CSV would read as 'no credentials')", w.Code, w.Body.String())
	}
}
