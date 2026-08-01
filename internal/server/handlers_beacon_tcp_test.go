package server

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/forgec2/forgec2/pkg/encoding"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

// tcpTestAgent mirrors the agent-side ECDH session (internal/payload/agent/cipher.go)
// so the test can speak the real wire protocol end-to-end.
type tcpTestAgent struct {
	privateKey *ecdh.PrivateKey
	sessionKey []byte
	uuid       string
}

func newTCPTestAgent(t *testing.T, uuid string) *tcpTestAgent {
	t.Helper()
	curve := ecdh.X25519()
	privateKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate agent key: %v", err)
	}
	return &tcpTestAgent{privateKey: privateKey, uuid: uuid}
}

func (a *tcpTestAgent) publicKeyB64() string {
	return base64.StdEncoding.EncodeToString(a.privateKey.PublicKey().Bytes())
}

func (a *tcpTestAgent) establishFromServerKey(serverPubB64 string) error {
	curve := ecdh.X25519()
	serverPub, err := curve.NewPublicKey(decodeStdB64(serverPubB64))
	if err != nil {
		return err
	}
	sharedSecret, err := a.privateKey.ECDH(serverPub)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(sharedSecret)
	a.sessionKey = hash[:]
	return nil
}

func (a *tcpTestAgent) encrypt(plaintext []byte) (string, error) {
	block, err := aes.NewCipher(a.sessionKey)
	if err != nil {
		return "", err
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(aesGCM.Seal(nonce, nonce, plaintext, nil)), nil
}

func (a *tcpTestAgent) decrypt(encoded string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(a.sessionKey)
	if err != nil {
		return nil, err
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := aesGCM.NonceSize()
	if len(data) < nonceSize {
		return nil, io.ErrUnexpectedEOF
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return aesGCM.Open(nil, nonce, ciphertext, nil)
}

func decodeStdB64(s string) []byte {
	data, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil
	}
	return data
}

// initTCPBeaconServer builds a server whose sessionManager is enabled (ECDH capable).
func initTCPBeaconServer(t *testing.T, database *gorm.DB) *Server {
	t.Helper()
	sm, err := crypto.NewSessionManager()
	if err != nil {
		t.Fatalf("new session manager: %v", err)
	}
	s := &Server{
		db:                    database,
		cfg:                   &config.Config{},
		sessionManager:        sm,
		beaconDedupCache:      make(map[string]time.Time),
		eventManager:          NewEventManager(database),
		socksEngine:           newSocksRelayEngine(),
		agentStatusCooldown:   make(map[string]time.Time),
		wsClients:             make(map[*websocket.Conn]*wsClientConn),
		agentPendingTasks:     make(map[string]int),
		screenMonitorImplants: make(map[string]time.Time),
	}
	return s
}

// tcpFrameConn runs handleTCPConnection over net.Pipe and returns the client side.
func tcpFrameConn(t *testing.T, s *Server) (net.Conn, func()) {
	t.Helper()
	serverSide, clientSide := net.Pipe()
	s.wg.Add(1)
	go s.handleTCPConnection(serverSide)
	return clientSide, func() { clientSide.Close() }
}

func tcpWriteFrame(t *testing.T, conn net.Conn, payload []byte) {
	t.Helper()
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := binary.Write(conn, binary.BigEndian, uint32(len(payload))); err != nil {
		t.Fatalf("write length: %v", err)
	}
	if _, err := conn.Write(payload); err != nil {
		t.Fatalf("write payload: %v", err)
	}
}

func tcpReadFrame(t *testing.T, conn net.Conn) []byte {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var msgLen uint32
	if err := binary.Read(conn, binary.BigEndian, &msgLen); err != nil {
		t.Fatalf("read length: %v", err)
	}
	if msgLen == 0 || msgLen > 16*1024*1024 {
		t.Fatalf("unexpected frame length %d", msgLen)
	}
	buf := make([]byte, msgLen)
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("read payload: %v", err)
	}
	return buf
}

// TestTCPBeaconPlaintext verifies a plaintext envelope is processed and answered
// with a plaintext beaconResponse (regression: TCP never decrypted envelopes).
func TestTCPBeaconPlaintext(t *testing.T) {
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	s := initTCPBeaconServer(t, database)
	conn, done := tcpFrameConn(t, s)
	defer done()

	agentUUID := "77777777-8888-4333-8444-999999999999"
	body := `{"uuid":"` + agentUUID + `","info":{"hostname":"TCP-PLAIN","username":"u","ip":"10.0.0.9"},"pv":1}`
	tcpWriteFrame(t, conn, []byte(body))

	respFrame := tcpReadFrame(t, conn)
	var resp beaconResponse
	if err := encoding.Unmarshal(respFrame, &resp); err != nil {
		t.Fatalf("plaintext response should be a beaconResponse, got %q: %v", respFrame, err)
	}

	var implant db.Implant
	if err := database.Where("id = ?", agentUUID).First(&implant).Error; err != nil {
		t.Fatalf("agent not registered: %v", err)
	}
	if !implant.LastSeen.IsZero() && time.Since(implant.LastSeen) > 10*time.Second {
		t.Error("last_seen should be recent")
	}
}

// TestTCPBeaconECDHFlow verifies the full ECDH handshake + encrypted exchange over
// the raw TCP transport, matching the HTTP beacon wire protocol (regression: TCP
// previously returned plaintext to an ECDH-expecting agent, so tasks never ran).
func TestTCPBeaconECDHFlow(t *testing.T) {
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	s := initTCPBeaconServer(t, database)
	conn, done := tcpFrameConn(t, s)
	defer done()

	agent := newTCPTestAgent(t, "aaaaaaaa-bbbb-4333-8444-cccccccccccc")

	// Step 1: ECDH handshake envelope
	handshake := `{"uuid":"` + agent.uuid + `","ecdh_pub":"` + agent.publicKeyB64() + `","info":{"hostname":"TCP-ECDH","username":"u","ip":"10.0.0.8"},"pv":1}`
	tcpWriteFrame(t, conn, []byte(handshake))

	respFrame := tcpReadFrame(t, conn)
	var handshakeResp struct {
		ECDHPub string `json:"ecdh_pub"`
	}
	if err := encoding.Unmarshal(respFrame, &handshakeResp); err != nil {
		t.Fatalf("handshake response parse failed: %v (body=%s)", err, respFrame)
	}
	if handshakeResp.ECDHPub == "" {
		t.Fatalf("handshake response must carry ecdh_pub, got %s", respFrame)
	}
	if err := agent.establishFromServerKey(handshakeResp.ECDHPub); err != nil {
		t.Fatalf("agent establish session: %v", err)
	}

	// Step 2: encrypted beacon (inner request is the same payload as the handshake)
	encryptedC, err := agent.encrypt([]byte(`{"uuid":"` + agent.uuid + `","info":{"hostname":"TCP-ECDH","username":"u","ip":"10.0.0.8"},"pv":1}`))
	if err != nil {
		t.Fatalf("agent encrypt: %v", err)
	}
	encryptedBody := `{"uuid":"` + agent.uuid + `","c":"` + encryptedC + `"}`
	tcpWriteFrame(t, conn, []byte(encryptedBody))

	respFrame = tcpReadFrame(t, conn)
	var encResp struct {
		ECDHPub   string `json:"ecdh_pub"`
		CipherB64 string `json:"c"`
	}
	if err := encoding.Unmarshal(respFrame, &encResp); err != nil {
		t.Fatalf("encrypted response parse failed: %v (body=%s)", err, respFrame)
	}
	if encResp.CipherB64 == "" {
		t.Fatalf("encrypted beacon response must carry c field, got %s", respFrame)
	}
	plaintext, err := agent.decrypt(encResp.CipherB64)
	if err != nil {
		t.Fatalf("agent decrypt response: %v", err)
	}
	var inner beaconResponse
	if err := encoding.Unmarshal(plaintext, &inner); err != nil {
		t.Fatalf("decrypted response is not a beaconResponse: %v", err)
	}

	var implant db.Implant
	if err := database.Where("id = ?", agent.uuid).First(&implant).Error; err != nil {
		t.Fatalf("agent not registered: %v", err)
	}
}

// TestTCPBeaconRejectsInvalidAgentID verifies the shared envelope decoder rejects
// traversal-style agent IDs over TCP (same guard as HTTP).
func TestTCPBeaconRejectsInvalidAgentID(t *testing.T) {
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	s := initTCPBeaconServer(t, database)
	conn, done := tcpFrameConn(t, s)
	defer done()

	body := `{"uuid":"../../etc/passwd","info":{"hostname":"EVIL","username":"u","ip":"10.0.0.7"},"pv":1}`
	tcpWriteFrame(t, conn, []byte(body))

	// Server closes the connection without responding.
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var msgLen uint32
	if err := binary.Read(conn, binary.BigEndian, &msgLen); err == nil {
		t.Fatalf("expected connection close, got frame length %d", msgLen)
	}
}

// TestTCPBeaconBadKeyClosesConn verifies an unknown beacon key terminates the TCP session.
func TestTCPBeaconBadKeyClosesConn(t *testing.T) {
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	s := initTCPBeaconServer(t, database)
	s.configMu.Lock()
	s.cfg.Server.BeaconKey = "supersecretkey"
	s.configMu.Unlock()

	conn, done := tcpFrameConn(t, s)
	defer done()

	body := `{"uuid":"bbbbbbbb-cccc-4333-8444-dddddddddddd","key":"wrong","info":{"hostname":"EVIL","username":"u","ip":"10.0.0.6"},"pv":1}`
	tcpWriteFrame(t, conn, []byte(body))

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var msgLen uint32
	if err := binary.Read(conn, binary.BigEndian, &msgLen); err == nil {
		t.Fatalf("expected connection close for bad key, got frame length %d", msgLen)
	}
}

func ginSetTestMode(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
}
