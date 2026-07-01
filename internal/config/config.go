package config

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration for ForgeC2
type Config struct {
	mu     sync.Mutex `yaml:"-"`
	Server struct {
		Port                  int    `yaml:"port"`
		Host                  string `yaml:"host"`
		TLSEnabled            bool   `yaml:"tls_enabled"`
		CertFile              string `yaml:"cert_file"`
		KeyFile               string `yaml:"key_file"`
		JWTSecret             string `yaml:"jwt_secret"`
		TCPEnabled            bool   `yaml:"tcp_enabled"`
		TCPAddr              string `yaml:"tcp_addr"`
		SMBEnabled           bool   `yaml:"smb_enabled"`
		SMBPipe              string `yaml:"smb_pipe"`
		DataDir              string `yaml:"data_dir"`
		DNSEnabled           bool   `yaml:"dns_enabled"`
		DNSDomain            string `yaml:"dns_domain"`
		ICMPEnabled          bool   `yaml:"icmp_enabled"`
		ICMPAddr             string `yaml:"icmp_addr"`
		OfflineThreshold     int    `yaml:"offline_threshold"`      // seconds
		SessionMaxAgeHours   int    `yaml:"session_max_age_hours"`  // JWT expiry
		CleanupRetentionDays int    `yaml:"cleanup_retention_days"` // auto-purge cutoff
		UpdateCheckRepo      string `yaml:"update_check_repo"`      // GitHub repo for update checks (e.g. "owner/repo")
	} `yaml:"server"`

	Database struct {
		Path string `yaml:"path"`
	} `yaml:"database"`

	Implant struct {
		DefaultInterval int    `yaml:"default_interval"` // seconds
		DefaultJitter   int    `yaml:"default_jitter"`   // percent
		DefaultUA       string `yaml:"default_user_agent"`
		DefaultSkipTLS  bool   `yaml:"default_skip_tls"`
	} `yaml:"implant"`

	Auth struct {
		PasswordHash string `yaml:"password_hash"` // bcrypt hash, set on first run
	} `yaml:"auth"`

	Crypto struct {
		Key string `yaml:"key"` // 32-byte hex key for beacon payload encryption (empty=disabled)
	} `yaml:"crypto"`

	Malleable struct {
		Enabled     bool              `yaml:"enabled"`
		ProfileName string            `yaml:"profile_name"` // preset name: default, microsoft, google_analytics, cloudflare_cdn, akamai
		StatusCode  int               `yaml:"status_code"`
		ContentType string            `yaml:"content_type"`
		Headers     map[string]string `yaml:"headers"`
		Prepend     string            `yaml:"prepend"`
		Append      string            `yaml:"append"`
	} `yaml:"malleable"`

	AI struct {
		Enabled      bool   `yaml:"enabled"`
		Provider     string `yaml:"provider"`      // deepseek, openai, claude, qianwen, custom
		APIKey       string `yaml:"api_key"`
		Model        string `yaml:"model"`
		Endpoint     string `yaml:"endpoint"`       // optional, override default
		SystemPrompt string `yaml:"system_prompt"`  // custom system prompt
	} `yaml:"ai"`

	Logging struct {
		Level string `yaml:"level"` // debug, info, warn, error
	} `yaml:"logging"`

	RateLimit struct {
		Login struct {
			MaxAttempts int    `yaml:"max_attempts"` // max login attempts per window
			Window      int    `yaml:"window"`       // window in seconds
			LockoutTime int    `yaml:"lockout_time"` // lockout duration in seconds
			Whitelist   []string `yaml:"whitelist"`   // whitelisted IPs
		} `yaml:"login"`
		API struct {
			Capacity  float64  `yaml:"capacity"`   // token bucket capacity (max burst)
			Rate      float64  `yaml:"rate"`       // tokens per second per user
			Whitelist []string `yaml:"whitelist"`   // whitelisted IPs
		} `yaml:"api"`
		Beacon struct {
			Limit  int `yaml:"limit"`  // requests per window
			Window int `yaml:"window"` // window in seconds
		} `yaml:"beacon"`
	} `yaml:"rate_limit"`
}

// DefaultConfig returns sensible defaults
func DefaultConfig() *Config {
	cfg := &Config{}
	cfg.Server.Port = 8080
	cfg.Server.Host = "0.0.0.0"
	cfg.Server.TLSEnabled = false
	cfg.Server.TCPEnabled = false
	cfg.Server.TCPAddr = ""
	cfg.Server.SMBEnabled = false
	cfg.Server.SMBPipe = "forgec2"
	cfg.Server.DNSEnabled = false
	cfg.Server.DNSDomain = ""
	cfg.Server.ICMPEnabled = false
	cfg.Server.ICMPAddr = "0.0.0.0"
	cfg.Server.DataDir = "data"
	cfg.Server.OfflineThreshold = 60
	cfg.Server.SessionMaxAgeHours = 24
	cfg.Server.CleanupRetentionDays = 30
	cfg.Server.UpdateCheckRepo = "forgec2/forgec2"

	cfg.Database.Path = filepath.Join(cfg.Server.DataDir, "db/forgec2.db")
	cfg.Server.CertFile = filepath.Join(cfg.Server.DataDir, "server.crt")
	cfg.Server.KeyFile = filepath.Join(cfg.Server.DataDir, "server.key")

	cfg.Implant.DefaultInterval = 10
	cfg.Implant.DefaultJitter = 20
	cfg.Implant.DefaultUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36"

	cfg.Malleable.Enabled = false
	cfg.Malleable.StatusCode = 200
	cfg.Malleable.ContentType = "application/json"
	cfg.Malleable.Headers = map[string]string{
		"Server": "nginx/1.24.0",
	}
	cfg.AI.Enabled = false
	cfg.AI.Provider = "deepseek"
	cfg.AI.Model = "deepseek-chat"
	cfg.AI.SystemPrompt = "You are the ForgeC2 red team operations assistant, running on the C2 server. You can list online agents, view target details, execute commands, view credentials, manage listeners, and more."
	cfg.Logging.Level = "info"

	cfg.RateLimit.Login.MaxAttempts = 5
	cfg.RateLimit.Login.Window = 60
	cfg.RateLimit.Login.LockoutTime = 900
	cfg.RateLimit.Login.Whitelist = []string{}

	cfg.RateLimit.API.Capacity = 100
	cfg.RateLimit.API.Rate = 50
	cfg.RateLimit.API.Whitelist = []string{"127.0.0.1", "::1"}

	cfg.RateLimit.Beacon.Limit = 100
	cfg.RateLimit.Beacon.Window = 60

	return cfg
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
			out, _ := yaml.Marshal(cfg)
			if err := os.WriteFile(path, out, 0644); err != nil {
				return nil, err
			}
			return cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Generate random JWT secret if not set
	if cfg.Server.JWTSecret == "" {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		cfg.Server.JWTSecret = hex.EncodeToString(key)
		if err := cfg.Save(path); err != nil {
			return nil, err
		}
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
	case "custom":
		return "https://api.openai.com/v1" // fallback for custom
	default:
		return "https://api.deepseek.com/v1"
	}
}

// Save persists the config (e.g. after setting password)
func (c *Config) Save(path string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	out, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0644)
}

// LoadFromData loads config from byte data
func (c *Config) LoadFromData(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return yaml.Unmarshal(data, c)
}
