//go:build windows

package main

import "testing"

func TestHighValueDefaultAllowsShell(t *testing.T) {
	p := defaultProfiles[EnvHighValue]
	if !p.AllowShell {
		t.Fatal("high_value default profile must allow interactive shell")
	}
}

func liveDetector(t *testing.T) *EnvironmentDetector {
	t.Helper()
	return &EnvironmentDetector{
		cpuCores: 8,
		totalRAM: 16 * 1024 * 1024 * 1024,
		users:    3,
		services: 80,
	}
}

func TestClassifyWorkstationSQLIsNotHighValue(t *testing.T) {
	ed := liveDetector(t)
	ed.isSQL = true
	ed.isServer = false
	ed.classify()
	if ed.profile == nil {
		t.Fatal("nil profile")
	}
	if ed.profile.Class == EnvHighValue {
		t.Fatalf("workstation with SQL tools classified as %s", ed.profile.ClassLabel)
	}
	if !ed.profile.AllowShell {
		t.Fatal("workstation profile blocked shell")
	}
}

func TestClassifyDomainJoinedWorkstationIsNotDC(t *testing.T) {
	ed := liveDetector(t)
	ed.isDomainJoined = true
	ed.domainName = "CORP"
	ed.isServer = false
	ed.classify()
	if ed.profile.Class == EnvHighValue {
		t.Fatal("domain-joined workstation must not be high_value")
	}
}

func TestCorporateScreenCapturePolicy(t *testing.T) {
	ordinary := adjustedOpsProfile(EnvCorporate, false)
	if !ordinary.AllowScreenCapture {
		t.Fatal("ordinary corporate workstation must allow screen capture")
	}
	managed := adjustedOpsProfile(EnvCorporate, true)
	if managed.AllowScreenCapture {
		t.Fatal("EDR-managed corporate workstation must block screen capture")
	}
}

func TestClassifyHighValueServerKeepsShell(t *testing.T) {
	ed := liveDetector(t)
	ed.isServer = true
	ed.isDC = true
	ed.classify()
	if ed.profile.Class != EnvHighValue {
		t.Fatalf("got class %s", ed.profile.ClassLabel)
	}
	if !ed.profile.AllowShell {
		t.Fatal("high_value server must still allow shell")
	}
	if ed.profile.AllowInjection {
		t.Fatal("high_value should still suppress injection")
	}
}
