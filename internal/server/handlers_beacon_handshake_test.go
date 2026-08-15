package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/forgec2/forgec2/pkg/encoding"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// v2TestServer builds a beacon router with a live session manager and master
// beacon key.
func v2TestServer(t *testing.T) (*Server, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	database := testutil.SetupTestDB(t)

	sm, err := crypto.NewSessionManager()
	if err != nil {
		t.Fatalf("new session manager: %v", err)
	}
	s := &Server{
		db:                database,
		cfg:               &config.Config{},
		sessionManager:    sm,
		regSecrets:        crypto.NewRegSecretStore(make([]byte, 32)),
		beaconDedupCache:  make(map[string]time.Time),
		eventManager:      NewEventManager(database),
		socksEngine:       newSocksRelayEngine(),
		agentPendingTasks: make(map[string]int),
	}
	s.configMu.Lock()
	s.cfg.Server.BeaconKey = "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
	s.configMu.Unlock()
	r := gin.New()
	r.POST("/beacon", s.handleBeacon)
	s.router = r
	return s, database
}

func v2Post(t *testing.T, s *Server, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/beacon", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	s.router.ServeHTTP(w, req)
	return w
}

// TestV2HandshakeDoesNotClaimTasks verifies an authenticated handshake frame
// only re-establishes the session and returns the server public key — it must
// NOT run the normal processing pipeline, otherwise pending tasks are claimed
// ("running") and stuck forever because the agent discards this response and
// re-beacons encrypted.
func TestV2HandshakeDoesNotClaimTasks(t *testing.T) {
	s, database := v2TestServer(t)

	agentUUID := "22222222-3333-4333-8444-666666666666"
	agent := v3TestAgent(t, s, agentUUID)

	// Register first (also must not claim tasks).
	w := v2Post(t, s, agent.registerFrame())
	if w.Code != http.StatusOK {
		t.Fatalf("registration: expected 200, got %d; body=%s", w.Code, w.Body.String())
	}

	// Seed a pending task AFTER registration so only encrypted beacons claim it.
	task := &db.Task{
		AgentID: agentUUID,
		Type:    "shell",
		Command: "whoami",
		Status:  "pending",
	}
	if err := database.Create(task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	// Authenticated handshake (fresh ephemeral key, MAC over uuid||pub||ts).
	w = v2Post(t, s, agent.handshakeFrame())
	if w.Code != http.StatusOK {
		t.Fatalf("handshake: expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	var hs struct {
		Seq     uint64 `json:"seq"`
		RegOK   bool   `json:"reg_ok"`
		ECDHPub string `json:"ecdh_pub"`
		Mac     string `json:"mac"`
	}
	if err := encoding.Unmarshal(w.Body.Bytes(), &hs); err != nil {
		t.Fatalf("handshake response parse: %v (body=%s)", err, w.Body.String())
	}
	if hs.ECDHPub == "" {
		t.Fatalf("handshake response must carry ecdh_pub: %s", w.Body.String())
	}
	if hs.RegOK {
		t.Fatalf("handshake must not re-register: %s", w.Body.String())
	}
	if !agent.verifyResponseMAC(hs.Seq, hs.ECDHPub, hs.Mac) {
		t.Fatalf("handshake response MAC mismatch: %s", w.Body.String())
	}

	// The pending task must NOT have been claimed.
	var claimed db.Task
	if err := database.First(&claimed, task.ID).Error; err != nil {
		t.Fatalf("reload task: %v", err)
	}
	if claimed.Status != "pending" {
		t.Fatalf("handshake claimed the task (status=%q), want pending", claimed.Status)
	}
}

// TestV2EncryptedBeaconDeliversTaskAfterRegistration proves the same task is
// delivered once the agent re-beacons with the established session (encrypted).
func TestV2EncryptedBeaconDeliversTaskAfterRegistration(t *testing.T) {
	s, database := v2TestServer(t)

	agentUUID := "33333333-4444-4333-8444-777777777777"
	agent := v3TestAgent(t, s, agentUUID)

	// Register.
	w := v2Post(t, s, agent.registerFrame())
	if w.Code != http.StatusOK {
		t.Fatalf("registration: expected 200, got %d; body=%s", w.Code, w.Body.String())
	}
	var regResp struct {
		Seq     uint64 `json:"seq"`
		RegOK   bool   `json:"reg_ok"`
		ECDHPub string `json:"ecdh_pub"`
		Mac     string `json:"mac"`
	}
	if err := encoding.Unmarshal(w.Body.Bytes(), &regResp); err != nil {
		t.Fatalf("register response parse: %v", err)
	}
	if !agent.verifyResponseMAC(regResp.Seq, regResp.ECDHPub, regResp.Mac) {
		t.Fatalf("register response MAC mismatch: %s", w.Body.String())
	}
	if err := agent.establishFromServerKey(regResp.ECDHPub); err != nil {
		t.Fatalf("establish session: %v", err)
	}

	// Seed pending task AFTER registration so only the encrypted beacon can claim it.
	task := &db.Task{
		AgentID: agentUUID,
		Type:    "shell",
		Command: "whoami",
		Status:  "pending",
	}
	if err := database.Create(task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	// Encrypted beacon (inner plaintext request).
	inner, _ := json.Marshal(map[string]interface{}{
		"uuid": agentUUID,
		"pv":   2,
		"info": map[string]string{"hostname": "HS", "username": "u", "ip": "10.0.0.10"},
	})
	encBody := agent.encryptedFrame(inner)
	if encBody == "" {
		t.Fatalf("agent encrypt failed")
	}
	w2 := v2Post(t, s, encBody)
	if w2.Code != http.StatusOK {
		t.Fatalf("encrypted beacon: expected 200, got %d; body=%s", w2.Code, w2.Body.String())
	}

	// Decrypt the response and verify the task is delivered.
	var encResp struct {
		CipherB64 string `json:"c"`
	}
	if err := encoding.Unmarshal(w2.Body.Bytes(), &encResp); err != nil {
		t.Fatalf("encrypted response parse: %v", err)
	}
	plain, err := agent.decryptWithAAD(encResp.CipherB64, agent.aad(agent.seq))
	if err != nil {
		t.Fatalf("decrypt response: %v", err)
	}
	var innerResp beaconResponse
	if err := encoding.Unmarshal(plain, &innerResp); err != nil {
		t.Fatalf("decrypted response parse: %v", err)
	}
	if len(innerResp.Tasks) != 1 || innerResp.Tasks[0].ID != task.ID {
		t.Fatalf("expected task %d delivered, got %+v", task.ID, innerResp.Tasks)
	}
}
