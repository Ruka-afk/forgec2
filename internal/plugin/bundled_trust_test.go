package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/testutil"
	"gopkg.in/yaml.v3"
)

// bundledTrustedKey reads the project signing key that ships in
// config.example.yaml. Reading it from the example (rather than hardcoding it
// here) keeps the single source of truth honest: if the key changes, this
// test fails until the bundled plugins are re-signed.
func bundledTrustedKey(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "config.example.yaml"))
	if err != nil {
		t.Fatalf("read config.example.yaml: %v", err)
	}
	var doc struct {
		Plugins struct {
			TrustedKeys   []string `yaml:"trusted_keys"`
			RequireSigned bool     `yaml:"require_signed"`
		} `yaml:"plugins"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse config.example.yaml: %v", err)
	}
	if len(doc.Plugins.TrustedKeys) == 0 {
		t.Fatal("config.example.yaml must ship the bundled plugin signing key")
	}
	if !doc.Plugins.RequireSigned {
		t.Fatal("config.example.yaml should require signed plugins now that the bundled set is signed")
	}
	return doc.Plugins.TrustedKeys
}

// TestBundledPluginsVerifyUnderStrictPolicy loads the real bundled plugin tree
// with the shipped trusted key and require_signed enabled. It is the regression
// test for the whole trust chain: any edit to a plugin script or to the shared
// lib/ without re-signing makes the server refuse to load it.
func TestBundledPluginsVerifyUnderStrictPolicy(t *testing.T) {
	keys, err := ParseTrustedPluginKeys(bundledTrustedKey(t))
	if err != nil {
		t.Fatalf("bundled key rejected: %v", err)
	}
	policy := VerificationPolicy(keys, true)

	root := filepath.Join("..", "..", "plugins")
	manifests, err := filepath.Glob(filepath.Join(root, "*", "*", "manifest.yaml"))
	if err != nil {
		t.Fatalf("glob manifests: %v", err)
	}
	if len(manifests) < 50 {
		t.Fatalf("expected the full bundled plugin set, found %d", len(manifests))
	}

	for _, manifestPath := range manifests {
		dir := filepath.Dir(manifestPath)
		manifest, err := LoadManifest(manifestPath)
		if err != nil {
			t.Fatalf("load %s: %v", manifestPath, err)
		}
		if err := manifest.Validate(); err != nil {
			t.Errorf("%s: manifest invalid: %v", manifestPath, err)
			continue
		}
		if manifest.Digest == "" || manifest.Signature == "" {
			t.Errorf("%s: bundled plugin is not signed (run cmd/sign-plugin)", manifestPath)
			continue
		}
		status, err := VerifyPackageWithPolicy(dir, root, manifest, policy)
		if err != nil {
			t.Errorf("%s: %v", manifestPath, err)
			continue
		}
		if !status.Verified {
			t.Errorf("%s: not verified: %s", manifestPath, status.Detail)
		}
	}
}

// TestBundledPluginTreeLoadsWithTrust proves the manager registers the whole
// signed tree under the strict policy (registration is where verification is
// enforced).
func TestBundledPluginTreeLoadsWithTrust(t *testing.T) {
	keys, err := ParseTrustedPluginKeys(bundledTrustedKey(t))
	if err != nil {
		t.Fatalf("bundled key rejected: %v", err)
	}
	m := NewManager(testutil.SetupTestDB(t))
	m.SetTrust(keys, true)
	if err := m.LoadFromDisk(filepath.Join("..", "..", "plugins")); err != nil {
		t.Fatalf("LoadFromDisk: %v", err)
	}
	names := m.List()
	if len(names) < 50 {
		t.Fatalf("expected the bundled plugins to register, got %d", len(names))
	}
	for _, p := range names {
		if strings.TrimSpace(p.Name()) == "" {
			t.Fatal("empty plugin name registered")
		}
	}
}
