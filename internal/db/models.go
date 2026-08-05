package db

import (
	"encoding/json"
	"time"

	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

const (
	PermAgentsRead         = "agents.read"
	PermAgentsWrite        = "agents.write"
	PermAgentsDelete       = "agents.delete"
	PermListenersRead      = "listeners.read"
	PermListenersWrite     = "listeners.write"
	PermListenersDelete    = "listeners.delete"
	PermTasksRead          = "tasks.read"
	PermTasksWrite         = "tasks.write"
	PermTasksDelete        = "tasks.delete"
	PermCredsRead          = "credentials.read"
	PermCredsWrite         = "credentials.write"
	PermCredsDelete        = "credentials.delete"
	PermFilesRead          = "files.read"
	PermFilesWrite         = "files.write"
	PermUsersRead          = "users.read"
	PermUsersWrite         = "users.write"
	PermUsersDelete        = "users.delete"
	PermSettingsRead       = "settings.read"
	PermSettingsWrite      = "settings.write"
	PermAuditRead          = "audit.read"
	PermGroupsRead         = "groups.read"
	PermGroupsWrite        = "groups.write"
	PermWorkflowsRead      = "workflows.read"
	PermWorkflowsWrite     = "workflows.write"
	PermPluginsRead        = "plugins.read"
	PermPluginsWrite       = "plugins.write"
	PermPluginsExecute     = "plugins.execute"
	PermPluginsDelete      = "plugins.delete"
	PermRolesRead          = "roles.read"
	PermRolesWrite         = "roles.write"
	PermCampaignsRead      = "campaigns.read"
	PermCampaignsWrite     = "campaigns.write"
	PermOpsecRead          = "opsec.read"
	PermOpsecWrite         = "opsec.write"
	PermIntelRead          = "intel.read"
	PermIntelWrite         = "intel.write"
	PermSchedulerRead      = "scheduler.read"
	PermSchedulerWrite     = "scheduler.write"
	PermNotificationsRead  = "notifications.read"
	PermNotificationsWrite = "notifications.write"
)

var RolePermissionsMap = map[string][]string{
	RoleAdmin: {
		PermAgentsRead, PermAgentsWrite, PermAgentsDelete,
		PermListenersRead, PermListenersWrite, PermListenersDelete,
		PermTasksRead, PermTasksWrite, PermTasksDelete,
		PermCredsRead, PermCredsWrite, PermCredsDelete,
		PermFilesRead, PermFilesWrite,
		PermUsersRead, PermUsersWrite, PermUsersDelete,
		PermSettingsRead, PermSettingsWrite,
		PermAuditRead,
		PermGroupsRead, PermGroupsWrite,
		PermWorkflowsRead, PermWorkflowsWrite,
		PermPluginsRead, PermPluginsWrite, PermPluginsExecute, PermPluginsDelete,
		PermRolesRead, PermRolesWrite,
		PermCampaignsRead, PermCampaignsWrite,
		PermOpsecRead, PermOpsecWrite,
		PermIntelRead, PermIntelWrite,
		PermSchedulerRead, PermSchedulerWrite,
		PermNotificationsRead, PermNotificationsWrite,
	},
	RoleUser: {
		PermAgentsRead, PermAgentsWrite,
		PermListenersRead, PermListenersWrite,
		PermTasksRead, PermTasksWrite,
		PermCredsRead, PermCredsWrite,
		PermFilesRead, PermFilesWrite,
		PermUsersRead,
		PermSettingsRead,
		PermAuditRead,
		PermGroupsRead, PermGroupsWrite,
		PermWorkflowsRead, PermWorkflowsWrite,
		PermRolesRead,
		PermCampaignsRead,
		PermOpsecRead,
		PermIntelRead,
		PermSchedulerRead,
		PermNotificationsRead,
	},
}

// TaskStats holds per-agent task status counts (non-persisted, computed at query time)
type TaskStats struct {
	Pending   int `json:"pending"`
	Running   int `json:"running"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
}

// Implant represents a connected implant (agent)
type Implant struct {
	ID         string    `gorm:"primaryKey" json:"id"`
	Hostname   string    `json:"hostname"`
	Username   string    `json:"username"`
	OS         string    `json:"os"`
	Arch       string    `json:"arch"`
	IP         string    `json:"ip"`
	PublicIP   string    `json:"public_ip"` // public IP from beacon connection
	Country    string    `json:"country"`   // GeoIP country
	City       string    `json:"city"`      // GeoIP city
	Latitude   float64   `json:"latitude"`  // GeoIP latitude
	Longitude  float64   `json:"longitude"` // GeoIP longitude
	LastSeen   time.Time `gorm:"index" json:"last_seen"`
	Status     string    `gorm:"index" json:"status"`          // online, offline
	Trusted    bool      `gorm:"default:false" json:"trusted"` // operator-approved agent
	Notes      string    `json:"notes"`
	Tags       string    `json:"tags"` // comma separated
	ListenerID uint      `json:"listener_id"`
	// Multi-hop Proxy Chain (ParentAgentID is the next-hop toward C2, distinct from P2P parent_id)
	ParentAgentID string `gorm:"size:36;default:''" json:"parent_agent_id,omitempty"`
	// P2P Beacon Chaining
	ParentID      string `gorm:"index" json:"parent_id"` // UUID of parent agent (empty if direct)
	P2PMode       string `json:"p2p_mode"`               // "", "smb", "tcp" 锟?how child connects
	P2PListenAddr string `json:"p2p_listen_addr"`        // smb pipe name or tcp addr for children
	// P2P Gossip Mesh
	PeerCount int    `gorm:"default:0" json:"peer_count"`
	BestRoute string `gorm:"default:''" json:"best_route"`
	// Agent metadata (reported every beacon)
	Version         string `json:"version"`          // agent build version
	ProtocolVersion uint   `json:"protocol_version"` // wire protocol version
	PID             int    `json:"pid"`              // agent process ID
	ProcessName     string `json:"process_name"`     // e.g. forgec2.exe
	Integrity       string `json:"integrity"`        // Low / Medium / High / System
	Elevated        bool   `json:"elevated"`         // running as admin/root
	Domain          string `json:"domain"`           // AD domain or workgroup
	// Per-agent sleep config (server-side tracking)
	CurrentInterval int    `json:"current_interval"` // current sleep interval (seconds)
	CurrentJitter   int    `json:"current_jitter"`   // current jitter percentage
	ActiveWindow    string `json:"active_window"`    // foreground window title (reported each beacon)
	// Working hours (server-side tracking)
	WorkingHoursStart string `gorm:"size:5" json:"working_hours_start"` // HH:MM
	WorkingHoursEnd   string `gorm:"size:5" json:"working_hours_end"`   // HH:MM
	WorkingHoursTZ    string `gorm:"size:50" json:"working_hours_tz"`   // IANA timezone
	// Per-agent kill date
	KillDate *time.Time `json:"kill_date,omitempty"` // agent self-destructs after this time
	// Agent self-assessed environment threat data
	EnvThreatScore int    `json:"env_threat_score" gorm:"default:0"`   // 0-100 agent-reported threat level
	EnvHoneypot    bool   `json:"env_honeypot" gorm:"default:false"`   // agent detected honeypot env
	EnvClass       string `json:"env_class" gorm:"size:32;default:''"` // environment classification
	// Protocol v2 identity & replay protection
	IdentityPub string    `gorm:"size:64;default:''" json:"identity_pub,omitempty"` // base64 X25519 identity public key (registered)
	Registered  bool      `gorm:"default:false" json:"registered"`                  // v2 registration handshake completed
	LastSeq     uint64    `gorm:"default:0" json:"last_seq,omitempty"`              // last accepted beacon seq (replay window)
	CreatedAt   time.Time `gorm:"index" json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Task represents a command/task sent to an agent
type Task struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	AgentID  string `gorm:"index" json:"agent_id"`
	Type     string `json:"type"`
	Command  string `json:"command"`
	Shell    string `json:"shell"`
	Path     string `json:"path,omitempty"`
	Data     string `json:"data,omitempty"`
	Offset   int64  `json:"offset,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Status   string `gorm:"index" json:"status"`
	Priority int    `gorm:"default:1" json:"priority"` // 0=low, 1=normal, 2=high, 3=urgent
	Result   string `json:"result"`
	Error    string `json:"error"`
	// File transfer progress tracking (optimization)
	Progress       int        `json:"progress,omitempty"`    // 0-100 percentage
	TotalBytes     int64      `json:"total_bytes,omitempty"` // total file size
	Transferred    int64      `json:"transferred,omitempty"` // bytes transferred so far
	CreatedBy      string     `json:"created_by"`            // operator username who created the task
	ClaimedBy      string     `gorm:"size:255" json:"claimed_by"`
	ClaimedAt      time.Time  `json:"claimed_at"`
	AcknowledgedAt *time.Time `gorm:"index" json:"acknowledged_at,omitempty"`
	// Task callbacks: optional URL to POST results to when task completes
	CallbackURL    string    `gorm:"size:1024" json:"callback_url,omitempty"`
	CallbackMethod string    `gorm:"size:10;default:'POST'" json:"callback_method,omitempty"`
	CallbackSent   bool      `gorm:"default:false" json:"callback_sent"`
	CreatedAt      time.Time `gorm:"index" json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Agent          Implant   `gorm:"foreignKey:AgentID" json:"-"`
}

// AuditLog represents a security audit log entry
type AuditLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	User      string    `json:"user"`                      // username or "system"
	Action    string    `gorm:"index" json:"action"`       // action type: login, logout, command, delete, etc.
	Resource  string    `json:"resource"`                  // affected resource
	AgentID   string    `gorm:"index" json:"agent_id"`     // related agent ID if applicable
	IP        string    `json:"ip"`                        // client IP address
	Success   bool      `json:"success"`                   // whether the action succeeded
	Error     string    `json:"error"`                     // error message if failed
	Details   string    `json:"details"`                   // additional details
	PrevHash  string    `gorm:"size:64" json:"prev_hash"`  // SHA-256 of the previous audit entry (append-only hash chain)
	EntryHash string    `gorm:"size:64" json:"entry_hash"` // SHA-256 hash of this entry (covers all fields)
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}

// Listener represents a C2 listener profile for agents to connect to.
// Supports multiple listeners (different hosts/ports/protocols) like in Cobalt Strike.
//
// Recommended: use "Scheme" for the full wire protocol.
// "Type" is kept for backward compatibility and derived from Scheme.
type Listener struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	Name     string `gorm:"uniqueIndex;size:128" json:"name"`
	Scheme   string `json:"scheme"` // "http", "https", "tcp", "tls"  (preferred)
	Type     string `json:"type"`   // "http", "tcp", "dns", "icmp" (derived, kept for compat)
	Host     string `json:"host"`   // IP or domain
	Port     int    `json:"port"`
	Protocol string `json:"protocol"` // deprecated alias for Scheme, kept for compat
	Notes    string `json:"notes"`
	Enabled  bool   `json:"enabled"`
	Tags     string `gorm:"size:500;default:''" json:"tags"`
	Color    string `gorm:"size:7;default:''" json:"color"`
	Status   string `gorm:"size:20;default:'running'" json:"status"`

	// DNS-specific: the DNS zone domain (e.g. "c2.example.com") and UDP listen address
	DNSDomain     string `gorm:"size:255" json:"dns_domain"`
	DNSListenAddr string `gorm:"size:255" json:"dns_listen_addr"`

	// ICMP-specific: the bind address (e.g. "0.0.0.0")
	ICMPAddr string `gorm:"size:255" json:"icmp_addr"`

	CreatedAt time.Time `gorm:"index" json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BeforeCreate hook for UUID
func (a *Implant) BeforeCreate(tx *gorm.DB) (err error) {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	return nil
}

// encryptField encrypts a single field value using loot encryption.
func encryptField(val *string) error {
	if *val == "" {
		return nil
	}
	enc, err := crypto.EncryptLoot(*val)
	if err != nil {
		return err
	}
	*val = enc
	return nil
}

// decryptField decrypts a single field value using loot encryption.
func decryptField(val *string) error {
	if *val == "" {
		return nil
	}
	dec, err := crypto.DecryptLoot(*val)
	if err != nil {
		return err
	}
	*val = dec
	return nil
}

// BeforeCreate encrypts sensitive fields before inserting into the database.
func (c *CredentialEntry) BeforeCreate(tx *gorm.DB) error {
	if err := encryptField(&c.Password); err != nil {
		return err
	}
	return encryptField(&c.Hash)
}

// AfterFind decrypts sensitive fields after loading from the database.
func (c *CredentialEntry) AfterFind(tx *gorm.DB) error {
	if err := decryptField(&c.Password); err != nil {
		return err
	}
	return decryptField(&c.Hash)
}

// BeforeUpdate encrypts sensitive fields before updating the database.
func (c *CredentialEntry) BeforeUpdate(tx *gorm.DB) error {
	if err := encryptField(&c.Password); err != nil {
		return err
	}
	return encryptField(&c.Hash)
}

func (cc *CloudCred) BeforeCreate(tx *gorm.DB) error {
	if err := encryptField(&cc.Key); err != nil {
		return err
	}
	if err := encryptField(&cc.Value); err != nil {
		return err
	}
	return encryptField(&cc.Extra)
}

func (cc *CloudCred) AfterFind(tx *gorm.DB) error {
	if err := decryptField(&cc.Key); err != nil {
		return err
	}
	if err := decryptField(&cc.Value); err != nil {
		return err
	}
	return decryptField(&cc.Extra)
}

func (cc *CloudCred) BeforeUpdate(tx *gorm.DB) error {
	if err := encryptField(&cc.Key); err != nil {
		return err
	}
	if err := encryptField(&cc.Value); err != nil {
		return err
	}
	return encryptField(&cc.Extra)
}

func (e *ExtC2Channel) BeforeCreate(tx *gorm.DB) error {
	enc, err := crypto.EncryptExtC2(e.BotToken)
	if err != nil {
		return encryptField(&e.BotToken)
	}
	e.BotToken = enc
	return nil
}

func (e *ExtC2Channel) AfterFind(tx *gorm.DB) error {
	dec, err := crypto.DecryptExtC2(e.BotToken)
	if err == nil && dec != e.BotToken {
		e.BotToken = dec
		return nil
	}
	return decryptField(&e.BotToken)
}

func (e *ExtC2Channel) BeforeUpdate(tx *gorm.DB) error {
	enc, err := crypto.EncryptExtC2(e.BotToken)
	if err != nil {
		return encryptField(&e.BotToken)
	}
	e.BotToken = enc
	return nil
}

func (r *Redirector) BeforeCreate(tx *gorm.DB) error {
	if err := encryptField(&r.SSHKey); err != nil {
		return err
	}
	return encryptField(&r.SSHPassword)
}

func (r *Redirector) AfterFind(tx *gorm.DB) error {
	if err := decryptField(&r.SSHKey); err != nil {
		return err
	}
	return decryptField(&r.SSHPassword)
}

func (r *Redirector) BeforeUpdate(tx *gorm.DB) error {
	if err := encryptField(&r.SSHKey); err != nil {
		return err
	}
	return encryptField(&r.SSHPassword)
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	return encryptField(&u.TOTPSecret)
}

func (u *User) AfterFind(tx *gorm.DB) error {
	return decryptField(&u.TOTPSecret)
}

func (u *User) BeforeUpdate(tx *gorm.DB) error {
	return encryptField(&u.TOTPSecret)
}

// TokenEntry records a stolen/created Windows token for an agent.
// It is the "Token Vault" -- persisted for replay across sessions.
type TokenEntry struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	AgentID     string    `gorm:"index" json:"agent_id"`
	PID         uint32    `json:"pid"`          // source process PID (0 = make_token)
	ProcessName string    `json:"process_name"` // e.g. lsass.exe, winlogon.exe
	Domain      string    `json:"domain"`       // domain / workgroup
	Username    string    `json:"username"`     // impersonated user
	LogonType   string    `json:"logon_type"`   // e.g. Interactive, Network
	Integrity   string    `json:"integrity"`    // Low / Medium / High / System
	TokenType   string    `json:"token_type"`   // Primary / Impersonation
	Source      string    `json:"source"`       // steal | make_token | duplicate
	Active      bool      `json:"active"`       // currently impersonated on this agent
	Notes       string    `json:"notes"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SocksSession tracks a C2-relayed SOCKS5 proxy tunnel.
// The C2 server opens a local TCP listener; SOCKS5 connections from the operator
// are tunnelled through the Beacon channel to the Agent which dials the target.
type SocksSession struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	AgentID    string    `gorm:"index" json:"agent_id"`
	ListenPort int       `json:"listen_port"` // server-side local port
	Status     string    `json:"status"`      // active | stopped
	BytesIn    int64     `json:"bytes_in"`    // operator 锟?agent
	BytesOut   int64     `json:"bytes_out"`   // agent 锟?operator
	ConnCount  int       `json:"conn_count"`  // total connections served
	Notes      string    `json:"notes"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// User represents an authenticated operator
// ForgeC2 multi-user support with role-based access control
// Roles: "admin" (full control), "user" (standard operator)
type User struct {
	ID                  uint      `gorm:"primaryKey" json:"id"`
	Username            string    `gorm:"uniqueIndex;size:64" json:"username"`
	PasswordHash        string    `json:"-"`
	Role                string    `json:"role"` // "admin" or "user"
	IsActive            bool      `json:"is_active"`
	ForcePasswordChange bool      `gorm:"default:false" json:"force_password_change"`
	LastLogin           time.Time `json:"last_login"`
	LastIP              string    `json:"last_ip"`
	LastActivity        time.Time `json:"last_activity"`   // last page request or API call
	ForceLogoutAt       time.Time `json:"force_logout_at"` // set by admin to invalidate all sessions
	LoginAttempts       int       `json:"-"`
	TOTPSecret          string    `json:"-"` // TOTP secret for 2FA, empty means 2FA disabled
	CreatedAt           time.Time `gorm:"index" json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// CredentialEntry stores a parsed credential harvested from an agent.
// Auto-populated when "creds" task results arrive, or manually added.
type CredentialEntry struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	AgentID   string    `gorm:"index" json:"agent_id"`
	Domain    string    `json:"domain"`
	Username  string    `json:"username"`
	Password  string    `json:"password"`
	Hash      string    `json:"hash"`   // NTLM / SHA etc.
	Source    string    `json:"source"` // lsass, sam, mimikatz, manual
	Type      string    `json:"type"`   // cleartext, ntlm, aes, kerberos
	Notes     string    `json:"notes"`
	Tags      string    `json:"tags"` // comma separated tags
	ExpiresAt time.Time `gorm:"index" json:"expires_at"`
	Confirmed bool      `json:"confirmed"` // whether credential has been verified
	TaskID    uint      `json:"task_id"`   // originating task (0 = manual)
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName overrides
func (Implant) TableName() string         { return "implants" }
func (Task) TableName() string            { return "tasks" }
func (AuditLog) TableName() string        { return "audit_logs" }
func (Listener) TableName() string        { return "listeners" }
func (TokenEntry) TableName() string      { return "token_entries" }
func (SocksSession) TableName() string    { return "socks_sessions" }
func (CredentialEntry) TableName() string { return "credential_entries" }

// BuildLog records a build attempt
type BuildLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Platform   string    `json:"platform"` // windows, linux
	Format     string    `json:"format"`   // exe, ps1, linux, stager
	C2URL      string    `json:"c2_url"`
	ListenerID uint      `json:"listener_id"`
	Filename   string    `json:"filename"`
	User       string    `json:"user"`   // operator username
	Status     string    `json:"status"` // success, failed
	Error      string    `json:"error"`
	OutputPath string    `json:"output_path"`
	CreatedAt  time.Time `json:"created_at"`
}

func (BuildLog) TableName() string { return "build_logs" }
func (User) TableName() string     { return "users" }

// ScanResult stores port/service scan results from agents
type ScanResult struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	AgentID   string    `gorm:"index" json:"agent_id"`
	TaskID    uint      `gorm:"index" json:"task_id"`
	TargetIP  string    `json:"target_ip"`
	Port      int       `json:"port"`
	Protocol  string    `json:"protocol"` // tcp, udp
	State     string    `json:"state"`    // open, closed, filtered
	Service   string    `json:"service"`  // http, ssh, smb, etc
	Version   string    `json:"version"`
	Banner    string    `json:"banner"`
	CreatedAt time.Time `json:"created_at"`
}

// NetworkHost stores discovered network hosts
type NetworkHost struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	AgentID   string    `gorm:"index" json:"agent_id"`
	IP        string    `json:"ip"`
	Hostname  string    `json:"hostname"`
	OS        string    `json:"os"`
	Services  string    `json:"services"` // JSON array of open ports/services
	LastSeen  time.Time `json:"last_seen"`
	CreatedAt time.Time `json:"created_at"`
}

// CommandTemplate stores reusable command templates
type CommandTemplate struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `json:"name"`
	Category    string    `json:"category"` // recon, privesc, lateral, exfil
	Command     string    `json:"command"`
	Description string    `json:"description"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

// BOFFile stores uploaded BOF (.o) files for reuse across agents
type BOFFile struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"uniqueIndex;size:256" json:"name"`
	Data        []byte    `json:"-"`
	Size        int64     `json:"size"`
	Description string    `json:"description"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (BOFFile) TableName() string { return "bof_files" }

// BOFLibrary stores BOF metadata with arch and author info for the library.
type BOFLibrary struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"uniqueIndex;size:256" json:"name"`
	Description string    `gorm:"size:1024" json:"description"`
	Data        []byte    `json:"-"`
	Arch        string    `gorm:"size:16" json:"arch"`
	Author      string    `gorm:"size:128" json:"author"`
	Size        int64     `json:"size"`
	CreatedBy   string    `gorm:"size:64" json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (BOFLibrary) TableName() string { return "bof_library" }

// ServerConfig stores key-value config for automation, events, etc.
type ServerConfig struct {
	Key       string    `gorm:"primaryKey;size:255" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (ServerConfig) TableName() string { return "server_configs" }

// WebhookConfig stores webhook endpoint configuration
type WebhookConfig struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:255;not null" json:"name"`
	URL       string    `gorm:"size:1024;not null" json:"url"`
	EventType string    `gorm:"size:255;not null" json:"event_type"`
	Method    string    `gorm:"size:16;default:'POST'" json:"method"`
	Headers   string    `gorm:"type:text" json:"headers"`
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (WebhookConfig) TableName() string { return "webhook_configs" }

// Plugin stores registered plugin metadata
type Plugin struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	Name            string    `gorm:"size:255;not null;uniqueIndex" json:"name"`
	Version         string    `gorm:"size:64" json:"version"`
	Description     string    `gorm:"size:1024" json:"description"`
	Author          string    `gorm:"size:255" json:"author"`
	Type            string    `gorm:"size:64" json:"type"` // "hook", "command", "report"
	Enabled         bool      `gorm:"default:true" json:"enabled"`
	Config          string    `gorm:"type:text" json:"config"`
	Category        string    `gorm:"size:64" json:"category"` // "recon", "privesc", "lateral", "report", "automation", "utility"
	Homepage        string    `gorm:"size:512" json:"homepage"`
	License         string    `gorm:"size:64" json:"license"`
	Dependencies    string    `gorm:"type:text" json:"dependencies"` // JSON array of dependency names
	Tags            string    `gorm:"size:512" json:"tags"`          // comma separated
	RatingOverall   float64   `gorm:"-" json:"rating_overall"`
	RatingCount     int       `gorm:"-" json:"rating_count"`
	UpdateAvailable bool      `gorm:"-" json:"update_available"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (Plugin) TableName() string { return "plugins" }

// PluginReview stores plugin ratings and comments
type PluginReview struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	PluginID  uint      `gorm:"index" json:"plugin_id"`
	UserID    uint      `json:"user_id"`
	Username  string    `gorm:"size:64" json:"username"`
	Rating    int       `gorm:"not null" json:"rating"` // 1-5
	Comment   string    `gorm:"type:text" json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}

func (PluginReview) TableName() string { return "plugin_reviews" }

// PluginDependency stores plugin dependency relationships
type PluginDependency struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	PluginID        uint      `gorm:"index" json:"plugin_id"`
	DependencyID    uint      `gorm:"index" json:"dependency_id"`
	Dependency      Plugin    `gorm:"foreignKey:DependencyID" json:"-"`
	RequiredVersion string    `gorm:"size:64" json:"required_version"`
	Optional        bool      `gorm:"default:false" json:"optional"`
	CreatedAt       time.Time `json:"created_at"`
}

func (PluginDependency) TableName() string { return "plugin_dependencies" }

// PluginUpdateStatus tracks plugin update information
type PluginUpdateStatus struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	PluginID        uint      `gorm:"uniqueIndex" json:"plugin_id"`
	LatestVersion   string    `gorm:"size:64" json:"latest_version"`
	UpdateAvailable bool      `json:"update_available"`
	UpdateURL       string    `gorm:"size:512" json:"update_url"`
	ReleaseNotes    string    `gorm:"type:text" json:"release_notes"`
	LastCheckedAt   time.Time `json:"last_checked_at"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (PluginUpdateStatus) TableName() string { return "plugin_update_status" }

func (ScanResult) TableName() string      { return "scan_results" }
func (NetworkHost) TableName() string     { return "network_hosts" }
func (CommandTemplate) TableName() string { return "command_templates" }

type AutomationRule struct {
	ID         string    `gorm:"primaryKey;size:255" json:"id"`
	Name       string    `gorm:"size:255;not null" json:"name"`
	Enabled    bool      `gorm:"default:true" json:"enabled"`
	EventType  string    `gorm:"size:255;not null" json:"event_type"`
	Conditions string    `gorm:"type:text" json:"conditions"`
	Actions    string    `gorm:"type:text" json:"actions"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (AutomationRule) TableName() string { return "automation_rules" }

type AlertRule struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:255;not null" json:"name"`
	Type        string    `gorm:"size:100;not null" json:"type"` // agent_offline, cpu_high, memory_high, disk_high, credential_found, agent_online
	Threshold   float64   `json:"threshold"`                     // Threshold (percentage or seconds)
	Enabled     bool      `gorm:"default:true" json:"enabled"`
	Description string    `gorm:"type:text" json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (AlertRule) TableName() string { return "alert_rules" }

type Alert struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	RuleID     uint      `json:"rule_id"`
	Rule       AlertRule `gorm:"foreignKey:RuleID" json:"rule"`
	Type       string    `gorm:"size:100;not null" json:"type"`
	Severity   string    `gorm:"size:20;default:'warning'" json:"severity"` // critical, warning, info
	Title      string    `gorm:"size:255;not null" json:"title"`
	Message    string    `gorm:"type:text" json:"message"`
	Source     string    `json:"source"`                                 // agent_id, system, etc.
	SourceName string    `json:"source_name"`                            // hostname, etc.
	Status     string    `gorm:"size:20;default:'active'" json:"status"` // active, acknowledged, resolved
	Details    string    `gorm:"type:text" json:"details"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (Alert) TableName() string { return "alerts" }

type SystemMetric struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	CPULoad     float64   `json:"cpu_load"`
	MemoryUsed  float64   `json:"memory_used"`
	MemoryTotal float64   `json:"memory_total"`
	DiskUsed    float64   `json:"disk_used"`
	DiskTotal   float64   `json:"disk_total"`
	NetIn       float64   `json:"net_in"`
	NetOut      float64   `json:"net_out"`
	Hostname    string    `json:"hostname"`
	CreatedAt   time.Time `json:"created_at"`
}

func (SystemMetric) TableName() string { return "system_metrics" }

type GeneratedReport struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:255" json:"name"`
	Template  string    `gorm:"size:50" json:"template"`
	Format    string    `gorm:"size:10;default:html" json:"format"`
	Content   string    `gorm:"type:text" json:"content"`
	Sections  string    `gorm:"type:text" json:"sections"`
	CreatedAt time.Time `json:"created_at"`
}

func (GeneratedReport) TableName() string { return "generated_reports" }

type RolePermission struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Role       string    `gorm:"size:32;index:idx_role_perm_role" json:"role"`
	Permission string    `gorm:"size:64;index:idx_role_perm_perm" json:"permission"`
	CreatedAt  time.Time `json:"created_at"`
}

func (RolePermission) TableName() string { return "role_permissions" }

func GetPermissionsForRole(role string) []string {
	if perms, ok := RolePermissionsMap[role]; ok {
		return perms
	}
	return []string{}
}

func RoleHasPermission(role, permission string) bool {
	// Check built-in roles first
	if perms, ok := RolePermissionsMap[role]; ok {
		for _, p := range perms {
			if p == permission {
				return true
			}
		}
	}
	return false
}

// RoleHasPermissionDB checks both built-in and custom DB-backed roles.
// Requires a database handle to query custom_roles table.
func RoleHasPermissionDB(database interface {
	Where(string, ...interface{}) *gorm.DB
}, role, permission string) bool {
	if RoleHasPermission(role, permission) {
		return true
	}
	var customRole CustomRole
	if database.Where("name = ?", role).First(&customRole).Error != nil {
		return false
	}
	perms := parsePermissionJSON(customRole.Permissions)
	for _, p := range perms {
		if p == permission {
			return true
		}
	}
	return false
}

func parsePermissionJSON(raw string) []string {
	if raw == "" {
		return nil
	}
	var perms []string
	if err := json.Unmarshal([]byte(raw), &perms); err != nil {
		return nil
	}
	return perms
}

func GetAllRoles() []string {
	return []string{RoleAdmin, RoleUser}
}

func GetAllPermissions() []string {
	return []string{
		PermAgentsRead, PermAgentsWrite, PermAgentsDelete,
		PermListenersRead, PermListenersWrite, PermListenersDelete,
		PermTasksRead, PermTasksWrite, PermTasksDelete,
		PermCredsRead, PermCredsWrite, PermCredsDelete,
		PermFilesRead, PermFilesWrite,
		PermUsersRead, PermUsersWrite, PermUsersDelete,
		PermSettingsRead, PermSettingsWrite,
		PermAuditRead,
		PermGroupsRead, PermGroupsWrite,
		PermWorkflowsRead, PermWorkflowsWrite,
		PermPluginsRead, PermPluginsWrite, PermPluginsExecute, PermPluginsDelete,
		PermRolesRead, PermRolesWrite,
		PermCampaignsRead, PermCampaignsWrite,
		PermOpsecRead, PermOpsecWrite,
		PermIntelRead, PermIntelWrite,
		PermSchedulerRead, PermSchedulerWrite,
		PermNotificationsRead, PermNotificationsWrite,
	}
}

type MeshPeer struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	AgentID   string    `gorm:"index;size:255" json:"agent_id"`
	PeerID    string    `gorm:"size:255" json:"peer_id"`
	PeerAddr  string    `gorm:"size:255" json:"peer_addr"`
	Latency   int       `json:"latency"` // milliseconds
	LastSeen  time.Time `json:"last_seen"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (MeshPeer) TableName() string { return "mesh_peers" }

type BloodHoundResult struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	AgentID          string    `gorm:"index" json:"agent_id"`
	TaskID           uint      `json:"task_id"`
	CollectionMethod string    `json:"collection_method"`
	FilePath         string    `json:"file_path"` // path to saved ZIP on server
	JSONPath         string    `json:"json_path"` // path to parsed JSON output (optional)
	Summary          string    `json:"summary"`
	UserCount        int       `json:"user_count"`
	ComputerCount    int       `json:"computer_count"`
	GroupCount       int       `json:"group_count"`
	SessionCount     int       `json:"session_count"`
	DomainAdminCount int       `json:"domain_admin_count"`
	SPNCount         int       `json:"spn_count"`
	CreatedAt        time.Time `gorm:"index" json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (BloodHoundResult) TableName() string { return "bloodhound_results" }

type BloodHoundFile struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:256" json:"name"`
	Data      []byte    `json:"-"`
	Size      int64     `json:"size"`
	Active    bool      `gorm:"default:true" json:"active"`
	CreatedBy string    `gorm:"size:64" json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (BloodHoundFile) TableName() string { return "bloodhound_files" }

type Campaign struct {
	ID          string    `gorm:"primaryKey;size:36" json:"id"`
	Name        string    `gorm:"size:256;not null" json:"name"`
	Description string    `gorm:"size:1024" json:"description"`
	Status      string    `gorm:"size:32;default:active" json:"status"` // active, completed, archived
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Agents      []Implant `gorm:"many2many:campaign_agents;" json:"agents,omitempty"`
}

func (Campaign) TableName() string { return "campaigns" }

type CampaignAgent struct {
	CampaignID string    `gorm:"primaryKey;size:36" json:"campaign_id"`
	AgentID    string    `gorm:"primaryKey;size:36" json:"agent_id"`
	AddedAt    time.Time `json:"added_at"`
}

func (CampaignAgent) TableName() string { return "campaign_agents" }

type OpsecHistory struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	AgentID   string    `gorm:"index" json:"agent_id"`
	TaskType  string    `json:"task_type"`
	RuleName  string    `json:"rule_name"`
	Allowed   bool      `json:"allowed"`
	Message   string    `json:"message"`
	RiskLevel int       `json:"risk_level"`
	Username  string    `json:"username"`
	Hostname  string    `json:"hostname"`
	CreatedAt time.Time `json:"created_at"`
}

func (OpsecHistory) TableName() string { return "opsec_history" }

type OpsecRule struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	Name          string    `gorm:"uniqueIndex;size:128" json:"name"`
	Description   string    `gorm:"size:512" json:"description"`
	RiskLevel     int       `json:"risk_level"`                  // 1=Low, 2=Medium, 3=High, 4=Critical
	DefaultAction int       `json:"default_action"`              // 0=Block, 1=Warn, 2=Bypass
	CheckType     string    `gorm:"size:64" json:"check_type"`   // maps to compiled Go check function
	TaskTypes     string    `gorm:"type:text" json:"task_types"` // comma-separated task type filter
	Conditions    string    `gorm:"type:text" json:"conditions"` // JSON condition rules
	Enabled       bool      `gorm:"default:true" json:"enabled"`
	CreatedBy     string    `gorm:"size:64" json:"created_by"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (OpsecRule) TableName() string { return "opsec_rules" }

type CircuitBreakerConfig struct {
	ID                 uint `gorm:"primaryKey" json:"id"`
	FailureThreshold   int  `gorm:"default:3" json:"failure_threshold"`
	CooldownSeconds    int  `gorm:"default:300" json:"cooldown_seconds"`
	HalfOpenMaxReqs    int  `gorm:"default:3" json:"half_open_max_reqs"`
	HealthCheckSeconds int  `gorm:"default:60" json:"health_check_seconds"`
}

func (CircuitBreakerConfig) TableName() string { return "circuit_breaker_configs" }

type CircuitBreakerEvent struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	ListenerID string    `gorm:"index" json:"listener_id"`
	OldState   string    `json:"old_state"`
	NewState   string    `json:"new_state"`
	Reason     string    `json:"reason"`
	CreatedAt  time.Time `json:"created_at"`
}

func (CircuitBreakerEvent) TableName() string { return "circuit_breaker_events" }

type CustomRole struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"uniqueIndex;size:32" json:"name"`
	Description string    `gorm:"size:256" json:"description"`
	Permissions string    `gorm:"type:text" json:"-"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (CustomRole) TableName() string { return "custom_roles" }

type SessionRecording struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TaskID    uint      `gorm:"index" json:"task_id"`
	AgentID   string    `gorm:"index" json:"agent_id"`
	Operator  string    `json:"operator"`
	Action    string    `json:"action"`
	Detail    string    `json:"detail"`
	Result    string    `json:"result"`
	Timestamp time.Time `json:"timestamp"`
}

func (SessionRecording) TableName() string { return "session_recordings" }

type PhishingTemplate struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:255;not null" json:"name"`
	Subject   string    `gorm:"size:512" json:"subject"`
	Body      string    `gorm:"type:text" json:"body"`
	FromName  string    `gorm:"size:255" json:"from_name"`
	FromEmail string    `gorm:"size:255" json:"from_email"`
	Type      string    `gorm:"size:50;default:html" json:"type"`
	CreatedBy string    `gorm:"size:255" json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (PhishingTemplate) TableName() string { return "phishing_templates" }

type PhishingCampaign struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Name       string    `gorm:"size:255;not null" json:"name"`
	TemplateID uint      `json:"template_id"`
	TargetList string    `gorm:"type:text" json:"target_list"`
	SMTPHost   string    `gorm:"size:255" json:"smtp_host"`
	SMTPPort   int       `json:"smtp_port"`
	SMTPUser   string    `gorm:"size:255" json:"smtp_user"`
	SMTPPass   string    `gorm:"size:255" json:"smtp_pass"`
	Status     string    `gorm:"size:50;default:draft" json:"status"`
	SentCount  int       `json:"sent_count"`
	OpenCount  int       `json:"open_count"`
	CredCount  int       `json:"cred_count"`
	CreatedBy  string    `gorm:"size:255" json:"created_by"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (PhishingCampaign) TableName() string { return "phishing_campaigns" }

func (c *PhishingCampaign) BeforeCreate(tx *gorm.DB) error {
	return encryptField(&c.SMTPPass)
}

func (c *PhishingCampaign) AfterFind(tx *gorm.DB) error {
	return decryptField(&c.SMTPPass)
}

func (c *PhishingCampaign) BeforeUpdate(tx *gorm.DB) error {
	return encryptField(&c.SMTPPass)
}

type PhishingEvent struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	CampaignID uint      `gorm:"index" json:"campaign_id"`
	Token      string    `gorm:"size:255;index" json:"token"`
	Email      string    `json:"email"`
	EventType  string    `gorm:"size:50" json:"event_type"`
	Payload    string    `gorm:"type:text" json:"payload"`
	IP         string    `json:"ip"`
	UserAgent  string    `json:"user_agent"`
	CreatedAt  time.Time `json:"created_at"`
}

func (PhishingEvent) TableName() string { return "phishing_events" }

func (e *PhishingEvent) BeforeCreate(tx *gorm.DB) error {
	return encryptField(&e.Payload)
}

func (e *PhishingEvent) AfterFind(tx *gorm.DB) error {
	return decryptField(&e.Payload)
}

func (e *PhishingEvent) BeforeUpdate(tx *gorm.DB) error {
	return encryptField(&e.Payload)
}

type AgentTag struct {
	ID        string     `gorm:"primaryKey;size:36" json:"id"`
	Name      string     `gorm:"uniqueIndex;size:100;not null" json:"name"`
	Color     string     `gorm:"size:7;default:#3498db" json:"color"` // hex color
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	Agents    []*Implant `gorm:"many2many:agent_tag_assignments;" json:"agents,omitempty"`
}

type AgentTagAssignment struct {
	AgentTagID string    `gorm:"primaryKey;size:36" json:"tag_id"`
	ImplantID  string    `gorm:"primaryKey;size:36" json:"agent_id"`
	CreatedAt  time.Time `json:"created_at"`
}

func (AgentTag) TableName() string           { return "agent_tags" }
func (AgentTagAssignment) TableName() string { return "agent_tag_assignments" }

// AutoTagRule defines a rule to automatically apply tags to agents.
type AutoTagRule struct {
	ID        string    `gorm:"primaryKey;size:36" json:"id"`
	Name      string    `gorm:"size:100;not null" json:"name"`
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	Condition string    `gorm:"type:text;not null" json:"condition"`
	TagID     string    `gorm:"size:36;not null" json:"tag_id"`
	Tag       *AgentTag `gorm:"foreignKey:TagID" json:"tag,omitempty"`
	Priority  int       `gorm:"default:0" json:"priority"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ScheduledTask is a task that runs on a schedule.
type ScheduledTask struct {
	ID        string    `gorm:"primaryKey;size:36" json:"id"`
	Name      string    `gorm:"size:200;not null" json:"name"`
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	AgentID   string    `gorm:"size:36;not null" json:"agent_id"`
	TaskType  string    `gorm:"size:50;not null" json:"task_type"`
	Command   string    `gorm:"type:text" json:"command"`
	Params    string    `gorm:"type:text" json:"params"`
	Schedule  string    `gorm:"size:100;not null" json:"schedule"`
	LastRun   time.Time `json:"last_run"`
	NextRun   time.Time `json:"next_run"`
	RunCount  int       `gorm:"default:0" json:"run_count"`
	CreatedBy string    `gorm:"size:100" json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Notification struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Type      string    `gorm:"size:50;not null;index" json:"type"`
	Title     string    `gorm:"size:255" json:"title"`
	Message   string    `gorm:"type:text" json:"message"`
	AgentID   string    `gorm:"size:36;index" json:"agent_id"`
	TaskID    uint      `json:"task_id,omitempty"`
	Severity  string    `gorm:"size:20;default:'info'" json:"severity"`
	Read      bool      `gorm:"default:false;index" json:"read"`
	CreatedAt time.Time `json:"created_at"`
}

func (Notification) TableName() string { return "notifications" }

// AgentGroup — hierarchical agent grouping (parent_id for nesting)
type AgentGroup struct {
	ID          string        `gorm:"primaryKey;size:36" json:"id"`
	Name        string        `gorm:"size:200;not null" json:"name"`
	Description string        `gorm:"size:500" json:"description"`
	Color       string        `gorm:"size:7;default:#2ecc71" json:"color"`
	ParentID    *string       `gorm:"size:36;index" json:"parent_id"`
	Parent      *AgentGroup   `gorm:"foreignKey:ParentID" json:"parent,omitempty"`
	Children    []*AgentGroup `gorm:"foreignKey:ParentID" json:"children,omitempty"`
	Agents      []*Implant    `gorm:"many2many:agent_group_assignments;" json:"agents,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

func (AgentGroup) TableName() string { return "agent_groups" }

type AgentGroupAssignment struct {
	AgentGroupID string    `gorm:"primaryKey;size:36" json:"group_id"`
	ImplantID    string    `gorm:"primaryKey;size:36" json:"agent_id"`
	CreatedAt    time.Time `json:"created_at"`
}

func (AgentGroupAssignment) TableName() string { return "agent_group_assignments" }

// Workflow — multi-step batch command pipeline
type Workflow struct {
	ID          string         `gorm:"primaryKey;size:36" json:"id"`
	Name        string         `gorm:"size:200;not null" json:"name"`
	Description string         `gorm:"size:1000" json:"description"`
	Enabled     bool           `gorm:"default:true" json:"enabled"`
	ScopeType   string         `gorm:"size:20;default:'all'" json:"scope_type"` // "all", "tags", "groups", "agents"
	ScopeIDs    string         `gorm:"type:text" json:"scope_ids"`              // JSON array of IDs
	Steps       []WorkflowStep `gorm:"foreignKey:WorkflowID" json:"steps,omitempty"`
	CreatedBy   string         `gorm:"size:100" json:"created_by"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

func (Workflow) TableName() string { return "workflows" }

type WorkflowStep struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	WorkflowID    string    `gorm:"size:36;index;not null" json:"workflow_id"`
	StepOrder     int       `gorm:"not null" json:"step_order"`
	TaskType      string    `gorm:"size:50;not null" json:"task_type"`
	Command       string    `gorm:"type:text" json:"command"`
	Shell         string    `gorm:"size:20;default:'cmd'" json:"shell"`
	TimeoutSec    int       `gorm:"default:60" json:"timeout_sec"`
	RepeatCount   int       `gorm:"default:0" json:"repeat_count"`
	RepeatDelay   int       `gorm:"default:0" json:"repeat_delay"`
	StopOnFailure bool      `gorm:"default:true" json:"stop_on_failure"`
	ConditionExpr string    `gorm:"type:text" json:"condition_expr"` // JSON condition for previous step output
	Condition     string    `gorm:"type:text" json:"condition"`      // condition expression e.g. "contains('success')"
	OnSuccess     string    `gorm:"size:255" json:"on_success"`      // "continue", or step ID to jump to
	OnFailure     string    `gorm:"size:255" json:"on_failure"`      // "abort", "continue", or step ID to jump to
	CreatedAt     time.Time `json:"created_at"`
}

func (WorkflowStep) TableName() string { return "workflow_steps" }

// WorkflowExecution records a single execution of a workflow
type WorkflowExecution struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	WorkflowID   string     `gorm:"size:36;index;not null" json:"workflow_id"`
	WorkflowName string     `gorm:"size:200" json:"workflow_name"`
	AgentIDs     string     `gorm:"type:text" json:"agent_ids"`
	Status       string     `gorm:"size:20;default:'running'" json:"status"`
	TasksCreated int        `json:"tasks_created"`
	AgentsCount  int        `json:"agents_count"`
	ErrorMsg     string     `gorm:"type:text" json:"error_msg,omitempty"`
	StartedAt    time.Time  `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

func (WorkflowExecution) TableName() string { return "workflow_executions" }

// WorkflowStepLog records individual step execution within a workflow execution
type WorkflowStepLog struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	ExecutionID  uint       `gorm:"index;not null" json:"execution_id"`
	StepOrder    int        `json:"step_order"`
	TaskType     string     `gorm:"size:50" json:"task_type"`
	Command      string     `gorm:"type:text" json:"command"`
	TaskID       uint       `json:"task_id"`
	AgentID      string     `gorm:"size:100" json:"agent_id"`
	Status       string     `gorm:"size:20" json:"status"`
	Result       string     `gorm:"type:text" json:"result"`
	BranchAction string     `gorm:"size:50" json:"branch_action,omitempty"`
	BranchTarget string     `gorm:"size:255" json:"branch_target,omitempty"`
	StartedAt    time.Time  `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

func (WorkflowStepLog) TableName() string { return "workflow_step_logs" }

// ChatMessage — multi-operator chat message
type ChatMessage struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Username  string    `gorm:"size:100;not null" json:"username"`
	Message   string    `gorm:"type:text;not null" json:"message"`
	Channel   string    `gorm:"size:50;default:'general'" json:"channel"`
	CreatedAt time.Time `json:"created_at"`
}

func (ChatMessage) TableName() string { return "chat_messages" }

// AIChatSession — a persisted AI assistant conversation
type AIChatSession struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Title     string    `gorm:"size:255;not null;default:'New Chat'" json:"title"`
	Owner     string    `gorm:"size:100;index" json:"owner"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (AIChatSession) TableName() string { return "ai_chat_sessions" }

// AIChatMessage — a single message within an AI conversation
type AIChatMessage struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	SessionID uint      `gorm:"index;not null" json:"session_id"`
	Role      string    `gorm:"size:20;not null" json:"role"` // user | assistant | tool
	Content   string    `gorm:"type:text;not null" json:"content"`
	ToolName  string    `gorm:"size:100" json:"tool_name,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func (AIChatMessage) TableName() string { return "ai_chat_messages" }

// ScheduledReport — recurring report generation and delivery
type ScheduledReport struct {
	ID            string    `gorm:"primaryKey;size:36" json:"id"`
	Name          string    `gorm:"size:200;not null" json:"name"`
	Enabled       bool      `gorm:"default:true" json:"enabled"`
	Schedule      string    `gorm:"size:100;not null" json:"schedule"` // "daily HH:MM", "weekly Sun 09:00", etc.
	Format        string    `gorm:"size:10;default:'html'" json:"format"`
	IncludeAgents bool      `gorm:"default:true" json:"include_agents"`
	IncludeTasks  bool      `gorm:"default:true" json:"include_tasks"`
	IncludeCreds  bool      `gorm:"default:true" json:"include_creds"`
	IncludeAudit  bool      `gorm:"default:true" json:"include_audit"`
	DeliveryType  string    `gorm:"size:20" json:"delivery_type"` // "", "email", "webhook", "file"
	DeliveryTo    string    `gorm:"size:500" json:"delivery_to"`  // email addr / webhook URL / file path
	LastRun       time.Time `json:"last_run"`
	NextRun       time.Time `json:"next_run"`
	RunCount      int       `gorm:"default:0" json:"run_count"`
	CreatedBy     string    `gorm:"size:100" json:"created_by"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (ScheduledReport) TableName() string { return "scheduled_reports" }

func (AutoTagRule) TableName() string   { return "auto_tag_rules" }
func (ScheduledTask) TableName() string { return "scheduled_tasks" }

type StagerToken struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Token        string    `gorm:"uniqueIndex;size:512" json:"token"`
	ListenerID   uint      `gorm:"index" json:"listener_id"`
	Architecture string    `json:"arch"`
	OS           string    `json:"os"`
	Format       string    `json:"format"`
	Used         bool      `gorm:"default:false;index" json:"used"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedBy    string    `json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (StagerToken) TableName() string { return "stager_tokens" }

type Script struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:255;not null" json:"name"`
	Description string    `gorm:"size:1024" json:"description"`
	Code        string    `gorm:"type:text;not null" json:"code"`
	Events      string    `gorm:"type:text" json:"events"`
	Enabled     bool      `gorm:"default:true" json:"enabled"`
	RunCount    int       `gorm:"default:0" json:"run_count"`
	LastRun     time.Time `json:"last_run"`
	CreatedBy   string    `gorm:"size:64" json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (Script) TableName() string { return "scripts" }

type Redirector struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:100" json:"name"`
	Host        string    `gorm:"size:255" json:"host"`
	Type        string    `gorm:"size:20" json:"type"`
	Status      string    `gorm:"size:20;default:'inactive'" json:"status"`
	Config      string    `gorm:"type:text" json:"config"`
	SSHUser     string    `gorm:"size:64" json:"ssh_user"`
	SSHPort     int       `gorm:"default:22" json:"ssh_port"`
	SSHKey      string    `gorm:"type:text" json:"ssh_key"`
	SSHPassword string    `gorm:"type:text" json:"ssh_password"`
	LastSeen    time.Time `json:"last_seen"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (Redirector) TableName() string { return "redirectors" }

// AgentLock represents a collaboration lock placed on an agent by an operator.
// Used to prevent two operators from driving the same agent simultaneously.
type AgentLock struct {
	ID        string    `gorm:"primaryKey;size:36" json:"id"`
	AgentID   string    `gorm:"uniqueIndex;size:36;not null" json:"agent_id"`
	LockedBy  string    `gorm:"size:64;not null" json:"locked_by"`
	LockedAt  time.Time `json:"locked_at"`
	Note      string    `gorm:"size:512" json:"note"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (AgentLock) TableName() string { return "agent_locks" }

// CloudCred stores cloud provider credentials stolen from an agent host
// (AWS / GCP / Azure access keys, tokens, etc.).
type CloudCred struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	AgentID   string    `gorm:"index;size:36;not null" json:"agent_id"`
	Provider  string    `gorm:"size:32;not null" json:"provider"` // aws | gcp | azure
	Type      string    `gorm:"size:64" json:"type"`              // e.g. access_key, token, sa_key
	Key       string    `gorm:"size:512" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	Extra     string    `gorm:"type:text" json:"extra"`
	CreatedAt time.Time `json:"created_at"`
}

func (CloudCred) TableName() string { return "cloud_creds" }

// ExtC2Channel stores configuration for External C2 channels (Discord, Slack).
type ExtC2Channel struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Type      string    `gorm:"size:20;not null" json:"type"` // "discord" or "slack"
	BotToken  string    `gorm:"size:500" json:"bot_token"`
	ChannelID string    `gorm:"size:100" json:"channel_id"`
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

func (ExtC2Channel) TableName() string { return "extc2_channels" }

// AgentStatusEvent records when an agent changes online/offline status.
type AgentStatusEvent struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	AgentID   string    `gorm:"index:idx_status_agent_time,priority:1;size:36;not null" json:"agent_id"`
	Status    string    `gorm:"size:20;not null" json:"status"` // "online", "offline", "stale"
	Timestamp time.Time `gorm:"index:idx_status_agent_time,priority:2" json:"timestamp"`
}

func (AgentStatusEvent) TableName() string { return "agent_status_events" }

type BackupCode struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	CodeHash  string    `gorm:"size:128;not null" json:"-"`
	Used      bool      `gorm:"default:false" json:"used"`
	UsedAt    time.Time `json:"used_at"`
	CreatedAt time.Time `json:"created_at"`
}

func (BackupCode) TableName() string { return "backup_codes" }

type UserSession struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	UserID            uint      `gorm:"index;not null" json:"user_id"`
	TokenHash         string    `gorm:"size:64;index;not null" json:"-"`
	IP                string    `gorm:"size:45" json:"ip"`
	UserAgent         string    `gorm:"size:512" json:"user_agent"`
	DeviceFingerprint string    `gorm:"size:128;index" json:"device_fingerprint,omitempty"`
	ExpiresAt         time.Time `json:"expires_at"`
	RevokedAt         time.Time `json:"revoked_at"`
	CreatedAt         time.Time `json:"created_at"`
}

func (UserSession) TableName() string { return "user_sessions" }

type PasswordHistory struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `gorm:"index;not null" json:"user_id"`
	PasswordHash string    `gorm:"size:256;not null" json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

func (PasswordHistory) TableName() string { return "password_history" }

// ApiKey represents a programmatic API key for REST API authentication.
type ApiKey struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	Name      string    `gorm:"size:128;not null" json:"name"`
	KeyHash   string    `gorm:"size:64;uniqueIndex;not null" json:"-"`
	Prefix    string    `gorm:"size:12;not null" json:"prefix"`
	LastUsed  time.Time `json:"last_used"`
	ExpiresAt time.Time `json:"expires_at"`
	Active    bool      `gorm:"default:true" json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (ApiKey) TableName() string { return "api_keys" }
