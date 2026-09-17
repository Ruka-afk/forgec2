package server

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
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

// TestLPortFwdConnFloodCapped proves a connect flood past the per-agent cap
// is refused (close queued for the agent) instead of pinning FDs + pump
// goroutines without bound.
func TestLPortFwdConnFloodCapped(t *testing.T) {
	ginSetTestMode(t)
	s := initV3BeaconServer(t, testutil.SetupTestDB(t), tenantVisibilityMasterHex)
	const agentID = "lpf-flood"
	const target = "127.0.0.1:9"
	s.registerLPortFwdDecl(agentID, target)

	for i := 0; i < lportfwdMaxConnsPerAgent; i++ {
		client, server := net.Pipe()
		defer client.Close()
		defer server.Close()
		key := lportfwdKey(agentID, uint64(i+1))
		s.lportfwdTargets[key] = &lportfwdTarget{agentID: agentID, target: target, tcpConn: server, lastActive: time.Now()}
	}

	s.processLPortFwdData(agentID, socksFrame{ConnID: 9999, Action: "lportfwd_connect", Data: []byte(target)})

	s.lportfwdMu.Lock()
	_, tracked := s.lportfwdTargets[lportfwdKey(agentID, 9999)]
	n := 0
	for _, tg := range s.lportfwdTargets {
		if tg.agentID == agentID {
			n++
		}
	}
	s.lportfwdMu.Unlock()
	if tracked || n != lportfwdMaxConnsPerAgent {
		t.Fatalf("over-cap connect must be refused, tracked=%v count=%d", tracked, n)
	}
	var sawClose bool
	for _, f := range s.socksEngine.collectLPortFwdFrames(agentID) {
		if f.ConnID == 9999 && f.Action == "lportfwd_close" {
			sawClose = true
		}
	}
	if !sawClose {
		t.Fatal("refused connect must queue a close for the agent")
	}
}

// TestLPortFwdQueueOverflowClosesLeg proves a full dedicated queue closes the
// target leg loudly: the next target payload tears the leg down (with a close
// for the agent) instead of silently corrupting the TCP stream.
func TestLPortFwdQueueOverflowClosesLeg(t *testing.T) {
	ginSetTestMode(t)
	s := initV3BeaconServer(t, testutil.SetupTestDB(t), tenantVisibilityMasterHex)
	const agentID = "lpf-overflow"

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
	_, portStr, _ := net.SplitHostPort(targetLn.Addr().String())
	declaredTarget := net.JoinHostPort("127.0.0.1", portStr)
	s.registerLPortFwdDecl(agentID, declaredTarget)

	s.processLPortFwdData(agentID, socksFrame{ConnID: 42, Action: "lportfwd_connect", Data: []byte(declaredTarget)})
	var target net.Conn
	select {
	case target = <-targetConnCh:
	case <-time.After(3 * time.Second):
		t.Fatal("server never dialed the target")
	}
	defer target.Close()

	// Fill the 2MB dedicated budget, then push target data through the pump.
	big := make([]byte, lportfwdMaxBytesPerAgent-10)
	if !s.socksEngine.enqueueLPortFwdFrame(agentID, socksFrame{ConnID: 42, Action: "lportfwd_data", Data: big}) {
		t.Fatal("first enqueue must fit")
	}
	if _, err := target.Write(make([]byte, 64)); err != nil {
		t.Fatalf("target write: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	closed := false
	for time.Now().Before(deadline) && !closed {
		s.lportfwdMu.Lock()
		_, closed = s.lportfwdTargets[lportfwdKey(agentID, 42)]
		s.lportfwdMu.Unlock()
		closed = !closed
		time.Sleep(10 * time.Millisecond)
	}
	if !closed {
		t.Fatal("overflowed leg must be torn down loudly")
	}
	var sawClose bool
	for _, f := range s.socksEngine.collectLPortFwdFrames(agentID) {
		if f.ConnID == 42 && f.Action == "lportfwd_close" {
			sawClose = true
		}
	}
	if !sawClose {
		t.Fatal("overflow close notification for agent missing")
	}
}

// TestLPortFwdCloseBypassesFullQueue proves the teardown notification survives
// a full backlog (previously a full 200-slot control queue swallowed it and
// leaked both legs until the 5-minute sweep).
func TestLPortFwdCloseBypassesFullQueue(t *testing.T) {
	ginSetTestMode(t)
	s := initV3BeaconServer(t, testutil.SetupTestDB(t), tenantVisibilityMasterHex)
	const agentID = "lpf-close"

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	s.lportfwdTargets[lportfwdKey(agentID, 7)] = &lportfwdTarget{agentID: agentID, target: "127.0.0.1:9", tcpConn: server, lastActive: time.Now()}

	// Saturate the data budget with filler frames.
	filler := make([]byte, 64*1024)
	for s.socksEngine.enqueueLPortFwdFrame(agentID, socksFrame{ConnID: 8, Action: "lportfwd_data", Data: filler}) {
	}
	s.lportfwdClose(agentID, 7)

	var sawClose bool
	for _, f := range s.socksEngine.collectLPortFwdFrames(agentID) {
		if f.ConnID == 7 && f.Action == "lportfwd_close" {
			sawClose = true
		}
	}
	if !sawClose {
		t.Fatal("close must bypass a full data queue")
	}
}

// TestLPortFwdIdleReaped proves legs idle past the timeout are reaped even
// for a live agent, while active legs survive.
func TestLPortFwdIdleReaped(t *testing.T) {
	ginSetTestMode(t)
	s := initV3BeaconServer(t, testutil.SetupTestDB(t), tenantVisibilityMasterHex)
	const agentID = "lpf-idle"
	if err := s.db.Create(&db.Implant{ID: agentID, LastSeen: time.Now()}).Error; err != nil {
		t.Fatalf("seed agent: %v", err)
	}

	staleClient, staleServer := net.Pipe()
	defer staleClient.Close()
	defer staleServer.Close()
	freshClient, freshServer := net.Pipe()
	defer freshClient.Close()
	defer freshServer.Close()
	s.lportfwdTargets[lportfwdKey(agentID, 1)] = &lportfwdTarget{agentID: agentID, tcpConn: staleServer, lastActive: time.Now().Add(-(lportfwdConnIdleTimeout + time.Minute))}
	s.lportfwdTargets[lportfwdKey(agentID, 2)] = &lportfwdTarget{agentID: agentID, tcpConn: freshServer, lastActive: time.Now()}

	s.cleanupStaleLPortFwd()

	s.lportfwdMu.Lock()
	_, staleGone := s.lportfwdTargets[lportfwdKey(agentID, 1)]
	_, freshKept := s.lportfwdTargets[lportfwdKey(agentID, 2)]
	s.lportfwdMu.Unlock()
	if staleGone {
		t.Fatal("idle leg must be reaped")
	}
	if !freshKept {
		t.Fatal("active leg must survive the sweep")
	}
}
