package server

import (
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

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
