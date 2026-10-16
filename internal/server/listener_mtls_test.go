package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gin-gonic/gin"
)

// ── throwaway PKI ───────────────────────────────────────────────────────────

// mtlsMaterial is a temporary PKI for listener mTLS tests: one client CA, a
// server pair for 127.0.0.1 signed by it, a client pair signed by it, and a
// second CA whose client certificate exists only to prove that foreign
// certificates are rejected (not merely requested).
type mtlsMaterial struct {
	caFile        string
	caPEM         []byte
	otherCAFile   string
	garbageCAFile string
	serverCert    string
	serverKey     string
	clientPair    tls.Certificate
	foreignPair   tls.Certificate
}

func writePEMFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

func newTestCA(t *testing.T, dir, name string) (*x509.Certificate, *ecdsa.PrivateKey, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating %s key: %v", name, err)
	}
	tpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("creating %s: %v", name, err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parsing %s: %v", name, err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	writePEMFile(t, filepath.Join(dir, name+".crt"), pemBytes)
	return cert, key, pemBytes
}

func issuePair(t *testing.T, dir, name string, ca *x509.Certificate, caKey *ecdsa.PrivateKey, usage x509.ExtKeyUsage, ips []net.IP) (certFile, keyFile string, pair tls.Certificate) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating %s key: %v", name, err)
	}
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: name},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{usage},
		IPAddresses:  ips,
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatalf("signing %s: %v", name, err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshalling %s key: %v", name, err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	certFile = filepath.Join(dir, name+".crt")
	keyFile = filepath.Join(dir, name+".key")
	writePEMFile(t, certFile, certPEM)
	writePEMFile(t, keyFile, keyPEM)
	pair, err = tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("loading %s pair: %v", name, err)
	}
	return certFile, keyFile, pair
}

func newMTLSTestMaterial(t *testing.T) *mtlsMaterial {
	t.Helper()
	dir := t.TempDir()
	ca, caKey, caPEM := newTestCA(t, dir, "listener-client-ca")
	otherCA, otherCAKey, _ := newTestCA(t, dir, "foreign-ca")
	serverCert, serverKey, _ := issuePair(t, dir, "server", ca, caKey, x509.ExtKeyUsageServerAuth, []net.IP{net.ParseIP("127.0.0.1")})
	_, _, clientPair := issuePair(t, dir, "client", ca, caKey, x509.ExtKeyUsageClientAuth, nil)
	_, _, foreignPair := issuePair(t, dir, "foreign-client", otherCA, otherCAKey, x509.ExtKeyUsageClientAuth, nil)
	garbage := filepath.Join(dir, "garbage.pem")
	writePEMFile(t, garbage, []byte("this is not a PEM bundle\n"))
	return &mtlsMaterial{
		caFile:        filepath.Join(dir, "listener-client-ca.crt"),
		caPEM:         caPEM,
		otherCAFile:   filepath.Join(dir, "foreign-ca.crt"),
		garbageCAFile: garbage,
		serverCert:    serverCert,
		serverKey:     serverKey,
		clientPair:    clientPair,
		foreignPair:   foreignPair,
	}
}

func newMTLSTestServer(t *testing.T, m *mtlsMaterial) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := config.DefaultConfig()
	cfg.Server.CertFile = m.serverCert
	cfg.Server.KeyFile = m.serverKey
	s := &Server{
		db:             testutil.SetupTestDB(t),
		cfg:            cfg,
		extraListeners: make(map[string]io.Closer),
		ctx:            context.Background(),
	}
	s.metrics = NewMetricsCollector(s)
	return s
}

func (s *Server) testTLSConfig(t *testing.T, o listenerTLSOverrides) *tls.Config {
	t.Helper()
	srv := &http.Server{}
	if err := s.configureTLSWith(srv, o); err != nil {
		t.Fatalf("configureTLSWith(%+v): %v", o, err)
	}
	return srv.TLSConfig
}

// startTLSTestServer serves plain HTTP over a TLS listener built from cfg and
// returns the dialable address.
func startTLSTestServer(t *testing.T, cfg *tls.Config) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})}
	go srv.Serve(tls.NewListener(ln, cfg))
	return ln.Addr().String(), func() { srv.Close() }
}

func clientTLSConfig(t *testing.T, m *mtlsMaterial, pair *tls.Certificate, maxVersion uint16) *tls.Config {
	t.Helper()
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(m.caPEM) {
		t.Fatal("test CA PEM did not parse")
	}
	cfg := &tls.Config{RootCAs: pool, MaxVersion: maxVersion}
	if pair != nil {
		cfg.Certificates = []tls.Certificate{*pair}
	}
	return cfg
}

// assertTLSConnectionAccepted requires the server to accept the client. A
// successful tls.Dial is NOT enough: in TLS 1.3 the client finishes its
// handshake before the server verifies the client certificate, so a rejected
// peer only shows up as a read error. An accepted, command-less peer simply
// stays quiet, which surfaces as a read timeout.
func assertTLSConnectionAccepted(t *testing.T, addr string, cfg *tls.Config) {
	t.Helper()
	conn, err := tls.Dial("tcp", addr, cfg)
	if err != nil {
		t.Fatalf("TLS handshake failed: %v", err)
	}
	defer conn.Close()
	if err := conn.SetReadDeadline(time.Now().Add(750 * time.Millisecond)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	_, err = conn.Read(make([]byte, 1))
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return
	}
	t.Fatalf("server closed the connection right after the handshake (read: %v): the client was rejected", err)
}

// assertTLSConnectionRejected requires the server to refuse the client, either
// during the handshake (TLS 1.2 and earlier) or right after it (TLS 1.3).
func assertTLSConnectionRejected(t *testing.T, addr string, cfg *tls.Config, what string) {
	t.Helper()
	conn, err := tls.Dial("tcp", addr, cfg)
	if err != nil {
		return // refused during the handshake
	}
	defer conn.Close()
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	_, err = conn.Read(make([]byte, 1))
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		t.Fatalf("%s: the connection stayed open, so the server did not enforce it", what)
	}
	if err == nil {
		t.Fatalf("%s: the server sent application data to an unauthenticated client", what)
	}
}

// ── tests ───────────────────────────────────────────────────────────────────

// TestConfigureTLSWithListenerOverrides pins the merge rule: per-listener knobs
// can only strengthen the server-wide posture, never relax it.
func TestConfigureTLSWithListenerOverrides(t *testing.T) {
	m := newMTLSTestMaterial(t)

	t.Run("no mTLS anywhere stays opt-in free", func(t *testing.T) {
		s := newMTLSTestServer(t, m)
		cfg := s.testTLSConfig(t, listenerTLSOverrides{})
		if cfg.MinVersion != tls.VersionTLS12 {
			t.Fatalf("MinVersion=%#x, want TLS 1.2", cfg.MinVersion)
		}
		if cfg.ClientAuth != tls.NoClientCert {
			t.Fatalf("ClientAuth=%v, want NoClientCert", cfg.ClientAuth)
		}
	})

	t.Run("server-wide mTLS covers extra listeners", func(t *testing.T) {
		s := newMTLSTestServer(t, m)
		s.cfg.Server.ClientCAFile = m.caFile
		s.cfg.Server.RequireClientCert = true
		cfg := s.testTLSConfig(t, listenerTLSOverrides{})
		if cfg.ClientAuth != tls.RequireAndVerifyClientCert {
			t.Fatalf("ClientAuth=%v, want RequireAndVerifyClientCert", cfg.ClientAuth)
		}
		if cfg.ClientCAs == nil {
			t.Fatal("ClientCAs not loaded from the server-wide CA")
		}
	})

	t.Run("listener can demand client certs on its own", func(t *testing.T) {
		s := newMTLSTestServer(t, m)
		cfg := s.testTLSConfig(t, listenerTLSOverrides{ClientCAFile: m.caFile, RequireClientCert: true})
		if cfg.ClientAuth != tls.RequireAndVerifyClientCert {
			t.Fatalf("ClientAuth=%v, want RequireAndVerifyClientCert", cfg.ClientAuth)
		}
	})

	t.Run("listener cannot drop the server-wide requirement", func(t *testing.T) {
		s := newMTLSTestServer(t, m)
		s.cfg.Server.ClientCAFile = m.caFile
		s.cfg.Server.RequireClientCert = true
		cfg := s.testTLSConfig(t, listenerTLSOverrides{MinTLSVersion: "1.3"})
		if cfg.ClientAuth != tls.RequireAndVerifyClientCert {
			t.Fatalf("ClientAuth=%v: a listener must not be able to disable server-wide mTLS", cfg.ClientAuth)
		}
		if cfg.MinVersion != tls.VersionTLS13 {
			t.Fatalf("MinVersion=%#x, want TLS 1.3", cfg.MinVersion)
		}
	})

	t.Run("listener CA replaces the server CA for that listener", func(t *testing.T) {
		s := newMTLSTestServer(t, m)
		// The server-wide CA path is deliberately unreadable: if the listener
		// CA were ignored, the config would fail to load it.
		s.cfg.Server.ClientCAFile = filepath.Join(t.TempDir(), "missing.pem")
		s.cfg.Server.RequireClientCert = true
		cfg := s.testTLSConfig(t, listenerTLSOverrides{ClientCAFile: m.caFile})
		if cfg.ClientAuth != tls.RequireAndVerifyClientCert {
			t.Fatalf("ClientAuth=%v, want RequireAndVerifyClientCert", cfg.ClientAuth)
		}
	})

	t.Run("requiring certs without any CA fails closed", func(t *testing.T) {
		s := newMTLSTestServer(t, m)
		srv := &http.Server{}
		err := s.configureTLSWith(srv, listenerTLSOverrides{RequireClientCert: true})
		if err == nil {
			t.Fatal("require_client_cert without a CA must fail the listener config")
		}
		if srv.TLSConfig != nil {
			t.Fatal("a failed TLS config must not be installed on the server")
		}
	})

	t.Run("invalid min_tls_version is refused", func(t *testing.T) {
		s := newMTLSTestServer(t, m)
		srv := &http.Server{}
		if err := s.configureTLSWith(srv, listenerTLSOverrides{MinTLSVersion: "1.1"}); err == nil {
			t.Fatal("min_tls_version 1.1 must be refused instead of degrading to the default")
		}
	})
}

// TestListenerMTLSRealHandshake proves the posture with real handshakes: a
// client from the configured CA is accepted, a client without a certificate or
// with one from a foreign CA is rejected, and a raised floor refuses older
// clients.
func TestListenerMTLSRealHandshake(t *testing.T) {
	m := newMTLSTestMaterial(t)
	s := newMTLSTestServer(t, m)
	s.cfg.Server.ClientCAFile = m.caFile
	s.cfg.Server.RequireClientCert = true
	addr, closeSrv := startTLSTestServer(t, s.testTLSConfig(t, listenerTLSOverrides{}))
	defer closeSrv()

	t.Run("client with a CA-signed certificate is accepted", func(t *testing.T) {
		// A real request, not just a handshake: proves the server accepted the
		// certificate and serves this client end to end.
		client := &http.Client{Transport: &http.Transport{TLSClientConfig: clientTLSConfig(t, m, &m.clientPair, 0)}}
		resp, err := client.Get("https://" + addr + "/health")
		if err != nil {
			t.Fatalf("HTTPS request with a CA-signed client certificate failed: %v", err)
		}
		defer resp.Body.Close()
		io.Copy(io.Discard, resp.Body)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status=%d, want 200", resp.StatusCode)
		}
	})

	t.Run("client without a certificate is rejected", func(t *testing.T) {
		assertTLSConnectionRejected(t, addr, clientTLSConfig(t, m, nil, 0), "client without a certificate")
	})

	t.Run("certificate from a foreign CA is rejected", func(t *testing.T) {
		assertTLSConnectionRejected(t, addr, clientTLSConfig(t, m, &m.foreignPair, 0), "client certificate signed by a foreign CA")
	})

	t.Run("TLS 1.3 floor refuses a 1.2 client", func(t *testing.T) {
		s2 := newMTLSTestServer(t, m)
		addr2, close2 := startTLSTestServer(t, s2.testTLSConfig(t, listenerTLSOverrides{MinTLSVersion: "1.3"}))
		defer close2()
		assertTLSConnectionRejected(t, addr2, clientTLSConfig(t, m, nil, tls.VersionTLS12), "a TLS 1.2 client against a 1.3 floor")
		assertTLSConnectionAccepted(t, addr2, clientTLSConfig(t, m, nil, 0))
	})

	t.Run("raw tls:// extra listener honours the same policy", func(t *testing.T) {
		s3 := newMTLSTestServer(t, m)
		key := fmt.Sprintf("tls://127.0.0.1:%d", freeTCPPort(t))
		ov := listenerTLSOverrides{ClientCAFile: m.caFile, RequireClientCert: true, MinTLSVersion: "1.3"}
		if err := s3.startExtraTCPListener(key, "tls", ov); err != nil {
			t.Fatalf("startExtraTCPListener: %v", err)
		}
		defer s3.stopExtraListener(key)
		dialAddr := strings.TrimPrefix(key, "tls://")

		assertTLSConnectionAccepted(t, dialAddr, clientTLSConfig(t, m, &m.clientPair, 0))
		assertTLSConnectionRejected(t, dialAddr, clientTLSConfig(t, m, nil, 0), "tls:// listener without a certificate")
	})
}

// TestValidateListenerTLSShape pins the API-boundary contract: impossible
// postures are rejected with a message that names the problem.
func TestValidateListenerTLSShape(t *testing.T) {
	m := newMTLSTestMaterial(t)
	cases := []struct {
		name     string
		scheme   string
		o        listenerTLSOverrides
		globalCA string
		wantErr  string
	}{
		{name: "no knobs is fine for any scheme", scheme: "tcp"},
		{name: "tls knobs on tcp are refused", scheme: "tcp", o: listenerTLSOverrides{MinTLSVersion: "1.3"}, wantErr: "only apply to https or tls"},
		{name: "min 1.2 accepted", scheme: "https", o: listenerTLSOverrides{MinTLSVersion: "1.2"}},
		{name: "min 1.3 accepted", scheme: "https", o: listenerTLSOverrides{MinTLSVersion: "1.3"}},
		{name: "min 1.1 refused", scheme: "https", o: listenerTLSOverrides{MinTLSVersion: "1.1"}, wantErr: "min_tls_version must be"},
		{name: "unreadable CA refused", scheme: "https", o: listenerTLSOverrides{ClientCAFile: filepath.Join(t.TempDir(), "missing.pem")}, wantErr: "reading client CA"},
		{name: "garbage CA refused", scheme: "https", o: listenerTLSOverrides{ClientCAFile: m.garbageCAFile}, wantErr: "no usable PEM"},
		{name: "require without any CA refused", scheme: "https", o: listenerTLSOverrides{RequireClientCert: true}, wantErr: "needs a client CA"},
		{name: "require with a listener CA accepted", scheme: "https", o: listenerTLSOverrides{RequireClientCert: true, ClientCAFile: m.caFile}},
		{name: "require with the server-wide CA accepted", scheme: "https", o: listenerTLSOverrides{RequireClientCert: true}, globalCA: m.caFile},
		{name: "listener CA without require accepted", scheme: "https", o: listenerTLSOverrides{ClientCAFile: m.caFile}},
		{name: "tls:// scheme accepted", scheme: "tls", o: listenerTLSOverrides{RequireClientCert: true, ClientCAFile: m.caFile}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newMTLSTestServer(t, m)
			s.cfg.Server.ClientCAFile = tc.globalCA
			err := s.validateListenerTLSShape(tc.scheme, tc.o)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("validateListenerTLSShape(%q, %+v) = %v, want nil", tc.scheme, tc.o, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validateListenerTLSShape(%q, %+v) = nil, want error containing %q", tc.scheme, tc.o, tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not mention %q", err.Error(), tc.wantErr)
			}
		})
	}
}

// TestListenerRequiresClientCertMergesBothSides covers the health-probe gate.
func TestListenerRequiresClientCertMergesBothSides(t *testing.T) {
	m := newMTLSTestMaterial(t)
	s := newMTLSTestServer(t, m)
	if s.listenerRequiresClientCert(nil) {
		t.Fatal("nil listener must not require client certs")
	}
	if s.listenerRequiresClientCert(&db.Listener{}) {
		t.Fatal("plain listener must not require client certs")
	}
	if !s.listenerRequiresClientCert(&db.Listener{RequireClientCert: true}) {
		t.Fatal("listener-level requirement must count")
	}
	s.cfg.Server.RequireClientCert = true
	if !s.listenerRequiresClientCert(&db.Listener{}) {
		t.Fatal("server-wide requirement must cover every listener")
	}
}

// TestProbeListenerSkipsHandshakeWhenClientCertsRequired: the prober has no
// client certificate, so a real handshake against an mTLS listener can only
// fail. The row must stay honest - TCP reachable reported as a skipped
// (shallower) check, a dead port reported as a real failure.
func TestProbeListenerSkipsHandshakeWhenClientCertsRequired(t *testing.T) {
	m := newMTLSTestMaterial(t)
	s := newMTLSTestServer(t, m)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	host, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(portStr)
	row := &db.Listener{ID: 1, Name: "mtls", Scheme: "https", Host: host, Port: port, RequireClientCert: true}

	h := s.probeListener(context.Background(), row)
	if !h.OK {
		t.Fatalf("reachable mTLS listener reported unhealthy: %s", h.Error)
	}
	if !h.Skipped {
		t.Fatal("the TLS handshake was not performed; the shallow check must be reported as skipped")
	}
	if !strings.Contains(h.Error, "client certificates required") {
		t.Fatalf("skip reason not surfaced: %q", h.Error)
	}

	ln.Close()
	h2 := s.probeListener(context.Background(), row)
	if h2.OK {
		t.Fatal("a dead mTLS listener must not stay green")
	}
	if h2.Skipped {
		t.Fatal("a dead port is a real failure, not a skipped check")
	}
}
