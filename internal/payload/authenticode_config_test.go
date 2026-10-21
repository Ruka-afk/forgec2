package payload

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

type testCertOptions struct {
	bits       int
	keyUsage   x509.KeyUsage
	eku        []x509.ExtKeyUsage
	notBefore  time.Time
	notAfter   time.Time
	subjectCN  string
	subjectOrg string
}

func makeTestCodeSigningCert(t *testing.T, opts testCertOptions) (*x509.Certificate, *rsa.PrivateKey) {
	t.Helper()
	if opts.bits == 0 {
		opts.bits = 2048
	}
	if opts.notBefore.IsZero() {
		opts.notBefore = time.Now().Add(-time.Hour)
	}
	if opts.notAfter.IsZero() {
		opts.notAfter = time.Now().Add(24 * time.Hour)
	}
	if opts.keyUsage == 0 {
		opts.keyUsage = x509.KeyUsageDigitalSignature
	}
	if opts.eku == nil {
		opts.eku = []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning}
	}
	if opts.subjectCN == "" {
		opts.subjectCN = "Test Code Signer"
	}
	if opts.subjectOrg == "" {
		opts.subjectOrg = "Test Org"
	}
	key, err := rsa.GenerateKey(rand.Reader, opts.bits)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: opts.subjectCN, Organization: []string{opts.subjectOrg}},
		NotBefore:             opts.notBefore,
		NotAfter:              opts.notAfter,
		KeyUsage:              opts.keyUsage,
		ExtKeyUsage:           opts.eku,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}
	return cert, key
}

func encodeTestPFX(t *testing.T, cert *x509.Certificate, key *rsa.PrivateKey, password string) []byte {
	t.Helper()
	pfx, err := pkcs12.Modern2023.Encode(key, cert, nil, password)
	if err != nil {
		t.Fatalf("encode PFX: %v", err)
	}
	return pfx
}

func encodeTestPEM(t *testing.T, cert *x509.Certificate, key *rsa.PrivateKey) (certPEM, keyPEM []byte) {
	t.Helper()
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal PKCS#8 key: %v", err)
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	return certPEM, keyPEM
}

func TestSignPEPFXAndPEM(t *testing.T) {
	cert, key := makeTestCodeSigningCert(t, testCertOptions{})
	pfx := encodeTestPFX(t, cert, key, "s3cret")
	certPEM, keyPEM := encodeTestPEM(t, cert, key)
	pe := makePE(t, 0x8664, true)

	for _, tc := range []struct {
		name string
		cfg  ImplantConfig
	}{
		{"pfx", ImplantConfig{SignMode: "pfx", SignPFX: pfx, SignPFXPassword: "s3cret"}},
		{"pem", ImplantConfig{SignMode: "pem", SignCertPEM: certPEM, SignKeyPEM: keyPEM}},
		{"self", ImplantConfig{SignMode: "self", SignOrg: "Neutral Org"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			signed, err := SignPE(pe, tc.cfg)
			if err != nil {
				t.Fatalf("SignPE: %v", err)
			}
			info, err := InspectPESignature(signed)
			if err != nil {
				t.Fatalf("InspectPESignature: %v", err)
			}
			if info.Signer == nil || len(info.Digest) != 32 {
				t.Fatalf("incomplete signature info: %+v", info)
			}
			if info.SigningTime.IsZero() {
				t.Fatal("signing time was not recorded")
			}
			if err := VerifyPESignature(signed); err != nil {
				t.Fatalf("VerifyPESignature: %v", err)
			}
			path := writeTestPE(t, signed)
			if err := validatePEWith(path, peExpect{
				Machine:             0x8664,
				Signed:              true,
				RequireDebugCleared: true,
			}); err != nil {
				t.Fatalf("validatePEWith: %v", err)
			}
		})
	}
}

func TestSignPEBadMaterial(t *testing.T) {
	goodCert, goodKey := makeTestCodeSigningCert(t, testCertOptions{})
	goodPFX := encodeTestPFX(t, goodCert, goodKey, "correct")
	serverCert, serverKey := makeTestCodeSigningCert(t, testCertOptions{
		eku: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	})
	serverPFX := encodeTestPFX(t, serverCert, serverKey, "pw")
	expiredCert, expiredKey := makeTestCodeSigningCert(t, testCertOptions{
		notBefore: time.Now().Add(-48 * time.Hour),
		notAfter:  time.Now().Add(-24 * time.Hour),
	})
	expiredPFX := encodeTestPFX(t, expiredCert, expiredKey, "pw")
	smallCert, smallKey := makeTestCodeSigningCert(t, testCertOptions{bits: 1024})
	smallPFX := encodeTestPFX(t, smallCert, smallKey, "pw")

	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ecDER, err := x509.MarshalECPrivateKey(ecKey)
	if err != nil {
		t.Fatal(err)
	}
	certPEM, _ := encodeTestPEM(t, goodCert, goodKey)
	ecKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: ecDER})

	pe := makePE(t, 0x8664, true)
	tests := []struct {
		name    string
		cfg     ImplantConfig
		wantSub string
	}{
		{"pfx empty", ImplantConfig{SignMode: "pfx"}, "empty"},
		{"pfx wrong password", ImplantConfig{SignMode: "pfx", SignPFX: goodPFX, SignPFXPassword: "wrong"}, "password"},
		{"pfx missing EKU", ImplantConfig{SignMode: "pfx", SignPFX: serverPFX, SignPFXPassword: "pw"}, "Code Signing"},
		{"pfx expired", ImplantConfig{SignMode: "pfx", SignPFX: expiredPFX, SignPFXPassword: "pw"}, "expired"},
		{"pfx small key", ImplantConfig{SignMode: "pfx", SignPFX: smallPFX, SignPFXPassword: "pw"}, "too small"},
		{"pem empty key", ImplantConfig{SignMode: "pem", SignCertPEM: certPEM}, "empty"},
		{"pem ecdsa", ImplantConfig{SignMode: "pem", SignCertPEM: certPEM, SignKeyPEM: ecKeyPEM}, "ECDSA"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := SignPE(pe, tc.cfg)
			if err == nil {
				t.Fatal("expected signing error")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("error %q does not mention %q", err, tc.wantSub)
			}
		})
	}
}

func TestInspectPESignatureUnsignedAndTampered(t *testing.T) {
	pe := makePE(t, 0x8664, true)
	if _, err := InspectPESignature(pe); !errors.Is(err, ErrPENotSigned) {
		t.Fatalf("unsigned InspectPESignature error = %v, want ErrPENotSigned", err)
	}
	if err := VerifyPESignature(pe); !errors.Is(err, ErrPENotSigned) {
		t.Fatalf("unsigned VerifyPESignature error = %v, want ErrPENotSigned", err)
	}

	signed, err := SignPESelfSigned(pe, "Tamper Test")
	if err != nil {
		t.Fatal(err)
	}
	tampered := append([]byte(nil), signed...)
	tampered[testPETextRawOff] ^= 0xFF
	if err := VerifyPESignature(tampered); err == nil {
		t.Fatal("tampered image passed signature verification")
	}
	if _, err := InspectPESignature(tampered); err != nil {
		t.Fatalf("inspection should not require a valid signature: %v", err)
	}
}

func TestValidateSigningMaterialPreflight(t *testing.T) {
	cert, key := makeTestCodeSigningCert(t, testCertOptions{})
	pfx := encodeTestPFX(t, cert, key, "pw")
	cfg := &ImplantConfig{Architecture: "amd64", SignMode: "pfx", SignPFX: pfx, SignPFXPassword: "pw"}
	if err := ValidateImplantBuild("windows", "exe", cfg); err != nil {
		t.Fatalf("valid PFX rejected: %v", err)
	}
	cfg.SignPFXPassword = "wrong"
	if err := ValidateImplantBuild("windows", "exe", cfg); err == nil {
		t.Fatal("wrong PFX password passed preflight")
	}
}
