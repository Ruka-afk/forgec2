package server

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"strconv"
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

// tcpTestAgent mirrors the agent-side v2 ECDH session + envelope builder
// (internal/payload/agent/cipher.go) so the test can speak the real wire
// protocol end-to-end.
type tcpTestAgent struct {
	privateKey *ecdh.PrivateKey
	sessionKey []byte
	uuid       string
	seq        uint64
	regKey     []byte // per-agent registration key ("" master => nil)
	secretID   string // v3 per-implant secret id (carried on registration frames)
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

// withRegKey derives the per-agent registration key from a master beacon key
// hex string, mirroring the server's DeriveRegistrationKeyFromHex.
func (a *tcpTestAgent) withRegKey(masterHex string) *tcpTestAgent {
	a.regKey = crypto.DeriveRegistrationKeyFromHex(masterHex, a.uuid)
	return a
}

// withRawRegKey sets the registration key directly (v3: the per-implant
// secret embedded in the binary, no master-key derivation).
func (a *tcpTestAgent) withRawRegKey(key []byte) *tcpTestAgent {
	a.regKey = key
	return a
}

// withSecretID sets the v3 per-implant secret id carried on registration.
func (a *tcpTestAgent) withSecretID(id string) *tcpTestAgent {
	a.secretID = id
	return a
}

func (a *tcpTestAgent) publicKeyB64() string {
	return base64.StdEncoding.EncodeToString(a.privateKey.PublicKey().Bytes())
}

// establishFromServerKey completes the ECDH handshake with the server public
// key, deriving the session key via HKDF bound to the agent ID (v2).
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
	a.sessionKey = crypto.DeriveSessionKey(sharedSecret, a.uuid)
	return nil
}

func (a *tcpTestAgent) encryptWithAAD(plaintext []byte, aad []byte) (string, error) {
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
	return base64.StdEncoding.EncodeToString(aesGCM.Seal(nonce, nonce, plaintext, aad)), nil
}

func (a *tcpTestAgent) decryptWithAAD(encoded string, aad []byte) ([]byte, error) {
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
	return aesGCM.Open(nil, nonce, ciphertext, aad)
}

// nextSeq allocates the next frame sequence (monotonic).
func (a *tcpTestAgent) nextSeq() uint64 {
	a.seq++
	return a.seq
}

// aad returns the v2 AAD for a frame: uuid + "\x00" + seq.
func (a *tcpTestAgent) aad(seq uint64) []byte {
	return []byte(a.uuid + "\x00" + strconv.FormatUint(seq, 10))
}

// registerFrame builds a v2 registration envelope (identity key bound).
func (a *tcpTestAgent) registerFrame() string {
	seq := a.nextSeq()
	ts := time.Now().Unix()
	idPub := a.publicKeyB64()
	env := map[string]interface{}{
		"uuid":     a.uuid,
		"seq":      seq,
		"ts":       ts,
		"ecdh_pub": idPub,
		"id_pub":   idPub,
	}
	if a.regKey != nil {
		env["reg_hmac"] = base64.StdEncoding.EncodeToString(crypto.ComputeRegHMAC(a.regKey, a.uuid, idPub, ts, seq))
	}
	if a.secretID != "" {
		env["secret_id"] = a.secretID
	}
	b, _ := json.Marshal(env)
	return string(b)
}

// handshakeFrame builds an authenticated v2 handshake envelope with a fresh
// ephemeral key (server restart recovery / rekey).
func (a *tcpTestAgent) handshakeFrame() string {
	seq := a.nextSeq()
	ts := time.Now().Unix()
	pub := a.publicKeyB64()
	env := map[string]interface{}{
		"uuid":     a.uuid,
		"seq":      seq,
		"ts":       ts,
		"ecdh_pub": pub,
	}
	if a.secretID != "" {
		// v3: the per-implant secret id is carried on handshake frames too, so
		// the server can authenticate the handshake against the secret store
		// even after the implant row is deleted.
		env["secret_id"] = a.secretID
	}
	if a.regKey != nil {
		mac := hmac.New(sha256.New, a.regKey)
		mac.Write([]byte(a.uuid))
		mac.Write([]byte(pub))
		mac.Write([]byte(strconv.FormatInt(ts, 10)))
		mac.Write([]byte(strconv.FormatUint(seq, 10)))
		env["mac"] = base64.StdEncoding.EncodeToString(mac.Sum(nil))
	}
	b, _ := json.Marshal(env)
	return string(b)
}

// encryptedFrame builds a v2 ciphertext envelope: AES-256-GCM with AAD
// uuid||seq over the inner plaintext beacon request.
func (a *tcpTestAgent) encryptedFrame(inner []byte) string {
	seq := a.nextSeq()
	ts := time.Now().Unix()
	cipherB64, err := a.encryptWithAAD(inner, a.aad(seq))
	if err != nil {
		return ""
	}
	b, _ := json.Marshal(map[string]interface{}{
		"uuid": a.uuid,
		"seq":  seq,
		"ts":   ts,
		"c":    cipherB64,
	})
	return string(b)
}

// verifyResponseMAC validates the server's auth response MAC:
// HMAC(regKey, uuid || seq || server_pub).
func (a *tcpTestAgent) verifyResponseMAC(seq uint64, serverPubB64, macB64 string) bool {
	if a.regKey == nil || macB64 == "" || serverPubB64 == "" {
		return false
	}
	mac := hmac.New(sha256.New, a.regKey)
	mac.Write([]byte(a.uuid))
	mac.Write([]byte(strconv.FormatUint(seq, 10)))
	mac.Write([]byte(serverPubB64))
	expected := mac.Sum(nil)
	got, err := base64.StdEncoding.DecodeString(macB64)
	if err != nil {
		return false
	}
	return hmac.Equal(expected, got)
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
		regSecrets:            crypto.NewRegSecretStore(make([]byte, 32)),
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

// v3TestAgent builds a test agent that carries a freshly minted per-implant
// v3 secret (the v2 master-key path is deprecated and rejected).
func v3TestAgent(t *testing.T, s *Server, uuid string) *tcpTestAgent {
	t.Helper()
	id, secretB64, err := s.createRegSecret()
	if err != nil {
		t.Fatalf("createRegSecret: %v", err)
	}
	secret, err := base64.StdEncoding.DecodeString(secretB64)
	if err != nil || len(secret) != 32 {
		t.Fatalf("v3 secret invalid: err=%v len=%d", err, len(secret))
	}
	return newTCPTestAgent(t, uuid).withRawRegKey(secret).withSecretID(id)
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

// TestTCPBeaconRejectsPlaintext verifies plaintext v1 frames are rejected over
// TCP (v2 has no plaintext frames; the connection is closed without a response).
func TestTCPBeaconRejectsPlaintext(t *testing.T) {
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	s := initTCPBeaconServer(t, database)
	conn, done := tcpFrameConn(t, s)
	defer done()

	agentUUID := "77777777-8888-4333-8444-999999999999"
	body := `{"uuid":"` + agentUUID + `","info":{"hostname":"TCP-PLAIN","username":"u","ip":"10.0.0.9"},"pv":1}`
	tcpWriteFrame(t, conn, []byte(body))

	// Server closes the connection without responding.
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var msgLen uint32
	if err := binary.Read(conn, binary.BigEndian, &msgLen); err == nil {
		t.Fatalf("expected connection close for plaintext frame, got frame length %d", msgLen)
	}

	var count int64
	database.Model(&db.Implant{}).Where("id = ?", agentUUID).Count(&count)
	if count != 0 {
		t.Fatalf("plaintext frame must not register an implant, count=%d", count)
	}
}

// TestTCPBeaconV2RegisterAndEncrypted verifies the full v2 registration +
// encrypted exchange over the raw TCP transport, matching the HTTP beacon wire
// protocol.
func TestTCPBeaconV3RegisterAndEncrypted(t *testing.T) {
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	s := initTCPBeaconServer(t, database)
	s.configMu.Lock()
	s.cfg.Server.BeaconKey = "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
	s.configMu.Unlock()
	conn, done := tcpFrameConn(t, s)
	defer done()

	// v3 per-implant secret (master-key path is deprecated).
	secretID, secretB64, err := s.createRegSecret()
	if err != nil {
		t.Fatalf("createRegSecret: %v", err)
	}
	secret, err := base64.StdEncoding.DecodeString(secretB64)
	if err != nil || len(secret) != 32 {
		t.Fatalf("v3 secret invalid: err=%v len=%d", err, len(secret))
	}
	agent := newTCPTestAgent(t, "aaaaaaaa-bbbb-4333-8444-cccccccccccc").
		withRawRegKey(secret).
		withSecretID(secretID)

	// Step 1: registration envelope
	tcpWriteFrame(t, conn, []byte(agent.registerFrame()))

	respFrame := tcpReadFrame(t, conn)
	var regResp struct {
		Seq     uint64 `json:"seq"`
		RegOK   bool   `json:"reg_ok"`
		ECDHPub string `json:"ecdh_pub"`
		Mac     string `json:"mac"`
	}
	if err := encoding.Unmarshal(respFrame, &regResp); err != nil {
		t.Fatalf("register response parse failed: %v (body=%s)", err, respFrame)
	}
	if !regResp.RegOK {
		t.Fatalf("registration must succeed, got %s", respFrame)
	}
	if !agent.verifyResponseMAC(regResp.Seq, regResp.ECDHPub, regResp.Mac) {
		t.Fatalf("register response MAC mismatch: %s", respFrame)
	}
	if err := agent.establishFromServerKey(regResp.ECDHPub); err != nil {
		t.Fatalf("agent establish session: %v", err)
	}

	// The implant row must be bound with the identity key.
	var implant db.Implant
	if err := database.Where("id = ?", agent.uuid).First(&implant).Error; err != nil {
		t.Fatalf("agent not registered: %v", err)
	}
	if !implant.Registered || implant.IdentityPub != agent.publicKeyB64() {
		t.Fatalf("implant identity not bound: registered=%v identity_pub=%q", implant.Registered, implant.IdentityPub)
	}

	// Step 2: encrypted beacon (inner request carries host info)
	inner, _ := json.Marshal(map[string]interface{}{
		"uuid": agent.uuid,
		"pv":   2,
		"info": map[string]string{"hostname": "TCP-ECDH", "username": "u", "ip": "10.0.0.8"},
	})
	encryptedBody := agent.encryptedFrame(inner)
	if encryptedBody == "" {
		t.Fatalf("agent encrypt failed")
	}
	tcpWriteFrame(t, conn, []byte(encryptedBody))

	respFrame = tcpReadFrame(t, conn)
	var encResp struct {
		CipherB64 string `json:"c"`
	}
	if err := encoding.Unmarshal(respFrame, &encResp); err != nil {
		t.Fatalf("encrypted response parse failed: %v (body=%s)", err, respFrame)
	}
	if encResp.CipherB64 == "" {
		t.Fatalf("encrypted beacon response must carry c field, got %s", respFrame)
	}
	// The response is encrypted with AAD uuid||(the seq used for the request).
	plaintext, err := agent.decryptWithAAD(encResp.CipherB64, agent.aad(agent.seq))
	if err != nil {
		t.Fatalf("agent decrypt response: %v", err)
	}
	var innerResp beaconResponse
	if err := encoding.Unmarshal(plaintext, &innerResp); err != nil {
		t.Fatalf("decrypted response is not a beaconResponse: %v", err)
	}
}

// TestTCPBeaconRestartRecovery verifies an agent re-handshakes after the server
// loses the in-memory session (restart): the encrypted frame is rejected, the
// authenticated handshake re-establishes the session, and the next encrypted
// frame is processed.
func TestTCPBeaconRestartRecovery(t *testing.T) {
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	const masterKey = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
	s := initTCPBeaconServer(t, database)
	s.configMu.Lock()
	s.cfg.Server.BeaconKey = masterKey
	s.configMu.Unlock()
	conn, done := tcpFrameConn(t, s)
	defer done()

	secretID, secretB64, err := s.createRegSecret()
	if err != nil {
		t.Fatalf("createRegSecret: %v", err)
	}
	secret, err := base64.StdEncoding.DecodeString(secretB64)
	if err != nil || len(secret) != 32 {
		t.Fatalf("v3 secret invalid: err=%v len=%d", err, len(secret))
	}
	agent := newTCPTestAgent(t, "dddddddd-eeee-4333-8444-ffffffffffff").
		withRawRegKey(secret).
		withSecretID(secretID)

	// Register (seq 1) then one encrypted beacon (seq 2).
	tcpWriteFrame(t, conn, []byte(agent.registerFrame()))
	respFrame := tcpReadFrame(t, conn)
	var regResp struct {
		Seq     uint64 `json:"seq"`
		RegOK   bool   `json:"reg_ok"`
		ECDHPub string `json:"ecdh_pub"`
		Mac     string `json:"mac"`
	}
	if err := encoding.Unmarshal(respFrame, &regResp); err != nil || !regResp.RegOK {
		t.Fatalf("registration failed: %v (body=%s)", err, respFrame)
	}
	if err := agent.establishFromServerKey(regResp.ECDHPub); err != nil {
		t.Fatalf("establish session: %v", err)
	}

	// Simulate server restart: fresh session manager (no sessions).
	s.sessionManager = nil
	sm, err := crypto.NewSessionManager()
	if err != nil {
		t.Fatalf("new session manager: %v", err)
	}
	s.sessionManager = sm

	// Encrypted frame is rejected (no session to decrypt).
	inner, _ := json.Marshal(map[string]interface{}{
		"uuid": agent.uuid, "pv": 2,
		"info": map[string]string{"hostname": "RESTART", "username": "u", "ip": "10.0.0.8"},
	})
	tcpWriteFrame(t, conn, []byte(agent.encryptedFrame(inner)))
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var msgLen uint32
	if err := binary.Read(conn, binary.BigEndian, &msgLen); err == nil {
		t.Fatalf("expected close for undecryptable frame, got length %d", msgLen)
	}

	// Reconnect (fresh TCP session) and re-handshake.
	conn.Close()
	conn, done = tcpFrameConn(t, s)
	defer done()
	tcpWriteFrame(t, conn, []byte(agent.handshakeFrame()))
	respFrame = tcpReadFrame(t, conn)
	var hsResp struct {
		Seq     uint64 `json:"seq"`
		RegOK   bool   `json:"reg_ok"`
		ECDHPub string `json:"ecdh_pub"`
		Mac     string `json:"mac"`
	}
	if err := encoding.Unmarshal(respFrame, &hsResp); err != nil {
		t.Fatalf("handshake response parse failed: %v (body=%s)", err, respFrame)
	}
	if hsResp.RegOK {
		t.Fatalf("re-handshake after registration must not re-register: %s", respFrame)
	}
	if !agent.verifyResponseMAC(hsResp.Seq, hsResp.ECDHPub, hsResp.Mac) {
		t.Fatalf("handshake response MAC mismatch: %s", respFrame)
	}
	if err := agent.establishFromServerKey(hsResp.ECDHPub); err != nil {
		t.Fatalf("re-establish session: %v", err)
	}

	// Encrypted frame now succeeds.
	tcpWriteFrame(t, conn, []byte(agent.encryptedFrame(inner)))
	respFrame = tcpReadFrame(t, conn)
	var encResp struct {
		CipherB64 string `json:"c"`
	}
	if err := encoding.Unmarshal(respFrame, &encResp); err != nil || encResp.CipherB64 == "" {
		t.Fatalf("expected encrypted response after recovery, got %s (err=%v)", respFrame, err)
	}
	if _, err := agent.decryptWithAAD(encResp.CipherB64, agent.aad(agent.seq)); err != nil {
		t.Fatalf("decrypt recovery response: %v", err)
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

// TestTCPBeaconUnauthenticatedFrameClosesConn verifies a handshake frame without
// a valid MAC terminates the TCP session (v2 auth replaces the old X-Beacon-Key
// gate).
func TestTCPBeaconUnauthenticatedFrameClosesConn(t *testing.T) {
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	s := initTCPBeaconServer(t, database)
	s.configMu.Lock()
	s.cfg.Server.BeaconKey = "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
	s.configMu.Unlock()

	conn, done := tcpFrameConn(t, s)
	defer done()

	// Handshake-shaped frame with no mac (or wrong mac).
	ts := time.Now().Unix()
	body, _ := json.Marshal(map[string]interface{}{
		"uuid":     "bbbbbbbb-cccc-4333-8444-dddddddddddd",
		"seq":      1,
		"ts":       ts,
		"ecdh_pub": base64.StdEncoding.EncodeToString(make([]byte, 32)),
	})
	tcpWriteFrame(t, conn, []byte(body))

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var msgLen uint32
	if err := binary.Read(conn, binary.BigEndian, &msgLen); err == nil {
		t.Fatalf("expected connection close for unauthenticated frame, got frame length %d", msgLen)
	}
}

// TestTCPBeaconRejectsV2MasterKey verifies the deprecated v2 master-key
// registration path is rejected: an implant that presents no per-implant
// secret_id (the old globally-derived key) is refused, forcing v3 per-implant
// secrets.
func TestTCPBeaconRejectsV2MasterKey(t *testing.T) {
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	s := initTCPBeaconServer(t, database)
	s.configMu.Lock()
	s.cfg.Server.BeaconKey = "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
	s.configMu.Unlock()
	conn, done := tcpFrameConn(t, s)
	defer done()

	// v2 master-key derived key, but NO secret_id.
	agent := newTCPTestAgent(t, "eeeeeeee-cccc-4333-8444-dddddddddddd").withRegKey(s.cfg.Server.BeaconKey)

	tcpWriteFrame(t, conn, []byte(agent.registerFrame()))
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var msgLen uint32
	if err := binary.Read(conn, binary.BigEndian, &msgLen); err == nil {
		t.Fatalf("expected connection close for v2 master-key frame, got length %d", msgLen)
	}
}

func ginSetTestMode(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
}
