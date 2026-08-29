//go:build linux || darwin
// +build linux darwin

package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

func usbEnum() (string, error) {
	if runtime.GOOS == "darwin" {
		return usbEnumDarwin()
	}
	return usbEnumLinux()
}

func usbEnumLinux() (string, error) {
	var sb strings.Builder
	sb.WriteString("=== block devices (Linux /sys/block removable) ===\n")
	sb.WriteString("name\tremovable\tsize_bytes\tmodel\tmount\n")
	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		return "", fmt.Errorf("usb_enum: %w", err)
	}
	mounts := linuxMounts()
	found := 0
	for _, e := range entries {
		name := e.Name()
		remRaw, _ := os.ReadFile(filepath.Join("/sys/block", name, "removable"))
		rem := strings.TrimSpace(string(remRaw)) == "1"
		sizeRaw, _ := os.ReadFile(filepath.Join("/sys/block", name, "size"))
		sectors, _ := strconv.ParseInt(strings.TrimSpace(string(sizeRaw)), 10, 64)
		modelRaw, _ := os.ReadFile(filepath.Join("/sys/block", name, "device", "model"))
		model := strings.TrimSpace(string(modelRaw))
		dev := "/dev/" + name
		mount := mounts[dev]
		if mount == "" {
			for k, v := range mounts {
				if strings.HasPrefix(k, dev) {
					mount = v
					break
				}
			}
		}
		fmt.Fprintf(&sb, "\t%s\t%v\t%d\t%s\t%s\n", name, rem, sectors*512, model, mount)
		found++
	}
	if found == 0 {
		sb.WriteString("(no /sys/block entries)\n")
	}
	return sb.String(), nil
}

func linuxMounts() map[string]string {
	out := map[string]string{}
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) >= 2 {
			out[fields[0]] = fields[1]
		}
	}
	return out
}

func usbEnumDarwin() (string, error) {
	var sb strings.Builder
	sb.WriteString("=== volumes (macOS diskutil + /Volumes) ===\n")
	cmd := exec.Command("diskutil", "list")
	if out, err := cmd.CombinedOutput(); err == nil {
		sb.Write(out)
	} else {
		sb.WriteString("diskutil list: " + err.Error() + "\n")
	}
	entries, err := os.ReadDir("/Volumes")
	if err == nil {
		sb.WriteString("\n--- /Volumes ---\n")
		for _, e := range entries {
			sb.WriteString(e.Name() + "\n")
		}
	}
	return sb.String(), nil
}

func unixRemovableMounts() []string {
	var mounts []string
	if runtime.GOOS == "darwin" {
		entries, err := os.ReadDir("/Volumes")
		if err != nil {
			return mounts
		}
		for _, e := range entries {
			if e.Name() == "Macintosh HD" {
				continue
			}
			mounts = append(mounts, filepath.Join("/Volumes", e.Name()))
		}
		return mounts
	}
	sys, err := os.ReadDir("/sys/block")
	if err != nil {
		return mounts
	}
	mp := linuxMounts()
	for _, e := range sys {
		remRaw, _ := os.ReadFile(filepath.Join("/sys/block", e.Name(), "removable"))
		if strings.TrimSpace(string(remRaw)) != "1" {
			continue
		}
		dev := "/dev/" + e.Name()
		if m := mp[dev]; m != "" {
			mounts = append(mounts, m)
			continue
		}
		for k, v := range mp {
			if strings.HasPrefix(k, dev) && v != "" {
				mounts = append(mounts, v)
				break
			}
		}
	}
	return mounts
}

func isRemovablePath(path string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	abs = filepath.Clean(abs)
	for _, m := range unixRemovableMounts() {
		m = filepath.Clean(m)
		if abs == m || strings.HasPrefix(abs, m+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

func usbDrop(src, destRoot string, hide bool) (string, error) {
	if _, err := os.Stat(src); err != nil {
		return "", fmt.Errorf("usb_drop: source: %w", err)
	}
	if strings.TrimSpace(destRoot) == "" {
		ms := unixRemovableMounts()
		if len(ms) == 0 {
			return "", fmt.Errorf("usb_drop: no removable volume mounted")
		}
		destRoot = ms[0]
	}
	if !isRemovablePath(destRoot) {
		return "", fmt.Errorf("usb_drop: destination %s is not a removable mount", destRoot)
	}
	dst := usbDropDestName(src, destRoot)
	n, err := copyRegularFile(src, dst)
	if err != nil {
		return "", fmt.Errorf("usb_drop copy: %w", err)
	}
	if hide {
		_ = os.Chmod(dst, 0o600)
	}
	return fmt.Sprintf("usb_drop: copied %d bytes to %s hide=%v", n, dst, hide), nil
}
