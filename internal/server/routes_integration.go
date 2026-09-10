package server

import (
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

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
	}
	intelWrite := auth.Group("/")
	intelWrite.Use(middleware.RequirePermission(db.PermIntelWrite))
	{
		intelWrite.POST("/cloud/steal", s.handleCloudSteal)
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
