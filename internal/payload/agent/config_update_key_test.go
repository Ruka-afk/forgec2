package main

import (
	"strings"
	"testing"
)

// TestConfigPushUpdateKey pins the trust-root delivery: a valid 64-hex key is
// pinned into the runtime var and persisted; invalid shapes are rejected with
// a targeted error and leave the previous key untouched.
func TestConfigPushUpdateKey(t *testing.T) {
	prev := updatePinnedPubKeyHex
	defer func() { updatePinnedPubKeyHex = prev }()

	res := TaskResult{}
	handleConfigPush(Task{ID: 1, Type: "config_push",
		Data: `{"update_pub_key":"` + strings.Repeat("ab", 32) + `"}`}, &res)
	if res.Error != "" {
		t.Fatalf("valid key rejected: %s", res.Error)
	}
	if updatePinnedPubKeyHex != strings.Repeat("ab", 32) {
		t.Fatalf("key not pinned: %q", updatePinnedPubKeyHex)
	}

	// Invalid length → error, previous key retained.
	res = TaskResult{}
	handleConfigPush(Task{ID: 2, Type: "config_push", Data: `{"update_pub_key":"zz"}`}, &res)
	if res.Error == "" || !strings.Contains(res.Error, "invalid update_pub_key") {
		t.Fatalf("invalid key accepted: %q (err=%v)", res.Error, res.Error != "")
	}
	if updatePinnedPubKeyHex != strings.Repeat("ab", 32) {
		t.Fatalf("failed push mutated pinned key: %q", updatePinnedPubKeyHex)
	}
}

// TestLoadBeaconStateRestoresUpdateKey covers persistence across restarts.
func TestLoadBeaconStateRestoresUpdateKey(t *testing.T) {
	prev := updatePinnedPubKeyHex
	defer func() { updatePinnedPubKeyHex = prev }()
	updatePinnedPubKeyHex = ""

	persistUpdatePubKey(strings.Repeat("cd", 32))
	updatePinnedPubKeyHex = ""
	loadBeaconState()
	if updatePinnedPubKeyHex != strings.Repeat("cd", 32) {
		t.Fatalf("restored %q", updatePinnedPubKeyHex)
	}
}
