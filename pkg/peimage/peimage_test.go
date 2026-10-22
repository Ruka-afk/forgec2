package peimage

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"testing"
)

const (
	testPEOff   = 0x80
	testOpt32   = 0xE0
	testOpt64   = 0xF0
	testPayload = 0x200
	testFileLen = 0x400
)

// makeTestPE returns a header-complete PE image large enough for the parser.
// It deliberately carries no section table: the digest only needs the optional
// header, while a real build is validated by the full PE validator elsewhere.
func makeTestPE(pe32plus bool, length int) []byte {
	if length < testFileLen {
		length = testFileLen
	}
	data := make([]byte, length)
	data[0], data[1] = 'M', 'Z'
	binary.LittleEndian.PutUint32(data[0x3C:], testPEOff)
	copy(data[testPEOff:], "PE\x00\x00")
	binary.LittleEndian.PutUint16(data[testPEOff+4:], 0x8664)
	binary.LittleEndian.PutUint16(data[testPEOff+6:], 1)
	optSize := uint16(testOpt32)
	magic := uint16(0x10B)
	if pe32plus {
		optSize = testOpt64
		magic = 0x20B
	}
	binary.LittleEndian.PutUint16(data[testPEOff+20:], optSize)
	opt := testPEOff + 24
	binary.LittleEndian.PutUint16(data[opt:], magic)
	checksumOff, secDirOff, _ := SecurityFields(data)
	binary.LittleEndian.PutUint32(data[checksumOff:], 0xDEADBEEF)
	binary.LittleEndian.PutUint32(data[secDirOff:], 0)
	binary.LittleEndian.PutUint32(data[secDirOff+4:], 0)
	for i := testPayload; i < testPayload+16; i++ {
		data[i] = byte(i)
	}
	return data
}

func TestDigestNonPE(t *testing.T) {
	data := []byte("not a PE image")
	got, err := Digest(data)
	if err != nil {
		t.Fatal(err)
	}
	want := sha256.Sum256(data)
	if !bytes.Equal(got, want[:]) {
		t.Fatal("non-PE digest is not the whole-file SHA-256")
	}
}

func TestSecurityFieldsBothMagics(t *testing.T) {
	for _, pe32plus := range []bool{false, true} {
		data := makeTestPE(pe32plus, testFileLen)
		checksumOff, secDirOff, err := SecurityFields(data)
		if err != nil {
			t.Fatalf("PE32+ = %v: %v", pe32plus, err)
		}
		opt := testPEOff + 24
		if checksumOff != opt+64 {
			t.Fatalf("checksum offset = %#x, want %#x", checksumOff, opt+64)
		}
		wantSec := opt + 0x60 + 4*8
		if pe32plus {
			wantSec = opt + 0x70 + 4*8
		}
		if secDirOff != wantSec {
			t.Fatalf("security directory offset = %#x, want %#x", secDirOff, wantSec)
		}
	}
}

func TestDigestIgnoresChecksumAndCertificateTable(t *testing.T) {
	for _, pe32plus := range []bool{false, true} {
		base := makeTestPE(pe32plus, testFileLen)
		want, err := Digest(base)
		if err != nil {
			t.Fatal(err)
		}

		signed := append([]byte(nil), base...)
		checksumOff, secDirOff, _ := SecurityFields(signed)
		tableOff := len(signed)
		signed = append(signed, make([]byte, 16)...)
		binary.LittleEndian.PutUint32(signed[secDirOff:], uint32(tableOff))
		binary.LittleEndian.PutUint32(signed[secDirOff+4:], 16)
		binary.LittleEndian.PutUint32(signed[checksumOff:], 0x12345678)
		got, err := Digest(signed)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("PE32+ = %v: signing changed the digest", pe32plus)
		}

		tampered := append([]byte(nil), base...)
		tampered[testPayload] ^= 0xFF
		changed, err := Digest(tampered)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Equal(changed, want) {
			t.Fatalf("PE32+ = %v: payload change did not change the digest", pe32plus)
		}
	}
}

func TestPadTo8MakesRoomForCertificateTableWithoutChangingDigest(t *testing.T) {
	// Length 0x401 forces seven bytes of padding.
	base := makeTestPE(true, testFileLen+1)
	padded := PadTo8(base)
	if len(padded)%8 != 0 {
		t.Fatalf("padded length %d is not 8-byte aligned", len(padded))
	}
	if !bytes.Equal(padded[:len(base)], base) {
		t.Fatal("padding modified the image")
	}
	for _, b := range padded[len(base):] {
		if b != 0 {
			t.Fatal("padding is not zero-filled")
		}
	}
	want, err := Digest(padded)
	if err != nil {
		t.Fatal(err)
	}

	signed := append([]byte(nil), padded...)
	checksumOff, secDirOff, _ := SecurityFields(signed)
	tableOff := len(signed)
	signed = append(signed, make([]byte, 16)...)
	binary.LittleEndian.PutUint32(signed[secDirOff:], uint32(tableOff))
	binary.LittleEndian.PutUint32(signed[secDirOff+4:], 16)
	binary.LittleEndian.PutUint32(signed[checksumOff:], 0xAABBCCDD)
	got, err := Digest(signed)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("appending an aligned certificate table after padding changed the digest")
	}
}

func TestPadTo8NonPEUnchanged(t *testing.T) {
	data := []byte("ELF-ish bytes")
	got := PadTo8(data)
	if &got[0] != &data[0] || len(got) != len(data) {
		t.Fatal("non-PE data must be returned unchanged")
	}
}

func TestMalformedPEMZ(t *testing.T) {
	for _, data := range [][]byte{
		[]byte("MZ"),
		bytes.Repeat([]byte{0}, 0x40),
	} {
		if len(data) == 0x40 {
			data[0], data[1] = 'M', 'Z'
		}
		if _, _, err := SecurityFields(data); err == nil {
			t.Fatal("expected malformed PE to be rejected")
		}
		if _, err := Digest(data); err == nil {
			t.Fatal("expected malformed PE digest to be rejected")
		}
	}
}
