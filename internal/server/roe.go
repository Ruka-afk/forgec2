package server

import (
	"fmt"
	"log/slog"
	"net"
	"regexp"
	"strings"

	"github.com/forgec2/forgec2/internal/db"
)

var ipv4InText = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)

// ipv6InText matches colon-hex groups (full, compressed, and v4-mapped
// forms). Candidates are validated with net.ParseIP; zone IDs are stripped.
var ipv6InText = regexp.MustCompile(`\b(?:[0-9a-fA-F]{0,4}:){2,}[0-9a-fA-F:.]*[0-9a-fA-F]\b`)

// hostInText extracts DNS-name-like tokens for domain allow/deny matching.
var hostInText = regexp.MustCompile(`\b(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}\b`)

var roeScopedTaskTypes = map[string]bool{
	"lateral": true, "lateral_wmi": true, "lateral_winrm": true, "lateral_psexec": true,
	"lateral_dcom": true, "ssh_lateral": true, "scp_upload": true, "ssh_tunnel": true,
	"portscan": true, "password_spray": true, "net_scan_smb": true, "net_enum_hosts": true,
	"coerce_printerbug": true, "coerce_petitpotam": true, "coerce_dfs": true,
	"relay_ntlm_start": true, "pass_the_hash": true, "usb_drop": true,
}

// roeScoped reports whether a task type falls under RoE target checks: the
// explicit network-operation set plus every dangerous type (credential
// theft, persistence, EDR antagonism, destructive ops). Destructive
// self-harm (uninstall/kill) is covered by target checks like anything else.
func roeScoped(taskType string) bool {
	return roeScopedTaskTypes[taskType] || dangerousTaskTypes[taskType]
}

var roeAlwaysAllowed = map[string]bool{
	"set_sleep": true, "help": true, "hostinfo": true,
}

// roeConfig is an immutable snapshot of the Rules-of-Engagement gate.
//
// The RoE section is hot-reloadable (diffConfig's "roe" token), and a reload
// swaps the whole struct under configMu's write lock. These checks read it on
// every task-creation path, so an unlocked read was a data race — and this is
// a security boundary: a torn read of a slice header could evaluate a target
// against a half-old/half-new allow list and let a denied IP through.
//
// The slices are shared by reference and that is safe: CopyFrom replaces the
// headers, it never writes a live backing array, so a snapshot keeps reading
// the generation it captured.
type roeConfig struct {
	enabled      bool
	deny         []string
	allow        []string
	denyDomains  []string
	allowDomains []string
}

// roeConfig reads the gate under the config read lock.
func (s *Server) roeConfig() roeConfig {
	s.configMu.RLock()
	defer s.configMu.RUnlock()
	return s.roeConfigLocked()
}

// roeConfigLocked reads the gate. The caller MUST hold s.configMu.
func (s *Server) roeConfigLocked() roeConfig { // config-lock: caller holds the read or write lock
	if s.cfg == nil {
		return roeConfig{}
	}
	return roeConfig{
		enabled:      s.cfg.Roe.Enabled,
		deny:         s.cfg.Roe.DenyCIDRs,
		allow:        s.cfg.Roe.AllowCIDRs,
		denyDomains:  s.cfg.Roe.DenyDomains,
		allowDomains: s.cfg.Roe.AllowDomains,
	}
}

// checkRoE is the Rules-of-Engagement gate for creation paths that do not
// already hold the agent row: it resolves the row, then hands off to
// checkRoELoaded. Batch paths that loaded the agents up front call
// checkRoELoaded directly to avoid one point SELECT per agent.
func (s *Server) checkRoE(agentID, taskType, command, data, path string) error {
	if !s.roeConfig().enabled {
		return nil
	}
	if roeAlwaysAllowed[taskType] {
		return nil
	}
	var agent db.Implant
	if err := s.db.Select("id, public_ip, ip").First(&agent, "id = ?", agentID).Error; err != nil {
		// Unknown agent: no IP to check, but scoped task types still get
		// the target checks below (a zero-valued agent's empty IPs are
		// skipped by the nil-parse in checkRoELoaded).
		agent = db.Implant{}
	}
	return s.checkRoELoaded(agent, taskType, command, data, path)
}

// checkRoELoaded runs the RoE checks against an already-loaded agent row.
// The agent's IPs are checked against the CIDR lists only when present:
// empty strings parse to nil and are skipped, so a zero-valued agent (row
// not found) is equivalent to the not-found case in checkRoE.
func (s *Server) checkRoELoaded(agent db.Implant, taskType, command, data, path string) error {
	roe := s.roeConfig()
	if !roe.enabled {
		return nil
	}
	if roeAlwaysAllowed[taskType] {
		return nil
	}

	deny := parseCIDRs(roe.deny)
	allow := parseCIDRs(roe.allow)
	denyDomains := normalizeDomains(roe.denyDomains)
	allowDomains := normalizeDomains(roe.allowDomains)

	for _, ipStr := range []string{agent.PublicIP, agent.IP} {
		ip := net.ParseIP(strings.TrimSpace(ipStr))
		if ip == nil {
			continue
		}
		if cidrContains(deny, ip) {
			return fmt.Errorf("RoE: agent IP %s is in a denied CIDR", ip)
		}
		if len(allow) > 0 && !cidrContains(allow, ip) && roeScoped(taskType) {
			return fmt.Errorf("RoE: agent IP %s is outside the allowed CIDRs", ip)
		}
	}

	if !roeScoped(taskType) {
		return nil
	}
	fields := command + "\x00" + data + "\x00" + path
	// IP targets (v4 + v6) across all free-text fields, not just Command.
	seen := make(map[string]struct{})
	for _, m := range ipv4InText.FindAllString(fields, -1) {
		if ip := net.ParseIP(m); ip != nil {
			seen[ip.String()] = struct{}{}
		}
	}
	for _, m := range ipv6InText.FindAllString(fields, -1) {
		m = strings.SplitN(m, "%", 2)[0] // strip zone id
		if ip := net.ParseIP(m); ip != nil {
			seen[ip.String()] = struct{}{}
		}
	}
	for ipStr := range seen {
		ip := net.ParseIP(ipStr)
		if cidrContains(deny, ip) {
			return fmt.Errorf("RoE: target %s is in a denied CIDR", ip)
		}
		if len(allow) > 0 && !cidrContains(allow, ip) {
			return fmt.Errorf("RoE: target %s is outside the allowed CIDRs", ip)
		}
	}
	// Hostname targets: suffix match against domain lists.
	for _, h := range hostInText.FindAllString(fields, -1) {
		host := strings.ToLower(h)
		for _, d := range denyDomains {
			if host == d || strings.HasSuffix(host, "."+d) {
				return fmt.Errorf("RoE: target %s matches denied domain %s", h, d)
			}
		}
		if len(allowDomains) > 0 {
			matched := false
			for _, d := range allowDomains {
				if host == d || strings.HasSuffix(host, "."+d) {
					matched = true
					break
				}
			}
			if !matched {
				return fmt.Errorf("RoE: target %s is outside the allowed domains", h)
			}
		}
	}
	return nil
}

// normalizeDomains lower-cases and trims domain list entries, dropping
// empties and leading dots/wildcards ("*.example.com" -> "example.com").
func normalizeDomains(raw []string) []string {
	var out []string
	for _, d := range raw {
		d = strings.ToLower(strings.TrimSpace(d))
		d = strings.TrimPrefix(d, "*.")
		d = strings.TrimPrefix(d, ".")
		if d != "" {
			out = append(out, d)
		}
	}
	return out
}

func parseCIDRs(raw []string) []net.IPNet {
	var out []net.IPNet
	for _, s := range raw {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if !strings.Contains(s, "/") {
			// Bare IP: /32 for v4, /128 for v6 (appending /32 to a bare
			// IPv6 address would make it unparsable and silently drop it).
			if strings.Contains(s, ":") {
				s += "/128"
			} else {
				s += "/32"
			}
		}
		_, n, err := net.ParseCIDR(s)
		if err == nil {
			out = append(out, *n)
		}
	}
	return out
}

func cidrContains(nets []net.IPNet, ip net.IP) bool {
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func (s *Server) queueAutoRecon(agent db.Implant) {
	s.configMu.RLock()
	var types []string
	if s.cfg != nil {
		types = s.cfg.Server.AutoRecon
	}
	s.configMu.RUnlock()
	// server.auto_recon is hot-reloadable; CopyFrom replaces the slice
	// header under the write lock, so read it locked and iterate the
	// snapshot (the backing array is never written in place).
	if len(types) == 0 {
		return
	}
	for _, tt := range types {
		tt = strings.TrimSpace(tt)
		if tt == "" {
			continue
		}
		if _, err := s.createTask(agent.ID, tt, "", "", "", "", 0, 0); err != nil {
			slog.Warn("auto-recon task skipped", "agent_id", agent.ID, "type", tt, "err", err)
		}
	}
}
