package server

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/forgec2/forgec2/internal/config"
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
