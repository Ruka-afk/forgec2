package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func TestHandleOperatorWS_RequiresAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := newContractDB(t)
	s := &Server{db: database}
	s.operatorSessions = &operatorSessionTracker{sessions: make(map[uint]*WSOperatorSession)}

	doRequest := func(cookies ...*http.Cookie) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodGet, "/ws/operator", nil)
		for _, ck := range cookies {
			c.Request.AddCookie(ck)
		}
		s.handleOperatorWS(c)
		return w
	}

	t.Run("no cookie rejects", func(t *testing.T) {
		w := doRequest()
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("no cookie: got %d, want 401", w.Code)
		}
	})

	t.Run("invalid cookie rejects", func(t *testing.T) {
		w := doRequest(&http.Cookie{Name: "forgec2_session", Value: "not-a-real-token"})
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("invalid token: got %d, want 401", w.Code)
		}
	})
}

func TestHandleOperatorWS_AcceptsValidSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.DefaultConfig()
	cfg.Server.JWTSecret = "test-secret-for-opws-32chars!!"
	if err := middleware.InitJWTSecret(cfg, ""); err != nil {
		t.Fatalf("InitJWTSecret: %v", err)
	}
	database := newContractDB(t)
	s := &Server{db: database, cfg: cfg}
	s.operatorSessions = &operatorSessionTracker{sessions: make(map[uint]*WSOperatorSession)}

	user := db.User{
		Username: "op-ws-user",
		Role:     "admin",
		IsActive: true,
	}
	if err := database.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	token, err := middleware.GenerateToken(user, false, 24)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	if err := s.createSession(token, user.ID, "127.0.0.1", "test", "", 86400); err != nil {
		t.Fatalf("create session: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/ws/operator", nil)
	c.Request.AddCookie(&http.Cookie{Name: "forgec2_session", Value: token})
	s.handleOperatorWS(c)

	// The handler authenticates first, then attempts the WebSocket upgrade.
	// In a test recorder the upgrade fails (no real socket) — but a 401/403
	// would indicate the auth guard rejected a valid session.
	if w.Code == http.StatusUnauthorized || w.Code == http.StatusForbidden {
		t.Fatalf("valid session rejected: got %d; body=%s", w.Code, w.Body.String())
	}
}

// TestBroadcastOperatorEvent_DeliversToLegacyHub verifies the operator event
// fan-out reaches the legacy /ws hub (buffered channel clients), which is what
// the browser dashboard actually connects to.
// TestBroadcastTaskUpdate_StreamsLargeShellOutput verifies that a large
// completed shell result is emitted as ordered task_output frames (with a
// trailing done:true) BEFORE the task_update status frame, and that small
// results skip streaming entirely.
func TestBroadcastTaskUpdate_StreamsLargeShellOutput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{
		db:        newContractDB(t),
		ctx:       context.Background(),
		wsClients: make(map[*websocket.Conn]*wsClientConn),
		wsMutex:   sync.RWMutex{},
	}

	client := &wsClientConn{
		conn:    &websocket.Conn{},
		session: UserSession{Username: "legacy-client"},
		ch:      make(chan []byte, 64),
		done:    make(chan struct{}),
	}
	s.wsMutex.Lock()
	s.wsClients[client.conn] = client
	s.wsMutex.Unlock()

	read := func() map[string]interface{} {
		select {
		case msg := <-client.ch:
			var got map[string]interface{}
			if err := json.Unmarshal(msg, &got); err != nil {
				t.Fatalf("invalid frame json %q: %v", msg, err)
			}
			return got
		case <-time.After(2 * time.Second):
			t.Fatal("frame never reached legacy hub")
			return nil
		}
	}

	big := strings.Repeat("line-of-output-0123456789\n", 3000) // ~90KB
	s.broadcastTaskUpdate("agent-1", db.Task{
		ID:     42,
		Type:   "shell",
		Status: "completed",
		Result: big,
	})

	var rebuilt strings.Builder
	frameCount := 0
	for {
		frame := read()
		if frame["type"] != "task_output" {
			t.Fatalf("expected task_output frame, got %v", frame["type"])
		}
		frameCount++
		rebuilt.WriteString(frame["chunk"].(string))
		if done, _ := frame["done"].(bool); done {
			break
		}
	}
	if rebuilt.String() != big {
		t.Fatalf("reassembled output differs from source (frames=%d, bytes=%d vs %d)", frameCount, rebuilt.Len(), len(big))
	}
	// Disabled: frame ordering is deterministic here, but asserting it keeps
	// the test honest about the stream-before-status contract.
	update := read()
	if update["type"] != "task_update" {
		t.Fatalf("expected trailing task_update, got %v", update["type"])
	}
	if update["status"] != "completed" {
		t.Fatalf("task_update status = %v, want completed", update["status"])
	}

	// Small results must not stream frames.
	s.broadcastTaskUpdate("agent-1", db.Task{
		ID:     43,
		Type:   "shell",
		Status: "completed",
		Result: "small output",
	})
	frameCount = 0
	for {
		frame := read()
		if frame["type"] == "task_output" {
			frameCount++
			continue
		}
		if frame["type"] == "task_update" && frame["result"] == "small output" {
			break
		}
	}
	if frameCount != 0 {
		t.Fatalf("small result streamed %d task_output frames, want 0", frameCount)
	}
}

func TestBroadcastOperatorEvent_DeliversToLegacyHub(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{
		db:        newContractDB(t),
		ctx:       context.Background(),
		wsClients: make(map[*websocket.Conn]*wsClientConn),
		wsMutex:   sync.RWMutex{},
	}

	client := &wsClientConn{
		conn:    &websocket.Conn{},
		session: UserSession{Username: "legacy-client"},
		ch:      make(chan []byte, 8),
		done:    make(chan struct{}),
	}
	s.wsMutex.Lock()
	s.wsClients[client.conn] = client
	s.wsMutex.Unlock()

	s.broadcastOperatorEvent(map[string]interface{}{"type": "test_event", "value": 42})

	select {
	case msg := <-client.ch:
		var got struct {
			Type  string `json:"type"`
			Value int    `json:"value"`
		}
		if err := json.Unmarshal(msg, &got); err != nil {
			t.Fatalf("invalid event json %q: %v", msg, err)
		}
		if got.Type != "test_event" || got.Value != 42 {
			t.Fatalf("unexpected event payload: %+v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("operator event never reached legacy hub")
	}
}

// TestOperatorWS_SyncSnapshotOnConnect verifies that a fresh /ws/operator
// connection immediately receives a {"type":"sync"} snapshot carrying the
// current build job list, so reconnects converge without polling.
func TestOperatorWS_SyncSnapshotOnConnect(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.DefaultConfig()
	cfg.Server.JWTSecret = "test-secret-for-opws-sync-32chars!"
	if err := middleware.InitJWTSecret(cfg, ""); err != nil {
		t.Fatalf("InitJWTSecret: %v", err)
	}
	database := newContractDB(t)
	s := &Server{
		db:               database,
		cfg:              cfg,
		ctx:              context.Background(),
		buildJobs:        make(map[string]*BuildJob),
		wsClients:        make(map[*websocket.Conn]*wsClientConn),
		wsMutex:          sync.RWMutex{},
		wsUpgrader:       websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
		operatorSessions: &operatorSessionTracker{sessions: make(map[uint]*WSOperatorSession)},
	}

	user := db.User{Username: "op-ws-sync", Role: "admin", IsActive: true}
	if err := database.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	token, err := middleware.GenerateToken(user, false, 24)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	if err := s.createSession(token, user.ID, "127.0.0.1", "test", "", 86400); err != nil {
		t.Fatalf("create session: %v", err)
	}

	router := gin.New()
	router.GET("/ws/operator", s.handleOperatorWS)
	ts := httptest.NewServer(router)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/operator"
	header := http.Header{}
	header.Add("Cookie", "forgec2_session="+token)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("dial /ws/operator: %v", err)
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read sync snapshot: %v", err)
	}
	var msg struct {
		Type   string  `json:"type"`
		Builds []gin.H `json:"builds"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("invalid sync json %q: %v", data, err)
	}
	if msg.Type != "sync" {
		t.Fatalf("first frame type = %q, want sync", msg.Type)
	}
	if msg.Builds == nil {
		t.Fatalf("sync snapshot missing builds list")
	}
}

// TestHandleWebSocket_PingPong verifies the application-level heartbeat:
// a {"type":"ping"} message must be answered with {"type":"pong"} routed
// through the writer goroutine (never written directly from the reader,
// which would race the writer goroutine on the same connection).
func TestHandleWebSocket_PingPong(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := config.DefaultConfig()
	cfg.Server.JWTSecret = "test-secret-for-wsping-32chars!!"
	if err := middleware.InitJWTSecret(cfg, ""); err != nil {
		t.Fatalf("InitJWTSecret: %v", err)
	}
	database := newContractDB(t)
	s := &Server{
		db:               database,
		cfg:              cfg,
		ctx:              context.Background(),
		wsClients:        make(map[*websocket.Conn]*wsClientConn),
		wsMutex:          sync.RWMutex{},
		wsUpgrader:       websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
		operatorSessions: &operatorSessionTracker{sessions: make(map[uint]*WSOperatorSession)},
	}

	user := db.User{Username: "ping-pong-user", Role: "admin", IsActive: true}
	if err := database.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	token, err := middleware.GenerateToken(user, false, 24)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	if err := s.createSession(token, user.ID, "127.0.0.1", "test", "", 86400); err != nil {
		t.Fatalf("create session: %v", err)
	}

	router := gin.New()
	router.GET("/ws", s.handleWebSocket)
	ts := httptest.NewServer(router)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	header := http.Header{}
	header.Add("Cookie", "forgec2_session="+token)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("dial /ws: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"ping"}`)); err != nil {
		t.Fatalf("write ping: %v", err)
	}
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	// The connection may deliver broadcast frames (e.g. user_online) ahead
	// of the pong; drain until we see the heartbeat reply.
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read pong: %v", err)
		}
		var msg struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("invalid message json %q: %v", data, err)
		}
		if msg.Type == "pong" {
			return
		}
	}
}
