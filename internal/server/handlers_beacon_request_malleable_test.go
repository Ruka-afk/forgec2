package server

import (
	"encoding/binary"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/testutil"
)

// TestBeaconRequestMalleableWrapping exercises the full HTTP beacon path with
// request-side malleable wrapping: a real v3 agent register frame is wrapped
// with the server's configured prepend/append (exactly what an agent built with
// those tokens would send), posted through handleBeacon, and must register
// successfully — proving the server strips the wrapping BEFORE decoding the
// envelope. A negative control (same wrapping, no server transform) must be
// rejected as a malformed payload, isolating the strip as the cause.
func TestBeaconRequestMalleableWrapping(t *testing.T) {
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	s, r := initBeaconTestServer(t, database)

	agent := v3TestAgent(t, s, "aaaaaaaa-2222-4333-8444-bbbbbbbbbbbb")
	frame := agent.registerFrame()

	const prepend, appendStr = "<html><body>", "</body></html>"
	wrap := func(b string) string { return prepend + b + appendStr }

	// Enable request-side wrapping on the server (matches the agent's tokens).
	s.configMu.Lock()
	s.cfg.Malleable.RequestPrepend = prepend
	s.cfg.Malleable.RequestAppend = appendStr
	s.configMu.Unlock()

	w := postJSON(r, "/beacon", wrap(frame))
	if w.Code != http.StatusOK {
		t.Fatalf("wrapped register failed: status=%d body=%s", w.Code, w.Body.String())
	}

	// Negative control: identical wrapped body but the server has NO request
	// transform configured, so the strip is a no-op and the leading "<html>"
	// corrupts the JSON => handler rejects it before decode. This proves the
	// first success depended on the strip, not on the frame being valid alone.
	s.configMu.Lock()
	s.cfg.Malleable.RequestPrepend = ""
	s.cfg.Malleable.RequestAppend = ""
	s.configMu.Unlock()

	w2 := postJSON(r, "/beacon", wrap(frame))
	if w2.Code == http.StatusOK {
		t.Fatalf("expected rejection for unstrippable wrapped body, got 200: %s", w2.Body.String())
	}
	if !strings.Contains(w2.Body.String(), "invalid beacon payload") {
		t.Fatalf("expected 'invalid beacon payload', got: %s", w2.Body.String())
	}
}

// TestBeaconRequestMalleableDisabledPassthrough ensures that when no request
// transform is configured the raw (unwrapped) frame is processed normally.
func TestBeaconRequestMalleableDisabledPassthrough(t *testing.T) {
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	s, r := initBeaconTestServer(t, database)

	agent := v3TestAgent(t, s, "cccccccc-2222-4333-8444-dddddddddddd")
	w := postJSON(r, "/beacon", agent.registerFrame())
	if w.Code != http.StatusOK {
		t.Fatalf("unwrapped register failed: status=%d body=%s", w.Code, w.Body.String())
	}
}

// TestBeaconContentLengthJitter exercises the HTTP beacon path with the
// agent's body-length padding: a real v3 register frame wrapped in the
// 8-byte big-endian length prefix + random trailing bytes (the exact framing
// padBeaconBody produces when ContentLengthJitter > 0) must register
// successfully — proving handleBeacon strips the padding before envelope
// decode. A negative control with a corrupted prefix must be rejected.
func TestBeaconContentLengthJitter(t *testing.T) {
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	s, r := initBeaconTestServer(t, database)

	agent := v3TestAgent(t, s, "dddddddd-2222-4333-8444-eeeeeeeeeeee")
	frame := []byte(agent.registerFrame())

	pad := func(body []byte, extra int) []byte {
		out := make([]byte, 8, 8+len(body)+extra)
		binary.BigEndian.PutUint64(out, uint64(len(body)))
		out = append(out, body...)
		for i := 0; i < extra; i++ {
			out = append(out, byte(i*31+7))
		}
		return out
	}

	// Registration is single-use, so each padded frame below needs its own
	// agent identity.
	for i, extra := range []int{0, 1, 512} {
		a := v3TestAgent(t, s, fmt.Sprintf("%08x-2222-4333-8444-eeeeeeeeeeee", i))
		w := postJSON(r, "/beacon", string(pad([]byte(a.registerFrame()), extra)))
		if w.Code != http.StatusOK {
			t.Fatalf("padded register failed (extra=%d): status=%d body=%s", extra, w.Code, w.Body.String())
		}
	}

	// Negative control: a corrupt length prefix must not be stripped; the
	// handler rejects the frame instead of crashing or slicing out of range.
	corrupt := pad(frame, 0)
	corrupt[0] = 0xff
	w := postJSON(r, "/beacon", string(corrupt))
	if w.Code == http.StatusOK {
		t.Fatalf("expected rejection for corrupt length prefix, got 200: %s", w.Body.String())
	}
}
