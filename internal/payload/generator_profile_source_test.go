package payload

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/malleable"
)

const testCSProfile = `
set sleeptime "60000";
set jitter "15";
set useragent "Mozilla/5.0 ForgeTest";

http-post {
  set uri "/submit";
  set verb "POST";
  set parameter "id";
  client {
    header "Accept" "*/*";
    id {
      parameter "id";
      base64;
    }
  }
  server {
    output { base64; }
  }
}
`

func TestSaveCSProfilePersistsSourceAndCompiledJSON(t *testing.T) {
	dir := t.TempDir()
	p, err := SaveCSProfile(dir, "test_profile", testCSProfile)
	if err != nil {
		t.Fatalf("SaveCSProfile: %v", err)
	}
	if p.Name != "test_profile" || p.Method != "POST" || p.Sleep != 60 || p.Jitter != 15 {
		t.Fatalf("compiled profile = %+v", p)
	}
	jsonPath := filepath.Join(dir, "profiles", "test_profile.json")
	sourcePath := filepath.Join(dir, "profiles", "test_profile.profile")
	if _, err := os.Stat(jsonPath); err != nil {
		t.Fatalf("compiled JSON missing: %v", err)
	}
	src, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("source sidecar missing: %v", err)
	}
	if string(src) != testCSProfile {
		t.Fatalf("source sidecar changed:\n%s", src)
	}
	loaded, err := ProfileSource(dir, "test_profile")
	if err != nil {
		t.Fatalf("ProfileSource: %v", err)
	}
	if loaded != testCSProfile {
		t.Fatalf("ProfileSource returned different text")
	}
	found := false
	for _, preset := range ListProfilePresets(dir) {
		if preset.Name == "test_profile" {
			found = true
			if preset.BeaconURI != "/submit" {
				t.Fatalf("preset = %+v", preset)
			}
		}
	}
	if !found {
		t.Fatal("custom profile not listed")
	}
}

func TestSaveCSProfileRejectsBuiltinAndInvalidNames(t *testing.T) {
	dir := t.TempDir()
	if _, err := SaveCSProfile(dir, "google", testCSProfile); err == nil || !strings.Contains(err.Error(), "built-in") {
		t.Fatalf("built-in overwrite error = %v", err)
	}
	if _, err := SaveCSProfile(dir, "../escape", testCSProfile); err == nil {
		t.Fatal("path-like profile name should be rejected")
	}
	if _, err := SaveCSProfile(dir, "bad", "http-get { set uri \"/x\"; "); err == nil {
		t.Fatal("unclosed block should be rejected")
	}
}

func TestDeleteProfileRemovesSource(t *testing.T) {
	dir := t.TempDir()
	if _, err := SaveCSProfile(dir, "delete_me", testCSProfile); err != nil {
		t.Fatalf("SaveCSProfile: %v", err)
	}
	if err := DeleteProfile(dir, "delete_me"); err != nil {
		t.Fatalf("DeleteProfile: %v", err)
	}
	for _, path := range []string{
		filepath.Join(dir, "profiles", "delete_me.json"),
		filepath.Join(dir, "profiles", "delete_me.profile"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("%s still exists (err=%v)", path, err)
		}
	}
	if err := DeleteProfile(dir, "delete_me"); err == nil {
		t.Fatal("second delete should fail")
	}
	if err := DeleteProfile(dir, "default"); err == nil {
		t.Fatal("deleting a built-in should fail")
	}
}

func TestSaveImportedV2JSONPreservesChains(t *testing.T) {
	dir := t.TempDir()
	raw := `{
  "name": "v2_import",
  "beacon_uris": ["/one", "/two"],
  "method": "GET",
  "sleep": 9,
  "jitter": 4,
  "client_metadata": [{"type": "base64"}],
  "server_output": [{"type": "prepend", "value": "PRE"}]
}`
	p, err := SaveImportedProfileNamed(dir, "", []byte(raw))
	if err != nil {
		t.Fatalf("SaveImportedProfileNamed: %v", err)
	}
	if p.Name != "v2_import" || p.Method != "GET" || p.BeaconURI != "/one" {
		t.Fatalf("v2 import changed target: %+v", p)
	}
	if p.ClientMetadata != "base64" || p.ServerOutput != "prepend:PRE" {
		t.Fatalf("v2 import lost chains: metadata=%q output=%q", p.ClientMetadata, p.ServerOutput)
	}
}

func TestProfileSourceRendersLegacyJSON(t *testing.T) {
	dir := t.TempDir()
	jsonProfile := `{
  "name": "legacy",
  "user_agent": "LegacyUA",
  "beacon_uri": "/legacy",
  "method": "POST",
  "sleep": 12,
  "jitter": 7,
  "headers": {"Accept": "*/*"}
}`
	if _, err := SaveImportedProfileNamed(dir, "", []byte(jsonProfile)); err != nil {
		t.Fatalf("SaveImportedProfileNamed: %v", err)
	}
	src, err := ProfileSource(dir, "legacy")
	if err != nil {
		t.Fatalf("ProfileSource: %v", err)
	}
	if !strings.Contains(src, `set useragent "LegacyUA";`) || !strings.Contains(src, `set uri "/legacy";`) {
		t.Fatalf("rendered legacy source missing fields:\n%s", src)
	}
}

func TestRenderedBuiltinsCompile(t *testing.T) {
	for _, p := range ListProfilePresets(t.TempDir()) {
		if !IsBuiltinProfile(p.Name) {
			continue
		}
		src := RenderProfileCS(p)
		res := malleable.ParseCSProfile(p.Name, src)
		if res.HasErrors() {
			t.Fatalf("rendered built-in %q has errors: %v\n%s", p.Name, res.ErrorMessages(), src)
		}
		if res.Profile.Method != p.Method || res.Profile.PrimaryURI() != p.BeaconURI {
			t.Fatalf("rendered built-in %q changed target: method=%q uri=%q, want %q/%q\n%s",
				p.Name, res.Profile.Method, res.Profile.PrimaryURI(), p.Method, p.BeaconURI, src)
		}
	}
}
