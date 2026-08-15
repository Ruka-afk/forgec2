package main

import "testing"

// TestECDHInvalidateRegeneratesKeypair verifies that invalidation rotates the
// ephemeral keypair so a subsequent handshake produces a new shared secret
// (forward secrecy on rekey). The original implementation only nulled the
// session key and reused the same private key forever.
func TestECDHInvalidateRegeneratesKeypair(t *testing.T) {
	sess, err := newECDSession()
	if err != nil {
		t.Fatalf("newECDSession: %v", err)
	}
	before := sess.publicKeyB64()
	if before == "" {
		t.Fatal("initial public key is empty")
	}

	sess.invalidate()

	after := sess.publicKeyB64()
	if after == "" {
		t.Fatal("public key is empty after invalidate")
	}
	if before == after {
		t.Fatal("invalidate() did not regenerate the ephemeral keypair")
	}

	sess.mu.RLock()
	if sess.sessionKey != nil {
		t.Fatal("sessionKey should be nil after invalidate")
	}
	sess.mu.RUnlock()
}
