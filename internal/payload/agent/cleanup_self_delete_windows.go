//go:build windows
// +build windows

package main

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// selfDelete removes the agent binary. A running image cannot be overwritten
// or deleted in place (the loader holds it, so write/delete access is refused
// while the process is mapped), so this schedules deletion for the moment the
// last handle closes (FileDispositionInfo) and, as a fallback, on the next
// reboot (MoveFileExW with MOVEFILE_DELAY_UNTIL_REBOOT). Returns an honest
// status string describing what actually happened.
func selfDelete() string {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Sprintf("self_delete: get exe path failed: %v", err)
	}

	// Best-effort overwrite: only succeeds when the file is not currently
	// mapped as a running image. Failures are swallowed — the deferred delete
	// below is the authoritative cleanup.
	if stat, err := os.Stat(exe); err == nil {
		zeroOutFile(exe, stat.Size())
	}

	ptr, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return fmt.Sprintf("self_delete: convert path failed: %v", err)
	}

	// Primary path: mark the file for deletion when its last handle closes.
	// Since Windows 8.1 the loader opens images with FILE_SHARE_DELETE, so the
	// disposition takes effect on process exit. Also try opening without write
	// access first — a mapped image refuses GENERIC_WRITE.
	if h, err := windows.CreateFile(ptr,
		windows.DELETE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0); err == nil {
		var disposition byte = 1 // FileDispositionInfo: delete when handles close
		if err := windows.SetFileInformationByHandle(h, windows.FileDispositionInfo, &disposition, 1); err == nil {
			windows.CloseHandle(h)
			return fmt.Sprintf("self_delete: deletion scheduled on last handle close (%s)", exe)
		}
		windows.CloseHandle(h)
	}

	// Fallback: delete on next reboot. Always safe, guaranteed to run once the
	// process no longer maps the image.
	if err := windows.MoveFileEx(ptr, nil, windows.MOVEFILE_DELAY_UNTIL_REBOOT); err != nil {
		return fmt.Sprintf("self_delete: unable to schedule deletion: %v", err)
	}
	return fmt.Sprintf("self_delete: deletion scheduled on next reboot (%s)", exe)
}

// zeroOutFile overwrites a file with zeros in chunks. Intended for files that
// are not currently mapped as a running image; errors are ignored.
func zeroOutFile(path string, size int64) {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return
	}
	defer f.Close()
	zero := make([]byte, 4096)
	for off := int64(0); off < size; {
		n := int64(len(zero))
		if off+n > size {
			n = size - off
		}
		if _, err := f.WriteAt(zero[:n], off); err != nil {
			return
		}
		off += n
	}
}
