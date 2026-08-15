package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/gin-gonic/gin"
)

// TestDownloadConfigRedactsSecrets ensures the admin "Download Config"
// endpoint redacts every crown-jewel secret, not just a hand-picked subset
// (S2): loot/TOTP/CSRF/ExtC2/backup keys, v2 beacon key, SSH password, and
// the extc2 API token must never appear in cleartext.
func TestDownloadConfigRedactsSecrets(t *testing.T) {
	gin.SetMode(gin.TestMode)

	loot := "LOOT_KEY_" + strings.Repeat("a", 32)
	content := fmt.Sprintf(`server:
  jwt_secret: JWT_SECRET_GHI
  beacon_key: BEACON_SECRET_JKL
  ssh_password: SSH_PASS_MNO
crypto:
  key: CRYPTO_KEY_111
  loot_key: %s
  totp_key: TOTP_SECRET_ABC
  csrf_key: CSRF_SECRET_DEF
  extc2_key: EXTC2_KEY_222
  backup_key: BACKUP_KEY_333
rate_limit:
  extc2:
    api_token: EXTC2_TOKEN_PQR
ai:
  api_key: AI_KEY_ZZZ
`, loot)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	s := &Server{ctx: context.Background(), cfg: &config.Config{}, configPath: cfgPath}
	s.cfg.Server.JWTSecret = "JWT_SECRET_GHI"
	s.cfg.Server.BeaconKey = "BEACON_SECRET_JKL"
	s.cfg.Server.SSHPassword = "SSH_PASS_MNO"
	s.cfg.Crypto.Key = "CRYPTO_KEY_111"
	s.cfg.Crypto.LootKey = loot
	s.cfg.Crypto.TotpKey = "TOTP_SECRET_ABC"
	s.cfg.Crypto.CsrfKey = "CSRF_SECRET_DEF"
	s.cfg.Crypto.ExtC2Key = "EXTC2_KEY_222"
	s.cfg.Crypto.BackupKey = "BACKUP_KEY_333"
	s.cfg.RateLimit.ExtC2.APIToken = "EXTC2_TOKEN_PQR"
	s.cfg.AI.APIKey = "AI_KEY_ZZZ"

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("user_role", "admin")
	s.handleDownloadConfig(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()

	leaks := []string{
		loot, "JWT_SECRET_GHI", "BEACON_SECRET_JKL", "SSH_PASS_MNO",
		"CRYPTO_KEY_111", "TOTP_SECRET_ABC", "CSRF_SECRET_DEF",
		"EXTC2_KEY_222", "BACKUP_KEY_333", "EXTC2_TOKEN_PQR", "AI_KEY_ZZZ",
	}
	for _, sec := range leaks {
		if strings.Contains(body, sec) {
			t.Fatalf("secret leaked in downloaded config: %s\n---\n%s", sec, body)
		}
	}
	if !strings.Contains(body, "loot_key: ****") {
		t.Fatalf("expected redacted loot_key marker, got:\n%s", body)
	}
}
