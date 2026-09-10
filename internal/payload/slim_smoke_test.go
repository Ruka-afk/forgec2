package payload

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSmokeSlimWindowsEXE builds a real light-profile Windows agent
// end-to-end (extract + tidy + compile) and asserts the artifact exists.
// Gated behind FORGEC2_SMOKE_BUILD like the other heavy build smokes.
func TestSmokeSlimWindowsEXE(t *testing.T) {
	if os.Getenv("FORGEC2_SMOKE_BUILD") == "" {
		t.Skip("set FORGEC2_SMOKE_BUILD=1 to run real toolchain builds")
	}
	outDir := t.TempDir()
	cfg := ImplantConfig{
		C2URL:           "http://127.0.0.1:8080",
		Protocol:        "http",
		BeaconTransport: "http",
		Interval:        10,
		Jitter:          10,
		Filename:        "slim_agent.exe",
		Architecture:    "amd64",
		PETimestampMode: "zero",
		PESectionMode:   "default",
		PEImportMode:    "none",
		Slim:            true,
	}
	out, err := GenerateWindowsEXE(cfg, outDir)
	if err != nil {
		t.Fatalf("GenerateWindowsEXE slim: %v", err)
	}
	fi, err := os.Stat(out)
	if err != nil {
		t.Fatalf("slim artifact missing: %v", err)
	}
	t.Logf("slim exe: %d bytes (%s)", fi.Size(), filepath.Base(out))
	if fi.Size() == 0 {
		t.Fatal("slim artifact is empty")
	}
}

// TestSmokeWin7WindowsEXE builds a legacy-toolchain Windows agent for
// Windows 7 / Server 2008 R2 targets. It downloads the go1.20.14 toolchain
// (~120MB) and repinned dependencies on first use, so it needs network and
// is gated behind FORGEC2_SMOKE_WIN7 (never CI). Validate the artifact on a
// real Win7/2008R2 host before relying on it.
func TestSmokeWin7WindowsEXE(t *testing.T) {
	if os.Getenv("FORGEC2_SMOKE_WIN7") == "" {
		t.Skip("set FORGEC2_SMOKE_WIN7=1 to run the legacy-toolchain build (needs network)")
	}
	outDir := t.TempDir()
	cfg := ImplantConfig{
		C2URL:           "http://127.0.0.1:8080",
		Protocol:        "http",
		BeaconTransport: "http",
		Interval:        10,
		Jitter:          10,
		Filename:        "win7_agent.exe",
		Architecture:    "amd64",
		PETimestampMode: "zero",
		PESectionMode:   "default",
		PEImportMode:    "none",
		Win7Compat:      true,
	}
	out, err := GenerateWindowsEXE(cfg, outDir)
	if err != nil {
		t.Fatalf("GenerateWindowsEXE win7: %v", err)
	}
	fi, err := os.Stat(out)
	if err != nil {
		t.Fatalf("win7 artifact missing: %v", err)
	}
	t.Logf("win7 exe: %d bytes (%s)", fi.Size(), filepath.Base(out))
	if fi.Size() == 0 {
		t.Fatal("win7 artifact is empty")
	}
}
