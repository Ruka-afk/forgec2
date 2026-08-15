package server

import (
	"testing"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/forgec2/forgec2/pkg/protocol"
)

// TestBuildNetworkConfigDeliveredDecryptable verifies the config-over-wire path
// end-to-end: the server encrypts a v3 implant's network config under the
// per-implant registration secret, and the value can be decrypted by the agent
// using the secret it was compiled with (RegSecretStr).
func TestBuildNetworkConfigDeliveredDecryptable(t *testing.T) {
	regKey := make([]byte, 32)
	for i := range regKey {
		regKey[i] = byte(0xA0 + i)
	}
	s := &Server{
		db:         testutil.SetupTestDB(t),
		regSecrets: crypto.NewRegSecretStore(make([]byte, 32)),
		cfg:        &config.Config{},
	}

	// Listener the implant is associated with.
	if err := s.db.Create(&db.Listener{
		ID:     7,
		Name:   "l7",
		Scheme: "https",
		Host:   "c2.example.com",
		Port:   8443,
		Enabled: true,
	}).Error; err != nil {
		t.Fatalf("create listener: %v", err)
	}
	imp := db.Implant{
		ID:              "test-agent",
		ListenerID:      7,
		CurrentInterval: 45,
		CurrentJitter:   10,
	}

	b64, err := s.buildNetworkConfig(imp, regKey)
	if err != nil {
		t.Fatalf("buildNetworkConfig: %v", err)
	}
	if b64 == "" {
		t.Fatal("expected a non-empty encrypted network config")
	}

	// The agent decrypts using the per-implant secret it was compiled with
	// (RegSecretStr base64-decodes to regKey). Decrypting with the same secret
	// proves the over-the-wire blob is consumable by the implant.
	nc, err := protocol.DecryptNetworkConfig(regKey, b64)
	if err != nil {
		t.Fatalf("agent-side decrypt failed: %v", err)
	}
	if nc.C2URL != "https://c2.example.com:8443" {
		t.Errorf("C2URL = %q, want https://c2.example.com:8443", nc.C2URL)
	}
	if nc.Interval != 45 || nc.Jitter != 10 {
		t.Errorf("sleep = %d/%d, want 45/10", nc.Interval, nc.Jitter)
	}
}

// TestBuildNetworkConfigSkipsWhenNoKey ensures a v2 (no per-implant secret)
// implant gets no over-the-wire config.
func TestBuildNetworkConfigSkipsWhenNoKey(t *testing.T) {
	s := &Server{db: testutil.SetupTestDB(t), cfg: &config.Config{}}
	if b64, err := s.buildNetworkConfig(db.Implant{}, nil); err == nil && b64 != "" {
		t.Fatalf("expected empty config for missing key, got %q", b64)
	}
}
