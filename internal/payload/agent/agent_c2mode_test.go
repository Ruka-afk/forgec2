//go:build linux || windows || darwin

package main

import (
	"testing"
)

func TestBuildTransportCandidatesIncludesDNSWhenConfigured(t *testing.T) {
	oldP, oldB, oldD, oldS := Protocol, BeaconTransport, DNSDomain, DNSServer
	defer func() { Protocol, BeaconTransport, DNSDomain, DNSServer = oldP, oldB, oldD, oldS }()

	Protocol, BeaconTransport = "dns", "dns"
	DNSDomain, DNSServer = "example.com", "1.1.1.1"
	cands := buildTransportCandidates()
	found := false
	for _, c := range cands {
		if c == "dns" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected dns in candidates when DNSDomain/DNSServer set, got %v", cands)
	}
}

func TestBuildTransportCandidatesExcludesDNSWhenUnconfigured(t *testing.T) {
	oldP, oldB, oldD, oldS := Protocol, BeaconTransport, DNSDomain, DNSServer
	defer func() { Protocol, BeaconTransport, DNSDomain, DNSServer = oldP, oldB, oldD, oldS }()

	Protocol, BeaconTransport = "http", "http"
	DNSDomain, DNSServer = "", ""
	cands := buildTransportCandidates()
	for _, c := range cands {
		if c == "dns" {
			t.Fatalf("dns should be excluded without DNSDomain/DNSServer, got %v", cands)
		}
	}
}

func TestTransportFailoverRotatesAfterThreshold(t *testing.T) {
	oldP, oldB, oldD, oldS := Protocol, BeaconTransport, DNSDomain, DNSServer
	oldStreak, oldIdx, oldCands := transportFailStreak, currentTransportIdx, transportCandidates
	defer func() {
		Protocol, BeaconTransport, DNSDomain, DNSServer = oldP, oldB, oldD, oldS
		transportFailStreak, currentTransportIdx, transportCandidates = oldStreak, oldIdx, oldCands
	}()

	Protocol, BeaconTransport = "dns", "dns"
	DNSDomain, DNSServer = "example.com", "1.1.1.1"
	transportFailStreak = 0
	currentTransportIdx = 0
	transportCandidates = nil // force rebuild

	if effectiveTransport() != "dns" {
		t.Fatalf("precondition: effective transport should be dns")
	}
	// One short of threshold must not rotate.
	for i := 0; i < dnsFallbackThreshold-1; i++ {
		maybeRotateTransport()
	}
	if effectiveTransport() != "dns" {
		t.Fatalf("should not rotate before threshold, got %s", effectiveTransport())
	}
	// The threshold trigger must advance to the next candidate (http).
	maybeRotateTransport()
	if effectiveTransport() != "http" {
		t.Fatalf("expected failover to http, got %s", effectiveTransport())
	}
}
