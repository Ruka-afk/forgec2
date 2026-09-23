//go:build linux || windows || darwin

package main

import (
	"testing"
)

// TestPrintParityWithServer pins the agent "print" codec to the server
// engine's vectors (internal/malleable printableEncode/printableDecode).
// The server emits lowercase hex and rejects odd-length / non-hex input;
// the agent must agree byte-for-byte or beacons break for profiles whose
// server_output/placement chain contains "print".
func TestPrintParityWithServer(t *testing.T) {
	// Server vector: printableEncode("Hi") == "4869".
	enc, err := agentApplyTransforms([]byte("Hi"), []agentTransformStep{{Name: "print"}}, true)
	if err != nil {
		t.Fatal(err)
	}
	if string(enc) != "4869" {
		t.Fatalf("print encode = %q, want %q (server vector)", enc, "4869")
	}
	dec, err := agentApplyTransforms([]byte("4869"), []agentTransformStep{{Name: "print"}}, false)
	if err != nil {
		t.Fatal(err)
	}
	if string(dec) != "Hi" {
		t.Fatalf("print decode = %q, want %q", dec, "Hi")
	}
	// Round-trip incl. empty input.
	for _, in := range []string{"", "hello world", "\x00\xff binary \x01"} {
		e, err := agentApplyTransforms([]byte(in), []agentTransformStep{{Name: "print"}}, true)
		if err != nil {
			t.Fatal(err)
		}
		d, err := agentApplyTransforms(e, []agentTransformStep{{Name: "print"}}, false)
		if err != nil {
			t.Fatal(err)
		}
		if string(d) != in {
			t.Fatalf("print round-trip = %q, want %q", d, in)
		}
	}
	// Server rejects odd length and uppercase/non-hex — agent must too.
	for _, bad := range []string{"abc", "ZZ", "48 69", "486G"} {
		if _, err := agentApplyTransforms([]byte(bad), []agentTransformStep{{Name: "print"}}, false); err == nil {
			t.Fatalf("print decode of %q should fail like the server", bad)
		}
	}
}
