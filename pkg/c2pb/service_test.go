package c2pb

import (
	"bytes"
	"encoding/json"
	"testing"
)

// TestEnvelopeJSONRoundTrip proves the JSON codec preserves an opaque protocol
// envelope byte-for-byte. The wire contract for c2pb is "opaque bytes inside a
// payload field": neither side may parse or reorder the envelope content inside
// the codec, so a round-trip inequality would corrupt handshake/registration/
// ciphertext frames.
func TestEnvelopeJSONRoundTrip(t *testing.T) {
	crafted := []byte(`{"type":"checkin","secret_id":"7f4a","mac":"ac:de:48:00:11:22","seq":9,"ctrl":{"sleep":30000},"ciphertext":"quJDlA=="}`)

	raw, err := JSONCodec.Marshal(&Envelope{Payload: crafted})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	got := new(Envelope)
	if err := JSONCodec.Unmarshal(raw, got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !bytes.Equal(got.Payload, crafted) {
		t.Fatalf("round-trip mismatch:\n got: %s\nwant: %s", got.Payload, crafted)
	}
}

func TestJSONCodecName(t *testing.T) {
	if JSONCodec.Name() != "json" {
		t.Fatalf("codec name = %q, want %q", JSONCodec.Name(), "json")
	}
}

// TestEnvelopePayloadWireKey locks the on-wire shape: a flat object whose only
// key is "payload" holding the envelope bytes base64-encoded (encoding/json's
// []byte rule). Renaming the key would desynchronize agent and server until
// both are rebuilt together, so the contract is pinned explicitly.
func TestEnvelopePayloadWireKey(t *testing.T) {
	raw, err := JSONCodec.Marshal(&Envelope{Payload: []byte{0x01, 0x02, 0x03}})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if len(m) != 1 {
		t.Fatalf("expected exactly one field, got %d (%s)", len(m), raw)
	}
	enc, ok := m["payload"]
	if !ok {
		t.Fatalf("expected payload key in %s", raw)
	}
	var b64 string
	if err := json.Unmarshal(enc, &b64); err != nil {
		t.Fatalf("payload must be a base64 string, got %s", enc)
	}
	if b64 != "AQID" {
		t.Fatalf("payload base64 = %q, want AQID (0x010203)", b64)
	}
}