package crypto

import (
	"bytes"
	"crypto/ecdh"
	"crypto/rand"
	"testing"
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

func TestSessionManagerNeedsRotation(t *testing.T) {
	sm, _ := NewSessionManager()
	curve := ecdh.X25519()
	agentKey, _ := curve.GenerateKey(rand.Reader)
	sm.EstablishSession("agent-1", agentKey.PublicKey().Bytes())

	if sm.NeedsRotation("agent-1") {
		t.Error("new session should not need rotation")
	}
	if sm.NeedsRotation("nonexistent") {
		t.Error("missing session should not need rotation")
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

func TestSessionManagerRotateKeyPair(t *testing.T) {
	sm, _ := NewSessionManager()
	oldPub := sm.GetPublicKey()

	if err := sm.RotateKeyPair(); err != nil {
		t.Fatalf("RotateKeyPair: %v", err)
	}

	newPub := sm.GetPublicKey()
	if bytes.Equal(oldPub, newPub) {
		t.Error("rotated key pair should differ from original")
	}
}

func TestSessionManagerRotateSessionKey(t *testing.T) {
	sm, _ := NewSessionManager()
	curve := ecdh.X25519()
	agentKey, _ := curve.GenerateKey(rand.Reader)
	sm.EstablishSession("agent-1", agentKey.PublicKey().Bytes())

	oldKey := make([]byte, 32)
	copy(oldKey, sm.GetSession("agent-1").SessionKey)

	newAgentKey, _ := curve.GenerateKey(rand.Reader)
	if err := sm.RotateSessionKey("agent-1", newAgentKey.PublicKey().Bytes()); err != nil {
		t.Fatalf("RotateSessionKey: %v", err)
	}

	newKey := sm.GetSession("agent-1").SessionKey
	if bytes.Equal(oldKey, newKey) {
		t.Error("rotated session key should differ")
	}
}

func TestSessionManagerRotateSessionKeyMissing(t *testing.T) {
	sm, _ := NewSessionManager()
	curve := ecdh.X25519()
	agentKey, _ := curve.GenerateKey(rand.Reader)
	err := sm.RotateSessionKey("nonexistent", agentKey.PublicKey().Bytes())
	if err != ErrNoSession {
		t.Errorf("expected ErrNoSession, got %v", err)
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
