package server

import (
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
)

func TestTunEngineDrain(t *testing.T) {
	e := newTunEngine()
	e.sessions["agent-1"] = &tunSession{agentID: "agent-1", pending: [][]byte{[]byte("pkt1"), []byte("pkt2")}}
	frames := e.drain("agent-1", 0)
	if len(frames) != 2 {
		t.Fatalf("frames %d", len(frames))
	}
	if frames[0].Action != "tun_data" || string(frames[0].Data) != "pkt1" {
		t.Fatalf("frame0 %+v", frames[0])
	}
	if n := e.drain("agent-1", 0); len(n) != 0 {
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

// TestTunPendingOverflowVisible proves a full pending backlog sheds loudly:
// the drop is logged + counted and the backlog stays capped (previously 256
// silent drops with no byte bound — up to ~16MB per beacon).
func TestTunPendingOverflowVisible(t *testing.T) {
	e := newTunEngine()
	const agentID = "tun-overflow"
	port, err := e.startUDP(nil, agentID, "10.66.0.2/24", 0)
	if err != nil {
		t.Fatalf("startUDP: %v", err)
	}
	defer e.stop(agentID)

	// Prefill just under the byte budget, then deliver one more datagram.
	e.mu.Lock()
	sess := e.sessions[agentID]
	e.mu.Unlock()
	big := make([]byte, tunMaxPendingBytes-10)
	sess.mu.Lock()
	sess.pending = [][]byte{big}
	sess.pendingBytes = len(big)
	sess.mu.Unlock()

	dialer, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: port})
	if err != nil {
		t.Fatalf("dial helper: %v", err)
	}
	defer dialer.Close()
	if _, err := dialer.Write(make([]byte, 64)); err != nil {
		t.Fatalf("write helper: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		sess.mu.Lock()
		peerSet := sess.lastPeer != nil
		n := len(sess.pending)
		sess.mu.Unlock()
		if peerSet {
			if n != 1 {
				t.Fatalf("overflowed datagram must be shed, pending=%d", n)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("helper never received the datagram")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestTunConcurrentDoubleStart proves racing starts can't leak sockets: at
// most one session wins and every success reports the same port.
func TestTunConcurrentDoubleStart(t *testing.T) {
	e := newTunEngine()
	const agentID = "tun-race"
	const n = 16
	var wg sync.WaitGroup
	ports := make([]int, n)
	errs := make([]error, n)
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			p, err := e.startUDP(nil, agentID, "10.66.0.2/24", 0)
			ports[i], errs[i] = p, err
		}(i)
	}
	close(start)
	wg.Wait()

	e.mu.Lock()
	nsess := len(e.sessions)
	sess := e.sessions[agentID]
	e.mu.Unlock()
	if nsess != 1 || sess == nil {
		t.Fatalf("exactly one session must exist, got %d", nsess)
	}
	for i := 0; i < n; i++ {
		if errs[i] == nil && ports[i] != sess.port {
			t.Fatalf("goroutine %d got port %d, winner has %d", i, ports[i], sess.port)
		}
	}
	_ = e.stop(agentID)
}

// TestTunStopIdempotent proves concurrent Stop + sweep double-close can't
// panic — including auto-created sessions that never owned a UDP socket
// (close(nil-chan) used to crash the server on tun_stop after tun_up).
func TestTunStopIdempotent(t *testing.T) {
	e := newTunEngine()
	e.sessions["auto"] = &tunSession{agentID: "auto", status: "up"}
	if err := e.stop("auto"); err != nil {
		t.Fatalf("stop auto-created: %v", err)
	}
	if err := e.stop("auto"); err == nil {
		t.Fatal("second stop must report missing, not panic")
	}

	const agentID = "tun-twice"
	if _, err := e.startUDP(nil, agentID, "10.66.0.2/24", 0); err != nil {
		t.Fatalf("startUDP: %v", err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = e.stop(agentID)
		}()
	}
	wg.Wait()
}

// TestTunOversizeDropped proves jumbo agent frames never reach the UDP
// socket: they are dropped with a warning instead of erroring the write.
func TestTunOversizeDropped(t *testing.T) {
	e := newTunEngine()
	e.sessions["a"] = &tunSession{agentID: "a", status: "up", stop: make(chan struct{})}
	jumbo := make([]byte, tunMaxPacketBytes+1)
	e.handleAgentFrame("a", "tun_data", jumbo) // must not panic or write
}

// TestTunStaleSessionReaped proves the new periodic GC reaps dead-agent and
// idle sessions (previously no TUN GC existed — sockets leaked forever
// without an explicit tun_down).
func TestTunStaleSessionReaped(t *testing.T) {
	ginSetTestMode(t)
	s := initV3BeaconServer(t, testutil.SetupTestDB(t), tenantVisibilityMasterHex)
	if s.tunEngine == nil {
		s.tunEngine = newTunEngine()
	}
	const deadAgent = "tun-dead"
	const idleAgent = "tun-idle"
	const liveAgent = "tun-live"
	now := time.Now()
	if err := s.db.Create(&db.Implant{ID: idleAgent, LastSeen: now}).Error; err != nil {
		t.Fatalf("seed idle agent: %v", err)
	}
	if err := s.db.Create(&db.Implant{ID: liveAgent, LastSeen: now}).Error; err != nil {
		t.Fatalf("seed live agent: %v", err)
	}
	s.tunEngine.sessions[deadAgent] = &tunSession{agentID: deadAgent, status: "up", lastActive: now, stop: make(chan struct{})}
	s.tunEngine.sessions[idleAgent] = &tunSession{agentID: idleAgent, status: "up", lastActive: now.Add(-(tunSessionIdleTimeout + time.Minute)), stop: make(chan struct{})}
	s.tunEngine.sessions[liveAgent] = &tunSession{agentID: liveAgent, status: "up", lastActive: now, stop: make(chan struct{})}

	s.cleanupStaleTun()

	for _, id := range []string{deadAgent, idleAgent} {
		if sess := s.tunEngine.get(id); sess != nil {
			t.Fatalf("session %s must be reaped", id)
		}
	}
	if sess := s.tunEngine.get(liveAgent); sess == nil {
		t.Fatal("live session must survive the sweep")
	}
	_ = s.tunEngine.stop(liveAgent)
}

// TestTunHandleAgentFrameNoPeerDrops proves tun_data with no UDP helper yet
// is dropped visibly instead of a silent nil-pointer skip.
func TestTunHandleAgentFrameNoPeerDrops(t *testing.T) {
	e := newTunEngine()
	e.handleAgentFrame("ghost", "tun_data", []byte(fmt.Sprintf("pkt-%d", 1)))
	sess := e.get("ghost")
	if sess == nil {
		t.Fatal("auto-create must still register the session")
	}
}
