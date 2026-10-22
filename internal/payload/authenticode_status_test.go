package payload

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"
)

func makeTrustedTestChain(t *testing.T) (*x509.Certificate, *x509.Certificate, *rsa.PrivateKey) {
	t.Helper()
	now := time.Now()
	rootKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	rootTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "ForgeC2 Test Root", Organization: []string{"ForgeC2 Tests"}},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTmpl, rootTmpl, &rootKey.PublicKey, rootKey)
	if err != nil {
		t.Fatal(err)
	}
	root, err := x509.ParseCertificate(rootDER)
	if err != nil {
		t.Fatal(err)
	}

	leafKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	leafTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(2),
		Subject:               pkix.Name{CommonName: "ForgeC2 Test Signer", Organization: []string{"ForgeC2 Tests"}},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		BasicConstraintsValid: true,
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, root, &leafKey.PublicKey, rootKey)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(leafDER)
	if err != nil {
		t.Fatal(err)
	}
	return leaf, root, leafKey
}

func TestClassifyPESignature(t *testing.T) {
	pe := makePE(t, 0x8664, true)

	if got := ClassifyPESignature(pe, nil); got.Status != PESignatureUnsigned {
		t.Fatalf("unsigned status = %q, want %q", got.Status, PESignatureUnsigned)
	}

	selfSigned, err := SignPESelfSigned(pe, "Self Test")
	if err != nil {
		t.Fatal(err)
	}
	got := ClassifyPESignature(selfSigned, nil)
	if got.Status != PESignatureUntrustedRoot {
		t.Fatalf("self-signed status = %q (%s), want %q", got.Status, got.Error, PESignatureUntrustedRoot)
	}
	if got.SignerSubject == "" || got.SignerSubject != got.SignerIssuer {
		t.Fatalf("self-signed report subjects = %q / %q, want equal non-empty values", got.SignerSubject, got.SignerIssuer)
	}

	tampered := append([]byte(nil), selfSigned...)
	tampered[testPETextRawOff] ^= 0xFF
	if got := ClassifyPESignature(tampered, nil); got.Status != PESignatureInvalid {
		t.Fatalf("tampered status = %q (%s), want %q", got.Status, got.Error, PESignatureInvalid)
	}

	leaf, root, leafKey := makeTrustedTestChain(t)
	trusted, err := SignPEWithChain(pe, []*x509.Certificate{leaf, root}, leafKey)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(root)
	got = ClassifyPESignature(trusted, roots)
	if got.Status != PESignatureValid {
		t.Fatalf("trusted status = %q (%s), want %q", got.Status, got.Error, PESignatureValid)
	}
	if got.SignerSubject == "" || got.SignerIssuer == "" {
		t.Fatal("trusted report is missing signer/issuer")
	}

	if got := ClassifyPESignature(trusted, nil); got.Status != PESignatureUntrustedRoot {
		t.Fatalf("test root without explicit pool status = %q (%s), want %q", got.Status, got.Error, PESignatureUntrustedRoot)
	}
}
