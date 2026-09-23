package payload

import (
	"path/filepath"
	"sync"
	"testing"
)

// TestBuildLdflagsStampsUpdatePubKey asserts the OTA closed loop's build-time
// step: generate-time ldflags must pin main.updatePinnedPubKeyHex so the
// implant can verify self_update signatures before any config_push.
func TestBuildLdflagsStampsUpdatePubKey(t *testing.T) {
	dir := t.TempDir()
	// Package once + path are process-global; reset so this test owns the key.
	updateSigningKeyOnce = sync.Once{}
	updateSigningKey = nil
	updateSigningErr = nil
	SetUpdateSigningKeyFile(filepath.Join(dir, "update_signing.key"))

	pub, err := UpdateSigningPublicKeyHex()
	if err != nil {
		t.Fatalf("public key: %v", err)
	}
	if pub == "" {
		t.Fatal("empty public key")
	}

	profile := defaultMalleableProfile()
	cfg := ImplantConfig{
		C2URL:     "http://10.0.0.1:8080",
		Protocol:  "http",
		Interval:  10,
		Jitter:    20,
		BeaconKey: "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899",
	}
	ldflags, _, _ := buildLdflags(cfg, profile, "linux")
	want := `-X "main.updatePinnedPubKeyHex=` + pub + `"`
	if !containsString(ldflags, want) {
		t.Fatalf("ldflags missing update pin %q: %s", want, ldflags)
	}
}
