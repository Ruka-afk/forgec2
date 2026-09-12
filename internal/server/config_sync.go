package server

import (
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

// Hot-reload sync table.
//
// diffConfig classifies changed fields as hot/static, but classification
// alone never applied anything: the old onReload only synced crypto keys
// and CSRF, so "hot" fields like rate limits, JWT, SIEM or log level were
// silently ignored. Every hot token below has either a live Apply hook or
// is documented live-read (Apply == nil means request paths already read
// it per-request under configMu — no hook needed, but the token stays
// listed so the status matrix stays truthful).

type reloadMode string

const (
	reloadHot    reloadMode = "hot"
	reloadStatic reloadMode = "static"
)

type reloadGroup struct {
	Token string
	Mode  reloadMode
	// Apply syncs already-constructed runtime state. Nil = live-read.
	Apply func(s *Server) error
}

// reloadGroups is the single source of truth shared by onReload dispatch
// and GET /config/reload-status. Tokens must match diffConfig outputs.
func reloadGroups() []reloadGroup {
	live := func(string) func(*Server) error { return nil }
	_ = live
	return []reloadGroup{
		{Token: "server.port", Mode: reloadStatic},
		{Token: "server.beacon_key", Mode: reloadHot, Apply: (*Server).syncRegSecrets},
		{Token: "crypto.key", Mode: reloadStatic},
		{Token: "server.jwt_secret", Mode: reloadHot, Apply: (*Server).syncJWTSecret},
		{Token: "crypto.loot_key", Mode: reloadHot, Apply: (*Server).syncCryptoKeys},
		{Token: "crypto.extc2_key", Mode: reloadHot, Apply: (*Server).syncCryptoKeys},
		{Token: "crypto.csrf_key", Mode: reloadHot, Apply: (*Server).syncCSRFSecret},
		{Token: "crypto.totp_key", Mode: reloadHot},
		{Token: "crypto.backup_key", Mode: reloadStatic},
		{Token: "crypto.force_ecdh", Mode: reloadStatic},
		{Token: "crypto.max_decrypted_payload_size", Mode: reloadHot},
		{Token: "database.driver", Mode: reloadStatic},
		{Token: "database.path", Mode: reloadStatic},
		{Token: "database.dsn", Mode: reloadStatic},
		{Token: "database.pool", Mode: reloadHot, Apply: (*Server).syncDBPool},
		{Token: "logging.level", Mode: reloadHot, Apply: (*Server).syncLogLevel},
		{Token: "server.tls", Mode: reloadStatic},
		{Token: "server.mtls", Mode: reloadStatic},
		{Token: "server.data_dir", Mode: reloadStatic},
		{Token: "server.listeners", Mode: reloadStatic},
		{Token: "auth", Mode: reloadStatic},
		{Token: "server.trusted_proxies", Mode: reloadHot, Apply: (*Server).syncTrustedProxies},
		{Token: "server.allowed_origins", Mode: reloadHot},
		{Token: "server.cookie_domain", Mode: reloadHot, Apply: (*Server).syncJWTSecret},
		{Token: "server.require_tls_for_auth", Mode: reloadHot, Apply: (*Server).syncJWTSecret},
		{Token: "server.enable_pprof", Mode: reloadHot},
		{Token: "server.enable_metrics", Mode: reloadHot},
		{Token: "server.socks_listen_host", Mode: reloadHot},
		{Token: "server.dns_obscure", Mode: reloadHot},
		{Token: "server.auto_recon", Mode: reloadHot},
		{Token: "server.lportfwd_enabled", Mode: reloadHot},
		{Token: "server.update_check", Mode: reloadHot},
		{Token: "server.vantage_points", Mode: reloadHot},
		{Token: "siem", Mode: reloadHot, Apply: (*Server).syncSIEM},
		{Token: "password_policy", Mode: reloadHot, Apply: (*Server).syncPasswordPolicy},
		{Token: "rate_limit.login", Mode: reloadHot},
		{Token: "rate_limit.beacon", Mode: reloadHot, Apply: (*Server).syncRateLimiters},
		{Token: "rate_limit.api", Mode: reloadHot, Apply: (*Server).syncRateLimiters},
		{Token: "rate_limit.extc2", Mode: reloadHot},
		{Token: "server.dns", Mode: reloadStatic},
		{Token: "server.grpc", Mode: reloadStatic},
		{Token: "malleable", Mode: reloadHot},
		{Token: "server.geoip_enabled", Mode: reloadHot},
		{Token: "server.cleanup_retention_days", Mode: reloadHot},
		{Token: "server.host", Mode: reloadStatic},
		{Token: "server.offline_threshold", Mode: reloadStatic},
		{Token: "server.session_max_age_hours", Mode: reloadStatic},
		{Token: "server.tcp", Mode: reloadStatic},
		{Token: "server.smb", Mode: reloadStatic},
		{Token: "server.icmp", Mode: reloadStatic},
		{Token: "server.udp", Mode: reloadStatic},
		{Token: "server.quic", Mode: reloadStatic},
		{Token: "server.ssh", Mode: reloadStatic},
		{Token: "implant", Mode: reloadHot},
		{Token: "security", Mode: reloadHot},
		{Token: "monitoring", Mode: reloadHot},
		{Token: "roe", Mode: reloadHot},
		{Token: "ai", Mode: reloadHot},
		{Token: "socks", Mode: reloadHot},
		{Token: "integrations", Mode: reloadHot},
		{Token: "server.tls_fingerprint", Mode: reloadHot, Apply: (*Server).syncTLSFingerprint},
	}
}

// reloadOutcome records the last reload for /config/reload-status.
type reloadOutcome struct {
	At       time.Time         `json:"at"`
	Changed  []string          `json:"changed"`
	Applied  []string          `json:"applied"`
	Failed   map[string]string `json:"failed,omitempty"`
	Rejected []string          `json:"rejected_static,omitempty"`
}

// --- Apply hooks (all idempotent) ---

func (s *Server) syncCryptoKeys() error {
	crypto.InitLootEncryption(s.cfg.Crypto.LootKey)
	crypto.InitExtC2Encryption(s.cfg.Crypto.ExtC2Key)
	return nil
}

func (s *Server) syncCSRFSecret() error {
	return middleware.InitCSRFSecret(s.cfg)
}

// syncJWTSecret reinstalls the signing key (with rotation grace) and the
// cookie/TLS flags derived from it. Callers: server.jwt_secret,
// server.cookie_domain, server.require_tls_for_auth.
func (s *Server) syncJWTSecret() error {
	return middleware.InitJWTSecret(s.cfg, "")
}

// syncRegSecrets rebuilds the v3 registration-secret store from a rotated
// master beacon key. WARNING: secrets sealed under the old master become
// unreadable, so already-deployed implants fail auth until regenerated —
// rotation is intentionally disruptive and always audited.
func (s *Server) syncRegSecrets() error {
	master, err := hex.DecodeString(s.cfg.Server.BeaconKey)
	if err != nil || len(master) == 0 {
		return fmt.Errorf("invalid server.beacon_key, keeping previous store")
	}
	s.regSecrets = crypto.NewRegSecretStore(master)
	slog.Warn("v3 registration secret store rebuilt from rotated beacon key; previously deployed implants must be regenerated")
	return nil
}

func (s *Server) syncRateLimiters() error {
	if s.rateLimiter != nil {
		s.rateLimiter.SetLimits(s.cfg.RateLimit.Beacon.Limit, time.Duration(s.cfg.RateLimit.Beacon.Window)*time.Second)
	}
	if s.apiRateLimiter != nil {
		s.apiRateLimiter.SetCapacityRate(s.cfg.RateLimit.API.Capacity, s.cfg.RateLimit.API.Rate)
		s.apiRateLimiter.SetWhitelist(s.cfg.RateLimit.API.Whitelist)
	}
	return nil
}

func (s *Server) syncSIEM() error {
	if !s.cfg.SIEM.Enabled || s.cfg.SIEM.URL == "" {
		if s.siem != nil {
			s.siem.UpdateConfig(false, "", "", "")
		}
		return nil
	}
	if s.siem == nil {
		s.siem = NewSIEMWebhook(s, s.cfg.SIEM.URL, s.cfg.SIEM.Token, s.cfg.SIEM.Actions)
		if s.siem != nil {
			s.siem.ReloadRules()
		}
		return nil
	}
	s.siem.UpdateConfig(true, s.cfg.SIEM.URL, s.cfg.SIEM.Token, s.cfg.SIEM.Actions)
	return nil
}

func (s *Server) syncLogLevel() error {
	if !SetLogLevel(s.cfg.Logging.Level) {
		return fmt.Errorf("unknown logging.level %q, keeping previous", s.cfg.Logging.Level)
	}
	return nil
}

func (s *Server) syncTrustedProxies() error {
	if s.router == nil {
		return nil
	}
	if len(s.cfg.Server.TrustedProxies) > 0 {
		if err := s.router.SetTrustedProxies(s.cfg.Server.TrustedProxies); err != nil {
			return fmt.Errorf("invalid trusted_proxies: %w", err)
		}
		middleware.SetTrustedProxyIPs(s.cfg.Server.TrustedProxies)
	} else {
		s.router.SetTrustedProxies(nil)
		middleware.SetTrustedProxyIPs(nil)
	}
	return nil
}

func (s *Server) syncPasswordPolicy() error {
	if s.cfg.PasswordPolicy.BcryptCost > 0 {
		middleware.SetBcryptCost(s.cfg.PasswordPolicy.BcryptCost)
	}
	return nil
}

func (s *Server) syncDBPool() error {
	if s.db == nil {
		return nil
	}
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	if s.cfg.Server.DBMaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(s.cfg.Server.DBMaxOpenConns)
	}
	if s.cfg.Server.DBMaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(s.cfg.Server.DBMaxIdleConns)
	}
	if s.cfg.Server.DBConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(s.cfg.Server.DBConnMaxLifetime)
	}
	return nil
}

func (s *Server) syncTLSFingerprint() error {
	s.initTLSFingerprint()
	return nil
}

var (
	lastReloadMu sync.Mutex
	lastReload   *reloadOutcome
)

func recordReloadOutcome(o *reloadOutcome) {
	lastReloadMu.Lock()
	defer lastReloadMu.Unlock()
	lastReload = o
}

// handleReloadStatus reports the hot/static matrix plus the last reload.
// Agents and auditors use it to tell "applied live" from "needs restart".
// Mechanism distinguishes hook-synced state ("hook") from per-request
// live reads ("live-read"), both of which take effect without restart.
func (s *Server) handleReloadStatus(c *gin.Context) {
	groups := reloadGroups()
	out := make([]gin.H, 0, len(groups))
	for _, g := range groups {
		mech := "restart"
		if g.Mode == reloadHot {
			mech = "live-read"
			if g.Apply != nil {
				mech = "hook"
			}
		}
		out = append(out, gin.H{"token": g.Token, "mode": string(g.Mode), "mechanism": mech})
	}
	lastReloadMu.Lock()
	last := lastReload
	lastReloadMu.Unlock()
	c.JSON(http.StatusOK, gin.H{"success": true, "groups": out, "last_reload": last})
}
