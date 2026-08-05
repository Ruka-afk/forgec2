package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Server.Port != 8000 {
		t.Errorf("expected port 8000, got %d", cfg.Server.Port)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("expected host 127.0.0.1, got %s", cfg.Server.Host)
	}
	if !cfg.Server.TLSEnabled {
		t.Errorf("expected tls_enabled true (secure default), got %v", cfg.Server.TLSEnabled)
	}
	if !cfg.Server.RequireTLSForAuth {
		t.Errorf("expected require_tls_for_auth true (secure default), got %v", cfg.Server.RequireTLSForAuth)
	}
	if cfg.Implant.DefaultInterval != 5 {
		t.Errorf("expected interval 5, got %d", cfg.Implant.DefaultInterval)
	}
	if cfg.Implant.DefaultJitter != 20 {
		t.Errorf("expected jitter 20, got %d", cfg.Implant.DefaultJitter)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("expected level info, got %s", cfg.Logging.Level)
	}
	if cfg.Listeners.MTLS.Addr != ":8443" {
		t.Errorf("expected mTLS addr :8443, got %s", cfg.Listeners.MTLS.Addr)
	}
	if cfg.AI.MaxConversationTurns != 0 {
		t.Errorf("expected AI max_conversation_turns 0 (unlimited), got %d", cfg.AI.MaxConversationTurns)
	}
	if cfg.AI.MaxToolRounds != 0 {
		t.Errorf("expected AI max_tool_rounds 0 (unlimited), got %d", cfg.AI.MaxToolRounds)
	}
	if cfg.AI.MaxDuplicateToolCalls != 0 {
		t.Errorf("expected AI max_duplicate_tool_calls 0 (unlimited), got %d", cfg.AI.MaxDuplicateToolCalls)
	}
}

func TestLoadNonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}
	if cfg.Server.Port != 8000 {
		t.Error("default port should be 8000")
	}
}

func TestLoadAndSave(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")

	cfg := DefaultConfig()
	cfg.Server.Port = 9090
	cfg.Server.JWTSecret = "test-secret-12345"

	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("config file was not created")
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Server.Port != 9090 {
		t.Errorf("expected port 9090, got %d", loaded.Server.Port)
	}
}

func TestJWTAutoGeneration(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")

	cfg := DefaultConfig()
	if cfg.Server.JWTSecret != "" {
		t.Error("default config should not have JWT secret")
	}

	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loaded.Server.JWTSecret) < 32 {
		t.Errorf("JWT secret too short: %d chars", len(loaded.Server.JWTSecret))
	}
}

func TestSaveConcurrentSafety(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")

	cfg := DefaultConfig()
	cfg.Server.JWTSecret = "test-secret-concurrent"

	done := make(chan bool, 5)
	for i := 0; i < 5; i++ {
		go func() {
			cfg.Save(path)
			done <- true
		}()
	}

	for i := 0; i < 5; i++ {
		<-done
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("config file should exist after concurrent saves")
	}
}

func TestIsWeakSecret(t *testing.T) {
	tests := []struct {
		secret string
		weak   bool
	}{
		{"", true},
		{"short", true},
		{"change_me", true},
		{"forgec2_secret_key_change_this_in_production", true},
		{"REDACTED_PLACEHOLDER_REPLACE_IN_PRODUCTION", true},
		{"my_placeholder_key_goes_here", true},
		{"secret", true},
		{"password", true},
		{"a]32chars-----------------------------------", false},
		{"a]32chars-----------------------------------2", false},
	}
	for _, tt := range tests {
		if got := isWeakSecret(tt.secret); got != tt.weak {
			t.Errorf("isWeakSecret(%q) = %v, want %v", tt.secret, got, tt.weak)
		}
	}
}

func TestIsWeakDefaultPassword(t *testing.T) {
	tests := []struct {
		pass string
		weak bool
	}{
		{"", true},
		{"Admin123!", true},
		{"admin", true},
		{"password123", true},
		{"Changeme1!", true},
		{"abcdefghijkl", true},
		{"NOSMALLCASE123456", true},
		{"Xy9!kZ2@mQ7vLp4w", false},
	}
	for _, tt := range tests {
		if got := isWeakDefaultPassword(tt.pass); got != tt.weak {
			t.Errorf("isWeakDefaultPassword(%q) = %v, want %v", tt.pass, got, tt.weak)
		}
	}
}

func TestValidateRejectsWeakDefaultPassword(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Auth.DefaultPasswd = "Admin123!"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() should reject weak auth.default_password")
	}
	cfg.Auth.DefaultPasswd = ""
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() with empty default_password should pass, got: %v", err)
	}
}

func TestAutoGenerateJWTSecret(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")

	// Write config with empty JWT secret
	cfg := DefaultConfig()
	cfg.Server.JWTSecret = ""
	out, _ := yaml.Marshal(cfg)
	os.WriteFile(path, out, 0644)

	// Load should auto-generate
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Server.JWTSecret == "" {
		t.Fatal("expected auto-generated JWT secret, got empty")
	}
	if len(loaded.Server.JWTSecret) != 64 { // 32 bytes hex
		t.Fatalf("expected 64-char hex secret, got %d chars", len(loaded.Server.JWTSecret))
	}
}

func TestAutoGenerateBeaconKey(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")

	cfg := DefaultConfig()
	cfg.Server.BeaconKey = ""
	out, _ := yaml.Marshal(cfg)
	os.WriteFile(path, out, 0644)

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Server.BeaconKey == "" {
		t.Fatal("expected auto-generated beacon key, got empty")
	}
	if len(loaded.Server.BeaconKey) != 64 { // 32 bytes hex
		t.Fatalf("expected 64-char hex beacon key, got %d chars", len(loaded.Server.BeaconKey))
	}
	if len(loaded.Server.JWTSecret) != 64 {
		t.Fatalf("JWT secret should also be generated alongside, got %d chars", len(loaded.Server.JWTSecret))
	}
}

func TestPreservesExplicitBeaconKey(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")

	cfg := DefaultConfig()
	cfg.Server.BeaconKey = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	out, _ := yaml.Marshal(cfg)
	os.WriteFile(path, out, 0644)

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Server.BeaconKey != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("explicit beacon key should be preserved, got %q", loaded.Server.BeaconKey)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr bool
		errMsg  string
	}{
		{"valid defaults", func(c *Config) {}, false, ""},
		{"port too low", func(c *Config) { c.Server.Port = 0 }, true, "server.port"},
		{"port too high", func(c *Config) { c.Server.Port = 99999 }, true, "server.port"},
		{"offline threshold zero", func(c *Config) { c.Server.OfflineThreshold = 0 }, true, "offline_threshold"},
		{"session max age zero", func(c *Config) { c.Server.SessionMaxAgeHours = 0 }, true, "session_max_age"},
		{"interval zero", func(c *Config) { c.Implant.DefaultInterval = 0 }, true, "default_interval"},
		{"jitter negative", func(c *Config) { c.Implant.DefaultJitter = -1 }, true, "default_jitter"},
		{"jitter over 100", func(c *Config) { c.Implant.DefaultJitter = 101 }, true, "default_jitter"},
		{"invalid log level", func(c *Config) { c.Logging.Level = "verbose" }, true, "logging.level"},
		{"login attempts zero", func(c *Config) { c.RateLimit.Login.MaxAttempts = 0 }, true, "max_attempts"},
		{"tcp enabled without addr", func(c *Config) { c.Server.TCPEnabled = true; c.Server.TCPAddr = "" }, true, "tcp_addr"},
		{"smb enabled without pipe", func(c *Config) { c.Server.SMBEnabled = true; c.Server.SMBPipe = "" }, true, "smb_pipe"},
		{"dns enabled without domain", func(c *Config) { c.Server.DNSEnabled = true; c.Server.DNSDomain = "" }, true, "dns_domain"},
		{"icmp enabled without addr", func(c *Config) { c.Server.ICMPEnabled = true; c.Server.ICMPAddr = "" }, true, "icmp_addr"},
		{"grpc enabled without addr", func(c *Config) { c.Server.GRPCEnabled = true; c.Server.GRPCAddr = "" }, true, "grpc_addr"},
		{"ssh enabled with zero port", func(c *Config) { c.Server.SSHEnabled = true; c.Server.SSHPort = 0 }, true, "ssh_port"},
		{"tls enabled without cert", func(c *Config) { c.Server.TLSEnabled = true; c.Server.CertFile = "" }, true, "cert_file"},
		{"tls enabled without key", func(c *Config) { c.Server.TLSEnabled = true; c.Server.KeyFile = "" }, true, "key_file"},
		{"sso enabled without client_id", func(c *Config) { c.SSO.Enabled = true; c.SSO.ClientID = "" }, true, "client_id"},
		{"api capacity zero", func(c *Config) { c.RateLimit.API.Capacity = 0 }, true, "capacity"},
		{"invalid socks dest", func(c *Config) { c.Socks.Enabled = true; c.Socks.AllowedDests = []string{"nocolon"} }, true, "host:port"},
		{"valid socks dest", func(c *Config) { c.Socks.Enabled = true; c.Socks.AllowedDests = []string{"10.0.0.1:443"} }, false, ""},
		{"mtls enabled without addr", func(c *Config) { c.Listeners.MTLS.Enabled = true; c.Listeners.MTLS.Addr = "" }, true, "mtls.addr"},
		{"mtls enabled without cert", func(c *Config) { c.Listeners.MTLS.Enabled = true; c.Listeners.MTLS.CertFile = "" }, true, "mtls.cert_file"},
		{"h2c enabled without addr", func(c *Config) { c.Listeners.H2C.Enabled = true; c.Listeners.H2C.Addr = "" }, true, "h2c.addr"},
		{"wg enabled without addr", func(c *Config) { c.Listeners.WG.Enabled = true; c.Listeners.WG.Addr = "" }, true, "wg.addr"},
		{"wg enabled without private key", func(c *Config) { c.Listeners.WG.Enabled = true; c.Listeners.WG.Addr = ":51820"; c.Listeners.WG.PrivateKey = "" }, true, "wg.private_key"},
		{"negative AI max_conversation_turns", func(c *Config) { c.AI.MaxConversationTurns = -1 }, true, "max_conversation_turns"},
		{"negative AI max_tool_rounds", func(c *Config) { c.AI.MaxToolRounds = -1 }, true, "max_tool_rounds"},
		{"negative AI max_duplicate_tool_calls", func(c *Config) { c.AI.MaxDuplicateToolCalls = -1 }, true, "max_duplicate_tool_calls"},
		{"zero AI limits (unlimited)", func(c *Config) { c.AI.MaxConversationTurns = 0; c.AI.MaxToolRounds = 0; c.AI.MaxDuplicateToolCalls = 0 }, false, ""},
		{"backup_key too short", func(c *Config) { c.Crypto.BackupKey = "aabb" }, true, "backup_key"},
		{"backup_key not hex", func(c *Config) { c.Crypto.BackupKey = strings.Repeat("zz", 32) }, true, "backup_key"},
		{"backup_key valid 64 hex", func(c *Config) { c.Crypto.BackupKey = strings.Repeat("ab", 32) }, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.modify(cfg)
			err := cfg.Validate()
			if tt.wantErr && err == nil {
				t.Errorf("expected error containing %q, got nil", tt.errMsg)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tt.wantErr && err != nil && !containsStr(err.Error(), tt.errMsg) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.errMsg)
			}
		})
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstr(s, sub))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
