package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// ── RD hub semantics ───────────────────────────────────────────────────────

// TestRDHubJoinReplacementAndLeaveOwnership guards the reconnect path: a
// second stream from the same operator replaces the registration, and the
// replaced socket's deferred Leave must not evict its successor.
func TestRDHubJoinReplacementAndLeaveOwnership(t *testing.T) {
	hub := &RDHub{sessions: make(map[string]*RDSession)}
	connA := &websocket.Conn{}
	connB := &websocket.Conn{}

	opA, replaced, err := hub.Join("agent-1", "user-1", connA)
	if err != nil || opA == nil || replaced != nil {
		t.Fatalf("first join: op=%v replaced=%v err=%v", opA, replaced, err)
	}
	if hub.OperatorCount() != 1 {
		t.Fatalf("operator count after first join = %d, want 1", hub.OperatorCount())
	}

	opB, replaced, err := hub.Join("agent-1", "user-1", connB)
	if err != nil || opB == nil {
		t.Fatalf("second join: op=%v err=%v", opB, err)
	}
	if replaced != connA {
		t.Fatalf("second join replaced=%v, want the first connection", replaced)
	}
	if hub.OperatorCount() != 1 {
		t.Fatalf("replacement changed operator count to %d, want 1", hub.OperatorCount())
	}

	hub.Leave("agent-1", "user-1", connA) // stale reader unwinds
	if got := hub.SessionOperatorCount("agent-1"); got != 1 {
		t.Fatalf("stale Leave evicted the successor: count=%d, want 1", got)
	}

	hub.Leave("agent-1", "user-1", connB)
	if hub.OperatorCount() != 0 || hub.SessionOperatorCount("agent-1") != 0 {
		t.Fatalf("final leave left state behind: total=%d session=%d", hub.OperatorCount(), hub.SessionOperatorCount("agent-1"))
	}
}

// TestRDHubEnforcesCaps proves both the per-agent and the global connection
// caps refuse new viewers while still allowing slot replacement.
func TestRDHubEnforcesCaps(t *testing.T) {
	hub := &RDHub{sessions: make(map[string]*RDSession)}

	fill := func(agentID string, n int) {
		t.Helper()
		for i := 0; i < n; i++ {
			if _, _, err := hub.Join(agentID, fmt.Sprintf("user-%d", i), &websocket.Conn{}); err != nil {
				t.Fatalf("join %s/user-%d: %v", agentID, i, err)
			}
		}
	}

	fill("agent-per", MaxRDOperatorsPerAgent)
	if _, _, err := hub.Join("agent-per", "user-extra", &websocket.Conn{}); !errors.Is(err, errRDHubFull) {
		t.Fatalf("per-agent cap not enforced: err=%v", err)
	}
	// Replacement of an existing slot stays allowed at the cap.
	if _, _, err := hub.Join("agent-per", "user-0", &websocket.Conn{}); err != nil {
		t.Fatalf("replacement at per-agent cap refused: %v", err)
	}

	// Global cap: top up other agents until the hub is full, then refuse.
	agentN := 0
	for hub.OperatorCount() < MaxRDOperatorConnections {
		agentN++
		fill(fmt.Sprintf("agent-global-%d", agentN), MaxRDOperatorsPerAgent)
	}
	if _, _, err := hub.Join("agent-any", "user-0", &websocket.Conn{}); !errors.Is(err, errRDHubFull) {
		t.Fatalf("global cap not enforced: err=%v", err)
	}
}

// ── Remote input validation + limiter ──────────────────────────────────────

func TestRemoteInputNormalize(t *testing.T) {
	valid := []struct {
		in   remoteInputRequest
		want remoteInputRequest
	}{
		{remoteInputRequest{Type: "MOVE", X: 10, Y: -20}, remoteInputRequest{Type: "move", X: 10, Y: -20}},
		{remoteInputRequest{Type: "click", X: 0, Y: 0}, remoteInputRequest{Type: "click"}},
		{remoteInputRequest{Type: " Key ", Key: "ENTER"}, remoteInputRequest{Type: "key", Key: "enter"}},
	}
	for _, tc := range valid {
		req := tc.in
		if err := req.normalize(); err != nil {
			t.Fatalf("normalize(%+v): %v", tc.in, err)
		}
		if req != tc.want {
			t.Fatalf("normalize(%+v) = %+v, want %+v", tc.in, req, tc.want)
		}
	}

	invalid := []remoteInputRequest{
		{Type: ""},
		{Type: "exec"},
		{Type: "move", X: MaxRemoteInputCoord + 1},
		{Type: "move", Y: -(MaxRemoteInputCoord + 1)},
		{Type: "key", Key: ""},
		{Type: "key", Key: strings.Repeat("k", MaxRemoteInputKeyLen+1)},
	}
	for _, req := range invalid {
		if err := req.normalize(); err == nil {
			t.Fatalf("normalize(%+v) accepted invalid event", req)
		}
	}
}

func TestRemoteInputRequestFromMap(t *testing.T) {
	req, err := remoteInputRequestFromMap("click", map[string]interface{}{"x": float64(12), "y": float64(34)})
	if err != nil || req.X != 12 || req.Y != 34 {
		t.Fatalf("fromMap integral floats: req=%+v err=%v", req, err)
	}
	if _, err := remoteInputRequestFromMap("move", map[string]interface{}{"x": 1.5}); err == nil {
		t.Fatal("fractional coordinate accepted")
	}
	if _, err := remoteInputRequestFromMap("move", map[string]interface{}{"x": "10"}); err == nil {
		t.Fatal("string coordinate accepted")
	}
	if _, err := remoteInputRequestFromMap("move", nil); err == nil {
		t.Fatal("nil data accepted")
	}
}

func TestFixedWindowLimiterWindows(t *testing.T) {
	var l fixedWindowLimiter
	now := time.Now()
	for i := 0; i < 3; i++ {
		if !l.allow("k", 3, time.Second, now) {
			t.Fatalf("event %d refused inside budget", i)
		}
	}
	if l.allow("k", 3, time.Second, now) {
		t.Fatal("over-budget event accepted")
	}
	if !l.allow("k", 3, time.Second, now.Add(time.Second)) {
		t.Fatal("next window not reset")
	}
	if removed := l.sweep(now.Add(time.Minute)); removed != 1 {
		t.Fatalf("sweep removed %d, want 1", removed)
	}
}

// newRemoteInputContext builds a JSON POST context for /api/agents/:id/input
// with the authenticated operator identity requireOperator + currentUserID need.
func newRemoteInputContext(agentID, body string) (*httptest.ResponseRecorder, *gin.Context) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/agents/"+agentID+"/input", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_role", "operator")
	c.Set("user_id", uint(1))
	c.Params = gin.Params{{Key: "id", Value: agentID}}
	return w, c
}

// TestHandleAgentRemoteInputRejectsInvalidEvents proves unknown types and
// out-of-range coordinates fail at the API boundary without creating tasks.
func TestHandleAgentRemoteInputRejectsInvalidEvents(t *testing.T) {
	ginSetTestMode(t)
	database := newContractDB(t)
	srv := newTestFileServer(t, database)
	agent := seedImplant(t, database)

	invalid := []string{
		`{"type":"","x":0,"y":0}`,
		`{"type":"shell","x":0,"y":0}`,
		`{"type":"move","x":40000,"y":0}`,
		`{"type":"move","x":0,"y":-40000}`,
		`{"type":"key","key":""}`,
		`{"type":"key","key":"` + strings.Repeat("k", MaxRemoteInputKeyLen+1) + `"}`,
	}
	for _, body := range invalid {
		w, c := newRemoteInputContext(agent.ID, body)
		srv.handleAgentRemoteInput(c)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("body %s: status=%d, want 400 (body=%s)", body, w.Code, w.Body.String())
		}
	}
	var count int64
	database.Model(&db.Task{}).Where("agent_id = ?", agent.ID).Count(&count)
	if count != 0 {
		t.Fatalf("invalid input events created %d tasks", count)
	}

	// A valid event still goes through and is normalized into the payload.
	w, c := newRemoteInputContext(agent.ID, `{"type":"MOVE","x":10,"y":20}`)
	srv.handleAgentRemoteInput(c)
	if w.Code != http.StatusOK {
		t.Fatalf("valid move: status=%d body=%s", w.Code, w.Body.String())
	}
	var task db.Task
	if err := database.Where("agent_id = ? AND type = ?", agent.ID, "remote_input").First(&task).Error; err != nil {
		t.Fatalf("load remote_input task: %v", err)
	}
	var payload remoteInputRequest
	if err := json.Unmarshal([]byte(task.Command), &payload); err != nil {
		t.Fatalf("task command is not JSON: %v (%s)", err, task.Command)
	}
	if payload.Type != "move" || payload.X != 10 || payload.Y != 20 {
		t.Fatalf("normalized payload = %+v", payload)
	}
}

// TestHandleAgentRemoteInputRateLimit proves the per-(operator,agent) window
// caps accepted events (and therefore task + audit writes).
func TestHandleAgentRemoteInputRateLimit(t *testing.T) {
	ginSetTestMode(t)
	database := newContractDB(t)
	srv := newTestFileServer(t, database)
	agent := seedImplant(t, database)

	body := `{"type":"move","x":1,"y":2}`
	for i := 0; i < MaxRemoteInputEventsPerWindow; i++ {
		w, c := newRemoteInputContext(agent.ID, body)
		srv.handleAgentRemoteInput(c)
		if w.Code != http.StatusOK {
			t.Fatalf("event %d: status=%d body=%s", i, w.Code, w.Body.String())
		}
	}
	w, c := newRemoteInputContext(agent.ID, body)
	srv.handleAgentRemoteInput(c)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("over-budget event: status=%d, want 429 (body=%s)", w.Code, w.Body.String())
	}

	// A different agent has its own window.
	other := seedImplant(t, database)
	w, c = newRemoteInputContext(other.ID, body)
	srv.handleAgentRemoteInput(c)
	if w.Code != http.StatusOK {
		t.Fatalf("other agent throttled: status=%d body=%s", w.Code, w.Body.String())
	}
}

// ── Dashboard socket frame filtering ───────────────────────────────────────

// TestBroadcastScreenshotFiltersNonOperators proves live screen frames are
// delivered only to clients that could fetch them over HTTP (agents.write):
// a read-only role on the same tenant must not watch a desktop through /ws.
func TestBroadcastScreenshotFiltersNonOperators(t *testing.T) {
	ginSetTestMode(t)
	database := newContractDB(t)
	s := &Server{
		db:                    database,
		ctx:                   context.Background(),
		wsClients:             make(map[*websocket.Conn]*wsClientConn),
		screenMonitorImplants: make(map[string]time.Time),
		screenFrames:          make(map[string]cachedScreenFrame),
	}
	if err := database.Create(&db.Implant{ID: "rd-role", TenantID: 1}).Error; err != nil {
		t.Fatal(err)
	}
	for _, u := range []db.User{
		{Username: "rd-role-viewer", Role: "viewer", TenantID: 1, IsActive: true},
		{Username: "rd-role-op", Role: "user", TenantID: 1, IsActive: true},
	} {
		if err := database.Create(&u).Error; err != nil {
			t.Fatal(err)
		}
	}

	if s.userCanViewScreen("rd-role-viewer") {
		t.Fatal("viewer role cleared the screen gate")
	}
	if !s.userCanViewScreen("rd-role-op") {
		t.Fatal("operator role blocked by the screen gate")
	}
	if s.userCanViewScreen("no-such-user") {
		t.Fatal("unknown user cleared the screen gate (must fail closed)")
	}

	viewer := &wsClientConn{conn: &websocket.Conn{}, tenantID: 1, canViewScreen: s.userCanViewScreen("rd-role-viewer"), ch: make(chan []byte, 4), done: make(chan struct{})}
	operator := &wsClientConn{conn: &websocket.Conn{}, tenantID: 1, canViewScreen: s.userCanViewScreen("rd-role-op"), ch: make(chan []byte, 4), done: make(chan struct{})}
	legacy := &wsClientConn{conn: &websocket.Conn{}, tenantID: 0, canViewScreen: true, ch: make(chan []byte, 4), done: make(chan struct{})}
	s.wsMutex.Lock()
	s.wsClients[viewer.conn] = viewer
	s.wsClients[operator.conn] = operator
	s.wsClients[legacy.conn] = legacy
	s.wsMutex.Unlock()

	s.BroadcastScreenshot("rd-role", "/9j/frame")

	if len(viewer.ch) != 0 {
		t.Fatalf("viewer received %d screen frames", len(viewer.ch))
	}
	for name, client := range map[string]*wsClientConn{"operator": operator, "legacy": legacy} {
		if len(client.ch) != 1 {
			t.Fatalf("%s received %d frames, want 1", name, len(client.ch))
		}
		var msg struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(<-client.ch, &msg); err != nil || msg.Type != "screenshot" {
			t.Fatalf("%s frame payload: type=%q err=%v", name, msg.Type, err)
		}
	}
}

// ── Page tenant scoping ────────────────────────────────────────────────────

// TestScreenPageCrossTenantRedirects proves the screen monitor page honors
// tenant scope (previously an unscoped read: any tenant could open another
// tenant's agent page and JSON payload by id).
func TestScreenPageCrossTenantRedirects(t *testing.T) {
	ginSetTestMode(t)
	s := initV3BeaconServer(t, testutil.SetupTestDB(t), tenantVisibilityMasterHex)
	if err := s.db.Create(&db.Implant{ID: "t-screen-page", TenantID: 2, Hostname: "OTHER"}).Error; err != nil {
		t.Fatal(err)
	}

	c, w := tenantScopedAdminContext(s, t, "screen-page-viewer", 1)
	c.Params = gin.Params{{Key: "id", Value: "t-screen-page"}}
	s.handleScreenMonitorPage(c)
	if w.Code != http.StatusFound {
		t.Fatalf("cross-tenant screen page status=%d, want 302 redirect", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/agents" {
		t.Fatalf("cross-tenant redirect location=%q", loc)
	}

	// Same-tenant agents still render.
	if err := s.db.Create(&db.Implant{ID: "t-screen-own", TenantID: 1, Hostname: "MINE"}).Error; err != nil {
		t.Fatal(err)
	}
	c2, w2 := tenantScopedAdminContext(s, t, "screen-page-viewer", 1)
	c2.Params = gin.Params{{Key: "id", Value: "t-screen-own"}}
	s.handleScreenMonitorPage(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("same-tenant screen page status=%d body=%s", w2.Code, w2.Body.String())
	}
}

// TestTokenPageCrossTenantRedirects proves the token management page (which
// lists token usernames/domains) is tenant-scoped too.
func TestTokenPageCrossTenantRedirects(t *testing.T) {
	ginSetTestMode(t)
	s := initV3BeaconServer(t, testutil.SetupTestDB(t), tenantVisibilityMasterHex)
	if err := s.db.Create(&db.Implant{ID: "t-token-page", TenantID: 2, Hostname: "OTHER"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&db.TokenEntry{AgentID: "t-token-page", Username: "foreign-user"}).Error; err != nil {
		t.Fatal(err)
	}

	c, w := tenantScopedAdminContext(s, t, "token-page-viewer", 1)
	c.Params = gin.Params{{Key: "id", Value: "t-token-page"}}
	s.handleTokenPage(c)
	if w.Code != http.StatusFound {
		t.Fatalf("cross-tenant token page status=%d, want 302 redirect", w.Code)
	}
	if body := w.Body.String(); strings.Contains(body, "foreign-user") {
		t.Fatalf("cross-tenant token data leaked: %s", body)
	}
}
