package malleable

import (
	"bytes"
	"testing"
)

// fuzzSingletons covers every decode path with fixed small values so the
// amplification bound below is meaningful (fuzzed values would need a
// looser, less useful bound).
var fuzzSingletons = []Transform{
	{Type: "base64"},
	{Type: "base64url"},
	{Type: "netbios"},
	{Type: "netbiosu"},
	{Type: "mask", Value: "secret;3"},
	{Type: "mask", Value: ";"},
	{Type: "print"},
	{Type: "strrep", Value: "a:bb"},
	{Type: "case"},
	{Type: "urlencode"},
	{Type: "uri_append", Value: ".php"},
	{Type: "append", Value: "TAIL"},
	{Type: "prepend", Value: "HEAD"},
	{Type: "xor", Value: "k"},
	{Type: "bogus-type", Value: "v"},
}

// FuzzTransformBlockDecode feeds attacker-shaped bytes through every decode
// path: must never panic, must stay bounded, must be deterministic. Decode
// errors are fine (callers treat them as misses); panics and amplification
// are not.
func FuzzTransformBlockDecode(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte(""),
		[]byte("a"),
		[]byte("YQ=="),
		[]byte("%zz%"),
		[]byte("ABC"),
		[]byte{0x00, 0xff, 0x80, 0x7f},
		{0xff, 0xff, 0xff},
		bytes.Repeat([]byte("A"), 1024),
	} {
		f.Add(seed, 0)
	}
	f.Fuzz(func(t *testing.T, data []byte, idx int) {
		if idx < 0 {
			// math.MinInt negation overflows; fold instead of negating.
			idx = -(idx + 1)
		}
		tr := fuzzSingletons[idx%len(fuzzSingletons)]
		tb := &TransformBlock{Transforms: []Transform{tr}}
		out, err := tb.Apply(data, false)
		_ = err
		if len(out) > 4*len(data)+64 {
			t.Fatalf("type %q amplified %d -> %d bytes", tr.Type, len(data), len(out))
		}
		out2, err2 := tb.Apply(data, false)
		if (err == nil) != (err2 == nil) || !bytes.Equal(out, out2) {
			t.Fatalf("type %q non-deterministic for %q", tr.Type, data)
		}
	})
}

// FuzzTransformChainDecode chains two decodes (the placement-scan shape:
// chained attacker bytes through chained transforms).
func FuzzTransformChainDecode(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte(""),
		[]byte("aGVsbG8="),
		[]byte("%41%42"),
		bytes.Repeat([]byte("a"), 512),
	} {
		f.Add(seed, 0, 1)
	}
	f.Fuzz(func(t *testing.T, data []byte, i, j int) {
		if i < 0 {
			i = -(i + 1)
		}
		if j < 0 {
			j = -(j + 1)
		}
		tb := &TransformBlock{Transforms: []Transform{
			fuzzSingletons[i%len(fuzzSingletons)],
			fuzzSingletons[j%len(fuzzSingletons)],
		}}
		out, _ := tb.Apply(data, false)
		if len(out) > 16*len(data)+256 {
			t.Fatalf("chain amplified %d -> %d bytes", len(data), len(out))
		}
	})
}

// FuzzParseWire feeds malformed wire strings: must never panic and must be
// deterministic. Note: values containing ';' do NOT round-trip by design
// (documented on ParseWire), so only stability is asserted, not losslessness.
func FuzzParseWire(f *testing.F) {
	for _, seed := range []string{
		"",
		"base64",
		"xor:key;mask:k;3",
		";;;",
		"base64:",
		":value-only",
		"BASE64 : spaced ; Mask : k",
		"a:b:c",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, wire string) {
		first := ParseWire(wire)
		second := ParseWire(wire)
		if len(first) != len(second) {
			t.Fatalf("non-deterministic parse of %q", wire)
		}
		for i := range first {
			if first[i] != second[i] {
				t.Fatalf("non-deterministic parse of %q", wire)
			}
		}
		rewire := StepsToWire(first)
		reparse := ParseWire(rewire)
		_ = reparse
	})
}
