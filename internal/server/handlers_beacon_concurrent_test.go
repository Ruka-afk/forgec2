package server

import (
	"encoding/json"
	"fmt"
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
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

func initBeaconTestServer(t *testing.T, database *gorm.DB) (*Server, *gin.Engine) {
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
	s.configMu.Lock()
	s.cfg.Server.BeaconKey = v2TestMasterKey
	s.configMu.Unlock()
	r := gin.New()
	r.POST("/beacon", s.handleBeacon)
	s.router = r
	return s, r
}

func TestBeaconConcurrentCheckins(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := testutil.SetupTestDB(t)
	s, r := initBeaconTestServer(t, database)

	numAgents := 5
	for i := range numAgents {
		agentID := fmt.Sprintf("11111111-2222-4333-8444-00000000000%d", i)
		agent := newTCPTestAgent(t, agentID).withRegKey(v2TestMasterKey)

		s.beaconDedupMu.Lock()
		s.beaconDedupCache = make(map[string]time.Time)
		s.beaconDedupMu.Unlock()

		w := postJSON(r, "/beacon", agent.registerFrame())
		if w.Code != 200 {
			t.Errorf("agent %d status=%d body=%s", i, w.Code, w.Body.String())
		}
	}

	var agentCount int64
	database.Model(&db.Implant{}).Count(&agentCount)
	if int(agentCount) != numAgents {
		t.Errorf("expected %d agents created, got %d", numAgents, agentCount)
	}
}

func TestBeaconReconnection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := testutil.SetupTestDB(t)
	s, r := initBeaconTestServer(t, database)

	agentUUID := "22222222-3333-4333-8444-555555555555"
	agent := newTCPTestAgent(t, agentUUID).withRegKey(v2TestMasterKey)

	clearDedup := func() {
		s.beaconDedupMu.Lock()
		s.beaconDedupCache = make(map[string]time.Time)
		s.beaconDedupMu.Unlock()
	}

	// First beacon: register (also establishes the session)
	clearDedup()
	w1 := postJSON(r, "/beacon", agent.registerFrame())
	if w1.Code != 200 {
		t.Fatalf("first beacon: expected 200, got %d; body=%s", w1.Code, w1.Body.String())
	}
	var regResp struct {
		Seq     uint64 `json:"seq"`
		RegOK   bool   `json:"reg_ok"`
		ECDHPub string `json:"ecdh_pub"`
		Mac     string `json:"mac"`
	}
	if err := encoding.Unmarshal(w1.Body.Bytes(), &regResp); err != nil {
		t.Fatalf("register response parse: %v", err)
	}
	if err := agent.establishFromServerKey(regResp.ECDHPub); err != nil {
		t.Fatalf("establish session: %v", err)
	}

	// Create a pending task
	task := &db.Task{
		AgentID:  agentUUID,
		Type:     "shell",
		Command:  "whoami",
		Status:   "pending",
		Priority: 1,
	}
	if err := database.Create(task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	// Second beacon: should receive task
	clearDedup()
	inner, _ := json.Marshal(map[string]interface{}{"uuid": agentUUID, "pv": 2})
	w2 := postJSON(r, "/beacon", agent.encryptedFrame(inner))
	if w2.Code != 200 {
		t.Fatalf("second beacon: expected 200, got %d; body=%s", w2.Code, w2.Body.String())
	}
	var encResp struct {
		CipherB64 string `json:"c"`
	}
	if err := encoding.Unmarshal(w2.Body.Bytes(), &encResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	plain, err := agent.decryptWithAAD(encResp.CipherB64, agent.aad(agent.seq))
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	var respAfterTask beaconResponse
	if err := encoding.Unmarshal(plain, &respAfterTask); err != nil {
		t.Fatalf("unmarshal inner: %v", err)
	}
	if len(respAfterTask.Tasks) == 0 {
		t.Fatal("expected task after first beacon+create")
	}

	// Simulate reconnection: update last_seen to be old
	database.Model(&db.Implant{}).Where("id = ?", agentUUID).
		Update("last_seen", time.Now().Add(-30*time.Minute))

	// Third beacon: reconnection (session still valid)
	clearDedup()
	inner2, _ := json.Marshal(map[string]interface{}{"uuid": agentUUID, "pv": 2})
	w3 := postJSON(r, "/beacon", agent.encryptedFrame(inner2))
	if w3.Code != 200 {
		t.Fatalf("reconnect beacon: expected 200, got %d; body=%s", w3.Code, w3.Body.String())
	}

	var agentRow db.Implant
	database.Where("id = ?", agentUUID).First(&agentRow)
	if time.Since(agentRow.LastSeen) > 10*time.Second {
		t.Error("last_seen should be updated after reconnection")
	}
}

func TestBeaconProtocolVersionRejection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := testutil.SetupTestDB(t)
	s, r := initBeaconTestServer(t, database)

	t.Run("v1 plaintext frames rejected", func(t *testing.T) {
		s.beaconDedupMu.Lock()
		s.beaconDedupCache = make(map[string]time.Time)
		s.beaconDedupMu.Unlock()
		w := postJSON(r, "/beacon", `{"uuid":"33333333-4444-4333-8444-000000000000","pv":1,"info":{"hostname":"old"}}`)
		if w.Code != http.StatusBadRequest {
			t.Errorf("v1 plaintext frame must be rejected, got %d", w.Code)
		}
	})

	t.Run("v2 registration accepted", func(t *testing.T) {
		agent := newTCPTestAgent(t, "44444444-5555-4333-8444-666666666666").withRegKey(v2TestMasterKey)
		s.beaconDedupMu.Lock()
		s.beaconDedupCache = make(map[string]time.Time)
		s.beaconDedupMu.Unlock()
		w := postJSON(r, "/beacon", agent.registerFrame())
		if w.Code != 200 {
			t.Errorf("valid v2 beacon should be accepted, got %d; body=%s", w.Code, w.Body.String())
		}
	})
}

// postJSON posts a JSON body to the given router and returns the recorder.
func postJSON(r *gin.Engine, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}
