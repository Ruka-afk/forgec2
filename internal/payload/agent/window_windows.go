//go:build windows
// +build windows

package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

// user32 / procGetWindowTextW are shared in agent_windows.go.
var (
	procEnumWindows           = user32.NewProc("EnumWindows")
	procIsWindowVisible       = user32.NewProc("IsWindowVisible")
	procGetWindowTextLengthW  = user32.NewProc("GetWindowTextLengthW")
	procGetWindowThreadProcID = user32.NewProc("GetWindowThreadProcessId")
	procPostMessageW          = user32.NewProc("PostMessageW")
	procIsWindow              = user32.NewProc("IsWindow")
)

const winCloseMsg = 0x0010 // WM_CLOSE

type winEntry struct {
	hwnd  uintptr
	pid   uint32
	title string
}

func enumVisibleWindows() ([]winEntry, error) {
	var out []winEntry
	cb := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		vis, _, _ := procIsWindowVisible.Call(hwnd)
		if vis == 0 {
			return 1
		}
		n, _, _ := procGetWindowTextLengthW.Call(hwnd)
		if n == 0 {
			return 1
		}
		buf := make([]uint16, n+1)
		procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(n+1))
		title := strings.TrimSpace(syscall.UTF16ToString(buf))
		if title == "" {
			return 1
		}
		var pid uint32
		procGetWindowThreadProcID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		out = append(out, winEntry{hwnd: hwnd, pid: pid, title: title})
		if len(out) >= 500 {
			return 0
		}
		return 1
	})
	r1, _, err := procEnumWindows.Call(cb, 0)
	if r1 == 0 {
		return nil, fmt.Errorf("window_list: EnumWindows failed: %v", err)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].hwnd < out[j].hwnd })
	return out, nil
}

func listWindowsWindows() (string, error) {
	wins, err := enumVisibleWindows()
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	sb.WriteString("HWND\tPID\tTITLE\n")
	for _, w := range wins {
		title := strings.ReplaceAll(w.title, "\t", " ")
		fmt.Fprintf(&sb, "%d\t%d\t%s\n", w.hwnd, w.pid, title)
	}
	fmt.Fprintf(&sb, "# windows=%d\n", len(wins))
	return sb.String(), nil
}

func closeWindowWindows(target string) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", fmt.Errorf("window_close: HWND or title substring required")
	}
	if n, err := strconv.ParseUint(target, 10, 64); err == nil {
		hwnd := uintptr(n)
		ok, _, _ := procIsWindow.Call(hwnd)
		if ok == 0 {
			return "", fmt.Errorf("window_close: not a window: %s", target)
		}
		r1, _, _ := procPostMessageW.Call(hwnd, winCloseMsg, 0, 0)
		if r1 == 0 {
			return "", fmt.Errorf("window_close: PostMessage failed for HWND %d", n)
		}
		return fmt.Sprintf("window_close: WM_CLOSE posted to HWND %d", n), nil
	}
	wins, err := enumVisibleWindows()
	if err != nil {
		return "", err
	}
	needle := strings.ToLower(target)
	for _, w := range wins {
		if strings.Contains(strings.ToLower(w.title), needle) {
			r1, _, _ := procPostMessageW.Call(w.hwnd, winCloseMsg, 0, 0)
			if r1 == 0 {
				return "", fmt.Errorf("window_close: PostMessage failed for %q", w.title)
			}
			return fmt.Sprintf("window_close: WM_CLOSE posted to %q (HWND %d, PID %d)", w.title, w.hwnd, w.pid), nil
		}
	}
	return "", fmt.Errorf("window_close: no visible window matching %q", target)
}
