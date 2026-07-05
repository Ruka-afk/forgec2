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
