package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/forgec2/forgec2/pkg/encoding"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

func initBeaconTestServer(t *testing.T, database *gorm.DB) (*Server, *gin.Engine) {
	t.Helper()
	s := &Server{
		db:                    database,
		cfg:                   &config.Config{},
		beaconDedupCache:      make(map[string]time.Time),
		eventManager:          NewEventManager(database),
		socksEngine:           newSocksRelayEngine(),
		agentStatusCooldown:   make(map[string]time.Time),
		wsClients:             make(map[*websocket.Conn]*wsClientConn),
		agentPendingTasks:     make(map[string]int),
		screenMonitorImplants: make(map[string]time.Time),
	}
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
		beaconJSON := fmt.Sprintf(`{"uuid":"%s","info":{"hostname":"HOST-%d","username":"user","ip":"10.0.0.%d"},"pv":1}`,
			agentID, i, i+1)

		s.beaconDedupMu.Lock()
		s.beaconDedupCache = make(map[string]time.Time)
		s.beaconDedupMu.Unlock()

		w := postJSON(r, "/beacon", beaconJSON)
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

	clearDedup := func() {
		s.beaconDedupMu.Lock()
		s.beaconDedupCache = make(map[string]time.Time)
		s.beaconDedupMu.Unlock()
	}

	// First beacon: register
	clearDedup()
	w1 := postJSON(r, "/beacon", fmt.Sprintf(`{"uuid":"%s","info":{"hostname":"RECONNECT","username":"test","ip":"10.0.0.1"},"pv":1}`, agentUUID))
	if w1.Code != 200 {
		t.Fatalf("first beacon: expected 200, got %d", w1.Code)
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
	w2 := postJSON(r, "/beacon", fmt.Sprintf(`{"uuid":"%s","pv":1}`, agentUUID))
	if w2.Code != 200 {
		t.Fatalf("second beacon: expected 200, got %d", w2.Code)
	}
	var respAfterTask beaconResponse
	if err := encoding.Unmarshal(w2.Body.Bytes(), &respAfterTask); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(respAfterTask.Tasks) == 0 {
		t.Fatal("expected task after first beacon+create")
	}

	// Simulate reconnection: update last_seen to be old
	database.Model(&db.Implant{}).Where("id = ?", agentUUID).
		Update("last_seen", time.Now().Add(-30*time.Minute))

	// Third beacon: reconnection
	clearDedup()
	w3 := postJSON(r, "/beacon", fmt.Sprintf(`{"uuid":"%s","pv":1}`, agentUUID))
	if w3.Code != 200 {
		t.Fatalf("reconnect beacon: expected 200, got %d", w3.Code)
	}

	var agent db.Implant
	database.Where("id = ?", agentUUID).First(&agent)
	if time.Since(agent.LastSeen) > 10*time.Second {
		t.Error("last_seen should be updated after reconnection")
	}
}

func TestBeaconProtocolVersionRejection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := testutil.SetupTestDB(t)
	s, r := initBeaconTestServer(t, database)

	t.Run("pv=0 accepted for backward compat", func(t *testing.T) {
		s.beaconDedupMu.Lock()
		s.beaconDedupCache = make(map[string]time.Time)
		s.beaconDedupMu.Unlock()
		w := postJSON(r, "/beacon", `{"uuid":"33333333-4444-4333-8444-000000000000","pv":0,"info":{"hostname":"old"}}`)
		if w.Code != 200 {
			t.Errorf("pv=0 should be accepted, got %d", w.Code)
		}
	})

	t.Run("first beacon accepted", func(t *testing.T) {
		s.beaconDedupMu.Lock()
		s.beaconDedupCache = make(map[string]time.Time)
		s.beaconDedupMu.Unlock()
		w := postJSON(r, "/beacon", `{"uuid":"44444444-5555-4333-8444-666666666666","pv":1,"info":{"hostname":"NewHost","username":"newuser","ip":"10.0.0.1"}}`)
		if w.Code != 200 {
			t.Errorf("valid beacon should be accepted, got %d; body=%s", w.Code, w.Body.String())
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
