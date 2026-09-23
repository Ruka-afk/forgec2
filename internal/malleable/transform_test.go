package malleable

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestTransformBlockNil(t *testing.T) {
	var tb *TransformBlock
	data := []byte("hello")
	result, err := tb.Apply(data, true)
	if err != nil {
		t.Fatalf("Apply() on nil block error = %v", err)
	}
	if !bytes.Equal(result, data) {
		t.Fatal("Apply on nil block should return original data")
	}
}

func TestTransformBlockEmpty(t *testing.T) {
	tb := &TransformBlock{}
	data := []byte("hello")
	result, err := tb.Apply(data, true)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(result, data) {
		t.Fatal("Apply with no transforms should return original data")
	}
}

func TestTransformBase64(t *testing.T) {
	tb := &TransformBlock{
		Transforms: []Transform{{Type: "base64"}},
	}
	original := []byte("hello world")
	encoded, err := tb.Apply(original, true)
	if err != nil {
		t.Fatalf("encode error = %v", err)
	}

	decoded, err := tb.Apply(encoded, false)
	if err != nil {
		t.Fatalf("decode error = %v", err)
	}

	if !bytes.Equal(decoded, original) {
		t.Fatalf("round trip: got %q, want %q", string(decoded), string(original))
	}
}

func TestTransformNetbios(t *testing.T) {
	tb := &TransformBlock{
		Transforms: []Transform{{Type: "netbios"}},
	}
	original := []byte("test")
	encoded, err := tb.Apply(original, true)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := tb.Apply(encoded, false)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(decoded, original) {
		t.Fatalf("round trip: got %q, want %q", string(decoded), string(original))
	}
}

func TestTransformAppend(t *testing.T) {
	tb := &TransformBlock{
		Transforms: []Transform{{Type: "append", Value: "!!!"}},
	}
	encoded, _ := tb.Apply([]byte("hello"), true)
	expected := "hello!!!"
	if string(encoded) != expected {
		t.Fatalf("got %q, want %q", string(encoded), expected)
	}
}

func TestTransformPrepend(t *testing.T) {
	tb := &TransformBlock{
		Transforms: []Transform{{Type: "prepend", Value: ">>>"}},
	}
	encoded, _ := tb.Apply([]byte("hello"), true)
	expected := ">>>hello"
	if string(encoded) != expected {
		t.Fatalf("got %q, want %q", string(encoded), expected)
	}
}

func TestTransformXor(t *testing.T) {
	tb := &TransformBlock{
		Transforms: []Transform{{Type: "xor", Value: "key"}},
	}
	original := []byte("hello")
	encoded, _ := tb.Apply(original, true)

	// XOR is symmetric, so apply again to decode
	decoded, _ := tb.Apply(encoded, true)
	if !bytes.Equal(decoded, original) {
		t.Fatalf("xor round trip: got %q, want %q", string(decoded), string(original))
	}
}

func TestTransformChain(t *testing.T) {
	tb := &TransformBlock{
		Transforms: []Transform{
			{Type: "base64"},
			{Type: "xor", Value: "secret"},
			{Type: "prepend", Value: "data:"},
		},
	}
	original := []byte("sensitive data")
	encoded, err := tb.Apply(original, true)
	if err != nil {
		t.Fatal(err)
	}

	// Can't just apply reverse because prepend isn't reversible.
	// Manually verify the chain was applied:
	if string(encoded[:5]) != "data:" {
		t.Fatalf("expected prepend 'data:', got %q", string(encoded[:5]))
	}
	payload := encoded[5:]
	xored, _ := (&TransformBlock{Transforms: []Transform{{Type: "xor", Value: "secret"}}}).Apply(payload, true)
	decoded, _ := base64.StdEncoding.DecodeString(string(xored))
	if !bytes.Equal(decoded, original) {
		t.Fatalf("chain round trip: got %q, want %q", string(decoded), string(original))
	}
}

func TestTransformMask(t *testing.T) {
	tests := []struct {
		name  string
		param string
		data  []byte
	}{
		{"no offset", "key", []byte("hello")},
		{"with offset", "key;3", []byte("test data")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tb := &TransformBlock{
				Transforms: []Transform{{Type: "mask", Value: tt.param}},
			}
			encoded, err := tb.Apply(tt.data, true)
			if err != nil {
				t.Fatal(err)
			}
			// Mask is XOR with key, so applying again undoes it
			decoded, _ := tb.Apply(encoded, true)
			if !bytes.Equal(decoded, tt.data) {
				t.Fatalf("mask round trip: got %q, want %q", string(decoded), string(tt.data))
			}
		})
	}
}

func TestTransformPrint(t *testing.T) {
	tb := &TransformBlock{
		Transforms: []Transform{{Type: "print"}},
	}
	original := []byte{0x48, 0x65, 0x6C, 0x6C, 0x6F}
	encoded, err := tb.Apply(original, true)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := tb.Apply(encoded, false)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(decoded, original) {
		t.Fatalf("print round trip: got %v, want %v", decoded, original)
	}
}

func TestTransformUnknown(t *testing.T) {
	tb := &TransformBlock{
		Transforms: []Transform{{Type: "unknown_type"}},
	}
	data := []byte("hello")
	result, err := tb.Apply(data, true)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(result, data) {
		t.Fatal("unknown transform should return data unchanged")
	}
}

func roundTrip(t *testing.T, typ, value string, original []byte) []byte {
	t.Helper()
	tb := &TransformBlock{Transforms: []Transform{{Type: typ, Value: value}}}
	encoded, err := tb.Apply(original, true)
	if err != nil {
		t.Fatalf("%s encode error = %v", typ, err)
	}
	decoded, err := tb.Apply(encoded, false)
	if err != nil {
		t.Fatalf("%s decode error = %v", typ, err)
	}
	if !bytes.Equal(decoded, original) {
		t.Fatalf("%s round trip: got %q, want %q", typ, decoded, original)
	}
	return encoded
}

func TestTransformBase64URL(t *testing.T) {
	// Unpadded lengths exercise the padding-repair branch on decode.
	for _, original := range [][]byte{[]byte(""), []byte("a"), []byte("ab"), []byte("abc"), []byte("hello world")} {
		encoded := roundTrip(t, "base64url", "", original)
		for _, b := range encoded {
			if b == '+' || b == '/' {
				t.Fatalf("base64url output not URL-safe: %q", encoded)
			}
		}
	}
}

func TestTransformNetbiosU(t *testing.T) {
	for _, original := range [][]byte{[]byte(""), []byte("A"), []byte("test"), []byte{0x00, 0xff, 0x80}} {
		roundTrip(t, "netbiosu", "", original)
	}
	// Odd trailing byte is dropped, never panics.
	tb := &TransformBlock{Transforms: []Transform{{Type: "netbiosu"}}}
	if _, err := tb.Apply([]byte("ABC"), false); err != nil {
		t.Fatalf("odd-length decode error = %v", err)
	}
}

func TestTransformStrrep(t *testing.T) {
	tb := &TransformBlock{Transforms: []Transform{{Type: "strrep", Value: "o:0"}}}
	encoded, err := tb.Apply([]byte("foo boo"), true)
	if err != nil {
		t.Fatalf("encode error = %v", err)
	}
	if string(encoded) != "f00 b00" {
		t.Fatalf("strrep encode = %q, want %q", encoded, "f00 b00")
	}
	decoded, err := tb.Apply(encoded, false)
	if err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if string(decoded) != "foo boo" {
		t.Fatalf("strrep round trip = %q, want %q", decoded, "foo boo")
	}
	// Empty old side is a documented no-op.
	noop := &TransformBlock{Transforms: []Transform{{Type: "strrep", Value: ":x"}}}
	out, err := noop.Apply([]byte("abc"), true)
	if err != nil || string(out) != "abc" {
		t.Fatalf("empty-old strrep = %q, %v; want unchanged", out, err)
	}
}

func TestTransformCase(t *testing.T) {
	tb := &TransformBlock{Transforms: []Transform{{Type: "case"}}}
	encoded, err := tb.Apply([]byte("Hello World 123!"), true)
	if err != nil {
		t.Fatalf("encode error = %v", err)
	}
	if string(encoded) != "HELLO WORLD 123!" {
		t.Fatalf("case encode = %q", encoded)
	}
	// Decode lowercases (one-way for mixed input by design).
	decoded, err := tb.Apply(encoded, false)
	if err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if string(decoded) != "hello world 123!" {
		t.Fatalf("case decode = %q", decoded)
	}
}

func TestTransformURLEncode(t *testing.T) {
	for _, original := range [][]byte{[]byte(""), []byte("a b+c/d?e=f&g"), []byte("100%")} {
		roundTrip(t, "urlencode", "", original)
	}
	// Illegal percent sequences decode to the input unchanged, never error.
	tb := &TransformBlock{Transforms: []Transform{{Type: "urlencode"}}}
	out, err := tb.Apply([]byte("%zz%"), false)
	if err != nil || string(out) != "%zz%" {
		t.Fatalf("bad-percent decode = %q, %v; want unchanged", out, err)
	}
}

func TestTransformURIAppend(t *testing.T) {
	tb := &TransformBlock{Transforms: []Transform{{Type: "uri_append", Value: ".php"}}}
	encoded, err := tb.Apply([]byte("/index"), true)
	if err != nil {
		t.Fatalf("encode error = %v", err)
	}
	if string(encoded) != "/index.php" {
		t.Fatalf("uri_append encode = %q", encoded)
	}
	decoded, err := tb.Apply(encoded, false)
	if err != nil {
		t.Fatalf("decode error = %v", err)
	}
	if string(decoded) != "/index" {
		t.Fatalf("uri_append round trip = %q", decoded)
	}
	// Missing suffix on decode passes through untouched.
	out, err := tb.Apply([]byte("/other"), false)
	if err != nil || string(out) != "/other" {
		t.Fatalf("suffix-less decode = %q, %v; want unchanged", out, err)
	}
}

// TestTransformChainDecodeReversed proves multi-step chains decode in
// reverse order (encode order inverted), matching the Go agent and Cobalt
// Strike output-block semantics. Previously decode ran forward, so any chain
// with 2+ order-sensitive steps (e.g. base64;xor) never round-tripped and
// placement decodes silently missed.
func TestTransformChainDecodeReversed(t *testing.T) {
	chains := [][]Transform{
		{{Type: "base64"}, {Type: "xor", Value: "secret"}},
		{{Type: "base64"}, {Type: "xor", Value: "k"}, {Type: "prepend", Value: "data:"}},
		{{Type: "print"}, {Type: "mask", Value: "m;k"}},
		{{Type: "netbios"}, {Type: "urlencode"}},
		{{Type: "base64url"}, {Type: "strrep", Value: "a:b"}, {Type: "append", Value: "!"}},
	}
	original := []byte(`{"type":"beacon","id":"AbC123+/=="}`)
	for i, chain := range chains {
		tb := &TransformBlock{Transforms: chain}
		enc, err := tb.Apply(original, true)
		if err != nil {
			t.Fatalf("chain %d encode: %v", i, err)
		}
		dec, err := tb.Apply(enc, false)
		if err != nil {
			t.Fatalf("chain %d decode: %v", i, err)
		}
		if !bytes.Equal(dec, original) {
			t.Fatalf("chain %d round trip: got %q, want %q", i, dec, original)
		}
	}
}
