package payload

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// win7Pins are go1.20-compatible dependency versions for legacy builds.
// Current pins require newer Go (fail fast with "requires go >=" otherwise),
// so the Win7 branch repins the whole module set. Slim is forced on (see
// NormalizeImplantConfig), which keeps grpc/quic/gorilla out entirely.
//
// Beyond the six direct modules this also pins the modernc transitive
// closure: without it MVS lifts e.g. klauspost/compress to the modern line
// (required by the parent module) and go1.20 fails compiling it.
func win7Pins() []string {
	return []string{
		"\tgolang.org/x/sys v0.19.0",
		"\tgolang.org/x/crypto v0.22.0",
		"\tgolang.org/x/net v0.24.0",
		"\tgolang.org/x/text v0.14.0",
		"\tgithub.com/refraction-networking/utls v1.5.4",
		// quic-go is only pulled for utls's QUIC transport-parameter helper
		// (quicvarint); pin the era version utls itself requires.
		"\tgithub.com/quic-go/quic-go v0.37.4",
		"\tgithub.com/Microsoft/go-winio v0.6.1",
		"\tmodernc.org/sqlite v1.21.0",
		"\tmodernc.org/libc v1.22.3",
		"\tmodernc.org/mathutil v1.5.0",
		"\tmodernc.org/memory v1.5.0",
		"\tgithub.com/klauspost/compress v1.15.9",
		"\tgithub.com/google/uuid v1.3.0",
		"\tgithub.com/dustin/go-humanize v1.0.0",
		"\tgithub.com/mattn/go-isatty v0.0.16",
		"\tlukechampine.com/uint128 v1.2.0",
		"\tgithub.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec",
		// pkg/protocol + pkg/encoding use cbor/msgpack; pin era lines or
		// MVS lifts them past go1.20 (same mechanism as klauspost above).
		"\tgithub.com/fxamacker/cbor/v2 v2.5.0",
		"\tgithub.com/vmihailenco/msgpack/v5 v5.3.5",
		"\tgithub.com/vmihailenco/tagparser/v2 v2.0.0",
		"\tgithub.com/x448/float16 v0.8.4",
	}
}

// materializeWin7Shim vendors the go1.20-clean parent packages
// (internal/crypto, pkg/protocol, pkg/encoding) from the embedded mirror
// into a stub module and repoints the agent go.mod's parent replace at it.
// Without this, go1.20 refuses to compile those packages out of the go-1.25
// main module ("cannot compile Go 1.25 code") even though the sources
// themselves are language-compatible (verified by isolated builds).
func materializeWin7Shim(buildDir string) error {
	stubRoot := filepath.Join(buildDir, win7ShimModulePath)
	for _, pkg := range win7ShimPkgs {
		entries, err := win7shimFS.ReadDir("win7shim/" + pkg.dir)
		if err != nil {
			return fmt.Errorf("win7shim mirror missing %s: run node scripts/sync-win7shim.mjs: %w", pkg.dir, err)
		}
		dstDir := filepath.Join(stubRoot, filepath.FromSlash(pkg.importPath))
		if err := os.MkdirAll(dstDir, 0o750); err != nil {
			return err
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
				continue
			}
			data, err := win7shimFS.ReadFile("win7shim/" + pkg.dir + "/" + e.Name())
			if err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(dstDir, e.Name()), data, 0o644); err != nil {
				return err
			}
		}
	}
	if err := os.WriteFile(filepath.Join(stubRoot, "go.mod"), []byte("module github.com/forgec2/forgec2\n\ngo 1.20\n"), 0o644); err != nil {
		return err
	}
	// Repoint the parent replace at the stub. forgeC2ModuleReplace emits a
	// single `replace github.com/forgec2/forgec2 => <repodir>` line.
	gomodPath := filepath.Join(buildDir, "go.mod")
	raw, err := os.ReadFile(gomodPath)
	if err != nil {
		return err
	}
	lines := strings.Split(string(raw), "\n")
	rewrote := false
	for i, ln := range lines {
		if strings.HasPrefix(strings.TrimSpace(ln), "replace github.com/forgec2/forgec2 =>") {
			lines[i] = "replace github.com/forgec2/forgec2 => ./" + win7ShimModulePath
			rewrote = true
		}
	}
	if !rewrote {
		return fmt.Errorf("win7shim: parent replace line not found in generated go.mod")
	}
	return os.WriteFile(gomodPath, []byte(strings.Join(lines, "\n")), 0o644)
}

// buildGoModWin7 emits the legacy module file for Win7/2008R2 targets:
// go 1.20 directive plus fully repinned dependencies. Slim is forced on
// for Win7 (see NormalizeImplantConfig), so grpc/quic never appear here.
//
// The replace block is load-bearing, not cosmetic: the agent imports
// github.com/forgec2/forgec2/pkg/*, whose parent module requires MODERN
// dependency versions — without replaces, MVS would lift every pin back
// to the modern line and go1.20 would fail with "requires go >=".
func buildGoModWin7() string {
	replaceDir := forgeC2ModuleReplace()
	deps := append([]string{}, win7Pins()...)
	base := "module agent\n\ngo 1.20\n\nrequire (\n"
	base += strings.Join(deps, "\n") + "\n)\n"
	base += replaceDir
	for _, dep := range win7Pins() {
		fields := strings.Fields(dep)
		if len(fields) != 2 {
			continue
		}
		base += "replace " + fields[0] + " => " + fields[0] + " " + fields[1] + "\n"
	}
	return base
}

func buildGoMod(goos string, isDLL bool, slim bool, win7 bool) string {
	replaceDir := forgeC2ModuleReplace()
	if win7 {
		return buildGoModWin7()
	}
	deps := []string{}
	deps = append(deps, "\tgolang.org/x/sys v0.46.0")
	deps = append(deps, "\tgolang.org/x/crypto v0.53.0")

	if !isDLL && !slim {
		deps = append(deps, "\tgolang.org/x/net v0.56.0")
		deps = append(deps, "\tgithub.com/gorilla/websocket v1.5.3")
		deps = append(deps, "\tgithub.com/quic-go/quic-go v0.54.1")
		deps = append(deps, "\tgithub.com/refraction-networking/utls v1.6.7")
	}

	if !isDLL && slim {
		// Slim keeps x/net (h2c/icmp transports) and utls (shared JA3
		// layer for HTTPS/DoT/mTLS); only the grpc/quic/wss stacks go.
		deps = append(deps, "\tgolang.org/x/net v0.56.0")
		deps = append(deps, "\tgithub.com/refraction-networking/utls v1.6.7")
	}

	if goos == "windows" {
		deps = append(deps, "\tgithub.com/Microsoft/go-winio v0.6.2")
		deps = append(deps, "\tmodernc.org/sqlite v1.52.0")
	}

	if goos == "windows" && !isDLL && !slim {
		deps = append(deps, "\tgoogle.golang.org/grpc v1.82.0")
		deps = append(deps, "\tnhooyr.io/websocket v1.8.17")
	}

	base := "module agent\n\ngo 1.25\n\nrequire (\n"
	base += strings.Join(deps, "\n") + "\n)\n"
	return base + replaceDir
}

// win7CompatConflict rejects option combinations the legacy toolchain
// cannot honor. Failing here beats shipping a subtly broken implant.
func win7CompatConflict(cfg *ImplantConfig) error {
	if !cfg.Win7Compat {
		return nil
	}
	if cfg.Obfuscate {
		return fmt.Errorf("win7compat forbids garble obfuscation (garble requires a modern Go toolchain)")
	}
	return nil
}

// slimTransportConflict rejects light builds whose selected transport was
// compiled out. Failing here beats shipping an implant that can never beacon.
func slimTransportConflict(cfg *ImplantConfig) error {
	if !cfg.Slim {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(cfg.BeaconTransport)) {
	case "grpc", "quic", "wss":
		return fmt.Errorf("slim (light) build excludes the %q transport: pick http/tcp/dns/icmp/ssh/mtls/h2c or disable slim", cfg.BeaconTransport)
	}
	return nil
}
