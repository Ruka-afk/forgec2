package server

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gin-gonic/gin"
)

// writeTestCert writes a self-signed cert+key pair into dir (server.crt /
// server.key) and returns the cert path.
func writeTestCert(t *testing.T, dir string, notAfter time.Time) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := x509.Certificate{
		SerialNumber:          big.NewInt(4242),
		Subject:               pkix.Name{CommonName: "forgec2-test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              notAfter,
		DNSNames:              []string{"forgec2.test"},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	certPath := filepath.Join(dir, "server.crt")
	if err := os.WriteFile(certPath, certPEM, 0600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "server.key"), keyPEM, 0600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	return certPath
}

func newCertsTestServer(t *testing.T, dir string) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := config.DefaultConfig()
	cfg.Server.CertFile = filepath.Join(dir, "server.crt")
	// DefaultConfig points key_file at data/server.key; pin it next to the
	// certificate so the read-only assertions below do not touch the repo.
	cfg.Server.KeyFile = filepath.Join(dir, "server.key")
	return &Server{db: testutil.SetupTestDB(t), cfg: cfg}
}

// TestGetCertInfoNotFound proves a missing certificate is a clean 404 (the
// settings page distinguishes "no cert" from "cert unreadable").
func TestGetCertInfoNotFound(t *testing.T) {
	dir := t.TempDir()
	s := newCertsTestServer(t, dir)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	s.handleGetCertInfo(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s, want 404 when no certificate exists", w.Code, w.Body.String())
	}
}

// TestGetCertInfoParsesFields proves a valid certificate is reported with its
// parsed fields (subject, self-signed flag, DNS names, serial).
func TestGetCertInfoParsesFields(t *testing.T) {
	dir := t.TempDir()
	certPath := writeTestCert(t, dir, time.Now().Add(48*time.Hour))
	s := newCertsTestServer(t, dir)
	_ = certPath

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	s.handleGetCertInfo(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{"forgec2-test", "forgec2.test", "4242"} {
		if !bytes.Contains([]byte(body), []byte(want)) {
			t.Fatalf("cert info missing %q: %s", want, body)
		}
	}
	if !bytes.Contains([]byte(body), []byte(`"is_self_signed":true`)) {
		t.Fatalf("is_self_signed not reported for a self-signed cert: %s", body)
	}
}

// TestGetCertInfoRejectsGarbagePEM proves an unparseable certificate is a 500,
// not a 200 with empty fields that reads like a valid cert.
func TestGetCertInfoRejectsGarbagePEM(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "server.crt"), []byte("not a pem"), 0600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	s := newCertsTestServer(t, dir)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	s.handleGetCertInfo(c)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d, want 500 for a malformed certificate", w.Code)
	}
}

// TestUploadCertRequiresFiles proves both the cert and the key parts are
// mandatory before anything is written.
func TestUploadCertRequiresFiles(t *testing.T) {
	dir := t.TempDir()
	s := newCertsTestServer(t, dir)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	c.Request.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	s.handleUploadCert(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s, want 400 (cert file required)", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "server.crt")); !os.IsNotExist(err) {
		t.Fatal("a rejected upload still wrote a certificate")
	}
}

// TestUploadCertRejectsExpired proves an already-expired certificate is
// refused: installing one would silently break every TLS beacon at the next
// handshake.
func TestUploadCertRejectsExpired(t *testing.T) {
	dir := t.TempDir()
	s := newCertsTestServer(t, dir)

	// Build an expired pair by re-signing the same template with a past date.
	src := t.TempDir()
	certPath := writeTestCert(t, src, time.Now().Add(-24*time.Hour))
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	keyPEM, err := os.ReadFile(filepath.Join(src, "server.key"))
	if err != nil {
		t.Fatalf("read key: %v", err)
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	cw, err := mw.CreateFormFile("cert", "server.crt")
	if err != nil {
		t.Fatalf("form file: %v", err)
	}
	cw.Write(certPEM)
	kw, err := mw.CreateFormFile("key", "server.key")
	if err != nil {
		t.Fatalf("form file: %v", err)
	}
	kw.Write(keyPEM)
	mw.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", &buf)
	c.Request.Header.Set("Content-Type", mw.FormDataContentType())
	s.handleUploadCert(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s, want 400 for an expired certificate", w.Code, w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "server.crt")); !os.IsNotExist(err) {
		t.Fatal("an expired certificate was written to disk")
	}
}

// TestUploadCertRejectsBadPEMTypes proves the PEM type guards (the cert must
// be a CERTIFICATE block).
func TestUploadCertRejectsBadPEMTypes(t *testing.T) {
	dir := t.TempDir()
	s := newCertsTestServer(t, dir)

	src := t.TempDir()
	writeTestCert(t, src, time.Now().Add(48*time.Hour))
	certPEM, _ := os.ReadFile(filepath.Join(src, "server.crt"))
	keyPEM, _ := os.ReadFile(filepath.Join(src, "server.key"))

	// Swap the parts: send the key where the cert belongs.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	cw, _ := mw.CreateFormFile("cert", "server.crt")
	cw.Write(keyPEM)
	kw, _ := mw.CreateFormFile("key", "server.key")
	kw.Write(certPEM)
	mw.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", &buf)
	c.Request.Header.Set("Content-Type", mw.FormDataContentType())
	s.handleUploadCert(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s, want 400 (cert PEM type mismatch)", w.Code, w.Body.String())
	}
}

// TestUploadCertHappyPathWritesFiles proves a valid pair is installed with
// 0600 permissions and reported back.
func TestUploadCertHappyPathWritesFiles(t *testing.T) {
	dir := t.TempDir()
	s := newCertsTestServer(t, dir)

	src := t.TempDir()
	writeTestCert(t, src, time.Now().Add(48*time.Hour))
	certPEM, _ := os.ReadFile(filepath.Join(src, "server.crt"))
	keyPEM, _ := os.ReadFile(filepath.Join(src, "server.key"))

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	cw, _ := mw.CreateFormFile("cert", "server.crt")
	cw.Write(certPEM)
	kw, _ := mw.CreateFormFile("key", "server.key")
	kw.Write(keyPEM)
	mw.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", &buf)
	c.Request.Header.Set("Content-Type", mw.FormDataContentType())
	s.handleUploadCert(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", w.Code, w.Body.String())
	}
	for _, name := range []string{"server.crt", "server.key"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("%s not written: %v", name, err)
		}
		// Go does not map Unix mode bits on Windows, so the permission
		// assertion only holds where the mode is actually honored.
		if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
			t.Fatalf("%s perms = %o, want 0600", name, info.Mode().Perm())
		}
	}
}

// TestUploadCertHonorsConfiguredKeyFile proves the uploaded key lands on the
// configured key_file path instead of being derived next to the certificate.
// The handler used to overwrite Server.KeyFile unconditionally, so a custom
// key path kept the stale key while the new certificate was installed — a
// mismatched pair the moment the server reloads TLS.
func TestUploadCertHonorsConfiguredKeyFile(t *testing.T) {
	dir := t.TempDir()
	keyDir := t.TempDir()
	s := newCertsTestServer(t, dir)
	customKey := filepath.Join(keyDir, "custom.key")
	s.cfg.Server.KeyFile = customKey

	src := t.TempDir()
	writeTestCert(t, src, time.Now().Add(48*time.Hour))
	certPEM, _ := os.ReadFile(filepath.Join(src, "server.crt"))
	keyPEM, _ := os.ReadFile(filepath.Join(src, "server.key"))

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	cw, _ := mw.CreateFormFile("cert", "server.crt")
	if _, err := cw.Write(certPEM); err != nil {
		t.Fatalf("write cert part: %v", err)
	}
	kw, _ := mw.CreateFormFile("key", "server.key")
	if _, err := kw.Write(keyPEM); err != nil {
		t.Fatalf("write key part: %v", err)
	}
	mw.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", &buf)
	c.Request.Header.Set("Content-Type", mw.FormDataContentType())
	s.handleUploadCert(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", w.Code, w.Body.String())
	}
	got, err := os.ReadFile(customKey)
	if err != nil {
		t.Fatalf("configured key_file not written: %v", err)
	}
	if !bytes.Equal(got, keyPEM) {
		t.Fatal("configured key_file does not contain the uploaded key")
	}
	if _, err := os.Stat(filepath.Join(dir, "server.key")); !os.IsNotExist(err) {
		t.Fatal("handler also wrote a derived server.key next to the certificate")
	}
}

// TestRegenerateCertReplacesExisting proves regeneration removes the old pair
// and writes a fresh self-signed certificate the info endpoint can read.
func TestRegenerateCertReplacesExisting(t *testing.T) {
	dir := t.TempDir()
	oldPath := writeTestCert(t, dir, time.Now().Add(48*time.Hour))
	oldBytes, err := os.ReadFile(oldPath)
	if err != nil {
		t.Fatalf("read old: %v", err)
	}
	s := newCertsTestServer(t, dir)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	s.handleRegenerateCert(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", w.Code, w.Body.String())
	}
	newBytes, err := os.ReadFile(oldPath)
	if err != nil {
		t.Fatalf("read new: %v", err)
	}
	if bytes.Equal(oldBytes, newBytes) {
		t.Fatal("regeneration produced the identical certificate")
	}
	if _, err := os.Stat(filepath.Join(dir, "server.key")); err != nil {
		t.Fatalf("private key missing after regeneration: %v", err)
	}

	// The new certificate must be parseable through the info endpoint.
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	s.handleGetCertInfo(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("info after regenerate: status=%d body=%s", w2.Code, w2.Body.String())
	}
}

// TestRegenerateCertHonorsConfiguredKeyFile pins the same key_file contract
// for regeneration: the fresh private key must land on the configured path.
// Writing it anywhere else leaves the server reading a stale key that no
// longer matches the regenerated certificate.
func TestRegenerateCertHonorsConfiguredKeyFile(t *testing.T) {
	dir := t.TempDir()
	keyDir := t.TempDir()
	writeTestCert(t, dir, time.Now().Add(48*time.Hour))
	customKey := filepath.Join(keyDir, "custom.key")
	// Seed the pair the way a custom key_file install looks: certificate in
	// one directory, key in the configured one.
	if err := os.Rename(filepath.Join(dir, "server.key"), customKey); err != nil {
		t.Fatalf("seed key: %v", err)
	}
	s := newCertsTestServer(t, dir)
	s.cfg.Server.KeyFile = customKey

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	s.handleRegenerateCert(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", w.Code, w.Body.String())
	}
	if _, err := os.Stat(customKey); err != nil {
		t.Fatalf("regenerated key not written to configured key_file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "server.key")); !os.IsNotExist(err) {
		t.Fatal("regeneration also wrote a derived server.key next to the certificate")
	}
}
