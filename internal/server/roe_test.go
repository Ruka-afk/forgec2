package server

import (
	"testing"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
)

func TestCheckRoEDeniesTarget(t *testing.T) {
	s := newAgentTestServer(t)
	s.cfg = &config.Config{}
	s.cfg.Roe.Enabled = true
	s.cfg.Roe.DenyCIDRs = []string{"10.9.9.0/24"}
	if err := s.db.Create(&db.Implant{ID: chromeTestUUID, Hostname: "box", IP: "10.0.0.5"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.checkRoE(chromeTestUUID, "portscan", "10.9.9.8:80,443"); err == nil {
		t.Fatal("expected deny")
	}
	if err := s.checkRoE(chromeTestUUID, "portscan", "10.0.0.9:22"); err != nil {
		t.Fatalf("in-scope scan blocked: %v", err)
	}
	if err := s.checkRoE(chromeTestUUID, "set_sleep", "10.9.9.8"); err != nil {
		t.Fatalf("set_sleep must bypass RoE: %v", err)
	}
}

func TestCheckRoEAllowList(t *testing.T) {
	s := newAgentTestServer(t)
	s.cfg = &config.Config{}
	s.cfg.Roe.Enabled = true
	s.cfg.Roe.AllowCIDRs = []string{"10.1.0.0/16"}
	if err := s.db.Create(&db.Implant{ID: chromeTestUUID, Hostname: "box", IP: "10.1.2.3"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.checkRoE(chromeTestUUID, "ssh_lateral", "8.8.8.8:22 root"); err == nil {
		t.Fatal("expected allow-list miss")
	}
}
