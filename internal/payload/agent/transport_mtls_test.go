//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	utls "github.com/refraction-networking/utls"
)

// resetMTLS clears the mTLS globals for one test case and restores the
// previous values (including anything set by another test) at cleanup.
func resetMTLS(t *testing.T) {
	t.Helper()
	origCert, origKey, origCA := MTLSCertStr, MTLSKeyStr, MTLSCAStr
	origLoaded := mtlsCertLoaded
	origClient := mtlsClientCert
	origPool := mtlsCAPool
	t.Cleanup(func() {
		MTLSCertStr, MTLSKeyStr, MTLSCAStr = origCert, origKey, origCA
		mtlsCertLoaded = origLoaded
		mtlsClientCert = origClient
		mtlsCAPool = origPool
	})
	MTLSCertStr, MTLSKeyStr, MTLSCAStr = "", "", ""
	mtlsCertLoaded = false
	mtlsClientCert = utls.Certificate{}
	mtlsCAPool = nil
}

// newTestCA returns a throwaway CA good for the duration of the test.
func newTestCA(t *testing.T) (*x509.Certificate, *ecdsa.PrivateKey, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "forgec2-mtls-test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return cert, key, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

// issueLeaf signs a leaf certificate with the CA (server when clientAuth is
// false, client when true) and returns the pair plus both PEM blocks.
func issueLeaf(t *testing.T, ca *x509.Certificate, caKey *ecdsa.PrivateKey, cn string, clientAuth bool) (tls.Certificate, []byte, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	eku := x509.ExtKeyUsageServerAuth
	if clientAuth {
		eku = x509.ExtKeyUsageClientAuth
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{eku},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:     []string{"localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	return pair, certPEM, keyPEM
}

// startMTLSServer runs an HTTPS server that requires and verifies a client
// certificate signed by clientCA. sawClientCert is set when a request arrives
// with a verified peer certificate.
func startMTLSServer(t *testing.T, serverCert tls.Certificate, clientCA *x509.Certificate) (*httptest.Server, *int32) {
	t.Helper()
	var sawClientCert int32
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
			atomic.StoreInt32(&sawClientCert, 1)
		}
		w.WriteHeader(http.StatusOK)
	}))
	pool := x509.NewCertPool()
	pool.AddCert(clientCA)
	srv.TLS = &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientCAs:    pool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return srv, &sawClientCert
}

// TestMTLSEndToEndClientCertificate proves the whole generated-agent path:
// base64-encoded PEM in the config blob vars -> initMTLS -> a real utls
// handshake against a server that requires a verified client certificate.
// Before the decode fix the pair never loaded and this request could not be
// authenticated.
func TestMTLSEndToEndClientCertificate(t *testing.T) {
	resetMTLS(t)
	ca, caKey, caPEM := newTestCA(t)
	serverCert, _, _ := issueLeaf(t, ca, caKey, "server.test", false)
	_, clientPEM, clientKeyPEM := issueLeaf(t, ca, caKey, "client.test", true)
	srv, sawClientCert := startMTLSServer(t, serverCert, ca)

	MTLSCertStr = base64.StdEncoding.EncodeToString(clientPEM)
	MTLSKeyStr = base64.StdEncoding.EncodeToString(clientKeyPEM)
	MTLSCAStr = base64.StdEncoding.EncodeToString(caPEM)
	initMTLS()

	if !mtlsCertLoaded {
		t.Fatal("client certificate was not loaded from base64 PEM")
	}
	if mtlsCAPool == nil {
		t.Fatal("mTLS CA pool was not populated")
	}
	resp, err := mtlsClient.Get(srv.URL)
	if err != nil {
		t.Fatalf("mTLS request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if atomic.LoadInt32(sawClientCert) != 1 {
		t.Fatal("server did not observe a verified client certificate")
	}
}

// TestMTLSWithoutClientCertificateRejected pins the fail-closed property: the
// same server rejects the connection when no client certificate is loaded.
func TestMTLSWithoutClientCertificateRejected(t *testing.T) {
	resetMTLS(t)
	ca, caKey, _ := newTestCA(t)
	serverCert, _, _ := issueLeaf(t, ca, caKey, "server.test", false)
	srv, _ := startMTLSServer(t, serverCert, ca)

	// Trust the test server (CA pool set) so the only failing factor is the
	// missing client certificate.
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	mtlsCAPool = pool

	resp, err := mtlsClient.Get(srv.URL)
	if err == nil {
		resp.Body.Close()
		t.Fatalf("expected handshake failure without a client certificate, got status %d", resp.StatusCode)
	}
}

// TestInitMTLSAcceptsRawPEM keeps hand-built -ldflags builds working: raw PEM
// values (not base64) must still load.
func TestInitMTLSAcceptsRawPEM(t *testing.T) {
	resetMTLS(t)
	ca, caKey, _ := newTestCA(t)
	_, clientPEM, clientKeyPEM := issueLeaf(t, ca, caKey, "client.test", true)

	MTLSCertStr = string(clientPEM)
	MTLSKeyStr = string(clientKeyPEM)
	initMTLS()

	if !mtlsCertLoaded {
		t.Fatal("raw PEM client certificate was not loaded")
	}
}

// TestInitMTLSRejectsInvalidMaterial covers the failure paths: garbage input
// neither panics nor leaves a half-initialized state, and a bogus CA bundle
// does not install an empty trust pool.
func TestInitMTLSRejectsInvalidMaterial(t *testing.T) {
	resetMTLS(t)

	MTLSCertStr = "not-a-certificate"
	MTLSKeyStr = "not-a-key"
	initMTLS()
	if mtlsCertLoaded {
		t.Fatal("garbage material must not load")
	}

	MTLSCertStr = base64.StdEncoding.EncodeToString([]byte("still garbage"))
	MTLSKeyStr = base64.StdEncoding.EncodeToString([]byte("still garbage"))
	initMTLS()
	if mtlsCertLoaded {
		t.Fatal("garbage base64 material must not load")
	}

	ca, caKey, _ := newTestCA(t)
	_, clientPEM, clientKeyPEM := issueLeaf(t, ca, caKey, "client.test", true)
	MTLSCertStr = base64.StdEncoding.EncodeToString(clientPEM)
	MTLSKeyStr = base64.StdEncoding.EncodeToString(clientKeyPEM)
	MTLSCAStr = "not-a-ca-bundle"
	initMTLS()
	if !mtlsCertLoaded {
		t.Fatal("valid pair with a bogus CA should still load the certificate")
	}
	if mtlsCAPool != nil {
		t.Fatal("bogus CA bundle must not install a trust pool")
	}
}

// TestLoadConfigBlobAppliesMTLSMaterial ties the generator's blob fields to
// the agent vars that initMTLS reads.
func TestLoadConfigBlobAppliesMTLSMaterial(t *testing.T) {
	blob := agentConfigBlob{
		C2URL:    "https://blob.mtls.test:8443",
		MTLSCert: "Y2VydA==",
		MTLSKey:  "a2V5",
		MTLSCA:   "Y2E=",
	}
	raw, err := json.Marshal(blob)
	if err != nil {
		t.Fatal(err)
	}

	orig := ConfigBlob
	origCert, origKey, origCA := MTLSCertStr, MTLSKeyStr, MTLSCAStr
	defer func() {
		ConfigBlob = orig
		MTLSCertStr, MTLSKeyStr, MTLSCAStr = origCert, origKey, origCA
	}()

	ConfigBlob = obfuscateForTest(t, raw)
	MTLSCertStr, MTLSKeyStr, MTLSCAStr = "", "", ""

	loadConfigBlob()

	if MTLSCertStr != blob.MTLSCert || MTLSKeyStr != blob.MTLSKey || MTLSCAStr != blob.MTLSCA {
		t.Fatalf("mTLS material not applied: %q %q %q", MTLSCertStr, MTLSKeyStr, MTLSCAStr)
	}
}
