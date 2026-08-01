package server

import (
	"log/slog"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleDashboard(c *gin.Context) {
	offlineCutoff := time.Now().Add(-s.offlineThreshold())
	todayStart := time.Now().AddDate(0, 0, -1)

	type dashCounts struct {
		TotalAgents    int64
		OnlineAgents   int64
		TodayTasks     int64
		PendingTasks   int64
		FailedTasks    int64
		TotalTasks     int64
		TotalCreds     int64
		TotalTokens    int64
		TotalAudits    int64
		TotalSocks     int64
		TotalListeners int64
	}

	var counts dashCounts
	s.db.Raw(`
		SELECT
			(SELECT COUNT(*) FROM implants) AS total_agents,
			(SELECT COUNT(*) FROM implants WHERE last_seen > ?) AS online_agents,
			(SELECT COUNT(*) FROM tasks WHERE created_at >= ?) AS today_tasks,
			(SELECT COUNT(*) FROM tasks WHERE status = 'pending') AS pending_tasks,
			(SELECT COUNT(*) FROM tasks WHERE status = 'failed') AS failed_tasks,
			(SELECT COUNT(*) FROM tasks) AS total_tasks,
			(SELECT COUNT(*) FROM credential_entries) AS total_creds,
			(SELECT COUNT(*) FROM token_entries) AS total_tokens,
			(SELECT COUNT(*) FROM audit_logs) AS total_audits,
			(SELECT COUNT(*) FROM socks_sessions) AS total_socks,
			(SELECT COUNT(*) FROM listeners) AS total_listeners
	`, offlineCutoff, todayStart).Scan(&counts)

	// Online agent list (recently active) - optimized with SELECT
	var recentAgents []db.Implant
	if err := s.db.Select("id", "hostname", "ip", "os", "arch", "last_seen").
		Where("last_seen > ?", offlineCutoff).
		Order("last_seen desc").Limit(10).Find(&recentAgents).Error; err != nil {
		slog.Error("Dashboard: failed to query recent agents", "err", err)
	}

	// Recent tasks
	var recentTasks []db.Task
	if err := s.db.Preload("Agent").
		Where("type NOT IN ?", []string{"screen_stream_start", "screen_stream_stop", "ls"}).
		Order("created_at desc").Limit(DashboardRecentTasks).Find(&recentTasks).Error; err != nil {
		slog.Error("Dashboard: failed to query recent tasks", "err", err)
	}

	stats := s.getNavStats()
	data := gin.H{
		"Title":          "ForgeC2 - Dashboard",
		"ActiveNav":      "dashboard",
		"TotalAgents":    counts.TotalAgents,
		"OnlineAgents":   counts.OnlineAgents,
		"TodayTasks":     counts.TodayTasks,
		"RecentTasks":    recentTasks,
		"PendingTasks":   counts.PendingTasks,
		"FailedTasks":    counts.FailedTasks,
		"TotalCreds":     counts.TotalCreds,
		"TotalTokens":    counts.TotalTokens,
		"TotalAudits":    counts.TotalAudits,
		"TotalSocks":     counts.TotalSocks,
		"TotalListeners": counts.TotalListeners,
		"TotalTasks":     counts.TotalTasks,
		"RecentAgents":   recentAgents,
	}
	for k, v := range stats {
		data[k] = v
	}

	s.renderPageOrJSON(c, data)
}

// --- Shared nav stats helper with caching (optimization #2) ---
const navStatsCacheTTL = 30 * time.Second

func (s *Server) getNavStats() gin.H {
	s.navStatsCacheMu.RLock()
	if time.Since(s.navStatsCacheAt) < navStatsCacheTTL && s.navStatsCache != nil {
		stats := s.navStatsCache
		s.navStatsCacheMu.RUnlock()
		return stats
	}
	s.navStatsCacheMu.RUnlock()

	offlineCutoff := time.Now().Add(-s.offlineThreshold())
	staleCutoff := time.Now().Add(-s.staleThreshold())

	var online, stale, offlineAgents, listenerCount, pendingTasks int64
	type agentStats struct {
		Online  int64
		Stale   int64
		Offline int64
	}
	var as agentStats
	s.db.Raw(`
		SELECT
			COALESCE(SUM(CASE WHEN last_seen > ? THEN 1 ELSE 0 END), 0) as online,
			COALESCE(SUM(CASE WHEN last_seen > ? AND last_seen <= ? THEN 1 ELSE 0 END), 0) as stale,
			COALESCE(SUM(CASE WHEN last_seen <= ? THEN 1 ELSE 0 END), 0) as offline
		FROM implants`, offlineCutoff, offlineCutoff, staleCutoff, offlineCutoff,
	).Scan(&as)
	online = as.Online
	stale = as.Stale
	offlineAgents = as.Offline
	if err := s.db.Model(&db.Listener{}).Where("enabled = ?", true).Count(&listenerCount).Error; err != nil {
		slog.Error("Failed to count listeners", "err", err)
	}
	if err := s.db.Model(&db.Task{}).Where("status = ?", "pending").Count(&pendingTasks).Error; err != nil {
		slog.Error("Failed to count pending tasks", "err", err)
	}

	onlineUsers := int64(len(s.getOnlineUsers()))

	newCache := gin.H{
		"online_count":   online,
		"stale_count":    stale,
		"offline_count":  offlineAgents,
		"listener_count": listenerCount,
		"pending_count":  pendingTasks,
		"online_users":   onlineUsers,
	}

	s.navStatsCacheMu.Lock()
	s.navStatsCache = newCache
	s.navStatsCacheAt = time.Now()
	s.navStatsCacheMu.Unlock()
	return newCache
}

func detectLanguage(c *gin.Context) string {
	if lang, err := c.Cookie("forgec2_lang"); err == nil && lang != "" {
		if IsLanguageSupported(lang) {
			return lang
		}
	}

	acceptLang := c.GetHeader("Accept-Language")
	if acceptLang != "" {
		parts := strings.Split(acceptLang, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if idx := strings.Index(part, ";"); idx > 0 {
				part = part[:idx]
			}
			part = strings.TrimSpace(part)
			if len(part) >= 2 {
				langCode := part[:2]
				if IsLanguageSupported(langCode) {
					return langCode
				}
			}
		}
	}

	return DefaultLanguage
}

// addUserToData injects user display info into gin.H from context
func (s *Server) addUserToData(c *gin.Context, data gin.H) {
	if user, ok := c.Get("user"); ok {
		data["user_display_name"] = user
	} else {
		data["user_display_name"] = "Operator"
	}
	if role, ok := c.Get("user_role"); ok {
		data["user_role"] = role
	} else {
		data["user_role"] = "operator"
	}
	data["server_version"] = ServerVersion

	currentLang := detectLanguage(c)
	data["current_lang"] = currentLang
	langInfo, _ := GetLanguageInfo(currentLang)
	data["current_lang_info"] = langInfo
	data["is_rtl"] = langInfo.RTL

	data["locales_json"] = GetTranslationsJSON(currentLang)
	data["online_users"] = []map[string]string{}
	if _, ok := data["search_query"]; !ok {
		data["search_query"] = ""
	}
}
