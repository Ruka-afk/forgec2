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

var roeScopedTaskTypes = map[string]bool{
	"lateral": true, "lateral_wmi": true, "lateral_winrm": true, "lateral_psexec": true,
	"lateral_dcom": true, "ssh_lateral": true, "scp_upload": true, "ssh_tunnel": true,
	"portscan": true, "password_spray": true, "net_scan_smb": true, "net_enum_hosts": true,
	"coerce_printerbug": true, "coerce_petitpotam": true, "coerce_dfs": true,
	"relay_ntlm_start": true, "pass_the_hash": true, "usb_drop": true,
}

var roeAlwaysAllowed = map[string]bool{
	"set_sleep": true, "uninstall": true, "kill": true, "help": true, "hostinfo": true,
}

func (s *Server) checkRoE(agentID, taskType, command string) error {
	if s.cfg == nil || !s.cfg.Roe.Enabled {
		return nil
	}
	if roeAlwaysAllowed[taskType] {
		return nil
	}

	deny := parseCIDRs(s.cfg.Roe.DenyCIDRs)
	allow := parseCIDRs(s.cfg.Roe.AllowCIDRs)

	var agent db.Implant
	if err := s.db.Select("id, public_ip, ip").First(&agent, "id = ?", agentID).Error; err == nil {
		for _, ipStr := range []string{agent.PublicIP, agent.IP} {
			ip := net.ParseIP(strings.TrimSpace(ipStr))
			if ip == nil {
				continue
			}
			if cidrContains(deny, ip) {
				return fmt.Errorf("RoE: agent IP %s is in a denied CIDR", ip)
			}
			if len(allow) > 0 && !cidrContains(allow, ip) && roeScopedTaskTypes[taskType] {
				return fmt.Errorf("RoE: agent IP %s is outside the allowed CIDRs", ip)
			}
		}
	}

	if !roeScopedTaskTypes[taskType] {
		return nil
	}
	for _, m := range ipv4InText.FindAllString(command, -1) {
		ip := net.ParseIP(m)
		if ip == nil {
			continue
		}
		if cidrContains(deny, ip) {
			return fmt.Errorf("RoE: target %s is in a denied CIDR", ip)
		}
		if len(allow) > 0 && !cidrContains(allow, ip) {
			return fmt.Errorf("RoE: target %s is outside the allowed CIDRs", ip)
		}
	}
	return nil
}

func parseCIDRs(raw []string) []net.IPNet {
	var out []net.IPNet
	for _, s := range raw {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if !strings.Contains(s, "/") {
			s += "/32"
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
	if s.cfg == nil {
		return
	}
	types := s.cfg.Server.AutoRecon
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
