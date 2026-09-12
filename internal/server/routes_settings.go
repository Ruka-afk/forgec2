package server

import (
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

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
		settingsRead.GET("/config/reload-status", s.handleReloadStatus)
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
		// Update-signing trust root: admin-only, signatures authorise code
		// execution on every pinned implant.
		settingsWrite.GET("/update-signing/public-key", middleware.RequireRole(db.RoleAdmin), s.handleUpdateSigningKey)
		settingsWrite.POST("/update-signing/sign", middleware.RequireRole(db.RoleAdmin), s.handleSignUpdate)
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
