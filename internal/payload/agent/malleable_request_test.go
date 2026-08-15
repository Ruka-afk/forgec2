package main

import (
	"bytes"
	"testing"
)

// TestWrapMalleableRequest verifies the agent applies request-side prepend/append
// symmetrically to the server's strip, so the enclosed envelope is recovered.
func TestWrapMalleableRequest(t *testing.T) {
	prevP, prevA := MalleableRequestPrepend, MalleableRequestAppend
	defer func() { MalleableRequestPrepend, MalleableRequestAppend = prevP, prevA }()

	body := []byte(`{"uuid":"abc"}`)

	// Disabled: passthrough.
	MalleableRequestPrepend, MalleableRequestAppend = "", ""
	if got := wrapMalleableRequest(body); !bytes.Equal(got, body) {
		t.Fatalf("disabled wrap changed body: %s", got)
	}

	MalleableRequestPrepend, MalleableRequestAppend = "<html>", "</html>"
	wrapped := wrapMalleableRequest(body)
	want := append([]byte("<html>"), body...)
	want = append(want, []byte("</html>")...)
	if !bytes.Equal(wrapped, want) {
		t.Fatalf("wrap = %s, want %s", wrapped, want)
	}

	// Strip at the server must recover the original (mirrors stripMalleableRequest).
	// We replicate the server's TrimPrefix/TrimSuffix here to validate symmetry.
	stripped := bytes.TrimPrefix(wrapped, []byte("<html>"))
	stripped = bytes.TrimSuffix(stripped, []byte("</html>"))
	if !bytes.Equal(stripped, body) {
		t.Fatalf("asymmetric wrap/strip: %s", stripped)
	}
}
