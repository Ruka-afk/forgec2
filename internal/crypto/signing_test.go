package crypto

import (
	"bytes"
	"testing"
)

func TestSigningKeyRoundTrip(t *testing.T) {
	pub, priv, err := GenerateSigningKey()
	if err != nil {
		t.Fatalf("GenerateSigningKey: %v", err)
	}
	if len(pub) == 0 || len(priv) == 0 {
		t.Fatalf("expected non-empty keys, pub=%d priv=%d", len(pub), len(priv))
	}

	data := []byte("beacon-handshake-payload")
	sig := SignData(data, priv)
	if !VerifySignature(data, sig, pub) {
		t.Fatal("VerifySignature returned false for valid signature")
	}
}

func TestSigningKeyVerifyFailsOnTamperedData(t *testing.T) {
	pub, priv, err := GenerateSigningKey()
	if err != nil {
		t.Fatalf("GenerateSigningKey: %v", err)
	}

	data := []byte("original message")
	sig := SignData(data, priv)

	tampered := []byte("original messagX")
	if VerifySignature(tampered, sig, pub) {
		t.Fatal("VerifySignature accepted tampered data")
	}
}

func TestSigningKeyVerifyFailsOnTamperedSignature(t *testing.T) {
	pub, priv, err := GenerateSigningKey()
	if err != nil {
		t.Fatalf("GenerateSigningKey: %v", err)
	}

	data := []byte("message")
	sig := SignData(data, priv)

	badSig := bytes.Repeat([]byte{0xFF}, len(sig))
	if VerifySignature(data, badSig, pub) {
		t.Fatal("VerifySignature accepted tampered signature")
	}
}

func TestVerifySignatureShortSignature(t *testing.T) {
	pub, priv, err := GenerateSigningKey()
	if err != nil {
		t.Fatalf("GenerateSigningKey: %v", err)
	}

	data := []byte("message")
	sig := SignData(data, priv)

	if VerifySignature(data, sig[:len(sig)-1], pub) {
		t.Fatal("VerifySignature accepted truncated signature")
	}
}

func TestVerifySignatureNilKeyNoPanic(t *testing.T) {
	_, priv, err := GenerateSigningKey()
	if err != nil {
		t.Fatalf("GenerateSigningKey: %v", err)
	}
	data := []byte("message")
	sig := SignData(data, priv)

	if VerifySignature(data, sig, nil) {
		t.Fatal("VerifySignature accepted nil public key")
	}
	if VerifySignature(data, sig, []byte{0x01, 0x02}) {
		t.Fatal("VerifySignature accepted malformed public key")
	}
}

func TestSignDataNilKeyNoPanic(t *testing.T) {
	if sig := SignData([]byte("message"), nil); sig != nil {
		t.Fatal("expected nil signature for nil private key")
	}
}

func TestVerifySignatureWrongKey(t *testing.T) {
	_, priv1, err := GenerateSigningKey()
	if err != nil {
		t.Fatalf("GenerateSigningKey #1: %v", err)
	}

	data := []byte("message")
	sig := SignData(data, priv1)

	pub3, _, err := GenerateSigningKey()
	if err != nil {
		t.Fatalf("GenerateSigningKey #2: %v", err)
	}
	if VerifySignature(data, sig, pub3) {
		t.Fatal("VerifySignature accepted signature from a different key")
	}
}
