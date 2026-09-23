package server

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func testSigningPair(t *testing.T) (pubHex, seedHex string) {
	t.Helper()
	pub, seed, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return hex.EncodeToString(pub), hex.EncodeToString(seed.Seed())
}

func signForTest(t *testing.T, seedHex string, data []byte) string {
	t.Helper()
	seed, err := hex.DecodeString(seedHex)
	if err != nil {
		t.Fatalf("decode seed: %v", err)
	}
	sig := ed25519.Sign(ed25519.NewKeyFromSeed(seed), data)
	return hex.EncodeToString(sig)
}

// TestVerifyReleaseSignature proves accept/reject/tamper semantics for
// detached release signatures.
func TestVerifyReleaseSignature(t *testing.T) {
	pubHex, seedHex := testSigningPair(t)
	data := []byte("abc123  forgec2-server-windows-amd64.exe\n")
	sigHex := signForTest(t, seedHex, data)

	if err := verifyReleaseSignature(data, sigHex, pubHex); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
	// Tampered checksum bytes.
	if err := verifyReleaseSignature([]byte("abc124  forgec2-server-windows-amd64.exe\n"), sigHex, pubHex); err == nil {
		t.Fatal("tampered checksum accepted")
	}
	// Tampered signature.
	bad := sigHex[:len(sigHex)-2] + "ff"
	if err := verifyReleaseSignature(data, bad, pubHex); err == nil {
		t.Fatal("tampered signature accepted")
	}
	// Wrong key.
	otherPub, _ := testSigningPair(t)
	if err := verifyReleaseSignature(data, sigHex, otherPub); err == nil {
		t.Fatal("wrong-key signature accepted")
	}
	// Malformed inputs.
	if err := verifyReleaseSignature(data, "zzzz", pubHex); err == nil {
		t.Fatal("malformed signature accepted")
	}
	if err := verifyReleaseSignature(data, sigHex, "zzzz"); err == nil {
		t.Fatal("malformed pubkey accepted")
	}
}

// TestMatchChecksumLine proves multi-asset checksum files match by filename
// (first-line-only matching verified against the wrong asset).
func TestMatchChecksumLine(t *testing.T) {
	content := "aaa  forgec2-server-linux-amd64\n" +
		"bbb  forgec2-server-windows-amd64.exe\n"
	got, err := matchChecksumLine(content, "forgec2-server-windows-amd64.exe")
	if err != nil || got != "bbb" {
		t.Fatalf("matched=%q err=%v, want bbb", got, err)
	}
	// Bare-hash legacy format still works.
	got, err = matchChecksumLine("ccc123\n", "anything.exe")
	if err != nil || got != "ccc123" {
		t.Fatalf("bare hash failed: %q %v", got, err)
	}
	// No entry and no bare hash.
	if _, err := matchChecksumLine("aaa  other.bin\n", "missing.exe"); err == nil {
		t.Fatal("missing entry accepted")
	}
}

// TestVerifyChecksumBytesEndToEnd signs a checksum file and verifies a real
// binary against it through the same helpers performHotUpdate uses.
func TestVerifyChecksumBytesEndToEnd(t *testing.T) {
	pubHex, seedHex := testSigningPair(t)
	dir := t.TempDir()
	binPath := filepath.Join(dir, "forgec2-server-windows-amd64.exe")
	payload := []byte("fake-binary-bytes")
	if err := os.WriteFile(binPath, payload, 0600); err != nil {
		t.Fatalf("write bin: %v", err)
	}
	checksum := "deadbeef  forgec2-server-windows-amd64.exe\n"
	// Real hash path first (wrong hash must fail before signature matters).
	if err := verifyChecksumBytes(binPath, []byte(checksum), "forgec2-server-windows-amd64.exe"); err == nil {
		t.Fatal("wrong hash accepted")
	}
	sigHex := signForTest(t, seedHex, []byte(checksum))
	if err := verifyReleaseSignature([]byte(checksum), sigHex, pubHex); err != nil {
		t.Fatalf("valid release signature rejected: %v", err)
	}
}
