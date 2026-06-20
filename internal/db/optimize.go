package db

import (
	"time"

	"gorm.io/gorm"
)

// DatabaseOptimizer provides database performance optimization utilities
type DatabaseOptimizer struct {
	db *gorm.DB
}

// NewDatabaseOptimizer creates a new optimizer instance
func NewDatabaseOptimizer(db *gorm.DB) *DatabaseOptimizer {
	return &DatabaseOptimizer{db: db}
}

// CreatePerformanceIndexes creates additional indexes for query optimization
func (opt *DatabaseOptimizer) CreatePerformanceIndexes() {
	// Agent indexes
	opt.db.Exec("CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status)")
	opt.db.Exec("CREATE INDEX IF NOT EXISTS idx_agents_listener_id ON agents(listener_id)")
	opt.db.Exec("CREATE INDEX IF NOT EXISTS idx_agents_hostname ON agents(hostname)")
	opt.db.Exec("CREATE INDEX IF NOT EXISTS idx_agents_ip ON agents(ip)")

	// Task indexes
	opt.db.Exec("CREATE INDEX IF NOT EXISTS idx_tasks_type ON tasks(type)")
	opt.db.Exec("CREATE INDEX IF NOT EXISTS idx_tasks_agent_created ON tasks(agent_id, created_at DESC)")
	opt.db.Exec("CREATE INDEX IF NOT EXISTS idx_tasks_status_created ON tasks(status, created_at)")

	// Credential indexes
	opt.db.Exec("CREATE INDEX IF NOT EXISTS idx_credentials_agent_id ON credentials(agent_id)")
	opt.db.Exec("CREATE INDEX IF NOT EXISTS idx_credentials_source ON credentials(source)")
	opt.db.Exec("CREATE INDEX IF NOT EXISTS idx_credentials_created ON credentials(created_at)")

	// Audit log indexes
	opt.db.Exec("CREATE INDEX IF NOT EXISTS idx_audit_user ON audit_logs(user)")
	opt.db.Exec("CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_logs(action)")
	opt.db.Exec("CREATE INDEX IF NOT EXISTS idx_audit_created ON audit_logs(created_at)")

	// Scan result indexes
	opt.db.Exec("CREATE INDEX IF NOT EXISTS idx_scan_agent_id ON scan_results(agent_id)")
	opt.db.Exec("CREATE INDEX IF NOT EXISTS idx_scan_type ON scan_results(scan_type)")
	opt.db.Exec("CREATE INDEX IF NOT EXISTS idx_scan_created ON scan_results(created_at)")

	// Chat message indexes
	opt.db.Exec("CREATE INDEX IF NOT EXISTS idx_chat_channel ON chat_messages(channel)")
	opt.db.Exec("CREATE INDEX IF NOT EXISTS idx_chat_user ON chat_messages(user)")
	opt.db.Exec("CREATE INDEX IF NOT EXISTS idx_chat_created ON chat_messages(created_at DESC)")
}

// OptimizeSQLite applies SQLite-specific optimizations
func (opt *DatabaseOptimizer) OptimizeSQLite() {
	// Enable WAL mode for better concurrent access
	opt.db.Exec("PRAGMA journal_mode = WAL;")

	// Increase cache size (2000 pages = ~8MB with 4KB pages)
	opt.db.Exec("PRAGMA cache_size = -2000;")

	// Use memory for temp storage
	opt.db.Exec("PRAGMA temp_store = MEMORY;")

	// Synchronous mode (NORMAL for balance between safety and performance)
	opt.db.Exec("PRAGMA synchronous = NORMAL;")

	// Memory mapping for faster reads
	opt.db.Exec("PRAGMA mmap_size = 268435456;") // 256MB

	// Optimize page size
	opt.db.Exec("PRAGMA page_size = 4096;")
}

// GetDatabaseStats returns database statistics
func (opt *DatabaseOptimizer) GetDatabaseStats() map[string]interface{} {
	stats := make(map[string]interface{})

	// Connection pool stats
	sqlDB, err := opt.db.DB()
	if err == nil {
		poolStats := sqlDB.Stats()
		stats["max_open_connections"] = poolStats.MaxOpenConnections
		stats["open_connections"] = poolStats.OpenConnections
		stats["in_use"] = poolStats.InUse
		stats["idle"] = poolStats.Idle
		stats["wait_count"] = poolStats.WaitCount
		stats["wait_duration_ms"] = poolStats.WaitDuration.Milliseconds()
		stats["max_idle_closed"] = poolStats.MaxIdleClosed
		stats["max_lifetime_closed"] = poolStats.MaxLifetimeClosed
	}

	// Table counts
	var agentCount, taskCount, credCount, auditCount int64
	opt.db.Model(&Agent{}).Count(&agentCount)
	opt.db.Model(&Task{}).Count(&taskCount)
	opt.db.Model(&CredentialEntry{}).Count(&credCount)
	opt.db.Model(&AuditLog{}).Count(&auditCount)

	stats["agent_count"] = agentCount
	stats["task_count"] = taskCount
	stats["credential_count"] = credCount
	stats["audit_count"] = auditCount

	// Database file size (for SQLite)
	// This would require filesystem access

	return stats
}

// VacuumDatabase performs database maintenance
func (opt *DatabaseOptimizer) VacuumDatabase() error {
	return opt.db.Exec("VACUUM;").Error
}

// AnalyzeDatabase updates query planner statistics
func (opt *DatabaseOptimizer) AnalyzeDatabase() error {
	return opt.db.Exec("ANALYZE;").Error
}

// QueryCache provides caching for frequently used queries
type QueryCache struct {
	cache    map[string]CacheEntry
	duration time.Duration
}

// CacheEntry represents a cached query result
type CacheEntry struct {
	Data      interface{}
	ExpiresAt time.Time
}

// NewQueryCache creates a new query cache
func NewQueryCache(duration time.Duration) *QueryCache {
	return &QueryCache{
		cache:    make(map[string]CacheEntry),
		duration: duration,
	}
}

// Get retrieves a cached value
func (qc *QueryCache) Get(key string) (interface{}, bool) {
	entry, exists := qc.cache[key]
	if !exists {
		return nil, false
	}

	if time.Now().After(entry.ExpiresAt) {
		delete(qc.cache, key)
		return nil, false
	}

	return entry.Data, true
}

// Set stores a value in cache
func (qc *QueryCache) Set(key string, data interface{}) {
	qc.cache[key] = CacheEntry{
		Data:      data,
		ExpiresAt: time.Now().Add(qc.duration),
	}
}

// Invalidate removes a cache entry
func (qc *QueryCache) Invalidate(key string) {
	delete(qc.cache, key)
}

// Clear removes all cache entries
func (qc *QueryCache) Clear() {
	qc.cache = make(map[string]CacheEntry)
}

// BatchOperations provides batch database operations
type BatchOperations struct {
	db        *gorm.DB
	batchSize int
}

// NewBatchOperations creates a new batch operations instance
func NewBatchOperations(db *gorm.DB, batchSize int) *BatchOperations {
	if batchSize <= 0 {
		batchSize = 100
	}
	return &BatchOperations{
		db:        db,
		batchSize: batchSize,
	}
}

// BatchCreateAgents creates agents in batches
func (bo *BatchOperations) BatchCreateAgents(agents []Agent) error {
	return bo.db.CreateInBatches(agents, bo.batchSize).Error
}

// BatchCreateTasks creates tasks in batches
func (bo *BatchOperations) BatchCreateTasks(tasks []Task) error {
	return bo.db.CreateInBatches(tasks, bo.batchSize).Error
}

// BatchUpdateAgentStatus updates multiple agent statuses
func (bo *BatchOperations) BatchUpdateAgentStatus(agentIDs []string, status string) error {
	return bo.db.Model(&Agent{}).
		Where("id IN ?", agentIDs).
		Update("status", status).Error
}

// BatchDeleteTasks deletes old tasks in batches
func (bo *BatchOperations) BatchDeleteTasks(before time.Time) (int64, error) {
	result := bo.db.Where("created_at < ?", before).Delete(&Task{})
	return result.RowsAffected, result.Error
}

// BatchDeleteAuditLogs deletes old audit logs in batches
func (bo *BatchOperations) BatchDeleteAuditLogs(before time.Time) (int64, error) {
	result := bo.db.Where("created_at < ?", before).Delete(&AuditLog{})
	return result.RowsAffected, result.Error
}

// QueryBuilder provides fluent query building
type QueryBuilder struct {
	db    *gorm.DB
	query *gorm.DB
}

// NewQueryBuilder creates a new query builder
func NewQueryBuilder(db *gorm.DB) *QueryBuilder {
	return &QueryBuilder{
		db:    db,
		query: db.Session(&gorm.Session{}),
	}
}

// FilterAgentsByStatus filters agents by status
func (qb *QueryBuilder) FilterAgentsByStatus(status string) *QueryBuilder {
	qb.query = qb.query.Where("status = ?", status)
	return qb
}

// FilterAgentsByListener filters agents by listener ID
func (qb *QueryBuilder) FilterAgentsByListener(listenerID uint) *QueryBuilder {
	qb.query = qb.query.Where("listener_id = ?", listenerID)
	return qb
}

// FilterAgentsByHostname filters agents by hostname (partial match)
func (qb *QueryBuilder) FilterAgentsByHostname(hostname string) *QueryBuilder {
	qb.query = qb.query.Where("hostname LIKE ?", "%"+hostname+"%")
	return qb
}

// FilterAgentsByIP filters agents by IP (partial match)
func (qb *QueryBuilder) FilterAgentsByIP(ip string) *QueryBuilder {
	qb.query = qb.query.Where("ip LIKE ?", "%"+ip+"%")
	return qb
}

// SortBy sorts by field
func (qb *QueryBuilder) SortBy(field string, ascending bool) *QueryBuilder {
	order := field
	if !ascending {
		order += " DESC"
	}
	qb.query = qb.query.Order(order)
	return qb
}

// Limit sets the limit
func (qb *QueryBuilder) Limit(limit int) *QueryBuilder {
	qb.query = qb.query.Limit(limit)
	return qb
}

// Offset sets the offset
func (qb *QueryBuilder) Offset(offset int) *QueryBuilder {
	qb.query = qb.query.Offset(offset)
	return qb
}

// Execute runs the query and returns agents
func (qb *QueryBuilder) Execute() ([]Agent, error) {
	var agents []Agent
	err := qb.query.Find(&agents).Error
	return agents, err
}

// Count returns the count
func (qb *QueryBuilder) Count() (int64, error) {
	var count int64
	err := qb.query.Model(&Agent{}).Count(&count).Error
	return count, err
}

// DatabaseHealthCheck performs a health check on the database
type DatabaseHealthCheck struct {
	db *gorm.DB
}

// NewDatabaseHealthCheck creates a new health check instance
func NewDatabaseHealthCheck(db *gorm.DB) *DatabaseHealthCheck {
	return &DatabaseHealthCheck{db: db}
}

// Check performs the health check
func (hc *DatabaseHealthCheck) Check() map[string]interface{} {
	result := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"checks":    make(map[string]bool),
	}

	// Check 1: Database connectivity
	err := hc.db.Exec("SELECT 1").Error
	result["checks"].(map[string]bool)["connectivity"] = err == nil

	// Check 2: Connection pool
	sqlDB, err := hc.db.DB()
	if err == nil {
		stats := sqlDB.Stats()
		result["checks"].(map[string]bool)["connection_pool"] = stats.OpenConnections < stats.MaxOpenConnections
		result["connection_stats"] = stats
	}

	// Check 3: Query performance
	start := time.Now()
	hc.db.Model(&Agent{}).Count(nil)
	duration := time.Since(start)
	result["checks"].(map[string]bool)["query_performance"] = duration < 100*time.Millisecond
	result["query_duration_ms"] = duration.Milliseconds()

	// Check 4: Index existence
	var indexCount int64
	hc.db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='index'").Scan(&indexCount)
	result["checks"].(map[string]bool)["indexes_exist"] = indexCount > 0
	result["index_count"] = indexCount

	// Determine overall status
	allPassed := true
	for _, passed := range result["checks"].(map[string]bool) {
		if !passed {
			allPassed = false
			break
		}
	}

	if !allPassed {
		result["status"] = "degraded"
	}

	return result
}
