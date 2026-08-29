package server

import (
	"testing"
)

func TestTunEngineDrain(t *testing.T) {
	e := newTunEngine()
	e.sessions["agent-1"] = &tunSession{agentID: "agent-1", pending: [][]byte{[]byte("pkt1"), []byte("pkt2")}}
	frames := e.drain("agent-1")
	if len(frames) != 2 {
		t.Fatalf("frames %d", len(frames))
	}
	if frames[0].Action != "tun_data" || string(frames[0].Data) != "pkt1" {
		t.Fatalf("frame0 %+v", frames[0])
	}
	if n := e.drain("agent-1"); len(n) != 0 {
		t.Fatalf("second drain %d", len(n))
	}
}

func TestTunEngineActive(t *testing.T) {
	e := newTunEngine()
	if e.active("nope") {
		t.Fatal("missing should be inactive")
	}
	e.sessions["a"] = &tunSession{status: "up"}
	if !e.active("a") {
		t.Fatal("up should be active")
	}
}
