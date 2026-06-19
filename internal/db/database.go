package db

import (
	"log/slog"
	"os"
	"path/filepath"

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

	// Auto-migrate all models
	if err := db.AutoMigrate(&Agent{}, &Task{}, &AuditLog{}, &Listener{}, &TokenEntry{}, &SocksSession{}, &CredentialEntry{}, &User{}); err != nil {
		return nil, err
	}

	// Seed default admin user if none exist
	var userCount int64
	db.Model(&User{}).Count(&userCount)
	if userCount == 0 {
		db.Create(&User{
			Username:     "admin",
			PasswordHash: "", // first login will set password
			Role:         "admin",
			IsActive:     true,
		})
		slog.Info("Default admin user created, set password on first login")
	}

	// Enable SQLite foreign key constraints
	db.Exec("PRAGMA foreign_keys = ON;")

	// Performance indexes for common queries (agents by last_seen, tasks by agent+status+created)
	db.Exec("CREATE INDEX IF NOT EXISTS idx_agents_last_seen ON agents(last_seen)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_tasks_agent_status_created ON tasks(agent_id, status, created_at)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_tasks_created_status ON tasks(created_at, status)")

	slog.Info("Database initialized", "path", dbPath)
	return db, nil
}
