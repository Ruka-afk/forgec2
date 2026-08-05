package server

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/testutil"
)

// testServerForChain builds a minimal Server with a known master beacon key so
// per-agent file chain keys can be derived.
func testServerForChain(t *testing.T) *Server {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.Server.BeaconKey = strings.Repeat("ab", 32) // 64 hex chars = 32 bytes
	return &Server{
		db:         testutil.SetupTestDB(t),
		cfg:        cfg,
		fileChains: newFileChainState(),
	}
}

// buildChainKey mimics the agent: deriveFileChainKey(regKey).
func buildChainKey(t *testing.T, s *Server, agentID string) []byte {
	t.Helper()
	key := s.fileChainKey(agentID)
	if key == nil {
		t.Fatalf("fileChainKey(%q) returned nil", agentID)
	}
	return key
}

func linkMAC(t *testing.T, key, prev []byte, data []byte) string {
	t.Helper()
	mac := crypto.FileChunkMAC(key, prev, data)
	if mac == nil {
		t.Fatal("FileChunkMAC returned nil")
	}
	return hex.EncodeToString(mac)
}

func TestFileChainVerifyAndCommit(t *testing.T) {
	s := testServerForChain(t)
	defer s.db.DB()

	chainKey := buildChainKey(t, s, "agent-1")
	chunk1 := []byte("first chunk payload")
	chunk2 := []byte("second chunk payload")

	// First link: chain is seeded with 32 zero bytes.
	mac1 := linkMAC(t, chainKey, make([]byte, 32), chunk1)
	if err := s.verifyAndCommitChain("agent-1", 11, mac1, chunk1); err != nil {
		t.Fatalf("verify chunk1: %v", err)
	}

	// Second link must chain from mac1.
	mac2 := linkMAC(t, chainKey, mustHex(t, mac1), chunk2)
	if err := s.verifyAndCommitChain("agent-1", 11, mac2, chunk2); err != nil {
		t.Fatalf("verify chunk2: %v", err)
	}

	// Tampered chunk (data changed, MAC from different data) must fail.
	if err := s.verifyAndCommitChain("agent-1", 11, mac2, []byte("tampered!!")); err == nil {
		t.Fatal("tampered chunk accepted")
	}

	// A chunk computed with the wrong previous link must fail (reordering).
	wrongPrev := linkMAC(t, chainKey, mustHex(t, mac2), chunk2)
	if err := s.verifyAndCommitChain("agent-1", 12, wrongPrev, chunk2); err == nil {
		t.Fatal("reordered/desynced chunk accepted")
	}

	// Malformed MAC hex must fail.
	if err := s.verifyAndCommitChain("agent-1", 13, "zz-not-hex", chunk1); err == nil {
		t.Fatal("malformed MAC accepted")
	}
}

func TestFileChainMissingMACAllowed(t *testing.T) {
	s := testServerForChain(t)
	defer s.db.DB()

	// Legacy agents / non-chunk tasks send no MAC — verification is a no-op.
	if err := s.verifyAndCommitChain("agent-1", 21, "", []byte("legacy chunk")); err != nil {
		t.Fatalf("empty MAC should pass, got: %v", err)
	}
}

func TestFileChainForPush(t *testing.T) {
	s := testServerForChain(t)
	defer s.db.DB()

	chainKey := buildChainKey(t, s, "agent-2")
	chunk1 := []byte("push chunk one")
	chunk2 := []byte("push chunk two")

	prev1, mac1, err := s.chainForPush("agent-2", 31, chunk1)
	if err != nil {
		t.Fatalf("chainForPush chunk1: %v", err)
	}
	if prev1 != hex.EncodeToString(make([]byte, 32)) {
		t.Errorf("first prev = %q, want zero seed", prev1)
	}
	if mac1 != linkMAC(t, chainKey, make([]byte, 32), chunk1) {
		t.Error("chunk1 MAC mismatch")
	}

	// Second push must chain from mac1.
	prev2, mac2, err := s.chainForPush("agent-2", 31, chunk2)
	if err != nil {
		t.Fatalf("chainForPush chunk2: %v", err)
	}
	if prev2 != mac1 {
		t.Errorf("second prev = %q, want mac1 %q", prev2, mac1)
	}
	if mac2 != linkMAC(t, chainKey, mustHex(t, mac1), chunk2) {
		t.Error("chunk2 MAC mismatch")
	}
}

func TestFileChainTaskIsolation(t *testing.T) {
	s := testServerForChain(t)
	defer s.db.DB()

	// Different tasks must not share chain state (both start from the seed).
	chunk := []byte("same bytes")
	macA := linkMAC(t, buildChainKey(t, s, "agent-3"), make([]byte, 32), chunk)
	macB := linkMAC(t, buildChainKey(t, s, "agent-3"), make([]byte, 32), chunk)
	if err := s.verifyAndCommitChain("agent-3", 41, macA, chunk); err != nil {
		t.Fatalf("task A: %v", err)
	}
	if err := s.verifyAndCommitChain("agent-3", 42, macB, chunk); err != nil {
		t.Fatalf("task B should verify independently: %v", err)
	}
}

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("bad hex %q: %v", s, err)
	}
	return b
}

func TestFileChainKeyStability(t *testing.T) {
	s := testServerForChain(t)
	defer s.db.DB()

	k1 := s.fileChainKey("agent-9")
	k2 := s.fileChainKey("agent-9")
	if k1 == nil || k2 == nil || hex.EncodeToString(k1) != hex.EncodeToString(k2) {
		t.Fatal("file chain key must be deterministic per agent")
	}
	k3 := s.fileChainKey("agent-8")
	if k3 == nil || strings.EqualFold(hex.EncodeToString(k1), hex.EncodeToString(k3)) {
		t.Fatal("different agents must have different chain keys")
	}
}
