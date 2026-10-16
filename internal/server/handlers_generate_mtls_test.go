package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// testMTLSPair returns a throwaway self-signed certificate/key PEM pair for
// exercising the generation form's mTLS validation.
func testMTLSPair(t *testing.T) (certPEM, keyPEM string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "generate-mtls-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})),
		string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))
}

func mtlsForm(cert, key, ca string) url.Values {
	form := url.Values{
		"c2_url":     {"https://127.0.0.1:8443"},
		"protocol":   {"https"},
		"interval":   {"60"},
		"dns_domain": {"c2.example.com"},
	}
	if cert != "" || key != "" || ca != "" {
		form.Set("mtls_cert", cert)
		form.Set("mtls_key", key)
		form.Set("mtls_ca", ca)
	}
	return form
}

// TestParseBinaryFormRejectsInvalidMTLSMaterial pins the API-boundary refusal:
// material that could never complete a handshake must answer 400 before any
// build work starts, mirroring the per-listener TLS validation.
func TestParseBinaryFormRejectsInvalidMTLSMaterial(t *testing.T) {
	s := mustTenantServer(t)
	cert, key := testMTLSPair(t)
	_, otherKey := testMTLSPair(t)

	cases := []struct {
		name string
		form url.Values
	}{
		{"key without certificate", mtlsForm("", key, "")},
		{"certificate without key", mtlsForm(cert, "", "")},
		{"mismatched pair", mtlsForm(cert, otherKey, "")},
		{"garbage material", mtlsForm("not a pem", "not a key", "")},
		{"garbage CA", mtlsForm(cert, key, "not a ca bundle")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, w := tenantCtx(http.MethodPost, "/api/generate/exe", "mtls-op", tc.form.Encode())
			c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			parsed, ok := s.parseBinaryForm(c)
			if ok || parsed != nil {
				t.Fatal("invalid mTLS material was accepted")
			}
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), "mTLS") {
				t.Fatalf("refusal does not name the mTLS material: %s", w.Body.String())
			}
		})
	}
}

// TestParseBinaryFormAcceptsAndTrimsMTLSMaterial covers the happy paths: no
// material, a bare pair, and a pair pasted with surrounding whitespace (the
// common case when copying from a PEM file) which must be normalized.
func TestParseBinaryFormAcceptsAndTrimsMTLSMaterial(t *testing.T) {
	s := mustTenantServer(t)
	cert, key := testMTLSPair(t)
	// The form layer trims surrounding whitespace, and a PEM block ends with a
	// newline, so the expected values are the trimmed originals.
	wantCert, wantKey := strings.TrimSpace(cert), strings.TrimSpace(key)

	c, w := tenantCtx(http.MethodPost, "/api/generate/exe", "mtls-op", mtlsForm("", "", "").Encode())
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if parsed, ok := s.parseBinaryForm(c); !ok || parsed.MTLSCert != "" || parsed.MTLSKey != "" {
		t.Fatalf("material-free form rejected: %d %s", w.Code, w.Body.String())
	}

	c, w = tenantCtx(http.MethodPost, "/api/generate/exe", "mtls-op", mtlsForm(cert, key, cert).Encode())
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if parsed, ok := s.parseBinaryForm(c); !ok || parsed.MTLSCert != wantCert || parsed.MTLSKey != wantKey || parsed.MTLSCA != wantCert {
		t.Fatalf("valid pair+CA rejected: %d %s", w.Code, w.Body.String())
	}

	c, w = tenantCtx(http.MethodPost, "/api/generate/exe", "mtls-op", mtlsForm("\n  "+cert+"\n", key+"  \n", "").Encode())
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	parsed, ok := s.parseBinaryForm(c)
	if !ok {
		t.Fatalf("whitespace-padded material rejected: %d %s", w.Code, w.Body.String())
	}
	if parsed.MTLSCert != wantCert || parsed.MTLSKey != wantKey {
		t.Fatal("surrounding whitespace was not trimmed")
	}
}

// TestBuildImplantConfigCarriesMTLSMaterial ties the parsed form to the
// generator config that feeds the config blob.
func TestBuildImplantConfigCarriesMTLSMaterial(t *testing.T) {
	s := dllServer(t)
	cert, key := testMTLSPair(t)

	cfg, err := s.buildImplantConfig(&binaryGenForm{
		C2URL:    "https://127.0.0.1:8443",
		Protocol: "https",
		MTLSCert: cert,
		MTLSKey:  key,
		MTLSCA:   cert,
	})
	if err != nil {
		t.Fatalf("buildImplantConfig: %v", err)
	}
	if cfg.MTLSCertPEM != cert || cfg.MTLSKeyPEM != key || cfg.MTLSCAPEM != cert {
		t.Fatal("mTLS material was not carried into the implant config")
	}
}
