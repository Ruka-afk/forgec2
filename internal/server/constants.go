package server

import "time"

const (
	ServerVersion = "2.1.0"

	BeaconRateLimit      = 100
	BeaconRateWindow     = 1 * time.Minute

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
	MaxJSONBodySize = 2 * 1024 * 1024 // 2 MB max for JSON/form request bodies
	MaxPendingTasksPerAgent = 50 // max pending tasks per agent before rejecting new ones

	// SOCKS Relay
	SocksMaxFrameSize   = 64 * 1024       // 64 KB per relay frame
	SocksFastInterval   = 500             // ms – agent fast-poll when relay active
	SocksCleanupTimeout = 5 * time.Minute // clean dead connections after 5 min
	SocksMaxConns       = 256             // max concurrent connections per relay session

	// ─── Query Limits ───
	APIAgentListLimit     = 500
	APITaskListLimit      = 200
	APICredentialListLimit = 500
	APIListenerListLimit  = 100
	APIAuditLogListLimit  = 200
	ExportTaskLimit       = 10000
	CSVResultTruncLen     = 500
	CSVErrorTruncLen      = 500
	TopologyAgentLimit    = 5000
	LootAgentLimit        = 5000
	AgentQueryLimit       = 5000
	AutoTagRuleLimit      = 200
	AutoTagAgentLimit     = 5000
	AutoTagAssignmentLimit = 50000
	BloodHoundResultLimit = 50000
	CampaignTaskLimit     = 10000
	MITRETimelineLimit    = 500
	DashboardTrafficLimit = 10000
	ClaudeMaxTokens       = 4096
	MaxBOFResultLimit     = 100
	AutomationRuleLimit   = 200

	// ─── Timeouts ───
	HTTPReadTimeout  = 30 * time.Second
	HTTPWriteTimeout = 60 * time.Second
	HTTPIdleTimeout  = 120 * time.Second

	WSReadDeadline    = 60 * time.Second
	WSPingInterval    = 30 * time.Second
	WSWriteDeadline   = 10 * time.Second
	WSMaxMessageSize  = 512 * 1024 // 512 KB
	BatchFlushDelay   = 1 * time.Second
	BatchFlushThreshold = 16

	TCPReadDeadline       = 60 * time.Second
	ActivityUpdateThrottle = 60 * time.Second

	BeaconPingInterval  = 30 * time.Second
	BeaconWriteDeadline = 10 * time.Second
	BeaconReadDeadline  = 60 * time.Second
	OperatorWriteDeadline = 5 * time.Second

	SOCKSHandshakeTimeout = 30 * time.Second
	SOCKSRelayWriteTimeout = 10 * time.Second

	RemoteDesktopWriteDeadline = 5 * time.Second
	MonitorMetricsInterval     = 30 * time.Second
	MonitorAlertInterval       = 1 * time.Minute
	StaleThreshold             = 30 * time.Minute

	AIAPITimeout       = 120 * time.Second
	AITaskWaitMax      = 60 * time.Second
	AITaskPollMinInterval = 250 * time.Millisecond
	WebhookHTTPTimeout = 10 * time.Second
	AutomationDownloadTimeout = 60 * time.Second
	GeoIPLookupTimeout = 5 * time.Second
	ReachabilityDialTimeout = 5 * time.Second
	DomainHealthCheckTimeout = 8 * time.Second
	WebhookDeliveryTimeout = 15 * time.Second
	UpdateCheckInitialDelay = 10 * time.Second
	UpdateCheckInterval = 1 * time.Hour
	GitHubAPITimeout    = 15 * time.Second
	GracefulShutdownDelay = 500 * time.Millisecond
	UpdateDownloadTimeout = 5 * time.Minute
	ChecksumDownloadTimeout = 30 * time.Second
	CBCheckInterval     = 60 * time.Second
	CBDialTimeout       = 5 * time.Second
	CBHealthCheckTimeout = 5 * time.Second
	ConfigReloadDebounce = 500 * time.Millisecond
	CacheCleanupInterval = 5 * time.Minute
	GRPCMaxRecvMsgSize  = 10 * 1024 * 1024 // 10 MB
	PluginUpdateCheckInterval = 6 * time.Hour
	ACMEProvisionTimeout = 5 * time.Minute
	CertExpiryEstimate   = 80 * 24 * time.Hour
	PeriodicCleanupInterval = 24 * time.Hour
	RPortFwdCleanupInterval = 5 * time.Minute

	// ─── Stale Agent Thresholds ───
	GhostAgentCutoff    = 1 * time.Hour
	PayloadCleanupInterval = 30 * time.Minute
	ListenerActiveThreshold = 5 * time.Minute
	MaxReportDateRange  = 365 * 24 * time.Hour

	// ─── Agent Offline Detection ───
	DefaultOfflineThresholdSec = 300

	// ─── Beacon Processing ───
	CallbackHTTPTimeout    = 30 * time.Second
	AuditLogResultMaxLen  = 300
	AuditLogDetailsMaxLen = 600

	// ─── Auth / Session ───
	MaxLoginDelayIter   = 10
	LoginBruteForceDelay = 500 * time.Millisecond
	DefaultSessionHours = 24
	SecondsPerHour      = 3600
	RememberMeMaxAgeSec = 7 * 86400
	LangCookieMaxAgeSec = 365 * 24 * 3600

	// ─── Validation Bounds ───
	MinOfflineThresholdSec = 5
	MaxOfflineThresholdSec = 3600
	MinSessionMaxAgeHours  = 1
	MaxSessionMaxAgeHours  = 720
	MinCleanupRetentionDays = 1
	MaxCleanupRetentionDays = 365
	MinHTTPStatusCode = 100
	MaxHTTPStatusCode = 599

	// ─── Auth Alert Defaults ───
	DefaultMaxLoginAttempts = 5
	DefaultLoginWindowSec   = 60
	DefaultLockoutTimeSec   = 900

	// ─── AI ───
	AIResponseTruncLen   = 8000
	AIThinkingPreviewLen = 300
	AIErrorBodyTruncLen  = 500
	AIToolResultTruncLen = 500
	AITaskResultTruncLen = 2000
	AIStreamBufSize      = 4096

	// ─── DNS ───
	DNSTXTChunkSize = 255

	// ─── Misc ───
	MaxSleepSeconds   = 86400
	MaxJitterPercent  = 100
	DefaultRedirectorPort = 443
	DefaultSSHPort    = 22
	CertFileName      = "fullchain.pem"
	KeyFileName       = "privkey.pem"
	AnthropicAPIVersion = "2023-06-01"
)
