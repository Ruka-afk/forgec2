package server

import (
	"context"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gorilla/websocket"
)

func tenantFanoutServer(t *testing.T) (*Server, map[string]chan []byte) {
	t.Helper()
	s := &Server{
		db:        testutil.SetupTestDB(t),
		ctx:       context.Background(),
		wsClients: make(map[*websocket.Conn]*wsClientConn),
	}
	if err := s.db.Create(&db.Implant{ID: "ws-a1", TenantID: 1}).Error; err != nil {
		t.Fatalf("seed agent: %v", err)
	}
	if err := s.db.Create(&db.Implant{ID: "ws-a2", TenantID: 2}).Error; err != nil {
		t.Fatalf("seed agent: %v", err)
	}
	chans := make(map[string]chan []byte)
	add := func(name string, tid uint) {
		ch := make(chan []byte, 8)
		chans[name] = ch
		conn := &websocket.Conn{}
		s.wsClients[conn] = &wsClientConn{
			conn:     conn,
			session:  UserSession{Username: name},
			tenantID: tid,
			ch:       ch,
			done:     make(chan struct{}),
		}
	}
	add("legacy", 0)
	add("t1", 1)
	add("t2", 2)
	return s, chans
}

func drain(ch chan []byte) int {
	n := 0
	for {
		select {
		case <-ch:
			n++
		default:
			return n
		}
	}
}

// TestBroadcastToTenantFilters proves agent-scoped events reach only legacy
// operators plus the agent's own tenant (previously every connected operator
// learned every tenant's hostnames, IDs and task types).
func TestBroadcastToTenantFilters(t *testing.T) {
	s, chans := tenantFanoutServer(t)

	s.broadcastTaskUpdate("ws-a1", db.Task{AgentID: "ws-a1", Type: "shell", Status: "completed", Result: "ok"})

	if n := drain(chans["legacy"]); n != 1 {
		t.Fatalf("legacy client got %d frames, want 1", n)
	}
	if n := drain(chans["t1"]); n != 1 {
		t.Fatalf("tenant-1 client got %d frames, want 1", n)
	}
	if n := drain(chans["t2"]); n != 0 {
		t.Fatalf("tenant-2 client got %d frames, want 0", n)
	}
}

// TestBroadcastToTenantLegacyAgentFansOut proves legacy (tenant-0) agents
// still reach every operator, matching tenantScope list semantics.
func TestBroadcastToTenantLegacyAgentFansOut(t *testing.T) {
	s, chans := tenantFanoutServer(t)
	if err := s.db.Create(&db.Implant{ID: "ws-leg", TenantID: 0}).Error; err != nil {
		t.Fatalf("seed agent: %v", err)
	}

	s.broadcastAgentOnline(db.Implant{ID: "ws-leg", Hostname: "LEG"}, true)

	for name, ch := range chans {
		if n := drain(ch); n != 1 {
			t.Fatalf("client %s got %d frames, want 1", name, n)
		}
	}
}

// TestBroadcastToTenantUnknownAgentLimitsToLegacy proves a broadcast for a
// missing agent row reaches legacy clients only (never a blind fan-out).
func TestBroadcastToTenantUnknownAgentLimitsToLegacy(t *testing.T) {
	s, chans := tenantFanoutServer(t)

	s.broadcastAgentDataUpdate("ws-ghost", map[string]interface{}{"x": 1})

	if n := drain(chans["legacy"]); n != 1 {
		t.Fatalf("legacy client got %d frames, want 1", n)
	}
	if n := drain(chans["t1"]); n != 0 {
		t.Fatalf("tenant-1 client got %d frames, want 0", n)
	}
	if n := drain(chans["t2"]); n != 0 {
		t.Fatalf("tenant-2 client got %d frames, want 0", n)
	}
}
