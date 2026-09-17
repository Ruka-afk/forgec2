package server

import (
	"bytes"
	"context"
	"net"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// TestSocksOutboundOverflowClosesConn proves a connection whose unsent
// backlog exceeds the byte cap is closed loudly (close frame queued) instead
// of silently shedding stream bytes.
func TestSocksOutboundOverflowClosesConn(t *testing.T) {
	e := newSocksRelayEngine()
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	e.connections[7] = &socksRelayConn{connID: 7, tcpConn: server, agentID: "a1"}

	// Fill just under the cap, then push over it.
	big := make([]byte, socksMaxOutboundBytes-10)
	e.enqueueFrame("a1", socksFrame{ConnID: 7, Action: "data", Data: big})
	e.enqueueFrame("a1", socksFrame{ConnID: 7, Action: "data", Data: make([]byte, 64)})

	e.mu.Lock()
	_, listed := e.connections[7]
	e.mu.Unlock()
	if listed {
		t.Fatal("overflowed conn must be dropped from the table")
	}
	e.controlFramesMu.Lock()
	closes := 0
	for _, f := range e.controlFrames["a1"] {
		if f.ConnID == 7 && f.Action == "close" {
			closes++
		}
	}
	e.controlFramesMu.Unlock()
	if closes != 1 {
		t.Fatalf("want exactly 1 close frame for dropped conn, got %d", closes)
	}
}

// TestSocksWriteFailureDropsConn proves a failed operator write removes the
// entry (previously it stayed listed until the 5-minute sweep, holding one
// of the 256 slots while the agent sent into the void).
func TestSocksWriteFailureDropsConn(t *testing.T) {
	e := newSocksRelayEngine()
	client, server := net.Pipe()
	e.connections[9] = &socksRelayConn{connID: 9, tcpConn: server, agentID: "a1"}
	client.Close() // reads fail -> writes fail

	s := &Server{}
	s.db = newTasksTestServer(t).db
	e.processAgentData(s, "a1", []socksFrame{{ConnID: 9, Action: "data", Data: []byte("hello")}})

	e.mu.Lock()
	_, listed := e.connections[9]
	e.mu.Unlock()
	if listed {
		t.Fatal("write-failed conn must be dropped from the table")
	}
	server.Close()
}

// TestFrameBudgetForTransport pins the MTU policy: datagram transports get
// small frames + a capped total, streams drain everything uncapped.
func TestFrameBudgetForTransport(t *testing.T) {
	for _, tr := range []string{"udp", "icmp", "dns"} {
		size, budget := frameBudgetForTransport(tr)
		if size != SocksLowMTUFrameSize || budget != SocksLowMTUBudget {
			t.Fatalf("%s: got (%d,%d)", tr, size, budget)
		}
	}
	for _, tr := range []string{"http", "tcp", "grpc", "quic", "ssh", "smb", "", "bogus"} {
		size, budget := frameBudgetForTransport(tr)
		if size != 0 || budget != 0 {
			t.Fatalf("%s: want uncapped, got (%d,%d)", tr, size, budget)
		}
	}
}

// TestLowMTUBudgetRequeuesLosslessly proves a datagram budget cuts the tunnel
// payload without loss or reorder: the remainder stays queued head-ordered
// for the next beacon, across all three data FIFOs.
func TestLowMTUBudgetRequeuesLosslessly(t *testing.T) {
	e := newSocksRelayEngine()
	const agentID = "mtu-1"

	// Rportfwd FIFO: 3×2KB frames, 4KB budget → 2 now, 1 requeued.
	for i := 0; i < 3; i++ {
		if !e.enqueueRPortFwdFrame(agentID, socksFrame{ConnID: 1, Action: "rportfwd_data", Data: bytes.Repeat([]byte{byte(i + 1)}, 2048)}) {
			t.Fatal("enqueue must fit the 2MB FIFO budget")
		}
	}
	first := e.collectPendingFrames(agentID, 1024, 4096)
	if len(first) != 2 {
		t.Fatalf("want 2 frames within budget, got %d", len(first))
	}
	rest := e.collectPendingFrames(agentID, 0, 0)
	if len(rest) != 1 || !bytes.Equal(rest[0].Data, bytes.Repeat([]byte{3}, 2048)) {
		t.Fatalf("requeued frame lost or reordered: %+v", rest)
	}

	// SOCKS data: 8KB stream, 1KB frames, 4KB budget → 4 frames now and a
	// byte-identical 4KB remainder later.
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	stream := append(bytes.Repeat([]byte{'A'}, 4096), bytes.Repeat([]byte{'B'}, 4096)...)
	e.connections[11] = &socksRelayConn{connID: 11, tcpConn: server, agentID: agentID,
		outbound: [][]byte{stream}, outboundBytes: len(stream)}
	got := e.collectPendingFrames(agentID, 1024, 4096)
	if len(got) != 4 {
		t.Fatalf("want 4×1KB frames, got %d", len(got))
	}
	var head []byte
	for _, f := range got {
		head = append(head, f.Data...)
	}
	tail := e.collectPendingFrames(agentID, 0, 0)
	var tailBytes []byte
	for _, f := range tail {
		if f.Action == "data" {
			tailBytes = append(tailBytes, f.Data...)
		}
	}
	if !bytes.Equal(append(head, tailBytes...), stream) {
		t.Fatal("budgeted SOCKS drain corrupted the byte stream")
	}
}

// TestSocksStopAllDrains proves server shutdown releases every relay
// listener + connection (previously FDs and accept loops leaked across the
// restart path) and is safe to run twice.
func TestSocksStopAllDrains(t *testing.T) {
	e := newSocksRelayEngine()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	ctx, cancel := context.WithCancel(context.Background())
	e.sessions["a9"] = &socksRelaySession{agentID: "a9", listener: ln, ctx: ctx, cancel: cancel}
	client, server := net.Pipe()
	defer client.Close()
	e.connections[3] = &socksRelayConn{connID: 3, tcpConn: server, agentID: "a9", lastActive: time.Now()}

	e.stopAll()
	e.stopAll() // idempotent

	e.mu.Lock()
	nsess, nconn := len(e.sessions), len(e.connections)
	e.mu.Unlock()
	if nsess != 0 || nconn != 0 {
		t.Fatalf("tables not drained: sessions=%d conns=%d", nsess, nconn)
	}
	if c, err := net.Dial("tcp", addr); err == nil {
		c.Close()
		t.Fatal("listener still accepting after stopAll")
	}
}

// TestSocksDropsCounted proves formerly-silent frame sheds are visible in
// SocksDroppedTotal: unknown-conn data (either direction), foreign-conn
// replay attempts, and malformed UDP datagrams.
func TestSocksDropsCounted(t *testing.T) {
	e := newSocksRelayEngine()
	counter := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "test_socks_dropped_total"}, []string{"reason"})
	e.SetDropCounter(counter)
	s := &Server{}

	// Agent→server data for a conn that doesn't exist.
	e.processAgentData(s, "a", []socksFrame{{ConnID: 77, Action: "data", Data: []byte("x")}})
	// Agent→server data for another agent's live conn (replay attempt).
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	e.connections[78] = &socksRelayConn{connID: 78, tcpConn: server, agentID: "b"}
	e.processAgentData(s, "a", []socksFrame{{ConnID: 78, Action: "data", Data: []byte("x")}})
	// Malformed UDP datagram on a live UDP leg.
	udpConn, _ := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	defer udpConn.Close()
	e.connections[79] = &socksRelayConn{connID: 79, agentID: "a", isUDP: true, udpConn: udpConn, udpClient: &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 9}}
	e.processAgentData(s, "a", []socksFrame{{ConnID: 79, Action: "udp_data", Data: []byte("short")}})
	// Server→agent data for a dead leg.
	e.enqueueFrame("a", socksFrame{ConnID: 80, Action: "data", Data: []byte("y")})

	count := func(reason string) float64 {
		return testutil.ToFloat64(counter.WithLabelValues(reason))
	}
	if got := count("unknown_conn"); got != 2 {
		t.Fatalf("unknown_conn=%v, want 2 (both directions)", got)
	}
	if got := count("foreign_conn"); got != 1 {
		t.Fatalf("foreign_conn=%v, want 1", got)
	}
	if got := count("udp_drop"); got != 1 {
		t.Fatalf("udp_drop=%v, want 1", got)
	}
}
