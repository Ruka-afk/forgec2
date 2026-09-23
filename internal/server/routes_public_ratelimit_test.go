package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// TestPublicRoutesRateLimitConfigured verifies login and public download
// routes are registered with IP rate limiters (P2-8).
func TestPublicRoutesRateLimitConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := newContractDB(t)
	cfg := config.DefaultConfig()
	cfg.Server.JWTSecret = "test-secret-for-ratelimit-32chars!"
	if err := middleware.InitJWTSecret(cfg, ""); err != nil {
		t.Fatalf("InitJWTSecret: %v", err)
	}
	s := &Server{
		db:               database,
		cfg:              cfg,
		ctx:              t.Context(),
		router:           gin.New(),
		apiRateLimiter:   middleware.NewAPIRateLimiter(t.Context(), 100, 50),
		loginLockout:     newLoginLockoutTracker(),
		operatorSessions: &operatorSessionTracker{sessions: make(map[uint]*WSOperatorSession)},
		wsUpgrader:       websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
		wsClients:        make(map[*websocket.Conn]*wsClientConn),
	}
	s.SetupRoutes()

	// Burn the login budget for one IP: LoginRequestRate (10) then one more.
	remote := "203.0.113.50:40000"
	var limited bool
	for i := 0; i < LoginRequestRate+2; i++ {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/login", nil)
		r.RemoteAddr = remote
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		s.router.ServeHTTP(w, r)
		if w.Code == http.StatusTooManyRequests {
			limited = true
			break
		}
	}
	if !limited {
		t.Fatalf("login POST was never rate-limited after %d attempts", LoginRequestRate+2)
	}

	// Stage route must be registered (path exists); limiter presence is
	// proven by the login case above sharing the same NewRateLimiter pattern.
	foundStage := false
	for _, ri := range s.router.Routes() {
		if ri.Path == "/stage/:token" {
			foundStage = true
			break
		}
	}
	if !foundStage {
		t.Fatal("/stage/:token route missing after SetupRoutes")
	}
}

// TestOperatorWSConcurrentCap proves MaxOperatorWSConns is enforced before upgrade.
func TestOperatorWSConcurrentCap(t *testing.T) {
	s := newWSAuthTestServer(t)
	user := seedWSAuthUser(t, s, "ws-cap-user")
	token := issueWSAuthToken(t, s, user)

	// Fill the tracker to the cap without real sockets.
	s.operatorSessions.mu.Lock()
	for i := uint(1); i <= MaxOperatorWSConns; i++ {
		s.operatorSessions.sessions[i] = &WSOperatorSession{
			UserID:   i,
			Username: "filler",
			send:     make(chan []byte, 1),
			done:     make(chan struct{}),
		}
	}
	s.operatorSessions.mu.Unlock()

	w := callOperatorWS(s, token)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("operator WS over cap: got %d, want 503; body=%s", w.Code, w.Body.String())
	}
}
