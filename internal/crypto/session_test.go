package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"testing"
	"time"
)

func TestSessionManagerEstablishAndGet(t *testing.T) {
	sm, err := NewSessionManager()
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	agentPub := generateAgentPublicKey(t)
	err = sm.EstablishSession("agent-1", agentPub)
	if err != nil {
		t.Fatalf("EstablishSession() error = %v", err)
	}

	session := sm.GetSession("agent-1")
	if session == nil {
		t.Fatal("GetSession() returned nil")
	}
	if session.AgentID != "agent-1" {
		t.Fatalf("AgentID = %q, want %q", session.AgentID, "agent-1")
	}
	if len(session.SessionKey) != 32 {
		t.Fatalf("SessionKey length = %d, want 32", len(session.SessionKey))
	}
}

func TestSessionManagerGetPublicKey(t *testing.T) {
	sm, err := NewSessionManager()
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}
	pub := sm.GetPublicKey()
	if len(pub) == 0 {
		t.Fatal("GetPublicKey() returned empty")
	}
}

func TestSessionManagerEncryptDecrypt(t *testing.T) {
	sm, err := NewSessionManager()
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	agentPub := generateAgentPublicKey(t)
	sm.EstablishSession("agent-1", agentPub)

	plaintext := []byte("secret mission data")
	encoded, err := sm.Encrypt("agent-1", plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	decrypted, err := sm.Decrypt("agent-1", encoded)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Fatalf("round trip: got %q, want %q", string(decrypted), string(plaintext))
	}
}

func TestSessionManagerEncryptNoSession(t *testing.T) {
	sm, err := NewSessionManager()
	if err != nil {
		t.Fatal(err)
	}
	_, err = sm.Encrypt("nonexistent", []byte("data"))
	if err == nil {
		t.Fatal("expected error for nonexistent agent")
	}
}

func TestSessionManagerIncrementMessageCount(t *testing.T) {
	sm, err := NewSessionManager()
	if err != nil {
		t.Fatal(err)
	}

	agentPub := generateAgentPublicKey(t)
	sm.EstablishSession("agent-1", agentPub)

	for i := 0; i < 5; i++ {
		sm.IncrementMessageCount("agent-1")
	}

	session := sm.GetSession("agent-1")
	if session.MessageCount != 5 {
		t.Fatalf("MessageCount = %d, want 5", session.MessageCount)
	}
}

func TestSessionManagerCleanupExpired(t *testing.T) {
	sm, err := NewSessionManager()
	if err != nil {
		t.Fatal(err)
	}

	agentPub := generateAgentPublicKey(t)
	sm.EstablishSession("agent-1", agentPub)

	// Force session to appear expired by adding an old one directly
	sm.mu.Lock()
	sm.sessions["agent-expired"] = &Session{
		AgentID:    "agent-expired",
		SessionKey: []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		LastUsed:   time.Now().Add(-2 * time.Hour),
	}
	sm.mu.Unlock()

	sm.CleanupExpiredSessions(1 * time.Hour)

	if sm.GetSession("agent-expired") != nil {
		t.Fatal("expired session should have been removed")
	}
	if sm.GetSession("agent-1") == nil {
		t.Fatal("active session should still exist")
	}
}

func TestTrafficObfuscator(t *testing.T) {
	t.Run("add and remove padding", func(t *testing.T) {
		to := NewTrafficObfuscator(0, 100)
		data := []byte("hello")
		padded := to.AddPadding(data)
		if len(padded) < len(data) {
			t.Fatalf("padded len %d < original %d", len(padded), len(data))
		}
		restored := to.RemovePadding(padded, len(data))
		if string(restored) != string(data) {
			t.Fatalf("remove padding: got %q, want %q", string(restored), string(data))
		}
	})

	t.Run("zero padding size", func(t *testing.T) {
		to := NewTrafficObfuscator(0, 0)
		data := []byte("hello")
		result := to.AddPadding(data)
		if len(result) != len(data) {
			t.Fatalf("expected no padding, got len %d", len(result))
		}
	})

	t.Run("remove padding with size larger than data", func(t *testing.T) {
		to := NewTrafficObfuscator(0, 100)
		data := []byte("short")
		result := to.RemovePadding(data, 100)
		if string(result) != string(data) {
			t.Fatalf("expected original data when originalSize > length")
		}
	})

	t.Run("add jitter with zero percent", func(t *testing.T) {
		to := NewTrafficObfuscator(0, 0)
		base := 10 * time.Second
		result := to.AddJitter(base)
		if result != base {
			t.Fatalf("expected no jitter, got %v", result)
		}
	})
}

func TestDomainFrontingManager(t *testing.T) {
	t.Run("round robin", func(t *testing.T) {
		domains := []string{"a.com", "b.com", "c.com"}
		dfm := NewDomainFrontingManager(domains)
		for i := 0; i < 6; i++ {
			got := dfm.GetNextDomain()
			expected := domains[i%3]
			if got != expected {
				t.Fatalf("iteration %d: got %q, want %q", i, got, expected)
			}
		}
	})

	t.Run("failover alias", func(t *testing.T) {
		dfm := NewDomainFrontingManager([]string{"x.com", "y.com"})
		first := dfm.GetNextDomain()
		second := dfm.Failover()
		if first == second {
			t.Fatal("Failover should return next domain")
		}
	})

	t.Run("empty domains", func(t *testing.T) {
		dfm := NewDomainFrontingManager(nil)
		if dfm.GetNextDomain() != "" {
			t.Fatal("expected empty string for no domains")
		}
	})

	t.Run("get active domains returns copy", func(t *testing.T) {
		domains := []string{"a.com", "b.com"}
		dfm := NewDomainFrontingManager(domains)
		got := dfm.GetActiveDomains()
		if len(got) != 2 || got[0] != "a.com" || got[1] != "b.com" {
			t.Fatalf("got %v, want [a.com b.com]", got)
		}
		got[0] = "modified"
		if dfm.GetActiveDomains()[0] != "a.com" {
			t.Fatal("GetActiveDomains should return a copy")
		}
	})
}

func TestEncryptDecryptRaw(t *testing.T) {
	sm, err := NewSessionManager()
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	agentPub := generateAgentPublicKey(t)
	sm.EstablishSession("agent-1", agentPub)

	plaintext := []byte("raw binary data \x00\x01\x02")
	encrypted, err := sm.Encrypt("agent-1", plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	decrypted, err := sm.Decrypt("agent-1", encrypted)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Fatalf("round trip: got %q, want %q", string(decrypted), string(plaintext))
	}
}

func TestEncryptDecryptB64(t *testing.T) {
	sm, err := NewSessionManager()
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	agentPub := generateAgentPublicKey(t)
	sm.EstablishSession("agent-1", agentPub)

	plaintext := []byte("base64 encoded test")
	encoded, err := sm.EncryptB64("agent-1", plaintext)
	if err != nil {
		t.Fatalf("EncryptB64() error = %v", err)
	}

	decrypted, err := sm.DecryptB64("agent-1", encoded)
	if err != nil {
		t.Fatalf("DecryptB64() error = %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Fatalf("round trip: got %q, want %q", string(decrypted), string(plaintext))
	}
}

func TestNeedsRotation(t *testing.T) {
	sm, err := NewSessionManager()
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	agentPub := generateAgentPublicKey(t)
	sm.EstablishSession("agent-1", agentPub)

	if sm.NeedsRotation("agent-1") {
		t.Fatal("fresh session should not need rotation")
	}

	// Hit message count threshold
	for i := 0; i < SessionMaxMessages; i++ {
		sm.IncrementMessageCount("agent-1")
	}

	if !sm.NeedsRotation("agent-1") {
		t.Fatal("session should need rotation after 100 messages")
	}
}

func TestRotateSessionKey(t *testing.T) {
	sm, err := NewSessionManager()
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	agentPub := generateAgentPublicKey(t)
	sm.EstablishSession("agent-1", agentPub)

	originalKey := make([]byte, 32)
	session := sm.GetSession("agent-1")
	copy(originalKey, session.SessionKey)

	// Rotate with a new agent key
	newAgentPub := generateAgentPublicKey(t)
	if err := sm.RotateSessionKey("agent-1", newAgentPub); err != nil {
		t.Fatalf("RotateSessionKey() error = %v", err)
	}

	session = sm.GetSession("agent-1")
	if string(session.SessionKey) == string(originalKey) {
		t.Fatal("session key should have changed after rotation")
	}
	if session.MessageCount != 0 {
		t.Fatal("message count should reset after rotation")
	}
}

func TestDecryptWrongAgent(t *testing.T) {
	sm, err := NewSessionManager()
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	agentPub1 := generateAgentPublicKey(t)
	sm.EstablishSession("agent-1", agentPub1)

	encrypted, _ := sm.Encrypt("agent-1", []byte("secret"))

	// Try decrypting with a different agent that has a different key pair
	agentPub2 := generateAgentPublicKey(t)
	sm.EstablishSession("agent-2", agentPub2)
	_, err = sm.Decrypt("agent-2", encrypted)
	if err == nil {
		t.Fatal("expected error when decrypting with wrong agent")
	}
}

func TestDecryptShortCiphertext(t *testing.T) {
	sm, err := NewSessionManager()
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	agentPub := generateAgentPublicKey(t)
	sm.EstablishSession("agent-1", agentPub)

	_, err = sm.Decrypt("agent-1", []byte("short"))
	if err == nil {
		t.Fatal("expected error for short ciphertext")
	}
}

func TestDecryptTampered(t *testing.T) {
	sm, err := NewSessionManager()
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}

	agentPub := generateAgentPublicKey(t)
	sm.EstablishSession("agent-1", agentPub)

	encrypted, _ := sm.Encrypt("agent-1", []byte("secret data"))

	// Tamper with the ciphertext
	encrypted[len(encrypted)-1] ^= 0xFF
	_, err = sm.Decrypt("agent-1", encrypted)
	if err == nil {
		t.Fatal("expected error for tampered ciphertext")
	}
}

func TestECDHFullHandshakeFlow(t *testing.T) {
	// Simulates full agent-server ECDH handshake + encrypted communication

	// Server side
	sm, err := NewSessionManager()
	if err != nil {
		t.Fatalf("NewSessionManager() error = %v", err)
	}
	serverPub := sm.GetPublicKey()

	// Agent side: generate ECDH key pair (simulate newECDSession)
	curve := ecdh.X25519()
	agentKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("agent key generation failed: %v", err)
	}
	agentPub := agentKey.PublicKey().Bytes()

	// Step 1: Agent sends agentPub to Server
	err = sm.EstablishSession("agent-1", agentPub)
	if err != nil {
		t.Fatalf("EstablishSession() error = %v", err)
	}

	// Step 2: Server responds with serverPub
	// Agent computes shared secret
	serverPubKey, err := curve.NewPublicKey(serverPub)
	if err != nil {
		t.Fatalf("server pub key parse failed: %v", err)
	}
	agentShared, err := agentKey.ECDH(serverPubKey)
	if err != nil {
		t.Fatalf("agent ECDH failed: %v", err)
	}

	// Step 3: Both sides derive session key
	// Server side (already done in EstablishSession)
	serverSession := sm.GetSession("agent-1")

	// Agent side (simulate establishFromServerKey)
	agentHash := sha256.Sum256(agentShared)

	if string(serverSession.SessionKey) != string(agentHash[:]) {
		t.Fatal("session keys don't match after handshake")
	}

	// Step 4: Agent encrypts a message
	agentMsg := []byte(`{"uuid":"agent-1","info":{"hostname":"test"}}`)
	block, _ := aes.NewCipher(agentHash[:])
	aesGCM, _ := cipher.NewGCM(block)
	nonce := make([]byte, aesGCM.NonceSize())
	rand.Read(nonce)
	agentCiphertext := aesGCM.Seal(nonce, nonce, agentMsg, nil)
	agentEncoded := base64.StdEncoding.EncodeToString(agentCiphertext)

	// Step 5: Server decrypts
	serverDecrypted, err := sm.DecryptB64("agent-1", agentEncoded)
	if err != nil {
		t.Fatalf("server decrypt failed: %v", err)
	}
	if string(serverDecrypted) != string(agentMsg) {
		t.Fatal("decrypted message mismatch")
	}

	// Step 6: Server encrypts response
	serverResp := []byte(`{"tasks":[]}`)
	serverEncoded, err := sm.EncryptB64("agent-1", serverResp)
	if err != nil {
		t.Fatalf("server encrypt failed: %v", err)
	}

	// Step 7: Agent decrypts response
	serverCiphertext, _ := base64.StdEncoding.DecodeString(serverEncoded)
	agentDecrypted, err := aesGCM.Open(nil, serverCiphertext[:12], serverCiphertext[12:], nil)
	if err != nil {
		t.Fatalf("agent decrypt failed: %v", err)
	}
	if string(agentDecrypted) != string(serverResp) {
		t.Fatal("response message mismatch")
	}

	// Step 8: Verify message count tracking
	if serverSession.MessageCount != 2 {
		t.Fatalf("expected 2 messages, got %d", serverSession.MessageCount)
	}
}

func generateAgentPublicKey(t *testing.T) []byte {
	t.Helper()
	curve := ecdh.X25519()
	key, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate agent key: %v", err)
	}
	return key.PublicKey().Bytes()
}
