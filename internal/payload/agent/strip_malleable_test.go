package main

import "testing"

func TestStripMalleableWrapping(t *testing.T) {
	// no wrappers configured: untouched even if the body looks suspicious
	MalleablePrepend, MalleableAppend = "", ""
	orig := []byte(`{"c":"abc"}`)
	if got := stripMalleableWrapping(append([]byte{}, orig...)); string(got) != string(orig) {
		t.Fatalf("no-op stripped unexpectedly: %q", got)
	}

	// both wrappers
	MalleablePrepend, MalleableAppend = "<html><body>", "</body></html>"
	body := []byte(`<html><body>{"c":"dGVzdA=="}</body></html>`)
	got := stripMalleableWrapping(append([]byte{}, body...))
	if string(got) != `{"c":"dGVzdA=="}` {
		t.Fatalf("prepend+append strip failed: %q", got)
	}

	// prepend only
	MalleablePrepend = "<!-- welcome -->"
	MalleableAppend = ""
	body = []byte(`<!-- welcome -->{"c":"x"}`)
	got = stripMalleableWrapping(append([]byte{}, body...))
	if string(got) != `{"c":"x"}` {
		t.Fatalf("prepend-only strip failed: %q", got)
	}

	// append only
	MalleablePrepend, MalleableAppend = "", "\r\n\r\n"
	body = []byte("{\"c\":\"x\"}\r\n\r\n")
	got = stripMalleableWrapping(append([]byte{}, body...))
	if string(got) != `{"c":"x"}` {
		t.Fatalf("append byte strip wrong: %q", got)
	}

	// unset for other tests in this package
	MalleablePrepend, MalleableAppend = "", ""
}

func TestStripMalleableWrappingToleratesMissingWrapper(t *testing.T) {
	MalleablePrepend, MalleableAppend = "<html><body>", "</body></html>"
	// Body without the configured wrapper must be returned as-is (the
	// server may have the profile disabled on this particular endpoint).
	body := []byte(`{"c":"raw"}`)
	got := stripMalleableWrapping(append([]byte{}, body...))
	if string(got) != `{"c":"raw"}` {
		t.Fatalf("missing wrapper must not corrupt the body: %q", got)
	}
	MalleablePrepend, MalleableAppend = "", ""
}