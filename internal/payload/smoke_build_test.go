package payload

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Smoke test: generate a real Windows agent with the config-blob ldflags to
// ensure the full build pipeline (go mod tidy + go build + blob injection)
// produces a working binary.
func TestSmokeGenerateWindowsEXE(t *testing.T) {
	if os.Getenv("FORGEC2_SMOKE_BUILD") != "1" {
		t.Skip("set FORGEC2_SMOKE_BUILD=1 to run the real agent build")
	}
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
		Architecture:    "amd64",
	}
	path, err := GenerateWindowsEXE(cfg, outDir)
	if err != nil {
		t.Fatalf("GenerateWindowsEXE failed: %v", err)
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
}

func fileSize(p string) int64 {
	fi, _ := os.Stat(p)
	if fi == nil {
		return 0
	}
	return fi.Size()
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
