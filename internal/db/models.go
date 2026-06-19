package db

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Agent represents a connected agent (implant)
type Agent struct {
	ID              string    `gorm:"primaryKey" json:"id"`
	Hostname        string    `json:"hostname"`
	Username        string    `json:"username"`
	OS              string    `json:"os"`
	Arch            string    `json:"arch"`
	IP              string    `json:"ip"`
	LastSeen        time.Time `json:"last_seen"`
	Status          string    `json:"status"` // online, offline
	Notes           string    `json:"notes"`
	Tags            string    `json:"tags"` // comma separated
	ListenerID      uint      `json:"listener_id"`
	// P2P Beacon Chaining
	ParentID      string `json:"parent_id"`       // UUID of parent agent (empty if direct)
	P2PMode       string `json:"p2p_mode"`        // "", "smb", "tcp" — how child connects
	P2PListenAddr string `json:"p2p_listen_addr"` // smb pipe name or tcp addr for children
	// Agent metadata (reported every beacon)
	Version         string `json:"version"`          // agent build version
	PID             int    `json:"pid"`              // agent process ID
	ProcessName     string `json:"process_name"`     // e.g. forgec2.exe
	Integrity       string `json:"integrity"`        // Low / Medium / High / System
	Elevated        bool   `json:"elevated"`         // running as admin/root
	Domain          string `json:"domain"`           // AD domain or workgroup
	// Per-agent sleep config (server-side tracking)
	CurrentInterval int    `json:"current_interval"` // current sleep interval (seconds)
	CurrentJitter   int    `json:"current_jitter"`   // current jitter percentage
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// Task represents a command/task sent to an agent
type Task struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	AgentID   string    `gorm:"index" json:"agent_id"`
	Type      string    `json:"type"`             // shell, screenshot, ps, ls, delete, read, upload, download, kill, keylogger_*, suspend/resume, killproc, clipboard_*, find, reg_*, elevate, screen_stream_*
	Command   string    `json:"command"`          // primary payload (cmd, path, url)
	Shell     string    `json:"shell"`            // shell choice (cmd.exe/powershell.exe) or secondary data (e.g. b64 for upload push)
	Path      string    `json:"path,omitempty"`   // explicit path for file ops
	Data      string    `json:"data,omitempty"`   // b64 content when applicable
	Offset    int64     `json:"offset,omitempty"` // for chunked file ops
	Size      int64     `json:"size,omitempty"`   // chunk size or total for file ops
	Status    string    `json:"status"`           // pending, running, completed, failed
	Result    string    `json:"result"`           // output
	Error     string    `json:"error"`            // error message
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Agent     Agent     `gorm:"foreignKey:AgentID" json:"-"`
}

// AuditLog represents a security audit log entry
type AuditLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	User      string    `json:"user"`     // username or "system"
	Action    string    `json:"action"`   // action type: login, logout, command, delete, etc.
	Resource  string    `json:"resource"` // affected resource
	AgentID   string    `json:"agent_id"` // related agent ID if applicable
	IP        string    `json:"ip"`       // client IP address
	Success   bool      `json:"success"`  // whether the action succeeded
	Error     string    `json:"error"`    // error message if failed
	Details   string    `json:"details"`  // additional details
	CreatedAt time.Time `json:"created_at"`
}

// Listener represents a C2 listener profile for agents to connect to.
// Supports multiple listeners (different hosts/ports/protocols) like in Cobalt Strike.
//
// Recommended: use "Scheme" for the full wire protocol.
// "Type" is kept for backward compatibility and derived from Scheme.
type Listener struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `json:"name"`
	Scheme    string    `json:"scheme"`    // "http", "https", "tcp", "tls"  (preferred)
	Type      string    `json:"type"`      // "http" or "tcp" (derived, kept for compat)
	Host      string    `json:"host"`      // IP or domain
	Port      int       `json:"port"`
	Protocol  string    `json:"protocol"`  // deprecated alias for Scheme, kept for compat
	Notes     string    `json:"notes"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BeforeCreate hook for UUID
func (a *Agent) BeforeCreate(tx *gorm.DB) (err error) {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	return nil
}

// TokenEntry records a stolen/created Windows token for an agent.
// It is the "Token Vault" -- persisted for replay across sessions.
type TokenEntry struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	AgentID     string    `gorm:"index" json:"agent_id"`
	PID         uint32    `json:"pid"`              // source process PID (0 = make_token)
	ProcessName string    `json:"process_name"`     // e.g. lsass.exe, winlogon.exe
	Domain      string    `json:"domain"`           // domain / workgroup
	Username    string    `json:"username"`         // impersonated user
	LogonType   string    `json:"logon_type"`       // e.g. Interactive, Network
	Integrity   string    `json:"integrity"`        // Low / Medium / High / System
	TokenType   string    `json:"token_type"`       // Primary / Impersonation
	Source      string    `json:"source"`           // steal | make_token | duplicate
	Active      bool      `json:"active"`           // currently impersonated on this agent
	Notes       string    `json:"notes"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SocksSession tracks a C2-relayed SOCKS5 proxy tunnel.
// The C2 server opens a local TCP listener; SOCKS5 connections from the operator
// are tunnelled through the Beacon channel to the Agent which dials the target.
type SocksSession struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	AgentID     string    `gorm:"index" json:"agent_id"`
	ListenPort  int       `json:"listen_port"`  // server-side local port
	Status      string    `json:"status"`       // active | stopped
	BytesIn     int64     `json:"bytes_in"`     // operator → agent
	BytesOut    int64     `json:"bytes_out"`    // agent → operator
	ConnCount   int       `json:"conn_count"`   // total connections served
	Notes       string    `json:"notes"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// User represents an authenticated operator
// ForgeC2 multi-user support with role-based access control
type User struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	Username      string    `gorm:"uniqueIndex;size:64" json:"username"`
	PasswordHash  string    `json:"-"`
	Role          string    `json:"role"`    // "admin" or "operator"
	IsActive      bool      `json:"is_active"`
	LastLogin     time.Time `json:"last_login"`
	LastIP        string    `json:"last_ip"`
	LoginAttempts int       `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CredentialEntry stores a parsed credential harvested from an agent.
// Auto-populated when "creds" task results arrive, or manually added.
type CredentialEntry struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	AgentID   string    `gorm:"index" json:"agent_id"`
	Domain    string    `json:"domain"`
	Username  string    `json:"username"`
	Password  string    `json:"password"`
	Hash      string    `json:"hash"`      // NTLM / SHA etc.
	Source    string    `json:"source"`    // lsass, sam, mimikatz, manual
	Type      string    `json:"type"`      // cleartext, ntlm, aes, kerberos
	Notes     string    `json:"notes"`
	TaskID    uint      `json:"task_id"`   // originating task (0 = manual)
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName overrides
func (Agent) TableName() string           { return "agents" }
func (Task) TableName() string            { return "tasks" }
func (AuditLog) TableName() string        { return "audit_logs" }
func (Listener) TableName() string        { return "listeners" }
func (TokenEntry) TableName() string      { return "token_entries" }
func (SocksSession) TableName() string    { return "socks_sessions" }
func (CredentialEntry) TableName() string { return "credential_entries" }
func (User) TableName() string           { return "users" }
