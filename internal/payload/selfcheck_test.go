package payload

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/forgec2/forgec2/pkg/peimage"
)

const testSelfCheckOffset = testPETextRawOff

func digestWithHashZeroed(t *testing.T, data []byte, idx int) string {
	t.Helper()
	calc := append([]byte(nil), data...)
	for i := idx; i < idx+len(selfCheckPlaceholder); i++ {
		calc[i] = 0
	}
	sum, err := peimage.Digest(calc)
	if err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(sum)
}

func TestPatchSelfCheckHashIsPEAwareAndSurvivesSigning(t *testing.T) {
	pe := makePE(t, 0x8664, true)
	// Force a non-8-byte length so the PE-aware patch must add alignment
	// padding before the certificate table can be appended.
	pe = append(pe, 0x01)
	copy(pe[testSelfCheckOffset:], selfCheckPlaceholder)
	path := filepath.Join(t.TempDir(), "selfcheck.exe")
	if err := os.WriteFile(path, pe, 0755); err != nil {
		t.Fatal(err)
	}

	if err := patchSelfCheckHash(path); err != nil {
		t.Fatalf("patchSelfCheckHash: %v", err)
	}
	patched, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(patched)%8 != 0 {
		t.Fatalf("patched PE length %d is not 8-byte aligned", len(patched))
	}
	if bytes.Contains(patched, []byte(selfCheckPlaceholder)) {
		t.Fatal("placeholder survived the patch")
	}
	embedded := string(patched[testSelfCheckOffset : testSelfCheckOffset+len(selfCheckPlaceholder)])
	if embedded != digestWithHashZeroed(t, patched, testSelfCheckOffset) {
		t.Fatal("build-time digest does not match the runtime reconstruction")
	}

	signed, err := SignPESelfSigned(patched, "Self Check Test")
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyPESignature(signed); err != nil {
		t.Fatalf("signed self-check image failed signature verification: %v", err)
	}
	if got := digestWithHashZeroed(t, signed, testSelfCheckOffset); got != embedded {
		t.Fatal("signing changed the self-check digest")
	}
}

func TestPatchSelfCheckHashTamperDetection(t *testing.T) {
	pe := makePE(t, 0x8664, true)
	copy(pe[testSelfCheckOffset:], selfCheckPlaceholder)
	path := filepath.Join(t.TempDir(), "selfcheck.exe")
	if err := os.WriteFile(path, pe, 0755); err != nil {
		t.Fatal(err)
	}
	if err := patchSelfCheckHash(path); err != nil {
		t.Fatal(err)
	}
	patched, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	embedded := string(patched[testSelfCheckOffset : testSelfCheckOffset+len(selfCheckPlaceholder)])
	tampered := append([]byte(nil), patched...)
	tampered[testPETextRawOff+0x40] ^= 0xFF
	if got := digestWithHashZeroed(t, tampered, testSelfCheckOffset); got == embedded {
		t.Fatal("tampering outside the hash region did not change the digest")
	}
}

func TestPatchSelfCheckHashNonPEUsesWholeFileDigest(t *testing.T) {
	data := append([]byte("ELF-test-payload"), []byte(selfCheckPlaceholder)...)
	path := filepath.Join(t.TempDir(), "selfcheck.elf")
	if err := os.WriteFile(path, data, 0755); err != nil {
		t.Fatal(err)
	}
	if err := patchSelfCheckHash(path); err != nil {
		t.Fatal(err)
	}
	patched, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(patched) != len(data) {
		t.Fatalf("non-PE file was padded from %d to %d bytes", len(data), len(patched))
	}
	idx := bytes.Index(data, []byte(selfCheckPlaceholder))
	embedded := string(patched[idx : idx+len(selfCheckPlaceholder)])
	calc := append([]byte(nil), patched...)
	for i := idx; i < idx+len(selfCheckPlaceholder); i++ {
		calc[i] = 0
	}
	sum := sha256.Sum256(calc)
	if embedded != hex.EncodeToString(sum[:]) {
		t.Fatal("non-PE self-check digest is not the whole-file SHA-256")
	}
}

func TestPatchSelfCheckHashRequiresUniquePlaceholder(t *testing.T) {
	pe := makePE(t, 0x8664, true)
	copy(pe[testSelfCheckOffset:], selfCheckPlaceholder)
	copy(pe[testSelfCheckOffset+0x80:], selfCheckPlaceholder)
	path := filepath.Join(t.TempDir(), "duplicate.exe")
	if err := os.WriteFile(path, pe, 0755); err != nil {
		t.Fatal(err)
	}
	if err := patchSelfCheckHash(path); err == nil {
		t.Fatal("duplicate placeholder was accepted")
	}
}
