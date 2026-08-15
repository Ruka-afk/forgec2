//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"encoding/base64"
	"testing"
)

// TestLoadAgentRegKeyRequiresV3Secret verifies the v2 fleet-key fallback is
// gone: without a per-implant v3 secret the agent must fail closed (nil), and a
// valid 32-byte v3 secret is returned verbatim.
func TestLoadAgentRegKeyRequiresV3Secret(t *testing.T) {
	orig := RegSecretStr
	defer func() { RegSecretStr = orig }()

	RegSecretStr = ""
	if k := loadAgentRegKey(); k != nil {
		t.Fatalf("expected nil reg key without v3 secret (v2 fallback must be disabled), got %d bytes", len(k))
	}

	secret := make([]byte, 32)
	for i := range secret {
		secret[i] = byte(i)
	}
	RegSecretStr = base64.StdEncoding.EncodeToString(secret)
	k := loadAgentRegKey()
	if len(k) != 32 || string(k) != string(secret) {
		t.Fatalf("v3 secret not returned correctly: len=%d", len(k))
	}
}
