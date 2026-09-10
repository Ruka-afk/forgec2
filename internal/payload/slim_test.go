package payload

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStripUTLSCreds(t *testing.T) {
	src, err := payloadFS.ReadFile("agent/transport_utls.go")
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	out, err := stripUTLSCreds(src)
	if err != nil {
		t.Fatalf("stripUTLSCreds: %v", err)
	}
	text := string(out)
	if strings.Contains(text, "google.golang.org/grpc") {
		t.Fatal("stripped utls still imports grpc")
	}
	if strings.Contains(text, "utlsCreds") {
		t.Fatal("stripped utls still contains utlsCreds")
	}
	// Shared dialers must survive the strip.
	for _, sym := range []string{"func utlsDialContext", "func dialUTLSTCP", "func newUTLSTransport", "func utlsClientHello"} {
		if !strings.Contains(text, sym) {
			t.Fatalf("stripped utls lost %s", sym)
		}
	}
	if _, err := stripUTLSCreds([]byte("package main\n")); err == nil {
		t.Fatal("expected error for source without marker/import")
	}
}

func TestSlimTransportConflict(t *testing.T) {
	slim := &ImplantConfig{Slim: true}
	for _, tr := range []string{"grpc", "quic", "wss", "gRPC"} {
		slim.BeaconTransport = tr
		if err := slimTransportConflict(slim); err == nil {
			t.Fatalf("slim+%s must conflict", tr)
		}
	}
	for _, tr := range []string{"http", "tcp", "dns", "icmp", "ssh", "mtls", "h2c", ""} {
		slim.BeaconTransport = tr
		if err := slimTransportConflict(slim); err != nil {
			t.Fatalf("slim+%q must be allowed: %v", tr, err)
		}
	}
	full := &ImplantConfig{BeaconTransport: "grpc"}
	if err := slimTransportConflict(full); err != nil {
		t.Fatalf("full+grpc must be allowed: %v", err)
	}
}

func TestBuildGoModSlim(t *testing.T) {
	full := buildGoMod("windows", false, false, false)
	for _, dep := range []string{"quic-go", "gorilla/websocket", "refraction-networking/utls", "golang.org/x/net"} {
		if !strings.Contains(full, dep) {
			t.Fatalf("full go.mod missing %s", dep)
		}
	}
	slim := buildGoMod("windows", false, true, false)
	for _, dep := range []string{"quic-go", "gorilla/websocket", "google.golang.org/grpc"} {
		if strings.Contains(slim, dep) {
			t.Fatalf("slim go.mod must not contain %s", dep)
		}
	}
	for _, dep := range []string{"refraction-networking/utls", "golang.org/x/net", "golang.org/x/sys"} {
		if !strings.Contains(slim, dep) {
			t.Fatalf("slim go.mod missing required %s", dep)
		}
	}
}

func TestBuildGoModWin7(t *testing.T) {
	mod := buildGoMod("windows", false, true, true)
	if !strings.Contains(mod, "go 1.20") {
		t.Fatalf("win7 go.mod must declare go 1.20:\n%s", mod)
	}
	for _, pin := range []string{"x/sys v0.19.0", "x/crypto v0.22.0", "x/net v0.24.0", "utls v1.5.4", "sqlite v1.21.0"} {
		if !strings.Contains(mod, pin) {
			t.Fatalf("win7 go.mod missing pin %s:\n%s", pin, mod)
		}
	}
	for _, dep := range []string{"quic-go", "gorilla/websocket", "google.golang.org/grpc", "go-winio v0.6.2", "sqlite v1.52.0", "go 1.25"} {
		if strings.Contains(mod, dep) {
			t.Fatalf("win7 go.mod must not contain %s", dep)
		}
	}
}

func TestWin7CompatConflict(t *testing.T) {
	if err := win7CompatConflict(&ImplantConfig{}); err != nil {
		t.Fatalf("plain config must pass: %v", err)
	}
	if err := win7CompatConflict(&ImplantConfig{Win7Compat: true, Obfuscate: true}); err == nil {
		t.Fatal("win7+garble must conflict")
	}
	if err := win7CompatConflict(&ImplantConfig{Win7Compat: true, Slim: true}); err != nil {
		t.Fatalf("win7+slim must pass: %v", err)
	}
}

func TestNormalizeForcesSlimOnWin7(t *testing.T) {
	cfg := ImplantConfig{Win7Compat: true}
	NormalizeImplantConfig(&cfg, t.TempDir())
	if !cfg.Slim {
		t.Fatal("Win7Compat must force Slim on")
	}
	cfg = ImplantConfig{}
	NormalizeImplantConfig(&cfg, t.TempDir())
	if cfg.Slim {
		t.Fatal("non-win7 must not force Slim")
	}
}

func TestGoToolchainEnv(t *testing.T) {
	t.Setenv("GOTOOLCHAIN", "go9.9.9")
	plain := goToolchainEnv(false)
	foundHost, foundPin := false, false
	for _, e := range plain {
		if e == "GOTOOLCHAIN=go9.9.9" {
			foundHost = true
		}
		if e == "GOTOOLCHAIN="+win7Toolchain {
			foundPin = true
		}
	}
	if !foundHost {
		t.Fatal("non-win7 must pass the host GOTOOLCHAIN through")
	}
	if foundPin {
		t.Fatal("non-win7 must not add the legacy pin")
	}
	legacy := goToolchainEnv(true)
	foundHost, foundPin = false, false
	for _, e := range legacy {
		if e == "GOTOOLCHAIN=go9.9.9" {
			foundHost = true
		}
		if e == "GOTOOLCHAIN="+win7Toolchain {
			foundPin = true
		}
	}
	if foundHost {
		t.Fatal("win7 env must replace, not append, GOTOOLCHAIN")
	}
	if !foundPin {
		t.Fatal("win7 env must pin GOTOOLCHAIN=" + win7Toolchain)
	}
}

func TestExtractAgentSourcesSlim(t *testing.T) {
	dir := t.TempDir()
	if err := extractAgentSources(payloadFS, dir, true); err != nil {
		t.Fatalf("extractAgentSources slim: %v", err)
	}
	for _, dropped := range []string{"transport_grpc.go", "transport_quic.go", "transport_wss.go"} {
		if _, err := os.Stat(filepath.Join(dir, dropped)); !os.IsNotExist(err) {
			t.Fatalf("slim build must drop %s", dropped)
		}
	}
	stubs, err := os.ReadFile(filepath.Join(dir, "transport_slim_stubs.go"))
	if err != nil {
		t.Fatalf("slim stubs missing: %v", err)
	}
	for _, sym := range []string{"func sendGRPCBeacon", "func sendQUICBeacon", "func sendWSSBeacon"} {
		if !strings.Contains(string(stubs), sym) {
			t.Fatalf("slim stubs missing %s", sym)
		}
	}
	utls, err := os.ReadFile(filepath.Join(dir, "transport_utls.go"))
	if err != nil {
		t.Fatalf("slim utls missing: %v", err)
	}
	if strings.Contains(string(utls), "google.golang.org/grpc") || strings.Contains(string(utls), "utlsCreds") {
		t.Fatal("slim utls still references grpc")
	}

	// Full extraction keeps everything and writes no stubs.
	fullDir := t.TempDir()
	if err := extractAgentSources(payloadFS, fullDir, false); err != nil {
		t.Fatalf("extractAgentSources full: %v", err)
	}
	if _, err := os.Stat(filepath.Join(fullDir, "transport_grpc.go")); err != nil {
		t.Fatalf("full build must keep transport_grpc.go: %v", err)
	}
	if _, err := os.Stat(filepath.Join(fullDir, "transport_slim_stubs.go")); !os.IsNotExist(err) {
		t.Fatal("full build must not write slim stubs")
	}
}

func TestMaybeCompressUPXSkipPaths(t *testing.T) {
	f := filepath.Join(t.TempDir(), "a.exe")
	if err := os.WriteFile(f, []byte("MZ-fake"), 0644); err != nil {
		t.Fatal(err)
	}
	// Disabled flag: untouched, no error.
	if err := maybeCompressUPX(f, ImplantConfig{}, "exe"); err != nil {
		t.Fatalf("disabled upx must no-op: %v", err)
	}
	// Unsupported format: skipped even when enabled.
	if err := maybeCompressUPX(f, ImplantConfig{UPX: true}, "macho"); err != nil {
		t.Fatalf("macho upx must skip: %v", err)
	}
	// Missing upx binary: skipped, file untouched. (If upx IS installed,
	// it rejects the fake input, restores the backup and returns an
	// error — either way the file must survive byte-identical.)
	before, _ := os.ReadFile(f)
	_ = maybeCompressUPX(f, ImplantConfig{UPX: true}, "exe")
	after, _ := os.ReadFile(f)
	if string(before) != string(after) {
		t.Fatal("upx must never corrupt the input on skip/failure paths")
	}
}

func TestMaterializeWin7Shim(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module agent\n\nrequire (\n)\n\nreplace github.com/forgec2/forgec2 => C:/repo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := materializeWin7Shim(dir); err != nil {
		t.Fatalf("materializeWin7Shim: %v", err)
	}
	for _, pkg := range []string{"internal/crypto", "pkg/protocol", "pkg/encoding"} {
		entries, err := os.ReadDir(filepath.Join(dir, "win7shimroot/github.com/forgec2/forgec2", pkg))
		if err != nil || len(entries) == 0 {
			t.Fatalf("shim package %s missing/empty: %v", pkg, err)
		}
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), "_test.go") {
				t.Fatalf("shim must exclude tests: %s", e.Name())
			}
		}
	}
	stubMod, err := os.ReadFile(filepath.Join(dir, "win7shimroot/github.com/forgec2/forgec2/go.mod"))
	if err != nil {
		t.Fatalf("stub go.mod missing: %v", err)
	}
	if !strings.Contains(string(stubMod), "module github.com/forgec2/forgec2") || !strings.Contains(string(stubMod), "go 1.20") {
		t.Fatalf("bad stub go.mod:\n%s", stubMod)
	}
	gomod, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gomod), "replace github.com/forgec2/forgec2 => ./win7shimroot/github.com/forgec2/forgec2") {
		t.Fatalf("agent go.mod replace not repointed:\n%s", gomod)
	}
}
