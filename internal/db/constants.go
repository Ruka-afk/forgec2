package db

import "time"

const (
	// SQLiteBusyTimeoutMS is the busy_timeout pragma applied to the SQLite DSN.
	SQLiteBusyTimeoutMS = 5000

	// SQLite pool sizing (SQLite allows a single writer; keep concurrency low).
	// A single open connection serializes writers inside the driver and avoids
	// SQLITE_BUSY churn under concurrent beacon check-ins. Postgres uses a
	// larger pool (see config_sync.go); SQLite must stay at 1.
	SQLiteMaxOpenConns    = 1
	SQLiteMaxIdleConns    = 1
	SQLiteConnMaxLifetime = 5 * time.Minute
	SQLiteConnMaxIdleTime = 2 * time.Minute
)
