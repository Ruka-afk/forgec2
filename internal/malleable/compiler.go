package malleable

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

type MalleableProfile struct {
	Name      string      `yaml:"name"`
	UserAgent string      `yaml:"user_agent"`
	Sleep     int         `yaml:"sleep"`
	Jitter    int         `yaml:"jitter"`
	HTTPGet   HTTPBlock   `yaml:"http-get"`
	HTTPPost  HTTPBlock   `yaml:"http-post"`
	PostEx    PostExBlock `yaml:"post-ex"`
}

type HTTPBlock struct {
	URI    string      `yaml:"uri"`
	Verb   string      `yaml:"verb"`
	Client ClientBlock `yaml:"client"`
	Server ServerBlock `yaml:"server"`
}

type ClientBlock struct {
	Metadata []TransformStep `yaml:"metadata"`
	ID       []TransformStep `yaml:"id"`
	Output   []TransformStep `yaml:"output"`
}

type ServerBlock struct {
	Output []TransformStep `yaml:"output"`
}

type TransformStep struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value,omitempty"`
}

type PostExBlock struct {
	PipeName string `yaml:"pipename"`
	Key      string `yaml:"key"`
}

func Compile(data []byte) (*MalleableProfile, error) {
	var p MalleableProfile
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("malleable compile: %w", err)
	}
	if p.Name == "" {
		return nil, fmt.Errorf("malleable compile: profile name is required")
	}
	if p.HTTPGet.URI == "" {
		p.HTTPGet.URI = "/api/v1/beacon"
	}
	if p.HTTPGet.Verb == "" {
		p.HTTPGet.Verb = "GET"
	}
	if p.HTTPPost.URI == "" {
		p.HTTPPost.URI = "/api/v1/beacon"
	}
	if p.HTTPPost.Verb == "" {
		p.HTTPPost.Verb = "POST"
	}
	if p.UserAgent == "" {
		p.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
	}
	if p.Sleep < 1 {
		p.Sleep = 10
	}
	return &p, nil
}

func CompileString(yamlContent string) (*MalleableProfile, error) {
	return Compile([]byte(yamlContent))
}

func CompilePreset(name string) (*MalleableProfile, error) {
	yamlContent, ok := Presets[name]
	if !ok {
		return nil, fmt.Errorf("malleable: unknown preset %q", name)
	}
	return CompileString(yamlContent)
}
