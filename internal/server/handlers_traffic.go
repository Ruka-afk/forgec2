package server

import (
	"bytes"
	"html/template"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type TrafficEntry struct {
	Time     time.Time `json:"time"`
	Method   string    `json:"method"`
	Path     string    `json:"path"`
	RemoteIP string    `json:"remote_ip"`
	AgentID  string    `json:"agent_id"`
	Status   int       `json:"status"`
	Size     int       `json:"size"`
	Latency  string    `json:"latency"`
}

const maxTrafficLogs = 500

type trafficRing struct {
	mu    sync.Mutex
	logs  []TrafficEntry
	index int
	count int
}

func newTrafficRing() *trafficRing {
	return &trafficRing{
		logs: make([]TrafficEntry, maxTrafficLogs),
	}
}

func (t *trafficRing) add(entry TrafficEntry) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.logs[t.index] = entry
	t.index = (t.index + 1) % maxTrafficLogs
	if t.count < maxTrafficLogs {
		t.count++
	}
}

func (t *trafficRing) recent(n int) []TrafficEntry {
	t.mu.Lock()
	defer t.mu.Unlock()
	if n <= 0 || n > maxTrafficLogs {
		n = maxTrafficLogs
	}
	if n > t.count {
		n = t.count
	}
	result := make([]TrafficEntry, n)
	start := (t.index - n + maxTrafficLogs) % maxTrafficLogs
	for i := 0; i < n; i++ {
		result[i] = t.logs[(start+i)%maxTrafficLogs]
	}
	return result
}

// trafficMiddleware captures beacon API requests for the live traffic viewer
func (s *Server) trafficMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		path := c.Request.URL.Path
		// Only capture API paths
		if len(path) < 8 || path[:7] != "/api/v1" {
			return
		}
		entry := TrafficEntry{
			Time:     start,
			Method:   c.Request.Method,
			Path:     path,
			RemoteIP: c.ClientIP(),
			AgentID:  c.GetHeader("X-Agent-ID"),
			Status:   c.Writer.Status(),
			Size:     c.Writer.Size(),
			Latency:  time.Since(start).Round(time.Millisecond).String(),
		}
		s.trafficLog.add(entry)
	}
}

func (s *Server) handleTrafficPage(c *gin.Context) {
	stats := s.getNavStats()
	data := gin.H{
		"Title":     "ForgeC2 - Traffic Monitor",
		"ActiveNav": "traffic",
	}
	s.addUserToData(c, data)
	for k, v := range stats {
		data[k] = v
	}

	var contentBuf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&contentBuf, "traffic_content", data); err != nil {
		slog.Error("Failed to render traffic content", "err", err)
		c.String(http.StatusInternalServerError, "Template error")
		return
	}

	data["Content"] = template.HTML(contentBuf.String())
	c.Header("Content-Type", "text/html; charset=utf-8")
	s.tmpl.ExecuteTemplate(c.Writer, "layout.html", data)
}

func (s *Server) handleTrafficData(c *gin.Context) {
	logs := s.trafficLog.recent(200)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": logs})
}
