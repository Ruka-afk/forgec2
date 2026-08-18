package server

import (
	"net/http/pprof"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

// registerPublicRoutes registers unauthenticated routes.
func (s *Server) registerPublicRoutes() {
	s.router.GET("/login", s.handleLoginPage)
	s.router.POST("/login", middleware.RequestBodyLimit(MaxJSONBodySize), s.handleLogin)
	s.router.POST("/api/login", middleware.RequestBodyLimit(MaxJSONBodySize), s.handleLogin)
	healthRateLimiter := middleware.NewRateLimiter(s.ctx, 30, time.Minute)
	s.router.GET("/health", healthRateLimiter.Limit(), s.handleHealthCheck)
	s.router.GET("/ready", healthRateLimiter.Limit(), s.handleReadyCheck)
	s.router.GET("/lang/set", s.handleSetLanguage)
	langRateLimiter := middleware.NewRateLimiter(s.ctx, 10, time.Minute)
	s.router.POST("/lang/set", middleware.RequestBodyLimit(MaxJSONBodySize), langRateLimiter.Limit(), s.handleSetLanguage)
	s.router.GET("/payloads/:id/:filename", s.handleServePayload)
	// Public phishing landing (credential capture) — no auth by design
	s.router.GET("/phishing/l/:token", s.handlePhishingLanding)
	s.router.POST("/phishing/l/:token", middleware.RequestBodyLimit(MaxJSONBodySize), s.handlePhishingLanding)

	// WebSocket endpoints: auth handled inside the handler (via cookie/query token)
	// to avoid redirect loops and support cookie-less connections.
	wsRateLimiter := middleware.NewRateLimiter(s.ctx, 10, time.Minute)
	s.router.GET("/ws", wsRateLimiter.Limit(), s.handleWebSocket)
	s.router.GET("/ws/beacon", wsRateLimiter.Limit(), s.handleWebSocketBeacon)
	s.router.GET("/extc2/ws", wsRateLimiter.Limit(), s.handleExternalC2WebSocket)
	s.router.GET("/ws/operator", wsRateLimiter.Limit(), s.handleOperatorWS)
}

// registerAgentRoutes registers dashboard, search, and agent CRUD routes.
func (s *Server) registerAgentRoutes(auth *gin.RouterGroup) {
	auth.GET("/", s.handleDashboard)
	auth.GET("/dashboard", s.handleDashboard)
	auth.GET("/search", s.handleSearchPage)
	// Search indexes agents, tasks and credentials (decrypted in results):
	// require both read permissions.
	auth.GET("/api/search", middleware.RequireAllPermissions(db.PermAgentsRead, db.PermCredsRead), s.handleAPISearch)

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
		agentsRead.GET("/api/agents/:id/screenshots", s.handleListAgentScreenshots)
		agentsRead.GET("/agents/:id/token", s.handleTokenPage)
		agentsRead.GET("/agents/:id/token/list", s.handleGetTokens)
		agentsRead.GET("/api/agents/:id/processes", s.handleGetProcesses)
		agentsRead.GET("/api/agents/:id/process-tree", s.handleGetProcessTree)
		agentsRead.GET("/agents/:id/config", s.handleGetAgentConfig)
		agentsRead.GET("/agents/:id/chain", s.handleAgentChainGet)
		agentsRead.GET("/agents/:id/status-history", s.handleAgentStatusHistory)
	}

	agentsWrite := auth.Group("/")
	agentsWrite.Use(middleware.RequirePermission(db.PermAgentsWrite))
	{
		agentsWrite.POST("/agents/:id/kill", s.handleKillAgent)
		agentsWrite.POST("/agents/:id/note", s.handleUpdateNote)
		agentsWrite.POST("/agents/:id/tasks/:taskId/cancel", s.handleCancelTask)
		agentsWrite.POST("/agents/:id/task/:taskId/rerun", s.handleRerunTask)
		agentsWrite.POST("/agents/batch", s.handleBatchCommand)
		agentsWrite.POST("/agents/bulk/task", s.handleBatchCommand)
		agentsWrite.GET("/agents/bulk/results", s.handleBulkResults)
		agentsWrite.POST("/agents/:id/trust", s.handleToggleAgentTrust)
		agentsWrite.POST("/api/agents/:id/input", s.handleAgentRemoteInput)
		agentsWrite.GET("/agents/:id/socks_relay/status", s.handleSocksRelayStatus)
		agentsWrite.POST("/agents/:id/config", s.handlePushAgentConfig)
		agentsWrite.POST("/agents/:id/chain/set", s.handleAgentChainSet)
		agentsWrite.POST("/agents/:id/chain/clear", s.handleAgentChainClear)
		agentsWrite.POST("/agents/:id/kill_date", s.handleSetKillDate)
		agentsWrite.DELETE("/agents/:id/kill_date", s.handleClearKillDate)
	}
	agentsDelete := auth.Group("/")
	agentsDelete.Use(middleware.RequirePermission(db.PermAgentsDelete))
	{
		agentsDelete.DELETE("/agents/:id", s.handleDeleteAgent)
		agentsDelete.POST("/agents/batch/delete", s.handleBulkDeleteAgents)
	}
}

// registerAgentCommandRoutes registers all agent command handlers, file ops, screen, tokens, SOCKS, rportfwd.
func (s *Server) registerAgentCommandRoutes(auth *gin.RouterGroup) {
	agentCmd := auth.Group("/agents/:id")
	agentCmd.Use(middleware.RequirePermission(db.PermAgentsWrite))
	{
		agentCmd.POST("/command", s.handleSendCommand)
		agentCmd.GET("/screenshot", s.handleGetAgentScreenshot)
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
		agentCmd.POST("/amsi_hardware_bp", s.handleAMSIHardwareBP)
		agentCmd.POST("/etw_hardware_bp", s.handleETWHardwareBP)
		agentCmd.POST("/run_evasion", s.handleRunEvasion)
		agentCmd.POST("/sandbox_detect_advanced", s.handleSandboxDetectAdvanced)
		agentCmd.POST("/set_sleep_mask_advanced", s.handleSetSleepMaskAdvanced)
		agentCmd.POST("/elevate/printnightmare", s.handleElevatePrintNightmare)
		agentCmd.POST("/execute_assembly", s.handleExecuteAssembly)
		agentCmd.POST("/kerberoast", s.handleKerberoast)
		agentCmd.POST("/password_spray", s.handlePasswordSpray)
		agentCmd.POST("/cred_check", s.handleCredCheck)
		agentCmd.POST("/mimikatz", s.handleMimikatz)
		agentCmd.POST("/modules/deploy", s.handleModulesDeploy)
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
		agentCmd.POST("/migrate", s.handleMigrate)
		agentCmd.POST("/self_update", s.handleSelfUpdate)
		agentCmd.POST("/lateral", s.handleLateral)
		agentCmd.POST("/socks", s.handleSocks)

		agentCmd.GET("/rportfwd/status", s.handleRPortFwdStatus)
		agentCmd.POST("/rportfwd/start", s.handleRPortFwdRelayStart)
		agentCmd.POST("/rportfwd/stop", s.handleRPortFwdRelayStop)

		agentCmd.POST("/download", s.handleDownload)
		agentCmd.POST("/upload", s.handleUploadFile)
		agentCmd.POST("/files/push", s.handleUploadFile)

		agentCmd.POST("/files/ls", s.handleListDir)
		agentCmd.POST("/files/delete", s.handleFileDelete)
		agentCmd.POST("/files/read", s.handleFileRead)
		agentCmd.POST("/files/upload", s.handleFileUploadFromAgent)
		agentCmd.POST("/files/pull", s.handleFileUploadFromAgent)
		agentCmd.GET("/files/exfil/:filename", s.handleFileExfilGet)

		agentCmd.POST("/screen/start", s.handleStartScreenMonitor)
		agentCmd.POST("/screen/stop", s.handleStopScreenMonitor)

		agentCmd.POST("/token/list_procs", s.handleTokenListProcs)
		agentCmd.POST("/token/steal", s.handleTokenSteal)
		agentCmd.POST("/token/make", s.handleTokenMake)
		agentCmd.POST("/token/revert", s.handleTokenRevert)
		agentCmd.POST("/token/whoami", s.handleTokenWhoami)
		agentCmd.DELETE("/token/:token_id", s.handleTokenDrop)
		agentCmd.POST("/token/:token_id/impersonate", s.handleTokenImpersonate)
		agentCmd.POST("/token/:token_id/note", s.handleTokenNoteUpdate)

		agentCmd.POST("/socks_relay/start", s.handleStartSocksRelay)
		agentCmd.POST("/socks_relay/stop", s.handleStopSocksRelay)

		agentCmd.POST("/container_detect", s.handleContainerDetect)
		agentCmd.POST("/container_escape", s.handleContainerEscape)
		agentCmd.POST("/container_docker", s.handleContainerDocker)
		agentCmd.POST("/container_k8s", s.handleContainerK8s)

		agentCmd.POST("/coerce/:type", s.handleCoerce)
		agentCmd.POST("/relay/start", s.handleNTLMRelayStart)
		agentCmd.POST("/relay/stop", s.handleNTLMRelayStop)
	}
}

// registerGenerateRoutes registers payload generation routes.
func (s *Server) registerGenerateRoutes(auth *gin.RouterGroup) {
	genRead := auth.Group("/")
	genRead.Use(middleware.RequirePermission(db.PermAgentsRead))
	{
		genRead.GET("/generate", s.handleGeneratePage)
		genRead.GET("/api/generate/profiles", s.handleListProfiles)
		genRead.GET("/generate/builds", s.handleBuildList)
		genRead.GET("/generate/builds/:id", s.handleBuildStatus)
		genRead.GET("/generate/builds/:id/download", s.handleBuildDownload)
	}
	genWrite := auth.Group("/")
	genWrite.Use(middleware.RequirePermission(db.PermAgentsWrite))
	{
		genWrite.POST("/api/generate/profile/import", s.handleImportProfile)
		genWrite.DELETE("/api/generate/profile/:name", s.handleDeleteProfile)
		genWrite.POST("/generate/exe", s.handleGenerateEXE)
		genWrite.POST("/generate/dll", s.handleGenerateDLL)
		genWrite.POST("/generate/ps1", s.handleGeneratePS1)
		genWrite.POST("/generate/linux", s.handleGenerateLinux)
		genWrite.POST("/generate/macos", s.handleGenerateMacOS)
		genWrite.POST("/generate/stager", s.handleGenerateStager)
		genWrite.POST("/generate/stager_linux", s.handleGenerateStagerLinux)
		genWrite.POST("/generate/one-liner", s.handleGenerateOneLiner)
		genWrite.POST("/generate/donut", s.handleGenerateDonut)
		genWrite.POST("/generate/shellcode", s.handleGenerateShellcode)
	}
}

// registerListenerRoutes registers listener CRUD and infrastructure routes.
func (s *Server) registerListenerRoutes(auth *gin.RouterGroup) {
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
		listenersWrite.POST("/api/listeners/:id/enable", s.handleEnableListener)
		listenersWrite.POST("/api/listeners/:id/disable", s.handleDisableListener)
	}
	listenersDelete := auth.Group("/")
	listenersDelete.Use(middleware.RequirePermission(db.PermListenersDelete))
	{
		listenersDelete.DELETE("/api/listeners/:id", s.handleDeleteListener)
	}

	auth.GET("/infrastructure", s.handleInfrastructurePage)
	auth.POST("/infrastructure/generate/nginx", middleware.RequirePermission(db.PermSettingsWrite), s.handleGenerateNginx)
	auth.POST("/infrastructure/generate/apache", middleware.RequirePermission(db.PermSettingsWrite), s.handleGenerateApache)
	auth.POST("/infrastructure/generate/haproxy", middleware.RequirePermission(db.PermSettingsWrite), s.handleGenerateHAProxy)
	// ACME provisioning writes certificates to disk and makes outbound network
	// requests — admin-only.
	auth.POST("/infrastructure/acme/provision", middleware.RequireRole(db.RoleAdmin), s.handleACMECertProvision)
	auth.GET("/infrastructure/profile/export", middleware.RequirePermission(db.PermSettingsWrite), s.handleProfileExport)
}

// registerReconRoutes registers pivoting, topology, loot, scanner, toolkit, timeline, report, lateral, templates, audit routes.
func (s *Server) registerReconRoutes(auth *gin.RouterGroup) {
	reconRead := auth.Group("/")
	reconRead.Use(middleware.RequirePermission(db.PermAgentsRead))
	{
		reconRead.GET("/pivoting", s.handlePivoting)
		reconRead.GET("/topology", s.handleTopologyPage)
		reconRead.GET("/api/topology/data", s.handleTopologyData)
		reconRead.GET("/loot", s.handleLootPage)
		reconRead.GET("/scanner", s.handleScannerPage)
		reconRead.GET("/api/scan/results/:taskId", s.handleScanResults)
		reconRead.GET("/api/scan/agent/:agentId", s.handleScanResultsByAgent)
		reconRead.GET("/api/scan/export/:taskId", s.handleExportScanResults)
		reconRead.GET("/privesc", s.handlePrivescPage)
		reconRead.GET("/api/privesc/history/:id", s.handlePrivescHistory)
		reconRead.GET("/toolkit", s.handleToolkitPage)
		reconRead.GET("/toolkit/results", s.handleToolkitRecentResults)
		reconRead.GET("/toolkit/agents/:id/info", s.handleToolkitAgentInfo)
		reconRead.GET("/toolkit/agents/:id/tasks", s.handleToolkitAgentTasks)
		reconRead.GET("/timeline", s.handleTimelinePage)
		reconRead.GET("/api/timeline/data", s.handleTimelineData)
		reconRead.GET("/api/timeline/export", s.handleTimelineExport)
		reconRead.POST("/api/timeline/export", s.handleTimelineExport)
		reconRead.GET("/report", s.handleReportPage)
		reconRead.GET("/api/report/agents", s.handleAPIGetReportAgents)
		reconRead.GET("/api/report/tasks", s.handleAPIGetReportTasks)
		reconRead.GET("/api/report/credentials", s.handleAPIGetReportCredentials)
		reconRead.GET("/api/report/network", s.handleAPIGetReportNetwork)
		reconRead.GET("/api/report/findings", s.handleAPIGetReportFindings)
		reconRead.GET("/api/report/history", s.handleAPIGetReportHistory)
		reconRead.GET("/api/report/export/pdf", s.handleAPIExportReportPDF)
		reconRead.GET("/lateral", s.handleLateralPage)
		reconRead.GET("/api/lateral/history/:id", s.handleLateralHistory)
		reconRead.GET("/templates", s.handleTemplatesPage)
		reconRead.GET("/api/templates", s.handleListTemplatesJSON)
		reconRead.GET("/api/templates/category/:category", s.handleGetTemplatesByCategory)
	}
	reconWrite := auth.Group("/")
	reconWrite.Use(middleware.RequirePermission(db.PermAgentsWrite))
	{
		reconWrite.POST("/loot/bulk-delete", s.handleLootBulkDelete)
		reconWrite.POST("/api/scan", s.handleScanTask)
		reconWrite.POST("/api/scan/result", s.handleProcessScanResult)
		reconWrite.POST("/api/browser/result", s.handleProcessBrowserResult)
		reconWrite.POST("/api/wifi/result", s.handleProcessWifiResult)
		reconWrite.POST("/api/lateral/result", s.handleProcessLateralResult)
		reconWrite.POST("/api/privesc/result", s.handleProcessPrivescResult)
		reconWrite.POST("/api/privesc/run", s.handlePrivescRun)
		reconWrite.POST("/api/privesc/execute", s.handlePrivescExecute)
		reconWrite.POST("/toolkit/agents/:id/action", s.handleToolkitQuickAction)
		reconWrite.POST("/api/report/generate", s.handleGenerateReport)
		reconWrite.DELETE("/api/report/:id", s.handleAPIDeleteReport)
		reconWrite.POST("/api/lateral/execute", s.handleAPILateralExecute)
		reconWrite.POST("/api/templates", s.handleCreateTemplate)
		reconWrite.PUT("/api/templates/:id", s.handleUpdateTemplate)
		reconWrite.DELETE("/api/templates/:id", s.handleDeleteTemplate)
	}

	auditRead := auth.Group("/")
	auditRead.Use(middleware.RequirePermission(db.PermAuditRead))
	{
		auditRead.GET("/audit", s.handleAuditLogPage)
		auditRead.GET("/audit/logs", s.handleGetAuditLogs)
	}
}

func (s *Server) registerDebugRoutes(auth *gin.RouterGroup) {
	if s.cfg.Server.EnableMetrics {
		auth.GET("/metrics", middleware.RequirePermission(db.PermSettingsRead), metricsPromHandler())
	}
	if s.cfg.Server.EnablePprof {
		pprofGroup := auth.Group("/debug/pprof")
		// pprof dumps process memory (JWT secret, loot keys, decrypted
		// credentials) — admin only.
		pprofGroup.Use(middleware.RequireRole(db.RoleAdmin))
		pprofGroup.GET("/", func(c *gin.Context) { pprof.Index(c.Writer, c.Request) })
		pprofGroup.GET("/cmdline", func(c *gin.Context) { pprof.Cmdline(c.Writer, c.Request) })
		pprofGroup.GET("/profile", func(c *gin.Context) { pprof.Profile(c.Writer, c.Request) })
		pprofGroup.GET("/symbol", func(c *gin.Context) { pprof.Symbol(c.Writer, c.Request) })
		pprofGroup.GET("/trace", func(c *gin.Context) { pprof.Trace(c.Writer, c.Request) })
		pprofGroup.GET("/heap", func(c *gin.Context) { pprof.Handler("heap").ServeHTTP(c.Writer, c.Request) })
		pprofGroup.GET("/goroutine", func(c *gin.Context) { pprof.Handler("goroutine").ServeHTTP(c.Writer, c.Request) })
		pprofGroup.GET("/block", func(c *gin.Context) { pprof.Handler("block").ServeHTTP(c.Writer, c.Request) })
		pprofGroup.GET("/mutex", func(c *gin.Context) { pprof.Handler("mutex").ServeHTTP(c.Writer, c.Request) })
		pprofGroup.GET("/threadcreate", func(c *gin.Context) { pprof.Handler("threadcreate").ServeHTTP(c.Writer, c.Request) })
	}
}

// registerSettingsRoutes registers settings, 2FA, i18n routes.
func (s *Server) registerSettingsRoutes(auth *gin.RouterGroup) {
	settingsRead := auth.Group("/")
	settingsRead.Use(middleware.RequirePermission(db.PermSettingsRead))
	{
		settingsRead.GET("/settings", s.handleSettingsPage)
		settingsRead.GET("/settings/webhooks", s.handleGetSettingsWebhooks)
		settingsRead.GET("/api/modules", s.handleModulesList)
		// The beacon PSK lets the holder mint authenticating implants: gate it
		// to roles that can actually build payloads.
		settingsRead.GET("/settings/beacon-key", middleware.RequirePermission(db.PermAgentsWrite), s.handleGetBeaconKey)
	}
	settingsWrite := auth.Group("/")
	settingsWrite.Use(middleware.RequirePermission(db.PermSettingsWrite))
	{
		settingsWrite.POST("/settings/password", s.handleChangePassword)
		settingsWrite.POST("/settings/agent", s.handleSaveAgentConfig)
		settingsWrite.POST("/settings/server", s.handleSaveServerConfig)
		settingsWrite.POST("/settings/malleable", s.handleSaveMalleableProfile)
		settingsWrite.POST("/config/reload", s.handleConfigReload)
		// Destructive / team-wide actions are admin-only.
		settingsWrite.POST("/settings/purge/tasks", middleware.RequireRole(db.RoleAdmin), s.handlePurgeTasks)
		settingsWrite.POST("/settings/purge/audit", middleware.RequireRole(db.RoleAdmin), s.handlePurgeAuditLogs)
		settingsWrite.POST("/settings/jwt/regenerate", middleware.RequireRole(db.RoleAdmin), s.handleRegenerateJWT)
		settingsWrite.POST("/settings/db/vacuum", s.handleDBVacuum)
		settingsWrite.POST("/settings/db/backup", s.handleDBBackup)
		// The raw database contains every secret (users, TOTP, API-key hashes,
		// encrypted creds) and restore swaps the live DB — admin only.
		settingsWrite.GET("/settings/db/backups", middleware.RequireRole(db.RoleAdmin), s.handleDBBackupList)
		settingsWrite.GET("/settings/db/backups/download", middleware.RequireRole(db.RoleAdmin), s.handleDBBackupDownload)
		settingsWrite.POST("/settings/db/restore", middleware.RequireRole(db.RoleAdmin), s.handleDBRestore)
		settingsWrite.GET("/settings/config/download", s.handleDownloadConfig)
		settingsWrite.POST("/settings/webhooks", s.handleSaveSettingsWebhooks)
		settingsWrite.POST("/settings/webhooks/test", s.handleTestSettingsWebhook)

		settingsWrite.POST("/settings/maintenance/purge", s.handleSettingsMaintenancePurge)

		// Mass agent self-destruct — admin only.
		settingsWrite.POST("/admin/emergency-stop", middleware.RequireRole(db.RoleAdmin), s.handleEmergencyStop)
		settingsWrite.GET("/admin/emergency-status", s.handleEmergencyStatus)

		// Fleet kill-switch broadcast (arm/disarm) — admin only.
		settingsWrite.POST("/admin/killswitch", middleware.RequireRole(db.RoleAdmin), s.handleKillSwitch)
		settingsWrite.GET("/admin/killswitch/status", middleware.RequireRole(db.RoleAdmin), s.handleKillSwitchStatus)

		settingsWrite.POST("/settings/totp/generate", s.handleTOTPGenerate)
		settingsWrite.POST("/settings/totp/enable", s.handleTOTPEnable)
		settingsWrite.POST("/settings/totp/disable", s.handleTOTPDisable)

		settingsWrite.GET("/settings/certs", s.handleGetCertInfo)
		settingsWrite.POST("/settings/certs/regenerate", s.handleRegenerateCert)
		settingsWrite.POST("/settings/certs/upload", s.handleUploadCert)

		settingsWrite.POST("/api/modules", s.handleModulesUpload)
		settingsWrite.DELETE("/api/modules/:name", s.handleModulesDelete)
	}

	auth.GET("/settings/totp/status", s.handleTOTPStatus)
	auth.GET("/api/me", s.handleGetCurrentUser)
	auth.POST("/api/auth/extend", s.handleExtendSession)
	auth.GET("/settings/totp/backup-codes/count", s.handleBackupCodeCount)

	auth.GET("/translations", s.handleTranslationsPage)
	auth.GET("/api/translations", s.handleGetTranslations)
	auth.GET("/api/translations/stats", s.handleTranslationStats)
	auth.GET("/api/translations/check", s.handleTranslationCheck)
}

// registerExtendedRoutes registers packer/mesh/chat/phishing-adjacent and other product API routes.
func (s *Server) registerExtendedRoutes(auth *gin.RouterGroup) {
	extRead := auth.Group("/")
	extRead.Use(middleware.RequirePermission(db.PermAgentsRead))
	{
		extRead.GET("/packer/templates", s.handleAPIPackerTemplates)
		extRead.GET("/packer/info", s.handleAPIPackerInfo)
		// Exposes server configuration internals — settings.read.
		extRead.GET("/api/settings", middleware.RequirePermission(db.PermSettingsRead), s.handleAPISettings)
		extRead.GET("/mesh/topology", s.handleAPIMeshTopology)
		extRead.GET("/translations/stats", s.handleAPITranslationsStats)
		extRead.GET("/api/privesc/results", s.handleAPIPrivesc)
		extRead.GET("/timeline/events", s.handleAPITimelineData)
		extRead.GET("/chat/history", s.handleAPIChatHistory)
		extRead.GET("/chat/channels", s.handleAPIChatChannels)
		extRead.GET("/chain/graph", s.handleAPIChainGraph)
		extRead.GET("/chain", s.handleAPIChainList)
		extRead.GET("/domain-fronting", s.handleAPIDomainFronting)
		extRead.GET("/rportfwd/sessions", s.handleAPIRPortFwdStatus)
		extRead.GET("/stager/tokens", s.handleAPIStagerTokens)
		extRead.GET("/ntlm/relay_status", s.handleNTLMRelayStatus)
		extRead.GET("/api/container/status", s.handleContainerStatus)
		extRead.GET("/api/container/agents", s.handleContainerAgents)
		extRead.GET("/extc2/channels", s.handleListExtC2Channels)
		extRead.GET("/extc2/configs", s.handleListExtC2Configs)
	}
	extWrite := auth.Group("/")
	extWrite.Use(middleware.RequirePermission(db.PermAgentsWrite))
	{
		extWrite.POST("/groups", s.handleAPICreateGroup)
		extWrite.PUT("/groups/:id", s.handleAPIUpdateGroup)
		extWrite.DELETE("/groups/:id", s.handleAPIDeleteGroup)
		extWrite.POST("/packer/artifact", s.handlePackerArtifact)
		extWrite.POST("/payload/bundle", s.handlePackerBundle)
		extWrite.POST("/mesh/route/:agentId", s.handleMeshRoute)
		extWrite.POST("/chat/send", s.handleAPISendChatMessage)
		extWrite.POST("/infra/front/list", s.handleAPIInfraFrontList)
		extWrite.POST("/infra/front/check", s.handleAPIInfraFrontCheck)
		extWrite.POST("/infra/front/config", s.handleAPIInfraFrontConfig)
		extWrite.POST("/stager/register", s.handleAPIStagerRegister)
		extWrite.DELETE("/stager/:id", s.handleAPIStagerDelete)
		extWrite.POST("/token/revert", s.handleAPITokenRevert)
		extWrite.POST("/extc2/discord", s.handleConfigureDiscordC2)
		extWrite.POST("/extc2/slack", s.handleConfigureSlackC2)
		extWrite.DELETE("/extc2/configs/:id", s.handleDeleteExtC2Config)
	}

	groupsWrite := auth.Group("/")
	groupsWrite.Use(middleware.RequirePermission(db.PermGroupsWrite))
	{
		groupsWrite.GET("/groups", s.handleAPIGroups)
	}

	workflowsRead := auth.Group("/")
	workflowsRead.Use(middleware.RequirePermission(db.PermAutomationRead))
	{
		workflowsRead.GET("/workflows", s.handleAPIWorkflows)
		workflowsRead.GET("/workflows/:id", s.handleAPIWorkflowsDetail)
		workflowsRead.GET("/workflows/:id/executions", s.handleListWorkflowExecutions)
		workflowsRead.GET("/workflows/:id/executions/:executionId", s.handleGetWorkflowExecution)
	}
	workflowsWrite := auth.Group("/")
	workflowsWrite.Use(middleware.RequirePermission(db.PermAutomationWrite))
	{
		workflowsWrite.POST("/workflows", s.handleAPICreateWorkflow)
		workflowsWrite.PUT("/workflows/:id", s.handleAPIUpdateWorkflow)
		workflowsWrite.DELETE("/workflows/:id", s.handleAPIDeleteWorkflow)
		workflowsWrite.POST("/workflows/:id/toggle", s.handleAPIWorkflowsToggle)
		workflowsWrite.POST("/workflows/:id/execute", s.handleAPIWorkflowsExecute)
	}

	phishingRead := auth.Group("/")
	phishingRead.Use(middleware.RequirePermission(db.PermCampaignsRead))
	{
		phishingRead.GET("/phishing/templates", s.handleAPIPhishingTemplates)
		phishingRead.GET("/phishing/campaigns", s.handleAPIPhishingCampaigns)
		phishingRead.GET("/phishing/captures", s.handleAPIPhishingCaptures)
	}
	phishingWrite := auth.Group("/")
	phishingWrite.Use(middleware.RequirePermission(db.PermCampaignsWrite))
	{
		phishingWrite.POST("/phishing/templates", s.handleAPICreatePhishingTemplate)
		phishingWrite.PUT("/phishing/templates/:id", s.handleAPIUpdatePhishingTemplate)
		phishingWrite.DELETE("/phishing/templates/:id", s.handleAPIDeletePhishingTemplate)
		phishingWrite.POST("/phishing/campaigns", s.handleAPICreatePhishingCampaign)
		phishingWrite.POST("/phishing/campaigns/:id/launch", s.handleAPILaunchPhishingCampaign)
		phishingWrite.POST("/phishing/campaigns/:id/stop", s.handleAPIStopPhishingCampaign)
		phishingWrite.DELETE("/phishing/campaigns/:id", s.handleAPIDeletePhishingCampaign)
	}

	cbRead := auth.Group("/")
	cbRead.Use(middleware.RequirePermission(db.PermOpsecRead))
	{
		cbRead.GET("/circuit-breaker/detail", s.handleAPICircuitBreakerDetail)
		cbRead.GET("/circuit-breaker/config", s.handleAPICircuitBreakerConfig)
		cbRead.GET("/circuit-breaker/events", s.handleAPICircuitBreakerEvents)
	}
	cbWrite := auth.Group("/")
	cbWrite.Use(middleware.RequirePermission(db.PermOpsecWrite))
	{
		cbWrite.POST("/circuit-breaker/config", s.handleAPICircuitBreakerSaveConfig)
		cbWrite.POST("/circuit-breaker/reset/:id", s.handleAPICircuitBreakerReset)
		cbWrite.POST("/circuit-breaker/toggle/:id", s.handleAPICircuitBreakerToggle)
	}

	tagsWrite := auth.Group("/")
	tagsWrite.Use(middleware.RequirePermission(db.PermAgentsWrite))
	{
		tagsWrite.GET("/api/tags", s.handleAPITagList)
		tagsWrite.POST("/api/tags", s.handleAPITagCreate)
		tagsWrite.PUT("/api/tags/:id", s.handleAPITagUpdate)
		tagsWrite.DELETE("/api/tags/:id", s.handleAPITagDelete)
		tagsWrite.GET("/api/agents/:id/tags", s.handleAgentTags)
		tagsWrite.POST("/agents/batch/tags", s.handleBatchAgentTags)
	}
}

// registerMonitorRoutes registers traffic, monitor/alert, and opsec guard routes.
func (s *Server) registerMonitorRoutes(auth *gin.RouterGroup) {
	monRead := auth.Group("/")
	monRead.Use(middleware.RequirePermission(db.PermAgentsRead))
	{
		monRead.GET("/builds", s.handleBuildLogs)
		monRead.GET("/builds/:id/download", s.handleDownloadBuild)
		monRead.GET("/traffic", s.handleTrafficPage)
		monRead.GET("/api/traffic", s.handleTrafficData)
		monRead.GET("/agents/:id/traffic-profile", s.handleTrafficProfileGet)
		monRead.GET("/api/monitor/metrics", s.handleGetSystemMetrics)
		monRead.GET("/api/monitor/metrics/history", s.handleGetMetricsHistory)
		monRead.GET("/api/monitor/alerts", s.handleGetAlerts)
		monRead.GET("/api/monitor/alerts/stats", s.handleGetAlertStats)
		monRead.GET("/api/monitor/alert-rules", s.handleGetAlertRules)
		monRead.GET("/api/monitor/agent-status", s.handleGetAgentStatus)
	}

	monWrite := auth.Group("/")
	monWrite.Use(middleware.RequirePermission(db.PermAgentsWrite))
	{
		monWrite.POST("/agents/:id/traffic-profile/adapt", s.handleTrafficProfileAdapt)
		monWrite.POST("/agents/:id/traffic-profile/auto-adapt", s.handleTrafficProfileAutoAdapt)
		monWrite.POST("/api/monitor/alert-rules", s.handleCreateAlertRule)
		monWrite.PUT("/api/monitor/alert-rules/:id", s.handleUpdateAlertRule)
		monWrite.DELETE("/api/monitor/alert-rules/:id", s.handleDeleteAlertRule)
		monWrite.POST("/api/monitor/alerts/:id/acknowledge", s.handleAcknowledgeAlert)
		monWrite.POST("/api/monitor/alerts/:id/resolve", s.handleResolveAlert)
	}

	opsecRead := auth.Group("/")
	opsecRead.Use(middleware.RequirePermission(db.PermOpsecRead))
	{
		opsecRead.POST("/api/opsec/check", s.handleOpsecCheck)
		opsecRead.GET("/api/opsec/rules", s.handleOpsecRulesList)
		opsecRead.GET("/api/circuit-breaker/status", s.handleCircuitBreakerStatus)
		opsecRead.GET("/api/opsec/rekey", s.handleGetRekeyStats)
	}
}

// registerDashboardCharts registers dashboard chart API routes.
func (s *Server) registerDashboardCharts(auth *gin.RouterGroup) {
	dashRead := auth.Group("/")
	dashRead.Use(middleware.RequirePermission(db.PermAgentsRead))
	dashRead.Use(middleware.CacheControl(5))
	{
		dashRead.GET("/api/dashboard/activity-heatmap", s.handleDashboardActivityHeatmap)
		dashRead.GET("/api/dashboard/os-distribution", s.handleDashboardOSDistribution)
		dashRead.GET("/api/dashboard/task-status", s.handleDashboardTaskStatus)
		dashRead.GET("/api/dashboard/listener-traffic", s.handleDashboardListenerTraffic)
		dashRead.GET("/api/dashboard/credential-types", s.handleDashboardCredentialTypes)
		dashRead.GET("/api/dashboard/agent-geo", s.handleDashboardAgentGeo)
		dashRead.GET("/api/dashboard/task-gantt", s.handleDashboardTaskGantt)
		dashRead.GET("/api/dashboard/attack-path", s.handleDashboardAttackPath)
		dashRead.GET("/api/dashboard/active-missions", s.handleActiveMissions)
	}
}

// registerBOFRoutes registers BOF management routes.
func (s *Server) registerBOFRoutes(auth *gin.RouterGroup) {
	bofRead := auth.Group("/")
	bofRead.Use(middleware.RequirePermission(db.PermAgentsRead))
	{
		bofRead.GET("/bof", s.handleBOFPage)
		bofRead.GET("/api/bof/list", s.handleBOFList)
		bofRead.GET("/api/bof/:id/download", s.handleBOFDownload)
		bofRead.GET("/api/bof/results", s.handleBOFRecentResults)
	}
	bofWrite := auth.Group("/")
	bofWrite.Use(middleware.RequirePermission(db.PermAgentsWrite))
	{
		bofWrite.POST("/api/bof/upload", s.handleBOFUpload)
		bofWrite.POST("/api/bof/:id/run", s.handleBOFRun)
		bofWrite.POST("/api/bof/:id/edit", s.handleBOFEdit)
		bofWrite.DELETE("/api/bof/:id", s.handleBOFDelete)
		bofWrite.POST("/agents/:id/bof/quick", s.handleBOFQuickRun)
		bofWrite.POST("/api/bof/repos/:id/rate", s.handleBOFRepoRate)
	}
}

// registerMiscRoutes registers update check, profile rotation, and stage/payload/screenshot serving routes.
func (s *Server) registerMiscRoutes(auth *gin.RouterGroup) {
	auth.GET("/api/update-check", s.handleUpdateCheck)
	auth.GET("/api/update-check/version", s.handleCheckVersion)
	auth.POST("/api/update-check/refresh", s.handleRefreshUpdateCheck)

	auth.POST("/api/update-check/hot-update", middleware.RequireRole(db.RoleAdmin), s.handleHotUpdate)

	miscWrite := auth.Group("/")
	miscWrite.Use(middleware.RequirePermission(db.PermAgentsWrite))
	{
		miscWrite.POST("/api/agents/:id/profile-rotate", s.handleProfileRotate)
	}

	s.router.GET("/stage/:token", s.handleServeStage)
	s.router.GET("/screenshots/:agent_id/:filename", middleware.AuthRequired(s.db), middleware.RequirePermission(db.PermAgentsRead), s.handleServeScreenshot)
}

// registerAutomationRoutes registers automation rules and BOF repository routes.
func (s *Server) registerAutomationRoutes(auth *gin.RouterGroup) {
	autoRead := auth.Group("/")
	autoRead.Use(middleware.RequirePermission(db.PermAutomationRead))
	{
		autoRead.GET("/automation", s.handleAutomationPage)
		autoRead.GET("/api/automation/rules", s.handleListAutomationRules)
		autoRead.GET("/api/webhooks", s.handleListWebhooks)
		autoRead.GET("/bof_repo", func(c *gin.Context) {
			s.renderPageOrJSON(c, gin.H{"Title": "BOF Repository", "ActiveNav": "bof_repo"})
		})
		autoRead.GET("/api/bof/repos", s.handleBOFRepoIndex)
	}
	autoWrite := auth.Group("/")
	autoWrite.Use(middleware.RequirePermission(db.PermAutomationWrite))
	{
		autoWrite.POST("/api/automation/rules", s.handleSaveAutomationRule)
		autoWrite.PUT("/api/automation/rules/:id", s.handleUpdateAutomationRule)
		autoWrite.DELETE("/api/automation/rules/:id", s.handleDeleteAutomationRule)
		autoWrite.POST("/api/automation/rules/:id/toggle", s.handleToggleAutomationRule)
		autoWrite.POST("/api/webhooks", s.handleCreateWebhook)
		autoWrite.DELETE("/api/webhooks/:id", s.handleDeleteWebhook)
		autoWrite.POST("/api/webhooks/test", s.handleTestWebhook)
		autoWrite.POST("/api/bof/repos/import", s.handleBOFRepoImport)
	}
}

// registerPluginRoutes registers plugin management and execution routes.
func (s *Server) registerPluginRoutes(auth *gin.RouterGroup) {
	pluginsRead := auth.Group("/api/plugins")
	pluginsRead.Use(middleware.RequirePermission(db.PermPluginsRead))
	pluginsRead.GET("", s.handlePluginList)
	pluginsRead.GET("/update-summary", s.handlePluginUpdateSummary)
	pluginsRead.GET("/:id", s.handlePluginGet)
	pluginsRead.GET("/:id/rating", s.handlePluginRating)
	pluginsRead.GET("/:id/reviews", s.handlePluginReviews)
	pluginsRead.GET("/:id/dependencies", s.handlePluginDependencies)
	pluginsRead.GET("/:id/update-status", s.handlePluginUpdateStatus)
	pluginsRead.GET("/:id/export", s.handlePluginExport)
	pluginsRead.GET("/:id/execute", s.handlePluginExecuteInfo)

	pluginsWrite := auth.Group("/api/plugins")
	pluginsWrite.Use(middleware.RequirePermission(db.PermPluginsWrite))
	pluginsWrite.POST("", s.handlePluginCreate)
	pluginsWrite.POST("/check-updates", s.handlePluginCheckUpdates)
	pluginsWrite.POST("/import", s.handlePluginImport)
	pluginsWrite.POST("/:id/reviews", s.handlePluginAddReview)
	pluginsWrite.POST("/:id/rating", s.handlePluginRate)
	pluginsWrite.POST("/:id/update", s.handlePluginUpdate)
	pluginsWrite.POST("/:id/toggle", s.handlePluginToggle)
	pluginsWrite.POST("/:id/install", s.handlePluginInstall)
	pluginsWrite.POST("/:id/enable", s.handlePluginEnable)
	pluginsWrite.POST("/:id/disable", s.handlePluginDisable)
	pluginsWrite.POST("/:id/report", s.handlePluginReport)

	pluginsExecute := auth.Group("/api/plugins")
	pluginsExecute.Use(middleware.RequirePermission(db.PermPluginsExecute))
	pluginsExecute.POST("/:id/execute", s.handlePluginExecute)

	pluginsDelete := auth.Group("/api/plugins")
	pluginsDelete.Use(middleware.RequirePermission(db.PermPluginsDelete))
	pluginsDelete.DELETE("/:id", s.handlePluginDelete)

	auth.GET("/plugins", s.handlePluginsPage)
}

// registerTaskRoutes registers task history and logout routes.
func (s *Server) registerTaskRoutes(auth *gin.RouterGroup) {
	tasksRead := auth.Group("/")
	tasksRead.Use(middleware.RequirePermission(db.PermTasksRead))
	{
		tasksRead.GET("/tasks", s.handleTaskHistory)
		tasksRead.GET("/tasks/export", s.handleExportTasks)
		tasksRead.GET("/tasks/:taskId", s.handleGetTaskStatus)
		tasksRead.POST("/tasks/batch-status", s.handleBatchTaskStatus)
	}

	tasksWrite := auth.Group("/")
	tasksWrite.Use(middleware.RequirePermission(db.PermAgentsWrite))
	{
		tasksWrite.POST("/tasks/:taskId/approve", s.handleApproveTask)
		tasksWrite.POST("/tasks/:taskId/reject", s.handleRejectTask)
	}

	auth.POST("/logout", s.handleLogout)
}

// registerCredentialRoutes registers credential management routes.
func (s *Server) registerCredentialRoutes(auth *gin.RouterGroup) {
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
		credsWrite.POST("/credentials/:cred_id/usage", s.apiRecordUsage)
	}
	credsDelete := auth.Group("/")
	credsDelete.Use(middleware.RequirePermission(db.PermCredsDelete))
	{
		credsDelete.DELETE("/credentials/:cred_id", s.handleDeleteCredential)
	}
}

// registerUserRoutes registers documentation, AI, WebSocket, tokens, user management, SOCKS sessions, and scripting routes.
func (s *Server) registerUserRoutes(auth *gin.RouterGroup) {
	userRead := auth.Group("/")
	userRead.Use(middleware.RequirePermission(db.PermSettingsRead))
	{
		userRead.GET("/docs", s.handleDocsPage)
		userRead.GET("/api/docs", s.handleAPIDocsRedirect)
		userRead.GET("/api/docs/", s.handleAPIDocs)
		userRead.GET("/api/docs/openapi.yaml", s.handleAPIDocsYAML)
		userRead.GET("/ai", s.handleAIPage)
		userRead.GET("/ai/sessions", s.handleAISessionsList)
		userRead.GET("/ai/sessions/:id/messages", s.handleAISessionsGet)
		userRead.GET("/tokens", s.handleGlobalTokensPage)
		userRead.GET("/socks/sessions", s.handleGetSocksSessions)
		userRead.GET("/scripting", s.handleScriptingPage)
		userRead.GET("/api/scripts", s.handleAPIGetScripts)
		userRead.GET("/api/scripts/history", s.handleAPIScriptsHistory)
	}
	userWrite := auth.Group("/")
	userWrite.Use(middleware.RequirePermission(db.PermSettingsWrite))
	{
		userWrite.POST("/ai/chat", s.handleAIChat)
		userWrite.POST("/ai/config", s.handleAIConfig)
		userWrite.POST("/ai/sessions", s.handleAISessionsCreate)
		userWrite.POST("/ai/sessions/:id/messages", s.handleAISessionsMessages)
		userWrite.PUT("/ai/sessions/:id", s.handleAISessionsUpdate)
		userWrite.DELETE("/ai/sessions/:id", s.handleAISessionsDelete)
		userWrite.POST("/api/scripts", s.handleAPISaveScript)
		userWrite.DELETE("/api/scripts/:id", s.handleAPIDeleteScript)
		userWrite.POST("/api/scripts/execute", s.handleAPIExecuteScript)
	}

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
		usersWrite.POST("/users/:id/force-logout", s.handleForceLogoutUser)
		usersWrite.GET("/users/:id/sessions", s.handleListUserSessions)
		usersWrite.POST("/users/:id/sessions/:sessionId/revoke", s.handleRevokeSession)
		usersWrite.POST("/users/:id/sessions/revoke-all", s.handleRevokeAllUserSessions)
	}
	usersDelete := auth.Group("/")
	usersDelete.Use(middleware.RequirePermission(db.PermUsersDelete))
	{
		usersDelete.DELETE("/users/:id", s.handleDeleteUser)
	}
}

// registerCampaignRoutes registers campaigns, notifications, redirectors, roles, and collab routes.
func (s *Server) registerCampaignRoutes(auth *gin.RouterGroup) {
	campaignsRead := auth.Group("/")
	campaignsRead.Use(middleware.RequirePermission(db.PermCampaignsRead))
	{
		campaignsRead.GET("/campaigns", s.handleCampaignsList)
		campaignsRead.GET("/campaigns/:id", s.handleCampaignGet)
		campaignsRead.GET("/campaigns/:id/mitre", s.handleCampaignMitre)
		campaignsRead.GET("/mitre/templates", s.handleMitreTemplates)
		campaignsRead.GET("/mitre/timeline", s.handleMitreTimeline)
		campaignsRead.GET("/mitre/phases", s.handleMitrePhases)
		campaignsRead.GET("/attack/coverage", s.handleAttackCoverage)
	}
	campaignsWrite := auth.Group("/")
	campaignsWrite.Use(middleware.RequirePermission(db.PermCampaignsWrite))
	{
		campaignsWrite.POST("/campaigns", s.handleCampaignCreate)
		campaignsWrite.POST("/campaigns/:id", s.handleCampaignUpdate)
		campaignsWrite.DELETE("/campaigns/:id", s.handleCampaignDelete)
		campaignsWrite.POST("/campaigns/:id/killchain", s.handleCampaignKillChain)
	}

	notificationsRead := auth.Group("/")
	notificationsRead.Use(middleware.RequirePermission(db.PermNotificationsRead))
	{
		notificationsRead.GET("/notifications", s.handleListNotifications)
	}
	notificationsWrite := auth.Group("/")
	notificationsWrite.Use(middleware.RequirePermission(db.PermNotificationsWrite))
	{
		notificationsWrite.PUT("/notifications/:id/read", s.handleMarkNotificationRead)
		notificationsWrite.PUT("/notifications/read-all", s.handleMarkAllNotificationsRead)
		notificationsWrite.DELETE("/notifications/:id", s.handleDeleteNotification)
		notificationsWrite.DELETE("/notifications", s.handleClearAllNotifications)
	}

	redirectorRead := auth.Group("/")
	redirectorRead.Use(middleware.RequirePermission(db.PermSettingsRead))
	{
		redirectorRead.GET("/redirectors", s.handleRedirectorList)
	}
	redirectorWrite := auth.Group("/")
	redirectorWrite.Use(middleware.RequirePermission(db.PermSettingsWrite))
	{
		redirectorWrite.POST("/redirectors", s.handleRedirectorCreate)
		redirectorWrite.PUT("/redirectors/:id", s.handleRedirectorUpdate)
		redirectorWrite.DELETE("/redirectors/:id", s.handleRedirectorDelete)
		redirectorWrite.POST("/redirectors/test-ssh", s.handleRedirectorTestSSH)
		redirectorWrite.POST("/redirectors/generate/:type", s.handleRedirectorGenerate)
		redirectorWrite.POST("/redirectors/deploy-ssh", s.handleRedirectorDeploySSH)
	}

	rolesRead := auth.Group("/")
	rolesRead.Use(middleware.RequirePermission(db.PermRolesRead))
	{
		rolesRead.GET("/api/roles", s.handleRolesList)
	}
	rolesWrite := auth.Group("/")
	rolesWrite.Use(middleware.RequirePermission(db.PermRolesWrite))
	{
		rolesWrite.POST("/api/roles", s.handleRolesCreate)
		rolesWrite.POST("/api/roles/:id", s.handleRolesUpdate)
		rolesWrite.DELETE("/api/roles/:id", s.handleRolesDelete)
	}

	collabRead := auth.Group("/")
	collabRead.Use(middleware.RequirePermission(db.PermAgentsRead))
	{
		collabRead.GET("/collab/agents", s.handleCollabAgents)
	}
	collabWrite := auth.Group("/")
	collabWrite.Use(middleware.RequirePermission(db.PermAgentsWrite))
	{
		collabWrite.POST("/collab/agents/:id/lock", s.handleCollabLock)
		collabWrite.POST("/collab/agents/:id/unlock", s.handleCollabUnlock)
		collabWrite.POST("/collab/tasks/:taskId/claim", s.handleCollabClaimTask)
		collabWrite.POST("/collab/tasks/:taskId/release", s.handleCollabReleaseTask)
	}
}

// registerIntegrationRoutes registers BloodHound, AutoTag, OPSEC, cloud sync, Chrome agents, integrations, rportfwd, agent link, and remote desktop routes.
func (s *Server) registerIntegrationRoutes(auth *gin.RouterGroup) {
	bhRead := auth.Group("/")
	bhRead.Use(middleware.RequirePermission(db.PermIntelRead))
	{
		bhRead.GET("/bloodhound/list", s.handleBloodHoundList)
		bhRead.GET("/bloodhound/status", s.handleBloodHoundStatus)
		bhRead.GET("/bloodhound/:id/download", s.handleBloodHoundDownload)
	}
	bhWrite := auth.Group("/")
	bhWrite.Use(middleware.RequirePermission(db.PermIntelWrite))
	{
		bhWrite.POST("/bloodhound/collect", s.handleBloodHoundCollect)
		bhWrite.DELETE("/bloodhound/:id", s.handleBloodHoundDelete)
		bhWrite.POST("/bloodhound/upload", s.handleBloodHoundUpload)
		bhWrite.POST("/bloodhound/result", s.handleBloodHoundResult)
	}

	autoTagRead := auth.Group("/")
	autoTagRead.Use(middleware.RequirePermission(db.PermSettingsRead))
	{
		autoTagRead.GET("/api/autotag/rules", s.handleAutoTagRules)
	}
	autoTagWrite := auth.Group("/")
	autoTagWrite.Use(middleware.RequirePermission(db.PermSettingsWrite))
	{
		autoTagWrite.POST("/api/autotag/rules", s.handleAutoTagCreate)
		autoTagWrite.PUT("/api/autotag/rules/:id", s.handleAutoTagUpdate)
		autoTagWrite.POST("/api/autotag/rules/:id/toggle", s.handleAutoTagToggle)
		autoTagWrite.DELETE("/api/autotag/rules/:id", s.handleAutoTagDelete)
		autoTagWrite.POST("/api/autotag/apply", s.handleAutoTagApply)
	}
	opsecRead := auth.Group("/")
	opsecRead.Use(middleware.RequirePermission(db.PermOpsecRead))
	{
		opsecRead.GET("/opsec/history", s.handleOpsecHistory)
	}
	opsecWrite := auth.Group("/")
	opsecWrite.Use(middleware.RequirePermission(db.PermOpsecWrite))
	{
		opsecWrite.POST("/opsec/rules", s.handleOpsecRuleCreate)
		opsecWrite.DELETE("/opsec/rules/:name", s.handleOpsecRuleDelete)
	}

	intelRead := auth.Group("/")
	intelRead.Use(middleware.RequirePermission(db.PermIntelRead))
	{
		intelRead.GET("/cloud/:agentId/results", s.handleCloudResults)
		intelRead.GET("/api/chrome/agents", s.handleChromeAgents)
	}
	intelWrite := auth.Group("/")
	intelWrite.Use(middleware.RequirePermission(db.PermIntelWrite))
	{
		intelWrite.POST("/cloud/steal", s.handleCloudSteal)
		intelWrite.POST("/chrome/agents/:uuid/tasks", s.handleChromeAgentTask)
	}

	integrationRead := auth.Group("/")
	integrationRead.Use(middleware.RequirePermission(db.PermSettingsRead))
	{
		integrationRead.GET("/integrations", s.handleIntegrationsList)
		integrationRead.GET("/integrations/malleable", s.handleActiveMalleable)
		integrationRead.GET("/rportfwd/status", s.handleRPortFwdGlobalStatus)
	}
	integrationWrite := auth.Group("/")
	integrationWrite.Use(middleware.RequirePermission(db.PermSettingsWrite))
	{
		integrationWrite.POST("/integrations", s.handleIntegrationsCreate)
		integrationWrite.PUT("/integrations/:id", s.handleIntegrationsUpdate)
		integrationWrite.POST("/integrations/:id/toggle", s.handleIntegrationsToggle)
		integrationWrite.DELETE("/integrations/:id", s.handleIntegrationsDelete)
	}

	linkWrite := auth.Group("/")
	linkWrite.Use(middleware.RequirePermission(db.PermAgentsWrite))
	{
		linkWrite.POST("/agents/:id/link", s.handleLinkAgent)
		linkWrite.POST("/agents/:id/unlink", s.handleUnlinkAgent)
	}

	auth.GET("/ws/remote-desktop", s.handleRDWebSocket)
	auth.GET("/rd/:id/frame", s.handleRDAPIGetFrame)
	auth.POST("/rd/:id/screenshot", s.handleRDAPIScreenshot)
}

// registerAPIKeyRoutes registers API key management routes (admin only).
func (s *Server) registerAPIKeyRoutes(auth *gin.RouterGroup) {
	apiKeys := auth.Group("/")
	apiKeys.Use(middleware.RequirePermission(db.PermSettingsWrite))
	{
		apiKeys.GET("/api/api-keys", s.handleListAPIKeys)
		apiKeys.POST("/api/api-keys", s.handleCreateAPIKey)
		apiKeys.DELETE("/api/api-keys/:id", s.handleRevokeAPIKey)
		apiKeys.POST("/api/api-keys/:id/rotate", s.handleRotateAPIKey)
	}
}
