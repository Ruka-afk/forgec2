package server

import (
	"fmt"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
)

// newMonitorTestServer builds a bare Server whose agent-offline broadcast
// path is safe to call: the cooldown map is initialized and the WS client
// map is nil (a no-op broadcast).
func newMonitorTestServer(t *testing.T, offlineThresholdSec int) *Server {
	t.Helper()
	cfg := &config.Config{}
	cfg.Server.OfflineThreshold = offlineThresholdSec
	s := &Server{
		db:                  newCleanupTestDB(t),
		cfg:                 cfg,
		agentStatusCooldown: make(map[string]time.Time),
	}
	return s
}

// TestCheckAgentAlertsReapsStaleAndOffline pins the sweep's transition
// semantics against the per-agent thresholds: a fresh agent is untouched,
// an online agent past the offline threshold flaps to stale, and a stale
// agent past the stale threshold (3x) is reaped to offline — with one
// status event recorded per transition.
func TestCheckAgentAlertsReapsStaleAndOffline(t *testing.T) {
	const offlineSec = 30 // stale threshold = 90s
	s := newMonitorTestServer(t, offlineSec)
	m := NewMonitorCollector(s)

	now := time.Now()
	seed := []db.Implant{
		{ID: "fresh", Status: "online", LastSeen: now.Add(-10 * time.Second)},
		{ID: "tostale", Status: "online", LastSeen: now.Add(-45 * time.Second)},
		{ID: "tooffline", Status: "stale", LastSeen: now.Add(-120 * time.Second)},
	}
	if err := s.db.Create(&seed).Error; err != nil {
		t.Fatalf("seed agents: %v", err)
	}

	m.checkAgentAlerts()

	statusOf := func(id string) string {
		var a db.Implant
		if err := s.db.Select("status").Where("id = ?", id).First(&a).Error; err != nil {
			t.Fatalf("read %s: %v", id, err)
		}
		return a.Status
	}
	if got := statusOf("fresh"); got != "online" {
		t.Errorf("fresh agent status = %q, want online", got)
	}
	if got := statusOf("tostale"); got != "stale" {
		t.Errorf("tostale agent status = %q, want stale", got)
	}
	if got := statusOf("tooffline"); got != "offline" {
		t.Errorf("tooffline agent status = %q, want offline", got)
	}

	var events []db.AgentStatusEvent
	if err := s.db.Order("id ASC").Find(&events).Error; err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("recorded %d status events, want 2 (one per transition)", len(events))
	}
	want := map[string]string{"tostale": "stale", "tooffline": "offline"}
	for _, ev := range events {
		if want[ev.AgentID] != ev.Status {
			t.Errorf("event for %s = %q, want %q", ev.AgentID, ev.Status, want[ev.AgentID])
		}
	}
}

// TestCheckAgentAlertsPagedSweepLargeFleet seeds one page over the reap
// page size so the sweep must cross a page boundary, then verifies every
// agent was flipped and every transition recorded — a single capped query
// would have left the tail of the fleet behind.
func TestCheckAgentAlertsPagedSweepLargeFleet(t *testing.T) {
	s := newMonitorTestServer(t, 30)
	m := NewMonitorCollector(s)

	const fleetSize = agentReapPageSize + 2
	now := time.Now()
	seed := make([]db.Implant, 0, fleetSize)
	for i := range fleetSize {
		seed = append(seed, db.Implant{
			ID:       fmt.Sprintf("swp-%05d", i),
			Status:   "online",
			LastSeen: now.Add(-45 * time.Second), // past the 30s offline threshold, before the 90s stale threshold
		})
	}
	// 5002 rows would need one INSERT statement here, which exceeds SQLite's
	// 32766 variable limit (~54 Implant columns); seed in bounded chunks.
	for start := 0; start < len(seed); start += 500 {
		end := min(start+500, len(seed))
		if err := s.db.Create(seed[start:end]).Error; err != nil {
			t.Fatalf("seed fleet: %v", err)
		}
	}

	m.checkAgentAlerts()

	var stale, events int64
	if err := s.db.Model(&db.Implant{}).Where("status = ?", "stale").Count(&stale).Error; err != nil {
		t.Fatalf("count stale: %v", err)
	}
	if stale != fleetSize {
		t.Fatalf("%d/%d agents flipped to stale", stale, fleetSize)
	}
	if err := s.db.Model(&db.AgentStatusEvent{}).Where("status = ?", "stale").Count(&events).Error; err != nil {
		t.Fatalf("count events: %v", err)
	}
	if events != fleetSize {
		t.Fatalf("%d/%d stale status events recorded", events, fleetSize)
	}
}
