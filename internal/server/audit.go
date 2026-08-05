package server

import (
	"crypto/sha256"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// auditChainMu serializes hash-chain appends. flushAuditEntries reads the last
// entry and back-fills prev_hash; two concurrent flushes (beacon checkins run
// in parallel) could otherwise read the same last hash and commit entries with
// identical prev_hash values, silently breaking the tamper-evident chain.
var auditChainMu sync.Mutex

// sensitivePatterns matches common sensitive field names in JSON or URL-encoded data.
// It replaces the value portion with "*****" to prevent secret leakage in audit logs.
var sensitivePatterns = []*regexp.Regexp{
	// JSON: "password":"secretvalue"
	regexp.MustCompile(`"(password|secret|token|api_key|api_secret|jwt|session_key|loot_key|private_key)"\s*:\s*"[^"]+"`),
	// JSON with escaped quotes
	regexp.MustCompile(`"(password|secret|token|api_key|api_secret|jwt|session_key|loot_key|private_key)"\s*:\s*'[^']+'`),
	// URL-encoded or plain: password=secretvalue
	regexp.MustCompile(`(?i)(password|secret|token|api_key|api_secret|jwt|session_key|loot_key|private_key)\s*[:=]\s*\S{4,}`),
	// Key material hex/blobs
	regexp.MustCompile(`(?i)(-----BEGIN.*?KEY-----)(.|\n)*?(-----END.*?KEY-----)`),
}

// sanitizeDetails masks sensitive field values in audit log details.
// This is a defense-in-depth measure — callers should also avoid sending secrets,
// but this catches accidental leakage.
func sanitizeDetails(details string) string {
	if details == "" || !strings.ContainsAny(details, "=:") {
		return details
	}
	result := details
	for _, re := range sensitivePatterns {
		result = re.ReplaceAllString(result, "$1:*****")
	}
	return result
}

// LogAuditRecord creates an audit log entry
// c may be nil for non-HTTP paths (e.g. TCP transport beacons)
func (s *Server) LogAuditRecord(c *gin.Context, action, resource, agentID, details string, success bool, err error) {
	entries := s.buildAuditEntries(c, []auditEntry{{action, resource, agentID, details, success, err}})
	s.flushAuditEntries(entries)
}

// auditEntry is a lightweight intermediate for batch collection.
type auditEntry struct {
	action, resource, agentID, details string
	success                            bool
	err                                error
}

// LogAuditRecords batch-inserts multiple audit log entries in a single DB round-trip.
// c may be nil for non-HTTP paths.
func (s *Server) LogAuditRecords(c *gin.Context, entries []auditEntry) {
	if len(entries) == 0 {
		return
	}
	logEntries := s.buildAuditEntries(c, entries)
	s.flushAuditEntries(logEntries)
}

func (s *Server) buildAuditEntries(c *gin.Context, entries []auditEntry) []db.AuditLog {
	var user, ip string
	if c != nil {
		if u, exists := c.Get("user"); exists {
			if us, ok := u.(string); ok {
				user = us
			} else {
				user = "system"
			}
		} else {
			user = "system"
		}
		ip = c.ClientIP()
		if ip == "" {
			ip = c.Request.RemoteAddr
		}
	} else {
		user = "system"
	}

	result := make([]db.AuditLog, 0, len(entries))
	for _, e := range entries {
		errorMsg := ""
		if e.err != nil {
			errorMsg = e.err.Error()
		}
		logEntry := db.AuditLog{
			User:     user,
			Action:   e.action,
			Resource: e.resource,
			AgentID:  e.agentID,
			IP:       ip,
			Success:  e.success,
			Error:    sanitizeDetails(errorMsg),
			Details:  sanitizeDetails(e.details),
		}
		result = append(result, logEntry)
	}
	return result
}

func (s *Server) flushAuditEntries(entries []db.AuditLog) {
	if len(entries) == 0 {
		return
	}
	// Serialize appends so the read-last-entry + insert is atomic with respect
	// to other appends (see auditChainMu docs).
	auditChainMu.Lock()
	defer auditChainMu.Unlock()

	// Compute append-only hash chain for tamper detection within a transaction
	// to ensure atomicity of the hash chain.
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var lastHash string
		var lastEntry db.AuditLog
		if err := tx.Order("id DESC").First(&lastEntry).Error; err == nil {
			lastHash = lastEntry.EntryHash
		}
		for i := range entries {
			entries[i].PrevHash = lastHash
			h := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%s|%s|%t|%s|%s",
				entries[i].User, entries[i].Action, entries[i].Resource,
				entries[i].AgentID, entries[i].IP, entries[i].Success,
				entries[i].Error, entries[i].Details)))
			entries[i].EntryHash = fmt.Sprintf("%x", h)
			lastHash = entries[i].EntryHash
		}
		return tx.CreateInBatches(entries, 50).Error
	})
	if err != nil {
		slog.Error("Failed to batch-create audit logs", "count", len(entries), "err", err)
	}
	if s.siem != nil {
		for _, logEntry := range entries {
			s.siem.Send(SIEMEvent{
				Timestamp: logEntry.CreatedAt,
				Action:    logEntry.Action,
				Resource:  logEntry.Resource,
				AgentID:   logEntry.AgentID,
				User:      logEntry.User,
				IP:        logEntry.IP,
				Success:   logEntry.Success,
				Error:     logEntry.Error,
				Details:   logEntry.Details,
			})
		}
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
			if s, ok := u.(string); ok {
				user = s
			} else {
				user = "anonymous"
			}
		} else {
			user = "anonymous"
		}

		ip := c.ClientIP()
		if ip == "" {
			ip = c.Request.RemoteAddr
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

// OperatorAction represents a structured audit trail entry for operator-initiated actions.
type OperatorAction struct {
	Action    string            `json:"action"`
	Resource  string            `json:"resource"`
	TargetID  string            `json:"target_id,omitempty"`
	Details   string            `json:"details,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	RiskLevel string            `json:"risk_level"`
}

// LogOperatorAction records a structured audit trail entry for high-value operator actions.
func (s *Server) LogOperatorAction(c *gin.Context, action OperatorAction) {
	user := "system"
	ip := ""
	if c != nil {
		if u, exists := c.Get("user"); exists {
			if us, ok := u.(string); ok {
				user = us
			}
		}
		ip = c.ClientIP()
	}

	details := sanitizeDetails(action.Details)
	if action.TargetID != "" {
		details = "target=" + action.TargetID + " " + details
	}

	logEntry := db.AuditLog{
		User:     user,
		Action:   "operator:" + action.Action,
		Resource: action.Resource,
		AgentID:  action.TargetID,
		IP:       ip,
		Success:  true,
		Details:  details,
	}

	// Route through the chained flush path so operator actions are part of the
	// same tamper-evident hash chain as beacon-triggered entries.
	s.flushAuditEntries([]db.AuditLog{logEntry})

	slog.Warn("Operator action",
		"user", user,
		"action", action.Action,
		"resource", action.Resource,
		"target", action.TargetID,
		"risk", action.RiskLevel,
		"ip", ip,
	)
}

// LogEmergencyAction logs a killswitch/emergency stop action.
func (s *Server) LogEmergencyAction(c *gin.Context, action string, agentCount int) {
	s.LogOperatorAction(c, OperatorAction{
		Action:    action,
		Resource:  "emergency",
		Details:   "affected_agents=" + itoaJARM(agentCount),
		RiskLevel: "critical",
	})
}

// LogUserManagementAction logs user CRUD operations.
func (s *Server) LogUserManagementAction(c *gin.Context, action, targetUser, details string) {
	s.LogOperatorAction(c, OperatorAction{
		Action:    action,
		Resource:  "user",
		TargetID:  targetUser,
		Details:   details,
		RiskLevel: "high",
	})
}

// LogPluginAction logs plugin install/uninstall/configure operations.
func (s *Server) LogPluginAction(c *gin.Context, action, pluginName, details string) {
	s.LogOperatorAction(c, OperatorAction{
		Action:    action,
		Resource:  "plugin",
		TargetID:  pluginName,
		Details:   details,
		RiskLevel: "medium",
	})
}

// LogConfigChange logs system configuration changes.
func (s *Server) LogConfigChange(c *gin.Context, field, oldValue, newValue string) {
	s.LogOperatorAction(c, OperatorAction{
		Action:    "config_change",
		Resource:  "system_config",
		TargetID:  field,
		Details:   "old=" + oldValue + " new=" + newValue,
		RiskLevel: "high",
	})
}
