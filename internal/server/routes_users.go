package server

import (
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func (s *Server) registerUserRoutes(auth *gin.RouterGroup) {
	userRead := auth.Group("/")
	userRead.Use(middleware.RequirePermission(db.PermSettingsRead))
	{
		userRead.GET("/docs", s.handleDocsPage)
		// Per-user saved list views (personal UI preference, any authenticated
		// operator with settings read may use them).
		userRead.GET("/api/saved-views", s.handleListSavedViews)
		userRead.GET("/api/docs", s.handleAPIDocsRedirect)
		userRead.GET("/api/docs/", s.handleAPIDocs)
		userRead.GET("/api/docs/openapi.yaml", s.handleAPIDocsYAML)
		userRead.GET("/tokens", s.handleGlobalTokensPage)
		userRead.GET("/socks/sessions", s.handleGetSocksSessions)
		userRead.GET("/scripting", s.handleScriptingPage)
		userRead.GET("/api/scripts", s.handleAPIGetScripts)
		userRead.GET("/api/scripts/history", s.handleAPIScriptsHistory)
	}
	userWrite := auth.Group("/")
	userWrite.Use(middleware.RequirePermission(db.PermSettingsWrite))
	{
		userWrite.POST("/api/saved-views", s.handleCreateSavedView)
		userWrite.DELETE("/api/saved-views/:id", s.handleDeleteSavedView)
		userWrite.POST("/api/scripts", s.handleAPISaveScript)
		userWrite.DELETE("/api/scripts/:id", s.handleAPIDeleteScript)
		userWrite.POST("/api/scripts/execute", s.handleAPIExecuteScript)
	}

	aiUse := auth.Group("/")
	aiUse.Use(middleware.RequirePermission(db.PermAIUse))
	{
		aiUse.GET("/ai", s.handleAIPage)
		aiUse.POST("/ai/chat", s.handleAIChat) // compatibility endpoint
		aiUse.GET("/ai/sessions", s.handleAISessionsList)
		aiUse.POST("/ai/sessions", s.handleAISessionsCreate)
		aiUse.GET("/ai/sessions/:id/messages", s.handleAISessionsGet)
		aiUse.POST("/ai/sessions/:id/messages", s.handleAISessionsMessages)
		aiUse.POST("/ai/sessions/:id/branch", s.handleAISessionBranch)
		aiUse.PUT("/ai/sessions/:id", s.handleAISessionsUpdate)
		aiUse.DELETE("/ai/sessions/:id", s.handleAISessionsDelete)

		aiUse.POST("/api/ai/runs", s.handleAIRunsCreate)
		aiUse.GET("/api/ai/runs", s.handleAIRunsList)
		aiUse.GET("/api/ai/runs/:id", s.handleAIRunsGet)
		aiUse.GET("/api/ai/runs/:id/events", s.handleAIRunEvents)
		aiUse.POST("/api/ai/runs/:id/cancel", s.handleAIRunCancel)
		aiUse.GET("/api/ai/intents", s.handleAIIntentsList)
		aiUse.POST("/api/ai/intents/:id/approve", s.handleAIIntentApprove)
		aiUse.POST("/api/ai/intents/:id/reject", s.handleAIIntentReject)
		aiUse.GET("/api/ai/profiles", s.handleAIProfilesList)
		aiUse.GET("/api/ai/sessions/:id/attachments", s.handleAIAttachmentsList)
		aiUse.POST("/api/ai/sessions/:id/attachments", s.handleAIAttachmentsUpload)
		aiUse.DELETE("/api/ai/attachments/:attachmentID", s.handleAIAttachmentDelete)
		aiUse.GET("/api/ai/knowledge/collections", s.handleAIKnowledgeCollectionsList)
		aiUse.POST("/api/ai/knowledge/collections", s.handleAIKnowledgeCollectionCreate)
		aiUse.DELETE("/api/ai/knowledge/collections/:collectionID", s.handleAIKnowledgeCollectionDelete)
		aiUse.GET("/api/ai/knowledge/collections/:collectionID/sources", s.handleAIKnowledgeSourcesList)
		aiUse.DELETE("/api/ai/knowledge/collections/:collectionID/sources/:sourceID", s.handleAIKnowledgeSourceDelete)
		aiUse.POST("/api/ai/knowledge/collections/:collectionID/attachments/:attachmentID", s.handleAIKnowledgePromoteAttachment)
		aiUse.POST("/api/ai/knowledge/search", s.handleAIKnowledgeSearch)
	}

	aiConfigure := auth.Group("/")
	aiConfigure.Use(middleware.RequirePermission(db.PermAIConfigure))
	{
		aiConfigure.POST("/ai/config", s.handleAIConfig)
		aiConfigure.POST("/api/ai/profiles", s.handleAIProfileCreate)
		aiConfigure.PUT("/api/ai/profiles/:id", s.handleAIProfileUpdate)
		aiConfigure.DELETE("/api/ai/profiles/:id", s.handleAIProfileDelete)
		aiConfigure.POST("/api/ai/profiles/:id/test", s.handleAIProfileTest)
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
