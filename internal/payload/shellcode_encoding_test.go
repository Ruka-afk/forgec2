package payload

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestSGN_RoundTrip(t *testing.T) {
	data := []byte("\xEB\x02\x90\x90\x48\x31\xC0\xC3")
	key := []byte{0x5C}

	encoded, err := EncodeShellcode(data, EncodeSGN, key)
	if err != nil {
		t.Fatalf("EncodeShellcode SGN failed: %v", err)
	}
	if len(encoded) != sgnStubLen+len(data) {
		t.Fatalf("expected blob length %d, got %d", sgnStubLen+len(data), len(encoded))
	}
	decoded, err := DecodeShellcode(encoded, EncodeSGN, key)
	if err != nil {
		t.Fatalf("DecodeShellcode SGN failed: %v", err)
	}
	if !bytes.Equal(decoded, data) {
		t.Fatal("SGN round-trip decode should match original")
	}
}

func TestSGN_StubKeyByteMatchesEncodeKey(t *testing.T) {
	key := byte(0x7B)
	encoded, err := EncodeShellcode([]byte("\xEB\x02\x90\xC3"), EncodeSGN, []byte{key})
	if err != nil {
		t.Fatalf("EncodeShellcode SGN failed: %v", err)
	}
	// The executed decoder stub must XOR with the same key the blob was
	// encoded with, or the artifact would decode to garbage at runtime.
	if got := encoded[sgnKeyByteOffset]; got != key {
		t.Fatalf("stub key byte = 0x%02X, want 0x%02X", got, key)
	}
}

func TestSGN_StubLengthField(t *testing.T) {
	data := []byte("\xEB\x02\x90\x90\xC3")
	encoded, err := EncodeShellcode(data, EncodeSGN, []byte{0x11})
	if err != nil {
		t.Fatalf("EncodeShellcode SGN failed: %v", err)
	}
	// The loop count must be payloadLen+1: the decoder starts one byte before
	// the payload (pop rdx; dec rdx) and must still cover the last payload byte.
	want := uint16(len(data) + 1)
	if got := binary.LittleEndian.Uint16(encoded[8:10]); got != want {
		t.Fatalf("stub loop count = %d, want %d", got, want)
	}
}

func TestSGN_EmptyPayload(t *testing.T) {
	if _, err := EncodeShellcode(nil, EncodeSGN, []byte{0x11}); err == nil {
		t.Fatal("expected error for empty SGN payload")
	}
}

func TestSGN_TooLarge(t *testing.T) {
	data := make([]byte, 0xFFFF)
	if _, err := EncodeShellcode(data, EncodeSGN, []byte{0x11}); err == nil {
		t.Fatal("expected error for oversized SGN payload")
	}
}

func TestSGN_DecodeDefaultsToStubKey(t *testing.T) {
	data := []byte("\xEB\x02\x90\xC3")
	key := []byte{0x33}
	encoded, err := EncodeShellcode(data, EncodeSGN, key)
	if err != nil {
		t.Fatalf("EncodeShellcode SGN failed: %v", err)
	}
	decoded, err := DecodeShellcode(encoded, EncodeSGN, nil)
	if err != nil {
		t.Fatalf("DecodeShellcode SGN without key failed: %v", err)
	}
	if !bytes.Equal(decoded, data) {
		t.Fatal("SGN decode without key should recover the stub-embedded key")
	}
}

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
