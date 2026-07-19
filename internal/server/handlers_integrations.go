package server

import (
	"github.com/gin-gonic/gin"
)

// handleIntegrationsList returns the set of integrations the server supports
// along with their enabled/configured status.
func (s *Server) handleIntegrationsList(c *gin.Context) {
	slackCfg := s.cfg.Integrations.Slack
	integrations := []map[string]interface{}{
		{
			"type":        "slack",
			"name":        "Slack",
			"enabled":     slackCfg.Enabled,
			"configured":  slackCfg.BotToken != "" && slackCfg.AppToken != "",
			"description": "Manage agents and dispatch tasks from Slack via Socket Mode.",
		},
		{
			"type":        "webhook",
			"name":        "Webhook",
			"enabled":     true,
			"configured":  true,
			"description": "Generic webhook delivery for events and notifications.",
		},
		{
			"type":        "email",
			"name":        "Email",
			"enabled":     false,
			"configured":  false,
			"description": "Send notifications and scheduled reports via SMTP.",
		},
		{
			"type":        "telegram",
			"name":        "Telegram",
			"enabled":     false,
			"configured":  false,
			"description": "Control agents and receive alerts via Telegram bot.",
		},
		{
			"type":        "discord",
			"name":        "Discord",
			"enabled":     false,
			"configured":  false,
			"description": "Send alerts and reports to a Discord channel.",
		},
		{
			"type":        "jira",
			"name":        "JIRA",
			"enabled":     false,
			"configured":  false,
			"description": "Auto-create JIRA tickets from alerts and findings.",
		},
		{
			"type":        "thehive",
			"name":        "TheHive",
			"enabled":     false,
			"configured":  false,
			"description": "Create TheHive alerts from C2 events.",
		},
	}
	respond(c, gin.H{"integrations": integrations})
}

// handleActiveMalleable returns the currently active malleable C2 profile.
func (s *Server) handleActiveMalleable(c *gin.Context) {
	mp := s.cfg.Malleable
	respond(c, gin.H{
		"success":     true,
		"enabled":     mp.Enabled,
		"status_code": mp.StatusCode,
		"content_type": mp.ContentType,
		"headers":     mp.Headers,
		"prepend":     mp.Prepend,
		"append":      mp.Append,
	})
}
