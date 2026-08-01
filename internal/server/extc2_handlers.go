package server

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleConfigureDiscordC2(c *gin.Context) {
	var req struct {
		BotToken  string `json:"bot_token" binding:"required"`
		ChannelID string `json:"channel_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "bot_token and channel_id are required")
		return
	}

	channel := db.ExtC2Channel{
		Type:      "discord",
		BotToken:  req.BotToken,
		ChannelID: req.ChannelID,
		Enabled:   true,
	}
	if err := s.db.Create(&channel).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Save config"))
		return
	}

	discord := NewDiscordExternalC2(s, req.BotToken, req.ChannelID)
	if err := discord.Start(); err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Start Discord C2"))
		return
	}

	s.LogAuditRecord(c, "extc2_discord_configure", "extc2", req.ChannelID, "Discord External C2 configured", true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true, "channel_id": req.ChannelID, "id": channel.ID})
}

func (s *Server) handleConfigureSlackC2(c *gin.Context) {
	var req struct {
		BotToken  string `json:"bot_token" binding:"required"`
		ChannelID string `json:"channel_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "bot_token and channel_id are required")
		return
	}

	channel := db.ExtC2Channel{
		Type:      "slack",
		BotToken:  req.BotToken,
		ChannelID: req.ChannelID,
		Enabled:   true,
	}
	if err := s.db.Create(&channel).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Save config"))
		return
	}

	slackC2 := NewSlackExternalC2(s, req.BotToken, req.ChannelID)
	if err := slackC2.Start(); err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Start Slack C2"))
		return
	}

	s.LogAuditRecord(c, "extc2_slack_configure", "extc2", req.ChannelID, "Slack External C2 configured", true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true, "channel_id": req.ChannelID, "id": channel.ID})
}

func (s *Server) handleListExtC2Configs(c *gin.Context) {
	var channels []db.ExtC2Channel
	if err := s.db.Limit(500).Find(&channels).Error; err != nil {
		slog.Error("Failed to list external C2 channels", "err", err)
	}
	for i := range channels {
		channels[i].BotToken = "***REDACTED***"
	}
	c.JSON(http.StatusOK, gin.H{"channels": channels})
}

func (s *Server) handleDeleteExtC2Config(c *gin.Context) {
	id := c.Param("id")
	var channel db.ExtC2Channel
	if err := s.db.First(&channel, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "Channel not found")
		return
	}

	channelKey := fmt.Sprintf("extc2-%s-%s", channel.Type, channel.ChannelID)
	s.extC2ChannelsMu.Lock()
	delete(s.extC2Channels, channelKey)
	s.extC2ChannelsMu.Unlock()

	if err := s.db.Delete(&channel).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "External C2 delete"))
		return
	}
	s.LogAuditRecord(c, "extc2_config_delete", "extc2", channel.ChannelID, fmt.Sprintf("Deleted %s External C2 config", channel.Type), true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) restoreExtC2Channels() {
	var channels []db.ExtC2Channel
	if err := s.db.Where("enabled = ?", true).Limit(500).Find(&channels).Error; err != nil {
		slog.Error("Failed to restore external C2 channels", "err", err)
	}

	for _, ch := range channels {
		switch ch.Type {
		case "discord":
			discord := NewDiscordExternalC2(s, ch.BotToken, ch.ChannelID)
			if err := discord.Start(); err != nil {
				slog.Error("Failed to restore Discord C2", "channel_id", ch.ChannelID, "error", err)
			} else {
				slog.Info("Restored Discord C2 channel", "channel_id", ch.ChannelID)
			}
		case "slack":
			slackC2 := NewSlackExternalC2(s, ch.BotToken, ch.ChannelID)
			if err := slackC2.Start(); err != nil {
				slog.Error("Failed to restore Slack C2", "channel_id", ch.ChannelID, "error", err)
			} else {
				slog.Info("Restored Slack C2 channel", "channel_id", ch.ChannelID)
			}
		}
	}
}
