package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// severityRank orders severities so a route with min_severity=warning only
// receives warning and critical notifications.
func severityRank(sev string) int {
	switch sev {
	case "critical", "error":
		return 3
	case "warning":
		return 2
	default: // info, success, ...
		return 1
	}
}

const notificationRouteTimeout = 8 * time.Second
const notificationRouteMask = "********"

func redactNotificationRoute(route db.NotificationRoute) db.NotificationRoute {
	if route.Secret != "" {
		route.Secret = notificationRouteMask
	}
	// Telegram targets are chat IDs, but Discord and generic webhook targets
	// commonly carry credentials in their path or query string.
	if route.Channel != "telegram" && route.Target != "" {
		route.Target = notificationRouteMask
	}
	return route
}

func validateNotificationRoute(channel, target, secret string) error {
	if !validNotificationChannel(channel) {
		return errors.New("channel must be discord, telegram or webhook")
	}
	if target == "" || target == notificationRouteMask {
		return errors.New("target required")
	}
	if channel == "telegram" {
		if secret == "" || secret == notificationRouteMask {
			return errors.New("telegram bot token required")
		}
		return nil
	}
	if err := validateExternalURL(target); err != nil {
		return fmt.Errorf("invalid notification target: %w", err)
	}
	return nil
}

// DispatchNotification persists a notification and fans it out to every
// enabled notification route whose minimum severity matches. Callers should
// use this instead of creating db.Notification rows directly.
func (s *Server) DispatchNotification(n *db.Notification) {
	if s == nil || s.db == nil || n == nil {
		return
	}
	if err := s.db.Create(n).Error; err != nil {
		slog.Error("Failed to persist notification", "title", n.Title, "error", err)
		return
	}

	var routes []db.NotificationRoute
	if err := s.db.Where("enabled = ?", true).Limit(50).Find(&routes).Error; err != nil {
		slog.Error("Failed to load notification routes", "error", err)
		return
	}
	rank := severityRank(n.Severity)
	for _, route := range routes {
		if rank < severityRank(route.MinSeverity) {
			continue
		}
		go func(route db.NotificationRoute) {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("Notification route panicked", "route", route.Name, "panic", r)
				}
			}()
			_ = s.sendNotificationRoute(route, n)
		}(route)
	}
}

// sendNotificationRoute delivers one notification over one channel.
func (s *Server) sendNotificationRoute(route db.NotificationRoute, n *db.Notification) error {
	client := ssrfSafeClient(&http.Client{Timeout: notificationRouteTimeout})
	var payload []byte
	target := ""

	text := fmt.Sprintf("[%s] %s\n%s", n.Severity, n.Title, n.Message)
	switch route.Channel {
	case "discord":
		payload, _ = json.Marshal(map[string]string{"content": text})
		target = route.Target
	case "telegram":
		api := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", route.Secret)
		payload, _ = json.Marshal(map[string]string{
			"chat_id": route.Target,
			"text":    text,
		})
		target = api
	case "webhook":
		payload, _ = json.Marshal(map[string]interface{}{
			"severity": n.Severity,
			"type":     n.Type,
			"title":    n.Title,
			"message":  n.Message,
			"agent_id": n.AgentID,
			"task_id":  n.TaskID,
			"ts":       time.Now().UTC().Format(time.RFC3339),
		})
		target = route.Target
	default:
		slog.Warn("Unknown notification route channel", "channel", route.Channel)
		return fmt.Errorf("unknown notification channel %q", route.Channel)
	}
	if target == "" {
		return errors.New("notification target is empty")
	}
	// Revalidate at delivery time as well as create/update time. Existing rows
	// may predate validation, and DNS may have changed since configuration.
	if err := validateExternalURL(target); err != nil {
		slog.Warn("Notification route target rejected", "route", route.Name, "error", err)
		return fmt.Errorf("notification target rejected: %w", err)
	}

	resp, err := client.Post(target, "application/json", bytes.NewReader(payload))
	if err != nil {
		// *url.Error embeds the full URL — for Telegram/Discord that URL
		// contains the bot token / webhook secret. Log the cause only.
		var ue *url.Error
		if errors.As(err, &ue) {
			slog.Warn("Notification route delivery failed", "route", route.Name, "error", ue.Err)
		} else {
			slog.Warn("Notification route delivery failed", "route", route.Name, "error", err)
		}
		return errors.New("notification delivery failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		slog.Warn("Notification route returned error status", "route", route.Name, "status", resp.StatusCode)
		return fmt.Errorf("notification route returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// ── REST management ─────────────────────────────────────────────────────────

func (s *Server) handleListNotificationRoutes(c *gin.Context) {
	var routes []db.NotificationRoute
	if err := s.db.Order("id").Find(&routes).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "query failed")
		return
	}
	// Never echo secrets or credential-bearing webhook URLs back to the UI.
	for i := range routes {
		routes[i] = redactNotificationRoute(routes[i])
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "routes": routes})
}

func validNotificationChannel(ch string) bool {
	return ch == "discord" || ch == "telegram" || ch == "webhook"
}

func validNotificationSeverity(severity string) bool {
	return severity == "info" || severity == "warning" || severity == "critical"
}

func (s *Server) handleCreateNotificationRoute(c *gin.Context) {
	var req db.NotificationRoute
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		respondError(c, http.StatusBadRequest, "name required")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Channel = strings.TrimSpace(req.Channel)
	req.Target = strings.TrimSpace(req.Target)
	req.Secret = strings.TrimSpace(req.Secret)
	if err := validateNotificationRoute(req.Channel, req.Target, req.Secret); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	req.ID = 0
	req.Enabled = true
	if req.MinSeverity == "" {
		req.MinSeverity = "info"
	}
	if !validNotificationSeverity(req.MinSeverity) {
		respondError(c, http.StatusBadRequest, "min_severity must be info, warning or critical")
		return
	}
	if err := s.db.Create(&req).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create route")
		return
	}
	s.LogAuditRecord(c, "notification_route_create", "settings", strconv.FormatUint(uint64(req.ID), 10), req.Name+" ("+req.Channel+")", true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true, "route": redactNotificationRoute(req)})
}

func (s *Server) handleUpdateNotificationRoute(c *gin.Context) {
	id := c.Param("id")
	var existing db.NotificationRoute
	if err := s.db.First(&existing, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "route not found")
		return
	}
	var req struct {
		Name        string `json:"name"`
		Channel     string `json:"channel"`
		Target      string `json:"target"`
		Secret      string `json:"secret"`
		MinSeverity string `json:"min_severity"`
		Enabled     *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}
	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Channel != "" {
		if !validNotificationChannel(req.Channel) {
			respondError(c, http.StatusBadRequest, "invalid channel")
			return
		}
		updates["channel"] = req.Channel
	}
	if req.Target != "" {
		if req.Target != notificationRouteMask {
			updates["target"] = strings.TrimSpace(req.Target)
		}
	}
	// An empty secret keeps the stored value (the UI masks it as ********).
	if req.Secret != "" && req.Secret != notificationRouteMask {
		updates["secret"] = strings.TrimSpace(req.Secret)
	}
	if req.MinSeverity != "" {
		if !validNotificationSeverity(req.MinSeverity) {
			respondError(c, http.StatusBadRequest, "invalid min_severity")
			return
		}
		updates["min_severity"] = req.MinSeverity
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	candidateChannel := existing.Channel
	if channel, ok := updates["channel"].(string); ok {
		candidateChannel = channel
	}
	candidateTarget := existing.Target
	if target, ok := updates["target"].(string); ok {
		candidateTarget = target
	}
	candidateSecret := existing.Secret
	if secret, ok := updates["secret"].(string); ok {
		candidateSecret = secret
	}
	if err := validateNotificationRoute(candidateChannel, candidateTarget, candidateSecret); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if len(updates) == 0 {
		c.JSON(http.StatusOK, gin.H{"success": true})
		return
	}
	if err := s.db.Model(&db.NotificationRoute{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to update route")
		return
	}
	s.LogAuditRecord(c, "notification_route_update", "settings", id, "", true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleDeleteNotificationRoute(c *gin.Context) {
	id := c.Param("id")
	res := s.db.Delete(&db.NotificationRoute{}, id)
	if res.Error != nil {
		respondError(c, http.StatusInternalServerError, "failed to delete route")
		return
	}
	if res.RowsAffected == 0 {
		respondError(c, http.StatusNotFound, "route not found")
		return
	}
	s.LogAuditRecord(c, "notification_route_delete", "settings", id, "", true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// handleTestNotificationRoute sends a test message through the configured
// channel so operators can verify credentials before relying on them.
func (s *Server) handleTestNotificationRoute(c *gin.Context) {
	id := c.Param("id")
	var route db.NotificationRoute
	if err := s.db.First(&route, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "route not found")
		return
	}
	test := &db.Notification{
		Type:     "test",
		Title:    "ForgeC2 test notification",
		Message:  "If you can read this, the \"" + route.Name + "\" route works.",
		Severity: "info",
	}
	if err := s.sendNotificationRoute(route, test); err != nil {
		respondError(c, http.StatusBadGateway, "notification delivery failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
