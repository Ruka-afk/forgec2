package server

import (
	"encoding/base64"
	"testing"
)

func TestDecodeBeaconIdentityBase64(t *testing.T) {
	info := map[string]string{
		"encoding": "base64",
		"hostname": base64.StdEncoding.EncodeToString([]byte("DESKTOP-LAB01")),
		"username": base64.StdEncoding.EncodeToString([]byte("Administrator")),
		"ip":       base64.StdEncoding.EncodeToString([]byte("192.168.1.24")),
	}
	h, u, ip := decodeBeaconIdentity(info)
	if h != "DESKTOP-LAB01" || u != "Administrator" || ip != "192.168.1.24" {
		t.Fatalf("decoded identity = %q %q %q", h, u, ip)
	}
}

func TestApplyDecodedBeaconIdentityRewritesInfo(t *testing.T) {
	info := map[string]string{
		"encoding": "base64",
		"hostname": base64.StdEncoding.EncodeToString([]byte("WIN-BOX")),
		"username": base64.StdEncoding.EncodeToString([]byte("bob")),
		"ip":       base64.StdEncoding.EncodeToString([]byte("10.1.2.3")),
		"os":       "windows",
	}
	applyDecodedBeaconIdentity(info)
	if info["hostname"] != "WIN-BOX" || info["username"] != "bob" || info["ip"] != "10.1.2.3" {
		t.Fatalf("info after apply: %+v", info)
	}
	if info["os"] != "windows" {
		t.Fatalf("os should stay plaintext, got %q", info["os"])
	}
}

func TestDecodeBeaconIdentityPlaintext(t *testing.T) {
	info := map[string]string{
		"hostname": "plain-host",
		"username": "alice",
		"ip":       "10.0.0.2",
	}
	h, u, ip := decodeBeaconIdentity(info)
	if h != "plain-host" || u != "alice" || ip != "10.0.0.2" {
		t.Fatalf("plaintext identity mutated: %q %q %q", h, u, ip)
	}
}
