package server

import (
	"net"
	"testing"
	"time"
)

// TestRPortFwdDedicatedQueueIsolated proves rportfwd frames land on the
// dedicated FIFO, not the 200-slot SOCKS control queue (bulk tunnel data
// used to evict closes and vice versa).
func TestRPortFwdDedicatedQueueIsolated(t *testing.T) {
	s := newTasksTestServer(t)
	s.socksEngine = newSocksRelayEngine()
	s.rportfwdListeners = make(map[string]*rportfwdRelay)

	s.sendRPortFwdFrame("a1", 1, "rportfwd_data", []byte("hello"))

	if n := len(s.socksEngine.collectRPortFwdFrames("a1", 0)); n != 1 {
		t.Fatalf("want 1 rportfwd frame, got %d", n)
	}
	s.socksEngine.controlFramesMu.Lock()
	n := len(s.socksEngine.controlFrames["a1"])
	s.socksEngine.controlFramesMu.Unlock()
	if n != 0 {
		t.Fatalf("rportfwd frames must not pollute control queue, got %d", n)
	}
	// Drained queue keeps connect -> data -> close order.
	s.sendRPortFwdFrame("a1", 1, "rportfwd_connect", []byte("h:1"))
	s.sendRPortFwdFrame("a1", 1, "rportfwd_data", []byte("d"))
	s.sendRPortFwdFrame("a1", 1, "rportfwd_close", nil)
	frames := s.socksEngine.collectRPortFwdFrames("a1", 0)
	if len(frames) != 3 || frames[0].Action != "rportfwd_connect" || frames[2].Action != "rportfwd_close" {
		t.Fatalf("rportfwd FIFO order broken: %+v", frames)
	}
}

// TestRPortFwdQueueOverflowClosesConn proves a full rportfwd queue closes the
// operator leg loudly instead of silently corrupting the stream.
func TestRPortFwdQueueOverflowClosesConn(t *testing.T) {
	s := newTasksTestServer(t)
	s.socksEngine = newSocksRelayEngine()
	s.rportfwdListeners = make(map[string]*rportfwdRelay)

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	connID := s.rportfwdNextID.Add(1)
	s.rportfwdListeners["a1:1081"] = &rportfwdRelay{
		agentID:  "a1",
		stopCh:   make(chan struct{}),
		listener: &rportfwdListener{connMap: map[uint64]net.Conn{connID: server}},
	}

	// Fill the 2MB budget, then push over it.
	big := make([]byte, rportfwdMaxBytesPerAgent-10)
	if !s.socksEngine.enqueueRPortFwdFrame("a1", socksFrame{ConnID: connID, Action: "rportfwd_data", Data: big}) {
		t.Fatal("first enqueue must fit")
	}
	s.sendRPortFwdFrame("a1", connID, "rportfwd_data", make([]byte, 64))

	s.rportfwdMu.Lock()
	relay := s.rportfwdListeners["a1:1081"]
	relay.listener.mu.Lock()
	_, listed := relay.listener.connMap[connID]
	relay.listener.mu.Unlock()
	s.rportfwdMu.Unlock()
	if listed {
		t.Fatal("overflowed operator conn must be closed and removed")
	}
}

// TestRPortFwdConnIDIsolation proves two relays for one agent don't
// cross-deliver: conn IDs are globally unique so a data frame reaches exactly
// one operator socket.
func TestRPortFwdConnIDIsolation(t *testing.T) {
	s := newTasksTestServer(t)
	s.socksEngine = newSocksRelayEngine()
	s.rportfwdListeners = make(map[string]*rportfwdRelay)

	c1, s1 := net.Pipe()
	defer c1.Close()
	defer s1.Close()
	c2, s2 := net.Pipe()
	defer c2.Close()
	defer s2.Close()

	id1 := s.rportfwdNextID.Add(1)
	id2 := s.rportfwdNextID.Add(1)
	if id1 == id2 {
		t.Fatal("conn IDs must be globally unique across relays")
	}
	s.rportfwdListeners["a1:1081"] = &rportfwdRelay{
		agentID: "a1", stopCh: make(chan struct{}),
		listener: &rportfwdListener{connMap: map[uint64]net.Conn{id1: s1}},
	}
	s.rportfwdListeners["a1:1082"] = &rportfwdRelay{
		agentID: "a1", stopCh: make(chan struct{}),
		listener: &rportfwdListener{connMap: map[uint64]net.Conn{id2: s2}},
	}

	go s.processRPortFwdData("a1", socksFrame{ConnID: id1, Action: "rportfwd_data", Data: []byte("ping")})

	buf := make([]byte, 16)
	_ = c1.SetReadDeadline(time.Now().Add(5 * time.Second))
	if n, err := c1.Read(buf); err != nil || string(buf[:n]) != "ping" {
		t.Fatalf("relay :1081 must get the frame, err=%v", err)
	}
	_ = c2.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	if n, _ := c2.Read(buf); n != 0 {
		t.Fatal("relay :1082 must NOT receive another relay's frame")
	}
}

// TestRPortFwdStopIdempotent proves concurrent Stop + sweep double-close
// can't panic (bare close(stopCh) used to).
func TestRPortFwdStopIdempotent(t *testing.T) {
	r := &rportfwdRelay{stopCh: make(chan struct{}), listener: &rportfwdListener{connMap: map[uint64]net.Conn{}}}
	r.stop()
	r.stop() // must not panic
}

// TestRPortFwdWriteFailureCloses proves a dead operator socket is removed and
// the agent is told to tear down its side.
func TestRPortFwdWriteFailureCloses(t *testing.T) {
	s := newTasksTestServer(t)
	s.socksEngine = newSocksRelayEngine()
	s.rportfwdListeners = make(map[string]*rportfwdRelay)

	client, server := net.Pipe()
	client.Close() // writes fail
	connID := s.rportfwdNextID.Add(1)
	s.rportfwdListeners["a1:1081"] = &rportfwdRelay{
		agentID: "a1", stopCh: make(chan struct{}),
		listener: &rportfwdListener{connMap: map[uint64]net.Conn{connID: server}},
	}
	defer server.Close()

	s.processRPortFwdData("a1", socksFrame{ConnID: connID, Action: "rportfwd_data", Data: []byte("hello")})

	s.rportfwdMu.Lock()
	_, listed := s.rportfwdListeners["a1:1081"].listener.connMap[connID]
	s.rportfwdMu.Unlock()
	if listed {
		t.Fatal("write-failed conn must be removed")
	}
	closes := 0
	for _, f := range s.socksEngine.collectRPortFwdFrames("a1", 0) {
		if f.ConnID == connID && f.Action == "rportfwd_close" {
			closes++
		}
	}
	if closes != 1 {
		t.Fatalf("want exactly 1 rportfwd_close to agent, got %d", closes)
	}
}

// TestRPortFwdStopAllDrains proves shutdown clears every relay, closes
// listeners and operator legs, and is safe to run twice.
func TestRPortFwdStopAllDrains(t *testing.T) {
	s := newTasksTestServer(t)
	s.socksEngine = newSocksRelayEngine()
	s.rportfwdListeners = make(map[string]*rportfwdRelay)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	client, server := net.Pipe()
	defer client.Close()
	r := &rportfwdRelay{server: s, agentID: "a9", stopCh: make(chan struct{}),
		listener: &rportfwdListener{ln: ln, connMap: map[uint64]net.Conn{1: server}}}
	s.rportfwdListeners["a9:1081"] = r
	go r.acceptLoop()
	time.Sleep(50 * time.Millisecond)

	s.stopAllRPortFwd()
	s.stopAllRPortFwd() // idempotent

	s.rportfwdMu.Lock()
	n := len(s.rportfwdListeners)
	s.rportfwdMu.Unlock()
	if n != 0 {
		t.Fatalf("relays not cleared: %d", n)
	}
	if c, err := net.Dial("tcp", addr); err == nil {
		c.Close()
		t.Fatal("relay listener still accepting after stopAll")
	}
}

// TestRPortFwdAtConnCapRefuses proves the 64-conn cap refuses new operator
// connections instead of exhausting FDs.
func TestRPortFwdAtConnCapRefuses(t *testing.T) {
	s := newTasksTestServer(t)
	s.socksEngine = newSocksRelayEngine()
	r := &rportfwdRelay{server: s, agentID: "a1", stopCh: make(chan struct{}),
		listener: &rportfwdListener{connMap: map[uint64]net.Conn{}}}
	for i := 0; i < rportfwdMaxConns; i++ {
		c, _ := net.Pipe()
		defer c.Close()
		r.listener.connMap[uint64(i+1)] = c
	}
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	r.handleConn(server)
	r.listener.mu.Lock()
	n := len(r.listener.connMap)
	r.listener.mu.Unlock()
	if n != rportfwdMaxConns {
		t.Fatalf("at-cap relay must refuse new conns, size=%d", n)
	}
}
