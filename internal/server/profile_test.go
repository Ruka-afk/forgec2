package server

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
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
