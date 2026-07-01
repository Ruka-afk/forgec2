package db

import (
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	// Pure Go SQLite driver (recommended for Windows without CGO)
	glebarez "github.com/glebarez/sqlite"
)

var queryCache sync.Map
var cacheExpiry = 5 * time.Minute

type cacheEntry struct {
	data      interface{}
	timestamp time.Time
}

func GetFromCache(key string) (interface{}, bool) {
	if entry, ok := queryCache.Load(key); ok {
		c := entry.(cacheEntry)
		if time.Since(c.timestamp) < cacheExpiry {
			return c.data, true
		}
		queryCache.Delete(key)
	}
	return nil, false
}

func SetCache(key string, data interface{}) {
	queryCache.Store(key, cacheEntry{
		data:      data,
		timestamp: time.Now(),
	})
}

func InvalidateCache(prefix string) {
	queryCache.Range(func(key, value interface{}) bool {
		if k, ok := key.(string); ok && len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			queryCache.Delete(key)
		}
		return true
	})
}

func ClearCache() {
	queryCache = sync.Map{}
}

// InitDB initializes the database using glebarez/sqlite pure Go driver
func InitDB(dbPath string, logLevel slog.Level) (*gorm.DB, error) {
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
	_ = db.Exec("ALTER TABLE agents RENAME TO implants").Error

	// Auto-migrate all models
	if err := db.AutoMigrate(&Implant{}, &Task{}, &AuditLog{}, &Listener{}, &TokenEntry{}, &SocksSession{}, &CredentialEntry{}, &User{}, &BuildLog{}, &AgentLock{}, &ScanResult{}, &ChatMessage{}, &OperatorNote{}, &NetworkHost{}, &CommandTemplate{}, &BOFFile{}, &ServerConfig{}, &WebhookConfig{}, &Plugin{}, &PluginReview{}, &PluginDependency{}, &PluginUpdateStatus{}, &RolePermission{}, &AutomationRule{}, &AlertRule{}, &Alert{}, &SystemMetric{}); err != nil {
		return nil, err
	}

	// Seed role permissions
	seedRolePermissions(db)

	// Ensure new columns exist (glebarez/sqlite AutoMigrate may not add all; ignore "duplicate column" errors)
	_ = db.Exec("ALTER TABLE implants ADD COLUMN pid INTEGER DEFAULT 0").Error
	_ = db.Exec("ALTER TABLE implants ADD COLUMN public_ip TEXT DEFAULT ''").Error
	_ = db.Exec("ALTER TABLE implants ADD COLUMN country TEXT DEFAULT ''").Error
	_ = db.Exec("ALTER TABLE implants ADD COLUMN city TEXT DEFAULT ''").Error
	_ = db.Exec("ALTER TABLE implants ADD COLUMN latitude REAL DEFAULT 0").Error
	_ = db.Exec("ALTER TABLE implants ADD COLUMN longitude REAL DEFAULT 0").Error
	_ = db.Exec("ALTER TABLE implants ADD COLUMN active_window TEXT DEFAULT ''").Error

	// Seed default admin user if none exist
	var userCount int64
	db.Model(&User{}).Count(&userCount)
	if userCount == 0 {
		defaultAdminHash, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
		if err != nil {
			slog.Error("Failed to hash default admin password", "err", err)
		} else {
			db.Create(&User{
				Username:     "admin",
				PasswordHash: string(defaultAdminHash),
				Role:         "admin",
				IsActive:     true,
			})
			slog.Info("Default admin user created with password 'admin'")
		}
	} else {
		var admin User
		result := db.Where("username = ?", "admin").First(&admin)
		if result.Error == nil {
			if os.Getenv("FORGEC2_RESET_ADMIN_PASSWORD") == "1" {
				newHash, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
				if err == nil {
					db.Model(&admin).Update("password_hash", string(newHash))
					slog.Info("Admin password reset to 'admin' via FORGEC2_RESET_ADMIN_PASSWORD")
				}
			}
		}
	}

	// Fix: Clear force_logout_at for all users (prevents persistent logout bug)
	db.Model(&User{}).Where("force_logout_at IS NOT NULL").Update("force_logout_at", nil)
	slog.Info("Cleared force_logout_at flags for all users")

	// Enable SQLite foreign key constraints
	db.Exec("PRAGMA foreign_keys = ON;")

	// Performance indexes for common queries
	db.Exec("CREATE INDEX IF NOT EXISTS idx_implants_last_seen ON implants(last_seen)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_implants_status ON implants(status)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_implants_listener_id ON implants(listener_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_implants_hostname ON implants(hostname)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_implants_ip ON implants(ip)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_tasks_agent_status_created ON tasks(agent_id, status, created_at)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_tasks_created_status ON tasks(created_at, status)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_tasks_type ON tasks(type)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_credential_entries_agent_id ON credential_entries(agent_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_credential_entries_source ON credential_entries(source)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_credential_entries_created ON credential_entries(created_at)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_audit_user ON audit_logs(user)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_logs(action)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_logs(created_at)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_scan_agent_id ON scan_results(agent_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_scan_created ON scan_results(created_at)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_chat_user ON chat_messages(user)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_chat_created ON chat_messages(created_at DESC)")

	// Additional indexes for common queries
	db.Exec("CREATE INDEX IF NOT EXISTS idx_implants_username ON implants(username)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_implants_os ON implants(os)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_implants_arch ON implants(arch)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_implants_elevated ON implants(elevated)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_implants_created ON implants(created_at)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_implants_parent_id ON implants(parent_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_tasks_result ON tasks(result)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_users_role ON users(role)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_users_active ON users(is_active)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_listeners_enabled ON listeners(enabled)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_listeners_scheme ON listeners(scheme)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_token_entries_active ON token_entries(active)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_token_entries_domain ON token_entries(domain)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_socks_sessions_status ON socks_sessions(status)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_credentials_type ON credential_entries(type)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_credentials_confirmed ON credential_entries(confirmed)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_build_logs_user ON build_logs(user)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_build_logs_status ON build_logs(status)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_network_hosts_ip ON network_hosts(ip)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_operator_notes_agent ON operator_notes(agent_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_command_templates_category ON command_templates(category)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_alerts_status ON alerts(status)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_alerts_severity ON alerts(severity)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_alerts_type ON alerts(type)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_alert_rules_enabled ON alert_rules(enabled)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_system_metrics_created ON system_metrics(created_at)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_automation_rules_enabled ON automation_rules(enabled)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_automation_rules_event ON automation_rules(event_type)")

	// SQLite performance optimizations
	db.Exec("PRAGMA journal_mode = WAL;")
	db.Exec("PRAGMA cache_size = -2000;")
	db.Exec("PRAGMA temp_store = MEMORY;")
	db.Exec("PRAGMA synchronous = NORMAL;")
	db.Exec("PRAGMA mmap_size = 268435456;")

	// Configure connection pool (optimization)
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(25)                 // Maximum open connections
	sqlDB.SetMaxIdleConns(10)                 // Maximum idle connections
	sqlDB.SetConnMaxLifetime(5 * time.Minute) // Connection max lifetime

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

func MigrateExistingUsersToAdmin(db *gorm.DB) {
	var users []User
	db.Find(&users)
	for _, user := range users {
		if user.Role == "" {
			db.Model(&user).Update("role", RoleAdmin)
		}
	}
}
