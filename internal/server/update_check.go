package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// defaultUpdateCheckURL is the GitHub releases API for ForgeC2
const defaultUpdateCheckURL = "https://api.github.com/repos/forgec2/forgec2/releases/latest"

// updateCheckState holds the latest version check result
type updateCheckState struct {
	mu            sync.RWMutex
	LatestVersion string
	CheckedAt     time.Time
	Available     bool
	Error         string
}

// GitHubRelease is a minimal representation of a GitHub release
type GitHubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Prerelease bool `json:"prerelease"`
}

// initUpdateChecker starts the background version check goroutine
func (s *Server) initUpdateChecker() {
	go func() {
		time.Sleep(10 * time.Second)
		s.checkForUpdate()
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			s.checkForUpdate()
		}
	}()
}

// checkForUpdate fetches the latest version and broadcasts if newer
func (s *Server) checkForUpdate() {
	latest, err := fetchLatestVersion()
	s.updateState.mu.Lock()
	s.updateState.CheckedAt = time.Now()
	if err != nil {
		s.updateState.Error = err.Error()
		s.updateState.Available = false
		slog.Debug("Update check failed", "err", err)
		s.updateState.mu.Unlock()
		return
	}
	s.updateState.LatestVersion = latest
	s.updateState.Error = ""
	s.updateState.mu.Unlock()

	if compareVersions(latest, ServerVersion) > 0 {
		slog.Info("New version available", "current", ServerVersion, "latest", latest)
		s.updateState.mu.Lock()
		s.updateState.Available = true
		s.updateState.mu.Unlock()
		s.broadcastUpdateNotification(latest)
	} else {
		s.updateState.mu.Lock()
		s.updateState.Available = false
		s.updateState.mu.Unlock()
	}
}

// fetchLatestVersion calls the GitHub API and returns the latest tag
func fetchLatestVersion() (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", defaultUpdateCheckURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ForgeC2/"+ServerVersion)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("github api: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github api status %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return "", fmt.Errorf("parse release: %w", err)
	}
	if release.TagName == "" {
		return "", fmt.Errorf("empty tag_name in response")
	}
	return release.TagName, nil
}

// compareVersions compares two semver strings.
// Returns: 1 if v1 > v2, -1 if v1 < v2, 0 if equal
func compareVersions(v1, v2 string) int {
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var n1, n2 int
		if i < len(parts1) {
			n1, _ = strconv.Atoi(parts1[i])
		}
		if i < len(parts2) {
			n2, _ = strconv.Atoi(parts2[i])
		}
		if n1 > n2 {
			return 1
		}
		if n1 < n2 {
			return -1
		}
	}
	return 0
}

// GitHubAsset represents a file in a GitHub release
type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// GitHubReleaseFull is a full release with assets
type GitHubReleaseFull struct {
	TagName string        `json:"tag_name"`
	HTMLURL string        `json:"html_url"`
	Assets  []GitHubAsset `json:"assets"`
}

// handleCheckVersion returns a JSON with current vs latest for the UI to display
func (s *Server) handleCheckVersion(c *gin.Context) {
	s.updateState.mu.RLock()
	latest := s.updateState.LatestVersion
	available := s.updateState.Available
	checkedAt := s.updateState.CheckedAt
	errStr := s.updateState.Error
	s.updateState.mu.RUnlock()

	resp := gin.H{
		"current_version": ServerVersion,
		"is_latest":       !available || compareVersions(ServerVersion, latest) >= 0,
		"checked_at":      checkedAt,
	}
	if latest != "" {
		resp["latest_version"] = latest
	}
	if available {
		resp["download_url"] = fmt.Sprintf("https://github.com/forgec2/forgec2/releases/tag/%s", latest)
	}
	if errStr != "" {
		resp["check_error"] = errStr
	}
	c.JSON(http.StatusOK, resp)
}

// handleHotUpdate triggers a self-update: downloads the latest release binary and restarts
func (s *Server) handleHotUpdate(c *gin.Context) {
	s.updateState.mu.RLock()
	latest := s.updateState.LatestVersion
	available := s.updateState.Available
	s.updateState.mu.RUnlock()

	if !available || latest == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "没有可用的更新"})
		return
	}

	if compareVersions(latest, ServerVersion) <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "当前已是最新版本"})
		return
	}

	// Trigger async hot-update
	go func() {
		if err := s.performHotUpdate(latest); err != nil {
			slog.Error("Hot update failed", "err", err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"message":        "正在下载并应用更新，服务器将自动重启...",
		"latest_version": latest,
	})
}

// performHotUpdate downloads the latest binary, replaces itself, and restarts
func (s *Server) performHotUpdate(latest string) error {
	assets, err := fetchReleaseAssets(latest)
	if err != nil {
		return fmt.Errorf("fetch release assets: %w", err)
	}

	// Find a matching binary for this platform
	binName := "server.exe"
	ext := ".exe"
	var downloadURL string
	for _, a := range assets {
		name := strings.ToLower(a.Name)
		if strings.HasSuffix(name, ext) && (strings.Contains(name, "windows") || strings.Contains(name, "amd64") || strings.Contains(name, "x64")) {
			downloadURL = a.BrowserDownloadURL
			binName = a.Name
			break
		}
	}
	if downloadURL == "" {
		// fallback: just take any .exe
		for _, a := range assets {
			if strings.HasSuffix(strings.ToLower(a.Name), ext) {
				downloadURL = a.BrowserDownloadURL
				binName = a.Name
				break
			}
		}
	}
	if downloadURL == "" {
		return fmt.Errorf("no matching binary found in release assets")
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}
	exePath, err = filepath.Abs(exePath)
	if err != nil {
		return fmt.Errorf("resolve absolute path: %w", err)
	}

	// Download to a temp file in the same directory
	tmpPath := filepath.Join(filepath.Dir(exePath), ".update."+binName)
	if err := downloadFile(downloadURL, tmpPath); err != nil {
		return fmt.Errorf("download binary: %w", err)
	}

	// Verify the downloaded file is valid
	if fi, err := os.Stat(tmpPath); err != nil || fi.Size() == 0 {
		os.Remove(tmpPath)
		return fmt.Errorf("downloaded binary is invalid")
	}

	// Create a restart script
	var scriptPath string
	var args []string
	if ext == ".exe" {
		scriptPath = filepath.Join(filepath.Dir(exePath), ".restart.bat")
		script := fmt.Sprintf(`@echo off
timeout /t 3 /nobreak >nul
copy /Y "%s" "%s" >nul
del "%s"
start "" "%s"
`, tmpPath, exePath, tmpPath, exePath)
		if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("create restart script: %w", err)
		}
		args = []string{"cmd", "/c", scriptPath}
	} else {
		// Unix-like: shell script
		scriptPath = filepath.Join(filepath.Dir(exePath), ".restart.sh")
		script := fmt.Sprintf(`#!/bin/sh
sleep 3
mv -f "%s" "%s"
rm -f "%s"
exec "%s" "$@"
`, tmpPath, exePath, tmpPath, exePath)
		if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("create restart script: %w", err)
		}
		args = []string{"/bin/sh", scriptPath}
	}

	// Broadcast update notification
	payload := map[string]interface{}{
		"type":    "server_restarting",
		"message": "服务器正在热更新并重启...",
		"version": latest,
	}
	msg, _ := json.Marshal(payload)
	s.broadcastToClients(msg)

	slog.Info("Starting hot update, server will restart", "version", latest)

	// Start the restart script and exit
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start restart script: %w", err)
	}

	// Graceful shutdown after a brief delay for the script to start
	time.Sleep(500 * time.Millisecond)
	os.Exit(0)
	return nil
}

// fetchReleaseAssets gets the asset list for a given tag
func fetchReleaseAssets(tag string) ([]GitHubAsset, error) {
	url := fmt.Sprintf("https://api.github.com/repos/forgec2/forgec2/releases/tags/%s", tag)
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ForgeC2/"+ServerVersion)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github api: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api status %d", resp.StatusCode)
	}

	var release GitHubReleaseFull
	if err := json.Unmarshal(body, &release); err != nil {
		return nil, fmt.Errorf("parse release: %w", err)
	}
	return release.Assets, nil
}

// downloadFile downloads a URL to a local file path
func downloadFile(url, dest string) error {
	slog.Info("Downloading update binary", "url", url, "dest", dest)
	client := &http.Client{Timeout: 5 * time.Minute}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "ForgeC2/"+ServerVersion)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	written, err := io.Copy(out, resp.Body)
	if err != nil {
		os.Remove(dest)
		return err
	}
	slog.Info("Downloaded new binary", "size", written)
	return nil
}

// broadcastUpdateNotification sends a new-version alert via WebSocket
func (s *Server) broadcastUpdateNotification(latest string) {
	payload := map[string]interface{}{
		"type":          "update_available",
		"current":       ServerVersion,
		"latest":        latest,
		"download_url":  fmt.Sprintf("https://github.com/forgec2/forgec2/releases/tag/%s", latest),
	}
	msg, err := json.Marshal(payload)
	if err != nil {
		slog.Error("Failed to marshal update notification", "err", err)
		return
	}
	s.broadcastToClients(msg)
}

// handleUpdateCheck returns the current update check status as JSON
func (s *Server) handleUpdateCheck(c *gin.Context) {
	s.updateState.mu.RLock()
	defer s.updateState.mu.RUnlock()

	resp := gin.H{
		"current_version": ServerVersion,
		"checked_at":      s.updateState.CheckedAt,
		"update_available": s.updateState.Available,
	}
	if s.updateState.Available {
		resp["latest_version"] = s.updateState.LatestVersion
		resp["download_url"] = fmt.Sprintf("https://github.com/forgec2/forgec2/releases/tag/%s", s.updateState.LatestVersion)
	}
	if s.updateState.Error != "" {
		resp["check_error"] = s.updateState.Error
	}
	c.JSON(http.StatusOK, resp)
}
