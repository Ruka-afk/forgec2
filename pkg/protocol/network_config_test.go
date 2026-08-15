package protocol

import (
	"bytes"
	"testing"
)

func TestNetworkConfigEncryptRoundTrip(t *testing.T) {
	secret := make([]byte, 32)
	for i := range secret {
		secret[i] = byte(i)
	}
	nc := &NetworkConfig{
		C2URL:            "https://c2.example.com/http",
		Protocol:         "http",
		BeaconTransport:  "http",
		Interval:         60,
		Jitter:           15,
		UserAgent:        "Mozilla/5.0",
		Proxy:            "http://proxy:8080",
		SkipTLSVerify:    true,
		DNSDomain:        "c2.example.com",
		DNSServer:        "1.2.3.4",
		DomainFront:      "cdn.example.com",
		MalleablePrepend: "<html>",
		MalleableAppend:  "</html>",
		BeaconURI:        "/api/v1/beacon",
	}
	b64, err := EncryptNetworkConfig(secret, nc)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	got, err := DecryptNetworkConfig(secret, b64)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal([]byte(got.C2URL), []byte(nc.C2URL)) {
		t.Errorf("C2URL mismatch: %q != %q", got.C2URL, nc.C2URL)
	}
	if got.Interval != nc.Interval || got.Jitter != nc.Jitter {
		t.Errorf("sleep mismatch: %+v != %+v", got, nc)
	}
	if got.MalleablePrepend != nc.MalleablePrepend || got.MalleableAppend != nc.MalleableAppend {
		t.Errorf("malleable mismatch: %+v", got)
	}
	if got.SkipTLSVerify != nc.SkipTLSVerify || got.Proxy != nc.Proxy {
		t.Errorf("proxy/tls mismatch: %+v", got)
	}
}

func TestNetworkConfigWrongKeyFails(t *testing.T) {
	secret := make([]byte, 32)
	other := make([]byte, 32)
	for i := range other {
		other[i] = byte(i + 1)
	}
	nc := &NetworkConfig{C2URL: "https://x", Interval: 10}
	b64, err := EncryptNetworkConfig(secret, nc)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if _, err := DecryptNetworkConfig(other, b64); err == nil {
		t.Fatal("decrypt with wrong key must fail")
	}
}

func TestNetworkConfigKeyLength(t *testing.T) {
	if _, err := DeriveNetworkConfigKey(make([]byte, 16)); err == nil {
		t.Fatal("non-32-byte secret must be rejected")
	}
}
