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
