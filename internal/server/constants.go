package server

import "time"

const (
	OfflineThreshold      = 60 * time.Second
	BeaconRateLimit       = 100
	BeaconRateWindow      = 1 * time.Minute
	LoginRateLimit        = 5
	LoginRateWindow       = 1 * time.Minute
	RateLimiterCleanup    = 5 * time.Minute
	DefaultPageSize       = 20
	MaxPageSize           = 100
	DefaultTaskPageSize   = 50
	MaxTaskPageSize       = 200
	AgentDetailTaskLimit  = 50
	AgentTasksLimit       = 20
	DashboardRecentTasks  = 5
	BeaconTaskFetchLimit  = 5

	MaxUploadSize = 50 * 1024 * 1024 // 50 MB max for file transfers
)
