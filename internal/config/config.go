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
	mu sync.Mutex `yaml:"-"`
	Server struct {
		Port        int    `yaml:"port"`
		Host        string `yaml:"host"`
		TLSEnabled  bool   `yaml:"tls_enabled"`
		CertFile    string `yaml:"cert_file"`
		KeyFile     string `yaml:"key_file"`
		JWTSecret   string `yaml:"jwt_secret"`
	} `yaml:"server"`

	Database struct {
		Path string `yaml:"path"`
	} `yaml:"database"`

	Agent struct {
		DefaultInterval int    `yaml:"default_interval"` // seconds
		DefaultJitter   int    `yaml:"default_jitter"`   // percent
		DefaultUA       string `yaml:"default_user_agent"`
	} `yaml:"agent"`

	Auth struct {
		PasswordHash string `yaml:"password_hash"` // bcrypt hash, set on first run
	} `yaml:"auth"`

	Logging struct {
		Level string `yaml:"level"` // debug, info, warn, error
	} `yaml:"logging"`
}

// DefaultConfig returns sensible defaults
func DefaultConfig() *Config {
	cfg := &Config{}
	cfg.Server.Port = 8080
	cfg.Server.Host = "0.0.0.0"
	cfg.Server.TLSEnabled = false
	cfg.Server.CertFile = "data/server.crt"
	cfg.Server.KeyFile = "data/server.key"

	cfg.Database.Path = "data/db/forgec2.db"

	cfg.Agent.DefaultInterval = 10
	cfg.Agent.DefaultJitter = 20
	cfg.Agent.DefaultUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36"

	cfg.Logging.Level = "info"
	return cfg
}

// Load loads config from file, creates default if not exists
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// create default config file
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
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
