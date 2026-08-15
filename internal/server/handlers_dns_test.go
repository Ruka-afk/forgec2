package server

import (
	"encoding/base64"
	"encoding/json"
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

func initDNSBeaconServer(t *testing.T, database *gorm.DB) *Server {
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

// TestDNSBeaconRejectsPlaintext verifies plaintext v1 frames are rejected by the
// DNS listener handler (v2 has no plaintext frames).
func TestDNSBeaconRejectsPlaintext(t *testing.T) {
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	s := initDNSBeaconServer(t, database)
	h := s.makeBeaconHandler()

	agentUUID := "99999999-8888-4333-8444-aaaaaaaaaaaa"
	body := `{"uuid":"` + agentUUID + `","info":{"hostname":"DNS-PLAIN","username":"u","ip":"10.0.0.9"},"pv":1}`

	respJSON := h(agentUUID, []byte(body))
	if len(respJSON) != 0 {
		t.Fatalf("plaintext DNS beacon must be rejected, got %q", respJSON)
	}

	var count int64
	database.Model(&db.Implant{}).Where("id = ?", agentUUID).Count(&count)
	if count != 0 {
		t.Fatalf("plaintext beacon must not register an implant, count=%d", count)
	}
}

// TestDNSBeaconV2RegisterAndEncrypted verifies the full v2 registration +
// encrypted exchange over the DNS listener handler.
func TestDNSBeaconV2RegisterAndEncrypted(t *testing.T) {
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	s := initDNSBeaconServer(t, database)
	const masterKey = "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
	s.configMu.Lock()
	s.cfg.Server.BeaconKey = masterKey
	s.configMu.Unlock()
	h := s.makeBeaconHandler()

	agent := v3TestAgent(t, s, "aaaaaaaa-bbbb-4333-8444-cccccccccccc")

	task := db.Task{AgentID: agent.uuid, Type: "shell", Command: "echo ok", Status: "pending"}
	if err := database.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	// Step 1: registration envelope
	respJSON := h(agent.uuid, []byte(agent.registerFrame()))
	if len(respJSON) == 0 {
		t.Fatal("registration should produce a response")
	}
	var regResp struct {
		Seq     uint64 `json:"seq"`
		RegOK   bool   `json:"reg_ok"`
		ECDHPub string `json:"ecdh_pub"`
		Mac     string `json:"mac"`
	}
	if err := encoding.Unmarshal(respJSON, &regResp); err != nil {
		t.Fatalf("register response parse failed: %v (body=%s)", err, respJSON)
	}
	if !regResp.RegOK {
		t.Fatalf("registration must succeed, got %s", respJSON)
	}
	if !agent.verifyResponseMAC(regResp.Seq, regResp.ECDHPub, regResp.Mac) {
		t.Fatalf("register response MAC mismatch: %s", respJSON)
	}
	if err := agent.establishFromServerKey(regResp.ECDHPub); err != nil {
		t.Fatalf("agent establish session: %v", err)
	}

	// Step 2: encrypted beacon carrying a result
	inner, _ := json.Marshal(map[string]interface{}{
		"uuid": agent.uuid,
		"pv":   2,
		"info": map[string]string{"hostname": "DNS-ECDH", "username": "u", "ip": "10.0.0.8"},
		"results": []map[string]interface{}{
			{"task_id": task.ID, "type": "shell", "output": "ok", "error": ""},
		},
	})
	encryptedBody := agent.encryptedFrame(inner)
	if encryptedBody == "" {
		t.Fatalf("agent encrypt failed")
	}
	respJSON = h(agent.uuid, []byte(encryptedBody))
	if len(respJSON) == 0 {
		t.Fatal("encrypted beacon should get a response")
	}
	var encResp struct {
		CipherB64 string `json:"c"`
	}
	if err := encoding.Unmarshal(respJSON, &encResp); err != nil {
		t.Fatalf("encrypted response parse failed: %v (body=%s)", err, respJSON)
	}
	if encResp.CipherB64 == "" {
		t.Fatalf("encrypted beacon response must carry c field, got %s", respJSON)
	}
	plaintext, err := agent.decryptWithAAD(encResp.CipherB64, agent.aad(agent.seq))
	if err != nil {
		t.Fatalf("agent decrypt response: %v", err)
	}
	var innerResp beaconResponse
	if err := encoding.Unmarshal(plaintext, &innerResp); err != nil {
		t.Fatalf("decrypted response is not a beaconResponse: %v", err)
	}

	var implant db.Implant
	if err := database.Where("id = ?", agent.uuid).First(&implant).Error; err != nil {
		t.Fatalf("agent not registered: %v", err)
	}

	var result db.Task
	if err := database.Where("id = ? AND agent_id = ?", task.ID, agent.uuid).First(&result).Error; err != nil {
		t.Fatalf("encrypted DNS result not stored: %v", err)
	}
	if result.Result != "ok" {
		t.Fatalf("result output = %q, want ok", result.Result)
	}
}

// TestDNSBeaconRejectsInvalidAgentID verifies the shared envelope decoder rejects
// traversal-style agent IDs over DNS (same guard as HTTP/TCP).
func TestDNSBeaconRejectsInvalidAgentID(t *testing.T) {
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	s := initDNSBeaconServer(t, database)
	h := s.makeBeaconHandler()

	body := `{"uuid":"../../etc/passwd","info":{"hostname":"EVIL","username":"u","ip":"10.0.0.7"},"pv":1}`
	respJSON := h("../../etc/passwd", []byte(body))
	if len(respJSON) != 0 {
		t.Fatalf("invalid agent ID should be rejected (empty response), got %q", respJSON)
	}
}

// TestDNSBeaconBadKeyRejected verifies an unauthenticated frame is rejected over
// DNS (v2 auth replaces the old X-Beacon-Key gate).
func TestDNSBeaconBadKeyRejected(t *testing.T) {
	ginSetTestMode(t)
	database := testutil.SetupTestDB(t)
	s := initDNSBeaconServer(t, database)
	s.configMu.Lock()
	s.cfg.Server.BeaconKey = "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
	s.configMu.Unlock()
	h := s.makeBeaconHandler()

	// Handshake-shaped frame with no mac.
	ts := time.Now().Unix()
	body, _ := json.Marshal(map[string]interface{}{
		"uuid":     "bbbbbbbb-cccc-4333-8444-dddddddddddd",
		"seq":      1,
		"ts":       ts,
		"ecdh_pub": base64.StdEncoding.EncodeToString(make([]byte, 32)),
	})
	respJSON := h("bbbbbbbb-cccc-4333-8444-dddddddddddd", []byte(body))
	if len(respJSON) != 0 {
		t.Fatalf("unauthenticated frame should be rejected (empty response), got %q", respJSON)
	}
}
