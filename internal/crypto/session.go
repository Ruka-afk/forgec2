package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"math/big"
	mrand "math/rand"
	"sync"
	"time"
)

const (
	SessionMaxMessages = 100
	SessionMaxAge      = 10 * time.Minute
)

var (
	ErrNoSession      = errors.New("no session for agent")
	ErrCiphertextTooShort = errors.New("ciphertext too short")
	ErrDecryptFailed      = errors.New("decryption failed")
)

// SessionManager manages ECDH key exchange and session keys with PFS
type SessionManager struct {
	privateKey *ecdh.PrivateKey
	sessions   map[string]*Session
	mu         sync.RWMutex
}

// Session represents a single agent session with PFS
type Session struct {
	AgentID      string
	SessionKey   []byte
	CreatedAt    time.Time
	MessageCount int
	LastUsed     time.Time
}

// NewSessionManager creates a new session manager with ECDH key pair
func NewSessionManager() (*SessionManager, error) {
	curve := ecdh.X25519()
	privateKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}

	return &SessionManager{
		privateKey: privateKey,
		sessions:   make(map[string]*Session),
	}, nil
}

// GetPublicKey returns the server's public key for distribution to agents
func (sm *SessionManager) GetPublicKey() []byte {
	return sm.privateKey.PublicKey().Bytes()
}

// EstablishSession performs ECDH key exchange with an agent
func (sm *SessionManager) EstablishSession(agentID string, agentPublicKey []byte) error {
	curve := ecdh.X25519()

	agentPub, err := curve.NewPublicKey(agentPublicKey)
	if err != nil {
		return err
	}

	sharedSecret, err := sm.privateKey.ECDH(agentPub)
	if err != nil {
		return err
	}

	hash := sha256.Sum256(sharedSecret)
	sessionKey := hash[:]

	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.sessions[agentID] = &Session{
		AgentID:      agentID,
		SessionKey:   sessionKey,
		CreatedAt:    time.Now(),
		MessageCount: 0,
		LastUsed:     time.Now(),
	}

	return nil
}

// GetSession retrieves an agent's session
func (sm *SessionManager) GetSession(agentID string) *Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.sessions[agentID]
}

// NeedsRotation checks if a session needs key rotation
func (sm *SessionManager) NeedsRotation(agentID string) bool {
	session := sm.GetSession(agentID)
	if session == nil {
		return false
	}
	return session.MessageCount >= SessionMaxMessages || time.Since(session.CreatedAt) >= SessionMaxAge
}

// RotateSessionKey generates a new session key for forward secrecy
func (sm *SessionManager) RotateSessionKey(agentID string, agentPublicKey []byte) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[agentID]
	if !exists {
		return ErrNoSession
	}

	curve := ecdh.X25519()
	agentPub, err := curve.NewPublicKey(agentPublicKey)
	if err != nil {
		return err
	}

	sharedSecret, err := sm.privateKey.ECDH(agentPub)
	if err != nil {
		return err
	}

	hash := sha256.Sum256(sharedSecret)
	session.SessionKey = hash[:]
	session.CreatedAt = time.Now()
	session.MessageCount = 0

	return nil
}

// IncrementMessageCount tracks message count for rotation
func (sm *SessionManager) IncrementMessageCount(agentID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if session, exists := sm.sessions[agentID]; exists {
		session.MessageCount++
		session.LastUsed = time.Now()
	}
}

// Encrypt encrypts data using AES-256-GCM with the session key, returns raw bytes (nonce+ciphertext)
func (sm *SessionManager) Encrypt(agentID string, plaintext []byte) ([]byte, error) {
	session := sm.GetSession(agentID)
	if session == nil {
		return nil, ErrNoSession
	}

	block, err := aes.NewCipher(session.SessionKey)
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

	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)

	sm.IncrementMessageCount(agentID)

	return ciphertext, nil
}

// EncryptB64 encrypts and returns base64-encoded ciphertext
func (sm *SessionManager) EncryptB64(agentID string, plaintext []byte) (string, error) {
	raw, err := sm.Encrypt(agentID, plaintext)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}

// Decrypt decrypts data using AES-256-GCM with the session key
// Input is raw bytes: nonce(12) + ciphertext
func (sm *SessionManager) Decrypt(agentID string, ciphertext []byte) ([]byte, error) {
	session := sm.GetSession(agentID)
	if session == nil {
		return nil, ErrNoSession
	}

	block, err := aes.NewCipher(session.SessionKey)
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, ErrCiphertextTooShort
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrDecryptFailed
	}

	sm.IncrementMessageCount(agentID)

	return plaintext, nil
}

// DecryptB64 decrypts base64-encoded ciphertext
func (sm *SessionManager) DecryptB64(agentID string, encodedCiphertext string) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encodedCiphertext)
	if err != nil {
		return nil, err
	}
	return sm.Decrypt(agentID, ciphertext)
}

// CleanupExpiredSessions removes old sessions
func (sm *SessionManager) CleanupExpiredSessions(maxAge time.Duration) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for agentID, session := range sm.sessions {
		if time.Since(session.LastUsed) > maxAge {
			delete(sm.sessions, agentID)
		}
	}
}

// TrafficObfuscator adds randomness to traffic patterns
type TrafficObfuscator struct {
	JitterPercent float64
	PaddingSize   int
}

func NewTrafficObfuscator(jitterPercent float64, paddingSize int) *TrafficObfuscator {
	return &TrafficObfuscator{
		JitterPercent: jitterPercent,
		PaddingSize:   paddingSize,
	}
}

func (to *TrafficObfuscator) AddJitter(baseDelay time.Duration) time.Duration {
	if to.JitterPercent == 0 {
		return baseDelay
	}
	jitter := float64(baseDelay) * to.JitterPercent
	randInt, _ := rand.Int(rand.Reader, big.NewInt(int64(jitter*2)))
	randomJitter := float64(randInt.Int64()) - jitter
	return baseDelay + time.Duration(randomJitter)
}

func (to *TrafficObfuscator) AddPadding(data []byte) []byte {
	if to.PaddingSize == 0 {
		return data
	}
	padding := make([]byte, mrand.Intn(to.PaddingSize))
	return append(data, padding...)
}

func (to *TrafficObfuscator) RemovePadding(data []byte, originalSize int) []byte {
	if originalSize > len(data) {
		return data
	}
	return data[:originalSize]
}

// DomainFrontingManager manages multiple CDN domains for fronting
type DomainFrontingManager struct {
	domains      []string
	currentIndex int
	mu           sync.Mutex
}

func NewDomainFrontingManager(domains []string) *DomainFrontingManager {
	return &DomainFrontingManager{
		domains:      domains,
		currentIndex: 0,
	}
}

func (dfm *DomainFrontingManager) GetNextDomain() string {
	dfm.mu.Lock()
	defer dfm.mu.Unlock()
	if len(dfm.domains) == 0 {
		return ""
	}
	domain := dfm.domains[dfm.currentIndex]
	dfm.currentIndex = (dfm.currentIndex + 1) % len(dfm.domains)
	return domain
}

func (dfm *DomainFrontingManager) Failover() string {
	return dfm.GetNextDomain()
}

func (dfm *DomainFrontingManager) GetActiveDomains() []string {
	dfm.mu.Lock()
	defer dfm.mu.Unlock()
	result := make([]string, len(dfm.domains))
	copy(result, dfm.domains)
	return result
}
