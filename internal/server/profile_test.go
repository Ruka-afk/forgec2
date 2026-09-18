package server

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/config"
	promtestutil "github.com/prometheus/client_golang/prometheus/testutil"
)

// TestStripMalleableRequest verifies the server inverts the agent's
// request-side wrapping so the enclosed JSON envelope is recovered unchanged.
func TestStripMalleableRequest(t *testing.T) {
	s := &Server{cfg: &config.Config{}}
	envelope := []byte(`{"uuid":"abc","c":"encrypted"}`)

	// No request transform configured: body passes through untouched.
	if got := s.stripMalleableRequest(envelope); !bytes.Equal(got, envelope) {
		t.Fatalf("no-op strip changed body: %s", got)
	}

	s.cfg.Malleable.RequestPrepend = "<html>"
	s.cfg.Malleable.RequestAppend = "</html>"

	wrapped := append([]byte("<html>"), envelope...)
	wrapped = append(wrapped, []byte("</html>")...)

	got := s.stripMalleableRequest(wrapped)
	if !bytes.Equal(got, envelope) {
		t.Fatalf("strip mismatch:\n got: %s\nwant: %s", got, envelope)
	}

	// Trimming a body that lacks the wrapper must not corrupt it.
	if got := s.stripMalleableRequest(envelope); !bytes.Equal(got, envelope) {
		t.Fatalf("stripping an unwrapped body corrupted it: %s", got)
	}
}

// TestApplyMalleableWrapping verifies raw (non-HTTP) beacon responses get the
// same prepend/append cover as the HTTP transport (I2), and that it is a no-op
// when malleable is disabled or no wrapper is configured.
func TestApplyMalleableWrapping(t *testing.T) {
	s := &Server{cfg: &config.Config{}}
	body := []byte(`{"uuid":"abc","c":"encrypted"}`)

	// Disabled: pass-through.
	if got := s.applyMalleableWrapping(body); !bytes.Equal(got, body) {
		t.Fatalf("disabled wrap changed body: %s", got)
	}

	s.cfg.Malleable.Enabled = true
	s.cfg.Malleable.Prepend = "<html>"
	s.cfg.Malleable.Append = "</html>"

	want := append([]byte("<html>"), body...)
	want = append(want, []byte("</html>")...)

	if got := s.applyMalleableWrapping(body); !bytes.Equal(got, want) {
		t.Fatalf("wrap mismatch:\n got: %s\nwant: %s", got, want)
	}

	// An empty prepend/append pair must remain a no-op even when enabled.
	s.cfg.Malleable.Prepend = ""
	s.cfg.Malleable.Append = ""
	if got := s.applyMalleableWrapping(body); !bytes.Equal(got, body) {
		t.Fatalf("enabled-but-empty wrap changed body: %s", got)
	}
}

// TestStripBodyPadding verifies the server undoes the agent's
// ContentLengthJitter framing (8-byte big-endian length + random padding) and
// that plain envelope bodies are never mis-stripped.
func TestStripBodyPadding(t *testing.T) {
	s := &Server{cfg: &config.Config{}}
	envelope := []byte(`{"uuid":"abc","c":"encrypted"}`)
	envelope = append(envelope, make([]byte, 200)...) // realistic envelope size

	// No padding configured on the agent side: the raw JSON body passes
	// through untouched (its leading bytes decode to an absurd length).
	if got := s.stripBodyPadding(envelope); !bytes.Equal(got, envelope) {
		t.Fatalf("unpadded body was modified: %s", got)
	}

	padBeaconBody := func(body []byte, pad int) []byte {
		out := make([]byte, 8, 8+len(body)+pad)
		binary.BigEndian.PutUint64(out, uint64(len(body)))
		out = append(out, body...)
		out = append(out, make([]byte, pad)...)
		return out
	}

	for _, pad := range []int{0, 1, 128, 1024} {
		padded := padBeaconBody(envelope, pad)
		got := s.stripBodyPadding(padded)
		if !bytes.Equal(got, envelope) {
			t.Fatalf("pad=%d: strip mismatch: got %d bytes, want %d", pad, len(got), len(envelope))
		}
	}

	// A malicious prefix claiming a length beyond the body must not slice
	// out of range: the strip must leave the body untouched.
	corrupt := padBeaconBody(envelope, 0)
	corrupt[0] = 0xff
	if got := s.stripBodyPadding(corrupt); !bytes.Equal(got, corrupt) {
		t.Fatalf("corrupt prefix was stripped instead of preserved")
	}
}

// coverTestServer builds a Server whose profile dir is isolated in TempDir.
// profileJSON, when non-empty, is stored as profiles/t.json (v1 payload form,
// which carries plain prepend/append response tokens).
func coverTestServer(t *testing.T, profileJSON string) *Server {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "profiles"), 0755); err != nil {
		t.Fatalf("mkdir profiles: %v", err)
	}
	if profileJSON != "" {
		if err := os.WriteFile(filepath.Join(dir, "profiles", "t.json"), []byte(profileJSON), 0644); err != nil {
			t.Fatalf("write profile: %v", err)
		}
	}
	cfg := &config.Config{}
	cfg.Server.DataDir = dir
	return &Server{cfg: cfg}
}

const coverV1Profile = `{"name":"t","prepend":"<PRE>","append":"<APP>"}`

// TestProfileBeaconPoolsPreset proves a named preset yields its request URIs
// and User-Agent for per-beacon rotation.
func TestProfileBeaconPoolsPreset(t *testing.T) {
	s := coverTestServer(t, "")
	s.cfg.Malleable.ProfileName = "microsoft"

	uris, uas := s.profileBeaconPools()
	foundURI := false
	for _, u := range uris {
		if u == "/common/oauth2/token" {
			foundURI = true
		}
		if !strings.HasPrefix(u, "/") {
			t.Fatalf("pool URI %q must stay path-only", u)
		}
	}
	if !foundURI || len(uris) < 2 {
		t.Fatalf("preset URIs = %v, want pool including /common/oauth2/token", uris)
	}
	if len(uas) == 0 {
		t.Fatal("preset should contribute a User-Agent")
	}
}

// TestPoolFiltersRejectHostileEntries proves the pool sanitizers drop
// absolute URLs, whitespace URIs, blank UAs and header-injecting UAs.
func TestPoolFiltersRejectHostileEntries(t *testing.T) {
	for _, u := range []string{"", "   ", "https://evil.example/x", "http://a/b", "/has space", "/tab\there", "no-leading-slash"} {
		if got, ok := filterPoolURI(u); ok {
			t.Errorf("filterPoolURI(%q) = %q, want reject", u, got)
		}
	}
	if got, ok := filterPoolURI("  /ok  "); !ok || got != "/ok" {
		t.Errorf("filterPoolURI trims valid URI: %q,%v", got, ok)
	}
	for _, ua := range []string{"", "   ", "bad\r\ninjected", "x\ny"} {
		if got, ok := filterPoolUA(ua); ok {
			t.Errorf("filterPoolUA(%q) = %q, want reject", ua, got)
		}
	}
	if got, ok := filterPoolUA("  UA-One  "); !ok || got != "UA-One" {
		t.Errorf("filterPoolUA trims valid UA: %q,%v", got, ok)
	}
}

// TestProfileBeaconPoolsV2File proves v2 BeaconURIs/URIs/UserAgents land in
// the pools.
func TestProfileBeaconPoolsV2File(t *testing.T) {
	s := coverTestServer(t, `{"name":"t","beacon_uris":["/c1","/c2"],"uris":["/c3"],"user_agents":["UA-One"]}`)
	s.cfg.Malleable.ProfileName = "t"

	uris, uas := s.profileBeaconPools()
	wantURIs := map[string]bool{"/c1": true, "/c2": true, "/c3": true}
	if len(uris) != len(wantURIs) {
		t.Fatalf("URIs = %v, want %v", uris, wantURIs)
	}
	for _, u := range uris {
		if !wantURIs[u] {
			t.Fatalf("unexpected URI %q", u)
		}
	}
	if len(uas) != 1 || uas[0] != "UA-One" {
		t.Fatalf("UAs = %v, want [UA-One]", uas)
	}
}

// TestProfileBeaconPoolsEmpty proves no profile means no rotation (fixed
// values preserved, zero behavior change for existing fleets).
func TestProfileBeaconPoolsEmpty(t *testing.T) {
	s := coverTestServer(t, "")
	if uris, uas := s.profileBeaconPools(); len(uris) != 0 || len(uas) != 0 {
		t.Fatalf("empty profile pools = (%v,%v), want empty", uris, uas)
	}
	s.cfg.Malleable.ProfileName = "no-such-profile"
	if uris, uas := s.profileBeaconPools(); len(uris) != 0 || len(uas) != 0 {
		t.Fatalf("missing profile pools = (%v,%v), want empty", uris, uas)
	}
}

// TestProfileJitterPoolsPreset proves preset parameter pools flow through.
func TestProfileJitterPoolsPreset(t *testing.T) {
	s := coverTestServer(t, "")
	s.cfg.Malleable.ProfileName = "microsoft"

	params, headers := s.profileJitterPools()
	if len(params) == 0 {
		t.Fatal("microsoft preset should contribute parameter names")
	}
	for _, p := range params {
		if strings.ContainsAny(p, " \t\r\n=&;") {
			t.Fatalf("param %q not wire-safe", p)
		}
	}
	if len(headers) != 0 {
		t.Fatalf("presets define no header pool, got %v", headers)
	}
}

// TestProfileJitterPoolsV2File proves v2 parameter_names + request_header_pool
// land in the pools (entry-level hostile filtering is unit-tested on
// splitPoolHeader; file validation rejects bad files wholesale first).
func TestProfileJitterPoolsV2File(t *testing.T) {
	s := coverTestServer(t, `{"name":"t","parameter_names":["id","op id",""],"request_header_pool":["X-Trace: abc"]}`)
	s.cfg.Malleable.ProfileName = "t"

	params, headers := s.profileJitterPools()
	if len(params) != 1 || params[0] != "id" {
		t.Fatalf("params = %v, want [id]", params)
	}
	if len(headers) != 1 || headers[0] != "X-Trace: abc" {
		t.Fatalf("headers = %v, want [X-Trace: abc]", headers)
	}
}

// TestSplitPoolHeader pins the pool line grammar shared with the agent
// parser (same wire, newline-joined).
func TestSplitPoolHeader(t *testing.T) {
	name, value, ok := splitPoolHeader("X-Trace: abc")
	if !ok || name != "X-Trace" || value != "abc" {
		t.Fatalf("got %q,%q,%v", name, value, ok)
	}
	if _, _, ok := splitPoolHeader("X-Trace: a:b:c"); !ok {
		t.Fatal("values may contain colons")
	} else {
		name, value, _ := splitPoolHeader("X-Trace: a:b:c")
		if name != "X-Trace" || value != "a:b:c" {
			t.Fatalf("got %q,%q", name, value)
		}
	}
	for _, bad := range []string{"", "nocolon", ":noval", "noname:", "   "} {
		if _, _, ok := splitPoolHeader(bad); ok {
			t.Fatalf("splitPoolHeader(%q) ok, want reject", bad)
		}
	}
}

// TestProfileJitterPoolsEmpty proves no profile means no jitter axes.
func TestProfileJitterPoolsEmpty(t *testing.T) {
	s := coverTestServer(t, "")
	if params, headers := s.profileJitterPools(); len(params) != 0 || len(headers) != 0 {
		t.Fatalf("empty profile pools = (%v,%v), want empty", params, headers)
	}
}

// TestNoteMalleableEventCounts proves every failure is counted (logs are
// throttled, metrics are not).
func TestNoteMalleableEventCounts(t *testing.T) {
	s := coverTestServer(t, "")
	s.metrics = NewMetricsCollector(s)

	s.noteMalleableEvent("encode_fail", "p1")
	s.noteMalleableEvent("encode_fail", "p1")
	s.noteMalleableEvent("profile_load_fail", "p2")

	// Every failure is counted (logs are throttled to one per key per 5m).
	if got := promtestutil.ToFloat64(s.metrics.MalleableEventsTotal.WithLabelValues("encode_fail", "p1")); got != 2 {
		t.Fatalf("encode_fail/p1=%v, want 2", got)
	}
	if got := promtestutil.ToFloat64(s.metrics.MalleableEventsTotal.WithLabelValues("profile_load_fail", "p2")); got != 1 {
		t.Fatalf("profile_load_fail/p2=%v, want 1", got)
	}
}

// TestLoadV2ProfileBrokenCounts proves a present-but-broken profile file
// fails visibly (nil + metric) instead of silently falling back fleet-wide.
func TestLoadV2ProfileBrokenCounts(t *testing.T) {
	s := coverTestServer(t, `{"name":`)
	s.metrics = NewMetricsCollector(s)

	// Overwrite the t.json written by coverTestServer with broken JSON under
	// the name loadV2Profile will look up.
	if v := s.loadV2Profile("t"); v != nil {
		t.Fatal("broken profile must load nil")
	}
	if n := promtestutil.CollectAndCount(s.metrics.MalleableEventsTotal); n != 1 {
		t.Fatalf("malleable event samples=%d, want 1", n)
	}
}

// TestApplyV2FileProfileEncodeFailureFallsBack proves a chain that passes
// validation but fails at encode time (mask with empty key "q;") falls back
// to the symmetric cover pair instead of a raw body.
func TestApplyV2FileProfileEncodeFailureFallsBack(t *testing.T) {
	s := coverTestServer(t, `{"name":"t","server_output":[{"type":"mask","value":";"}]}`)
	s.metrics = NewMetricsCollector(s)
	s.cfg.Malleable.Prepend = "<MP>"
	s.cfg.Malleable.Append = "</MP>"
	body := []byte(`{"uuid":"abc"}`)

	out, _, _, ok := s.applyV2FileProfile("t", body)
	if !ok {
		t.Fatal("v2 profile should apply (fallback, not refusal)")
	}
	want := append([]byte("<MP>"), body...)
	want = append(want, []byte("</MP>")...)
	if !bytes.Equal(out, want) {
		t.Fatalf("fallback mismatch:\n got: %s\nwant: %s", out, want)
	}
	if n := promtestutil.CollectAndCount(s.metrics.MalleableEventsTotal); n != 1 {
		t.Fatalf("malleable event samples=%d, want 1", n)
	}
}

// TestEffectiveCoverTokensMatrix pins the single-source rule: the pair is
// atomic, explicit mp.* wins, v2 files fill a fully-empty pair, presets
// resolve empty (their cover is transform chains, not raw bytes).
func TestEffectiveCoverTokensMatrix(t *testing.T) {
	// Explicit mp pair wins even with a profile name set.
	s := coverTestServer(t, coverV1Profile)
	s.cfg.Malleable.Prepend = "<MP>"
	s.cfg.Malleable.Append = "</MP>"
	s.cfg.Malleable.ProfileName = "t"
	if pre, app := s.effectiveCoverTokens(); pre != "<MP>" || app != "</MP>" {
		t.Fatalf("mp pair = (%q,%q), want (<MP>,</MP>)", pre, app)
	}

	// Partial mp pair still wins whole (atomicity: never mix sources).
	s.cfg.Malleable.Append = ""
	if pre, app := s.effectiveCoverTokens(); pre != "<MP>" || app != "" {
		t.Fatalf("partial mp pair = (%q,%q), want (<MP>,\"\")", pre, app)
	}

	// Empty mp + v2 file tokens: v2 pair fills in.
	s.cfg.Malleable.Prepend = ""
	s.cfg.Malleable.ProfileName = "t"
	if pre, app := s.effectiveCoverTokens(); pre != "<PRE>" || app != "<APP>" {
		t.Fatalf("v2 fallback = (%q,%q), want (<PRE>,<APP>)", pre, app)
	}

	// Empty mp + named preset: no raw bytes (chains only).
	s.cfg.Malleable.ProfileName = "microsoft"
	if pre, app := s.effectiveCoverTokens(); pre != "" || app != "" {
		t.Fatalf("preset pair = (%q,%q), want empty", pre, app)
	}

	// Empty mp + missing file: empty.
	s.cfg.Malleable.ProfileName = "no-such-profile"
	if pre, app := s.effectiveCoverTokens(); pre != "" || app != "" {
		t.Fatalf("missing file pair = (%q,%q), want empty", pre, app)
	}

	// Empty mp + no profile: empty.
	s.cfg.Malleable.ProfileName = ""
	if pre, app := s.effectiveCoverTokens(); pre != "" || app != "" {
		t.Fatalf("bare pair = (%q,%q), want empty", pre, app)
	}
}

// TestApplyMalleableWrappingV2Fallback proves raw transports get cover from a
// v2 file when mp.* are empty (previously bare), using agent-symmetric bytes.
func TestApplyMalleableWrappingV2Fallback(t *testing.T) {
	s := coverTestServer(t, coverV1Profile)
	s.cfg.Malleable.Enabled = true
	s.cfg.Malleable.ProfileName = "t"
	body := []byte(`{"uuid":"abc"}`)

	want := append([]byte("<PRE>"), body...)
	want = append(want, []byte("<APP>")...)
	if got := s.applyMalleableWrapping(body); !bytes.Equal(got, want) {
		t.Fatalf("wrap mismatch:\n got: %s\nwant: %s", got, want)
	}
}

// TestApplyV2FileProfileUsesSymmetricPair proves mixed config (mp pair set +
// v2 file with different tokens) wraps the mp pair the agent actually strips
// — previously the v2 pair, bricking parsing on live agents.
func TestApplyV2FileProfileUsesSymmetricPair(t *testing.T) {
	s := coverTestServer(t, coverV1Profile)
	s.cfg.Malleable.Prepend = "<MP>"
	s.cfg.Malleable.Append = "</MP>"
	body := []byte(`{"uuid":"abc"}`)

	out, _, _, ok := s.applyV2FileProfile("t", body)
	if !ok {
		t.Fatal("v2 profile should apply")
	}
	want := append([]byte("<MP>"), body...)
	want = append(want, []byte("</MP>")...)
	if !bytes.Equal(out, want) {
		t.Fatalf("wrap mismatch:\n got: %s\nwant: %s", out, want)
	}
}
