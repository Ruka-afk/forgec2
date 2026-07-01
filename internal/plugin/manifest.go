package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Manifest describes a ForgeC2 plugin package.
type Manifest struct {
	Name        string                 `yaml:"name" json:"name"`
	Version     string                 `yaml:"version" json:"version"`
	Type        string                 `yaml:"type" json:"type"`
	Description string                 `yaml:"description" json:"description"`
	Author      string                 `yaml:"author" json:"author"`
	Category    string                 `yaml:"category" json:"category"`
	Entry       string                 `yaml:"entry" json:"entry"`                       // script path
	Interpreter string                 `yaml:"interpreter" json:"interpreter"`           // python, powershell, bash, go
	Timeout     int                    `yaml:"timeout" json:"timeout"`                   // seconds
	Params      []ManifestParam        `yaml:"params" json:"params"`                     // command/report params
	Events      []string               `yaml:"events,omitempty" json:"events,omitempty"` // for hooks
	Config      map[string]interface{} `yaml:"config,omitempty" json:"config,omitempty"` // default config
}

// ManifestParam defines a user-configurable parameter for a plugin.
type ManifestParam struct {
	Name        string `yaml:"name" json:"name"`
	Type        string `yaml:"type" json:"type"`
	Required    bool   `yaml:"required" json:"required"`
	Default     string `yaml:"default,omitempty" json:"default,omitempty"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

// ValidPluginTypes lists the plugin types supported by ForgeC2.
var ValidPluginTypes = []string{"command", "hook", "report"}

// ValidInterpreters lists the interpreters the executor can invoke.
var ValidInterpreters = []string{"python", "python3", "powershell", "pwsh", "bash", "sh", "go"}

// LoadManifest reads and parses a manifest.yaml file.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest %s: %w", path, err)
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("failed to parse manifest %s: %w", path, err)
	}
	return &m, nil
}

// Validate checks that the manifest contains the required fields and valid values.
func (m *Manifest) Validate() error {
	if strings.TrimSpace(m.Name) == "" {
		return fmt.Errorf("plugin name is required")
	}
	if strings.TrimSpace(m.Version) == "" {
		return fmt.Errorf("plugin version is required")
	}
	if !isValidValue(m.Type, ValidPluginTypes) {
		return fmt.Errorf("invalid plugin type %q, must be one of %v", m.Type, ValidPluginTypes)
	}
	if strings.TrimSpace(m.Entry) == "" {
		return fmt.Errorf("plugin entry is required")
	}
	if !isValidValue(m.Interpreter, ValidInterpreters) {
		return fmt.Errorf("invalid interpreter %q, must be one of %v", m.Interpreter, ValidInterpreters)
	}
	if m.Timeout < 0 {
		return fmt.Errorf("plugin timeout cannot be negative")
	}
	return nil
}

// Save writes the manifest to a YAML file inside the given directory.
func (m *Manifest) Save(dir string) error {
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create plugin directory %s: %w", dir, err)
	}
	out, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}
	path := filepath.Join(dir, "manifest.yaml")
	return os.WriteFile(path, out, 0644)
}

// DefaultTimeout returns the configured timeout or a sensible default.
func (m *Manifest) DefaultTimeout() int {
	if m.Timeout > 0 {
		return m.Timeout
	}
	return 60
}

// ConfigJSON returns the default config as a JSON string for DB storage.
func (m *Manifest) ConfigJSON() string {
	if len(m.Config) == 0 {
		return ""
	}
	b, _ := json.Marshal(m.Config)
	return string(b)
}

func isValidValue(value string, allowed []string) bool {
	for _, a := range allowed {
		if value == a {
			return true
		}
	}
	return false
}
