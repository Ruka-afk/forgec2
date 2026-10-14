package server

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"io"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/db"
)

// ── Original probes (restored) ──────────────────────────────────────────────

func TestListenerHealthTrackerCountsConsecutiveFailures(t *testing.T) {
	tracker := &listenerHealthTracker{state: map[uint]*listenerHealth{}}

	first := &listenerHealth{ListenerID: 7, Error: "first failure"}
	tracker.observe(first)
	if first.Failures != 1 {
		t.Fatalf("first failure count = %d, want 1", first.Failures)
	}

	second := &listenerHealth{ListenerID: 7, Error: "second failure"}
	tracker.observe(second)
	if second.Failures != listenerFailThreshold {
		t.Fatalf("second failure count = %d, want %d", second.Failures, listenerFailThreshold)
	}

	recovered := &listenerHealth{ListenerID: 7, OK: true}
	tracker.observe(recovered)
	if recovered.Failures != 0 {
		t.Fatalf("successful probe retained %d failures", recovered.Failures)
	}

	afterRecovery := &listenerHealth{ListenerID: 7, Error: "new sequence"}
	tracker.observe(afterRecovery)
	if afterRecovery.Failures != 1 {
		t.Fatalf("failure after recovery count = %d, want 1", afterRecovery.Failures)
	}
}

func TestListenerHealthTrackerSkippedProbeResetsFailures(t *testing.T) {
	tracker := &listenerHealthTracker{state: map[uint]*listenerHealth{}}
	tracker.observe(&listenerHealth{ListenerID: 3, Failures: 9})

	skipped := &listenerHealth{ListenerID: 3, OK: true, Skipped: true, Failures: 9}
	tracker.observe(skipped)
	if skipped.Failures != 0 {
		t.Fatalf("skipped probe retained %d failures", skipped.Failures)
	}
}

func TestListenerHealthTrackerPrunesAndSortsSnapshot(t *testing.T) {
	tracker := &listenerHealthTracker{state: map[uint]*listenerHealth{}}
	tracker.observe(&listenerHealth{ListenerID: 9, OK: true})
	tracker.observe(&listenerHealth{ListenerID: 2, OK: true})
	tracker.observe(&listenerHealth{ListenerID: 5, OK: true})

	tracker.prune(map[uint]struct{}{9: {}, 2: {}})
	snapshot := tracker.snapshot()
	if len(snapshot) != 2 {
		t.Fatalf("snapshot length = %d, want 2", len(snapshot))
	}
	if snapshot[0].ListenerID != 2 || snapshot[1].ListenerID != 9 {
		t.Fatalf("snapshot order = [%d, %d], want [2, 9]", snapshot[0].ListenerID, snapshot[1].ListenerID)
	}
}

func TestProbeSSHReadsBanner(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_, _ = conn.Write([]byte("SSH-2.0-ForgeC2-test\r\n"))
			conn.Close()
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if ok, err := probeSSH(ctx, ln.Addr().String()); !ok || err != nil {
		t.Fatalf("probeSSH = %v, %v; want true, nil", ok, err)
	}
	if ok, _ := probeSSH(ctx, "127.0.0.1:1"); ok {
		t.Fatal("probeSSH against closed port must fail")
	}
}

func TestProbeH2CPriorKnowledge(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				// Read preface, answer with an empty SETTINGS frame.
				buf := make([]byte, 24+9)
				readFullForTest(c, buf)
				_, _ = c.Write([]byte{0, 0, 0, 0x4, 0, 0, 0, 0, 0})
			}(conn)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if ok, err := probeH2C(ctx, ln.Addr().String()); !ok || err != nil {
		t.Fatalf("probeH2C = %v, %v; want true, nil", ok, err)
	}
}

func readFullForTest(conn net.Conn, buf []byte) {
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for off := 0; off < len(buf); {
		n, err := conn.Read(buf[off:])
		if err != nil || n == 0 {
			return
		}
		off += n
	}
}

// ── Helpers for the added probes ────────────────────────────────────────────

// loopbackListener binds a loopback TCP listener and returns its address. The
// listener is closed by t.Cleanup.
func loopbackListener(t *testing.T) string {
	t.Helper()
	return loopbackListenerNoClose(t).Addr().String()
}

func loopbackListenerNoClose(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })
	return ln
}

func addrPort(t *testing.T, addr string) int {
	t.Helper()
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split %q: %v", addr, err)
	}
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		t.Fatalf("parse %q: %v", portStr, err)
	}
	return port
}

// tlsListener serves a real, unexpired self-signed certificate.
func tlsListener(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "probe"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		DNSNames:     []string{"probe"},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, key.Public(), key)
	if err != nil {
		t.Fatalf("cert: %v", err)
	}
	// PrivateKey is required: without it the server cannot sign and answers a
	// handshake-failure alert.
	cert := tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
	// No NextProtos: the probe offers no ALPN protocols, and a server that
	// advertises any would answer with a no_application_protocol alert.
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}})
	if err != nil {
		t.Fatalf("tls listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				// The server must complete its side of the handshake, or the
				// client blocks until its deadline.
				if tlsConn, ok := c.(*tls.Conn); ok {
					_ = tlsConn.Handshake()
				}
				c.Close()
			}(conn)
		}
	}()
	return ln.Addr().String()
}

// sshBannerListener serves an SSH banner and holds the connection open.
func sshBannerListener(t *testing.T) string {
	t.Helper()
	ln := loopbackListenerNoClose(t)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_, _ = c.Write([]byte("SSH-2.0-ForgeC2-test\r\n"))
				buf := make([]byte, 1)
				_, _ = c.Read(buf)
			}(conn)
		}
	}()
	return ln.Addr().String()
}

// ── Added probe coverage ────────────────────────────────────────────────────

// TestProbeTCPReachesLiveListener proves the dial probe distinguishes a live
// listener from a dead one, and that a cancelled context fails fast.
func TestProbeTCPReachesLiveListener(t *testing.T) {
	ctx := context.Background()
	if !probeTCP(ctx, loopbackListener(t)) {
		t.Fatal("a live listener was reported dead")
	}
	// Port 0 is never a valid destination, so the dial fails immediately.
	if probeTCP(ctx, "127.0.0.1:0") {
		t.Fatal("an unreachable endpoint was reported live")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if probeTCP(cancelled, "127.0.0.1:0") {
		t.Fatal("a cancelled context reported a live endpoint")
	}
}

// TestProbeTLSCompletesHandshake pins the handshake probe: a real TLS listener
// passes, a plain-TCP endpoint yields an error rather than a bare false, and an
// unreachable target fails with its own error.
func TestProbeTLSCompletesHandshake(t *testing.T) {
	ctx := context.Background()

	if ok, err := probeTLS(ctx, tlsListener(t)); !ok || err != nil {
		t.Fatalf("probeTLS on a TLS listener = (%v, %v), want (true, nil)", ok, err)
	}

	ok, err := probeTLS(ctx, loopbackListener(t))
	if ok {
		t.Fatal("a plain TCP endpoint passed the TLS probe")
	}
	if err == nil {
		t.Fatal("a failed TLS probe returned no error")
	}

	if ok, err := probeTLS(ctx, "127.0.0.1:0"); ok || err == nil {
		t.Fatalf("unreachable = (%v, %v), want (false, error)", ok, err)
	}
}

// TestProbeDNSFailsForDeadResolver proves the UDP exchange reports the error
// instead of hanging or reporting green.
func TestProbeDNSFailsForDeadResolver(t *testing.T) {
	ok, err := probeDNS(context.Background(), "127.0.0.1", 0, "")
	if ok {
		t.Fatal("an unreachable DNS endpoint was reported live")
	}
	if err == nil {
		t.Fatal("a failed DNS probe returned no error")
	}
}

// TestProbeListenerDispatchesKnownSchemes proves each scheme routes to its own
// probe and reports the target address.
func TestProbeListenerDispatchesKnownSchemes(t *testing.T) {
	s := mustTenantServer(t)

	cases := []struct {
		name   string
		addr   string
		scheme string
		wantOK bool
	}{
		{"tcp live", loopbackListener(t), "tcp", true},
		{"http live", loopbackListener(t), "http", true},
		{"tcp dead", "127.0.0.1:0", "tcp", false},
		{"https live", tlsListener(t), "https", true},
		{"tls live", tlsListener(t), "tls", true},
		{"ssh live", sshBannerListener(t), "ssh", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ln := &db.Listener{ID: 7, Name: tc.scheme, Host: "127.0.0.1", Port: addrPort(t, tc.addr), Scheme: tc.scheme, Enabled: true}
			h := s.probeListener(context.Background(), ln)
			if h.OK != tc.wantOK {
				t.Fatalf("probeListener(%s) OK = %v, want %v (err=%q)", tc.scheme, h.OK, tc.wantOK, h.Error)
			}
			if h.Scheme != tc.scheme || h.ListenerID != 7 {
				t.Fatalf("health = %+v", h)
			}
			if !tc.wantOK && h.Error == "" {
				t.Fatal("a failed probe recorded no reason")
			}
		})
	}
}

// TestProbeListenerReportsGapForUnknownScheme proves an unsupported transport
// is marked skipped rather than silently counted as either healthy or failed.
// The code deliberately keeps OK=true alongside Skipped=true so an unprobeable
// transport never trips the failure threshold — the gap is reported in Error.
func TestProbeListenerReportsGapForUnknownScheme(t *testing.T) {
	s := mustTenantServer(t)
	addr := loopbackListener(t)

	for _, scheme := range []string{"icmp", "grpc", "smb", "totally-unknown", ""} {
		t.Run(scheme, func(t *testing.T) {
			ln := &db.Listener{ID: 1, Name: "odd", Host: "127.0.0.1", Port: addrPort(t, addr), Scheme: scheme, Enabled: true}
			h := s.probeListener(context.Background(), ln)
			if !h.Skipped {
				t.Fatalf("scheme %q was probed instead of skipped", scheme)
			}
			if !h.OK {
				t.Fatal("an unsupported scheme reported a failure that would count toward the flip threshold")
			}
			if h.Error == "" {
				t.Fatal("the probe gap was not reported in the error field")
			}
			if h.Addr != addr {
				t.Fatalf("addr = %q, want %q", h.Addr, addr)
			}
			tracker := &listenerHealthTracker{state: map[uint]*listenerHealth{}}
			tracker.observe(h)
			if tracker.state[1].Failures != 0 {
				t.Fatal("a skipped probe counted as a failure")
			}
		})
	}
}

// TestProbeListenerUDPUsesLocalHandle proves UDP/QUIC liveness is the local
// handle, not a network probe: an enabled row with no socket must fail rather
// than report green.
func TestProbeListenerUDPUsesLocalHandle(t *testing.T) {
	s := mustTenantServer(t)
	s.extraListeners = map[string]io.Closer{}

	udp := &db.Listener{ID: 1, Name: "udp", Host: "127.0.0.1", Port: 5353, Scheme: "udp", Enabled: true}
	udpHealth := s.probeListener(context.Background(), udp)
	if udpHealth.OK {
		t.Fatal("a UDP row with no live handle reported healthy")
	}
	if udpHealth.Error == "" {
		t.Fatal("the missing handle was not reported")
	}

	quic := &db.Listener{ID: 2, Name: "quic", Host: "127.0.0.1", Port: 443, Scheme: "quic", Enabled: true}
	if h := s.probeListener(context.Background(), quic); h.OK {
		t.Fatal("a QUIC row with no running listener reported healthy")
	}
}

// TestListenerHealthSnapshotIsACopy pins that snapshot returns copies, so a
// reader cannot corrupt tracker state through them.
func TestListenerHealthSnapshotIsACopy(t *testing.T) {
	tracker := &listenerHealthTracker{state: map[uint]*listenerHealth{}}
	tracker.observe(&listenerHealth{ListenerID: 4, OK: true})
	snapshot := tracker.snapshot()
	if len(snapshot) != 1 {
		t.Fatalf("snapshot length = %d, want 1", len(snapshot))
	}
	snapshot[0].Failures = 99
	if tracker.state[4].Failures == 99 {
		t.Fatal("snapshot returned live pointers into the tracker")
	}
}

// TestCheckAllListenersSweepAndFlip proves the whole sweep: enabled rows are
// probed, the failure counter is carried, and only at the threshold does the
// row's status flip to error. A disabled row is never probed.
func TestCheckAllListenersSweepAndFlip(t *testing.T) {
	s := mustTenantServer(t)
	s.extraListeners = map[string]io.Closer{}

	live := addrPort(t, loopbackListener(t))

	if err := s.db.Create(&db.Listener{Name: "live", Host: "127.0.0.1", Port: live, Scheme: "tcp", Enabled: true, Status: "active"}).Error; err != nil {
		t.Fatalf("seed live: %v", err)
	}
	if err := s.db.Create(&db.Listener{Name: "dead", Host: "127.0.0.1", Port: 0, Scheme: "tcp", Enabled: true, Status: "active"}).Error; err != nil {
		t.Fatalf("seed dead: %v", err)
	}
	if err := s.db.Create(&db.Listener{Name: "off", Host: "127.0.0.1", Port: 0, Scheme: "tcp", Enabled: false, Status: "active"}).Error; err != nil {
		t.Fatalf("seed off: %v", err)
	}

	// Reset the shared tracker so this test is deterministic.
	lhTracker.prune(map[uint]struct{}{})

	s.checkAllListeners()

	snapshot := lhTracker.snapshot()
	byName := map[string]listenerHealth{}
	for _, h := range snapshot {
		byName[h.Name] = h
	}
	if _, ok := byName["off"]; ok {
		t.Fatal("a disabled listener was probed")
	}

	var liveRow, deadRow db.Listener
	if err := s.db.Where("name = ?", "live").First(&liveRow).Error; err != nil {
		t.Fatalf("reload live: %v", err)
	}
	if err := s.db.Where("name = ?", "dead").First(&deadRow).Error; err != nil {
		t.Fatalf("reload dead: %v", err)
	}

	if !byName["live"].OK {
		t.Fatalf("the live row reported unhealthy: %+v", byName["live"])
	}
	if liveRow.Status == "error" {
		t.Fatal("the live row was marked as an error")
	}
	if byName["dead"].Failures != 1 {
		t.Fatalf("dead Failures = %d, want 1 after a single sweep", byName["dead"].Failures)
	}
	if deadRow.Status != "active" {
		t.Fatalf("the dead row flipped to %q after one failure, want the threshold to gate it", deadRow.Status)
	}

	// A second sweep crosses the threshold and flips exactly once.
	s.checkAllListeners()
	if err := s.db.Where("name = ?", "dead").First(&deadRow).Error; err != nil {
		t.Fatalf("reload dead: %v", err)
	}
	if deadRow.Status != "error" {
		t.Fatalf("status = %q, want %q at the failure threshold", deadRow.Status, "error")
	}
	snapshot = lhTracker.snapshot()
	byName = map[string]listenerHealth{}
	for _, h := range snapshot {
		byName[h.Name] = h
	}
	if byName["dead"].Failures != 2 {
		t.Fatalf("Failures = %d, want 2", byName["dead"].Failures)
	}

	// Recovery clears the flip.
	if err := s.db.Model(&db.Listener{}).Where("name = ?", "dead").Update("port", live).Error; err != nil {
		t.Fatalf("repair: %v", err)
	}
	s.checkAllListeners()
	if err := s.db.Where("name = ?", "dead").First(&deadRow).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if deadRow.Status == "error" {
		t.Fatal("a recovered row stayed marked as an error")
	}
}

// TestCheckAllListenersToleratesMissingContext proves the sweep works with no
// server context rather than panicking.
func TestCheckAllListenersToleratesMissingContext(t *testing.T) {
	s := mustTenantServer(t)
	s.ctx = nil
	if err := s.db.Create(&db.Listener{Name: "n", Host: "127.0.0.1", Port: 0, Scheme: "tcp", Enabled: true}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	lhTracker.prune(map[uint]struct{}{})
	s.checkAllListeners()
}
