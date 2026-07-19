package db

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	// Pure Go SQLite driver (recommended for Windows without CGO)
	glebarez "github.com/glebarez/sqlite"
)

// execMigration runs a migration SQL statement and logs unexpected errors.
// Duplicate column/index errors are expected during migrations and are silently ignored.
func execMigration(db *gorm.DB, sql, label string) {
	if err := db.Exec(sql).Error; err != nil && !isMigrationIgnorable(err) {
		slog.Warn("Migration warning", "label", label, "err", err)
	}
}

func isMigrationIgnorable(err error) bool {
	s := err.Error()
	for _, kw := range []string{"duplicate column", "already exists", "no such table"} {
		if len(s) >= len(kw) {
			for i := 0; i <= len(s)-len(kw); i++ {
				if s[i:i+len(kw)] == kw {
					return true
				}
			}
		}
	}
	return false
}

// generateRandomPassword creates a random alphanumeric password of the given length.
func generateRandomPassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		// Fallback to hex if rand fails
		key := make([]byte, length/2+1)
		rand.Read(key)
		return hex.EncodeToString(key)[:length]
	}
	for i, v := range b {
		b[i] = charset[v%byte(len(charset))]
	}
	return string(b)
}

// queryCache is a bounded TTL cache replacing the old unbounded sync.Map.
// Max 1000 entries, 5-minute expiry.
var queryCache = NewTTLCache(1000, 5*time.Minute)

func GetFromCache(key string) (interface{}, bool) {
	return queryCache.Get(key)
}

func SetCache(key string, data interface{}) {
	queryCache.Set(key, data)
}

func InvalidateCache(prefix string) {
	queryCache.InvalidateByPrefix(prefix)
}

func ClearCache() {
	queryCache.Clear()
}

// InitDB initializes the database using glebarez/sqlite pure Go driver
func InitDB(dbPath string, logLevel slog.Level, defaultPassword ...string) (*gorm.DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0700); err != nil {
		return nil, err
	}

	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}

	if logLevel == slog.LevelDebug {
		gormConfig.Logger = logger.Default.LogMode(logger.Info)
	}

	// Open database with glebarez/sqlite
	db, err := gorm.Open(glebarez.Open(dbPath), gormConfig)
	if err != nil {
		return nil, err
	}

	// Rename legacy table (Agent → Implant rename, ignore if already renamed)
	execMigration(db, "ALTER TABLE agents RENAME TO implants", "rename_agents_to_implants")

	// Auto-migrate all models
	if err := db.AutoMigrate(&Implant{}, &Task{}, &AuditLog{}, &Listener{}, &TokenEntry{}, &SocksSession{}, &CredentialEntry{}, &User{}, &BuildLog{}, &ScanResult{}, &NetworkHost{}, &CommandTemplate{}, &BOFFile{}, &BOFLibrary{}, &ServerConfig{}, &WebhookConfig{}, &Plugin{}, &PluginReview{}, &PluginDependency{}, &PluginUpdateStatus{}, &RolePermission{}, &AutomationRule{}, &AlertRule{}, &Alert{}, &SystemMetric{}, &GeneratedReport{}, &Campaign{}, &CampaignAgent{}, &MeshPeer{}, &BloodHoundResult{}, &BloodHoundFile{}, &SessionRecording{}, &OpsecHistory{}, &OpsecRule{}, &CircuitBreakerConfig{}, &CircuitBreakerEvent{}, &CustomRole{}, &PhishingTemplate{}, &PhishingCampaign{}, &PhishingEvent{}, &AgentTag{}, &AgentTagAssignment{}, &AutoTagRule{}, &ScheduledTask{}, &Notification{}, &AgentGroup{}, &AgentGroupAssignment{}, &Workflow{}, &WorkflowStep{}, &WorkflowExecution{}, &WorkflowStepLog{}, &ChatMessage{}, &ScheduledReport{}, &StagerToken{}, &Redirector{}, &AgentLock{}, &CloudCred{}, &AIChatSession{}, &AIChatMessage{}, &ExtC2Channel{}); err != nil {
		return nil, err
	}

	// Seed role permissions
	seedRolePermissions(db)

	// Migrate old roles
	MigrateOldRoles(db)

	// Ensure new columns exist (glebarez/sqlite AutoMigrate may not add all; ignore "duplicate column" errors)
	execMigration(db, "ALTER TABLE implants ADD COLUMN pid INTEGER DEFAULT 0", "add_pid")
	execMigration(db, "ALTER TABLE implants ADD COLUMN public_ip TEXT DEFAULT ''", "add_public_ip")
	execMigration(db, "ALTER TABLE implants ADD COLUMN country TEXT DEFAULT ''", "add_country")
	execMigration(db, "ALTER TABLE implants ADD COLUMN city TEXT DEFAULT ''", "add_city")
	execMigration(db, "ALTER TABLE implants ADD COLUMN latitude REAL DEFAULT 0", "add_latitude")
	execMigration(db, "ALTER TABLE implants ADD COLUMN longitude REAL DEFAULT 0", "add_longitude")
	execMigration(db, "ALTER TABLE implants ADD COLUMN active_window TEXT DEFAULT ''", "add_active_window")
	execMigration(db, "ALTER TABLE implants ADD COLUMN trusted INTEGER DEFAULT 0", "add_trusted")

	// Seed default admin user if none exist
	var userCount int64
	db.Model(&User{}).Count(&userCount)
	if userCount == 0 {
		// Use configured default password, or generate a random one
		adminPass := ""
		if len(defaultPassword) > 0 && defaultPassword[0] != "" {
			adminPass = defaultPassword[0]
		} else {
			adminPass = generateRandomPassword(12)
		}
		defaultAdminHash, err := bcrypt.GenerateFromPassword([]byte(adminPass), bcrypt.DefaultCost)
		if err != nil {
			slog.Error("Failed to hash default admin password", "err", err)
		} else {
			db.Create(&User{
				Username:            "admin",
				PasswordHash:        string(defaultAdminHash),
				Role:                "admin",
				IsActive:            true,
				ForcePasswordChange: true,
			})
			slog.Warn("╔══════════════════════════════════════════════════════════╗")
			slog.Warn("║  DEFAULT ADMIN CREDENTIALS (CHANGE IMMEDIATELY!)       ║")
			slog.Warn(fmt.Sprintf("║  Username: admin  Password: %-24s  ║", adminPass))
			slog.Warn("║  You will be prompted to change this on first login.   ║")
			slog.Warn("╚══════════════════════════════════════════════════════════╝")
		}
	}

	// Fix: Clear force_logout_at for all users (prevents persistent logout bug)
	db.Model(&User{}).Where("force_logout_at IS NOT NULL").Update("force_logout_at", nil)
	slog.Info("Cleared force_logout_at flags for all users")

	// Enable SQLite foreign key constraints
	db.Exec("PRAGMA foreign_keys = ON;")

	// Performance indexes for common queries
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_implants_last_seen ON implants(last_seen)", "idx_implants_last_seen")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_implants_status ON implants(status)", "idx_implants_status")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_implants_listener_id ON implants(listener_id)", "idx_implants_listener_id")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_implants_hostname ON implants(hostname)", "idx_implants_hostname")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_implants_ip ON implants(ip)", "idx_implants_ip")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_tasks_agent_status_created ON tasks(agent_id, status, created_at)", "idx_tasks_agent_status_created")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_tasks_created_status ON tasks(created_at, status)", "idx_tasks_created_status")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_tasks_type ON tasks(type)", "idx_tasks_type")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_credential_entries_agent_id ON credential_entries(agent_id)", "idx_credential_entries_agent_id")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_credential_entries_source ON credential_entries(source)", "idx_credential_entries_source")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_credential_entries_created ON credential_entries(created_at)", "idx_credential_entries_created")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_audit_user ON audit_logs(user)", "idx_audit_user")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_logs(action)", "idx_audit_action")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_logs(created_at)", "idx_audit_created")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_scan_agent_id ON scan_results(agent_id)", "idx_scan_agent_id")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_scan_created ON scan_results(created_at)", "idx_scan_created")

	// Additional indexes for common queries
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_implants_username ON implants(username)", "idx_implants_username")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_implants_os ON implants(os)", "idx_implants_os")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_implants_arch ON implants(arch)", "idx_implants_arch")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_implants_elevated ON implants(elevated)", "idx_implants_elevated")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_implants_created ON implants(created_at)", "idx_implants_created")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_implants_parent_id ON implants(parent_id)", "idx_implants_parent_id")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)", "idx_users_username")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_users_role ON users(role)", "idx_users_role")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_users_active ON users(is_active)", "idx_users_active")
	execMigration(db, "ALTER TABLE listeners ADD COLUMN tags VARCHAR(500) DEFAULT ''", "add_listeners_tags")
	execMigration(db, "ALTER TABLE listeners ADD COLUMN color VARCHAR(7) DEFAULT ''", "add_listeners_color")
	execMigration(db, "ALTER TABLE listeners ADD COLUMN status VARCHAR(20) DEFAULT 'running'", "add_listeners_status")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_listeners_enabled ON listeners(enabled)", "idx_listeners_enabled")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_listeners_scheme ON listeners(scheme)", "idx_listeners_scheme")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_token_entries_active ON token_entries(active)", "idx_token_entries_active")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_token_entries_domain ON token_entries(domain)", "idx_token_entries_domain")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_socks_sessions_status ON socks_sessions(status)", "idx_socks_sessions_status")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_credentials_type ON credential_entries(type)", "idx_credentials_type")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_credentials_confirmed ON credential_entries(confirmed)", "idx_credentials_confirmed")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_build_logs_user ON build_logs(user)", "idx_build_logs_user")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_build_logs_status ON build_logs(status)", "idx_build_logs_status")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_network_hosts_ip ON network_hosts(ip)", "idx_network_hosts_ip")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_command_templates_category ON command_templates(category)", "idx_command_templates_category")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_alerts_status ON alerts(status)", "idx_alerts_status")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_alerts_severity ON alerts(severity)", "idx_alerts_severity")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_alerts_type ON alerts(type)", "idx_alerts_type")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_alert_rules_enabled ON alert_rules(enabled)", "idx_alert_rules_enabled")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_system_metrics_created ON system_metrics(created_at)", "idx_system_metrics_created")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_automation_rules_enabled ON automation_rules(enabled)", "idx_automation_rules_enabled")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_automation_rules_event ON automation_rules(event_type)", "idx_automation_rules_event")

	// Composite indexes for common query patterns
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_tasks_agent_created ON tasks(agent_id, created_at DESC)", "idx_tasks_agent_created")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_credential_entries_agent_created ON credential_entries(agent_id, created_at)", "idx_credential_entries_agent_created")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_notifications_type_read_created ON notifications(type, read, created_at)", "idx_notifications_type_read_created")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_notifications_read_created ON notifications(read, created_at)", "idx_notifications_read_created")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_implants_status_last_seen ON implants(status, last_seen)", "idx_implants_status_last_seen")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_audit_logs_action_created ON audit_logs(action, created_at)", "idx_audit_logs_action_created")

	// Security fix: additional query-path indexes
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_tasks_agent_status ON tasks(agent_id, status)", "idx_tasks_agent_status")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status)", "idx_tasks_status")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_tasks_type ON tasks(type)", "idx_tasks_type")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_cred_entries_type ON credential_entries(type)", "idx_cred_entries_type")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_alerts_status_severity ON alerts(status, severity)", "idx_alerts_status_severity")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_alerts_rule_source ON alerts(rule_id, source, status)", "idx_alerts_rule_source")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_chat_messages_channel ON chat_messages(channel)", "idx_chat_messages_channel")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_scheduled_tasks_next_run ON scheduled_tasks(next_run, enabled)", "idx_scheduled_tasks_next_run")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_opsec_history_agent_created ON opsec_history(agent_id, created_at)", "idx_opsec_history_agent_created")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_network_hosts_agent_ip ON network_hosts(agent_id, ip)", "idx_network_hosts_agent_ip")
	execMigration(db, "CREATE INDEX IF NOT EXISTS idx_phishing_events_type_created ON phishing_events(event_type, created_at)", "idx_phishing_events_type_created")

	// SQLite performance optimizations
	execMigration(db, "PRAGMA journal_mode = WAL;", "pragma_wal")
	execMigration(db, "PRAGMA cache_size = -2000;", "pragma_cache")
	execMigration(db, "PRAGMA temp_store = MEMORY;", "pragma_temp_store")
	execMigration(db, "PRAGMA synchronous = NORMAL;", "pragma_sync")
	execMigration(db, "PRAGMA mmap_size = 268435456;", "pragma_mmap")

	slog.Info("Database initialized", "path", dbPath)
	return db, nil
}

func seedRolePermissions(db *gorm.DB) {
	var count int64
	db.Model(&RolePermission{}).Count(&count)
	if count > 0 {
		return
	}

	for role, perms := range RolePermissionsMap {
		for _, perm := range perms {
			db.Create(&RolePermission{
				Role:       role,
				Permission: perm,
			})
		}
	}
	slog.Info("Role permissions seeded", "roles", len(RolePermissionsMap))
}

// MigrateOldRoles migrates "operator"/"viewer"/"guest" to "user"
func MigrateOldRoles(db *gorm.DB) {
	result := db.Model(&User{}).Where("role IN ?", []string{"operator", "viewer", "guest"}).Update("role", RoleUser)
	if result.RowsAffected > 0 {
		slog.Info("Migrated old roles to 'user'", "count", result.RowsAffected)
	}
}
