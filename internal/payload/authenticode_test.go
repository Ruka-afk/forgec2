package payload

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/binary"
	"slices"
	"testing"
)

// parseAuthenticode walks ContentInfo → SignedData and returns the embedded
// certificate plus the signer info, so tests can cryptographically verify the
// signature the way a verifier would.
func parseAuthenticode(t *testing.T, pkcs7 []byte) (*x509.Certificate, signerInfo) {
	t.Helper()
	var ci contentInfo
	if _, err := asn1.Unmarshal(pkcs7, &ci); err != nil {
		t.Fatalf("unmarshal ContentInfo: %v", err)
	}
	if !ci.ContentType.Equal(oidSignedData) {
		t.Fatalf("content type = %v, want signedData", ci.ContentType)
	}
	var sd signedData
	if _, err := asn1.Unmarshal(ci.Content.Bytes, &sd); err != nil {
		t.Fatalf("unmarshal SignedData: %v", err)
	}
	cert, err := x509.ParseCertificate(sd.Certificates.Bytes)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}
	if len(sd.SignerInfos) != 1 {
		t.Fatalf("signer infos = %d, want 1", len(sd.SignerInfos))
	}
	return cert, sd.SignerInfos[0]
}

func TestSignPESelfSignedRoundTrip(t *testing.T) {
	pe := makePE(t, 0x8664, true)
	binary.LittleEndian.PutUint32(pe[0xD8:], 0xDEADBEEF) // pre-existing checksum

	signed, err := SignPESelfSigned(pe, "Acme Corp")
	if err != nil {
		t.Fatalf("SignPESelfSigned: %v", err)
	}
	if len(signed) <= len(pe) {
		t.Fatalf("signed image must be larger than input")
	}

	checksumOff, secDirOff, err := peSecurityFields(signed)
	if err != nil {
		t.Fatalf("peSecurityFields: %v", err)
	}
	tableOff := int(binary.LittleEndian.Uint32(signed[secDirOff:]))
	tableSize := int(binary.LittleEndian.Uint32(signed[secDirOff+4:]))
	if tableOff == 0 || tableOff%8 != 0 || tableOff+tableSize != len(signed) {
		t.Fatalf("security directory off=%d size=%d len=%d: table must be 8-aligned and at EOF",
			tableOff, tableSize, len(signed))
	}

	// WIN_CERTIFICATE header: length / revision 0x0200 / PKCS_SIGNED_DATA.
	dwLength := int(binary.LittleEndian.Uint32(signed[tableOff:]))
	wRevision := binary.LittleEndian.Uint16(signed[tableOff+4:])
	wCertType := binary.LittleEndian.Uint16(signed[tableOff+6:])
	if dwLength != tableSize || wRevision != winCertRevisionV2 || wCertType != winCertTypePKCSSignedData {
		t.Fatalf("WIN_CERTIFICATE{len=%d rev=%#x type=%#x}, want len=%d rev=0x200 type=2",
			dwLength, wRevision, wCertType, tableSize)
	}

	pkcs7 := signed[tableOff+8 : tableOff+tableSize]
	cert, si := parseAuthenticode(t, pkcs7)

	// Certificate subject carries the requested organization.
	if org := cert.Subject.Organization; len(org) != 1 || org[0] != "Acme Corp" {
		t.Fatalf("cert subject org = %v, want [Acme Corp]", org)
	}
	if !slices.Contains(cert.ExtKeyUsage, x509.ExtKeyUsageCodeSigning) {
		t.Fatal("cert missing CodeSigning EKU")
	}

	// Signature verification: RSA over the universal-SET encoding of the
	// authenticated attributes (PKCS#7 §9.2).
	pub, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		t.Fatalf("cert public key = %T, want *rsa.PublicKey", cert.PublicKey)
	}
	attrSet := derTLV(0x31, si.AuthenticatedAttributes.Bytes)
	digestOfAttrs := sha256.Sum256(attrSet)
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, digestOfAttrs[:], si.EncryptedDigest); err != nil {
		t.Fatalf("signature over authenticated attributes invalid: %v", err)
	}

	// The signed messageDigest attribute must equal the Authenticode digest of
	// the unsigned image AND of the signed image (the certificate table is
	// excluded from both).
	wantDigest, err := AuthenticodeDigest(pe)
	if err != nil {
		t.Fatalf("digest(unsigned): %v", err)
	}
	gotDigestSigned, err := AuthenticodeDigest(signed)
	if err != nil {
		t.Fatalf("digest(signed): %v", err)
	}
	if !bytes.Equal(wantDigest, gotDigestSigned) {
		t.Fatal("digest invariance broken: signed-image digest differs from unsigned")
	}

	var attrs []pkcs7Attribute
	// AuthenticatedAttributes.Bytes is a concatenation of attribute TLVs;
	// wrap it in a synthetic SEQUENCE so asn1 can decode the stream.
	if _, err := asn1.Unmarshal(derTLV(0x30, si.AuthenticatedAttributes.Bytes), &attrs); err != nil {
		t.Fatalf("unmarshal attributes: %v", err)
	}
	foundDigest, foundTime := false, false
	for _, a := range attrs {
		switch {
		case a.Type.Equal(oidMessageDigest):
			if len(a.Value) != 1 || !bytes.Equal(a.Value[0].Bytes, wantDigest) {
				t.Fatal("messageDigest attribute does not match recomputed PE digest")
			}
			foundDigest = true
		case a.Type.Equal(oidSigningTime):
			foundTime = true
		case a.Type.Equal(oidContentType):
			if len(a.Value) != 1 {
				t.Fatal("contentType attribute must carry exactly one value")
			}
			var ct asn1.ObjectIdentifier
			if _, err := asn1.Unmarshal(a.Value[0].FullBytes, &ct); err != nil || !ct.Equal(oidSPCIndirectData) {
				t.Fatalf("contentType attribute = %v (err %v), want SPC_INDIRECT_DATA_OBJID", ct, err)
			}
		}
	}
	if !foundDigest || !foundTime {
		t.Fatalf("attributes incomplete: messageDigest=%v signingTime=%v", foundDigest, foundTime)
	}

	// Refreshed optional-header checksum must be self-consistent.
	if got := computePEChecksum(signed, checksumOff); got != binary.LittleEndian.Uint32(signed[checksumOff:]) {
		t.Fatalf("stored checksum %#08x != computed %#08x",
			binary.LittleEndian.Uint32(signed[checksumOff:]), got)
	}
}

func TestSignPESelfSignedRejectsDoubleSign(t *testing.T) {
	pe := makePE(t, 0x8664, true)
	signed, err := SignPESelfSigned(pe, "")
	if err != nil {
		t.Fatalf("first sign: %v", err)
	}
	if _, err := SignPESelfSigned(signed, ""); err == nil {
		t.Fatal("second sign must fail on an already-signed image")
	}
}

func TestSignPESelfSignedRejectsNonPE(t *testing.T) {
	for name, blob := range map[string][]byte{
		"empty":     {},
		"short":     []byte("MZ"),
		"no-pe":     append([]byte("MZ"), make([]byte, 0x100)...),
		"shellcode": {0xCC, 0xCC, 0xCC},
	} {
		if _, err := SignPESelfSigned(blob, ""); err == nil {
			t.Fatalf("%s: expected error for non-PE input", name)
		}
	}
}

// TestBuildArtifactSelfSignedRequiresPE pins the honesty guard: a shellcode
// blob with cert_option=self_signed is rejected instead of silently ignoring
// the certificate request. Uses the raw output path so no Go toolchain runs.
func TestBuildArtifactSelfSignedRequiresPE(t *testing.T) {
	shellcode := base64.StdEncoding.EncodeToString([]byte{0xC3})
	_, _, err := BuildArtifact(BuildArtifactRequest{
		ShellcodeB64: shellcode,
		OutputType:   "raw",
		CertOption:   string(CertSelfSigned),
	}, t.TempDir())
	if err == nil {
		t.Fatal("self_signed on non-PE output must be rejected")
	}
}

// TestBuildArtifact_SelfSignedEXE exercises the full packer pipeline: a real
// toolchain-built loader EXE must come out carrying a registered Authenticode
// table whose embedded certificate carries the requested organization.
func TestBuildArtifact_SelfSignedEXE(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a Go toolchain build")
	}
	artifact, _, err := BuildArtifact(BuildArtifactRequest{
		ShellcodeB64: base64.StdEncoding.EncodeToString(tinyShellcode),
		OutputType:   "exe",
		CertOption:   string(CertSelfSigned),
		CertOrg:      "Integration Org",
	}, "data")
	if err != nil {
		t.Fatalf("BuildArtifact: %v", err)
	}
	assertPE(t, artifact, 0x8664)

	_, secDirOff, err := peSecurityFields(artifact)
	if err != nil {
		t.Fatalf("peSecurityFields: %v", err)
	}
	tableOff := int(binary.LittleEndian.Uint32(artifact[secDirOff:]))
	tableSize := int(binary.LittleEndian.Uint32(artifact[secDirOff+4:]))
	if tableOff == 0 || tableOff%8 != 0 || tableOff+tableSize != len(artifact) {
		t.Fatalf("certificate table not registered correctly (off=%d size=%d len=%d)",
			tableOff, tableSize, len(artifact))
	}
	cert, _ := parseAuthenticode(t, artifact[tableOff+8:tableOff+tableSize])
	if org := cert.Subject.Organization; len(org) != 1 || org[0] != "Integration Org" {
		t.Fatalf("cert org = %v, want [Integration Org]", org)
	}
}

// TestBuildArtifactUnsignedPathUnchanged guards regression: cert "none" keeps
// producing an unmodified raw artifact with no security directory surprises.
func TestBuildArtifactUnsignedPathUnchanged(t *testing.T) {
	raw := []byte{0xC3}
	out, filename, err := BuildArtifact(BuildArtifactRequest{
		ShellcodeB64:   base64.StdEncoding.EncodeToString(raw),
		OutputType:     "raw",
		CertOption:     string(CertNone),
		OutputFilename: "blob.bin",
	}, t.TempDir())
	if err != nil {
		t.Fatalf("BuildArtifact: %v", err)
	}
	if filename != "blob.bin" || !bytes.Equal(out, raw) {
		t.Fatalf("raw passthrough changed: file=%s out=%v", filename, out)
	}
}
