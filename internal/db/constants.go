package db

import "time"

const (
	// SQLiteBusyTimeoutMS is the busy_timeout pragma applied to the SQLite DSN.
	SQLiteBusyTimeoutMS = 5000

	// SQLite pool sizing (SQLite allows a single writer; keep concurrency low).
	SQLiteMaxOpenConns    = 10
	SQLiteMaxIdleConns    = 5
	SQLiteConnMaxLifetime = 5 * time.Minute
	SQLiteConnMaxIdleTime = 2 * time.Minute
)
