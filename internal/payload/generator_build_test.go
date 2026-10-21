package payload

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestValidateImplantBuildPreflight(t *testing.T) {
	base := func() *ImplantConfig { return &ImplantConfig{Architecture: "amd64"} }
	tests := []struct {
		name    string
		goos    string
		format  string
		mutate  func(*ImplantConfig)
		wantErr bool
	}{
		{name: "windows exe", goos: "windows", format: "exe"},
		{name: "linux arm", goos: "linux", format: "elf", mutate: func(c *ImplantConfig) { c.Architecture = "arm" }},
		{name: "darwin arm64", goos: "darwin", format: "macho", mutate: func(c *ImplantConfig) { c.Architecture = "arm64" }},
		{name: "unsupported os", goos: "freebsd", format: "elf", wantErr: true},
		{name: "windows 386 unsupported", goos: "windows", format: "exe", mutate: func(c *ImplantConfig) { c.Architecture = "386" }, wantErr: true},
		{name: "win7 non-windows", goos: "linux", format: "elf", mutate: func(c *ImplantConfig) { c.Win7Compat = true }, wantErr: true},
		{name: "selfcheck upx conflict", goos: "windows", format: "exe", mutate: func(c *ImplantConfig) {
			c.SelfCheck = true
			c.UPX = true
		}, wantErr: true},
		{name: "selfcheck dll rejected", goos: "windows", format: "dll", mutate: func(c *ImplantConfig) {
			c.SelfCheck = true
		}, wantErr: true},
		{name: "selfcheck sign conflict", goos: "windows", format: "exe", mutate: func(c *ImplantConfig) {
			c.SelfCheck = true
			c.SignMode = "self"
		}, wantErr: true},
		{name: "sign self exe", goos: "windows", format: "exe", mutate: func(c *ImplantConfig) { c.SignMode = "self" }},
		{name: "sign self dll", goos: "windows", format: "dll", mutate: func(c *ImplantConfig) { c.SignMode = "self" }},
		{name: "sign non-windows", goos: "linux", format: "elf", mutate: func(c *ImplantConfig) { c.SignMode = "self" }, wantErr: true},
		{name: "sign script unsupported", goos: "windows", format: "ps1", mutate: func(c *ImplantConfig) { c.SignMode = "self" }, wantErr: true},
		{name: "sign unknown mode", goos: "windows", format: "exe", mutate: func(c *ImplantConfig) { c.SignMode = "magic" }, wantErr: true},
		{name: "slim grpc conflict", goos: "linux", format: "elf", mutate: func(c *ImplantConfig) {
			c.Slim = true
			c.BeaconTransport = "grpc"
		}, wantErr: true},
		{name: "upx dll unsupported", goos: "windows", format: "dll", mutate: func(c *ImplantConfig) { c.UPX = true }, wantErr: true},
		{name: "upx macho unsupported", goos: "darwin", format: "macho", mutate: func(c *ImplantConfig) { c.UPX = true }, wantErr: true},
		{name: "dll garble unsupported", goos: "windows", format: "dll", mutate: func(c *ImplantConfig) { c.Obfuscate = true }, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base()
			if tc.mutate != nil {
				tc.mutate(cfg)
			}
			err := ValidateImplantBuild(tc.goos, tc.format, cfg)
			if tc.wantErr && err == nil {
				t.Fatal("expected validation error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}

// TestValidateImplantBuildUPXAvailability pins the fail-fast behavior: a
// checked UPX box without a usable upx executable must reject the request
// instead of publishing an uncompressed binary that looks like a success.
func TestValidateImplantBuildUPXAvailability(t *testing.T) {
	newCfg := func() *ImplantConfig { return &ImplantConfig{Architecture: "amd64", UPX: true} }

	t.Run("missing", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		if err := ValidateImplantBuild("windows", "exe", newCfg()); err == nil {
			t.Fatal("expected missing upx to fail preflight")
		}
	})

	t.Run("present", func(t *testing.T) {
		dir := t.TempDir()
		name := "upx"
		if runtime.GOOS == "windows" {
			name = "upx.exe"
		}
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", dir)
		if err := ValidateImplantBuild("windows", "exe", newCfg()); err != nil {
			t.Fatalf("unexpected validation error with upx present: %v", err)
		}
	})
}

func TestValidateELF(t *testing.T) {
	makeELF := func(class byte, machine uint16) []byte {
		buf := make([]byte, 64)
		copy(buf, "\x7fELF")
		buf[4] = class
		buf[5] = 1                                   // little endian
		buf[6] = 1                                   // EV_CURRENT
		binary.LittleEndian.PutUint16(buf[16:18], 2) // ET_EXEC
		binary.LittleEndian.PutUint16(buf[18:20], machine)
		return buf
	}
	tests := []struct {
		name   string
		data   []byte
		goarch string
		ok     bool
	}{
		{name: "amd64", data: makeELF(2, 62), goarch: "amd64", ok: true},
		{name: "arm64", data: makeELF(2, 183), goarch: "arm64", ok: true},
		{name: "arm", data: makeELF(1, 40), goarch: "arm", ok: true},
		{name: "wrong class", data: makeELF(1, 62), goarch: "amd64"},
		{name: "wrong machine", data: makeELF(2, 183), goarch: "amd64"},
		{name: "truncated", data: []byte("\x7fELF"), goarch: "amd64"},
		{name: "not elf", data: []byte("MZ................"), goarch: "amd64"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "artifact")
			if err := os.WriteFile(path, tc.data, 0644); err != nil {
				t.Fatal(err)
			}
			err := validateELF(path, tc.goarch)
			if tc.ok && err != nil {
				t.Fatalf("validateELF: %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestValidateMachO(t *testing.T) {
	makeMachO := func(cputype uint32) []byte {
		buf := make([]byte, 32)
		binary.LittleEndian.PutUint32(buf[:4], 0xfeedfacf)
		binary.LittleEndian.PutUint32(buf[4:8], cputype)
		return buf
	}
	tests := []struct {
		name   string
		data   []byte
		goarch string
		ok     bool
	}{
		{name: "amd64", data: makeMachO(0x01000007), goarch: "amd64", ok: true},
		{name: "arm64", data: makeMachO(0x0100000c), goarch: "arm64", ok: true},
		{name: "wrong cpu", data: makeMachO(0x0100000c), goarch: "amd64"},
		{name: "not macho", data: []byte("not a mach-o...................."), goarch: "amd64"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "artifact")
			if err := os.WriteFile(path, tc.data, 0644); err != nil {
				t.Fatal(err)
			}
			err := validateMachO(path, tc.goarch)
			if tc.ok && err != nil {
				t.Fatalf("validateMachO: %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestUniqueBuildName(t *testing.T) {
	a := uniqueBuildName("forgec2_agent", ".exe")
	b := uniqueBuildName("forgec2_agent", ".exe")
	if a == b {
		t.Fatalf("uniqueBuildName produced duplicate %q", a)
	}
	if !strings.HasSuffix(a, ".exe") || !strings.HasPrefix(a, "forgec2_agent_") {
		t.Fatalf("unexpected default name %q", a)
	}
}

func TestPrepareBuildOutputKeepsExtension(t *testing.T) {
	dir := t.TempDir()
	final, temp, err := prepareBuildOutput(dir, "agent.exe")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(final) != dir || filepath.Base(final) != "agent.exe" {
		t.Fatalf("unexpected final path %q", final)
	}
	if filepath.Ext(temp) != ".exe" || !strings.Contains(filepath.Base(temp), ".tmp-") {
		t.Fatalf("unexpected temp path %q", temp)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("output directory missing: %v", err)
	}
}

func TestCommitBuildOutputPublishesAtomically(t *testing.T) {
	dir := t.TempDir()
	final := filepath.Join(dir, "agent.exe")
	if err := os.WriteFile(final, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	temp := filepath.Join(dir, "agent.tmp-dead.exe")
	if err := os.WriteFile(temp, []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := commitBuildOutput(temp, final, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(temp); !os.IsNotExist(err) {
		t.Fatalf("temp output still exists after commit: %v", err)
	}
	data, err := os.ReadFile(final)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Fatalf("final output was not replaced: %q", data)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(final)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0111 == 0 {
			t.Fatalf("published executable is not executable: %v", info.Mode().Perm())
		}
	}
}

func TestReplaceModuleDirective(t *testing.T) {
	in := "module github.com/forgec2/forgec2\n\ngo 1.26.0\n"
	got := replaceModuleDirective(in, "agent")
	if !strings.HasPrefix(got, "module agent\n") {
		t.Fatalf("module directive not replaced:\n%s", got)
	}
	if !strings.Contains(got, "go 1.26.0") {
		t.Fatalf("other directives were dropped:\n%s", got)
	}
}

func TestStageRootAgentModule(t *testing.T) {
	dir := t.TempDir()
	ok, err := stageRootAgentModule(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Skip("root go.mod/go.sum not available in this checkout")
	}
	mod, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(mod)
	if !strings.HasPrefix(text, "module agent\n") {
		t.Fatalf("staged module has wrong name:\n%s", text)
	}
	if !strings.Contains(text, "replace github.com/forgec2/forgec2 =>") {
		t.Fatalf("staged module missing local replace:\n%s", text)
	}
	sum, err := os.ReadFile(filepath.Join(dir, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	if len(sum) == 0 {
		t.Fatal("staged go.sum is empty")
	}
}

func TestGoModuleEnvReplacesDuplicateKeys(t *testing.T) {
	env := goModuleEnv("GOOS=linux", "GOARCH=arm64", "CGO_ENABLED=0")
	count := map[string]int{}
	for _, entry := range env {
		key, _, ok := strings.Cut(entry, "=")
		if ok {
			count[key]++
		}
	}
	for _, key := range []string{"GOOS", "GOARCH", "CGO_ENABLED"} {
		if count[key] != 1 {
			t.Fatalf("expected exactly one %s entry, got %d", key, count[key])
		}
	}
}
