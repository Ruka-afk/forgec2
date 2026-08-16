package main

import (
	"encoding/base64"
	"testing"

	"github.com/forgec2/forgec2/pkg/protocol"
)

// TestApplyServerNetworkConfig verifies that a server-delivered, secret-bound
// network config is decrypted and applied over the compile-time defaults, and
// that the runtime parse picks up the new values.
func TestApplyServerNetworkConfig(t *testing.T) {
	secret := make([]byte, 32)
	for i := range secret {
		secret[i] = byte(0x5A + i)
	}
	prevReg := RegSecretStr
	RegSecretStr = base64.StdEncoding.EncodeToString(secret)
	defer func() { RegSecretStr = prevReg }()

	// Preserve globals mutated by the apply path so other tests are unaffected.
	defer func(
		c2, ua, px, mp, ma, df, dnsDom, dnsSrv, bt, bu, proto string,
		iv, jit int, stv bool, lid uint, cidx int32, c2s []string,
	) func() {
		return func() {
			C2URL, UserAgent, ProxyStr, MalleablePrepend, MalleableAppend = c2, ua, px, mp, ma
			DomainFront, DNSDomain, DNSServer, BeaconTransport, C2URL, Protocol = df, dnsDom, dnsSrv, bt, c2, proto
			Interval, Jitter, SkipTLSVerify, ListenerID = iv, jit, stv, lid
			c2URLsStore(c2s, cidx)
		}
	}(C2URL, UserAgent, ProxyStr, MalleablePrepend, MalleableAppend,
		DomainFront, DNSDomain, DNSServer, BeaconTransport, C2URL, Protocol,
		Interval, Jitter, SkipTLSVerify, ListenerID, currentC2Idx.Load(), c2URLsSnapshot())()

	nc := &protocol.NetworkConfig{
		C2URL:            "https://delivered.example.com/http",
		Protocol:         "http",
		BeaconTransport:  "http",
		Interval:         90,
		Jitter:           12,
		UserAgent:        "CustomUA/1.0",
		Proxy:            "http://proxy:8080",
		SkipTLSVerify:    true,
		DomainFront:      "front.example.com",
		MalleablePrepend: "<x>",
		MalleableAppend:  "</x>",
		DNSDomain:        "dns.example.com",
		DNSServer:        "9.9.9.9",
	}
	b64, err := protocol.EncryptNetworkConfig(secret, nc)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	applyServerNetworkConfig(b64)

	if ProxyStr != nc.Proxy {
		t.Errorf("ProxyStr = %q, want %q", ProxyStr, nc.Proxy)
	}
	if MalleablePrepend != nc.MalleablePrepend || MalleableAppend != nc.MalleableAppend {
		t.Errorf("malleable = %q/%q, want %q/%q", MalleablePrepend, MalleableAppend, nc.MalleablePrepend, nc.MalleableAppend)
	}
	if Interval != nc.Interval {
		t.Errorf("Interval = %d, want %d", Interval, nc.Interval)
	}
	if Jitter != nc.Jitter {
		t.Errorf("Jitter = %d, want %d", Jitter, nc.Jitter)
	}
	if SkipTLSVerify != nc.SkipTLSVerify {
		t.Errorf("SkipTLSVerify = %v, want %v", SkipTLSVerify, nc.SkipTLSVerify)
	}
	if C2URL != nc.C2URL {
		t.Errorf("C2URL = %q, want %q", C2URL, nc.C2URL)
	}
	if urls := c2URLsSnapshot(); len(urls) != 1 || urls[0] != nc.C2URL {
		t.Errorf("C2URLs = %v, want [%q]", urls, nc.C2URL)
	}
}

// TestApplyServerNetworkConfigEmpty is a no-op when no config is delivered.
func TestApplyServerNetworkConfigEmpty(t *testing.T) {
	prevReg := RegSecretStr
	RegSecretStr = ""
	defer func() { RegSecretStr = prevReg }()
	// Must not panic with an empty blob.
	applyServerNetworkConfig("")
}
