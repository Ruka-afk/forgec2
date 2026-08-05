package crypto

import (
	"bytes"
	"crypto/ecdh"
	"crypto/rand"
	"testing"
	"time"
)

func TestSessionManagerEstablishAndGet(t *testing.T) {
	sm, err := NewSessionManager()
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}

	curve := ecdh.X25519()
	agentKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	if err := sm.EstablishSession("agent-1", agentKey.PublicKey().Bytes()); err != nil {
		t.Fatalf("EstablishSession: %v", err)
	}

	sess := sm.GetSession("agent-1")
	if sess == nil {
		t.Fatal("GetSession returned nil for established session")
	}
	if sess.AgentID != "agent-1" {
		t.Errorf("AgentID = %q, want %q", sess.AgentID, "agent-1")
	}
	if len(sess.SessionKey) != 32 {
		t.Errorf("SessionKey len = %d, want 32", len(sess.SessionKey))
	}
}

func TestSessionManagerGetSessionMissing(t *testing.T) {
	sm, _ := NewSessionManager()
	if s := sm.GetSession("nonexistent"); s != nil {
		t.Errorf("expected nil for missing session, got %+v", s)
	}
}

func TestSessionManagerEncryptDecrypt(t *testing.T) {
	sm, _ := NewSessionManager()
	curve := ecdh.X25519()
	agentKey, _ := curve.GenerateKey(rand.Reader)

	if err := sm.EstablishSession("agent-1", agentKey.PublicKey().Bytes()); err != nil {
		t.Fatalf("EstablishSession: %v", err)
	}

	plaintext := []byte("hello, world")
	encrypted, err := sm.Encrypt("agent-1", plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if bytes.Equal(encrypted, plaintext) {
		t.Error("encrypted data should differ from plaintext")
	}

	decrypted, err := sm.Decrypt("agent-1", encrypted)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("decrypted = %q, want %q", decrypted, plaintext)
	}
}

func TestSessionManagerEncryptB64DecryptB64(t *testing.T) {
	sm, _ := NewSessionManager()
	curve := ecdh.X25519()
	agentKey, _ := curve.GenerateKey(rand.Reader)

	sm.EstablishSession("agent-1", agentKey.PublicKey().Bytes())

	plaintext := []byte("base64 test data")
	b64, err := sm.EncryptB64("agent-1", plaintext)
	if err != nil {
		t.Fatalf("EncryptB64: %v", err)
	}
	if b64 == "" {
		t.Fatal("EncryptB64 returned empty string")
	}

	decrypted, err := sm.DecryptB64("agent-1", b64)
	if err != nil {
		t.Fatalf("DecryptB64: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("decrypted = %q, want %q", decrypted, plaintext)
	}
}

func TestSessionManagerEncryptMissingSession(t *testing.T) {
	sm, _ := NewSessionManager()
	_, err := sm.Encrypt("nonexistent", []byte("data"))
	if err != ErrNoSession {
		t.Errorf("expected ErrNoSession, got %v", err)
	}
}

func TestSessionManagerDecryptMissingSession(t *testing.T) {
	sm, _ := NewSessionManager()
	_, err := sm.Decrypt("nonexistent", []byte("data"))
	if err != ErrNoSession {
		t.Errorf("expected ErrNoSession, got %v", err)
	}
}

func TestSessionManagerDecryptInvalidCiphertext(t *testing.T) {
	sm, _ := NewSessionManager()
	curve := ecdh.X25519()
	agentKey, _ := curve.GenerateKey(rand.Reader)
	sm.EstablishSession("agent-1", agentKey.PublicKey().Bytes())

	_, err := sm.Decrypt("agent-1", []byte("short"))
	if err == nil {
		t.Error("expected error for short ciphertext")
	}
}

func TestSessionManagerIncrementMessageCount(t *testing.T) {
	sm, _ := NewSessionManager()
	curve := ecdh.X25519()
	agentKey, _ := curve.GenerateKey(rand.Reader)
	sm.EstablishSession("agent-1", agentKey.PublicKey().Bytes())

	sess := sm.GetSession("agent-1")
	if sess.MessageCount != 0 {
		t.Errorf("initial MessageCount = %d, want 0", sess.MessageCount)
	}

	sm.IncrementMessageCount("agent-1")
	sess = sm.GetSession("agent-1")
	if sess.MessageCount != 1 {
		t.Errorf("MessageCount after increment = %d, want 1", sess.MessageCount)
	}

	sm.IncrementMessageCount("agent-1")
	sm.IncrementMessageCount("agent-1")
	sess = sm.GetSession("agent-1")
	if sess.MessageCount != 3 {
		t.Errorf("MessageCount after 3 increments = %d, want 3", sess.MessageCount)
	}
}

func TestSessionManagerEstablishAllowsOverwrite(t *testing.T) {
	// v2 semantics: EstablishSession may overwrite an active session because
	// the caller (processAuthFrame) has already authenticated the agent via
	// the registration key. Rekey / restart recovery depends on this.
	sm, _ := NewSessionManager()
	curve := ecdh.X25519()
	agentKey, _ := curve.GenerateKey(rand.Reader)
	if err := sm.EstablishSession("agent-1", agentKey.PublicKey().Bytes()); err != nil {
		t.Fatalf("initial EstablishSession: %v", err)
	}

	// Use the session so it counts as active.
	plaintext := []byte("data")
	if _, err := sm.Encrypt("agent-1", plaintext); err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// A new handshake (rekey) must replace the session key.
	otherKey, _ := curve.GenerateKey(rand.Reader)
	if err := sm.EstablishSession("agent-1", otherKey.PublicKey().Bytes()); err != nil {
		t.Errorf("expected overwrite to succeed, got %v", err)
	}

	// The new key must decrypt; the old key material is gone.
	encrypted, err := sm.Encrypt("agent-1", plaintext)
	if err != nil {
		t.Fatalf("Encrypt after overwrite: %v", err)
	}
	decrypted, err := sm.Decrypt("agent-1", encrypted)
	if err != nil || string(decrypted) != "data" {
		t.Errorf("post-overwrite roundtrip failed: decrypted=%q err=%v", decrypted, err)
	}
}

func TestSessionManagerEstablishAllowsIdleOverwrite(t *testing.T) {
	sm, _ := NewSessionManagerWithConfig(5 * time.Minute)
	curve := ecdh.X25519()
	agentKey, _ := curve.GenerateKey(rand.Reader)
	if err := sm.EstablishSession("agent-1", agentKey.PublicKey().Bytes()); err != nil {
		t.Fatalf("initial EstablishSession: %v", err)
	}

	// A never-used session may be replaced (harmless: nothing was encrypted yet).
	newKey, _ := curve.GenerateKey(rand.Reader)
	if err := sm.EstablishSession("agent-1", newKey.PublicKey().Bytes()); err != nil {
		t.Errorf("expected idle session overwrite to succeed, got %v", err)
	}
}

func TestSessionManagerEstablishAllowsExpiredOverwrite(t *testing.T) {
	sm, _ := NewSessionManagerWithConfig(5 * time.Minute)
	curve := ecdh.X25519()
	agentKey, _ := curve.GenerateKey(rand.Reader)
	if err := sm.EstablishSession("agent-1", agentKey.PublicKey().Bytes()); err != nil {
		t.Fatalf("initial EstablishSession: %v", err)
	}
	if _, err := sm.Encrypt("agent-1", []byte("data")); err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Age the session beyond maxAge so an agent re-handshaking (e.g. after a
	// process restart) is allowed to establish a fresh session.
	sess := sm.GetSession("agent-1")
	sess.CreatedAt = time.Now().Add(-10 * time.Minute)

	newKey, _ := curve.GenerateKey(rand.Reader)
	if err := sm.EstablishSession("agent-1", newKey.PublicKey().Bytes()); err != nil {
		t.Errorf("expected expired session overwrite to succeed, got %v", err)
	}
	if got := sm.GetSession("agent-1").MessageCount; got != 0 {
		t.Errorf("MessageCount after re-handshake = %d, want 0", got)
	}
}

func TestSessionManagerMultipleAgents(t *testing.T) {
	sm, _ := NewSessionManager()
	curve := ecdh.X25519()

	for i := 0; i < 10; i++ {
		agentKey, _ := curve.GenerateKey(rand.Reader)
		agentID := "agent-" + string(rune('a'+i))
		sm.EstablishSession(agentID, agentKey.PublicKey().Bytes())
	}

	for i := 0; i < 10; i++ {
		agentID := "agent-" + string(rune('a'+i))
		sess := sm.GetSession(agentID)
		if sess == nil {
			t.Errorf("session missing for %s", agentID)
		}
	}
}

func TestSessionManagerConcurrentAccess(t *testing.T) {
	sm, _ := NewSessionManager()
	curve := ecdh.X25519()
	agentKey, _ := curve.GenerateKey(rand.Reader)
	sm.EstablishSession("agent-1", agentKey.PublicKey().Bytes())

	done := make(chan struct{})
	for i := 0; i < 50; i++ {
		go func() {
			sm.GetSession("agent-1")
			sm.IncrementMessageCount("agent-1")
			done <- struct{}{}
		}()
	}
	for i := 0; i < 50; i++ {
		<-done
	}

	sess := sm.GetSession("agent-1")
	if sess.MessageCount != 50 {
		t.Errorf("MessageCount = %d, want 50", sess.MessageCount)
	}
}

// TestSessionManagerSessionCap verifies the session map never exceeds maxSessions,
// even when unique (spoofed) agent IDs are handed-shaken in a loop.
func TestSessionManagerSessionCap(t *testing.T) {
	sm, _ := NewSessionManager()
	sm.maxSessions = 5
	curve := ecdh.X25519()

	for i := 0; i < 50; i++ {
		agentKey, _ := curve.GenerateKey(rand.Reader)
		agentID := "flood-" + string(rune('a'+(i%26))) + "-" + string(rune('0'+(i/26)))
		if err := sm.EstablishSession(agentID, agentKey.PublicKey().Bytes()); err != nil {
			t.Fatalf("EstablishSession %s: %v", agentID, err)
		}
		if sm.mu.RLock(); len(sm.sessions) > sm.maxSessions {
			sm.mu.RUnlock()
			t.Fatalf("session map grew past cap: len=%d cap=%d", len(sm.sessions), sm.maxSessions)
		} else {
			sm.mu.RUnlock()
		}
	}

	sm.mu.RLock()
	got := len(sm.sessions)
	sm.mu.RUnlock()
	if got != sm.maxSessions {
		t.Errorf("session map size = %d, want capped at %d", got, sm.maxSessions)
	}
}

// TestSessionManagerGetPublicKeyConcurrent ensures GetPublicKey is safe under
// concurrent access (regression: it read privateKey without a lock).
func TestSessionManagerGetPublicKeyConcurrent(t *testing.T) {
	sm, _ := NewSessionManager()
	done := make(chan struct{})
	for i := 0; i < 50; i++ {
		go func() {
			key := sm.GetPublicKey()
			if len(key) == 0 {
				t.Error("GetPublicKey returned empty key")
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < 50; i++ {
		<-done
	}
}

func BenchmarkSessionManagerEncrypt(b *testing.B) {
	sm, _ := NewSessionManager()
	curve := ecdh.X25519()
	agentKey, _ := curve.GenerateKey(rand.Reader)
	sm.EstablishSession("bench-agent", agentKey.PublicKey().Bytes())

	plaintext := make([]byte, 256)
	rand.Read(plaintext)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sm.Encrypt("bench-agent", plaintext)
	}
}

func BenchmarkSessionManagerDecrypt(b *testing.B) {
	sm, _ := NewSessionManager()
	curve := ecdh.X25519()
	agentKey, _ := curve.GenerateKey(rand.Reader)
	sm.EstablishSession("bench-agent", agentKey.PublicKey().Bytes())

	plaintext := make([]byte, 256)
	rand.Read(plaintext)
	encrypted, _ := sm.Encrypt("bench-agent", plaintext)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sm.Decrypt("bench-agent", encrypted)
	}
}

func BenchmarkSessionManagerEstablish(b *testing.B) {
	sm, _ := NewSessionManager()
	curve := ecdh.X25519()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		agentKey, _ := curve.GenerateKey(rand.Reader)
		sm.EstablishSession("bench-agent", agentKey.PublicKey().Bytes())
	}
}
