package db

import (
	"log/slog"
	"os"
	"path/filepath"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	// 纯 Go SQLite 驱动（推荐用于 Windows 无 CGO 环境）
	glebarez "github.com/glebarez/sqlite"
)

// InitDB 使用 glebarez/sqlite 纯 Go 驱动（彻底解决 CGO 问题）
func InitDB(dbPath string, logLevel slog.Level) (*gorm.DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, err
	}

	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}

	if logLevel == slog.LevelDebug {
		gormConfig.Logger = logger.Default.LogMode(logger.Info)
	}

	// 使用 glebarez/sqlite 打开数据库
	db, err := gorm.Open(glebarez.Open(dbPath), gormConfig)
	if err != nil {
		return nil, err
	}

	// 自动迁移表结构
	if err := db.AutoMigrate(&Agent{}, &Task{}, &AuditLog{}); err != nil {
		return nil, err
	}

	// 开启 SQLite 外键约束
	db.Exec("PRAGMA foreign_keys = ON;")

	slog.Info("Database initialized", "path", dbPath)
	return db, nil
}
