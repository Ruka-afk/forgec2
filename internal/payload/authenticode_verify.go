package payload

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

// ErrPENotSigned is returned when a PE has no registered certificate table.
var ErrPENotSigned = errors.New("PE is not signed")

// PESignatureInfo is the operator-facing view of an embedded Authenticode
// signature. It describes the signer and the image digest without asserting
// trust; use VerifyPESignature for cryptographic validation.
type PESignatureInfo struct {
	Signer      *x509.Certificate
	Chain       []*x509.Certificate
	SigningTime time.Time
	Digest      []byte // PE image digest stored in SpcIndirectDataContent
}

type parsedPESignature struct {
	info            *PESignatureInfo
	contentDER      []byte
	attrsBody       []byte
	messageDigest   []byte
	contentType     asn1.ObjectIdentifier
	digestAlgorithm asn1.ObjectIdentifier
	encryptionAlg   asn1.ObjectIdentifier
	encryptedDigest []byte
}

// InspectPESignature parses the Authenticode certificate table (if any) and
// returns the signer, embedded chain, signing time and image digest. It does
// not verify the signature; call VerifyPESignature for that.
func InspectPESignature(peBytes []byte) (*PESignatureInfo, error) {
	parsed, err := parsePESignature(peBytes)
	if err != nil {
		return nil, err
	}
	return parsed.info, nil
}

// VerifyPESignature validates the embedded PKCS#7 signature against the PE
// bytes and checks the signing certificate's code-signing suitability. It does
// not build a trust path to a CA — a self-signed certificate is expected to be
// untrusted by Windows, but its signature must still be internally consistent.
func VerifyPESignature(peBytes []byte) error {
	parsed, err := parsePESignature(peBytes)
	if err != nil {
		return err
	}
	info := parsed.info
	if err := validateCodeSigningCertificate(info.Signer); err != nil {
		return fmt.Errorf("authenticode: %w", err)
	}
	if !parsed.digestAlgorithm.Equal(oidSHA256) {
		return fmt.Errorf("authenticode: unsupported digest algorithm %s", parsed.digestAlgorithm)
	}
	if !parsed.encryptionAlg.Equal(oidRSAEncryption) && !parsed.encryptionAlg.Equal(oidSHA256WithRSA) {
		return fmt.Errorf("authenticode: unsupported signature algorithm %s", parsed.encryptionAlg)
	}

	pub, ok := info.Signer.PublicKey.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("authenticode: signer key is %T, not RSA", info.Signer.PublicKey)
	}
	attrsDigest := sha256.Sum256(derTLV(0x31, parsed.attrsBody))
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, attrsDigest[:], parsed.encryptedDigest); err != nil {
		return fmt.Errorf("authenticode: signature does not verify: %w", err)
	}

	contentDigest, err := spcContentDigest(parsed.contentDER)
	if err != nil {
		return fmt.Errorf("authenticode: %w", err)
	}
	if !bytes.Equal(parsed.messageDigest, contentDigest[:]) {
		return errors.New("authenticode: messageDigest attribute does not match SpcIndirectDataContent")
	}
	if !parsed.contentType.Equal(oidSPCIndirectData) {
		return fmt.Errorf("authenticode: unexpected content type %s", parsed.contentType)
	}

	want, err := AuthenticodeDigest(peBytes)
	if err != nil {
		return fmt.Errorf("authenticode: %w", err)
	}
	if !bytes.Equal(info.Digest, want) {
		return errors.New("authenticode: image digest does not match the signed digest")
	}
	return nil
}

// PESignatureStatus is the operator-facing classification of an embedded
// Authenticode signature. Trust is separate from cryptographic validity: a
// self-signed signature verifies cryptographically but has no trusted root.
type PESignatureStatus string

const (
	PESignatureUnsigned      PESignatureStatus = "unsigned"
	PESignatureValid         PESignatureStatus = "valid"
	PESignatureUntrustedRoot PESignatureStatus = "untrusted_root"
	PESignatureInvalid       PESignatureStatus = "invalid"
)

// PESignatureReport describes an Authenticode signature for build status/API
// output. SignerSubject and SignerIssuer are distinguished names; they are
// equal for a self-signed certificate.
type PESignatureReport struct {
	Status        PESignatureStatus `json:"status"`
	SignerSubject string            `json:"signer_subject,omitempty"`
	SignerIssuer  string            `json:"signer_issuer,omitempty"`
	SigningTime   time.Time         `json:"signing_time,omitempty"`
	Error         string            `json:"error,omitempty"`
}

// ClassifyPESignature reports whether a PE is unsigned, carries a valid
// signature chaining to a trusted root, carries a cryptographically valid
// signature with an untrusted root (including self-signed), or is invalid.
//
// roots may be nil to use the host trust store; tests pass an explicit pool to
// exercise the trusted path without modifying the host. Signature
// cryptographic validation is performed first, so an invalid/tampered image is
// never reported as merely untrusted. Non-Windows targets do not call this.
func ClassifyPESignature(peBytes []byte, roots *x509.CertPool) PESignatureReport {
	report := PESignatureReport{Status: PESignatureInvalid}
	info, err := InspectPESignature(peBytes)
	if err != nil {
		if errors.Is(err, ErrPENotSigned) {
			report.Status = PESignatureUnsigned
			return report
		}
		report.Error = err.Error()
		return report
	}
	report.SignerSubject = info.Signer.Subject.String()
	report.SignerIssuer = info.Signer.Issuer.String()
	report.SigningTime = info.SigningTime

	if err := VerifyPESignature(peBytes); err != nil {
		report.Error = err.Error()
		return report
	}
	if roots == nil {
		systemRoots, err := x509.SystemCertPool()
		if err != nil {
			report.Status = PESignatureUntrustedRoot
			report.Error = "system certificate pool unavailable: " + err.Error()
			return report
		}
		roots = systemRoots
	}
	intermediates := x509.NewCertPool()
	for _, cert := range info.Chain {
		if cert != nil && !bytes.Equal(cert.Raw, info.Signer.Raw) {
			intermediates.AddCert(cert)
		}
	}
	if _, err := info.Signer.Verify(x509.VerifyOptions{
		Roots:         roots,
		Intermediates: intermediates,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
	}); err != nil {
		var unknown x509.UnknownAuthorityError
		if errors.As(err, &unknown) {
			report.Status = PESignatureUntrustedRoot
		} else {
			report.Status = PESignatureInvalid
		}
		report.Error = err.Error()
		return report
	}
	report.Status = PESignatureValid
	return report
}

func parsePESignature(peBytes []byte) (*parsedPESignature, error) {
	_, secDirOff, err := peSecurityFields(peBytes)
	if err != nil {
		return nil, fmt.Errorf("authenticode: %w", err)
	}
	tableOff := uint64(binary.LittleEndian.Uint32(peBytes[secDirOff:]))
	tableSize := uint64(binary.LittleEndian.Uint32(peBytes[secDirOff+4:]))
	if tableOff == 0 || tableSize == 0 {
		return nil, ErrPENotSigned
	}
	if tableOff+tableSize > uint64(len(peBytes)) {
		return nil, errors.New("authenticode: certificate table is outside the file")
	}
	if tableSize < 8 {
		return nil, errors.New("authenticode: truncated WIN_CERTIFICATE")
	}
	length := uint64(binary.LittleEndian.Uint32(peBytes[tableOff:]))
	revision := binary.LittleEndian.Uint16(peBytes[tableOff+4:])
	certType := binary.LittleEndian.Uint16(peBytes[tableOff+6:])
	if length < 8 || tableOff+length > tableOff+tableSize {
		return nil, fmt.Errorf("authenticode: invalid WIN_CERTIFICATE length %d", length)
	}
	if revision != winCertRevisionV2 {
		return nil, fmt.Errorf("authenticode: unsupported WIN_CERTIFICATE revision 0x%04x", revision)
	}
	if certType != winCertTypePKCSSignedData {
		return nil, fmt.Errorf("authenticode: unsupported WIN_CERTIFICATE type 0x%04x", certType)
	}
	pkcs7 := peBytes[tableOff+8 : tableOff+length]

	var ci contentInfo
	if _, err := asn1.Unmarshal(pkcs7, &ci); err != nil {
		return nil, fmt.Errorf("authenticode: parse ContentInfo: %w", err)
	}
	if !ci.ContentType.Equal(oidSignedData) {
		return nil, fmt.Errorf("authenticode: unexpected ContentInfo type %s", ci.ContentType)
	}
	if ci.Content.Class != asn1.ClassContextSpecific || ci.Content.Tag != 0 {
		return nil, errors.New("authenticode: SignedData is not [0] EXPLICIT")
	}
	var sd signedData
	if _, err := asn1.Unmarshal(ci.Content.Bytes, &sd); err != nil {
		return nil, fmt.Errorf("authenticode: parse SignedData: %w", err)
	}
	if len(sd.SignerInfos) == 0 {
		return nil, errors.New("authenticode: SignedData has no signer")
	}
	if len(sd.SignerInfos) > 1 {
		return nil, errors.New("authenticode: multiple signers are not supported")
	}
	si := sd.SignerInfos[0]
	if !sd.ContentInfo.ContentType.Equal(oidSPCIndirectData) {
		return nil, fmt.Errorf("authenticode: unexpected encapsulated content type %s", sd.ContentInfo.ContentType)
	}
	if sd.ContentInfo.Content.Class != asn1.ClassContextSpecific || sd.ContentInfo.Content.Tag != 0 {
		return nil, errors.New("authenticode: encapsulated content is not [0] EXPLICIT")
	}
	var content spcIndirectDataContent
	if _, err := asn1.Unmarshal(sd.ContentInfo.Content.Bytes, &content); err != nil {
		return nil, fmt.Errorf("authenticode: parse SpcIndirectDataContent: %w", err)
	}
	if !content.Data.Type.Equal(oidSPCPEImageData) {
		return nil, fmt.Errorf("authenticode: unexpected SPC content type %s", content.Data.Type)
	}
	if !content.MessageDigest.Algorithm.Algorithm.Equal(oidSHA256) {
		return nil, fmt.Errorf("authenticode: unsupported image digest algorithm %s", content.MessageDigest.Algorithm.Algorithm)
	}
	if len(content.MessageDigest.Digest) != sha256.Size {
		return nil, fmt.Errorf("authenticode: image digest length %d, want %d", len(content.MessageDigest.Digest), sha256.Size)
	}

	certs, err := parseCertificateSet(sd.Certificates)
	if err != nil {
		return nil, err
	}
	var signer *x509.Certificate
	for _, cert := range certs {
		if cert.SerialNumber != nil && si.IssuerAndSerialNumber.SerialNumber != nil &&
			cert.SerialNumber.Cmp(si.IssuerAndSerialNumber.SerialNumber) == 0 &&
			bytes.Equal(cert.RawIssuer, si.IssuerAndSerialNumber.IssuerName.FullBytes) {
			signer = cert
			break
		}
	}
	if signer == nil {
		return nil, errors.New("authenticode: signer certificate is missing from the PKCS#7 certificate set")
	}

	var attrs []pkcs7Attribute
	// The authenticated attributes are [0] IMPLICIT SET; the SET body is a
	// concatenation of attribute TLVs. encoding/asn1 expects a universal
	// SEQUENCE when unmarshalling into a slice, so wrap the body in a
	// synthetic SEQUENCE for parsing only. The signature itself remains over
	// the universal SET encoding.
	if _, err := asn1.Unmarshal(derTLV(0x30, si.AuthenticatedAttributes.Bytes), &attrs); err != nil {
		return nil, fmt.Errorf("authenticode: parse authenticated attributes: %w", err)
	}
	var (
		messageDigest []byte
		contentType   asn1.ObjectIdentifier
		signingTime   time.Time
	)
	for _, attr := range attrs {
		if len(attr.Value) == 0 {
			continue
		}
		switch {
		case attr.Type.Equal(oidMessageDigest):
			if attr.Value[0].Tag != asn1.TagOctetString {
				return nil, errors.New("authenticode: messageDigest attribute is not an OCTET STRING")
			}
			messageDigest = append([]byte(nil), attr.Value[0].Bytes...)
		case attr.Type.Equal(oidContentType):
			var oid asn1.ObjectIdentifier
			if _, err := asn1.Unmarshal(attr.Value[0].FullBytes, &oid); err != nil {
				return nil, fmt.Errorf("authenticode: parse contentType attribute: %w", err)
			}
			contentType = oid
		case attr.Type.Equal(oidSigningTime):
			var t time.Time
			if _, err := asn1.Unmarshal(attr.Value[0].FullBytes, &t); err == nil {
				signingTime = t
			}
		}
	}
	if messageDigest == nil {
		return nil, errors.New("authenticode: SignedData is missing the messageDigest attribute")
	}
	if contentType == nil {
		return nil, errors.New("authenticode: SignedData is missing the contentType attribute")
	}

	return &parsedPESignature{
		info: &PESignatureInfo{
			Signer:      signer,
			Chain:       certs,
			SigningTime: signingTime,
			Digest:      append([]byte(nil), content.MessageDigest.Digest...),
		},
		contentDER:      append([]byte(nil), sd.ContentInfo.Content.Bytes...),
		attrsBody:       append([]byte(nil), si.AuthenticatedAttributes.Bytes...),
		messageDigest:   messageDigest,
		contentType:     contentType,
		digestAlgorithm: si.DigestAlgorithm.Algorithm,
		encryptionAlg:   si.DigestEncryptionAlgorithm.Algorithm,
		encryptedDigest: append([]byte(nil), si.EncryptedDigest...),
	}, nil
}

func parseCertificateSet(raw asn1.RawValue) ([]*x509.Certificate, error) {
	if len(raw.Bytes) == 0 {
		return nil, errors.New("authenticode: PKCS#7 certificate set is empty")
	}
	var certs []*x509.Certificate
	rest := raw.Bytes
	for len(rest) > 0 {
		var value asn1.RawValue
		var err error
		rest, err = asn1.Unmarshal(rest, &value)
		if err != nil {
			return nil, fmt.Errorf("authenticode: parse certificate: %w", err)
		}
		cert, err := x509.ParseCertificate(value.FullBytes)
		if err != nil {
			return nil, fmt.Errorf("authenticode: parse certificate: %w", err)
		}
		certs = append(certs, cert)
	}
	return certs, nil
}
