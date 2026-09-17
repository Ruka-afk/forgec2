package server

import (
	"bytes"
	"encoding/base64"
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

func udpTestAddr() net.Addr {
	return &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 9999}
}

// TestUDPBeaconRejectsPlaintext verifies plaintext v1 frames are rejected by
// the UDP datagram path (v2 has no plaintext frames; nothing is sent back).
func TestUDPBeaconRejectsPlaintext(t *testing.T) {
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	s := initDNSBeaconServer(t, database)

	agentUUID := "aaaaaaaa-1111-4333-8444-bbbbbbbbbbbb"
	body := `{"uuid":"` + agentUUID + `","info":{"hostname":"UDP-PLAIN","username":"u","ip":"10.0.0.9"},"pv":1}`

	if resp := s.handleUDPBeacon([]byte(body), udpTestAddr()); len(resp) != 0 {
		t.Fatalf("plaintext UDP beacon must be rejected, got %d bytes", len(resp))
	}

	var count int64
	database.Model(&db.Implant{}).Where("id = ?", agentUUID).Count(&count)
	if count != 0 {
		t.Fatalf("plaintext beacon must not register an implant, count=%d", count)
	}
}

// TestUDPBeaconRejectsInvalidAgentID verifies traversal-style agent IDs die
// on the UDP path like every other transport.
func TestUDPBeaconRejectsInvalidAgentID(t *testing.T) {
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	s := initDNSBeaconServer(t, database)

	body := `{"uuid":"../../etc/passwd","info":{"hostname":"EVIL","username":"u","ip":"10.0.0.7"},"pv":1}`
	if resp := s.handleUDPBeacon([]byte(body), udpTestAddr()); len(resp) != 0 {
		t.Fatalf("invalid agent ID should be rejected (empty response), got %q", resp)
	}
}

// TestUDPBeaconBadKeyRejected verifies an unauthenticated handshake frame is
// rejected over UDP.
func TestUDPBeaconBadKeyRejected(t *testing.T) {
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	s := initDNSBeaconServer(t, database)
	s.configMu.Lock()
	s.cfg.Server.BeaconKey = "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
	s.configMu.Unlock()

	ts := time.Now().Unix()
	body, _ := json.Marshal(map[string]interface{}{
		"uuid":     "bbbbbbbb-cccc-4333-8444-dddddddddddd",
		"seq":      1,
		"ts":       ts,
		"ecdh_pub": base64.StdEncoding.EncodeToString(make([]byte, 32)),
	})
	if resp := s.handleUDPBeacon(body, udpTestAddr()); len(resp) != 0 {
		t.Fatalf("unauthenticated frame should be rejected (empty response), got %q", resp)
	}
}

// TestUDPBeaconV2RegisterAndEncrypted verifies the full v2 registration +
// encrypted exchange over a UDP datagram round trip.
func TestUDPBeaconV2RegisterAndEncrypted(t *testing.T) {
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	// Result persistence encrypts at rest: without this the vault fails
	// closed and the stored output is blank (same requirement the DNS/TCP
	// flow tests inherit from suite order).
	crypto.InitLootEncryption(strings.Repeat("22", 32))
	s := initDNSBeaconServer(t, database)
	const masterKey = "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
	s.configMu.Lock()
	s.cfg.Server.BeaconKey = masterKey
	s.configMu.Unlock()

	agent := v3TestAgent(t, s, "cccccccc-dddd-4333-8444-eeeeeeeeeeee")

	task := db.Task{AgentID: agent.uuid, Type: "shell", Command: "echo ok", Status: "pending"}
	if err := database.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	resp := s.handleUDPBeacon([]byte(agent.registerFrame()), udpTestAddr())
	if len(resp) == 0 {
		t.Fatal("registration should produce a response datagram")
	}
	var regResp struct {
		Seq     uint64 `json:"seq"`
		RegOK   bool   `json:"reg_ok"`
		ECDHPub string `json:"ecdh_pub"`
		Mac     string `json:"mac"`
	}
	if err := encoding.Unmarshal(resp, &regResp); err != nil {
		t.Fatalf("register response parse failed: %v (body=%s)", err, resp)
	}
	if !regResp.RegOK {
		t.Fatalf("registration must succeed, got %s", resp)
	}
	if !agent.verifyResponseMAC(regResp.Seq, regResp.ECDHPub, regResp.Mac) {
		t.Fatalf("register response MAC mismatch: %s", resp)
	}
	if err := agent.establishFromServerKey(regResp.ECDHPub); err != nil {
		t.Fatalf("agent establish session: %v", err)
	}

	inner, _ := json.Marshal(map[string]interface{}{
		"uuid": agent.uuid,
		"pv":   2,
		"info": map[string]string{"hostname": "UDP-ECDH", "username": "u", "ip": "10.0.0.8"},
		"results": []map[string]interface{}{
			{"task_id": task.ID, "type": "shell", "output": "ok", "error": ""},
		},
	})
	encryptedBody := agent.encryptedFrame(inner)
	if encryptedBody == "" {
		t.Fatalf("agent encrypt failed")
	}
	resp = s.handleUDPBeacon([]byte(encryptedBody), udpTestAddr())
	if len(resp) == 0 {
		t.Fatal("encrypted beacon should get a response datagram")
	}
	var encResp struct {
		CipherB64 string `json:"c"`
	}
	if err := encoding.Unmarshal(resp, &encResp); err != nil {
		t.Fatalf("encrypted response parse failed: %v (body=%s)", err, resp)
	}
	if encResp.CipherB64 == "" {
		t.Fatalf("encrypted beacon response must carry c field, got %s", resp)
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
		t.Fatalf("encrypted UDP result not stored: %v", err)
	}
	if result.Result != "ok" {
		t.Fatalf("result output = %q status=%q error=%q, want ok", result.Result, result.Status, result.Error)
	}
}

// TestICMPHandlerRejectsPlaintext verifies the ICMP-wired handler refuses
// plaintext without needing raw sockets (privileged in CI).
func TestICMPHandlerRejectsPlaintext(t *testing.T) {
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	s := initDNSBeaconServer(t, database)
	h := s.makeBeaconHandler("icmp")

	agentUUID := "dddddddd-1111-4333-8444-eeeeeeeeeeee"
	body := `{"uuid":"` + agentUUID + `","info":{"hostname":"ICMP-PLAIN","username":"u","ip":"10.0.0.9"},"pv":1}`
	if respJSON := h(agentUUID, []byte(body)); len(respJSON) != 0 {
		t.Fatalf("plaintext ICMP beacon must be rejected, got %q", respJSON)
	}

	var count int64
	database.Model(&db.Implant{}).Where("id = ?", agentUUID).Count(&count)
	if count != 0 {
		t.Fatalf("plaintext beacon must not register an implant, count=%d", count)
	}
}

// TestICMPHandlerBadKeyRejected verifies unauthenticated frames die on the
// ICMP-wired handler.
func TestICMPHandlerBadKeyRejected(t *testing.T) {
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	s := initDNSBeaconServer(t, database)
	s.configMu.Lock()
	s.cfg.Server.BeaconKey = "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
	s.configMu.Unlock()
	h := s.makeBeaconHandler("icmp")

	ts := time.Now().Unix()
	body, _ := json.Marshal(map[string]interface{}{
		"uuid":     "eeeeeeee-ffff-4333-8444-aaaaaaaaaaaa",
		"seq":      1,
		"ts":       ts,
		"ecdh_pub": base64.StdEncoding.EncodeToString(make([]byte, 32)),
	})
	if respJSON := h("eeeeeeee-ffff-4333-8444-aaaaaaaaaaaa", body); len(respJSON) != 0 {
		t.Fatalf("unauthenticated frame should be rejected (empty response), got %q", respJSON)
	}
}

// TestDNSMTUBudgetEndToEnd proves the low-MTU budget works across real
// beacons: an 8KB tunnel backlog over the dns-budgeted handler arrives
// truncated per beacon (≤4KB) and losslessly over two beacons, in order.
func TestDNSMTUBudgetEndToEnd(t *testing.T) {
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	crypto.InitLootEncryption(strings.Repeat("22", 32))
	s := initDNSBeaconServer(t, database)
	const masterKey = "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
	s.configMu.Lock()
	s.cfg.Server.BeaconKey = masterKey
	s.configMu.Unlock()
	h := s.makeBeaconHandler("dns")

	agent := v3TestAgent(t, s, "ffffffff-0000-4333-8444-111111111111")
	regRespBytes := h(agent.uuid, []byte(agent.registerFrame()))
	var regResp struct {
		Seq     uint64 `json:"seq"`
		RegOK   bool   `json:"reg_ok"`
		ECDHPub string `json:"ecdh_pub"`
		Mac     string `json:"mac"`
	}
	if err := encoding.Unmarshal(regRespBytes, &regResp); err != nil || !regResp.RegOK {
		t.Fatalf("registration failed: %v (%s)", err, regRespBytes)
	}
	if err := agent.establishFromServerKey(regResp.ECDHPub); err != nil {
		t.Fatalf("establish: %v", err)
	}

	// 8KB patterned backlog: 4 chunks the reassembly order can verify.
	var backlog []byte
	for i := 0; i < 4; i++ {
		chunk := bytes.Repeat([]byte{byte('A' + i)}, 2048)
		backlog = append(backlog, chunk...)
		if !s.socksEngine.enqueueRPortFwdFrame(agent.uuid, socksFrame{ConnID: 5, Action: "rportfwd_data", Data: chunk}) {
			t.Fatalf("enqueue chunk %d: queue should fit 8KB", i)
		}
	}

	beacon := func() []socksFrame {
		inner, _ := json.Marshal(map[string]interface{}{
			"uuid": agent.uuid, "pv": 2,
			"info": map[string]string{"hostname": "MTU", "username": "u", "ip": "10.0.0.8"},
		})
		body := agent.encryptedFrame(inner)
		if body == "" {
			t.Fatalf("agent encrypt failed")
		}
		respJSON := h(agent.uuid, []byte(body))
		if len(respJSON) == 0 {
			t.Fatal("budgeted beacon got no response")
		}
		var encResp struct {
			CipherB64 string `json:"c"`
		}
		if err := encoding.Unmarshal(respJSON, &encResp); err != nil || encResp.CipherB64 == "" {
			t.Fatalf("response has no ciphertext: %v (%s)", err, respJSON)
		}
		plaintext, err := agent.decryptWithAAD(encResp.CipherB64, agent.aad(agent.seq))
		if err != nil {
			t.Fatalf("decrypt: %v", err)
		}
		var innerResp beaconResponse
		if err := encoding.Unmarshal(plaintext, &innerResp); err != nil {
			t.Fatalf("response is not a beaconResponse: %v", err)
		}
		return innerResp.SocksFrames
	}

	sumData := func(frames []socksFrame) (int, []byte) {
		var n int
		var all []byte
		for _, f := range frames {
			if f.Action != "rportfwd_data" {
				t.Fatalf("unexpected frame action %q", f.Action)
			}
			n += len(f.Data)
			all = append(all, f.Data...)
		}
		return n, all
	}

	first := beacon()
	n1, _ := sumData(first)
	if n1 == 0 || n1 > SocksLowMTUBudget {
		t.Fatalf("first beacon carried %d tunnel bytes, want (0,%d]", n1, SocksLowMTUBudget)
	}
	second := beacon()
	n2, rest := sumData(second)
	if n1+n2 != len(backlog) {
		t.Fatalf("two-beacon total=%d, want %d (lossless)", n1+n2, len(backlog))
	}
	var firstBytes []byte
	for _, f := range first {
		firstBytes = append(firstBytes, f.Data...)
	}
	if !bytes.Equal(append(firstBytes, rest...), backlog) {
		t.Fatal("reassembled stream differs from backlog (reorder/loss)")
	}
}
