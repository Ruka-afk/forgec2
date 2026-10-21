package payload

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

// Authenticode signing material handling.
//
// PFX and PEM inputs are per-build and live only in memory: they are parsed,
// embedded into the PKCS#7 signature, and never written to disk, logged, or
// included in audit records. Error messages deliberately describe the material
// (type, password mismatch) without echoing it.

// validateCodeSigningCertificate enforces the certificate properties that
// matter for Authenticode. It does not need the private key, so verification
// can reuse it.
func validateCodeSigningCertificate(cert *x509.Certificate) error {
	if cert == nil {
		return errors.New("missing signing certificate")
	}
	pub, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("certificate public key is %T; only RSA keys are supported", cert.PublicKey)
	}
	if pub.N == nil || pub.N.BitLen() < 2048 {
		return fmt.Errorf("RSA key is too small (%d bits); use at least 2048", pub.N.BitLen())
	}
	const clockSkew = 5 * time.Minute
	now := time.Now()
	if now.Add(clockSkew).Before(cert.NotBefore) {
		return fmt.Errorf("certificate is not valid before %s", cert.NotBefore.UTC().Format(time.RFC3339))
	}
	if now.Add(-clockSkew).After(cert.NotAfter) {
		return fmt.Errorf("certificate expired on %s", cert.NotAfter.UTC().Format(time.RFC3339))
	}
	if cert.KeyUsage != 0 && cert.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		return errors.New("certificate KeyUsage does not include digitalSignature")
	}
	codeSigning := false
	for _, eku := range cert.ExtKeyUsage {
		if eku == x509.ExtKeyUsageCodeSigning {
			codeSigning = true
			break
		}
	}
	if !codeSigning {
		return errors.New("certificate is missing the Code Signing extended key usage")
	}
	return nil
}

// validateCodeSigningCert additionally proves the private key belongs to the
// certificate before we use it to sign.
func validateCodeSigningCert(cert *x509.Certificate, key *rsa.PrivateKey) error {
	if err := validateCodeSigningCertificate(cert); err != nil {
		return err
	}
	if key == nil {
		return errors.New("missing RSA private key")
	}
	pub, _ := cert.PublicKey.(*rsa.PublicKey)
	if key.PublicKey.N == nil || key.PublicKey.N.Cmp(pub.N) != 0 || key.PublicKey.E != pub.E {
		return errors.New("private key does not match the certificate public key")
	}
	if err := key.Validate(); err != nil {
		return fmt.Errorf("invalid RSA private key: %w", err)
	}
	return nil
}

// loadPFXChain decodes a PKCS#12 bundle into the leaf certificate, any
// intermediate certificates bundled with it, and the RSA private key.
func loadPFXChain(pfxData []byte, password string) ([]*x509.Certificate, *rsa.PrivateKey, error) {
	if len(pfxData) == 0 {
		return nil, nil, errors.New("PFX data is empty")
	}
	keyAny, leaf, cas, err := pkcs12.DecodeChain(pfxData, password)
	if err != nil {
		return nil, nil, fmt.Errorf("parse PFX (wrong password or unsupported encryption): %w", err)
	}
	if leaf == nil {
		return nil, nil, errors.New("PFX does not contain a signing certificate")
	}
	key, ok := keyAny.(*rsa.PrivateKey)
	if !ok {
		return nil, nil, fmt.Errorf("PFX private key is %T; only RSA keys are supported", keyAny)
	}
	if err := validateCodeSigningCert(leaf, key); err != nil {
		return nil, nil, fmt.Errorf("PFX certificate: %w", err)
	}
	chain := make([]*x509.Certificate, 0, 1+len(cas))
	chain = append(chain, leaf)
	chain = append(chain, cas...)
	return chain, key, nil
}

// loadPEMChain decodes a PEM certificate chain and RSA private key. PKCS#8 and
// PKCS#1 private keys are supported; EC keys are rejected explicitly rather
// than producing an RSA-only PKCS#7 that Windows cannot verify.
func loadPEMChain(certPEM, keyPEM []byte) ([]*x509.Certificate, *rsa.PrivateKey, error) {
	if len(certPEM) == 0 {
		return nil, nil, errors.New("PEM certificate data is empty")
	}
	if len(keyPEM) == 0 {
		return nil, nil, errors.New("PEM private key data is empty")
	}

	var chain []*x509.Certificate
	rest := certPEM
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, nil, fmt.Errorf("parse PEM certificate: %w", err)
		}
		chain = append(chain, cert)
	}
	if len(chain) == 0 {
		return nil, nil, errors.New("no CERTIFICATE block found in PEM data")
	}

	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return nil, nil, errors.New("no PEM block found in private key data")
	}
	var keyAny any
	var err error
	switch block.Type {
	case "PRIVATE KEY":
		keyAny, err = x509.ParsePKCS8PrivateKey(block.Bytes)
	case "RSA PRIVATE KEY":
		keyAny, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	case "EC PRIVATE KEY":
		return nil, nil, errors.New("ECDSA private keys are not supported yet; use an RSA key")
	default:
		return nil, nil, fmt.Errorf("unsupported PEM private key type %q; use PKCS#8 or PKCS#1 RSA", block.Type)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("parse PEM private key: %w", err)
	}
	key, ok := keyAny.(*rsa.PrivateKey)
	if !ok {
		return nil, nil, fmt.Errorf("private key is %T; only RSA keys are supported", keyAny)
	}
	if err := validateCodeSigningCert(chain[0], key); err != nil {
		return nil, nil, fmt.Errorf("PEM certificate: %w", err)
	}
	return chain, key, nil
}

// SignPE dispatches on cfg.SignMode and returns a signed copy of pe.
func SignPE(pe []byte, cfg ImplantConfig) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.SignMode)) {
	case "self":
		return SignPESelfSigned(pe, cfg.SignOrg)
	case "pfx":
		chain, key, err := loadPFXChain(cfg.SignPFX, cfg.SignPFXPassword)
		if err != nil {
			return nil, err
		}
		return SignPEWithChain(pe, chain, key)
	case "pem":
		chain, key, err := loadPEMChain(cfg.SignCertPEM, cfg.SignKeyPEM)
		if err != nil {
			return nil, err
		}
		return SignPEWithChain(pe, chain, key)
	default:
		return nil, fmt.Errorf("unsupported sign mode %q", cfg.SignMode)
	}
}

// validateSigningMaterial parses the selected signing material so a bad
// password or unusable certificate fails preflight instead of after an
// expensive compile. Self-signed mode has no operator material to validate.
func validateSigningMaterial(cfg ImplantConfig) error {
	switch strings.ToLower(strings.TrimSpace(cfg.SignMode)) {
	case "self":
		return nil
	case "pfx":
		_, _, err := loadPFXChain(cfg.SignPFX, cfg.SignPFXPassword)
		return err
	case "pem":
		_, _, err := loadPEMChain(cfg.SignCertPEM, cfg.SignKeyPEM)
		return err
	default:
		return fmt.Errorf("unsupported sign mode %q", cfg.SignMode)
	}
}

// signPEIfRequested signs a finalized PE in place. It is a no-op when signing
// was not requested. The file mode is preserved so the later atomic rename
// does not turn an executable into a non-executable file.
func signPEIfRequested(path string, cfg ImplantConfig) error {
	if !cfg.CodeSigningEnabled() {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read PE for signing: %w", err)
	}
	signed, err := SignPE(data, cfg)
	if err != nil {
		return err
	}
	mode := os.FileMode(0755)
	if info, statErr := os.Stat(path); statErr == nil {
		mode = info.Mode().Perm()
	}
	if err := os.WriteFile(path, signed, mode); err != nil {
		return fmt.Errorf("write signed PE: %w", err)
	}
	return nil
}
