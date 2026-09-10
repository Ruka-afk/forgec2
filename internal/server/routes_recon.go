package server

import (
	"net/http/pprof"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

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
		reconRead.GET("/api/report/generated/:id", s.handleAPIGetGeneratedReport)
		reconRead.GET("/api/report/export/html", s.handleAPIExportReportHTML)
		reconRead.GET("/lateral", s.handleLateralPage)
		reconRead.GET("/api/lateral/history/:id", s.handleLateralHistory)
		reconRead.GET("/templates", s.handleTemplatesPage)
		reconRead.GET("/api/templates", s.handleListTemplatesJSON)
		reconRead.GET("/api/templates/category/:category", s.handleGetTemplatesByCategory)
		macrosRead := auth.Group("/")
		macrosRead.Use(middleware.RequirePermission(db.PermAgentsRead))
		{
			macrosRead.GET("/api/macros", s.handleListMacros)
			macrosRead.GET("/api/macro-runs", s.handleListMacroRuns)
			macrosRead.GET("/api/macro-runs/:id", s.handleGetMacroRun)
		}
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
		iocRead := auth.Group("/")
		iocRead.Use(middleware.RequirePermission(db.PermAgentsRead))
		{
			iocRead.GET("/api/ioc", s.handleListIOCs)
			iocRead.GET("/api/ioc/export", s.handleExportIOCs)
		}
		reconWrite.POST("/api/lateral/execute", s.handleAPILateralExecute)
		reconWrite.POST("/api/templates", s.handleCreateTemplate)
		reconWrite.PUT("/api/templates/:id", s.handleUpdateTemplate)
		reconWrite.DELETE("/api/templates/:id", s.handleDeleteTemplate)
		reconWrite.POST("/api/macros", s.handleCreateMacro)
		reconWrite.PUT("/api/macros/:id", s.handleUpdateMacro)
		reconWrite.DELETE("/api/macros/:id", s.handleDeleteMacro)
		reconWrite.POST("/api/macros/:id/run", s.handleRunMacro)
		reconWrite.POST("/api/macro-runs/:id/stop", s.handleStopMacroRun)
		reconWrite.POST("/api/scheduler/oneshot", s.handleCreateOneShotTask)
		reconWrite.DELETE("/api/scheduler/oneshot/:id", s.handleCancelOneShotTask)
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
