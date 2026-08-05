package server

import "time"

const (
	ServerVersion = "2.5.0"

	BeaconRateLimit  = 100
	BeaconRateWindow = 1 * time.Minute

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

	MaxUploadSize           = 50 * 1024 * 1024 // 50 MB max for file transfers
	MaxResultSize           = 1 * 1024 * 1024  // 1 MB max per task result to prevent DB bloat
	MaxJSONBodySize         = 2 * 1024 * 1024  // 2 MB max for JSON/form request bodies
	MaxPendingTasksPerAgent = 50               // max pending tasks per agent before rejecting new ones
	MaxCommandLength        = 10000            // max characters in a command string
	MaxNotesLength          = 5000             // max characters in agent notes/tags
	MaxChatMessageBytes     = 8 * 1024         // max bytes in a chat message
	MaxShellcodeSize        = 10 * 1024 * 1024 // 10 MB max decoded shellcode payload
	MaxTechniqueLength      = 64               // max characters in an injection/spawn technique name
	MaxTargetLength         = 128              // max characters in a spawn target executable

	// ─── Map Size Limits ───
	MaxWSConnections     = 256              // max concurrent WebSocket dashboard clients
	MaxExtC2QueuePerChan = 200              // max queued tasks per ext C2 channel before dropping
	MaxBuildJobs         = 50               // max concurrent build jobs
	MaxScreenMonitors    = 100              // max screen monitor registrations
	MaxRPortFwdListeners = 100              // max reverse port forward listeners
	MaxExtraListeners    = 64               // max dynamically created listeners
	MaxDomainFrontStatus = 128              // max domain fronting entries
	StaleMapCleanupAge   = 30 * time.Minute // clean map entries older than this

	// SOCKS Relay
	SocksMaxFrameSize   = 64 * 1024       // 64 KB per relay frame
	SocksFastInterval   = 500             // ms – agent fast-poll when relay active
	SocksCleanupTimeout = 5 * time.Minute // clean dead connections after 5 min
	SocksMaxConns       = 256             // max concurrent connections per relay session

	// ─── Query Limits ───
	APIAgentListLimit      = 500
	APITaskListLimit       = 200
	APICredentialListLimit = 500
	APIListenerListLimit   = 100
	APIAuditLogListLimit   = 200
	ExportTaskLimit        = 10000
	CSVResultTruncLen      = 500
	CSVErrorTruncLen       = 500
	TopologyAgentLimit     = 5000
	LootAgentLimit         = 5000
	AgentQueryLimit        = 5000
	AutoTagRuleLimit       = 200
	AutoTagAgentLimit      = 5000
	AutoTagAssignmentLimit = 50000
	BloodHoundResultLimit  = 50000
	CampaignTaskLimit      = 10000
	MITRETimelineLimit     = 500
	DashboardTrafficLimit  = 10000
	ClaudeMaxTokens        = 4096
	MaxBOFResultLimit      = 100
	AutomationRuleLimit    = 200

	// ─── Timeouts ───
	HTTPReadTimeout  = 30 * time.Second
	HTTPWriteTimeout = 60 * time.Second
	HTTPIdleTimeout  = 120 * time.Second

	WSReadDeadline      = 60 * time.Second
	WSPingInterval      = 30 * time.Second
	WSWriteDeadline     = 10 * time.Second
	WSMaxMessageSize    = 512 * 1024 // 512 KB
	WSReadBufSize       = 16 * 1024  // 16 KB
	WSWriteBufSize      = 16 * 1024  // 16 KB
	BatchFlushDelay     = 1 * time.Second
	BatchFlushThreshold = 16

	TCPReadDeadline        = 60 * time.Second
	TCPWriteDeadline       = 10 * time.Second
	ActivityUpdateThrottle = 60 * time.Second

	BeaconPingInterval    = 30 * time.Second
	BeaconWriteDeadline   = 10 * time.Second
	BeaconReadDeadline    = 60 * time.Second
	OperatorWriteDeadline = 5 * time.Second

	SOCKSHandshakeTimeout  = 30 * time.Second
	SOCKSRelayWriteTimeout = 10 * time.Second

	RemoteDesktopWriteDeadline = 5 * time.Second
	RemoteDesktopReadDeadline  = 5 * time.Minute
	MonitorMetricsInterval     = 30 * time.Second
	MonitorAlertInterval       = 1 * time.Minute

	AIAPITimeout              = 120 * time.Second
	AITaskWaitMax             = 60 * time.Second
	AITaskPollMinInterval     = 250 * time.Millisecond
	WebhookHTTPTimeout        = 10 * time.Second
	AutomationDownloadTimeout = 60 * time.Second
	GeoIPLookupTimeout        = 5 * time.Second
	ReachabilityDialTimeout   = 5 * time.Second
	DomainHealthCheckTimeout  = 8 * time.Second
	WebhookDeliveryTimeout    = 15 * time.Second
	UpdateCheckInitialDelay   = 10 * time.Second
	UpdateCheckInterval       = 1 * time.Hour
	GitHubAPITimeout          = 15 * time.Second
	GracefulShutdownDelay     = 500 * time.Millisecond
	UpdateDownloadTimeout     = 5 * time.Minute
	ChecksumDownloadTimeout   = 30 * time.Second
	CBCheckInterval           = 60 * time.Second
	CBDialTimeout             = 5 * time.Second
	CBHealthCheckTimeout      = 5 * time.Second
	ConfigReloadDebounce      = 500 * time.Millisecond
	CacheCleanupInterval      = 5 * time.Minute
	GRPCMaxRecvMsgSize        = 10 * 1024 * 1024 // 10 MB
	PluginUpdateCheckInterval = 6 * time.Hour
	ACMEProvisionTimeout      = 5 * time.Minute
	CertExpiryEstimate        = 80 * 24 * time.Hour
	PeriodicCleanupInterval   = 24 * time.Hour
	RPortFwdCleanupInterval   = 5 * time.Minute

	// ─── Stale Agent Thresholds ───
	GhostAgentCutoff        = 1 * time.Hour
	PayloadCleanupInterval  = 30 * time.Minute
	ListenerActiveThreshold = 5 * time.Minute
	MaxReportDateRange      = 365 * 24 * time.Hour

	// ─── Beacon Processing ───
	CallbackHTTPTimeout      = 30 * time.Second
	StaleTaskRequeueInterval = 5 * time.Minute
	StaleRunningTaskTimeout  = 10 * time.Minute
	// BeaconMaxBodySize caps unauthenticated beacon payloads. Beacons are
	// ECDH-encrypted (base64 ~1.33x expansion) and may carry file chunks, so the
	// cap is larger than MaxJSONBodySize while still bounding memory exhaustion.
	BeaconMaxBodySize = 64 * 1024 * 1024
	// AckedTaskResultTimeout bounds tasks the agent acknowledged but never
	// returned a result for. Acknowledged tasks are never dispatched twice (they
	// may still be executing), so after this window they are marked failed
	// instead of being left "running" forever.
	AckedTaskResultTimeout = 30 * time.Minute
	AuditLogResultMaxLen   = 300
	AuditLogDetailsMaxLen  = 600

	// ─── Auth / Session ───
	DefaultSessionHours = 24
	SecondsPerHour      = 3600
	RememberMeMaxAgeSec = 7 * 86400
	LangCookieMaxAgeSec = 365 * 24 * 3600

	// ─── Validation Bounds ───
	MinOfflineThresholdSec  = 5
	MaxOfflineThresholdSec  = 3600
	MinSessionMaxAgeHours   = 1
	MaxSessionMaxAgeHours   = 720
	MinCleanupRetentionDays = 1
	MaxCleanupRetentionDays = 365
	MinHTTPStatusCode       = 100
	MaxHTTPStatusCode       = 599

	// ─── Auth Alert Defaults ───
	DefaultMaxLoginAttempts = 5
	DefaultLoginWindowSec   = 60
	DefaultLockoutTimeSec   = 900

	// ─── AI ───
	AIResponseTruncLen   = 8000
	AIThinkingPreviewLen = 300
	AIErrorBodyTruncLen  = 500
	aiRetryMax           = 2
	AIToolResultTruncLen = 500
	AITaskResultTruncLen = 2000
	AIStreamBufSize      = 4096

	// ─── DNS ───
	DNSTXTChunkSize = 255

	// ─── Misc ───
	MaxSleepSeconds       = 86400
	MaxJitterPercent      = 100
	DefaultRedirectorPort = 443
	DefaultSSHPort        = 22
	CertFileName          = "fullchain.pem"
	KeyFileName           = "privkey.pem"
	AnthropicAPIVersion   = "2023-06-01"

	// ─── Beacon Dedup ───
	MaxBeaconDedupEntries = 10000
	BeaconDedupWindow     = 5 * time.Second  // window for duplicate detection
	BeaconDedupStaleAge   = 30 * time.Second // entries older than this are purged
	BeaconDedupCleanup    = 60 * time.Second // cleanup ticker interval

	// ─── Beacon Protocol v2 ───
	// BeaconSessionRekeyMessages is the session message count after which the
	// agent is asked to rotate keying material with a fresh handshake.
	BeaconSessionRekeyMessages = 10000

	// ─── Batch / Request Limits ───
	MaxBatchAgentLimit   = 500
	MaxTaskIDsPerRequest = 200
	MaxBulkCancelLimit   = 100
	PasswordHistoryMax   = 10
	DefaultBOFLimit      = 20
	TopoRecencyMinutes   = 30
	TopoAgentLimit       = 30

	// ─── Plugin / Callback Timeouts ───
	PluginHookTimeout = 30 * time.Second

	// ─── Server Lifecycle ───
	GracefulShutdownTimeout = 15 * time.Second
	InFlightDrainTimeout    = 5 * time.Second
	HTTPClientShortTimeout  = 30 * time.Second
	HTTPClientLongTimeout   = 5 * time.Minute
	HTTPMaxIdleConns        = 20
	HTTPMaxIdleConnsPerHost = 5
	HTTPIdleConnTimeout     = 90 * time.Second
	TCPMaxMessageSize       = 16 * 1024 * 1024 // 16 MB
	GeoIPSemaphoreSize      = 10
	TaskWorkerPoolSize      = 32

	// ─── Agent Offline / Stale ───
	DefaultOfflineThresholdSec  = 60
	StaleThresholdMultiplier    = 3
	DefaultCleanupRetentionDays = 30

	// ─── Backup ───
	BackupRetainCount = 7

	// ─── Auth / Security ───
	BcryptCost             = 12
	AuthUserCacheTTL       = 5 * time.Minute
	PasswordChangeCooldown = 5 * time.Minute

	// ─── Activity Middleware ───
	ActivityCleanupInterval = 10 * time.Minute

	// ─── Audit / SIEM ───
	AuditAlertCheckInterval = 10 * time.Minute
	LockoutEntryMaxEntries  = 10000
	LockoutCleanupInterval  = 10 * time.Minute
	LockoutStaleGrace       = 5 * time.Minute
	SIEMBatchSize           = 100
	SIEMBatchInterval       = 10 * time.Second
	SIEMHTTPTimeout         = 10 * time.Second
)
