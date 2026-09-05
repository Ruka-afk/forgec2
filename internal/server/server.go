package server

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/payload"
	"github.com/forgec2/forgec2/internal/plugin"
	"github.com/forgec2/forgec2/internal/server/middleware"
	"github.com/forgec2/forgec2/internal/server/opsec"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
	"gorm.io/gorm"
)

// ── Server core: struct, constructor, events, config reload, routes ────────
// Transports live in server_ws.go / server_net.go / server_listeners.go,
// lifecycle in server_lifecycle.go, retention in server_cleanup.go,
// HTTP glue in server_http.go.

type Server struct {
	cfg            *config.Config
	configMu       sync.RWMutex
	db             *gorm.DB
	router         *gin.Engine
	wsClients      map[*websocket.Conn]*wsClientConn
	wsMutex        sync.RWMutex
	wsUpgrader     websocket.Upgrader
	rateLimiter    *middleware.RateLimiter
	apiRateLimiter *middleware.APIRateLimiter
	loginLockout   *loginLockoutTracker
	socksEngine    *socksRelayEngine
	cookieProxy    *cookieProxyEngine
	tunEngine      *tunEngine
	startTime      time.Time

	dnsListener           *DNSBeaconListener
	icmpListener          *ICMPBeaconListener
	grpcListener          *GRPCListener
	sshListener           *SSHBeaconListener
	h2cListener           *H2CBeaconListener
	quicListener          *QUICBeaconListener
	smbLn                 net.Listener
	tcpLn                 net.Listener
	udpConn               net.PacketConn
	screenMonitorImplants map[string]time.Time
	screenMonitorMu       sync.Mutex
	screenFrames          map[string]cachedScreenFrame
	screenFrameMu         sync.RWMutex

	// P0-3: rportfwd (reverse port forward)
	rportfwdListeners map[string]*rportfwdRelay
	rportfwdMu        sync.Mutex

	// lportfwd: agent-local listeners tunneled through the beacon; the
	// teamserver dials the final target on the agent's behalf. Keyed by the
	// frame ConnID chosen by the agent.
	lportfwdTargets map[string]*lportfwdTarget // key: agentID|connID (cross-agent hijack fix)
	lportfwdMu      sync.Mutex
	// lportfwdDeclared tracks operator-declared targets per agent
	// (from lportfwd_start task commands). lportfwd_connect frames must
	// match one — otherwise a compromised implant gets an arbitrary-dial
	// SSRF primitive into the teamserver's network.
	lportfwdDeclared map[string]map[string]bool

	trafficLog   *trafficRing
	trafficBytes *trafficByteAccumulator
	updateState  updateCheckState

	// Server-side traffic auto-adapt loop: last set_sleep trigger per agent.
	// RWMutex because the per-beacon hot path only reads the timestamp.
	autoAdaptMu   sync.RWMutex
	autoAdaptLast map[string]time.Time

	// Domain fronting
	domainFrontDomains []string
	domainFrontMu      sync.Mutex
	domainFrontAuto    bool
	domainFrontStatus  map[string]*frontDomainState

	// WebSocket hub
	wsHub        *WebSocketHub
	wsHubOnce    sync.Once
	shutdownOnce sync.Once

	// Operator WebSocket sessions (real-time updates)
	operatorSessions *operatorSessionTracker

	// Event system
	eventManager *EventManager

	// AI runs are detached from individual HTTP requests. The broker only keeps
	// live subscribers and cancellation handles; replayable events live in DB.
	aiRuns        *aiRunBroker
	aiRunCreateMu sync.Mutex

	// Per-(agent,domain) consecutive-failure fuse for credential checks
	credCheckFuse *credCheckFuseTracker

	// Beacon cipher removed, use ECDH session encryption (cfg.Crypto.Key = "ecdh:")

	// ECDH session manager (nil = disabled / old XOR mode)
	sessionManager *crypto.SessionManager

	// v3 per-implant registration secret store (seals/unseals reg secrets)
	regSecrets *crypto.RegSecretStore

	monitorCollector *MonitorCollector

	// Plugin marketplace
	marketplace *plugin.Marketplace

	// Plugin execution manager
	pluginManager *plugin.Manager

	// Circuit breaker for listener health
	circuitBreaker *CircuitBreaker

	// Optimizations
	configReloader *ConfigReloader
	backupManager  *BackupManager
	configPath     string
	ctx            context.Context
	ctxCancel      context.CancelFunc
	wg             sync.WaitGroup

	// Extra listeners started dynamically via the UI (create listener).
	// Each entry is keyed by "scheme://host:port".
	extraListeners   map[string]io.Closer
	extraListenersMu sync.Mutex

	metrics *MetricsCollector

	// GeoIP lookup concurrency limiter
	geoIPSem chan struct{}

	// Task result background worker pool: limits concurrent goroutines spawned
	// for callbacks, plugin hooks, and token processing to prevent OOM under load.
	taskWorkerSem chan struct{}

	// NTLM relay session tracking
	ntlmRelays *ntlmRelayStore

	// Bulk operation history ring buffer
	bulkHistory   []BulkResult
	bulkHistoryMu sync.Mutex

	// Per-agent task queue depth tracking (agentID → pending task count)
	agentPendingTasks   map[string]int
	agentPendingTasksMu sync.Mutex

	// Beacon deduplication: track recently processed beacon fingerprints
	beaconDedupMu    sync.Mutex
	beaconDedupCache map[string]time.Time

	// Per-agent seq flood lockout (agentID → unlock time). Set by acceptSeq
	// when a frame sequence jump exceeds the hard cap; guards the replay
	// window against a key-holding actor replay-flooding a valid frame.
	seqLockoutMu sync.Mutex
	seqLockout   map[string]time.Time

	// P2P relay depth guard: bounds recursive envelope relay nesting so a
	// maliciously deep parent chain cannot stack-overflow the handler.
	relayDepthMu sync.Mutex
	relayDepth   int

	// Task result idempotency: agentID + result id → processed timestamp.
	// Results re-sent after a dropped frame carry a new envelope seq, so
	// dedupe on the agent-supplied result id instead.
	resultDedupeMu    sync.Mutex
	resultDedupeCache map[string]time.Time

	// Phishing landing page rate limiting keyed by token+IP
	landingLimiterMu    sync.Mutex
	landingLimiterHits  map[string]int
	landingLimiterSince map[string]time.Time

	// External C2 channels (WebSocket relay, Discord, Slack)
	extC2Channels map[string]*extC2WSChannel
	// extC2Runners tracks LIVE poller instances (Discord/Slack) so a deleted
	// channel can actually be stopped — previously the delete handler only
	// removed the metadata entry and the run loop kept reconnecting with the
	// "deleted" bot token until process restart.
	extC2Runners    map[string]extC2Runner
	extC2ChannelsMu sync.Mutex
	extC2TaskQueue  map[string][]extC2Task
	extC2TaskMu     sync.Mutex
	extC2Notify     map[string]chan struct{} // per-channel notification for push-based task delivery

	// Async build job tracking
	buildJobs   map[string]*BuildJob
	buildJobsMu sync.RWMutex

	// Serialized build execution queue: heavy go/garble toolchain invocations
	// are queued through a single worker so concurrent generate requests cannot
	// exhaust CPU/disk. Lazily initialized on first submit.
	buildQueue     chan *queuedBuild
	buildQueueOnce sync.Once

	// buildSem bounds total concurrent toolchain invocations (queue worker AND
	// synchronous stager/one-liner/shellcode builds) so a flood of generate
	// requests cannot exhaust CPU/RAM. Lazily initialized on first submit.
	buildSem chan struct{}

	// transientArtifacts tracks generated files (stager/one-liner builds) that
	// are served once and never tracked as BuildJobs, so they must be reaped
	// separately to avoid unbounded growth of data/agents.
	transientArtifacts   map[string]time.Time
	transientArtifactsMu sync.Mutex

	// Serializes lazy /stage stage-2 payload builds per token so concurrent
	// fetches for the same token do not trigger redundant toolchain builds.
	stageBuildLocks   map[string]*sync.Mutex
	stageBuildLocksMu sync.Mutex

	// Embedded frontend static files (nil = API-only mode)
	staticFS fs.FS

	// Main HTTP server (for graceful shutdown)
	httpServer *http.Server

	// In-flight request tracker for graceful shutdown
	inFlight *middleware.InFlightTracker

	// Flapping suppression: track last status event time per agent
	agentStatusCooldown   map[string]time.Time
	agentStatusCooldownMu sync.Mutex

	// Pending TOTP setup state (between generate and enable), keyed by userID
	pendingTOTP   map[uint]*pendingTOTPState
	pendingTOTPMu sync.Mutex

	// Reusable HTTP clients with connection pooling
	httpClient     *http.Client
	httpClientLong *http.Client

	// Cached automation rules
	automationRules   []AutomationRule
	automationRulesMu sync.RWMutex
	automationRulesAt time.Time

	// Nav stats cache (keyed by tenant id; 0 = legacy/unscoped operators)
	navStatsCache   map[uint]navStatsEntry
	navStatsCacheMu sync.RWMutex

	// SIEM webhook for security event forwarding
	siem *SIEMWebhook

	// TLS fingerprint randomization (JARM/JA3)
	tlsFingerprint *TLSFingerprintManager

	// Password change rate limiter (userID → last change time)
	pwdChangeTimes   map[uint]time.Time
	pwdChangeTimesMu sync.Mutex

	// JARM/JA3 continuous validation
	tlsCertMonitor *TLSCertMonitor

	// OPSEC adaptive threat manager
	opsecAdaptive *opsec.AdaptiveManager

	// Chunked file-transfer HMAC integrity chain state
	fileChains *fileChainState

	// Fleet kill-switch broadcast state (mirrors the KillSwitch DB row; the
	// beacon hot path reads this cache instead of querying on every check-in)
	killSwitchMu    sync.RWMutex
	killSwitchArmed bool
	killSwitchToken string

	// Transport obfuscation (DNS/ICMP)
	transportObfuscation *TransportObfuscationManager
}

// BulkResult tracks a batch command operation.
type BulkResult struct {
	ID        int       `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Command   string    `json:"command"`
	TaskType  string    `json:"task_type"`
	Created   int       `json:"tasks_created"`
	Skipped   int       `json:"skipped_locked"`
	Failed    int       `json:"failed"`
	Operator  string    `json:"operator"`
}

const maxBulkHistory = 50

func New(cfg *config.Config, database *gorm.DB) *Server {
	payload.SetConfiguredGoProxy(cfg.Implant.GoProxy)
	gin.SetMode(gin.ReleaseMode)

	if err := middleware.InitJWTSecret(cfg, ""); err != nil {
		slog.Error("Failed to initialize JWT secret", "err", err)
		os.Exit(1)
	}
	if err := middleware.InitCSRFSecret(cfg); err != nil {
		slog.Error("Failed to initialize CSRF secret", "err", err)
		os.Exit(1)
	}
	crypto.InitLootEncryption(cfg.Crypto.LootKey)
	crypto.InitExtC2Encryption(cfg.Crypto.ExtC2Key)

	inFlight := middleware.NewInFlightTracker()

	r := gin.New()
	r.RedirectTrailingSlash = false
	// Secure default: do NOT trust X-Forwarded-For / X-Real-IP headers unless the
	// operator explicitly configures trusted proxy IPs/CIDRs. Otherwise any client
	// can spoof the header to bypass login lockout, account lockout, and rate limits.
	if len(cfg.Server.TrustedProxies) > 0 {
		if err := r.SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
			slog.Error("Invalid trusted_proxies config, ignoring", "error", err, "proxies", cfg.Server.TrustedProxies)
		}
		middleware.SetTrustedProxyIPs(cfg.Server.TrustedProxies)
	} else {
		r.SetTrustedProxies(nil)
		middleware.SetTrustedProxyIPs(nil)
	}
	r.Use(gin.Recovery())
	r.Use(inFlight.Middleware())
	r.Use(middleware.RequestID())
	r.Use(middleware.SecurityHeaders(cfg.Server.TLSEnabled))
	r.Use(middleware.NoCache())
	r.Use(middleware.ErrorHandler())

	s := &Server{
		cfg:                   cfg,
		db:                    database,
		router:                r,
		wsClients:             make(map[*websocket.Conn]*wsClientConn),
		loginLockout:          newLoginLockoutTracker(),
		socksEngine:           newSocksRelayEngine(),
		cookieProxy:           newCookieProxyEngine(),
		tunEngine:             newTunEngine(),
		startTime:             time.Now(),
		screenMonitorImplants: make(map[string]time.Time),
		screenFrames:          make(map[string]cachedScreenFrame),
		rportfwdListeners:     make(map[string]*rportfwdRelay),
		lportfwdTargets:       make(map[string]*lportfwdTarget),
		lportfwdDeclared:      make(map[string]map[string]bool),
		trafficLog:            newTrafficRing(),
		trafficBytes:          newTrafficByteAccumulator(),
		autoAdaptLast:         make(map[string]time.Time),
		eventManager:          NewEventManager(database),
		operatorSessions:      &operatorSessionTracker{sessions: make(map[uint]*WSOperatorSession)},
		credCheckFuse:         newCredCheckFuseTracker(),
		extraListeners:        make(map[string]io.Closer),
		domainFrontStatus:     make(map[string]*frontDomainState),
		agentPendingTasks:     make(map[string]int),
		beaconDedupCache:      make(map[string]time.Time),
		seqLockout:            make(map[string]time.Time),
		landingLimiterHits:    make(map[string]int),
		landingLimiterSince:   make(map[string]time.Time),
		agentStatusCooldown:   make(map[string]time.Time),
		ntlmRelays:            newNTLMRelayStore(),
		extC2Channels:         make(map[string]*extC2WSChannel),
		extC2Runners:          make(map[string]extC2Runner),
		extC2TaskQueue:        make(map[string][]extC2Task),
		extC2Notify:           make(map[string]chan struct{}),
		buildJobs:             make(map[string]*BuildJob),
		geoIPSem:              make(chan struct{}, GeoIPSemaphoreSize),
		taskWorkerSem:         make(chan struct{}, TaskWorkerPoolSize),
		wsUpgrader: websocket.Upgrader{
			ReadBufferSize:  WSReadBufSize,
			WriteBufferSize: WSWriteBufSize,
			CheckOrigin: func(r *http.Request) bool {
				return allowedOrigin(cfg, r)
			},
		},
		httpClient: &http.Client{
			Timeout: HTTPClientShortTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        HTTPMaxIdleConns,
				MaxIdleConnsPerHost: HTTPMaxIdleConnsPerHost,
				IdleConnTimeout:     HTTPIdleConnTimeout,
				TLSClientConfig: &tls.Config{
					MinVersion: tls.VersionTLS12,
				},
			},
		},
		httpClientLong: &http.Client{
			Timeout: HTTPClientLongTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        HTTPClientMaxIdleConns,
				MaxIdleConnsPerHost: HTTPClientMaxIdlePerHost,
				IdleConnTimeout:     HTTPIdleConnTimeout,
				TLSClientConfig: &tls.Config{
					MinVersion: tls.VersionTLS12,
				},
			},
		},
	}

	s.ctx, s.ctxCancel = context.WithCancel(context.Background())
	s.aiRuns = newAIRunBroker()
	s.initializeAIRuns()

	// Beacon dedup cache cleanup (bounded to prevent memory exhaustion)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(BeaconDedupCleanup)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.beaconDedupMu.Lock()
				now := time.Now()
				for k, t := range s.beaconDedupCache {
					if now.Sub(t) > BeaconDedupStaleAge {
						delete(s.beaconDedupCache, k)
					}
				}
				s.beaconDedupMu.Unlock()

				s.pwdChangeTimesMu.Lock()
				for k, t := range s.pwdChangeTimes {
					if now.Sub(t) > PwdChangeCleanupInterval {
						delete(s.pwdChangeTimes, k)
					}
				}
				s.pwdChangeTimesMu.Unlock()
			case <-s.ctx.Done():
				return
			}
		}
	}()

	s.inFlight = inFlight

	// Initialize rate limiters with context for graceful shutdown
	s.rateLimiter = middleware.NewRateLimiter(s.ctx, cfg.RateLimit.Beacon.Limit, time.Duration(cfg.RateLimit.Beacon.Window)*time.Second)
	s.apiRateLimiter = middleware.NewAPIRateLimiter(s.ctx, cfg.RateLimit.API.Capacity, cfg.RateLimit.API.Rate)

	// Start periodic cleanup for login lockout entries
	s.loginLockout.startCleanup(s.ctx, cfg.RateLimit.Login.Window)

	// Start periodic cleanup for beacon seq-flood lockout entries (S5)
	s.startSeqLockoutCleanup(s.ctx)

	// Beacon payload encryption via ECDH session (cfg.Crypto.Key = "ecdh:")
	if strings.HasPrefix(cfg.Crypto.Key, "ecdh:") {
		sm, err := crypto.NewSessionManager()
		if err != nil {
			slog.Error("Failed to initialize ECDH session manager, falling back to XOR", "err", err)
		} else {
			s.sessionManager = sm
			slog.Info("ECDH session encryption enabled")
		}
	}

	// v3 per-implant registration secret store (server-side sealing key
	// derived from the master beacon key; never embedded in payloads).
	if beaconKeyHex := cfg.Server.BeaconKey; beaconKeyHex != "" {
		if master, err := hex.DecodeString(beaconKeyHex); err == nil && len(master) > 0 {
			s.regSecrets = crypto.NewRegSecretStore(master)
			slog.Info("v3 per-implant registration secrets enabled")
		}
	}

	// Periodic cleanup of stale ECDH sessions to prevent unbounded map growth
	if s.sessionManager != nil {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			ticker := time.NewTicker(5 * time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					s.sessionManager.CleanupExpiredSessions(30 * time.Minute)
				case <-s.ctx.Done():
					return
				}
			}
		}()
	}

	// Periodic cleanup of orphaned v3 registration secrets: a secret is created
	// before the toolchain runs, so a failed build leaves an unbound, unusable
	// row. Sweep unbound secrets older than regSecretOrphanTTL so they don't
	// accumulate indefinitely.
	if s.regSecrets != nil {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			ticker := time.NewTicker(time.Hour)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					s.cleanupOrphanedRegSecrets(regSecretOrphanTTL)
				case <-s.ctx.Done():
					return
				}
			}
		}()
	}

	s.apiRateLimiter.SetWhitelist(cfg.RateLimit.API.Whitelist)

	s.metrics = NewMetricsCollector(s)
	s.metrics.Register(prometheus.DefaultRegisterer)
	r.Use(metricsMiddleware(s.metrics))

	if cfg.SIEM.Enabled && cfg.SIEM.URL != "" {
		s.siem = NewSIEMWebhook(s, cfg.SIEM.URL, cfg.SIEM.Token, cfg.SIEM.Actions)
		s.siem.ReloadRules()
		slog.Info("SIEM webhook enabled", "url", cfg.SIEM.URL)
	}

	// TLS fingerprint randomization (JARM/JA3)
	s.initTLSFingerprint()

	// Password change rate limiter
	s.pwdChangeTimes = make(map[uint]time.Time)

	// TLS certificate stability monitor
	s.tlsCertMonitor = NewTLSCertMonitor(s.cfg.TLSFingerprint.JARMEnabled)

	// OPSEC adaptive threat manager
	s.opsecAdaptive = opsec.NewAdaptiveManager()
	s.opsecAdaptive.StartDecayLoop()

	s.fileChains = newFileChainState()

	// Fleet kill-switch broadcast state
	s.reloadKillSwitchState()

	// Transport obfuscation (DNS/ICMP)
	s.transportObfuscation = NewTransportObfuscationManager()

	// NOTE: setupRoutes() is NOT called here. Call SetupRoutes() after
	// SetStaticFS() to ensure the static file middleware runs before route handlers.

	// Initialize plugin marketplace
	s.marketplace = plugin.NewMarketplace(database)
	if s.cfg.Server.UpdateCheckEnabled {
		// Both the marketplace and the version checker phone home to
		// api.github.com — strictly opt-in for egress hygiene.
		s.marketplace.StartUpdateChecker(PluginUpdateCheckInterval)
	}

	// Initialize plugin execution manager
	s.pluginManager = plugin.NewManager(database)
	s.pluginManager.SetMarketplace(s.marketplace)
	pluginDir := filepath.Join(s.cfg.Server.DataDir, "plugins")
	if err := os.MkdirAll(pluginDir, 0750); err != nil {
		slog.Warn("Failed to create plugin data directory", "dir", pluginDir, "err", err)
	}
	if err := s.pluginManager.LoadFromDisk(pluginDir); err != nil {
		slog.Warn("Failed to load plugins from data directory", "dir", pluginDir, "err", err)
	}
	if err := s.pluginManager.LoadFromDisk("plugins"); err != nil {
		slog.Warn("Failed to load bundled plugins", "dir", "plugins", "err", err)
	}

	// Register event handlers
	s.eventManager.On(EventImplantCheckin, func(evt Event) {
		s.dispatchEvent(evt, true)
	})
	s.eventManager.On(EventImplantDisconnect, func(evt Event) {
		s.dispatchEvent(evt, true)
	})
	s.eventManager.On(EventTaskComplete, func(evt Event) {
		s.dispatchEvent(evt, false)
	})
	s.eventManager.On(EventTaskFail, func(evt Event) {
		// Failures go through the alert/notification path so operators get
		// pushed on task errors, not just webhooks/email.
		s.dispatchEvent(evt, true)
	})
	s.eventManager.On(EventCredentialFound, func(evt Event) {
		s.dispatchEvent(evt, true)
	})
	s.migrateAutomationRules()
	s.registerBuiltinAutomations()

	s.monitorCollector = NewMonitorCollector(s)
	s.monitorCollector.Start()

	s.loadScriptsFromDB()
	s.installScriptingBridge()

	return s
}

// dispatchEvent dispatches webhooks, notifications, alerts, and automation rules for an event.
func (s *Server) dispatchEvent(evt Event, includeAlert bool) {
	s.triggerWebhooks(evt)
	s.triggerEmailNotifications(evt)
	if includeAlert {
		s.TriggerAlertForEvent(evt)
	}
	rules := s.loadAutomationRules()
	for _, rule := range rules {
		if rule.Enabled && rule.EventType == string(evt.Type) {
			s.evaluateRule(evt, rule)
		}
	}
}

func (s *Server) InitOptimizations(configPath string) {
	s.configPath = configPath
	s.configReloader = NewConfigReloader(s.cfg, configPath, func(cfg *config.Config, changed []string) {
		slog.Info("Config reloaded, applying runtime changes", "changed", changed)

		s.configMu.Lock()
		s.cfg.CopyFrom(cfg)
		s.configMu.Unlock()

		for _, field := range changed {
			switch field {
			case "crypto.key", "server.jwt_secret", "crypto.loot_key", "crypto.extc2_key":
				crypto.InitLootEncryption(s.cfg.Crypto.LootKey)
				crypto.InitExtC2Encryption(s.cfg.Crypto.ExtC2Key)
				slog.Info("Crypto primitives updated from reloaded config", "field", field)
			case "crypto.csrf_key":
				if err := middleware.InitCSRFSecret(s.cfg); err != nil {
					slog.Error("Config reload: invalid crypto.csrf_key, keeping previous CSRF key", "err", err)
				}
			}
		}

		// Invalidate automation rule cache so next request fetches fresh rules
		s.automationRulesMu.Lock()
		s.automationRulesAt = time.Time{}
		s.automationRulesMu.Unlock()

		// Invalidate nav stats cache
		s.navStatsCacheMu.Lock()
		s.navStatsCache = nil
		s.navStatsCacheMu.Unlock()

		slog.Info("Config reload applied successfully")
	})
	if err := s.configReloader.Start(); err != nil {
		slog.Warn("Failed to start config reloader", "error", err)
	}

	backupDir := filepath.Join(s.cfg.Server.DataDir, "backups")

	var err error
	s.backupManager, err = NewBackupManager(s.db, s.cfg.Database.Path, backupDir, s.backupKeyHex())
	if err != nil {
		slog.Warn("Failed to initialize backup manager", "error", err)
		return
	}

	if err := s.backupManager.Start("daily"); err != nil {
		slog.Warn("Failed to start backup manager", "error", err)
	}
}

func (s *Server) SetupRoutes() {
	// Request logging middleware
	s.router.Use(middleware.RequestLogger())

	// CORS middleware — applies to all routes
	s.router.Use(middleware.CORS(s.cfg.Server.AllowedOrigins))

	// Unauthenticated routes
	s.registerPublicRoutes()

	// Protected routes
	auth := s.router.Group("/")
	auth.Use(middleware.AuthRequired(s.db))
	auth.Use(middleware.CSRFProtect())
	auth.Use(middleware.RequestBodyLimit(MaxJSONBodySize))
	auth.Use(s.apiRateLimiter.LimitByUser())
	auth.Use(s.AuditMiddleware())
	auth.Use(s.ActivityMiddleware())
	{
		s.registerAgentRoutes(auth)
		s.registerAgentCommandRoutes(auth)
		s.registerAutomationRoutes(auth)
		s.registerPluginRoutes(auth)
		s.registerTaskRoutes(auth)
		s.registerCredentialRoutes(auth)
		s.registerGenerateRoutes(auth)
		s.registerListenerRoutes(auth)
		s.registerReconRoutes(auth)
		s.registerSettingsRoutes(auth)
		s.registerDebugRoutes(auth)
		s.registerUserRoutes(auth)
		s.registerCampaignRoutes(auth)
		s.registerIntegrationRoutes(auth)
		s.registerDNSRoutes(auth)
		s.registerExtendedRoutes(auth)
		s.registerAPIKeyRoutes(auth)
	}

	// Agent Beacon API (no auth — agents check in unauthenticated)
	beaconAPI := s.router.Group("/api/v1")
	beaconAPI.Use(s.rateLimiter.Limit())
	beaconAPI.Use(s.trafficMiddleware())
	beaconAPI.Use(middleware.RequestBodyLimit(BeaconMaxBodySize))
	{
		beaconAPI.POST("/beacon", s.handleBeacon)
		beaconAPI.POST("/screen_frame", s.handleScreenFrame)

		// Malleable profile support (similar to Cobalt Strike)
		beaconAPI.POST("/generate_204", s.handleBeacon)
		beaconAPI.POST("/th", s.handleBeacon)
		beaconAPI.GET("/generate_204", s.handleBeacon)
		beaconAPI.GET("/th", s.handleBeacon)
	}

	// Protected REST API (authentication required)
	restAPI := s.router.Group("/api/v1")
	restAPI.Use(middleware.AuthRequired(s.db))
	restAPI.Use(middleware.CSRFProtect())
	restAPI.Use(middleware.RequestBodyLimit(MaxJSONBodySize))
	restAPI.Use(s.apiRateLimiter.LimitByUser())
	{
		s.registerAPIRoutes(restAPI)
	}

	// Root-level malleable profile routes (agent beacon_uri does NOT include /api/v1/ prefix).
	// These sit outside the beaconAPI group, so apply the same rate limit and
	// traffic capture explicitly. Without the limiter, these endpoints were an
	// unbounded path into the decrypt hot path.
	s.router.POST("/generate_204", middleware.RequestBodyLimit(BeaconMaxBodySize), s.rateLimiter.Limit(), s.trafficMiddleware(), s.handleBeacon)
	// GET carries the limit too: handleBeacon does c.GetRawData() which
	// buffers the whole body BEFORE decodeBeaconEnvelope's length check can
	// fire — an unauthenticated chunked body was a memory-amplification vector.
	s.router.GET("/generate_204", middleware.RequestBodyLimit(BeaconMaxBodySize), s.rateLimiter.Limit(), s.trafficMiddleware(), s.handleBeacon)

	// Catch-all for profile-defined beacon URIs (e.g. bing /th?id=...)
	s.router.GET("/th", middleware.RequestBodyLimit(BeaconMaxBodySize), s.rateLimiter.Limit(), s.trafficMiddleware(), s.handleBeacon)
	s.router.POST("/th", middleware.RequestBodyLimit(BeaconMaxBodySize), s.rateLimiter.Limit(), s.trafficMiddleware(), s.handleBeacon)
	// Custom profile URIs must beacon even when no global malleable preset is
	// enabled: per-implant profiles bake arbitrary URIs at build time. Gate on
	// the Accept header (not the malleable flag) so SPA/API traffic still 404s
	// while any non-JSON client falls through to beacon decoding, which
	// rejects non-beacon bodies with 400. Rate limit + body limit apply
	// before the decrypt hot path in all cases.
	s.router.NoRoute(func(c *gin.Context) {
		if c.GetHeader("Accept") != "application/json" {
			middleware.RequestBodyLimit(BeaconMaxBodySize)(c)
			if c.IsAborted() {
				return
			}
			s.rateLimiter.Limit()(c)
			if c.IsAborted() {
				return
			}
			s.handleBeacon(c)
			return
		}
		respondError(c, http.StatusNotFound, "not found")
	})

	// External C2 (redirector-facing, no auth, rate-limited)
	extc2 := s.router.Group("/extc2/v1")
	extc2.Use(s.rateLimiter.Limit())
	{
		extc2.POST("/receive", s.handleExtC2Receive)
		extc2.POST("/send", s.handleExtC2Send)
	}

	// Post-group authenticated routes
	s.registerMonitorRoutes(auth)
	s.registerDashboardCharts(auth)
	s.registerBOFRoutes(auth)
	s.registerMiscRoutes(auth)
}
