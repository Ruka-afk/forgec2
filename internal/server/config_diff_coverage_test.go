package server

import (
	"testing"

	"github.com/forgec2/forgec2/internal/config"
)

// TestDiffConfigTokenCoverage locks the hot-reload contract: every classified
// token must fire when its field changes (hot or static — never silent),
// and every token in reloadGroups must be producible by diffConfig.
func TestDiffConfigTokenCoverage(t *testing.T) {
	base := func(t *testing.T) *config.Config {
		t.Helper()
		cfg := config.DefaultConfig()
		cfg.Server.Port = 8000
		cfg.Server.JWTSecret = "test-secret-for-reloader-32chars!!"
		setServerTestKeys(cfg)
		return cfg
	}
	cases := []struct {
		token  string
		mode   string // "hot" or "static"
		mutate func(*config.Config)
	}{
		{"server.port", "static", func(c *config.Config) { c.Server.Port++ }},
		{"server.beacon_key", "hot", func(c *config.Config) { c.Server.BeaconKey += "x" }},
		{"crypto.key", "static", func(c *config.Config) { c.Crypto.Key += "x" }},
		{"server.jwt_secret", "hot", func(c *config.Config) { c.Server.JWTSecret += "x" }},
		{"crypto.loot_key", "hot", func(c *config.Config) { c.Crypto.LootKey += "x" }},
		{"crypto.extc2_key", "hot", func(c *config.Config) { c.Crypto.ExtC2Key += "x" }},
		{"crypto.csrf_key", "hot", func(c *config.Config) { c.Crypto.CsrfKey += "x" }},
		{"crypto.totp_key", "hot", func(c *config.Config) { c.Crypto.TotpKey += "x" }},
		{"crypto.backup_key", "static", func(c *config.Config) { c.Crypto.BackupKey += "x" }},
		{"crypto.force_ecdh", "static", func(c *config.Config) { c.Crypto.ForceECDH = !c.Crypto.ForceECDH }},
		{"crypto.max_decrypted_payload_size", "hot", func(c *config.Config) { c.Crypto.MaxDecryptedPayloadSize++ }},
		{"database.driver", "static", func(c *config.Config) { c.Database.Driver += "x" }},
		{"database.path", "static", func(c *config.Config) { c.Database.Path += "x" }},
		{"database.dsn", "static", func(c *config.Config) { c.Database.DSN += "x" }},
		{"database.pool", "hot", func(c *config.Config) { c.Server.DBMaxOpenConns++ }},
		{"logging.level", "hot", func(c *config.Config) { c.Logging.Level += "x" }},
		{"server.tls", "static", func(c *config.Config) { c.Server.TLSEnabled = !c.Server.TLSEnabled }},
		{"server.mtls", "static", func(c *config.Config) { c.Server.RequireClientCert = !c.Server.RequireClientCert }},
		{"server.data_dir", "static", func(c *config.Config) { c.Server.DataDir += "x" }},
		{"server.listeners", "static", func(c *config.Config) { c.Listeners.H2C.Enabled = !c.Listeners.H2C.Enabled }},
		{"auth", "static", func(c *config.Config) { c.Auth.DefaultPasswd += "x" }},
		{"siem", "hot", func(c *config.Config) { c.SIEM.Token += "x" }},
		{"password_policy", "hot", func(c *config.Config) { c.PasswordPolicy.RequireUpper = !c.PasswordPolicy.RequireUpper }},
		{"rate_limit.login", "hot", func(c *config.Config) { c.RateLimit.Login.LockoutTime++ }},
		{"rate_limit.api", "hot", func(c *config.Config) { c.RateLimit.API.Rate++ }},
		{"rate_limit.beacon", "hot", func(c *config.Config) { c.RateLimit.Beacon.Limit++ }},
		{"rate_limit.extc2", "hot", func(c *config.Config) { c.RateLimit.ExtC2.Burst++ }},
		{"server.dns", "static", func(c *config.Config) { c.Server.DNSAddr += "x" }},
		{"server.grpc", "static", func(c *config.Config) { c.Server.GRPCAddr += "x" }},
		{"server.host", "static", func(c *config.Config) { c.Server.Host += "x" }},
		{"server.offline_threshold", "static", func(c *config.Config) { c.Server.OfflineThreshold++ }},
		{"server.session_max_age_hours", "static", func(c *config.Config) { c.Server.SessionMaxAgeHours++ }},
		{"server.tcp", "static", func(c *config.Config) { c.Server.TCPAddr += "x" }},
		{"server.smb", "static", func(c *config.Config) { c.Server.SMBPipe += "x" }},
		{"server.icmp", "static", func(c *config.Config) { c.Server.ICMPEnabled = !c.Server.ICMPEnabled }},
		{"server.udp", "static", func(c *config.Config) { c.Server.UDPAddr += "x" }},
		{"server.quic", "static", func(c *config.Config) { c.Server.QUICAddr += "x" }},
		{"server.ssh", "static", func(c *config.Config) { c.Server.SSHUser += "x" }},
		{"malleable", "hot", func(c *config.Config) { c.Malleable.StatusCode++ }},
		{"server.geoip_enabled", "hot", func(c *config.Config) { c.Server.GeoIPEnabled = !c.Server.GeoIPEnabled }},
		{"server.cleanup_retention_days", "hot", func(c *config.Config) { c.Server.CleanupRetentionDays++ }},
		{"server.allowed_origins", "hot", func(c *config.Config) { c.Server.AllowedOrigins = append(c.Server.AllowedOrigins, "x") }},
		{"server.trusted_proxies", "hot", func(c *config.Config) { c.Server.TrustedProxies = append(c.Server.TrustedProxies, "x") }},
		{"server.cookie_domain", "hot", func(c *config.Config) { c.Server.CookieDomain += "x" }},
		{"server.require_tls_for_auth", "hot", func(c *config.Config) { c.Server.RequireTLSForAuth = !c.Server.RequireTLSForAuth }},
		{"server.enable_pprof", "hot", func(c *config.Config) { c.Server.EnablePprof = !c.Server.EnablePprof }},
		{"server.enable_metrics", "hot", func(c *config.Config) { c.Server.EnableMetrics = !c.Server.EnableMetrics }},
		{"server.socks_listen_host", "hot", func(c *config.Config) { c.Server.SocksListenHost += "x" }},
		{"server.dns_obscure", "hot", func(c *config.Config) { c.Server.DNSObscure = !c.Server.DNSObscure }},
		{"server.auto_recon", "hot", func(c *config.Config) { c.Server.AutoRecon = append(c.Server.AutoRecon, "x") }},
		{"server.lportfwd_enabled", "hot", func(c *config.Config) { c.Server.LPortFwdEnabled = !c.Server.LPortFwdEnabled }},
		{"server.update_check", "hot", func(c *config.Config) { c.Server.UpdateCheckRepo += "x" }},
		{"server.vantage_points", "hot", func(c *config.Config) { c.Server.VantagePoints = append(c.Server.VantagePoints, "x") }},
		{"server.tls_fingerprint", "hot", func(c *config.Config) { c.TLSFingerprint.JARMEnabled = !c.TLSFingerprint.JARMEnabled }},
		{"implant", "hot", func(c *config.Config) { c.Implant.DefaultInterval++ }},
		{"security", "hot", func(c *config.Config) { c.Security.RequireApproval = !c.Security.RequireApproval }},
		{"monitoring", "hot", func(c *config.Config) { c.Monitoring.CooldownSeconds++ }},
		{"roe", "hot", func(c *config.Config) { c.Roe.Enabled = !c.Roe.Enabled }},
		{"ai", "hot", func(c *config.Config) { c.AI.MaxToolRounds++ }},
		{"socks", "hot", func(c *config.Config) { c.Socks.Enabled = !c.Socks.Enabled }},
		{"integrations", "hot", func(c *config.Config) { c.Integrations.Slack.BotToken += "x" }},
	}
	for _, tc := range cases {
		t.Run(tc.token, func(t *testing.T) {
			old := base(t)
			mod, err := copyConfig(old)
			if err != nil {
				t.Fatalf("copy: %v", err)
			}
			tc.mutate(mod)
			hot, static := diffConfig(old, mod)
			found := false
			for _, h := range hot {
				if h == tc.token {
					found = true
					if tc.mode != "hot" {
						t.Fatalf("token %q reported hot, want %s", tc.token, tc.mode)
					}
				}
			}
			for _, s := range static {
				if s == tc.token {
					found = true
					if tc.mode != "static" {
						t.Fatalf("token %q reported static, want %s", tc.token, tc.mode)
					}
				}
			}
			if !found {
				t.Fatalf("token %q not reported (hot=%v static=%v): silent no-op", tc.token, hot, static)
			}
		})
	}
}

// TestReloadGroupsParity asserts every diffConfig token has a reloadGroups
// entry and vice versa, so the status matrix can never drift from the
// classifier.
func TestReloadGroupsParity(t *testing.T) {
	inTable := map[string]bool{}
	for _, g := range reloadGroups() {
		if inTable[g.Token] {
			t.Fatalf("duplicate token %q in reloadGroups", g.Token)
		}
		inTable[g.Token] = true
	}
	old := config.DefaultConfig()
	setServerTestKeys(old)
	mod, err := copyConfig(old)
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	// Flip one leaf per top-level area so every token fires at once, then
	// require exact set equality between classifier output and table.
	mod.Server.Port++
	mod.Server.BeaconKey += "x"
	mod.Logging.Level += "x"
	hot, static := diffConfig(old, mod)
	for _, tok := range append(append([]string{}, hot...), static...) {
		if !inTable[tok] {
			t.Fatalf("diffConfig token %q missing from reloadGroups", tok)
		}
	}
}

// TestSameConfigFile verifies the watcher matches the config across
// relative spellings (directory watches report joined paths).
func TestSameConfigFile(t *testing.T) {
	if !sameConfigFile("/x/config.yaml", "/x/config.yaml") {
		t.Fatal("identical paths must match")
	}
	if sameConfigFile("/x/other.yaml", "/x/config.yaml") {
		t.Fatal("different files must not match")
	}
	if sameConfigFile("", "/x/config.yaml") {
		t.Fatal("empty event must not match")
	}
}
