package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleListListeners(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	query := s.db.Model(&db.Listener{})

	if tag := c.Query("tag"); tag != "" {
		query = query.Where("tags LIKE ?", "%"+tag+"%")
	}
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	query.Count(&total)

	var listeners []db.Listener
	query.Order("created_at desc").Offset(offset).Limit(pageSize).Find(&listeners)
	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"data":      listeners,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (s *Server) handleListenerDetail(c *gin.Context) {
	id := c.Param("id")
	var listener db.Listener
	if err := s.db.First(&listener, id).Error; err != nil {
		c.String(http.StatusNotFound, "Listener not found")
		return
	}

	var agents []db.Implant
	s.db.Where("listener_id = ?", listener.ID).Order("last_seen desc").Limit(5000).Find(&agents)

	activeCount := 0
	now := time.Now()
	for _, a := range agents {
		if now.Sub(a.LastSeen) < ListenerActiveThreshold {
			activeCount++
		}
	}

	stats := s.getNavStats()
	data := gin.H{
		"Title":        fmt.Sprintf("ForgeC2 - Listener %s", listener.Name),
		"ActiveNav":    "listeners",
		"Listener":     listener,
		"Agents":       agents,
		"TotalAgents":  len(agents),
		"ActiveAgents": activeCount,
	}
	for k, v := range stats {
		data[k] = v
	}

	s.renderPageOrJSON(c, data)
}

// startListenerForRecord creates a real network listener for a DB listener record.
// Handles HTTP/HTTPS/TCP/TLS schemes, port conflict detection, and main server port skip.
func (s *Server) startListenerForRecord(l *db.Listener, context string) {
	scheme := l.Scheme
	if scheme == "" {
		scheme = l.Type
	}
	if scheme != "http" && scheme != "https" && scheme != "tcp" && scheme != "tls" {
		return
	}

	addr := l.Host + ":" + itoa(l.Port)
	key := scheme + "://" + addr

	// Skip if this is the main server address (already served)
	mainAddr := s.cfg.Server.Host + ":" + itoa(s.cfg.Server.Port)
	if addr == mainAddr && (scheme == "http" || scheme == "https") {
		slog.Debug("Listener matches main server address, no extra listener needed",
			"key", key, "context", context)
		return
	}

	// Check port availability
	if !isPortAvailable(l.Host, l.Port) {
		slog.Warn("Port not available for listener, skipping",
			"key", key, "context", context)
		return
	}

	if err := s.startExtraListener(key, addr, scheme); err != nil {
		slog.Error("Failed to start listener",
			"key", key, "context", context, "err", err)
	}
}

func (s *Server) handleCreateListener(c *gin.Context) {
	var l db.Listener
	if err := c.ShouldBindJSON(&l); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if l.Name == "" {
		l.Name = "Listener " + fmt.Sprintf("%d", l.Port)
	}

	// Normalize: prefer Scheme, derive Type and Protocol
	if l.Scheme != "" {
		l.Protocol = l.Scheme
		switch l.Scheme {
		case "http", "https":
			l.Type = "http"
		case "dns":
			l.Type = "dns"
		default:
			l.Type = "tcp"
		}
	} else if l.Protocol != "" {
		l.Scheme = l.Protocol
		switch l.Protocol {
		case "http", "https":
			l.Type = "http"
		case "dns":
			l.Type = "dns"
		default:
			l.Type = "tcp"
		}
	} else if l.Type != "" {
		switch l.Type {
		case "http":
			l.Scheme = "http"
			l.Protocol = "http"
		case "dns":
			l.Scheme = "dns"
			l.Protocol = "dns"
		default:
			l.Scheme = "tcp"
			l.Protocol = "tcp"
		}
	}

	l.Enabled = true
	l.Status = "running"
	if err := s.db.Create(&l).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Listener create"))
		return
	}

	// Start a real listener for the requested port/type.
	s.startListenerForRecord(&l, "created")

	c.JSON(http.StatusOK, gin.H{"success": true, "listener": l})
}

func (s *Server) handleUpdateListener(c *gin.Context) {
	id := c.Param("id")
	var l db.Listener
	if err := s.db.First(&l, id).Error; err != nil {
		respondError(c, http.StatusNotFound, "listener not found")
		return
	}
	var updates struct {
		Name     string `json:"name"`
		Scheme   string `json:"scheme"`
		Type     string `json:"type"`
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Protocol string `json:"protocol"`
		Notes    string `json:"notes"`
		Enabled  *bool  `json:"enabled"`
		Tags     string `json:"tags"`
		Color    string `json:"color"`
		Status   string `json:"status"`
	}
	if err := c.ShouldBindJSON(&updates); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if updates.Name != "" {
		l.Name = updates.Name
	}
	if updates.Scheme != "" {
		l.Scheme = updates.Scheme
		l.Protocol = updates.Scheme
		switch updates.Scheme {
		case "http", "https":
			l.Type = "http"
		case "dns":
			l.Type = "dns"
		default:
			l.Type = "tcp"
		}
	} else if updates.Protocol != "" {
		l.Protocol = updates.Protocol
		l.Scheme = updates.Protocol
		switch updates.Protocol {
		case "http", "https":
			l.Type = "http"
		case "dns":
			l.Type = "dns"
		default:
			l.Type = "tcp"
		}
	} else if updates.Type != "" {
		l.Type = updates.Type
		switch updates.Type {
		case "http":
			l.Scheme = "http"
			l.Protocol = "http"
		case "dns":
			l.Scheme = "dns"
			l.Protocol = "dns"
		default:
			l.Scheme = "tcp"
			l.Protocol = "tcp"
		}
	}
	if updates.Host != "" {
		l.Host = updates.Host
	}
	if updates.Port != 0 {
		l.Port = updates.Port
	}
	if updates.Notes != "" {
		l.Notes = updates.Notes
	}
	if updates.Enabled != nil {
		l.Enabled = *updates.Enabled
	}
	if updates.Tags != "" {
		l.Tags = updates.Tags
	}
	if updates.Color != "" {
		l.Color = updates.Color
	}
	if updates.Status != "" {
		l.Status = updates.Status
	}
	if err := s.db.Save(&l).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Listener update"))
		return
	}

	// Sync extra listener state (start/stop/restart)
	oldKey := l.Scheme + "://" + l.Host + ":" + itoa(l.Port)
	changed := updates.Host != "" || updates.Port != 0 || updates.Scheme != "" || updates.Protocol != "" || updates.Type != ""
	if changed {
		s.stopExtraListener(oldKey)
	}
	if l.Enabled {
		s.startListenerForRecord(&l, "updated")
	} else if updates.Enabled != nil && !*updates.Enabled {
		s.stopExtraListener(oldKey)
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "listener": l})
}

func (s *Server) handleDeleteListener(c *gin.Context) {
	id := c.Param("id")

	// Check if any agents are using this listener
	var agentCount int64
	s.db.Model(&db.Implant{}).Where("listener_id = ?", id).Count(&agentCount)
	if agentCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":       fmt.Sprintf("Cannot delete listener: %d agents still using this listener", agentCount),
			"agent_count": agentCount,
		})
		return
	}

	// Load listener to stop any running extra listener
	var l db.Listener
	if err := s.db.First(&l, id).Error; err == nil {
		key := l.Scheme + "://" + l.Host + ":" + itoa(l.Port)
		s.stopExtraListener(key)
	}

	if err := s.db.Delete(&db.Listener{}, id).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Listener delete"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleEnableListener(c *gin.Context) {
	id := c.Param("id")
	s.db.Model(&db.Listener{}).Where("id = ?", id).Updates(map[string]interface{}{"enabled": true, "status": "running"})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Listener enabled"})
}

func (s *Server) handleDisableListener(c *gin.Context) {
	id := c.Param("id")
	s.db.Model(&db.Listener{}).Where("id = ?", id).Updates(map[string]interface{}{"enabled": false, "status": "stopped"})
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Listener disabled"})
}

func (s *Server) handleListenersPage(c *gin.Context) {
	var listeners []db.Listener
	s.db.Order("created_at desc").Find(&listeners)

	enabled := 0
	httpC := 0
	tcpC := 0
	dnsC := 0
	for _, l := range listeners {
		if l.Enabled {
			enabled++
		}
		if l.Type == "http" {
			httpC++
		} else if l.Type == "tcp" {
			tcpC++
		} else if l.Type == "dns" {
			dnsC++
		}
	}

	stats := s.getNavStats()
	data := gin.H{
		"Title":        "ForgeC2 - Listeners",
		"ActiveNav":    "listeners",
		"Listeners":    listeners,
		"Total":        len(listeners),
		"EnabledCount": enabled,
		"HttpCount":    httpC,
		"TcpCount":     tcpC,
		"DnsCount":     dnsC,
	}
	for k, v := range stats {
		data[k] = v
	}

	s.renderPageOrJSON(c, data)
}
