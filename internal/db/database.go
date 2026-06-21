package db

import (
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	// Pure Go SQLite driver (recommended for Windows without CGO)
	glebarez "github.com/glebarez/sqlite"
)

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
	if err := db.AutoMigrate(&Implant{}, &Task{}, &AuditLog{}, &Listener{}, &TokenEntry{}, &SocksSession{}, &CredentialEntry{}, &User{}, &BuildLog{}, &AgentLock{}, &ScanResult{}, &ChatMessage{}, &OperatorNote{}, &NetworkHost{}, &CommandTemplate{}, &BOFFile{}, &ServerConfig{}, &WebhookConfig{}, &Plugin{}); err != nil {
		return nil, err
	}

	// Ensure new columns exist (glebarez/sqlite AutoMigrate may not add all; ignore "duplicate column" errors)
	_ = db.Exec("ALTER TABLE implants ADD COLUMN pid INTEGER DEFAULT 0").Error
	_ = db.Exec("ALTER TABLE implants ADD COLUMN public_ip TEXT DEFAULT ''").Error
	_ = db.Exec("ALTER TABLE implants ADD COLUMN country TEXT DEFAULT ''").Error
	_ = db.Exec("ALTER TABLE implants ADD COLUMN city TEXT DEFAULT ''").Error
	_ = db.Exec("ALTER TABLE implants ADD COLUMN latitude REAL DEFAULT 0").Error
	_ = db.Exec("ALTER TABLE implants ADD COLUMN longitude REAL DEFAULT 0").Error

	// Seed default admin user if none exist
	var userCount int64
	db.Model(&User{}).Count(&userCount)
	if userCount == 0 {
		// Pre-hashed password "admin" using bcrypt (cost=10)
		// Generated with: bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
		defaultAdminHash := "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"
		db.Create(&User{
			Username:     "admin",
			PasswordHash: defaultAdminHash,
			Role:         "admin",
			IsActive:     true,
		})
		slog.Info("Default admin user created with password 'admin'")
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
	db.Exec("CREATE INDEX IF NOT EXISTS idx_credentials_agent_id ON credentials(agent_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_credentials_source ON credentials(source)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_credentials_created ON credentials(created_at)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_audit_user ON audit_logs(user)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_logs(action)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_logs(created_at)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_scan_agent_id ON scan_results(agent_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_scan_type ON scan_results(scan_type)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_scan_created ON scan_results(created_at)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_chat_channel ON chat_messages(channel)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_chat_user ON chat_messages(user)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_chat_created ON chat_messages(created_at DESC)")

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
