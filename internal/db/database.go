package db

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	// Pure Go SQLite driver (recommended for Windows without CGO)
	glebarez "github.com/glebarez/sqlite"
	// PostgreSQL driver (optional, for production deployments)
	"gorm.io/driver/postgres"
)

// lockAwareLogger wraps a GORM logger so SQLite lock contention errors
// ("database is locked") are surfaced through slog even when GORM runs in
// Silent mode, where statement errors would otherwise be swallowed.
type lockAwareLogger struct {
	logger.Interface
}

// Trace intercepts statement execution errors before the wrapped logger
// applies its (possibly Silent) log level filter.
func (l lockAwareLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if err != nil && isSQLiteLockError(err) {
		sql, rows := fc()
		slog.Error("SQLite lock error", "error", err, "sql", sql, "rows", rows)
	}
	l.Interface.Trace(ctx, begin, fc, err)
}

// Error surfaces lock errors reported through the logger's Error path
// (e.g. failed transaction begin) the same way.
func (l lockAwareLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	for _, d := range data {
		if e, ok := d.(error); ok && isSQLiteLockError(e) {
			slog.Error("SQLite lock error", "error", e)
			return
		}
	}
	l.Interface.Error(ctx, msg, data...)
}

func isSQLiteLockError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "database is locked") || strings.Contains(s, "database table is locked")
}

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
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

// generateRandomPassword creates a random alphanumeric password of the given length.
func generateRandomPassword(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	charsetLen := int(len(charset))
	threshold := 256 - (256 % charsetLen)
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crypto/rand.Read failed: %w", err)
	}
	for i, v := range b {
		for int(v) >= threshold {
			if _, err := rand.Read(b[i : i+1]); err != nil {
				return "", fmt.Errorf("crypto/rand.Read failed: %w", err)
			}
			v = b[i]
		}
		b[i] = charset[int(v)%charsetLen]
	}
	return string(b), nil
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
// or PostgreSQL via gorm.io/driver/postgres when driver="postgres" and dsn is set.
func InitDB(dbPath string, logLevel slog.Level, defaultPassword ...string) (*gorm.DB, error) {
	return InitDBWithDriver("", "", dbPath, logLevel, 25, 5, 30*time.Minute, defaultPassword...)
}

func InitDBWithDriver(driver, dsn, fallbackPath string, logLevel slog.Level, dbMaxOpenConns int, dbMaxIdleConns int, dbConnMaxLifetime time.Duration, defaultPassword ...string) (*gorm.DB, error) {
	gormConfig := &gorm.Config{
		Logger: lockAwareLogger{Interface: logger.Default.LogMode(logger.Silent)},
	}
	if logLevel == slog.LevelDebug {
		gormConfig.Logger = lockAwareLogger{Interface: logger.Default.LogMode(logger.Info)}
	}

	var db *gorm.DB
	var isSQLite bool
	var err error

	if driver == "postgres" && dsn != "" {
		db, err = gorm.Open(postgres.Open(dsn), gormConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
		}
		sqlDB, err := db.DB()
		if err != nil {
			return nil, fmt.Errorf("get underlying DB: %w", err)
		}
		sqlDB.SetMaxOpenConns(dbMaxOpenConns)
		sqlDB.SetMaxIdleConns(dbMaxIdleConns)
		sqlDB.SetConnMaxLifetime(dbConnMaxLifetime)
		slog.Info("Database initialized (PostgreSQL)", "dsn", redactDSN(dsn))
	} else {
		if err := os.MkdirAll(filepath.Dir(fallbackPath), 0700); err != nil {
			return nil, err
		}
		// Enable foreign_keys and busy_timeout at the DSN level so every
		// pooled connection inherits the pragmas (a per-connection PRAGMA
		// only affects the single connection it runs on). Applies to both
		// file and in-memory DSNs; DSNs that already carry query
		// parameters are left untouched.
		sqliteDSN := fallbackPath
		if !strings.Contains(sqliteDSN, "?") {
			sqliteDSN += "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
		}
		db, err = gorm.Open(glebarez.Open(sqliteDSN), gormConfig)
		if err != nil {
			return nil, err
		}
		isSQLite = true
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.SetMaxOpenConns(10)
			sqlDB.SetMaxIdleConns(5)
			sqlDB.SetConnMaxLifetime(5 * time.Minute)
			sqlDB.SetConnMaxIdleTime(2 * time.Minute)
		} else {
			slog.Warn("Failed to configure DB connection pool", "err", err)
		}
		slog.Info("Database initialized (SQLite)", "path", fallbackPath)
	}

	// Apply gormigrate schema migrations FIRST. They must run before AutoMigrate
	// so table renames (e.g. agents -> implants) operate on the pre-existing
	// schema; running AutoMigrate first would create the new table and silently
	// strand the old table's data.
	if err := runSchemaMigrations(db); err != nil {
		return nil, fmt.Errorf("schema migration failed: %w", err)
	}

	// Auto-migrate all models (creates any tables/columns still missing on
	// fresh installs and after historical migrations)
	if err := db.AutoMigrate(&Implant{}, &Task{}, &AuditLog{}, &Listener{}, &TokenEntry{}, &SocksSession{}, &CredentialEntry{}, &User{}, &BuildLog{}, &ScanResult{}, &NetworkHost{}, &CommandTemplate{}, &BOFFile{}, &BOFLibrary{}, &ServerConfig{}, &WebhookConfig{}, &Plugin{}, &PluginReview{}, &PluginDependency{}, &PluginUpdateStatus{}, &RolePermission{}, &AutomationRule{}, &AlertRule{}, &Alert{}, &SystemMetric{}, &GeneratedReport{}, &Campaign{}, &CampaignAgent{}, &MeshPeer{}, &BloodHoundResult{}, &BloodHoundFile{}, &OpsecHistory{}, &OpsecRule{}, &CircuitBreakerConfig{}, &CircuitBreakerEvent{}, &CustomRole{}, &PhishingTemplate{}, &PhishingCampaign{}, &PhishingEvent{}, &AgentTag{}, &AgentTagAssignment{}, &AutoTagRule{}, &ScheduledTask{}, &Notification{}, &AgentGroup{}, &AgentGroupAssignment{}, &Workflow{}, &WorkflowStep{}, &WorkflowExecution{}, &WorkflowStepLog{}, &ChatMessage{}, &StagerToken{}, &Redirector{}, &AgentLock{}, &CloudCred{}, &AIChatSession{}, &AIChatMessage{}, &ExtC2Channel{}, &AgentStatusEvent{}, &BackupCode{}, &UserSession{}, &PasswordHistory{}, &ApiKey{}, &Script{}, &RegSecret{}, &KillSwitch{}); err != nil {
		return nil, err
	}

	// Apply index migrations LAST, once every table/column they target is
	// guaranteed to exist on all upgrade paths.
	if err := runIndexMigrations(db); err != nil {
		return nil, fmt.Errorf("index migration failed: %w", err)
	}

	// Seed role permissions
	seedRolePermissions(db)

	// Seed default admin user if none exist
	var userCount int64
	if err := db.Model(&User{}).Count(&userCount).Error; err != nil {
		return nil, fmt.Errorf("failed to check for existing users: %w", err)
	}
	if userCount == 0 {
		adminPass := ""
		if len(defaultPassword) > 0 && defaultPassword[0] != "" {
			adminPass = defaultPassword[0]
		} else {
			var genErr error
			adminPass, genErr = generateRandomPassword(12)
			if genErr != nil {
				slog.Error("Failed to generate random admin password", "err", genErr)
				return nil, genErr
			}
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
			redacted := redactString(adminPass)
			slog.Warn("╔══════════════════════════════════════════════════════════╗")
			slog.Warn("║  DEFAULT ADMIN CREDENTIALS (CHANGE IMMEDIATELY!)       ║")
			slog.Warn(fmt.Sprintf("║  Username: admin  Password: %-24s  ║", redacted))
			slog.Warn("║  Check config.yaml for the actual password.            ║")
			slog.Warn("╚══════════════════════════════════════════════════════════╝")
		}
	}

	if isSQLite {
		db.Exec("PRAGMA foreign_keys = ON;")
		execMigration(db, "PRAGMA journal_mode = WAL;", "pragma_wal")
		execMigration(db, "PRAGMA journal_size_limit = 67108864;", "pragma_journal_limit")
		execMigration(db, "PRAGMA cache_size = -2000;", "pragma_cache")
		execMigration(db, "PRAGMA temp_store = MEMORY;", "pragma_temp_store")
		execMigration(db, "PRAGMA synchronous = NORMAL;", "pragma_sync")
		execMigration(db, "PRAGMA busy_timeout = 5000;", "pragma_busy_timeout")
		execMigration(db, "PRAGMA mmap_size = 268435456;", "pragma_mmap")
		execMigration(db, "PRAGMA auto_vacuum = INCREMENTAL;", "pragma_auto_vacuum")
	}

	return db, nil
}

func seedRolePermissions(db *gorm.DB) {
	var count int64
	if err := db.Model(&RolePermission{}).Count(&count).Error; err != nil {
		return
	}
	if count > 0 {
		return
	}

	for role, perms := range RolePermissionsMap {
		for _, perm := range perms {
			if err := db.Create(&RolePermission{
				Role:       role,
				Permission: perm,
			}).Error; err != nil {
				slog.Error("Failed to seed role permission", "role", role, "permission", perm, "error", err)
			}
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

func redactString(s string) string {
	if len(s) <= 4 {
		return strings.Repeat("*", len(s))
	}
	return s[:2] + strings.Repeat("*", len(s)-4) + s[len(s)-2:]
}

func redactDSN(dsn string) string {
	parts := strings.Split(dsn, " ")
	out := make([]string, len(parts))
	for i, p := range parts {
		lower := strings.ToLower(p)
		if strings.HasPrefix(lower, "password=") || strings.HasPrefix(lower, "pwd=") {
			out[i] = p[:strings.Index(p, "=")+1] + "****"
		} else {
			out[i] = p
		}
	}
	return strings.Join(out, " ")
}
