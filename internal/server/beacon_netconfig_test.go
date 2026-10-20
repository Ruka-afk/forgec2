package server

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/forgec2/forgec2/pkg/encoding"
	"github.com/forgec2/forgec2/pkg/protocol"
)

// TestNetworkConfigForBeaconLifecycle pins the refresh decision: ship on first
// use and on change, stay quiet while unchanged, refresh on the periodic
// interval, and never emit a blob that cannot fit a datagram frame.
func TestNetworkConfigForBeaconLifecycle(t *testing.T) {
	const masterHex = "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
	database := testutil.SetupTestDB(t)
	s := initV3BeaconServer(t, database, masterHex)

	secretID, secretB64, err := s.createRegSecret(0)
	if err != nil {
		t.Fatalf("createRegSecret: %v", err)
	}
	secret := decodeStdB64(secretB64)
	if len(secret) != 32 {
		t.Fatalf("reg secret must be 32 bytes, got %d", len(secret))
	}
	imp := db.Implant{ID: "netcfg-lifecycle", SecretID: secretID, CurrentInterval: 30, CurrentJitter: 10}
	if err := database.Create(&imp).Error; err != nil {
		t.Fatalf("create implant: %v", err)
	}

	now := time.Now()
	blob := s.networkConfigForBeacon(imp, now, 0)
	if blob == "" {
		t.Fatal("first beacon must deliver the network config")
	}
	nc, err := protocol.DecryptNetworkConfig(secret, blob)
	if err != nil {
		t.Fatalf("decrypt delivered config: %v", err)
	}
	if nc.Interval != 30 || nc.Jitter != 10 {
		t.Fatalf("delivered config mismatch: interval=%d jitter=%d", nc.Interval, nc.Jitter)
	}

	// Unchanged and still inside the refresh interval: nothing is resent.
	if got := s.networkConfigForBeacon(imp, now.Add(time.Minute), 0); got != "" {
		t.Fatal("unchanged config must not be resent on every beacon")
	}

	// A changed sleep value is delivered on the very next beacon (the sleep
	// update bumps the config generation server-side).
	changed := imp
	changed.CurrentInterval = 45
	s.markNetworkConfigChanged()
	blob = s.networkConfigForBeacon(changed, now.Add(2*time.Minute), 0)
	if blob == "" {
		t.Fatal("changed config must be delivered immediately")
	}
	nc, err = protocol.DecryptNetworkConfig(secret, blob)
	if err != nil {
		t.Fatalf("decrypt changed config: %v", err)
	}
	if nc.Interval != 45 {
		t.Fatalf("changed config interval = %d, want 45", nc.Interval)
	}

	// Unchanged after the refresh interval: resent as a self-heal.
	if got := s.networkConfigForBeacon(changed, now.Add(2*time.Minute+networkConfigRefreshInterval), 0); got == "" {
		t.Fatal("periodic refresh must resend the config")
	}

	// A datagram transport with a tiny frame budget refuses an oversized blob,
	// and the refusal must not be recorded (a stream beacon still gets it).
	bigger := changed
	bigger.CurrentInterval = 60
	s.markNetworkConfigChanged()
	if got := s.networkConfigForBeacon(bigger, now.Add(3*time.Minute), 8); got != "" {
		t.Fatal("oversized config must be skipped on a datagram transport")
	}
	if got := s.networkConfigForBeacon(bigger, now.Add(3*time.Minute), 0); got == "" {
		t.Fatal("skipped datagram refresh must not mark the config as delivered")
	}
}

// TestNetworkConfigForBeaconRequiresSecret makes sure v2/master-key agents (no
// per-implant secret id) are never handed a blob they cannot decrypt.
func TestNetworkConfigForBeaconRequiresSecret(t *testing.T) {
	s := &Server{}
	if got := s.networkConfigForBeacon(db.Implant{ID: "a", SecretID: "b"}, time.Now(), 0); got != "" {
		t.Fatalf("expected no config without a reg-secret store, got %q", got)
	}
}

func TestPruneNetworkConfigDeliveries(t *testing.T) {
	s := &Server{}
	now := time.Now()
	s.recordNetworkConfigDelivery("old", "fp", now.Add(-2*time.Hour))
	s.recordNetworkConfigDelivery("new", "fp", now)
	s.pruneNetworkConfigDeliveries(now.Add(-time.Hour))

	s.networkConfigMu.Lock()
	defer s.networkConfigMu.Unlock()
	if _, ok := s.networkConfigDelivered["old"]; ok {
		t.Fatal("stale delivery bookkeeping should be pruned")
	}
	if _, ok := s.networkConfigDelivered["new"]; !ok {
		t.Fatal("recent delivery bookkeeping must be kept")
	}
}

// TestEncryptedBeaconCarriesNetworkConfig is the end-to-end wiring check: a v3
// agent registers, its server-side sleep values change, and the next encrypted
// beacon response carries a config blob the agent's registration secret can
// decrypt. The following beacon (nothing changed) carries no blob.
func TestEncryptedBeaconCarriesNetworkConfig(t *testing.T) {
	s, database := v2TestServer(t)
	const agentUUID = "12121212-3434-4567-8901-232323232323"
	agent := v3TestAgent(t, s, agentUUID)

	// Register and establish the session.
	w := v2Post(t, s, agent.registerFrame())
	if w.Code != http.StatusOK {
		t.Fatalf("registration: expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	var regResp struct {
		Seq     uint64 `json:"seq"`
		RegOK   bool   `json:"reg_ok"`
		ECDHPub string `json:"ecdh_pub"`
		Mac     string `json:"mac"`
	}
	if err := encoding.Unmarshal(w.Body.Bytes(), &regResp); err != nil {
		t.Fatalf("parse registration response: %v", err)
	}
	if !regResp.RegOK {
		t.Fatalf("registration failed: %s", w.Body.String())
	}
	if err := agent.establishFromServerKey(regResp.ECDHPub); err != nil {
		t.Fatalf("establish session: %v", err)
	}

	// The operator changes the agent's sleep values server-side (set_sleep
	// result, UI edit, etc. The agent must learn about it without re-registering.
	if err := database.Model(&db.Implant{}).Where("id = ?", agentUUID).
		Update("current_interval", 47).Error; err != nil {
		t.Fatalf("update sleep: %v", err)
	}

	beacon := func() beaconResponse {
		t.Helper()
		s.beaconDedupMu.Lock()
		s.beaconDedupCache = make(map[string]time.Time)
		s.beaconDedupMu.Unlock()
		inner, _ := json.Marshal(map[string]interface{}{"uuid": agentUUID, "pv": 2})
		w := v2Post(t, s, agent.encryptedFrame(inner))
		if w.Code != http.StatusOK {
			t.Fatalf("beacon: expected 200, got %d; body=%s", w.Code, w.Body.String())
		}
		var encResp struct {
			CipherB64 string `json:"c"`
		}
		if err := encoding.Unmarshal(w.Body.Bytes(), &encResp); err != nil {
			t.Fatalf("parse encrypted response: %v (body=%s)", err, w.Body.String())
		}
		plain, err := agent.decryptWithAAD(encResp.CipherB64, agent.aad(agent.seq))
		if err != nil {
			t.Fatalf("decrypt response: %v", err)
		}
		var resp beaconResponse
		if err := encoding.Unmarshal(plain, &resp); err != nil {
			t.Fatalf("parse beacon response: %v", err)
		}
		return resp
	}

	first := beacon()
	if first.NetworkConfig == "" {
		t.Fatal("encrypted beacon must carry the changed network config")
	}
	nc, err := protocol.DecryptNetworkConfig(agent.regKey, first.NetworkConfig)
	if err != nil {
		t.Fatalf("agent cannot decrypt network config: %v", err)
	}
	if nc.Interval != 47 {
		t.Fatalf("delivered interval = %d, want 47", nc.Interval)
	}

	second := beacon()
	if second.NetworkConfig != "" {
		t.Fatal("unchanged config must not be resent on the next beacon")
	}
}
