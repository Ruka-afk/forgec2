package server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/testutil"
)

// TestResyncRefusedWhileSessionLive closes the UUID-existence oracle: a frame
// that fails AEAD while the server still holds that agent's session is
// unauthenticated garbage and must be answered with a plain 400, not a
// MAC-signed resync. The legitimate recovery path (server lost the session) is
// covered by TestDecryptFailureResyncsKnownAgent.
func TestResyncRefusedWhileSessionLive(t *testing.T) {
	s, _ := v2TestServer(t)

	agentUUID := "77777777-8888-4999-8999-aaaaaaaaaaaa"
	agent := v3TestAgent(t, s, agentUUID)
	if w := v2Post(t, s, agent.registerFrame()); w.Code != http.StatusOK {
		t.Fatalf("registration: %d body=%s", w.Code, w.Body.String())
	}
	if !s.sessionManager.HasSession(agentUUID) {
		t.Fatal("expected a live session after handshake")
	}

	garbage := base64.StdEncoding.EncodeToString([]byte("not-aead-ciphertext"))
	env, _ := json.Marshal(map[string]interface{}{
		"uuid": agentUUID,
		"seq":  99,
		"ts":   time.Now().Unix(),
		"c":    garbage,
	})
	w := v2Post(t, s, string(env))
	if w.Code == http.StatusOK {
		t.Fatalf("live-session garbage frame got a signed response: %s", w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err == nil {
		if ecdh, _ := resp["ecdh_pub"].(string); ecdh != "" {
			t.Fatalf("oracle: garbage frame received a resync with ecdh_pub: %s", w.Body.String())
		}
	}

	// After the server loses the session, the same frame earns the resync the
	// agent needs to recover.
	s.sessionManager.RemoveSession(agentUUID)
	if w := v2Post(t, s, string(env)); w.Code != http.StatusOK {
		t.Fatalf("lost-session frame must still resync: %d body=%s", w.Code, w.Body.String())
	}
}

func grpcMTLSServer(t *testing.T, caPath string) *Server {
	t.Helper()
	ginSetTestMode(t)
	dir := t.TempDir()
	certFile, keyFile := writeTestServerCert(t, dir)
	cfg := config.DefaultConfig()
	cfg.Server.JWTSecret = "test-secret-grpc-mtls-32char!"
	setServerTestKeys(cfg)
	cfg.Server.TLSEnabled = true
	cfg.Server.CertFile = certFile
	cfg.Server.KeyFile = keyFile
	cfg.Server.GRPCAddr = testFreeAddr(t)
	cfg.Server.RequireClientCert = true
	cfg.Server.ClientCAFile = caPath
	return New(cfg, testutil.SetupTestDB(t))
}

// writeTestServerCert generates a throwaway self-signed server cert/key pair.
func writeTestServerCert(t *testing.T, dir string) (string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(4),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	certFile := filepath.Join(dir, "srv.crt")
	keyFile := filepath.Join(dir, "srv.key")
	if err := os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	return certFile, keyFile
}

// TestGRPCMTLSFailClosed proves the gRPC listener refuses to start instead of
// silently serving without client authentication when mTLS material is
// missing, unreadable, or unparseable.
func TestGRPCMTLSFailClosed(t *testing.T) {
	cases := []struct {
		name   string
		caPath func(t *testing.T) string
	}{
		{"missing CA file", func(t *testing.T) string { return filepath.Join(t.TempDir(), "absent.crt") }},
		{"unparseable CA file", func(t *testing.T) string {
			p := filepath.Join(t.TempDir(), "bad.crt")
			if err := os.WriteFile(p, []byte("-----BEGIN CERTIFICATE-----\nnot base64\n-----END CERTIFICATE-----\n"), 0600); err != nil {
				t.Fatalf("write bad CA: %v", err)
			}
			return p
		}},
		{"empty CA path", func(t *testing.T) string { return "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := grpcMTLSServer(t, tc.caPath(t))
			s.startGRPCListener()
			if s.grpcListener != nil {
				addr := s.cfg.Server.GRPCAddr
				s.grpcListener.Stop()
				t.Fatalf("gRPC listener started without valid client CA (%s)", addr)
			}
			// Nothing must be listening on the configured address.
			conn, err := net.DialTimeout("tcp", s.cfg.Server.GRPCAddr, 500*time.Millisecond)
			if err == nil {
				conn.Close()
				t.Fatalf("something is listening on %s after fail-closed start", s.cfg.Server.GRPCAddr)
			}
		})
	}
}

// TestGRPCMTLSDisabledStaysLabInsecure pins the lab default: without
// require_client_cert the listener still starts (plaintext lab mode), so the
// fail-closed change cannot be mistaken for "gRPC always requires mTLS".
func TestGRPCMTLSDisabledStaysLabInsecure(t *testing.T) {
	ginSetTestMode(t)
	cfg := config.DefaultConfig()
	cfg.Server.JWTSecret = "test-secret-grpc-lab-32chars!"
	setServerTestKeys(cfg)
	cfg.Server.GRPCAddr = testFreeAddr(t)
	cfg.Server.TLSEnabled = false
	s := New(cfg, testutil.SetupTestDB(t))
	s.startGRPCListener()
	if s.grpcListener == nil {
		t.Fatal("lab-mode gRPC listener should start without mTLS config")
	}
	s.grpcListener.Stop()
}
