package server

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	fc2crypto "github.com/forgec2/forgec2/internal/crypto"
)

// TestServeNegotiatesHTTP2OnTheTLSListener pins the reason the listener uses
// http.Server.ServeTLS: the rotating TLS profile advertises "h2" through ALPN,
// and only ServeTLS installs the HTTP/2 handler. Serving a hand-built
// tls.Listener instead completes the handshake, advertises h2, and then fails
// every request: the CI smoke client and every browser saw a dead console
// while a curl restricted to HTTP/1.1 answered normally.
func TestServeNegotiatesHTTP2OnTheTLSListener(t *testing.T) {
	dir := t.TempDir()
	certFile := filepath.Join(dir, "srv.crt")
	keyFile := filepath.Join(dir, "srv.key")
	if err := fc2crypto.GenerateSelfSignedCert(certFile, keyFile); err != nil {
		t.Fatalf("generate certificate: %v", err)
	}

	cfg := &config.Config{}
	cfg.Server.TLSEnabled = true
	cfg.Server.CertFile = certFile
	cfg.Server.KeyFile = keyFile

	s := &Server{
		cfg: cfg,
		// The profile is what advertises h2; without it the listener would
		// never negotiate HTTP/2 and this test would prove nothing.
		tlsFingerprint: NewTLSFingerprintManager(true, true, "24h", "chrome"),
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "proto=%s", r.Proto)
	})}
	if err := s.configureTLS(srv); err != nil {
		t.Fatalf("configureTLS: %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	served := make(chan error, 1)
	go func() { served <- s.serve(srv, ln) }()
	t.Cleanup(func() {
		_ = srv.Close()
		<-served
	})

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true}, // self-signed test certificate
			ForceAttemptHTTP2: true,
		},
	}
	resp, err := client.Get("https://" + ln.Addr().String() + "/health")
	if err != nil {
		t.Fatalf("an HTTP/2 client cannot talk to the TLS listener: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64))
	if resp.ProtoMajor != 2 {
		t.Fatalf("negotiated %s, want HTTP/2.0 (ALPN advertises h2): body=%s", resp.Proto, body)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}
