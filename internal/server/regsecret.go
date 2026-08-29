package server

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/forgec2/forgec2/internal/db"
)

// v3 per-implant registration secrets: generated at build time, sealed at rest
// (AES-256-GCM under a server-only HKDF key), and embedded into exactly one
// binary. The fleet master beacon key is no longer compiled into new payloads,
// so extracting a single implant cannot derive the registration keys of any
// other implant.

// createRegSecret persists a fresh per-implant registration secret and returns
// its public id and the base64 secret to embed in the implant's config blob.
func (s *Server) createRegSecret() (id string, secretB64 string, err error) {
	if s.regSecrets == nil {
		return "", "", errors.New("v3 reg secret store not initialized")
	}
	secret, err := s.regSecrets.GenerateSecret()
	if err != nil {
		return "", "", err
	}
	sealed, err := s.regSecrets.Seal(secret)
	if err != nil {
		return "", "", err
	}
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return "", "", err
	}
	id = hex.EncodeToString(idBytes)
	if err := s.db.Create(&db.RegSecret{ID: id, SecretEnc: sealed}).Error; err != nil {
		return "", "", err
	}
	return id, base64.StdEncoding.EncodeToString(secret), nil
}

// regSecretByID returns the unsealed registration secret for a public secret
// id, or nil when the id is unknown or the store is unavailable.
func (s *Server) regSecretByID(id string) []byte {
	if id == "" || s.regSecrets == nil {
		return nil
	}
	var row db.RegSecret
	if err := s.db.Where("id = ?", id).First(&row).Error; err != nil {
		return nil
	}
	secret, err := s.regSecrets.Unseal(row.SecretEnc)
	if err != nil {
		return nil
	}
	return secret
}

// regSecretForAuth resolves the per-implant secret for an authentication frame
// and enforces that the secret is bound to the presenting agent. A secret that
// has already been bound to a different agent_id is rejected, so a secret
// extracted from one implant cannot be replayed to impersonate another. An
// unbound secret (agent_id == "") is allowed and is bound to agentID on first
// registration (see bindRegSecret).
func (s *Server) regSecretForAuth(id, agentID string) ([]byte, bool) {
	if id == "" || s.regSecrets == nil {
		return nil, false
	}
	var row db.RegSecret
	if err := s.db.Where("id = ?", id).First(&row).Error; err != nil {
		return nil, false
	}
	if row.AgentID != "" && row.AgentID != agentID {
		return nil, false
	}
	secret, err := s.regSecrets.Unseal(row.SecretEnc)
	if err != nil {
		return nil, false
	}
	return secret, true
}

// ensureV3RegSecret creates a fresh per-implant v3 registration secret and
// returns its public id and base64 secret. On success beaconKey is cleared,
// because v3 payloads must never carry the fleet master key.
//
// A creation failure is fatal: with v2 master-key auth deprecated and rejected
// server-side, silently falling back to the master key would build an implant
// that can never register. Callers must abort the build when err is non-nil.
// When beaconKey is empty there is no auth material at all — processAuthFrame
// rejects any frame without a SecretID — so generation must FAIL instead of
// handing the operator a payload that can never register (P2-4).
func (s *Server) ensureV3RegSecret(beaconKey string) (id, secretB64, clearedKey string, err error) {
	if s == nil || beaconKey == "" {
		return "", "", "", fmt.Errorf("server.beacon_key is not configured: generated payloads could never register (v2 master-key auth is deprecated)")
	}
	id, secretB64, err = s.createRegSecret()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create v3 registration secret: %w", err)
	}
	return id, secretB64, "", nil
}

// bindRegSecret records which implant a v3 secret registered with, so a stolen
// secret can only ever impersonate that one agent.
func (s *Server) bindRegSecret(id, agentID string) {
	if id == "" {
		return
	}
	s.db.Model(&db.RegSecret{}).Where("id = ?", id).Updates(map[string]interface{}{
		"agent_id": agentID,
		"bound":    true,
	})
}

// cleanupOrphanedRegSecrets deletes v3 registration secrets that were never
// bound to an agent and are older than ttl. A secret is created before the
// toolchain runs, so a failed build leaves an unbound, unusable row behind;
// without this sweep they accumulate indefinitely. A successfully deployed
// agent binds its secret on first check-in well within the TTL, so only true
// orphans are removed.
func (s *Server) cleanupOrphanedRegSecrets(ttl time.Duration) {
	if s.regSecrets == nil {
		return
	}
	cutoff := time.Now().Add(-ttl)
	res := s.db.Where("bound = ? AND created_at < ?", false, cutoff).Delete(&db.RegSecret{})
	if res.Error != nil {
		slog.Warn("cleanup orphaned reg secrets failed", "err", res.Error)
	}
}
