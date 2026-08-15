//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"fmt"
	"net"
	"testing"
	"time"
)

// stubConn is a minimal net.Conn used to exercise the P2P connection limiter
// without opening real sockets.
type stubConn struct{ raddr net.Addr }

func (s *stubConn) Read([]byte) (int, error)         { return 0, nil }
func (s *stubConn) Write([]byte) (int, error)        { return 0, nil }
func (s *stubConn) Close() error                     { return nil }
func (s *stubConn) LocalAddr() net.Addr              { return nil }
func (s *stubConn) RemoteAddr() net.Addr             { return s.raddr }
func (s *stubConn) SetDeadline(time.Time) error      { return nil }
func (s *stubConn) SetReadDeadline(time.Time) error  { return nil }
func (s *stubConn) SetWriteDeadline(time.Time) error { return nil }

type stubAddr string

func (a stubAddr) Network() string { return "tcp" }
func (a stubAddr) String() string  { return string(a) }

// resetP2PConnLimiter clears the package-level limiter state between tests.
func resetP2PConnLimiter() {
	p2pConnLimiterMu.Lock()
	defer p2pConnLimiterMu.Unlock()
	p2pConnTotal = 0
	p2pConnByIP = map[string]int{}
}

// TestP2PConnLimiterPerSource verifies a single source cannot open more than
// p2pMaxConnsPerIP relay sockets (B3), while freeing a slot lets a new one in.
func TestP2PConnLimiterPerSource(t *testing.T) {
	resetP2PConnLimiter()
	defer resetP2PConnLimiter()

	ip := "10.0.0.9"
	conns := make([]*stubConn, 0, p2pMaxConnsPerIP+1)
	defer func() {
		for _, c := range conns {
			p2pEndConn(c)
		}
	}()

	accepted := 0
	for i := 0; i < p2pMaxConnsPerIP; i++ {
		c := &stubConn{raddr: stubAddr(ip + ":5000")}
		if p2pAcceptConn(c) {
			conns = append(conns, c)
			accepted++
		}
	}
	if accepted != p2pMaxConnsPerIP {
		t.Fatalf("expected %d accepts from one source, got %d", p2pMaxConnsPerIP, accepted)
	}

	// One more from the same source must be rejected.
	over := &stubConn{raddr: stubAddr(ip + ":5001")}
	if p2pAcceptConn(over) {
		t.Fatal("connection from an already-saturated source should be rejected")
	}

	// Freeing a slot allows a new connection from that source.
	p2pEndConn(conns[0])
	conns = conns[1:]
	if !p2pAcceptConn(over) {
		t.Fatal("after freeing a slot, a new connection should be accepted")
	}
	p2pEndConn(over)
}

// TestP2PConnLimiterGlobal verifies the global connection budget caps total
// relay sockets regardless of source (B3).
func TestP2PConnLimiterGlobal(t *testing.T) {
	resetP2PConnLimiter()
	defer resetP2PConnLimiter()

	conns := make([]*stubConn, 0, p2pMaxTotalConns+1)
	defer func() {
		for _, c := range conns {
			p2pEndConn(c)
		}
	}()

	accepted := 0
	for i := 0; i < p2pMaxTotalConns+5; i++ {
		c := &stubConn{raddr: stubAddr(fmt.Sprintf("10.0.%d.%d:5000", i/255, i%255))}
		if p2pAcceptConn(c) {
			conns = append(conns, c)
			accepted++
		}
	}
	if accepted != p2pMaxTotalConns {
		t.Fatalf("expected global cap %d, got %d", p2pMaxTotalConns, accepted)
	}
}

// TestP2PChildUUIDBindingRejectsSpoof verifies the parent binds each child
// connection to the first UUID it claims and rejects a second connection that
// tries to impersonate an already-owned (or any) child UUID, preventing relay
// queue poisoning / impersonation via unauthenticated P2P connections.
func TestP2PChildUUIDBindingRejectsSpoof(t *testing.T) {
	// Use non-zero-size types: Go can share the address of zero-size values
	// (e.g. &struct{}{}), which would make connA and connB compare equal.
	connA := &struct{ id int }{1}
	connB := &struct{ id int }{2}
	defer p2pReleaseChild(connA)
	defer p2pReleaseChild(connB)

	victim := "victim-uuid"

	// First connection claims the victim successfully.
	if !p2pClaimChild(connA, victim) {
		t.Fatal("first connection should be able to claim the victim UUID")
	}
	// A second connection claiming the same victim is rejected.
	if p2pClaimChild(connB, victim) {
		t.Fatal("second connection must be rejected for an already-owned UUID")
	}
	// The second connection may claim a *different* UUID (separate child).
	if !p2pClaimChild(connB, "other-uuid") {
		t.Fatal("second connection should claim a different UUID")
	}
	// The first connection is bound and must not claim a different UUID.
	if p2pClaimChild(connA, "other-uuid") {
		t.Fatal("connection bound to victim must not claim a different UUID")
	}
}
