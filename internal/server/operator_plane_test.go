package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/gin-gonic/gin"
)

func TestOperatorIPAllowed(t *testing.T) {
	if !operatorIPAllowed(nil, "8.8.8.8") {
		t.Fatalf("empty allowlist must allow")
	}
	allow := []string{"10.0.0.0/8", "203.0.113.7", "::1"}
	for _, ip := range []string{"10.1.2.3", "203.0.113.7", "::1"} {
		if !operatorIPAllowed(allow, ip) {
			t.Fatalf("member %s denied", ip)
		}
	}
	for _, ip := range []string{"192.168.1.1", "203.0.113.8", "not-an-ip", ""} {
		if operatorIPAllowed(allow, ip) {
			t.Fatalf("non-member %q allowed", ip)
		}
	}
}

func planeTestServer(t *testing.T) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	return &Server{cfg: &config.Config{}}
}

func planeCtx(remoteAddr string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/api/test", nil)
	c.Request.RemoteAddr = remoteAddr
	return c, w
}

func TestOperatorPlaneGuardCIDR(t *testing.T) {
	s := planeTestServer(t)
	s.cfg.Server.OperatorAllowedCIDRs = []string{"10.0.0.0/8"}

	c, w := planeCtx("192.168.1.9:4444")
	s.operatorPlaneGuard()(c)
	if !c.IsAborted() || w.Code != http.StatusForbidden {
		t.Fatalf("outsider not denied: aborted=%v code=%d", c.IsAborted(), w.Code)
	}

	c, w = planeCtx("10.9.9.9:4444")
	s.operatorPlaneGuard()(c)
	if c.IsAborted() {
		t.Fatalf("member denied: code=%d body=%s", w.Code, w.Body.String())
	}

	// Disabled allowlist: open.
	s.cfg.Server.OperatorAllowedCIDRs = nil
	c, _ = planeCtx("192.168.1.9:4444")
	s.operatorPlaneGuard()(c)
	if c.IsAborted() {
		t.Fatalf("disabled guard denied")
	}
}

// issueTestClientCert builds a CA + client cert pair, writes the CA bundle
// to dir, and returns the client cert for PeerCertificates injection.
func issueTestClientCert(t *testing.T, dir string) *x509.Certificate {
	t.Helper()
	caKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	caTpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test-op-ca"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTpl, caTpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create CA: %v", err)
	}
	caCert, _ := x509.ParseCertificate(caDER)
	caPath := filepath.Join(dir, "op-ca.crt")
	if err := os.WriteFile(caPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0600); err != nil {
		t.Fatalf("write CA: %v", err)
	}
	leafKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	leafTpl := &x509.Certificate{
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "alice"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		DNSNames: []string{"alice"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTpl, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create leaf: %v", err)
	}
	leaf, _ := x509.ParseCertificate(leafDER)
	return leaf
}

func TestOperatorPlaneGuardMTLS(t *testing.T) {
	s := planeTestServer(t)
	s.cfg.Server.OperatorMTLS = true
	s.cfg.Server.OperatorClientCAFile = filepath.Join(t.TempDir(), "op-ca.crt")
	leaf := issueTestClientCert(t, filepath.Dir(s.cfg.Server.OperatorClientCAFile))

	// Valid client cert passes.
	c, _ := planeCtx("10.0.0.5:1")
	c.Request.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{leaf}}
	s.operatorPlaneGuard()(c)
	if c.IsAborted() {
		t.Fatalf("valid client cert denied")
	}

	// No cert denied.
	c, w := planeCtx("10.0.0.5:1")
	s.operatorPlaneGuard()(c)
	if !c.IsAborted() || w.Code != http.StatusForbidden {
		t.Fatalf("cert-less request not denied")
	}

	// Unknown CA denied: fresh server, cert file pointing at another CA.
	s2 := planeTestServer(t)
	otherDir := t.TempDir()
	issueTestClientCert(t, otherDir) // different CA on disk
	s2.cfg.Server.OperatorMTLS = true
	s2.cfg.Server.OperatorClientCAFile = filepath.Join(otherDir, "op-ca.crt")
	c, _ = planeCtx("10.0.0.5:1")
	c.Request.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{leaf}}
	s2.operatorPlaneGuard()(c)
	if !c.IsAborted() {
		t.Fatalf("foreign-CA cert accepted")
	}

	// mTLS disabled: cert-less passes.
	s3 := planeTestServer(t)
	c, _ = planeCtx("10.0.0.5:1")
	s3.operatorPlaneGuard()(c)
	if c.IsAborted() {
		t.Fatalf("disabled mTLS denied")
	}
}

func TestConfigureTLSOperatorMTLSRequestsCert(t *testing.T) {
	dir := t.TempDir()
	// Minimal self-signed server cert so the loader succeeds.
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(3), Subject: pkix.Name{CommonName: "localhost"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		DNSNames: []string{"localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, _ := x509.MarshalECPrivateKey(key)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	certFile := filepath.Join(dir, "srv.crt")
	keyFile := filepath.Join(dir, "srv.key")
	if err := os.WriteFile(certFile, certPEM, 0600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(keyFile, keyPEM, 0600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	s := planeTestServer(t)
	s.cfg.Server.TLSEnabled = true
	s.cfg.Server.CertFile = certFile
	s.cfg.Server.KeyFile = keyFile
	s.cfg.Server.OperatorMTLS = true
	srv := &http.Server{}
	if err := s.configureTLS(srv); err != nil {
		t.Fatalf("configureTLS: %v", err)
	}
	// Request (not require): beacons without certs must keep working on the
	// shared listener; enforcement lives in the middleware.
	if srv.TLSConfig.ClientAuth != tls.RequestClientCert {
		t.Fatalf("ClientAuth=%v, want RequestClientCert", srv.TLSConfig.ClientAuth)
	}
}
