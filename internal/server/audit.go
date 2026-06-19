package server

import (
	"log/slog"
	"net/http"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// LogAuditRecord creates an audit log entry
// c may be nil for non-HTTP paths (e.g. TCP transport beacons)
func (s *Server) LogAuditRecord(c *gin.Context, action, resource, agentID, details string, success bool, err error) {
	var user string
	if c != nil {
		if u, exists := c.Get("user"); exists {
			user = u.(string)
		} else {
			user = "system"
		}
	} else {
		user = "system"
	}

	ip := ""
	if c != nil {
		ip = c.ClientIP()
		if ip == "" {
			ip = c.Request.Header.Get("X-Forwarded-For")
		}
		if ip == "" {
			ip = c.Request.Header.Get("X-Real-IP")
		}
	}

	errorMsg := ""
	if err != nil {
		errorMsg = err.Error()
	}

	logEntry := db.AuditLog{
		User:     user,
		Action:   action,
		Resource: resource,
		AgentID:  agentID,
		IP:       ip,
		Success:  success,
		Error:    errorMsg,
		Details:  details,
	}

	if err := s.db.Create(&logEntry).Error; err != nil {
		slog.Error("Failed to create audit log", "err", err)
	}
}

// AuditMiddleware is a middleware to log API access
func (s *Server) AuditMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip logging for static files and health checks
		path := c.Request.URL.Path
		if path == "/favicon.ico" || path == "/health" {
			c.Next()
			return
		}

		// Log the request before processing
		var user string
		if u, exists := c.Get("user"); exists {
			user = u.(string)
		} else {
			user = "anonymous"
		}

		ip := c.ClientIP()
		if ip == "" {
			ip = c.Request.Header.Get("X-Forwarded-For")
		}

		slog.Info("API access",
			"method", c.Request.Method,
			"path", path,
			"user", user,
			"ip", ip,
		)

		// Process the request
		c.Next()

		// Log the response
		statusCode := c.Writer.Status()
		success := statusCode >= http.StatusOK && statusCode < http.StatusBadRequest

		// Create audit log for important actions
		if shouldLogAction(path) {
			s.LogAuditRecord(c, getActionType(path), path, "", "", success, nil)
		}
	}
}

// shouldLogAction determines if an action should be logged
func shouldLogAction(path string) bool {
	// Log authentication, agent management, and command actions
	actionsToLog := []string{
		"/login",
		"/logout",
		"/agents/",
		"/generate/",
		"/tasks",
	}
	for _, action := range actionsToLog {
		if len(path) >= len(action) && path[:len(action)] == action {
			return true
		}
	}
	return false
}

// getActionType extracts the action type from path
func getActionType(path string) string {
	if len(path) >= 7 && path[:7] == "/login" {
		return "login"
	}
	if len(path) >= 8 && path[:8] == "/logout" {
		return "logout"
	}
	if len(path) >= 8 && path[:8] == "/agents/" {
		return "agent_action"
	}
	if len(path) >= 10 && path[:10] == "/generate/" {
		return "generate"
	}
	if len(path) >= 6 && path[:6] == "/tasks" {
		return "view_tasks"
	}
	return "api_access"
}
