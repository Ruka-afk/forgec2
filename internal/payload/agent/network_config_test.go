package main

import (
	"encoding/base64"
	"strings"
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

	// Build-time chain: the delivered primary must replace this first entry
	// while the operator's failover backups survive.
	const backupURL = "https://backup.example.com:9443"
	C2URL = "https://compile-primary.example.com:443," + backupURL
	c2URLsStore(splitC2URLList(C2URL), 0)

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
	wantURLs := []string{nc.C2URL, backupURL}
	if C2URL != strings.Join(wantURLs, ",") {
		t.Errorf("C2URL = %q, want %q", C2URL, strings.Join(wantURLs, ","))
	}
	if urls := c2URLsSnapshot(); strings.Join(urls, ",") != strings.Join(wantURLs, ",") {
		t.Errorf("C2URLs = %v, want %v", urls, wantURLs)
	}
}

func TestMergeServerC2URL(t *testing.T) {
	tests := []struct {
		name     string
		primary  string
		existing []string
		want     string
	}{
		{
			name:     "replaces primary and keeps backups",
			primary:  "https://new.example.com:443",
			existing: []string{"https://old.example.com:443", "https://b1.example.com:443", "https://b2.example.com:443"},
			want:     "https://new.example.com:443,https://b1.example.com:443,https://b2.example.com:443",
		},
		{
			name:     "dedupes a backup that becomes primary",
			primary:  "https://b1.example.com:443",
			existing: []string{"https://old.example.com:443", "https://b1.example.com:443", "https://b2.example.com:443"},
			want:     "https://b1.example.com:443,https://b2.example.com:443",
		},
		{
			name:     "empty primary leaves chain untouched",
			primary:  "",
			existing: []string{"https://old.example.com:443", "https://b1.example.com:443"},
			want:     "https://old.example.com:443,https://b1.example.com:443",
		},
		{
			name:     "empty chain yields primary",
			primary:  "https://new.example.com:443",
			existing: nil,
			want:     "https://new.example.com:443",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := strings.Join(mergeServerC2URL(tc.primary, tc.existing), ","); got != tc.want {
				t.Fatalf("mergeServerC2URL() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestHasValidC2SchemeCoversAllTransports(t *testing.T) {
	for _, u := range []string{
		"https://c2.example.com:443",
		"wss://c2.example.com:443",
		"h2c://c2.example.com:8080",
		"grpc://c2.example.com:443",
		"quic://c2.example.com:443",
		"mtls://c2.example.com:443",
	} {
		if !hasValidC2Scheme(u) {
			t.Errorf("hasValidC2Scheme(%q) = false", u)
		}
	}
	if hasValidC2Scheme("javascript://evil") {
		t.Error("javascript scheme was accepted")
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
