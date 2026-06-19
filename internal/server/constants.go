package server

import "time"

const (
	BeaconRateLimit      = 100
	BeaconRateWindow     = 1 * time.Minute
	LoginRateLimit       = 5
	LoginRateWindow      = 1 * time.Minute
	RateLimiterCleanup   = 5 * time.Minute
	DefaultPageSize      = 20
	MaxPageSize          = 100
	DefaultTaskPageSize  = 50
	MaxTaskPageSize      = 200
	AgentDetailTaskLimit = 50
	AgentTasksLimit      = 20
	DashboardRecentTasks = 5
	BeaconTaskFetchLimit = 10

	MaxUploadSize = 50 * 1024 * 1024 // 50 MB max for file transfers
	MaxResultSize = 1 * 1024 * 1024  // 1 MB max per task result to prevent DB bloat

	// SOCKS Relay
	SocksMaxFrameSize   = 64 * 1024        // 64 KB per relay frame
	SocksFastInterval   = 500              // ms – agent fast-poll when relay active
	SocksCleanupTimeout = 5 * time.Minute  // clean dead connections after 5 min
	SocksMaxConns       = 256              // max concurrent connections per relay session
)
