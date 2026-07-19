package server

import (
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (s *Server) handleDashboard(c *gin.Context) {
	// Calculate offline cutoff once (optimization #3)
	offlineCutoff := time.Now().Add(-s.offlineThreshold())

	// Concurrent database queries (optimization #1)
	var wg sync.WaitGroup
	var (
		totalAgents    int64
		onlineAgents   int64
		todayTasks     int64
		pendingTasks   int64
		failedTasks    int64
		totalCreds     int64
		totalTokens    int64
		totalAudits    int64
		totalSocks     int64
		totalListeners int64
		totalTasks     int64
	)

	wg.Add(11)
	dashRecover := func() {
		if r := recover(); r != nil {
			slog.Error("Dashboard stats goroutine panicked", "recover", r)
		}
	}
	go func() { defer wg.Done(); defer dashRecover(); s.db.Session(&gorm.Session{NewDB: true}).Model(&db.Implant{}).Count(&totalAgents) }()
	go func() {
		defer wg.Done()
		defer dashRecover()
		s.db.Session(&gorm.Session{NewDB: true}).Model(&db.Implant{}).Where("last_seen > ?", offlineCutoff).Count(&onlineAgents)
	}()
	go func() {
		defer wg.Done()
		defer dashRecover()
		s.db.Session(&gorm.Session{NewDB: true}).Model(&db.Task{}).Where("created_at >= ?", time.Now().AddDate(0, 0, -1)).Count(&todayTasks)
	}()
	go func() { defer wg.Done(); defer dashRecover(); s.db.Session(&gorm.Session{NewDB: true}).Model(&db.Task{}).Where("status = ?", "pending").Count(&pendingTasks) }()
	go func() { defer wg.Done(); defer dashRecover(); s.db.Session(&gorm.Session{NewDB: true}).Model(&db.Task{}).Where("status = ?", "failed").Count(&failedTasks) }()
	go func() { defer wg.Done(); defer dashRecover(); s.db.Session(&gorm.Session{NewDB: true}).Model(&db.Task{}).Count(&totalTasks) }()
	go func() { defer wg.Done(); defer dashRecover(); s.db.Session(&gorm.Session{NewDB: true}).Model(&db.CredentialEntry{}).Count(&totalCreds) }()
	go func() { defer wg.Done(); defer dashRecover(); s.db.Session(&gorm.Session{NewDB: true}).Model(&db.TokenEntry{}).Count(&totalTokens) }()
	go func() { defer wg.Done(); defer dashRecover(); s.db.Session(&gorm.Session{NewDB: true}).Model(&db.AuditLog{}).Count(&totalAudits) }()
	go func() { defer wg.Done(); defer dashRecover(); s.db.Session(&gorm.Session{NewDB: true}).Model(&db.SocksSession{}).Count(&totalSocks) }()
	go func() { defer wg.Done(); defer dashRecover(); s.db.Session(&gorm.Session{NewDB: true}).Model(&db.Listener{}).Count(&totalListeners) }()
	wg.Wait()

	// Online agent list (recently active) - optimized with SELECT
	var recentAgents []db.Implant
	s.db.Select("id", "hostname", "ip", "os", "arch", "last_seen").
		Where("last_seen > ?", offlineCutoff).
		Order("last_seen desc").Limit(10).Find(&recentAgents)

	// Recent tasks
	var recentTasks []db.Task
	s.db.Preload("Agent").
		Where("type NOT IN ?", []string{"screen_stream_start", "screen_stream_stop", "ls"}).
		Order("created_at desc").Limit(DashboardRecentTasks).Find(&recentTasks)

	stats := s.getNavStats()
	data := gin.H{
		"Title":          "ForgeC2 - Dashboard",
		"ActiveNav":      "dashboard",
		"TotalAgents":    totalAgents,
		"OnlineAgents":   onlineAgents,
		"TodayTasks":     todayTasks,
		"RecentTasks":    recentTasks,
		"PendingTasks":   pendingTasks,
		"FailedTasks":    failedTasks,
		"TotalCreds":     totalCreds,
		"TotalTokens":    totalTokens,
		"TotalAudits":    totalAudits,
		"TotalSocks":     totalSocks,
		"TotalListeners": totalListeners,
		"TotalTasks":     totalTasks,
		"RecentAgents":   recentAgents,
	}
	for k, v := range stats {
		data[k] = v
	}

	s.renderPageOrJSON(c, data)
}

// --- Shared nav stats helper with caching (optimization #2) ---
var (
	navStatsCache     gin.H
	navStatsCacheTime time.Time
	navStatsCacheMu   sync.RWMutex
	navStatsCacheTTL  = 5 * time.Second
)

func (s *Server) getNavStats() gin.H {
	navStatsCacheMu.RLock()
	if time.Since(navStatsCacheTime) < navStatsCacheTTL && navStatsCache != nil {
		stats := navStatsCache
		navStatsCacheMu.RUnlock()
		return stats
	}
	navStatsCacheMu.RUnlock()

	// Cache expired, recalculate
	navStatsCacheMu.Lock()
	defer navStatsCacheMu.Unlock()

	// Double-check after acquiring write lock
	if time.Since(navStatsCacheTime) < navStatsCacheTTL && navStatsCache != nil {
		return navStatsCache
	}

	offlineCutoff := time.Now().Add(-s.offlineThreshold())
	staleCutoff := time.Now().Add(-StaleThreshold)

	var online int64
	s.db.Model(&db.Implant{}).Where("last_seen > ?", offlineCutoff).Count(&online)

	var stale int64
	s.db.Model(&db.Implant{}).Where("last_seen <= ? AND last_seen > ?", offlineCutoff, staleCutoff).Count(&stale)

	var offlineAgents int64
	s.db.Model(&db.Implant{}).Where("last_seen <= ?", staleCutoff).Count(&offlineAgents)

	var listenerCount int64
	s.db.Model(&db.Listener{}).Where("enabled = ?", true).Count(&listenerCount)

	var pendingTasks int64
	s.db.Model(&db.Task{}).Where("status = ?", "pending").Count(&pendingTasks)

	onlineUsers := int64(len(s.getOnlineUsers()))

	navStatsCache = gin.H{
		"online_count":   online,
		"stale_count":    stale,
		"offline_count":  offlineAgents,
		"listener_count": listenerCount,
		"pending_count":  pendingTasks,
		"online_users":   onlineUsers,
	}
	navStatsCacheTime = time.Now()
	return navStatsCache
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
