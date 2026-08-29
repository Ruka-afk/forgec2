package payload

import (
	"crypto/ed25519"
	"encoding/hex"
	"testing"
)

// TestUpdateSigningRoundTrip pins the full trust chain: server signs a
// digest, the implant-side verification (same ed25519 call the agent makes)
// accepts it under the stamped public key and rejects tampering.
func TestUpdateSigningRoundTrip(t *testing.T) {
	dir := t.TempDir()
	SetUpdateSigningKeyFile(dir + "/update_signing.key")

	pubHex, err := UpdateSigningPublicKeyHex()
	if err != nil {
		t.Fatalf("public key: %v", err)
	}
	pub, err := hex.DecodeString(pubHex)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		t.Fatalf("public key size/decode: len=%d err=%v", len(pub), err)
	}

	digest := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	sigHex, err := SignUpdateHash(digest)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	sig, err := hex.DecodeString(sigHex)
	if err != nil || len(sig) != ed25519.SignatureSize {
		t.Fatalf("signature size/decode: len=%d err=%v", len(sig), err)
	}

	hash, _ := hex.DecodeString(digest)
	if !ed25519.Verify(ed25519.PublicKey(pub), hash, sig) {
		t.Fatal("valid signature failed verification")
	}
	tampered := append([]byte{}, sig...)
	tampered[0] ^= 0xFF
	if ed25519.Verify(ed25519.PublicKey(pub), hash, tampered) {
		t.Fatal("tampered signature accepted")
	}
}

// TestSignUpdateHashRejectsBadDigest covers input validation at the API
// boundary: non-hex and wrong-length digests must not produce signatures.
func TestSignUpdateHashRejectsBadDigest(t *testing.T) {
	dir := t.TempDir()
	SetUpdateSigningKeyFile(dir + "/update_signing.key")
	for name, bad := range map[string]string{
		"not hex":   "zz",
		"too short": "aabb",
	} {
		if _, err := SignUpdateHash(bad); err == nil {
			t.Errorf("%s: expected rejection", name)
		}
	}
}
