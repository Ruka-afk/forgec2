package payload

import (
	"bytes"
	"testing"
)

func TestEncodeShellcode_XOR(t *testing.T) {
	data := []byte("hello world\x90\x90\x90")
	key := []byte{0xAA}

	encoded, err := EncodeShellcode(data, EncodeXOR, key)
	if err != nil {
		t.Fatalf("EncodeShellcode XOR failed: %v", err)
	}
	if bytes.Equal(encoded, data) {
		t.Fatal("encoded data should differ from original")
	}
	if len(encoded) != len(data) {
		t.Fatalf("expected length %d, got %d", len(data), len(encoded))
	}

	decoded, err := DecodeShellcode(encoded, EncodeXOR, key)
	if err != nil {
		t.Fatalf("DecodeShellcode XOR failed: %v", err)
	}
	if !bytes.Equal(decoded, data) {
		t.Fatal("round-trip decode should match original")
	}
}

func TestEncodeShellcode_None(t *testing.T) {
	data := []byte("hello world\x90\x90\x90")
	encoded, err := EncodeShellcode(data, EncodeNone, nil)
	if err != nil {
		t.Fatalf("EncodeShellcode none failed: %v", err)
	}
	if !bytes.Equal(encoded, data) {
		t.Fatal("EncodeNone should return data unchanged")
	}
}

func TestEncodeShellcode_Empty(t *testing.T) {
	encoded, err := EncodeShellcode(nil, EncodeXOR, []byte{0xAA})
	if err != nil {
		t.Fatalf("EncodeShellcode empty failed: %v", err)
	}
	if len(encoded) != 0 {
		t.Fatalf("expected empty result, got %d bytes", len(encoded))
	}
}

func TestEncodeShellcode_Unknown(t *testing.T) {
	_, err := EncodeShellcode([]byte("test"), "unknown_encoder", nil)
	if err == nil {
		t.Fatal("expected error for unknown encoder")
	}
}

func TestEncodeShellcode_EmptyKey(t *testing.T) {
	data := []byte("test data")
	encoded, err := EncodeShellcode(data, EncodeXOR, nil)
	if err != nil {
		t.Fatalf("EncodeShellcode with nil key failed: %v", err)
	}
	if bytes.Equal(encoded, data) {
		t.Fatal("encoded data should differ with nil key")
	}
}

func TestEncodeShellcode_AES(t *testing.T) {
	data := []byte("sensitive payload data\x90")
	key := []byte("16bytekey!!12345")

	encoded, err := EncodeShellcode(data, EncodeAES, key)
	if err != nil {
		t.Fatalf("EncodeShellcode AES failed: %v", err)
	}
	if bytes.Equal(encoded, data) {
		t.Fatal("AES encoded data should differ from original")
	}

	decoded, err := DecodeShellcode(encoded, EncodeAES, key)
	if err != nil {
		t.Fatalf("DecodeShellcode AES failed: %v", err)
	}
	if !bytes.Equal(decoded, data) {
		t.Fatal("AES round-trip decode should match original")
	}
}

func TestEncodeShellcode_AESWrongKey(t *testing.T) {
	data := []byte("sensitive payload data\x90")
	key := []byte("16bytekey!!12345")

	encoded, err := EncodeShellcode(data, EncodeAES, key)
	if err != nil {
		t.Fatalf("EncodeShellcode AES failed: %v", err)
	}

	wrongKey := []byte("wrongkey!!1234567")
	decoded, err := DecodeShellcode(encoded, EncodeAES, wrongKey)
	if err != nil {
		t.Logf("got expected error with wrong key: %v", err)
		return
	}
	if bytes.Equal(decoded, data) {
		t.Fatal("decoding with wrong key should not produce original data")
	}
}
