package payload

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

// The Wintun runtime staging is deliberately optional: generation must work
// with or without the fetched DLLs, and the build tag must only appear when
// the DLL for the requested windows architecture is actually present.

func TestWintunBuildTags(t *testing.T) {
	dir := t.TempDir()
	dll := filepath.Join(dir, "assets", "wintun", "amd64", "wintun.dll")
	if err := os.MkdirAll(filepath.Dir(dll), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dll, []byte("MZ"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		goos, goarch string
		want         bool
	}{
		{"windows", "amd64", true},
		{"windows", "arm64", false}, // only amd64 is staged in this fixture
		{"windows", "386", false},   // not a supported windows arch
		{"linux", "amd64", false},
		{"darwin", "arm64", false},
	} {
		got := len(wintunBuildTags(tc.goos, tc.goarch, dir)) > 0
		if got != tc.want {
			t.Fatalf("wintunBuildTags(%s/%s) tagged=%v, want %v", tc.goos, tc.goarch, got, tc.want)
		}
	}

	// No assets at all: no tag, no error.
	if tags := wintunBuildTags("windows", "amd64", t.TempDir()); tags != nil {
		t.Fatalf("wintunBuildTags without assets = %v, want nil", tags)
	}
}

func TestExtractAgentAssets(t *testing.T) {
	fsys := fstest.MapFS{
		"agent/assets/wintun/amd64/wintun.dll": {Data: []byte("MZ-amd64")},
		"agent/assets/wintun/LICENSE.txt":      {Data: []byte("license")},
	}
	dir := t.TempDir()
	if err := extractAgentAssets(fsys, dir); err != nil {
		t.Fatalf("extractAgentAssets: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "assets", "wintun", "amd64", "wintun.dll"))
	if err != nil {
		t.Fatalf("staged DLL missing: %v", err)
	}
	if string(got) != "MZ-amd64" {
		t.Fatalf("staged DLL content = %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "assets", "wintun", "LICENSE.txt")); err != nil {
		t.Fatalf("staged license missing: %v", err)
	}

	// A checkout without staged runtimes extracts nothing and errors nothing.
	empty := fstest.MapFS{"agent/agent.go": {Data: []byte("package main\n")}}
	if err := extractAgentAssets(empty, t.TempDir()); err != nil {
		t.Fatalf("extractAgentAssets without assets: %v", err)
	}
}

func TestExtractAgentSourcesStagesFetchedWintun(t *testing.T) {
	if _, err := fs.Stat(payloadFS, "agent/assets/wintun"); err != nil {
		t.Skip("wintun runtimes not staged in this checkout")
	}
	dir := t.TempDir()
	if err := extractAgentSources(payloadFS, dir, mustModulePlan(t, &ImplantConfig{})); err != nil {
		t.Fatalf("extractAgentSources: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "assets", "wintun", "amd64", "wintun.dll")); err != nil {
		t.Fatalf("staged DLL not copied into the build tree: %v", err)
	}
}
