package server

import (
	"net"
	"testing"
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
