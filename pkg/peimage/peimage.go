// Package peimage contains the small amount of PE parsing shared by the
// server-side payload generator and the runtime self-check in the agent.
//
// The Authenticode digest and the self-check digest must agree bit for bit:
// the server computes the digest over the unsigned, 8-byte-padded image and
// the agent recomputes it over the delivered file after the certificate table
// has been appended. Keeping the parsing and hashing rules in one package
// (rather than two copies that can silently drift) is what makes that
// invariant testable.
package peimage

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
)

// IsPE reports whether data starts with the DOS "MZ" signature. It is a cheap
// discriminator, not a full validity check: Digest and SecurityFields return a
// descriptive error for a truncated or malformed image.
func IsPE(data []byte) bool {
	return len(data) >= 2 && data[0] == 'M' && data[1] == 'Z'
}

// SecurityFields locates the two optional-header regions excluded from the
// Authenticode digest: the checksum field and the security directory entry.
// The offsets are absolute file offsets.
func SecurityFields(data []byte) (checksumOff, secDirOff int, err error) {
	if !IsPE(data) {
		return 0, 0, errors.New("not a PE image (missing MZ header)")
	}
	if len(data) < 0x40 {
		return 0, 0, errors.New("not a PE image (truncated DOS header)")
	}
	peOff := int(binary.LittleEndian.Uint32(data[0x3C:0x40]))
	if peOff <= 0 || peOff+24 > len(data) {
		return 0, 0, errors.New("not a PE image (truncated header)")
	}
	if string(data[peOff:peOff+4]) != "PE\x00\x00" {
		return 0, 0, errors.New("not a PE image (missing PE signature)")
	}
	opt := peOff + 24
	if opt+2 > len(data) {
		return 0, 0, errors.New("not a PE image (truncated optional header)")
	}
	magic := binary.LittleEndian.Uint16(data[opt:])
	var ddOff int
	switch magic {
	case 0x10B: // PE32
		ddOff = opt + 0x60
	case 0x20B: // PE32+
		ddOff = opt + 0x70
	default:
		return 0, 0, fmt.Errorf("unsupported optional header magic 0x%04X", magic)
	}
	checksumOff = opt + 64
	secDirOff = ddOff + 4*8 // data directory index 4 = IMAGE_DIRECTORY_ENTRY_SECURITY
	if secDirOff+8 > len(data) {
		return 0, 0, errors.New("PE image truncated before security directory")
	}
	return checksumOff, secDirOff, nil
}

// Digest returns the SHA-256 digest of data using the same algorithm as
// Authenticode: for a PE image, the checksum and security-directory fields are
// omitted and everything from the registered certificate table onward is
// excluded. For non-PE images (ELF, Mach-O) it hashes the whole file, which is
// what the self-check uses outside Windows.
//
// Omitting the two fields (rather than hashing zeros in their place) is
// important: it matches WinVerifyTrust and keeps the digest invariant when
// signing rewrites the checksum, installs the security directory and appends
// the certificate table.
func Digest(data []byte) ([]byte, error) {
	if !IsPE(data) {
		sum := sha256.Sum256(data)
		return sum[:], nil
	}

	checksumOff, secDirOff, err := SecurityFields(data)
	if err != nil {
		return nil, err
	}
	end := len(data)
	if tableOff := int(binary.LittleEndian.Uint32(data[secDirOff:])); tableOff > 0 && tableOff <= len(data) {
		end = tableOff
	}
	if checksumOff+4 > secDirOff || secDirOff+8 > end {
		return nil, errors.New("PE security fields overlap or are truncated")
	}

	h := sha256.New()
	h.Write(data[:checksumOff])
	h.Write(data[checksumOff+4 : secDirOff])
	h.Write(data[secDirOff+8 : end])
	return h.Sum(nil), nil
}

// PadTo8 appends zero bytes to a PE so a certificate table can start at an
// 8-byte-aligned file offset without the signer introducing padding the
// build-time digest never covered. Non-PE images are returned unchanged.
//
// The caller must write the returned slice back to disk. The returned slice may
// alias data when no padding is needed.
func PadTo8(data []byte) []byte {
	if !IsPE(data) || len(data)%8 == 0 {
		return data
	}
	out := make([]byte, len(data), len(data)+7)
	copy(out, data)
	return append(out, make([]byte, 8-len(data)%8)...)
}
