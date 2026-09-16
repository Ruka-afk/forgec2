package server

import (
	"testing"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
)

// roeTestUUID was previously shared from the (now removed) chrome handler
// tests; it is just a fixed UUID string with no chrome semantics.
const roeTestUUID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

func TestCheckRoEDeniesTarget(t *testing.T) {
	s := newAgentTestServer(t)
	s.cfg = &config.Config{}
	s.cfg.Roe.Enabled = true
	s.cfg.Roe.DenyCIDRs = []string{"10.9.9.0/24"}
	if err := s.db.Create(&db.Implant{ID: roeTestUUID, Hostname: "box", IP: "10.0.0.5"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.checkRoE(roeTestUUID, "portscan", "10.9.9.8:80,443", "", ""); err == nil {
		t.Fatal("expected deny")
	}
	if err := s.checkRoE(roeTestUUID, "portscan", "10.0.0.9:22", "", ""); err != nil {
		t.Fatalf("in-scope scan blocked: %v", err)
	}
	if err := s.checkRoE(roeTestUUID, "set_sleep", "10.9.9.8", "", ""); err != nil {
		t.Fatalf("set_sleep must bypass RoE: %v", err)
	}
}

func TestCheckRoEAllowList(t *testing.T) {
	s := newAgentTestServer(t)
	s.cfg = &config.Config{}
	s.cfg.Roe.Enabled = true
	s.cfg.Roe.AllowCIDRs = []string{"10.1.0.0/16"}
	if err := s.db.Create(&db.Implant{ID: roeTestUUID, Hostname: "box", IP: "10.1.2.3"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.checkRoE(roeTestUUID, "ssh_lateral", "8.8.8.8:22 root", "", ""); err == nil {
		t.Fatal("expected allow-list miss")
	}
}

func TestCheckRoEScansDataAndPath(t *testing.T) {
	s := newAgentTestServer(t)
	s.cfg = &config.Config{}
	s.cfg.Roe.Enabled = true
	s.cfg.Roe.DenyCIDRs = []string{"10.9.9.0/24"}
	if err := s.db.Create(&db.Implant{ID: roeTestUUID, Hostname: "box", IP: "10.0.0.5"}).Error; err != nil {
		t.Fatal(err)
	}
	// Denied target smuggled via Data/Path instead of Command.
	if err := s.checkRoE(roeTestUUID, "lateral", "clean", "target=10.9.9.8", ""); err == nil {
		t.Fatal("expected deny via Data field")
	}
	if err := s.checkRoE(roeTestUUID, "lateral", "clean", "", `C:\out\10.9.9.8.txt`); err == nil {
		t.Fatal("expected deny via Path field")
	}
	if err := s.checkRoE(roeTestUUID, "lateral", "clean", "target=10.0.0.9", ""); err != nil {
		t.Fatalf("clean data blocked: %v", err)
	}
}

func TestCheckRoEIPv6AndDangerousScope(t *testing.T) {
	s := newAgentTestServer(t)
	s.cfg = &config.Config{}
	s.cfg.Roe.Enabled = true
	s.cfg.Roe.DenyCIDRs = []string{"fd00:dead::/32"}
	if err := s.db.Create(&db.Implant{ID: roeTestUUID, Hostname: "box", IP: "10.0.0.5"}).Error; err != nil {
		t.Fatal(err)
	}
	// Credential theft is RoE-scoped via the dangerous-types union.
	if err := s.checkRoE(roeTestUUID, "mimikatz", "sekurlsa::logonpasswords target fd00:dead::5", "", ""); err == nil {
		t.Fatal("expected deny for IPv6 target on dangerous type")
	}
	// Destructive ops are no longer always-allowed.
	if err := s.checkRoE(roeTestUUID, "uninstall", "target fd00:dead::6", "", ""); err == nil {
		t.Fatal("uninstall must be RoE-scoped now")
	}
}

func TestCheckRoEDomains(t *testing.T) {
	s := newAgentTestServer(t)
	s.cfg = &config.Config{}
	s.cfg.Roe.Enabled = true
	s.cfg.Roe.DenyDomains = []string{"evil.example.com"}
	s.cfg.Roe.AllowDomains = []string{"corp.example.com"}
	if err := s.db.Create(&db.Implant{ID: roeTestUUID, Hostname: "box", IP: "10.0.0.5"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.checkRoE(roeTestUUID, "lateral", "copy to sub.evil.example.com", "", ""); err == nil {
		t.Fatal("expected deny for denied domain")
	}
	if err := s.checkRoE(roeTestUUID, "lateral", "copy to files.corp.example.com", "", ""); err != nil {
		t.Fatalf("allowed domain blocked: %v", err)
	}
	if err := s.checkRoE(roeTestUUID, "lateral", "copy to other.example.org", "", ""); err == nil {
		t.Fatal("expected allow-list miss for other domain")
	}
}
