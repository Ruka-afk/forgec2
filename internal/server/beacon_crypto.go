package server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/util"
	"github.com/forgec2/forgec2/pkg/encoding"
	"github.com/forgec2/forgec2/pkg/protocol"
	"github.com/google/uuid"
)

// ── Registration key / replay window / envelope auth ──────────────────────

// deriveRegKey returns the per-agent registration key. v3: when the implant
// has a bound per-implant secret, that secret is used directly (the master key
// is never embedded in v3 payloads). v2 legacy: derived from the master beacon
// key. nil when neither is available.
func (s *Server) deriveRegKey(agentID string) []byte {
	var secretID string
	if err := s.db.Model(&db.Implant{}).Where("id = ?", agentID).Pluck("secret_id", &secretID).Error; err == nil && secretID != "" {
		if key := s.regSecretByID(secretID); key != nil {
			return key
		}
	}
	return crypto.DeriveRegistrationKeyFromHex(s.serverBeaconKey(), agentID)
}

// computeAuthMAC computes the authentication MAC for handshake/registration
// request frames and handshake response frames:
//
//	request:  HMAC(regKey, uuid || ecdh_pub || ts || seq)
//	response: HMAC(regKey, uuid || seq || server_pub)
//
// An attacker without the registration key cannot forge either direction.
// Binding seq into the request MAC prevents replaying a captured
// authentication frame with an inflated sequence number.
func computeAuthMAC(regKey []byte, parts ...string) string {
	mac := hmac.New(sha256.New, regKey)
	for _, p := range parts {
		mac.Write([]byte(p))
	}
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// acceptSeq atomically advances last_seq for an implant. Returns false when
// the frame is a replay (seq <= last_seq), the implant does not exist yet, or
// the agent is in a seq-flood lockout. The gap flag reports an unreasonably
// large jump (possible desync or replay flood) so callers can log an alert.
//
// Jumps exceeding the adaptive cap are rejected WITHOUT advancing the window
// and put the agent into a short in-memory lockout. With the sequence number
// now bound into the frame MAC, a passive attacker cannot inflate seq; the
// cap is defense-in-depth against a key-holding actor replay-flooding a
// captured valid frame to burn the replay window.
func (s *Server) acceptSeq(agentID string, seq uint64) (accepted bool) {
	return s.acceptSeqInner(agentID, seq, true)
}

// acceptHandshakeSeq is acceptSeq for authenticated handshake frames, except
// the flood lockout is never consulted nor extended: a locked-out agent's
// only recovery path is the handshake itself, and handshakes are
// regKey-MAC-authenticated upstream. Replay and jump-cap checks still apply.
func (s *Server) acceptHandshakeSeq(agentID string, seq uint64) (accepted bool) {
	return s.acceptSeqInner(agentID, seq, false)
}

func (s *Server) acceptSeqInner(agentID string, seq uint64, useLockout bool) (accepted bool) {
	if useLockout {
		s.seqLockoutMu.Lock()
		unlock, locked := s.seqLockout[agentID]
		s.seqLockoutMu.Unlock()
		if locked && time.Now().Before(unlock) {
			return false
		}
	}

	var row struct {
		LastSeq   uint64
		UpdatedAt time.Time
	}
	if err := s.db.Model(&db.Implant{}).Where("id = ?", agentID).
		Select("last_seq, updated_at").Scan(&row).Error; err != nil {
		return false
	}
	// No row (or unreadable row): behave as before — reject. Callers treat
	// this as "missing row".
	if row.UpdatedAt.IsZero() {
		var lastSeq uint64
		if err := s.db.Model(&db.Implant{}).Where("id = ?", agentID).Pluck("last_seq", &lastSeq).Error; err != nil {
			return false
		}
		row.LastSeq = lastSeq
	}
	lastSeq := row.LastSeq
	if seq <= lastSeq {
		return false
	}
	jump := seq - lastSeq
	if jump > s.seqJumpCap(agentID, row.UpdatedAt) {
		// Cryptographically valid frame but an implausible jump. Do not
		// advance the window and (for non-handshake frames) lock the agent
		// out briefly so a flood cannot keep hammering the database or the
		// handshake path. Audit once per imposition, not per frame.
		if useLockout {
			s.seqLockoutMu.Lock()
			_, already := s.seqLockout[agentID]
			if s.seqLockout == nil {
				s.seqLockout = make(map[string]time.Time)
			}
			if !already || time.Now().After(s.seqLockout[agentID]) {
				s.seqLockout[agentID] = time.Now().Add(seqLockoutDuration)
				s.LogAuditRecord(nil, "seq_lockout", "agent", agentID, "seq jump exceeded adaptive cap", true, nil)
			}
			s.seqLockoutMu.Unlock()
		}
		slog.Warn("Beacon seq jump exceeded cap, rejecting frame", "agent_id", agentID, "seq", seq, "last_seq", lastSeq)
		return false
	}
	res := s.db.Model(&db.Implant{}).
		Where("id = ? AND last_seq = ?", agentID, lastSeq).
		Update("last_seq", seq)
	if res.Error != nil || res.RowsAffected != 1 {
		return false
	}
	return true
}

// seqJumpCap returns the maximum acceptable single-frame sequence advance:
// base maxSeqJump scaled up by the agent's sleep interval and silence time
// (each failed beacon advances the agent counter by ~8), capped absolutely.
func (s *Server) seqJumpCap(agentID string, lastUpdate time.Time) uint64 {
	cap := uint64(maxSeqJump)
	interval := s.agentBeaconInterval(agentID)
	if interval > 0 && !lastUpdate.IsZero() {
		if elapsed := time.Since(lastUpdate).Seconds(); elapsed > 0 {
			if expected := uint64(elapsed/float64(interval))*16 + uint64(maxSeqJump); expected > cap {
				cap = expected
			}
		}
	}
	if cap > maxSeqJumpAbsolute {
		cap = maxSeqJumpAbsolute
	}
	return cap
}

// agentBeaconInterval returns the agent's sleep interval in seconds (implant
// row, else server default, else 60s floor).
func (s *Server) agentBeaconInterval(agentID string) int64 {
	var iv int
	if err := s.db.Model(&db.Implant{}).Where("id = ?", agentID).Pluck("current_interval", &iv).Error; err == nil && iv > 0 {
		return int64(iv)
	}
	s.configMu.RLock()
	def := s.cfg.Implant.DefaultInterval
	s.configMu.RUnlock()
	if def > 0 {
		return int64(def)
	}
	return 60
}

// startSeqLockoutCleanup periodically evicts expired seqLockout entries so the
// map cannot grow without bound across many distinct agent IDs (a key-holding
// actor could otherwise drive unbounded memory growth). Mirrors the
// loginLockoutTracker cleanup pattern (S5).
func (s *Server) startSeqLockoutCleanup(ctx context.Context) {
	// Tracked so graceful shutdown joins it instead of closing the DB
	// underneath a mid-sweep Pluck (same discipline as all server loops).
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("seqLockout cleanup recovered from panic", "err", r)
			}
		}()
		ticker := time.NewTicker(SeqLockoutCleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				now := time.Now()
				s.seqLockoutMu.Lock()
				for id, unlock := range s.seqLockout {
					if now.After(unlock) {
						delete(s.seqLockout, id)
					}
				}
				s.seqLockoutMu.Unlock()
			}
		}
	}()
}

// currentLastSeq returns the implant's persisted last_seq (0 if unknown).
func (s *Server) currentLastSeq(agentID string) uint64 {
	var lastSeq uint64
	if err := s.db.Model(&db.Implant{}).Where("id = ?", agentID).Pluck("last_seq", &lastSeq).Error; err != nil {
		return 0
	}
	return lastSeq
}

// buildResyncResponse returns a MAC-signed plaintext envelope carrying the
// server's current last_seq for an agent whose beacon was rejected as a replay
// (its sequence fell behind the server's). The agent verifies the MAC (keyed
// by its registration key) and fast-forwards its counter. Only emitted when a
// real implant row exists, so an anonymous actor probing UUIDs learns nothing.
//
// Rekey is always set: a rejected encrypted frame means the server cannot read
// this agent right now (lost session after restart/sweep, or seq behind), so
// the agent must re-handshake on the next beacon rather than keep encrypting
// under a key the server no longer holds.
func (s *Server) buildResyncResponse(agentID string, seq uint64) ([]byte, bool) {
	// Require a real implant row: deriveRegKey falls back to a master-derived
	// key for unknown UUIDs, so without this gate an anonymous actor could
	// probe UUIDs and receive a MAC-signed resync (an existence oracle).
	var imp db.Implant
	if err := s.db.First(&imp, "id = ?", agentID).Error; err != nil {
		return nil, false
	}
	lastSeq := s.currentLastSeq(agentID)
	regKey := s.deriveRegKey(agentID)
	if regKey == nil {
		return nil, false
	}
	serverPub := base64.StdEncoding.EncodeToString(s.sessionManager.GetPublicKey())
	resp := beaconResponse{
		ProtocolVersion: protocol.CurrentProtocolVersion,
		Seq:             seq,
		LastSeq:         lastSeq,
		ECDHPub:         serverPub,
		Rekey:           true,
		Mac:             computeAuthMAC(regKey, agentID, strconv.FormatUint(seq, 10), serverPub),
		SessionCipher:   s.sessionManager.Cipher(),
	}
	respBytes, ok := marshalJSONSafe(resp)
	if !ok {
		return nil, false
	}
	return respBytes, true
}

// resyncResponseFor returns MAC-signed resync bytes for a rejected encrypted
// frame, so every transport can answer identically: the agent re-handshakes
// instead of error-looping. Returns false for garbage frames (no CipherB64,
// invalid ID) and unknown agents.
//
// Two rejection classes may earn a resync:
//   - the ciphertext authenticated but the sequence window rejected it
//     (envelope.authed): a genuinely desynced real agent, and
//   - the server holds no session for that UUID any more (restart / sweep):
//     the agent cannot possibly decrypt, so a resync + rekey is the only way
//     it learns to re-handshake.
//
// A frame that failed AEAD while the server still holds a live session is
// unauthenticated garbage. Answering it would turn every transport into a
// "does this UUID exist?" oracle (MAC-signed response for known agents only)
// and a free response amplifier, so it is dropped.
func (s *Server) resyncResponseFor(env beaconEnvelope) ([]byte, bool) {
	if env.CipherB64 == "" || !isValidAgentID(env.UUID) {
		return nil, false
	}
	if !env.authed && s.sessionManager != nil && s.sessionManager.HasSession(env.UUID) {
		return nil, false
	}
	return s.buildResyncResponse(env.UUID, env.Seq)
}

// ecdhSessionIdleCutoff is the floor for session eviction: sessions idle
// longer than this are dropped even for chatty agents.
const ecdhSessionIdleFloor = 30 * time.Minute

// sweepECDHSessions drops sessions idle beyond max(floor, 3x the agent's
// sleep interval). Per-agent intervals come from the implant rows; agents
// without a row or with interval<=0 fall back to the floor.
func (s *Server) sweepECDHSessions() {
	if s.sessionManager == nil || s.db == nil {
		return
	}
	lastUsed := s.sessionManager.SessionsLastUsed()
	if len(lastUsed) == 0 {
		return
	}
	ids := make([]string, 0, len(lastUsed))
	for id := range lastUsed {
		ids = append(ids, id)
	}
	// Chunk the IN query so a huge fleet cannot blow the variable limit.
	const chunkSize = 500
	intervals := make(map[string]int, len(ids))
	for i := 0; i < len(ids); i += chunkSize {
		end := i + chunkSize
		if end > len(ids) {
			end = len(ids)
		}
		var rows []struct {
			ID              string
			CurrentInterval int
		}
		if err := s.db.Model(&db.Implant{}).Where("id IN ?", ids[i:end]).
			Select("id, current_interval").Scan(&rows).Error; err != nil {
			slog.Warn("ECDH session sweep: interval lookup failed, skipping cycle", "err", err)
			return
		}
		for _, r := range rows {
			intervals[r.ID] = r.CurrentInterval
		}
	}
	now := time.Now()
	dropped := 0
	for id, used := range lastUsed {
		cutoff := ecdhSessionIdleFloor
		if iv := intervals[id]; iv > 0 {
			if d := 3 * time.Duration(iv) * time.Second; d > cutoff {
				cutoff = d
			}
		}
		if now.Sub(used) > cutoff {
			s.sessionManager.DropSession(id)
			dropped++
		}
	}
	if dropped > 0 {
		slog.Info("ECDH session sweep dropped idle sessions", "dropped", dropped, "live", len(lastUsed)-dropped)
	}
}

// decodeBeaconEnvelope parses and authenticates a v2 transport envelope.
// effectiveMaxPayload returns MaxDecryptedPayloadSize or the 10MB default.
func (s *Server) effectiveMaxPayload() int {
	s.configMu.RLock()
	maxPayload := s.cfg.Crypto.MaxDecryptedPayloadSize
	s.configMu.RUnlock()
	if maxPayload <= 0 {
		maxPayload = 10 * 1024 * 1024 // default 10MB
	}
	return maxPayload
}

// Returns the envelope, the inner (decrypted) request when applicable, and the
// frame kind. Protocol v1/plaintext frames are rejected outright.
func (s *Server) decodeBeaconEnvelope(raw []byte) (envelope beaconEnvelope, req beaconRequest, kind beaconFrameKind) {
	// Reject oversized bodies before JSON/base64/AES-GCM work. The plaintext
	// payload is capped at maxPayload, so the base64 ciphertext body cannot
	// legitimately exceed ~4/3 of that plus envelope overhead. Checking raw
	// length up front prevents a BeaconMaxBodySize (64MB) body from being
	// fully base64-decoded and run through AES-GCM before the post-decrypt
	// size guard fires — an anonymous actor can no longer drive GB/min of
	// decrypt work per IP.
	maxPayload := s.effectiveMaxPayload()
	rawLimit := (maxPayload+2)/3*4 + 64*1024
	if len(raw) > rawLimit {
		slog.Warn("Beacon body exceeds raw limit", "size", len(raw), "max", rawLimit)
		return beaconEnvelope{}, beaconRequest{}, frameRejected
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		slog.Warn("Beacon: invalid envelope JSON")
		return beaconEnvelope{}, beaconRequest{}, frameRejected
	}
	if envelope.UUID == "" {
		envelope.UUID = util.NewString()
	}
	if !isValidAgentID(envelope.UUID) {
		slog.Warn("Beacon rejected: invalid agent ID", "agent_id", envelope.UUID)
		return beaconEnvelope{}, beaconRequest{}, frameRejected
	}
	// Canonicalize before session/DB/AAD use: DNS delivers bare hex UUIDs.
	envelope.UUID = normalizeAgentID(envelope.UUID)

	// Timestamp window: reject stale or clock-skewed frames.
	now := time.Now().Unix()
	if envelope.Ts == 0 || now-envelope.Ts > beaconTsTolerance || envelope.Ts-now > beaconTsTolerance {
		slog.Warn("Beacon rejected: timestamp outside tolerance", "agent_id", envelope.UUID, "ts", envelope.Ts)
		return beaconEnvelope{}, beaconRequest{}, frameRejected
	}

	if envelope.CipherB64 != "" {
		if s.sessionManager == nil {
			slog.Warn("Beacon rejected: session manager unavailable", "agent_id", envelope.UUID)
			return beaconEnvelope{}, beaconRequest{}, frameRejected
		}
		plaintext, err := s.sessionManager.DecryptWithAADB64(envelope.UUID, envelope.CipherB64, []byte(envelope.UUID+"\x00"+strconv.FormatUint(envelope.Seq, 10)))
		if err != nil {
			slog.Warn("ECDH decryption failed", "agent_id", envelope.UUID, "err", err)
			// Return the PARSED envelope, not the zero value: the caller's
			// resync gate keys on env.CipherB64 != "" to distinguish "could
			// not decrypt at all" (unknown session: server restarted or the
			// entry was swept) from garbage. With the envelope, a known agent
			// whose session the server no longer holds gets the MAC-signed
			// resync + rekey signal and re-handshakes on the next beacon
			// instead of 400-looping forever. While a live session exists for
			// that UUID, resyncResponseFor refuses to answer.
			return envelope, beaconRequest{}, frameRejected
		}

		// Advance the replay window only after the frame passed AEAD
		// authentication. Advancing it earlier let any anonymous actor who
		// knows a UUID burn the window with garbage frames and permanently
		// lock out the real agent.
		accepted := s.acceptSeq(envelope.UUID, envelope.Seq)
		if !accepted {
			slog.Warn("Beacon rejected: replay or out-of-order seq", "agent_id", envelope.UUID, "seq", envelope.Seq)
			// Return the PARSED envelope, not the zero value: the caller's
			// resync gate keys on env.CipherB64 != "" to distinguish "could
			// not decrypt at all" from "decrypt ok but sequence behind" —
			// only the latter gets the MAC-signed resync response that lets
			// a desynced agent fast-forward instead of being locked out.
			// authed marks this frame as AEAD-verified so the resync path can
			// trust it.
			envelope.authed = true
			return envelope, beaconRequest{}, frameRejected
		}

		maxPayload := s.effectiveMaxPayload()
		if len(plaintext) > maxPayload {
			slog.Warn("Beacon decrypted payload too large", "agent_id", envelope.UUID, "size", len(plaintext), "max", maxPayload)
			return beaconEnvelope{}, beaconRequest{}, frameRejected
		}
		if err := encoding.Unmarshal(plaintext, &req); err != nil {
			slog.Warn("Beacon decrypted payload parse failed", "agent_id", envelope.UUID, "err", err)
			return beaconEnvelope{}, beaconRequest{}, frameRejected
		}
		req.UUID = envelope.UUID
		req.Seq = envelope.Seq
		// NOTE: no explicit IncrementMessageCount here — DecryptWithAADB64
		// already advances the counter internally; the extra call made
		// MessageCount grow ~2× and fired rekey twice as early as intended.
		return envelope, req, frameEncrypted
	}

	if envelope.ECDHPub != "" {
		if envelope.IdentityPub != "" && envelope.RegHMAC != "" {
			return envelope, beaconRequest{}, frameRegister
		}
		if envelope.Mac != "" {
			return envelope, beaconRequest{}, frameHandshake
		}
		slog.Warn("Beacon rejected: unauthenticated handshake frame", "agent_id", envelope.UUID)
		return beaconEnvelope{}, beaconRequest{}, frameRejected
	}

	// v2 has no plaintext frames.
	slog.Warn("Beacon rejected: plaintext frame not allowed", "agent_id", envelope.UUID)
	return beaconEnvelope{}, beaconRequest{}, frameRejected
}

func isValidAgentID(id string) bool {
	if id == "" {
		return false
	}
	check := strings.TrimPrefix(id, "ws_")
	if check == id {
		check = strings.TrimPrefix(id, "unknown-")
	}
	if check == "" {
		return false
	}
	u, err := uuid.Parse(check)
	if err != nil {
		return false
	}
	// Accept the canonical dashed form and the 32-char hex form (DNS qname
	// labels[0] uses bare hex with no dashes).
	if strings.EqualFold(u.String(), check) {
		return true
	}
	return len(check) == 32 && strings.EqualFold(strings.ReplaceAll(u.String(), "-", ""), check)
}

// normalizeAgentID returns the canonical lowercase dashed form of an agent ID
// that has already passed isValidAgentID. DNS transports deliver bare 32-hex
// IDs; session lookup, DB queries and AAD all expect the dashed form.
func normalizeAgentID(id string) string {
	check := strings.TrimPrefix(id, "ws_")
	if check == id {
		check = strings.TrimPrefix(id, "unknown-")
	}
	if u, err := uuid.Parse(check); err == nil {
		return u.String()
	}
	return id
}
