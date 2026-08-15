package server

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/forgec2/forgec2/pkg/encoding"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

// initV3BeaconServer builds a server with the v3 reg-secret store initialized
// from a master beacon key (as in Server.New).
func initV3BeaconServer(t *testing.T, database *gorm.DB, masterHex string) *Server {
	t.Helper()
	sm, err := crypto.NewSessionManager()
	if err != nil {
		t.Fatalf("new session manager: %v", err)
	}
	master, _ := hex.DecodeString(masterHex)
	s := &Server{
		db:                    database,
		cfg:                   &config.Config{},
		sessionManager:        sm,
		regSecrets:            crypto.NewRegSecretStore(master),
		beaconDedupCache:      make(map[string]time.Time),
		eventManager:          NewEventManager(database),
		socksEngine:           newSocksRelayEngine(),
		agentStatusCooldown:   make(map[string]time.Time),
		wsClients:             make(map[*websocket.Conn]*wsClientConn),
		agentPendingTasks:     make(map[string]int),
		screenMonitorImplants: make(map[string]time.Time),
	}
	s.configMu.Lock()
	s.cfg.Server.BeaconKey = masterHex
	s.configMu.Unlock()
	return s
}

// TestTCPBeaconV3RegisterAndHandshake verifies the v3 per-implant secret flow:
// registration carries a secret_id, the server resolves the key from the store
// (not the master key), binds the secret to the implant, and a subsequent
// handshake authenticates with the same per-implant key.
func TestTCPBeaconV3RegisterAndHandshake(t *testing.T) {
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	const masterHex = "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
	s := initV3BeaconServer(t, database, masterHex)

	// The v3 store must be usable.
	id, secretB64, err := s.createRegSecret()
	if err != nil {
		t.Fatalf("createRegSecret: %v", err)
	}
	if id == "" || secretB64 == "" {
		t.Fatalf("createRegSecret returned empty id/secret")
	}
	secret, err := base64.StdEncoding.DecodeString(secretB64)
	if err != nil || len(secret) != 32 {
		t.Fatalf("v3 secret not a 32-byte base64 value: err=%v len=%d", err, len(secret))
	}

	conn, done := tcpFrameConn(t, s)
	defer done()

	// Agent uses ONLY the embedded per-implant secret (never the master key).
	agent := newTCPTestAgent(t, "aaaaaaaa-bbbb-4333-8444-cccccccccccc").
		withRawRegKey(secret).
		withSecretID(id)

	// Step 1: registration with secret_id.
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
		t.Fatalf("v3 registration must succeed, got %s", respFrame)
	}
	if !agent.verifyResponseMAC(regResp.Seq, regResp.ECDHPub, regResp.Mac) {
		t.Fatalf("register response MAC mismatch: %s", respFrame)
	}
	if err := agent.establishFromServerKey(regResp.ECDHPub); err != nil {
		t.Fatalf("agent establish session: %v", err)
	}

	// The implant row must be bound with the secret id and identity.
	var implant db.Implant
	if err := database.Where("id = ?", agent.uuid).First(&implant).Error; err != nil {
		t.Fatalf("agent not registered: %v", err)
	}
	if !implant.Registered || implant.SecretID != id || implant.IdentityPub != agent.publicKeyB64() {
		t.Fatalf("implant v3 binding wrong: registered=%v secret_id=%q identity=%q",
			implant.Registered, implant.SecretID, implant.IdentityPub)
	}
	// The secret row must be marked bound to this agent.
	var row db.RegSecret
	if err := database.Where("id = ?", id).First(&row).Error; err != nil {
		t.Fatalf("reg secret row missing: %v", err)
	}
	if !row.Bound || row.AgentID != agent.uuid {
		t.Fatalf("reg secret not bound: bound=%v agent_id=%q", row.Bound, row.AgentID)
	}

	// Step 2: handshake with the same per-implant key (server resolves via
	// the bound secret_id, NOT the master key).
	tcpWriteFrame(t, conn, []byte(agent.handshakeFrame()))
	respFrame = tcpReadFrame(t, conn)
	var hResp struct {
		Seq     uint64 `json:"seq"`
		ECDHPub string `json:"ecdh_pub"`
		Mac     string `json:"mac"`
	}
	if err := encoding.Unmarshal(respFrame, &hResp); err != nil {
		t.Fatalf("handshake response parse failed: %v (body=%s)", err, respFrame)
	}
	if !agent.verifyResponseMAC(hResp.Seq, hResp.ECDHPub, hResp.Mac) {
		t.Fatalf("handshake response MAC mismatch: %s", respFrame)
	}
}

// TestTCPBeaconV3RejectsUnknownSecret verifies a registration carrying an
// unknown secret_id is rejected even when the master key is configured (an
// attacker with only the master-derived key cannot impersonate a v3 agent).
func TestTCPBeaconV3RejectsUnknownSecret(t *testing.T) {
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	const masterHex = "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
	s := initV3BeaconServer(t, database, masterHex)

	// A real secret exists but the attacker does not know it; they register
	// with a forged secret_id and the master-derived reg key.
	id, _, err := s.createRegSecret()
	if err != nil {
		t.Fatalf("createRegSecret: %v", err)
	}
	agent := newTCPTestAgent(t, "bbbbbbbb-cccc-4333-8444-dddddddddddd").
		withRegKey(masterHex).
		withSecretID(id)

	conn, done := tcpFrameConn(t, s)
	defer done()
	tcpWriteFrame(t, conn, []byte(agent.registerFrame()))
	// Rejected registrations close the connection without a response.
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var msgLen uint32
	if err := binary.Read(conn, binary.BigEndian, &msgLen); err == nil {
		t.Fatalf("expected connection close for rejected v3 registration, got frame length %d", msgLen)
	}

	// Totally unknown secret_id must also be rejected.
	agent2 := newTCPTestAgent(t, "cccccccc-dddd-4333-8444-eeeeeeeeeeee").
		withRawRegKey(make([]byte, 32)).
		withSecretID("deadbeefdeadbeefdeadbeefdeadbeef")
	conn2, done2 := tcpFrameConn(t, s)
	defer done2()
	tcpWriteFrame(t, conn2, []byte(agent2.registerFrame()))
	conn2.SetReadDeadline(time.Now().Add(5 * time.Second))
	if err := binary.Read(conn2, binary.BigEndian, &msgLen); err == nil {
		t.Fatalf("expected connection close for unknown secret_id, got frame length %d", msgLen)
	}
}

// TestTCPBeaconV3DeriveRegKeyUsesBoundSecret verifies that after a v3
// registration, deriveRegKey resolves the per-implant secret (bound) and does
// NOT fall back to the master key.
func TestTCPBeaconV3DeriveRegKeyUsesBoundSecret(t *testing.T) {
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	const masterHex = "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
	s := initV3BeaconServer(t, database, masterHex)

	id, secretB64, err := s.createRegSecret()
	if err != nil {
		t.Fatalf("createRegSecret: %v", err)
	}
	secret, _ := base64.StdEncoding.DecodeString(secretB64)

	agent := newTCPTestAgent(t, "dddddddd-eeee-4333-8444-ffffffffffff").
		withRawRegKey(secret).
		withSecretID(id)

	conn, done := tcpFrameConn(t, s)
	defer done()
	tcpWriteFrame(t, conn, []byte(agent.registerFrame()))
	respFrame := tcpReadFrame(t, conn)
	var regResp struct {
		Seq   uint64 `json:"seq"`
		RegOK bool   `json:"reg_ok"`
	}
	if err := encoding.Unmarshal(respFrame, &regResp); err != nil || !regResp.RegOK {
		t.Fatalf("v3 registration failed: %v (body=%s)", err, respFrame)
	}

	got := s.deriveRegKey(agent.uuid)
	if len(got) != 32 || !hmacEqual(got, secret) {
		t.Fatalf("deriveRegKey must return the bound v3 secret, not the master-derived key")
	}
}

// hmacEqual is a constant-time byte comparison for test assertions.
func hmacEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := range a {
		v |= a[i] ^ b[i]
	}
	return v == 0
}

// TestTCPBeaconV3RecoveryAfterRowDeletion verifies that a v3 agent whose
// implant row is hard-deleted server-side can still recover: the handshake
// authenticates with the per-implant secret (which outlives the row), the
// server signals re-registration, and the subsequent registration frame
// re-binds the agent without operator intervention.
func TestTCPBeaconV3RecoveryAfterRowDeletion(t *testing.T) {
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	const masterHex = "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
	s := initV3BeaconServer(t, database, masterHex)

	id, secretB64, err := s.createRegSecret()
	if err != nil {
		t.Fatalf("createRegSecret: %v", err)
	}
	secret, _ := base64.StdEncoding.DecodeString(secretB64)

	agent := newTCPTestAgent(t, "eeeeeeee-1111-4333-8444-aaaaaaaaaaaa").
		withRawRegKey(secret).
		withSecretID(id)

	conn, done := tcpFrameConn(t, s)
	defer done()

	// Step 1: initial registration.
	tcpWriteFrame(t, conn, []byte(agent.registerFrame()))
	respFrame := tcpReadFrame(t, conn)
	var regResp struct {
		Seq     uint64 `json:"seq"`
		RegOK   bool   `json:"reg_ok"`
		ECDHPub string `json:"ecdh_pub"`
		Mac     string `json:"mac"`
	}
	if err := encoding.Unmarshal(respFrame, &regResp); err != nil || !regResp.RegOK {
		t.Fatalf("v3 registration failed: %v (body=%s)", err, respFrame)
	}
	if !agent.verifyResponseMAC(regResp.Seq, regResp.ECDHPub, regResp.Mac) {
		t.Fatalf("register response MAC mismatch: %s", respFrame)
	}

	// Step 2: operator (or bug) hard-deletes the implant row. The per-implant
	// secret row persists, so the agent can still prove ownership.
	if err := database.Unscoped().Where("id = ?", agent.uuid).Delete(&db.Implant{}).Error; err != nil {
		t.Fatalf("hard delete failed: %v", err)
	}
	var afterDelete int64
	database.Unscoped().Model(&db.Implant{}).Where("id = ?", agent.uuid).Count(&afterDelete)
	if afterDelete != 0 {
		t.Fatalf("implant row should be gone, count=%d", afterDelete)
	}

	// Step 3: agent (still local-registered) sends a handshake. The server must
	// authenticate it via the per-implant secret and signal re-registration.
	tcpWriteFrame(t, conn, []byte(agent.handshakeFrame()))
	respFrame = tcpReadFrame(t, conn)
	var hsResp struct {
		Seq        uint64 `json:"seq"`
		ECDHPub    string `json:"ecdh_pub"`
		Mac        string `json:"mac"`
		Reregister bool   `json:"reregister"`
	}
	if err := encoding.Unmarshal(respFrame, &hsResp); err != nil {
		t.Fatalf("handshake response parse failed: %v (body=%s)", err, respFrame)
	}
	if !agent.verifyResponseMAC(hsResp.Seq, hsResp.ECDHPub, hsResp.Mac) {
		t.Fatalf("re-register response MAC mismatch: %s", respFrame)
	}
	if !hsResp.Reregister {
		t.Fatalf("expected reregister signal after row deletion, got %s", respFrame)
	}

	// Step 4: agent re-registers with a fresh registration frame.
	tcpWriteFrame(t, conn, []byte(agent.registerFrame()))
	respFrame = tcpReadFrame(t, conn)
	var reg2 struct {
		Seq     uint64 `json:"seq"`
		RegOK   bool   `json:"reg_ok"`
		ECDHPub string `json:"ecdh_pub"`
		Mac     string `json:"mac"`
	}
	if err := encoding.Unmarshal(respFrame, &reg2); err != nil || !reg2.RegOK {
		t.Fatalf("v3 re-registration failed: %v (body=%s)", err, respFrame)
	}
	if !agent.verifyResponseMAC(reg2.Seq, reg2.ECDHPub, reg2.Mac) {
		t.Fatalf("re-register response MAC mismatch: %s", respFrame)
	}

	// The row must be re-created and bound to the same secret id.
	var implant db.Implant
	if err := database.Where("id = ?", agent.uuid).First(&implant).Error; err != nil {
		t.Fatalf("agent not re-registered: %v", err)
	}
	if !implant.Registered || implant.SecretID != id {
		t.Fatalf("re-registered implant wrong: registered=%v secret_id=%q", implant.Registered, implant.SecretID)
	}
}

// TestTCPBeaconV3SecretCannotImpersonateOtherAgent verifies the per-implant
// secret is bound to the agent that first registered it: a secret extracted
// from one implant cannot be replayed to authenticate as a different agent.
// Before this fix, regSecretByID ignored the presenting UUID, so holding any
// valid v3 secret let an attacker forge the MAC for an arbitrary victim UUID.
func TestTCPBeaconV3SecretCannotImpersonateOtherAgent(t *testing.T) {
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	const masterHex = "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
	s := initV3BeaconServer(t, database, masterHex)

	id, secretB64, err := s.createRegSecret()
	if err != nil {
		t.Fatalf("createRegSecret: %v", err)
	}
	secret, _ := base64.StdEncoding.DecodeString(secretB64)

	// Step 1: legitimate agent A registers with secret X (binds X to A).
	connA, doneA := tcpFrameConn(t, s)
	defer doneA()
	agentA := newTCPTestAgent(t, "aaaaaaaa-bbbb-4333-8444-cccccccccccc").
		withRawRegKey(secret).
		withSecretID(id)
	tcpWriteFrame(t, connA, []byte(agentA.registerFrame()))
	respA := tcpReadFrame(t, connA)
	var regRespA struct{ RegOK bool `json:"reg_ok"` }
	if err := encoding.Unmarshal(respA, &regRespA); err != nil || !regRespA.RegOK {
		t.Fatalf("agent A registration failed: %v (body=%s)", err, respA)
	}

	// Step 2: attacker presents the SAME secret X but claims a different UUID B.
	// The server must reject the misbound secret and close the connection.
	connB, doneB := tcpFrameConn(t, s)
	defer doneB()
	agentB := newTCPTestAgent(t, "bbbbbbbb-cccc-4333-8444-dddddddddddd").
		withRawRegKey(secret).
		withSecretID(id)
	tcpWriteFrame(t, connB, []byte(agentB.registerFrame()))
	connB.SetReadDeadline(time.Now().Add(5 * time.Second))
	var msgLen uint32
	if err := binary.Read(connB, binary.BigEndian, &msgLen); err == nil {
		t.Fatalf("impersonation accepted: secret bound to A was used to register B (frame len %d)", msgLen)
	}

	// Agent B must not have a row (an unauthenticated UUID must never write DB rows).
	var count int64
	database.Model(&db.Implant{}).Where("id = ?", agentB.uuid).Count(&count)
	if count != 0 {
		t.Fatalf("impersonation created a row for B: count=%d", count)
	}
}

// TestBuildResyncResponseRejectsUnknownUUID verifies the resync endpoint does
// not leak a MAC-signed response for an unknown UUID (an existence oracle).
// Before the fix, deriveRegKey fell back to a master-derived key, so any UUID
// — registered or not — received a valid resync envelope.
func TestBuildResyncResponseRejectsUnknownUUID(t *testing.T) {
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	const masterHex = "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
	s := initV3BeaconServer(t, database, masterHex)

	if _, ok := s.buildResyncResponse("deadbeef-0000-0000-0000-000000000000", 1); ok {
		t.Fatal("resync oracle: unknown UUID must not produce a signed resync response")
	}
}
