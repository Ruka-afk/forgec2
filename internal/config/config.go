package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration for ForgeC2
type Config struct {
	mu         sync.Mutex `yaml:"-"`
	ConfigPath string     `yaml:"-"` // absolute path to the config file, set on Load
	Server     struct {
		Port                 int           `yaml:"port"`
		Host                 string        `yaml:"host"`
		TLSEnabled           bool          `yaml:"tls_enabled"`
		CertFile             string        `yaml:"cert_file"`
		KeyFile              string        `yaml:"key_file"`
		ClientCAFile         string        `yaml:"client_ca_file"`      // mTLS: CA cert file for client verification
		RequireClientCert    bool          `yaml:"require_client_cert"` // mTLS: require client certificate for beacon auth
		JWTSecret            string        `yaml:"jwt_secret"`
		TCPEnabled           bool          `yaml:"tcp_enabled"`
		TCPAddr              string        `yaml:"tcp_addr"`
		SMBEnabled           bool          `yaml:"smb_enabled"`
		SMBPipe              string        `yaml:"smb_pipe"`
		DataDir              string        `yaml:"data_dir"`
		DNSEnabled           bool          `yaml:"dns_enabled"`
		DNSDomain            string        `yaml:"dns_domain"`
		DNSAddr              string        `yaml:"dns_addr"`
		GRPCEnabled          bool          `yaml:"grpc_enabled"`
		GRPCAddr             string        `yaml:"grpc_addr"`
		ICMPEnabled           bool          `yaml:"icmp_enabled"`
		ICMPAddr             string        `yaml:"icmp_addr"`
		UDPEnabled           bool          `yaml:"udp_enabled"`
		UDPAddr              string        `yaml:"udp_addr"`
		OfflineThreshold     int           `yaml:"offline_threshold"`      // seconds
		SessionMaxAgeHours   int           `yaml:"session_max_age_hours"`  // JWT expiry
		CleanupRetentionDays int           `yaml:"cleanup_retention_days"` // auto-purge cutoff
		UpdateCheckRepo      string        `yaml:"update_check_repo"`      // GitHub repo for update checks (e.g. "owner/repo")
		UpdateCheckEnabled   bool          `yaml:"update_check_enabled"`   // OPT-IN: phone home to GitHub releases (default OFF for egress hygiene)
		VantagePoints        []string      `yaml:"vantage_points"`         // external proxy URLs for circuit breaker probing
		SSHEnabled           bool          `yaml:"ssh_enabled"`            // enable SSH transport listener
		SSHPort              int           `yaml:"ssh_port"`               // SSH listener port (default 2222)
		SSHAddr              string        `yaml:"ssh_addr"`               // SSH listener addr (default :ssh_port)
		SSHHostKey           string        `yaml:"ssh_host_key"`           // path to SSH host key (auto-generated if missing)
		SSHUser              string        `yaml:"ssh_user"`               // SSH user for agent auth
		SSHPassword          string        `yaml:"ssh_password"`           // SSH password (empty = any password or key-only)
		SSHKeyAuth           bool          `yaml:"ssh_key_auth"`           // allow public key authentication
		GeoIPEnabled         bool          `yaml:"geoip_enabled"`          // enable GeoIP lookup via ip-api.com (opt-in)
		AllowedOrigins       []string      `yaml:"allowed_origins"`        // allowed WebSocket/CORS origins (default: localhost,127.0.0.1,::1)
		TrustedProxies       []string      `yaml:"trusted_proxies"`        // trusted reverse proxy IPs/CIDRs for X-Forwarded-For (empty = trust none, use direct client IP)
		CookieDomain         string        `yaml:"cookie_domain"`          // domain for session/CSRF cookies (for cross-origin deployments)
		BeaconKey            string        `yaml:"beacon_key"`             // optional pre-shared key for agent beacon auth (X-Beacon-Key header)
		RequireTLSForAuth    bool          `yaml:"require_tls_for_auth"`   // require TLS before issuing session cookies (strongly recommended in production)
		EnablePprof          bool          `yaml:"enable_pprof"`           // expose /debug/pprof (default false; requires auth when enabled)
		EnableMetrics        bool          `yaml:"enable_metrics"`         // expose /metrics (default false; requires auth when enabled)
		SocksListenHost      string        `yaml:"socks_listen_host"`      // bind host for SOCKS/rportfwd (default 127.0.0.1)
		DBMaxOpenConns       int           `yaml:"db_max_open_conns"`      // max open connections for PostgreSQL pool (default 25)
		DBMaxIdleConns       int           `yaml:"db_max_idle_conns"`      // max idle connections for PostgreSQL pool (default 5)
		DBConnMaxLifetime    time.Duration `yaml:"db_conn_max_lifetime"`   // max connection lifetime for PostgreSQL pool (default 30m)
	} `yaml:"server"`

	Database struct {
		Path   string `yaml:"path"`
		Driver string `yaml:"driver"` // "sqlite" (default) or "postgres"
		DSN    string `yaml:"dsn"`    // PostgreSQL connection string (e.g. "host=localhost user=forgec2 password=... dbname=forgec2 port=5432 sslmode=disable")
	} `yaml:"database"`

	Implant struct {
		DefaultInterval     int    `yaml:"default_interval"` // seconds
		MinInterval         int    `yaml:"min_interval"`     // OPSEC: minimum allowed beacon interval in seconds (0 = no minimum, default 5)
		DefaultJitter       int    `yaml:"default_jitter"`   // percent
		MinJitter           int    `yaml:"min_jitter"`       // OPSEC: minimum allowed jitter percent (0 = no minimum, default 10)
		DefaultUA           string `yaml:"default_user_agent"`
		DefaultSkipTLS      bool   `yaml:"default_skip_tls"`
		DefaultWorkingStart string `yaml:"default_working_start"` // HH:MM local time (empty = disabled)
		DefaultWorkingEnd   string `yaml:"default_working_end"`   // HH:MM local time (empty = disabled)
		DefaultWorkingTZ    string `yaml:"default_working_tz"`    // IANA timezone (e.g. "America/New_York"), empty = UTC
		DNSDoHURL           string `yaml:"dns_doh_url"`           // DNS-over-HTTPS endpoint
		DNSDoTAddr          string `yaml:"dns_dot_addr"`          // DNS-over-TLS address:port
		DNSIPv6             bool   `yaml:"dns_ipv6"`              // enable IPv6 AAAA tunneling
	} `yaml:"implant"`

	Auth struct {
		PasswordHash  string `yaml:"password_hash"`    // bcrypt hash, set on first run
		DefaultPasswd string `yaml:"default_password"` // plaintext; used only on first boot if password_hash is empty
	} `yaml:"auth"`

	PasswordPolicy struct {
		MinLength     int  `yaml:"min_length"`     // minimum password length (default 8)
		RequireUpper  bool `yaml:"require_upper"`  // require uppercase letter (default true)
		RequireLower  bool `yaml:"require_lower"`  // require lowercase letter (default true)
		RequireDigit  bool `yaml:"require_digit"`  // require digit (default true)
		RequireSymbol bool `yaml:"require_symbol"` // require special character (default false)
		MaxAge        int  `yaml:"max_age_days"`   // force password rotation every N days (0 = disabled)
		BcryptCost    int  `yaml:"bcrypt_cost"`    // bcrypt hash cost (0 = default 10, min 4, max 31)
	} `yaml:"password_policy"`

	Crypto struct {
		Key                     string `yaml:"key"`                        // 32-byte hex key for XOR encryption, or "ecdh:" for ECDH+AES-256-GCM (empty=disabled)
		LootKey                 string `yaml:"loot_key"`                   // REQUIRED: 32-byte hex key for AES-256-GCM loot encryption (independent of the JWT secret)
		ExtC2Key                string `yaml:"extc2_key"`                  // REQUIRED: 32-byte hex key for AES-256-GCM ExtC2 channel encryption (independent)
		BackupKey               string `yaml:"backup_key"`                 // REQUIRED: 32-byte hex key for encrypted .fbk backups (independent)
		TotpKey                 string `yaml:"totp_key"`                   // REQUIRED: 32-byte hex key for TOTP secrets / SMTP / SSH-redirector credentials (independent)
		CsrfKey                 string `yaml:"csrf_key"`                   // REQUIRED: 32-byte hex key for CSRF token binding (independent)
		ForceECDH               bool   `yaml:"force_ecdh"`                 // reject plaintext beacons when ECDH is enabled
		MaxDecryptedPayloadSize int    `yaml:"max_decrypted_payload_size"` // max bytes for decrypted beacon body (0 = default 10MB)
	} `yaml:"crypto"`

	Malleable struct {
		Enabled     bool              `yaml:"enabled"`
		ProfileName string            `yaml:"profile_name"` // preset name: default, microsoft, google_analytics, cloudflare_cdn, akamai
		StatusCode  int               `yaml:"status_code"`
		ContentType string            `yaml:"content_type"`
		Headers     map[string]string `yaml:"headers"`
		Prepend     string            `yaml:"prepend"`
		Append      string            `yaml:"append"`
		// Request-side transforms: applied by the agent to the OUTGOING beacon
		// body (and as request headers) and stripped by the server on inbound.
		// Kept separate from the response-side Prepend/Append so operators can
		// shape upload and download traffic independently (Cobalt-Strike-style
		// http-get/http-post prepend/append).
		RequestPrepend  string            `yaml:"request_prepend"`
		RequestAppend   string            `yaml:"request_append"`
		RequestHeaders  map[string]string `yaml:"request_headers"`
	} `yaml:"malleable"`

	AI struct {
		Enabled               bool   `yaml:"enabled"`
		Provider              string `yaml:"provider"` // deepseek, openai, claude, qianwen, custom
		APIKey                string `yaml:"api_key"`
		Model                 string `yaml:"model"`
		Endpoint              string `yaml:"endpoint"` // optional, override default
		SystemPrompt          string `yaml:"system_prompt"`
		MaxConversationTurns  int    `yaml:"max_conversation_turns"`   // 0 = unlimited (default)
		MaxToolRounds         int    `yaml:"max_tool_rounds"`          // 0 = unlimited (default)
		MaxDuplicateToolCalls int    `yaml:"max_duplicate_tool_calls"` // 0 = unlimited; else cap identical tool+args repeats
		AllowExecute          bool   `yaml:"allow_execute"`            // permit AI to run commands on agents (default false = safe)
	} `yaml:"ai"`

	Logging struct {
		Level string `yaml:"level"` // debug, info, warn, error
	} `yaml:"logging"`

	SIEM struct {
		Enabled bool   `yaml:"enabled"`
		URL     string `yaml:"url"`     // webhook URL for SIEM event forwarding
		Token   string `yaml:"token"`   // optional bearer token for SIEM webhook
		Actions string `yaml:"actions"` // comma-separated action filters (empty = all)
	} `yaml:"siem"`

	Integrations struct {
		Slack SlackConfig `yaml:"slack"`
	} `yaml:"integrations"`

	Listeners ListenersConfig `yaml:"listeners"`

	// Allow external programs to write plain SOCKS5 operations server -> agent
	Socks struct {
		Enabled      bool        `yaml:"enabled"`
		AuthRequired bool        `yaml:"auth_required"`
		Users        []SocksUser `yaml:"users"`
		AllowedDests []string    `yaml:"allowed_destinations"`
	} `yaml:"socks"`

	TLSFingerprint TLSFingerprintConfig `yaml:"tls_fingerprint"`

	RateLimit struct {
		Login struct {
			MaxAttempts int      `yaml:"max_attempts"` // max login attempts per window
			Window      int      `yaml:"window"`       // window in seconds
			LockoutTime int      `yaml:"lockout_time"` // lockout duration in seconds
			Whitelist   []string `yaml:"whitelist"`    // whitelisted IPs
		} `yaml:"login"`
		API struct {
			Capacity  float64  `yaml:"capacity"`  // token bucket capacity (max burst)
			Rate      float64  `yaml:"rate"`      // tokens per second per user
			Whitelist []string `yaml:"whitelist"` // whitelisted IPs
		} `yaml:"api"`
		Beacon struct {
			Limit  int `yaml:"limit"`  // requests per window
			Window int `yaml:"window"` // window in seconds
		} `yaml:"beacon"`
		ExtC2 struct {
			Enabled    bool    `yaml:"enabled"`
			APIToken   string  `yaml:"api_token"`   // shared secret for extc2 API auth (empty = no auth)
			Rate       float64 `yaml:"rate"`        // requests per second per beacon ID
			Burst      int     `yaml:"burst"`       // max burst size
			CleanupAge int     `yaml:"cleanup_age"` // minutes to keep idle entries
		} `yaml:"extc2"`
	} `yaml:"rate_limit"`

	// Security holds operator-facing guardrails.
	Security struct {
		// RequireApproval enforces a two-man rule: task types flagged
		// RequiresApproval (irreversible / high-impact ops) are created
		// in "pending_approval" state and must be approved by an operator
		// DIFFERENT from the task creator before the beacon claims them.
		RequireApproval bool `yaml:"require_approval"`
	} `yaml:"security"`
}

// DefaultConfig returns sensible defaults
func DefaultConfig() *Config {
	cfg := &Config{}
	cfg.Server.Port = 8000
	cfg.Server.Host = "127.0.0.1" // change to "0.0.0.0" for production access
	cfg.Server.TLSEnabled = true  // self-signed cert auto-generated on first run
	cfg.Server.RequireTLSForAuth = true
	cfg.Server.TCPEnabled = false
	cfg.Server.TCPAddr = ""
	cfg.Server.SMBEnabled = false
	cfg.Server.SMBPipe = "forgec2"
	cfg.Server.DNSEnabled = false
	cfg.Server.DNSDomain = ""
	cfg.Server.DNSAddr = ":53"
	cfg.Server.ICMPEnabled = false
	cfg.Server.ICMPAddr = "0.0.0.0"
	cfg.Server.UDPEnabled = false
	cfg.Server.UDPAddr = ":8899"
	cfg.Server.SSHEnabled = false
	cfg.Server.SSHPort = 2222
	cfg.Server.SSHAddr = ""
	cfg.Server.SSHUser = "forgec2"
	cfg.Server.SSHPassword = ""
	cfg.Server.SSHKeyAuth = true
	cfg.Server.DataDir = "data"
	cfg.Server.SSHHostKey = filepath.Join(cfg.Server.DataDir, "ssh_host_key")
	cfg.Server.OfflineThreshold = 60
	cfg.Server.SessionMaxAgeHours = 24
	cfg.Server.CleanupRetentionDays = 30
	cfg.Server.UpdateCheckRepo = "forgec2/forgec2"
	cfg.Server.UpdateCheckEnabled = false // update checks phone home to api.github.com — opt-in only
	cfg.Server.EnablePprof = false
	cfg.Server.EnableMetrics = false
	cfg.Server.SocksListenHost = "127.0.0.1"
	cfg.Server.DBMaxOpenConns = 25
	cfg.Server.DBMaxIdleConns = 5
	cfg.Server.DBConnMaxLifetime = 30 * time.Minute

	// Secure by default: enforce the two-man rule for all task types flagged
	// dangerous (see server/tasktypes.go dangerousTaskTypes). Solo operators
	// can still approve their own tasks; set security.require_approval: false
	// to disable. The dangerous list now covers credential-access (creds,
	// mimikatz, kerberoast, dpapi_*) and destructive delete.
	cfg.Security.RequireApproval = true

	cfg.Database.Path = filepath.Join(cfg.Server.DataDir, "db/forgec2.db")
	cfg.Database.Driver = "sqlite"
	cfg.Server.CertFile = filepath.Join(cfg.Server.DataDir, "server.crt")
	cfg.Server.KeyFile = filepath.Join(cfg.Server.DataDir, "server.key")

	cfg.Implant.DefaultInterval = 5
	cfg.Implant.MinInterval = 5
	cfg.Implant.MinJitter = 10

	cfg.PasswordPolicy.MinLength = 8
	cfg.PasswordPolicy.RequireUpper = true
	cfg.PasswordPolicy.RequireLower = true
	cfg.PasswordPolicy.RequireDigit = true
	cfg.PasswordPolicy.RequireSymbol = false
	cfg.PasswordPolicy.MaxAge = 0
	cfg.Implant.DefaultJitter = 20
	cfg.Implant.DefaultUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36"
	cfg.Implant.DefaultWorkingStart = ""
	cfg.Implant.DefaultWorkingEnd = ""
	cfg.Implant.DefaultWorkingTZ = ""
	cfg.Implant.DNSDoHURL = "https://dns.google/dns-query"
	cfg.Implant.DNSDoTAddr = "1.1.1.1:853"
	cfg.Implant.DNSIPv6 = false

	cfg.Malleable.Enabled = false
	cfg.Malleable.StatusCode = 200
	cfg.Malleable.ContentType = "application/json"
	cfg.Malleable.Headers = map[string]string{
		"Server": "nginx/1.24.0",
	}
	cfg.AI.Enabled = false
	cfg.AI.Provider = "deepseek"
	cfg.AI.Model = "deepseek-chat"
	cfg.AI.MaxConversationTurns = 0  // 0 = unlimited
	cfg.AI.MaxToolRounds = 0         // 0 = unlimited
	cfg.AI.MaxDuplicateToolCalls = 0 // 0 = unlimited
	cfg.AI.SystemPrompt = "You are the ForgeC2 red team operations assistant, running on the C2 server. You can list online agents, view target details, execute commands, view credentials, manage listeners, and more."
	cfg.TLSFingerprint.JARMEnabled = true
	cfg.TLSFingerprint.JARMRotate = "24h"
	cfg.TLSFingerprint.JA3Enabled = true
	cfg.TLSFingerprint.JA3Profile = "random"
	cfg.TLSFingerprint.JA3Rotate = "24h"

	cfg.Logging.Level = "info"

	// Secure default: ECDH+AES-256-GCM beacon encryption enabled and plaintext
	// beacons rejected. Operators who need legacy XOR or unencrypted beacons can
	// explicitly override crypto.key / crypto.force_ecdh in config.yaml.
	cfg.Crypto.Key = "ecdh:"
	cfg.Crypto.ForceECDH = true

	cfg.RateLimit.Login.MaxAttempts = 5
	cfg.RateLimit.Login.Window = 60
	cfg.RateLimit.Login.LockoutTime = 900
	cfg.RateLimit.Login.Whitelist = []string{}

	cfg.RateLimit.API.Capacity = 100
	cfg.RateLimit.API.Rate = 50
	cfg.RateLimit.API.Whitelist = []string{"127.0.0.1", "::1"}

	cfg.RateLimit.Beacon.Limit = 100
	cfg.RateLimit.Beacon.Window = 60

	cfg.Socks.Enabled = false
	cfg.Socks.AuthRequired = false
	cfg.Socks.Users = []SocksUser{}
	cfg.Socks.AllowedDests = []string{}

	cfg.RateLimit.ExtC2.Enabled = true
	cfg.RateLimit.ExtC2.Rate = 10
	cfg.RateLimit.ExtC2.Burst = 20
	cfg.RateLimit.ExtC2.CleanupAge = 30

	// Listener defaults
	cfg.Listeners.H2C.Enabled = false
	cfg.Listeners.H2C.Addr = ":8081"

	return cfg
}

// isWeakSecret returns true if the secret is commonly known, too short, or trivially guessable.
func isWeakSecret(s string) bool {
	if len(s) < 32 {
		return true
	}
	weak := []string{
		"forgec2_secret_key_change_this_in_production",
		"change_me", "changeme", "change_this", "secret", "password", "admin",
		"jwt_secret", "your_secret_here", "todo", "placeholder", "redacted",
		"replace_in_production", "replace_me",
		"0b0d003179038fb3742671c8242100f77f03be74aa69dbc38139fe1e500d9dfd",
	}
	lower := strings.ToLower(s)
	for _, w := range weak {
		if strings.Contains(lower, w) {
			return true
		}
	}
	return false
}

// isWeakDefaultPassword rejects trivially guessable first-boot admin
// passwords (e.g. the historical "Admin123!" example value). Operators must
// either choose a strong password or leave default_password empty so the
// server auto-generates a random one.
func isWeakDefaultPassword(s string) bool {
	if len(s) < 12 {
		return true
	}
	lower := strings.ToLower(s)
	weak := []string{
		"admin", "password", "changeme", "change_me", "123456", "12345678",
		"qwerty", "letmein", "welcome", "forgec2", "default", "test",
	}
	for _, w := range weak {
		if strings.Contains(lower, w) {
			return true
		}
	}
	hasUpper := false
	hasLower := false
	hasDigit := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			hasLower = true
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= '0' && r <= '9':
			hasDigit = true
		}
	}
	if !(hasUpper && hasLower && hasDigit) {
		return true
	}
	return false
}

// Load loads config from file, creates default if not exists
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// create default config file
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				return nil, err
			}
			// Fresh deployment: mint independent random storage keys now so
			// loot/ExtC2/backup/TOTP/CSRF primitives are cryptographically
			// isolated from the JWT secret (a JWT compromise must not decrypt
			// credentials, backups, or bind CSRF tokens).
			for _, gen := range []struct {
				field *string
				slogW string
			}{
				{&cfg.Server.JWTSecret, "JWT secret"},
				{&cfg.Crypto.LootKey, "loot key"},
				{&cfg.Crypto.ExtC2Key, "ExtC2 key"},
				{&cfg.Crypto.BackupKey, "backup key"},
				{&cfg.Crypto.TotpKey, "TOTP key"},
				{&cfg.Crypto.CsrfKey, "CSRF key"},
				{&cfg.Server.BeaconKey, "beacon key"},
			} {
				key := make([]byte, 32)
				if _, rerr := rand.Read(key); rerr != nil {
					return nil, rerr
				}
				*gen.field = hex.EncodeToString(key)
			}
			out, _ := yaml.Marshal(cfg)
			if err := os.WriteFile(path, out, 0600); err != nil {
				return nil, err
			}
			cfg.ConfigPath = path
			return cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	cfg.ConfigPath = path

	// Env override for JWT secret (takes precedence over config file)
	if envSecret := os.Getenv("FORGEC2_JWT_SECRET"); envSecret != "" {
		cfg.Server.JWTSecret = envSecret
	}

	// Auto-generate JWT secret if using default/insecure value
	if cfg.Server.JWTSecret == "" || isWeakSecret(cfg.Server.JWTSecret) {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		cfg.Server.JWTSecret = hex.EncodeToString(key)
		if err := cfg.Save(path); err != nil {
			return nil, err
		}
		slog.Warn("JWT secret auto-generated. To use a custom secret, set FORGEC2_JWT_SECRET env var or edit server.jwt_secret in config.yaml")
	}

	// Auto-generate beacon pre-shared key if empty (agents must be built with it)
	if cfg.Server.BeaconKey == "" || isWeakSecret(cfg.Server.BeaconKey) {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		cfg.Server.BeaconKey = hex.EncodeToString(key)
		if err := cfg.Save(path); err != nil {
			return nil, err
		}
		slog.Warn("Beacon key auto-generated. To use a custom key, set server.beacon_key in config.yaml")
	}

	// Auto-generate ExtC2 API token if using default/insecure value
	if cfg.RateLimit.ExtC2.APIToken == "" || isWeakSecret(cfg.RateLimit.ExtC2.APIToken) {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		cfg.RateLimit.ExtC2.APIToken = hex.EncodeToString(key)
		if err := cfg.Save(path); err != nil {
			return nil, err
		}
		slog.Warn("ExtC2 API token auto-generated. To use a custom token, edit rate_limit.extc2.api_token in config.yaml")
	}

	// Env override for loot key (takes precedence over config file)
	if envLootKey := os.Getenv("FORGEC2_LOOT_KEY"); envLootKey != "" {
		cfg.Crypto.LootKey = envLootKey
	}

	// Env override for ExtC2 key (takes precedence over config file)
	if envExtC2Key := os.Getenv("FORGEC2_EXTC2_KEY"); envExtC2Key != "" {
		cfg.Crypto.ExtC2Key = envExtC2Key
	}

	// Env override for backup key (takes precedence over config file)
	if envBackupKey := os.Getenv("FORGEC2_BACKUP_KEY"); envBackupKey != "" {
		cfg.Crypto.BackupKey = envBackupKey
	}

	// Env override for TOTP key (takes precedence over config file)
	if envTotpKey := os.Getenv("FORGEC2_TOTP_KEY"); envTotpKey != "" {
		cfg.Crypto.TotpKey = envTotpKey
	}

	// Env override for CSRF key (takes precedence over config file)
	if envCsrfKey := os.Getenv("FORGEC2_CSRF_KEY"); envCsrfKey != "" {
		cfg.Crypto.CsrfKey = envCsrfKey
	}

	// Env override for AI API key (takes precedence over config file)
	if envAIKey := os.Getenv("FORGEC2_AI_API_KEY"); envAIKey != "" {
		cfg.AI.APIKey = envAIKey
	}

	// Env overrides for critical settings
	if envDBPath := os.Getenv("FORGEC2_DB_PATH"); envDBPath != "" {
		cfg.Database.Path = envDBPath
	}
	if envDBDriver := os.Getenv("FORGEC2_DB_DRIVER"); envDBDriver != "" {
		cfg.Database.Driver = envDBDriver
	}
	if envDBDSN := os.Getenv("FORGEC2_DB_DSN"); envDBDSN != "" {
		cfg.Database.DSN = envDBDSN
	}
	if envHost := os.Getenv("FORGEC2_HOST"); envHost != "" {
		cfg.Server.Host = envHost
	}
	if envPort := os.Getenv("FORGEC2_PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil && p > 0 && p <= 65535 {
			cfg.Server.Port = p
		}
	}
	if envSlackToken := os.Getenv("FORGEC2_SLACK_BOT_TOKEN"); envSlackToken != "" {
		cfg.Integrations.Slack.BotToken = envSlackToken
	}

	return cfg, nil
}

// Endpoint returns the API endpoint for the configured provider
func (c *Config) AIEndpoint() string {
	if c.AI.Endpoint != "" {
		return c.AI.Endpoint
	}
	switch c.AI.Provider {
	case "openai":
		return "https://api.openai.com/v1"
	case "deepseek":
		return "https://api.deepseek.com/v1"
	case "qianwen":
		return "https://dashscope.aliyuncs.com/compatible-mode/v1"
	case "claude":
		return "https://api.anthropic.com/v1"
	case "longcat":
		return "https://api.longcat.chat/openai"
	case "custom":
		return "https://api.openai.com/v1"
	default:
		return "https://api.deepseek.com/v1"
	}
}

// Validate checks configuration for invalid or dangerous values.
func (c *Config) Validate() error {
	var errs []error

	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		errs = append(errs, errors.New("server.port must be between 1 and 65535"))
	}
	if c.Server.OfflineThreshold < 1 {
		errs = append(errs, errors.New("server.offline_threshold must be >= 1 second"))
	}
	if c.Server.OfflineThreshold > 86400 {
		errs = append(errs, errors.New("server.offline_threshold must be <= 86400 (24h)"))
	}
	if c.Server.SessionMaxAgeHours < 1 {
		errs = append(errs, errors.New("server.session_max_age_hours must be >= 1"))
	}
	if c.Server.SessionMaxAgeHours > 8760 {
		errs = append(errs, errors.New("server.session_max_age_hours must be <= 8760 (1 year)"))
	}
	if c.Server.CleanupRetentionDays < 0 {
		errs = append(errs, errors.New("server.cleanup_retention_days must be >= 0"))
	}
	if c.Server.CleanupRetentionDays > 3650 {
		errs = append(errs, errors.New("server.cleanup_retention_days must be <= 3650 (10 years)"))
	}
	if c.Implant.DefaultInterval < 1 {
		errs = append(errs, errors.New("implant.default_interval must be >= 1 second"))
	}
	if c.Implant.DefaultInterval > 86400 {
		errs = append(errs, errors.New("implant.default_interval must be <= 86400 (24h)"))
	}
	if c.Implant.DefaultJitter < 0 || c.Implant.DefaultJitter > 100 {
		errs = append(errs, errors.New("implant.default_jitter must be between 0 and 100"))
	}
	if c.Implant.MinInterval < 0 {
		errs = append(errs, errors.New("implant.min_interval must be >= 0 (0 = no minimum)"))
	}
	if c.Implant.MinInterval > 86400 {
		errs = append(errs, errors.New("implant.min_interval must be <= 86400 (24h)"))
	}
	if c.Implant.MinJitter < 0 || c.Implant.MinJitter > 100 {
		errs = append(errs, errors.New("implant.min_jitter must be between 0 and 100"))
	}
	if c.Implant.DefaultInterval < c.Implant.MinInterval {
		errs = append(errs, fmt.Errorf("implant.default_interval (%d) must be >= implant.min_interval (%d)", c.Implant.DefaultInterval, c.Implant.MinInterval))
	}
	if c.Implant.DefaultJitter < c.Implant.MinJitter {
		errs = append(errs, fmt.Errorf("implant.default_jitter (%d) must be >= implant.min_jitter (%d)", c.Implant.DefaultJitter, c.Implant.MinJitter))
	}
	if c.Logging.Level != "" && c.Logging.Level != "debug" && c.Logging.Level != "info" && c.Logging.Level != "warn" && c.Logging.Level != "error" {
		errs = append(errs, errors.New(`logging.level must be one of: debug, info, warn, error`))
	}
	if c.RateLimit.Login.MaxAttempts < 1 {
		errs = append(errs, errors.New("rate_limit.login.max_attempts must be >= 1"))
	}
	if c.RateLimit.Login.Window < 1 {
		errs = append(errs, errors.New("rate_limit.login.window must be >= 1 second"))
	}
	if c.RateLimit.Login.LockoutTime < 1 {
		errs = append(errs, errors.New("rate_limit.login.lockout_time must be >= 1 second"))
	}
	if c.RateLimit.API.Rate < 0 {
		errs = append(errs, errors.New("rate_limit.api.rate must be >= 0"))
	}
	if c.RateLimit.Beacon.Limit < 1 {
		errs = append(errs, errors.New("rate_limit.beacon.limit must be >= 1"))
	}
	if c.RateLimit.Beacon.Window < 1 {
		errs = append(errs, errors.New("rate_limit.beacon.window must be >= 1 second"))
	}
	if c.Server.TCPEnabled && c.Server.TCPAddr == "" {
		errs = append(errs, errors.New("server.tcp_addr is required when tcp_enabled is true"))
	}
	if c.Server.SMBEnabled && c.Server.SMBPipe == "" {
		errs = append(errs, errors.New("server.smb_pipe is required when smb_enabled is true"))
	}
	if c.Server.DNSEnabled && c.Server.DNSDomain == "" {
		errs = append(errs, errors.New("server.dns_domain is required when dns_enabled is true"))
	}
	if c.Server.ICMPEnabled && c.Server.ICMPAddr == "" {
		errs = append(errs, errors.New("server.icmp_addr is required when icmp_enabled is true"))
	}
	if c.Server.GRPCEnabled && c.Server.GRPCAddr == "" {
		errs = append(errs, errors.New("server.grpc_addr is required when grpc_enabled is true"))
	}
	if c.Server.SSHEnabled && c.Server.SSHPort <= 0 {
		errs = append(errs, errors.New("server.ssh_port must be > 0 when ssh_enabled is true"))
	}

	// AI validation
	if c.AI.Enabled && c.AI.APIKey == "" && os.Getenv("FORGEC2_AI_API_KEY") == "" {
		slog.Warn("ai.api_key is empty — AI features will fail at runtime unless set via env FORGEC2_AI_API_KEY")
	}
	if c.AI.MaxConversationTurns < 0 {
		errs = append(errs, errors.New("ai.max_conversation_turns must be >= 0 (0 = unlimited)"))
	}
	if c.AI.MaxToolRounds < 0 {
		errs = append(errs, errors.New("ai.max_tool_rounds must be >= 0 (0 = unlimited)"))
	}
	if c.AI.MaxDuplicateToolCalls < 0 {
		errs = append(errs, errors.New("ai.max_duplicate_tool_calls must be >= 0 (0 = unlimited)"))
	}
	if c.AI.Enabled && c.AI.Provider != "" {
		validProviders := map[string]bool{"openai": true, "anthropic": true, "claude": true, "google": true, "deepseek": true, "qianwen": true, "longcat": true, "local": true, "custom": true}
		if !validProviders[c.AI.Provider] {
			errs = append(errs, errors.New("ai.provider must be one of: openai, anthropic, claude, google, deepseek, qianwen, longcat, local, custom"))
		}
	}

	// TLS validation. cert_file/key_file paths are required when TLS is on,
	// but the files themselves may be absent: the server auto-generates a
	// self-signed certificate at startup (Run -> GenerateSelfSignedCert).
	if c.Server.TLSEnabled {
		if c.Server.CertFile == "" {
			errs = append(errs, errors.New("server.cert_file is required when server.tls_enabled is true"))
		}
		if c.Server.KeyFile == "" {
			errs = append(errs, errors.New("server.key_file is required when server.tls_enabled is true"))
		}
	}
	if c.Server.RequireTLSForAuth && !c.Server.TLSEnabled {
		slog.Warn("server.require_tls_for_auth is enabled but server.tls_enabled is false — session cookies will NOT be secure over plain HTTP")
	}

	// Rate limit validation
	if c.RateLimit.API.Capacity <= 0 {
		errs = append(errs, errors.New("rate_limit.api.capacity must be > 0"))
	}
	if c.RateLimit.ExtC2.Rate < 0 {
		errs = append(errs, errors.New("rate_limit.extc2.rate must be >= 0"))
	}
	if c.RateLimit.ExtC2.Burst < 1 {
		errs = append(errs, errors.New("rate_limit.extc2.burst must be >= 1 (burst=0 blocks all requests)"))
	}
	if c.RateLimit.ExtC2.CleanupAge < 0 {
		errs = append(errs, errors.New("rate_limit.extc2.cleanup_age must be >= 0"))
	}
	if c.RateLimit.ExtC2.APIToken == "" {
		slog.Warn("rate_limit.extc2.api_token is empty — a random token will be auto-generated on startup")
	}
	if c.Malleable.Enabled && c.Malleable.StatusCode != 0 && (c.Malleable.StatusCode < 100 || c.Malleable.StatusCode > 599) {
		errs = append(errs, errors.New("malleable.status_code must be between 100 and 599 or 0 for default"))
	}
	if c.PasswordPolicy.MinLength < 4 {
		errs = append(errs, errors.New("password_policy.min_length must be >= 4"))
	}
	if c.PasswordPolicy.MinLength > 128 {
		errs = append(errs, errors.New("password_policy.min_length must be <= 128"))
	}
	if c.PasswordPolicy.MaxAge < 0 {
		errs = append(errs, errors.New("password_policy.max_age_days must be >= 0"))
	}
	if c.PasswordPolicy.BcryptCost < 0 {
		errs = append(errs, errors.New("password_policy.bcrypt_cost must be >= 0 (0 = default 10)"))
	}
	if c.PasswordPolicy.BcryptCost > 0 && (c.PasswordPolicy.BcryptCost < 4 || c.PasswordPolicy.BcryptCost > 31) {
		errs = append(errs, errors.New("password_policy.bcrypt_cost must be between 4 and 31 when non-zero"))
	}
	if c.Auth.DefaultPasswd != "" && isWeakDefaultPassword(c.Auth.DefaultPasswd) {
		errs = append(errs, errors.New("auth.default_password is too weak — set a strong password or leave it empty to auto-generate a random one on first boot"))
	}
	if c.Socks.Enabled {
		for _, dest := range c.Socks.AllowedDests {
			if !strings.Contains(dest, ":") {
				errs = append(errs, fmt.Errorf("socks.allowed_destinations entry %q must be in host:port format", dest))
			}
		}
	}

if c.Listeners.H2C.Enabled && c.Listeners.H2C.Addr == "" {
		errs = append(errs, errors.New("listeners.h2c.addr is required when listeners.h2c.enabled is true"))
	}

	// Crypto validation — the v2/v3 beacon stack implements ONLY the ECDH
	// chain (HKDF+X25519+AES-256-GCM+MAC+replay window). A legacy XOR key or
	// a disabled (empty) mode cannot authenticate any built implant:
	// registration requires the per-implant v3 secret which only the ECDH
	// session manager issues, and plaintext frames are rejected by the
	// protocol. Refuse to boot rather than run silently-broken beacons.
	if c.Crypto.Key == "" || !strings.HasPrefix(c.Crypto.Key, "ecdh:") {
		errs = append(errs, errors.New("crypto.key must be \"ecdh:\" (ECDH beacon encryption) — the v2/v3 beacon protocol does not implement the legacy XOR/plaintext modes, so any other value leaves every built implant unable to authenticate"))
	}
	if c.Crypto.MaxDecryptedPayloadSize < 0 {
		errs = append(errs, errors.New("crypto.max_decrypted_payload_size must be >= 0 (0 = default 10MB)"))
	}
	if c.Crypto.MaxDecryptedPayloadSize > 104857600 {
		errs = append(errs, errors.New("crypto.max_decrypted_payload_size must be <= 104857600 (100MB)"))
	}
	// Crypto validation — storage keys are REQUIRED and must be 32-byte hex.
	// The legacy SHA-256(jwt_secret) derivation cascade was removed (breaking
	// change): each key is independent so a JWT compromise cannot decrypt
	// credentials, backups, or bind CSRF tokens.
	cryptoKeys := []struct {
		name string
		val  string
	}{
		{"crypto.loot_key", c.Crypto.LootKey},    // loot credentials (FC2ENC:)
		{"crypto.extc2_key", c.Crypto.ExtC2Key},  // ExtC2 channel (FC2EXT:)
		{"crypto.backup_key", c.Crypto.BackupKey}, // .fbk backups
		{"crypto.totp_key", c.Crypto.TotpKey},     // TOTP secrets / SMTP / SSH redirector credentials
		{"crypto.csrf_key", c.Crypto.CsrfKey},     // CSRF token binding
	}
	for _, k := range cryptoKeys {
		if k.val == "" {
			errs = append(errs, fmt.Errorf("%s is required (auto-generated only on fresh installs; the legacy derivation from server.jwt_secret was removed - set a 64-character hex key in config.yaml or the corresponding FORGEC2_%s env var; note existing encrypted data created with the old derived key can no longer be decrypted)", k.name, envSuffixForKey(k.name)))
			continue
		}
		if len(k.val) != 64 {
			errs = append(errs, fmt.Errorf("%s must be a 64-character hex string (32 bytes)", k.name))
			continue
		}
		if _, err := hex.DecodeString(k.val); err != nil {
			errs = append(errs, fmt.Errorf("%s must be a valid hex string", k.name))
		}
	}

	return errors.Join(errs...)
}

// envSuffixForKey maps a config field path to the FORGEC2_* env var suffix.
func envSuffixForKey(field string) string {
	return strings.ToUpper(strings.SplitN(field, ".", 2)[1])
}

// Lock acquires the config mutex for safe concurrent reads/writes.
func (c *Config) Lock() {
	c.mu.Lock()
}

// Unlock releases the config mutex.
func (c *Config) Unlock() {
	c.mu.Unlock()
}

// Save persists the config (e.g. after setting password)
func (c *Config) Save(path string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	out, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0600)
}

// CopyFrom copies all exported fields from src into c with mutex protection
func (c *Config) CopyFrom(src *Config) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Server = src.Server
	c.Database = src.Database
	c.Implant = src.Implant
	c.Auth = src.Auth
	c.Crypto = src.Crypto
	c.Malleable = src.Malleable
	c.AI = src.AI
	c.Logging = src.Logging
	c.RateLimit = src.RateLimit
	c.Integrations = src.Integrations
	c.Listeners = src.Listeners
	c.Socks = src.Socks
	c.TLSFingerprint = src.TLSFingerprint
}

// SocksUser holds SOCKS5 username/password auth credentials.
type SocksUser struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// TLSFingerprintConfig holds JARM/JA3 fingerprint randomization settings
type TLSFingerprintConfig struct {
	JARMEnabled bool   `yaml:"jarm_randomize"`
	JARMRotate  string `yaml:"jarm_rotate_interval"`
	JA3Enabled  bool   `yaml:"ja3_randomize"`
	JA3Profile  string `yaml:"ja3_profile"`
	JA3Rotate   string `yaml:"ja3_rotate_interval"`
}

// SlackConfig holds Slack integration settings
type SlackConfig struct {
	Enabled       bool   `yaml:"enabled"`
	AppToken      string `yaml:"app_token"`
	BotToken      string `yaml:"bot_token"`
	SigningSecret string `yaml:"signing_secret"`
}

// ListenersConfig holds transport listener configurations.
type ListenersConfig struct {
	H2C H2CListenerConfig `yaml:"h2c"`
}

// H2CListenerConfig configures the H2C (HTTP/2 cleartext) listener.
type H2CListenerConfig struct {
	Enabled bool   `yaml:"enabled"`
	Addr    string `yaml:"addr"`
}

// LoadFromData loads config from byte data
func (c *Config) LoadFromData(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return yaml.Unmarshal(data, c)
}

// AllowedOrigin checks if the given origin hostname is in the configured allow list.
// Returns false (deny) when no origins are configured.
func (c *Config) AllowedOrigin(hostname string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.Server.AllowedOrigins) == 0 {
		return false
	}
	for _, allowed := range c.Server.AllowedOrigins {
		if hostname == allowed {
			return true
		}
	}
	return false
}
