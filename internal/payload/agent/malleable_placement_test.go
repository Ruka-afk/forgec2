package main

import (
	"testing"
)

// TestBuildPlacementValues verifies cover copies land at the right locations
// with the chain applied, while the canonical body is untouched by this path.
func TestBuildPlacementValues(t *testing.T) {
	prev := MalleablePlacementStr
	defer func() { MalleablePlacementStr = prev }()

	body := []byte(`{"uuid":"abc"}`)
	MalleablePlacementStr = `[{"target":"query:id","chain":"base64"},{"target":"cookie:SESSION","chain":"xor:k"},{"target":"header:X-Cover","chain":""}]`
	q, cookies, headers := buildPlacementValues(body)
	if q["id"] == "" || cookies["SESSION"] == "" || headers["X-Cover"] == "" {
		t.Fatalf("missing placement values: q=%v cookies=%v headers=%v", q, cookies, headers)
	}
	// Empty chain means raw copy.
	if headers["X-Cover"] != string(body) {
		t.Fatalf("empty chain should copy raw body")
	}
	// Malformed JSON never breaks beacons.
	MalleablePlacementStr = `not-json`
	if q, c, h := buildPlacementValues(body); len(q) != 0 || len(c) != 0 || len(h) != 0 {
		t.Fatalf("malformed placements should yield nothing")
	}
	// Body targets are no-ops here.
	MalleablePlacementStr = `[{"target":"body","chain":"base64"}]`
	if q, c, h := buildPlacementValues(body); len(q) != 0 || len(c) != 0 || len(h) != 0 {
		t.Fatalf("body placement should be a no-op in cover copies")
	}
}

// TestSortedHeaderKeys ensures custom headers emit deterministically.
func TestSortedHeaderKeys(t *testing.T) {
	m := map[string]string{"Zebra": "1", "apple": "2", "Mango": "3"}
	got := sortedHeaderKeys(m)
	want := []string{"Mango", "Zebra", "apple"}
	if len(got) != len(want) {
		t.Fatalf("keys = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("keys = %v, want %v", got, want)
		}
	}
}

// TestXorMaskFullKey ensures the agent engine matches the server's
// full repeating-key semantics (previously single-byte on xor).
func TestXorMaskFullKey(t *testing.T) {
	data := []byte("hello world")
	steps := []agentTransformStep{{Name: "xor", Value: "key"}}
	enc, err := agentApplyTransforms(data, steps, true)
	if err != nil {
		t.Fatal(err)
	}
	dec, err := agentApplyTransforms(enc, steps, false)
	if err != nil {
		t.Fatal(err)
	}
	if string(dec) != string(data) {
		t.Fatalf("xor round-trip = %q, want %q", dec, data)
	}
	msteps := []agentTransformStep{{Name: "mask", Value: "secret;3"}}
	menc, err := agentApplyTransforms(data, msteps, true)
	if err != nil {
		t.Fatal(err)
	}
	mdec, err := agentApplyTransforms(menc, msteps, false)
	if err != nil {
		t.Fatal(err)
	}
	if string(mdec) != string(data) {
		t.Fatalf("mask round-trip = %q, want %q", mdec, data)
	}
}
