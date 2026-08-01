package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
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

func TestSessionManagerTryRotateSessionKey(t *testing.T) {
	sm, _ := NewSessionManager()
	curve := ecdh.X25519()
	agentKey, _ := curve.GenerateKey(rand.Reader)
	if err := sm.EstablishSession("agent-1", agentKey.PublicKey().Bytes()); err != nil {
		t.Fatalf("initial EstablishSession: %v", err)
	}

	// Encrypt under the original session key.
	plaintext := []byte("rotation data")
	ciphertext, err := sm.Encrypt("agent-1", plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Agent rotates its keypair; the ciphertext is now encrypted under the new
	// session key (agent-new-priv * server-pub == server-priv * agent-new-pub).
	newAgentKey, _ := curve.GenerateKey(rand.Reader)
	newCipher, err := encryptWithAgentPub(sm, newAgentKey.PublicKey().Bytes(), plaintext)
	if err != nil {
		t.Fatalf("encryptWithAgentPub: %v", err)
	}

	// TryRotateSessionKey must re-key and authenticate the new ciphertext.
	got, err := sm.TryRotateSessionKey("agent-1", newAgentKey.PublicKey().Bytes(), newCipher)
	if err != nil {
		t.Fatalf("TryRotateSessionKey: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Errorf("decrypted plaintext mismatch: got %q want %q", got, plaintext)
	}

	// The old ciphertext (under the previous key) must no longer decrypt.
	if _, err := sm.Decrypt("agent-1", ciphertext); err == nil {
		t.Error("old ciphertext should not decrypt after rotation")
	}
}

func TestSessionManagerTryRotateSessionKeyRejectsForged(t *testing.T) {
	sm, _ := NewSessionManager()
	curve := ecdh.X25519()
	agentKey, _ := curve.GenerateKey(rand.Reader)
	if err := sm.EstablishSession("agent-1", agentKey.PublicKey().Bytes()); err != nil {
		t.Fatalf("initial EstablishSession: %v", err)
	}

	// Legitimate ciphertext under the current key.
	plaintext := []byte("still intact")
	if _, err := sm.Encrypt("agent-1", plaintext); err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Attacker forges an ecdh_pub but does NOT know the matching private key, so
	// they can only send a ciphertext encrypted under the OLD (still-live) key.
	// The rotation must be rejected without destroying the active session.
	forgedKey, _ := curve.GenerateKey(rand.Reader)
	forgedCipher, err := sm.Encrypt("agent-1", []byte("stale ciphertext"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	if _, err := sm.TryRotateSessionKey("agent-1", forgedKey.PublicKey().Bytes(), forgedCipher); err == nil {
		t.Error("expected forged rotation to be rejected")
	}

	// Session key must be untouched: original key still decrypts.
	enc, err := sm.Encrypt("agent-1", plaintext)
	if err != nil {
		t.Fatalf("Encrypt after rejected rotation: %v", err)
	}
	dec, err := sm.Decrypt("agent-1", enc)
	if err != nil || !bytes.Equal(dec, plaintext) {
		t.Errorf("session key was clobbered by forged rotation: dec=%q err=%v", dec, err)
	}
}

func TestSessionManagerTryRotateSessionKeyMissing(t *testing.T) {
	sm, _ := NewSessionManager()
	curve := ecdh.X25519()
	agentKey, _ := curve.GenerateKey(rand.Reader)
	_, err := sm.TryRotateSessionKey("nonexistent", agentKey.PublicKey().Bytes(), []byte("x"))
	if err != ErrNoSession {
		t.Errorf("expected ErrNoSession, got %v", err)
	}
}

func TestSessionManagerEstablishRejectsActiveOverwrite(t *testing.T) {
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

	// Replaying a handshake (same or different key) must not overwrite it.
	otherKey, _ := curve.GenerateKey(rand.Reader)
	if err := sm.EstablishSession("agent-1", otherKey.PublicKey().Bytes()); err != ErrSessionActive {
		t.Errorf("expected ErrSessionActive for in-use session, got %v", err)
	}
	if err := sm.EstablishSession("agent-1", agentKey.PublicKey().Bytes()); err != ErrSessionActive {
		t.Errorf("expected ErrSessionActive for in-use session replay, got %v", err)
	}

	// The original session key must still decrypt.
	encrypted, err := sm.Encrypt("agent-1", plaintext)
	if err != nil {
		t.Fatalf("Encrypt after rejected overwrite: %v", err)
	}
	decrypted, err := sm.Decrypt("agent-1", encrypted)
	if err != nil || string(decrypted) != "data" {
		t.Errorf("session key was clobbered by rejected handshake: decrypted=%q err=%v", decrypted, err)
	}
}

func TestSessionManagerEstablishAllowsIdleOverwrite(t *testing.T) {
	sm, _ := NewSessionManagerWithConfig(100, 5*time.Minute)
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
	sm, _ := NewSessionManagerWithConfig(100, 5*time.Minute)
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

// encryptWithAgentPub derives a session key from the given agent public key
// using the server's private key and returns the AES-256-GCM ciphertext. It
// simulates an agent beacon encrypted under a NEW rotated session key.
func encryptWithAgentPub(sm *SessionManager, agentPubBytes, plaintext []byte) ([]byte, error) {
	curve := ecdh.X25519()
	agentPub, err := curve.NewPublicKey(agentPubBytes)
	if err != nil {
		return nil, err
	}
	sm.mu.Lock()
	shared, err := sm.privateKey.ECDH(agentPub)
	sm.mu.Unlock()
	if err != nil {
		return nil, err
	}
	key := sha256.Sum256(shared)
	return encryptAESGCM(key[:], plaintext)
}

// encryptAESGCM encrypts plaintext under the provided AES-256 key and returns
// raw nonce+ciphertext bytes (mirrors SessionManager.Encrypt's wire format).
func encryptAESGCM(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return aesGCM.Seal(nonce, nonce, plaintext, nil), nil
}
