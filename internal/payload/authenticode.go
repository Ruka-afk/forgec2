package payload

// Authenticode signing for packer artifacts.
//
// Implements the subset of Microsoft Authenticode needed to embed a
// self-signed code-signing signature into a PE image:
//
//   - a fresh self-signed X.509 certificate (RSA-2048, SHA-256, CodeSigning EKU)
//   - a PKCS#7 SignedData whose encapContentInfo is SpcIndirectDataContent
//     carrying the Authenticode message digest of the image
//   - a WIN_CERTIFICATE (v2, PKCS_SIGNED_DATA) entry appended to the file and
//     registered in the optional header's security data directory
//
// The digest is computed exactly as WinVerifyTrust does: SHA-256 over the
// whole image with the optional-header Checksum field and the Security
// Directory entry (offset+size) zeroed. Everything else — section tables,
// timestamps, imports, resources — is covered by the digest.
//
// DER encoding notes: encoding/asn1 cannot express IMPLICIT context tags or
// hand-sorted SET OF payloads, so the few structures that need them
// ([0] EXPLICIT content, [0] IMPLICIT certificates, [0] IMPLICIT SET of
// authenticated attributes, the [1] IMPLICIT SpcSerializedObject moniker) are
// pre-encoded as asn1.RawValue with explicit TLV helpers below.

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

const (
	winCertRevisionV2         = 0x0200
	winCertTypePKCSSignedData = 0x0002
)

var (
	// spcMonikerClassID is the classId GUID signtool writes into the
	// SpcPeImageData moniker for PE images
	// (C43F7AA6-448C-11D4-9CBA-00C04FC2ADC9, little-endian byte order).
	// It is signed metadata only — verification never inspects it — but it is
	// kept identical to signtool output so parsers see a familiar shape.
	spcMonikerClassID = []byte{
		0xA6, 0x7A, 0x3F, 0xC4, 0x8C, 0x44, 0xD4, 0x11,
		0x9C, 0xBA, 0x00, 0xC0, 0x4F, 0xC2, 0xAD, 0xC9,
	}

	oidSignedData      = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 2}
	oidSHA256          = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 1}
	oidRSAEncryption   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 1}
	oidContentType     = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 3}
	oidMessageDigest   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 4}
	oidSigningTime     = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 5}
	oidSPCIndirectData = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 2, 1, 4}
	oidSPCPEImageData  = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 311, 2, 1, 15}
)

// ── minimal DER helpers ─────────────────────────────────────────────────────

func derLength(n int) []byte {
	switch {
	case n < 0x80:
		return []byte{byte(n)}
	case n < 0x100:
		return []byte{0x81, byte(n)}
	default:
		return []byte{0x82, byte(n >> 8), byte(n)}
	}
}

func derTLV(tag byte, body []byte) []byte {
	out := make([]byte, 0, len(body)+4)
	out = append(out, tag)
	out = append(out, derLength(len(body))...)
	return append(out, body...)
}

// ── PKCS#7 / SPC structures ────────────────────────────────────────────────

type algorithmIdentifier struct {
	Algorithm  asn1.ObjectIdentifier
	Parameters asn1.RawValue `asn1:"optional"` // NULL for RSA/SHA-256
}

var nullParams = asn1.RawValue{Tag: asn1.TagNull}

type issuerAndSerialNumber struct {
	IssuerName   asn1.RawValue // cert.RawSubject
	SerialNumber *big.Int
}

type pkcs7Attribute struct {
	Type  asn1.ObjectIdentifier
	Value []asn1.RawValue `asn1:"set"`
}

type digestInfo struct {
	Algorithm algorithmIdentifier
	Digest    []byte
}

type spcAttributeTypeAndOptionalValue struct {
	Type  asn1.ObjectIdentifier
	Value spcPeImageData
}

type spcPeImageData struct {
	Flags asn1.BitString
	File  asn1.RawValue `asn1:"optional"` // [1] IMPLICIT moniker, pre-encoded
}

type spcIndirectDataContent struct {
	Data          spcAttributeTypeAndOptionalValue
	MessageDigest digestInfo
}

type contentInfo struct {
	ContentType asn1.ObjectIdentifier
	Content     asn1.RawValue `asn1:"optional"` // [0] EXPLICIT, pre-encoded
}

type signerInfo struct {
	Version                   int
	IssuerAndSerialNumber     issuerAndSerialNumber
	DigestAlgorithm           algorithmIdentifier
	AuthenticatedAttributes   asn1.RawValue `asn1:"optional"` // [0] IMPLICIT SET, pre-encoded
	DigestEncryptionAlgorithm algorithmIdentifier
	EncryptedDigest           []byte
}

type signedData struct {
	Version          int
	DigestAlgorithms []algorithmIdentifier `asn1:"set"`
	ContentInfo      contentInfo
	Certificates     asn1.RawValue `asn1:"optional"` // [0] IMPLICIT SET OF Certificate
	SignerInfos      []signerInfo  `asn1:"set"`
}

// buildAuthenticatedAttributes returns the raw SET body (concatenated,
// OID-ascending attribute TLVs). Callers wrap it twice: once with the [0]
// IMPLICIT tag for embedding in SignerInfo and once as a universal SET for
// the signature input, per PKCS#7 §9.2 ("the value of the contents octets of
// the authenticatedAttributes field").
//
// Attribute order matters: SET OF requires ascending DER order, and
// contentType(…9.3) < messageDigest(…9.4) < signingTime(…9.5) already sorts.
func buildAuthenticatedAttributes(digest []byte, signingTime time.Time) ([]byte, error) {
	oidDER, err := asn1.Marshal(oidSPCIndirectData)
	if err != nil {
		return nil, err
	}
	timeDER, err := asn1.Marshal(signingTime.Truncate(time.Second))
	if err != nil {
		return nil, err
	}
	attrs := []pkcs7Attribute{
		{Type: oidContentType, Value: []asn1.RawValue{{FullBytes: oidDER}}},
		{Type: oidMessageDigest, Value: []asn1.RawValue{{Tag: asn1.TagOctetString, Bytes: digest}}},
		{Type: oidSigningTime, Value: []asn1.RawValue{{FullBytes: timeDER}}},
	}
	body := make([]byte, 0, 128)
	for _, a := range attrs {
		ab, err := asn1.Marshal(a)
		if err != nil {
			return nil, err
		}
		body = append(body, ab...)
	}
	return body, nil
}

// spcMoniker encodes the [1] IMPLICIT SpcSerializedObject inside
// SpcPeImageData: classId OCTET STRING + empty data OCTET STRING. IMPLICIT
// replaces the inner SEQUENCE tag, so the body carries the sequence's
// contents under a constructed context-1 tag.
func spcMoniker() asn1.RawValue {
	body := append(derTLV(0x04, spcMonikerClassID), derTLV(0x04, nil)...)
	return asn1.RawValue{Class: asn1.ClassContextSpecific, Tag: 1, IsCompound: true, Bytes: body}
}

func buildPKCS7(cert *x509.Certificate, key *rsa.PrivateKey, digest []byte, signingTime time.Time) ([]byte, error) {
	attrBody, err := buildAuthenticatedAttributes(digest, signingTime)
	if err != nil {
		return nil, fmt.Errorf("authenticated attributes: %w", err)
	}

	// Sign the universal-SET encoding of the attributes (PKCS#7 §9.2).
	h := sha256.Sum256(derTLV(0x31, attrBody))
	encryptedDigest, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, h[:])
	if err != nil {
		return nil, fmt.Errorf("sign attributes: %w", err)
	}

	contentVal := spcIndirectDataContent{
		Data: spcAttributeTypeAndOptionalValue{
			Type:  oidSPCPEImageData,
			Value: spcPeImageData{File: spcMoniker()},
		},
		MessageDigest: digestInfo{
			Algorithm: algorithmIdentifier{Algorithm: oidSHA256, Parameters: nullParams},
			Digest:    digest,
		},
	}
	contentDER, err := asn1.Marshal(contentVal)
	if err != nil {
		return nil, fmt.Errorf("spc indirect data: %w", err)
	}

	sd := signedData{
		Version: 1,
		DigestAlgorithms: []algorithmIdentifier{
			{Algorithm: oidSHA256, Parameters: nullParams},
		},
		ContentInfo: contentInfo{
			ContentType: oidSPCIndirectData,
			Content:     asn1.RawValue{Class: asn1.ClassContextSpecific, Tag: 0, IsCompound: true, Bytes: contentDER},
		},
		Certificates: asn1.RawValue{Class: asn1.ClassContextSpecific, Tag: 0, IsCompound: true, Bytes: cert.Raw},
		SignerInfos: []signerInfo{{
			Version: 1,
			IssuerAndSerialNumber: issuerAndSerialNumber{
				IssuerName:   asn1.RawValue{FullBytes: cert.RawSubject},
				SerialNumber: cert.SerialNumber,
			},
			DigestAlgorithm:           algorithmIdentifier{Algorithm: oidSHA256, Parameters: nullParams},
			AuthenticatedAttributes:   asn1.RawValue{Class: asn1.ClassContextSpecific, Tag: 0, IsCompound: true, Bytes: attrBody},
			DigestEncryptionAlgorithm: algorithmIdentifier{Algorithm: oidRSAEncryption, Parameters: nullParams},
			EncryptedDigest:           encryptedDigest,
		}},
	}
	sdDER, err := asn1.Marshal(sd)
	if err != nil {
		return nil, fmt.Errorf("signedData: %w", err)
	}
	ci := contentInfo{
		ContentType: oidSignedData,
		Content:     asn1.RawValue{Class: asn1.ClassContextSpecific, Tag: 0, IsCompound: true, Bytes: sdDER},
	}
	return asn1.Marshal(ci)
}

// selfSignedCodeCert mints a throwaway code-signing certificate. It is not a
// CA, carries only DigitalSignature usage plus the CodeSigning EKU, and is
// valid from yesterday (clock skew) for three years.
func selfSignedCodeCert(organization string) (*x509.Certificate, *rsa.PrivateKey, error) {
	org := strings.TrimSpace(organization)
	if org == "" {
		org = "ForgeC2"
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("generate key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 63))
	if err != nil {
		return nil, nil, fmt.Errorf("generate serial: %w", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: org, Organization: []string{org}},
		NotBefore:             time.Now().Add(-24 * time.Hour),
		NotAfter:              time.Now().AddDate(3, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, fmt.Errorf("create certificate: %w", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, fmt.Errorf("parse certificate: %w", err)
	}
	return cert, key, nil
}

// ── PE plumbing ────────────────────────────────────────────────────────────

// peSecurityFields locates the two optional-header regions excluded from the
// Authenticode digest: the checksum field and the security directory entry.
func peSecurityFields(data []byte) (checksumOff, secDirOff int, err error) {
	if len(data) < 0x40 || data[0] != 'M' || data[1] != 'Z' {
		return 0, 0, errors.New("not a PE image (missing MZ header)")
	}
	peOff := int(binary.LittleEndian.Uint32(data[0x3C:0x40]))
	if peOff <= 0 || peOff+24 > len(data) {
		return 0, 0, errors.New("not a PE image (truncated header)")
	}
	if string(data[peOff:peOff+4]) != "PE\x00\x00" {
		return 0, 0, errors.New("not a PE image (missing PE signature)")
	}
	opt := peOff + 24
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

// AuthenticodeDigest computes the SHA-256 message digest WinVerifyTrust
// verifies: the whole image with the optional-header checksum field and the
// security-directory entry zeroed, EXCLUDING the certificate table itself
// (everything from the registered table offset onward is not hashed — the
// table contains the signature). On an unsigned image this hashes the full
// file; on a signed image it reproduces exactly the digest that was signed.
func AuthenticodeDigest(data []byte) ([]byte, error) {
	checksumOff, secDirOff, err := peSecurityFields(data)
	if err != nil {
		return nil, err
	}
	end := len(data)
	if tableOff := int(binary.LittleEndian.Uint32(data[secDirOff:])); tableOff > 0 && tableOff <= len(data) {
		end = tableOff
	}
	scrub := make([]byte, end)
	copy(scrub, data[:end])
	binary.LittleEndian.PutUint32(scrub[checksumOff:], 0)
	clear(scrub[secDirOff : secDirOff+8])
	sum := sha256.Sum256(scrub)
	return sum[:], nil
}

// computePEChecksum implements the standard PE optional-header checksum
// accumulation (16-bit little-endian words, carry-folded, skipping the
// checksum field itself, plus the image length).
func computePEChecksum(data []byte, checksumOff int) uint32 {
	var sum uint64
	for off := 0; off+1 < len(data); off += 2 {
		if off >= checksumOff && off < checksumOff+4 {
			continue
		}
		sum += uint64(binary.LittleEndian.Uint16(data[off : off+2]))
		sum = (sum & 0xFFFF) + (sum >> 16)
	}
	sum = (sum & 0xFFFF) + (sum >> 16)
	return uint32(sum) + uint32(len(data))
}

// appendCertificateTable appends an 8-byte-aligned WIN_CERTIFICATE entry and
// registers it in the security directory, then refreshes the optional-header
// checksum so loaders that validate it see a consistent image (WinVerifyTrust
// zeroes that field for its digest either way).
func appendCertificateTable(pe []byte, secDirOff, checksumOff int, pkcs7 []byte) ([]byte, error) {
	const align = 8
	pad := (align - len(pkcs7)%align) % align
	entryLen := 8 + len(pkcs7) + pad

	out := make([]byte, len(pe), len(pe)+entryLen+align)
	copy(out, pe)
	for len(out)%align != 0 {
		out = append(out, 0)
	}
	tableOff := len(out)

	entry := make([]byte, entryLen)
	binary.LittleEndian.PutUint32(entry[0:], uint32(entryLen))
	binary.LittleEndian.PutUint16(entry[4:], winCertRevisionV2)
	binary.LittleEndian.PutUint16(entry[6:], winCertTypePKCSSignedData)
	copy(entry[8:], pkcs7)
	out = append(out, entry...)

	// The security directory "RVA" is a FILE offset for this directory.
	binary.LittleEndian.PutUint32(out[secDirOff:], uint32(tableOff))
	binary.LittleEndian.PutUint32(out[secDirOff+4:], uint32(entryLen))

	binary.LittleEndian.PutUint32(out[checksumOff:], computePEChecksum(out, checksumOff))
	return out, nil
}

// SignPESelfSigned appends an Authenticode signature generated with a fresh
// self-signed certificate to a PE image and registers it in the security data
// directory. organization becomes the certificate subject CN/O (empty →
// "ForgeC2"). The returned slice is a new buffer; the input is not modified.
func SignPESelfSigned(pe []byte, organization string) ([]byte, error) {
	checksumOff, secDirOff, err := peSecurityFields(pe)
	if err != nil {
		return nil, fmt.Errorf("authenticode: %w", err)
	}
	// Appending a second table would invalidate the first one's directory
	// registration; refuse rather than ship a doubly-signed image.
	if dirOff := binary.LittleEndian.Uint32(pe[secDirOff:]); dirOff != 0 {
		return nil, errors.New("authenticode: image already carries a signature table")
	}

	cert, key, err := selfSignedCodeCert(organization)
	if err != nil {
		return nil, fmt.Errorf("authenticode: %w", err)
	}

	digest, err := AuthenticodeDigest(pe)
	if err != nil {
		return nil, fmt.Errorf("authenticode: %w", err)
	}

	pkcs7, err := buildPKCS7(cert, key, digest, time.Now().UTC())
	if err != nil {
		return nil, fmt.Errorf("authenticode: %w", err)
	}

	signed, err := appendCertificateTable(pe, secDirOff, checksumOff, pkcs7)
	if err != nil {
		return nil, fmt.Errorf("authenticode: %w", err)
	}
	return signed, nil
}
