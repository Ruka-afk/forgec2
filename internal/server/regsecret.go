package server

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"

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
