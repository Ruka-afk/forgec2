package server

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/forgec2/forgec2/pkg/protocol"
)

// TestLPortFwdServerRelay exercises the teamserver half: connect frame dials
// the target, data frames flow both directions through the socksEngine queue,
// and close tears the target leg down.
func TestLPortFwdServerRelay(t *testing.T) {
	ginSetTestMode(t)
	s := initV3BeaconServer(t, testutil.SetupTestDB(t), tenantVisibilityMasterHex)
	const agentID = "lpf-agent-1"

	// Fake target the server will dial.
	targetLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("target listen: %v", err)
	}
	defer targetLn.Close()
	targetConnCh := make(chan net.Conn, 1)
	go func() {
		c, err := targetLn.Accept()
		if err == nil {
			targetConnCh <- c
		}
	}()

	// Operator declares the target via lportfwd_start (P1: connect frames
	// must match a declared target — undeclared ones are rejected).
	_, targetStr, _ := net.SplitHostPort(targetLn.Addr().String())
	declaredTarget := net.JoinHostPort("127.0.0.1", targetStr)
	s.registerLPortFwdDecl(agentID, declaredTarget)

	// Agent announces a tunneled connection.
	s.processLPortFwdData(agentID, socksFrame{ConnID: 42, Action: "lportfwd_connect", Data: []byte(declaredTarget)})

	var target net.Conn
	select {
	case target = <-targetConnCh:
	case <-time.After(3 * time.Second):
		t.Fatal("server never dialed the target")
	}
	defer target.Close()

	// Agent -> target payload.
	s.processLPortFwdData(agentID, socksFrame{ConnID: 42, Action: "lportfwd_data", Data: []byte("ping")})
	target.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 4)
	if _, err := io.ReadFull(target, buf); err != nil {
		t.Fatalf("target read: %v", err)
	}
	if string(buf) != "ping" {
		t.Fatalf("target got %q", string(buf))
	}

	// Target -> agent payload must land in the outbound queue.
	if _, err := target.Write([]byte("pong")); err != nil {
		t.Fatalf("target write: %v", err)
	}
	var gotPong bool
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !gotPong {
		for _, f := range s.collectSocksFrames(agentID) {
			if f.Action == "lportfwd_data" && string(f.Data) == "pong" {
				gotPong = true
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !gotPong {
		t.Fatal("outbound queue never carried the target response")
	}

	// Close frame tears down and notifies the agent.
	s.processLPortFwdData(agentID, socksFrame{ConnID: 42, Action: "lportfwd_close"})
	time.Sleep(50 * time.Millisecond)
	s.lportfwdMu.Lock()
	_, stillTracked := s.lportfwdTargets[lportfwdKey(agentID, 42)]
	s.lportfwdMu.Unlock()
	if stillTracked {
		t.Fatal("connection still tracked after close")
	}
	var sawClose bool
	for _, f := range s.collectSocksFrames(agentID) {
		if f.Action == "lportfwd_close" && f.ConnID == 42 {
			sawClose = true
		}
	}
	if !sawClose {
		t.Fatal("close notification for agent missing")
	}
}

// TestLPortFwdUndeclaredTargetRejected pins the P1 fix: a connect frame for
// a target the operator never declared must be refused without dialing.
func TestLPortFwdUndeclaredTargetRejected(t *testing.T) {
	ginSetTestMode(t)
	s := initV3BeaconServer(t, testutil.SetupTestDB(t), tenantVisibilityMasterHex)

	targetLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("target listen: %v", err)
	}
	defer targetLn.Close()
	dialed := make(chan net.Conn, 1)
	go func() {
		c, err := targetLn.Accept()
		if err == nil {
			dialed <- c
		}
	}()

	s.processLPortFwdData("lpf-agent-2", socksFrame{ConnID: 7, Action: "lportfwd_connect", Data: []byte(targetLn.Addr().String())})

	select {
	case c := <-dialed:
		c.Close()
		t.Fatal("undeclared target was dialed")
	case <-time.After(500 * time.Millisecond):
		// expected: no dial
	}
	s.lportfwdMu.Lock()
	n := len(s.lportfwdTargets)
	s.lportfwdMu.Unlock()
	if n != 0 {
		t.Fatalf("rejected connect left tracked targets: %d", n)
	}
}

// TestLPortFwdConfigGate pins the kill switch: task creation is refused when
// server.lportfwd_enabled is false and allowed when true.
func TestLPortFwdConfigGate(t *testing.T) {
	s := newTasksTestServer(t)
	s.cfg = &config.Config{}

	s.cfg.Server.LPortFwdEnabled = false
	if _, err := s.createTask("agent-x", protocol.TaskTypeLPortFwdStart, "8080|127.0.0.1:80", "", "", "", 0, 0); err == nil {
		t.Fatal("lportfwd_start must be refused when disabled")
	}

	s.cfg.Server.LPortFwdEnabled = true
	if _, err := s.createTask("agent-x", protocol.TaskTypeLPortFwdStart, "8080|127.0.0.1:80", "", "", "", 0, 0); err != nil {
		t.Fatalf("lportfwd_start must be allowed when enabled: %v", err)
	}
}
