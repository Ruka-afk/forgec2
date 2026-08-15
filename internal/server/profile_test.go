package server

import (
	"bytes"
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
