package server

import (
	"encoding/binary"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/forgec2/forgec2/pkg/encoding"
)

// smbFrameConn runs handleSMBConnection over net.Pipe (same length-prefixed
// framing as TCP) and returns the client side.
func smbFrameConn(t *testing.T, s *Server) (net.Conn, func()) {
	t.Helper()
	serverSide, clientSide := net.Pipe()
	go s.handleSMBConnection(serverSide)
	return clientSide, func() { clientSide.Close() }
}

// TestSMBBeaconRejectsPlaintext verifies plaintext v1 frames are rejected over
// SMB (v2 has no plaintext frames; the connection is closed without resync).
func TestSMBBeaconRejectsPlaintext(t *testing.T) {
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	s := initTCPBeaconServer(t, database)
	conn, done := smbFrameConn(t, s)
	defer done()

	agentUUID := "aaaaaaaa-bbbb-4333-8444-cccccccccccc"
	body := `{"uuid":"` + agentUUID + `","info":{"hostname":"SMB-PLAIN","username":"u","ip":"10.0.0.9"},"pv":1}`
	tcpWriteFrame(t, conn, []byte(body))

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

// TestSMBBeaconV2RegisterAndEncrypted verifies the full v2 registration +
// encrypted exchange over the SMB transport.
func TestSMBBeaconV2RegisterAndEncrypted(t *testing.T) {
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	crypto.InitLootEncryption(strings.Repeat("22", 32))
	s := initTCPBeaconServer(t, database)
	s.configMu.Lock()
	s.cfg.Server.BeaconKey = "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
	s.configMu.Unlock()
	conn, done := smbFrameConn(t, s)
	defer done()

	agent := v3TestAgent(t, s, "dddddddd-eeee-4333-8444-ffffffffffff")

	task := db.Task{AgentID: agent.uuid, Type: "shell", Command: "echo ok", Status: "pending"}
	if err := database.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	tcpWriteFrame(t, conn, []byte(agent.registerFrame()))
	regRespBytes := tcpReadFrame(t, conn)
	var regResp struct {
		Seq     uint64 `json:"seq"`
		RegOK   bool   `json:"reg_ok"`
		ECDHPub string `json:"ecdh_pub"`
		Mac     string `json:"mac"`
	}
	if err := encoding.Unmarshal(regRespBytes, &regResp); err != nil {
		t.Fatalf("register response parse failed: %v", err)
	}
	if !regResp.RegOK {
		t.Fatalf("registration must succeed, got %s", regRespBytes)
	}
	if !agent.verifyResponseMAC(regResp.Seq, regResp.ECDHPub, regResp.Mac) {
		t.Fatalf("register response MAC mismatch")
	}
	if err := agent.establishFromServerKey(regResp.ECDHPub); err != nil {
		t.Fatalf("agent establish session: %v", err)
	}

	inner, _ := json.Marshal(map[string]interface{}{
		"uuid": agent.uuid,
		"pv":   2,
		"info": map[string]string{"hostname": "SMB-ECDH", "username": "u", "ip": "10.0.0.8"},
		"results": []map[string]interface{}{
			{"task_id": task.ID, "type": "shell", "output": "ok", "error": ""},
		},
	})
	encryptedBody := agent.encryptedFrame(inner)
	if encryptedBody == "" {
		t.Fatalf("agent encrypt failed")
	}
	tcpWriteFrame(t, conn, []byte(encryptedBody))
	encRespBytes := tcpReadFrame(t, conn)
	var encResp struct {
		CipherB64 string `json:"c"`
	}
	if err := encoding.Unmarshal(encRespBytes, &encResp); err != nil {
		t.Fatalf("encrypted response parse failed: %v", err)
	}
	if encResp.CipherB64 == "" {
		t.Fatalf("encrypted beacon response must carry c field")
	}
	plaintext, err := agent.decryptWithAAD(encResp.CipherB64, agent.aad(agent.seq))
	if err != nil {
		t.Fatalf("agent decrypt response: %v", err)
	}
	var innerResp beaconResponse
	if err := encoding.Unmarshal(plaintext, &innerResp); err != nil {
		t.Fatalf("decrypted response is not a beaconResponse: %v", err)
	}

	var result db.Task
	if err := database.Where("id = ? AND agent_id = ?", task.ID, agent.uuid).First(&result).Error; err != nil {
		t.Fatalf("encrypted SMB result not stored: %v", err)
	}
	if result.Result != "ok" {
		t.Fatalf("result output = %q status=%q, want ok", result.Result, result.Status)
	}
}
