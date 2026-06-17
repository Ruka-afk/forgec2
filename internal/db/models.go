package db

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Agent represents a connected agent (implant)
type Agent struct {
	ID        string `gorm:"primaryKey" json:"id"`
	Hostname  string `json:"hostname"`
	Username  string `json:"username"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	IP        string `json:"ip"`
	LastSeen  time.Time `json:"last_seen"`
	Status    string `json:"status"` // online, offline
	Notes     string `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Task represents a command/task sent to an agent
type Task struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	AgentID   string    `json:"agent_id"`
	Type      string    `json:"type"`      // shell, screenshot, ps, ls, delete, read, upload, download, kill
	Command   string    `json:"command"`   // primary payload (cmd, path, url)
	Shell     string    `json:"shell"`     // shell choice (cmd.exe/powershell.exe) or secondary data (e.g. b64 for upload push)
	Path      string    `json:"path,omitempty"`      // explicit path for file ops
	Data      string    `json:"data,omitempty"`      // b64 content when applicable
	Status    string    `json:"status"`    // pending, running, completed, failed
	Result    string    `json:"result"`    // output
	Error     string    `json:"error"`     // error message
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Agent     Agent     `gorm:"foreignKey:AgentID" json:"-"`
}

// AuditLog represents a security audit log entry
type AuditLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	User      string    `json:"user"`      // username or "system"
	Action    string    `json:"action"`    // action type: login, logout, command, delete, etc.
	Resource  string    `json:"resource"`  // affected resource
	AgentID   string    `json:"agent_id"`  // related agent ID if applicable
	IP        string    `json:"ip"`        // client IP address
	Success   bool      `json:"success"`   // whether the action succeeded
	Error     string    `json:"error"`     // error message if failed
	Details   string    `json:"details"`   // additional details
	CreatedAt time.Time `json:"created_at"`
}

// BeforeCreate hook for UUID
func (a *Agent) BeforeCreate(tx *gorm.DB) (err error) {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	return nil
}

// TableName overrides
func (Agent) TableName() string    { return "agents" }
func (Task) TableName() string     { return "tasks" }
func (AuditLog) TableName() string { return "audit_logs" }
