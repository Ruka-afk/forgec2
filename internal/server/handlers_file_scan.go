package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/forgec2/forgec2/pkg/protocol"
	"github.com/gin-gonic/gin"
)

const (
	fileScanDefaultMaxDepth      = 8
	fileScanHardMaxDepth         = 16
	fileScanDefaultMaxFiles      = 200
	fileScanHardMaxFiles         = 1000
	fileScanDefaultMaxFileBytes  = 1 * 1024 * 1024
	fileScanHardMaxFileBytes     = 8 * 1024 * 1024
	fileScanDefaultMaxTotalBytes = 32 * 1024 * 1024
	fileScanHardMaxTotalBytes    = 64 * 1024 * 1024
	fileScanDefaultMaxFindings   = 500
	fileScanHardMaxFindings      = 2000
	fileScanDefaultTimeout       = 30
	fileScanHardTimeout          = 60
)

var fileScanAllowedRules = map[string]bool{
	"email": true, "phone": true, "bank_card": true, "ssn": true,
	"aws_key": true, "private_key": true, "jwt": true, "password": true,
}

var fileScanDefaultRules = []string{"email", "phone", "bank_card"}

type fileScanRequest struct {
	Path           string `json:"path" form:"path"`
	Pattern        string `json:"pattern" form:"pattern"`
	Command        string `json:"command" form:"command"`
	Rules          string `json:"rules" form:"rules"`
	IncludeHidden  bool   `json:"include_hidden" form:"include_hidden"`
	MaxDepth       int    `json:"max_depth" form:"max_depth"`
	MaxFiles       int    `json:"max_files" form:"max_files"`
	MaxFileBytes   int64  `json:"max_file_bytes" form:"max_file_bytes"`
	MaxTotalBytes  int64  `json:"max_total_bytes" form:"max_total_bytes"`
	MaxFindings    int    `json:"max_findings" form:"max_findings"`
	TimeoutSeconds int    `json:"timeout_seconds" form:"timeout_seconds"`
}

// handleFileScan queues a bounded sensitive-data scan. It validates limits and
// detector names, then forwards only structured options. Matches are created
// later by the agent; this handler never sees or logs their values.
func (s *Server) handleFileScan(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	var req fileScanRequest
	bindJSONOrForm(c, &req)

	root := strings.TrimSpace(req.Path)
	if root == "" {
		root = strings.TrimSpace(c.PostForm("path"))
	}
	if root == "" {
		respondError(c, http.StatusBadRequest, "scan root path required")
		return
	}
	if len(root) > fileSearchMaxRootLength {
		respondError(c, http.StatusBadRequest, fmt.Sprintf("scan root path too long (max %d characters)", fileSearchMaxRootLength))
		return
	}
	if strings.ContainsRune(root, '\x00') {
		respondError(c, http.StatusBadRequest, "scan root path contains a NUL byte")
		return
	}

	pattern := strings.TrimSpace(req.Pattern)
	if pattern == "" {
		pattern = strings.TrimSpace(req.Command)
	}
	if pattern == "" {
		pattern = strings.TrimSpace(c.PostForm("pattern"))
	}
	if pattern == "" {
		pattern = strings.TrimSpace(c.PostForm("command"))
	}
	patterns := normalizeServerFileSearchPatterns(pattern)
	if len(patterns) == 0 {
		patterns = []string{"*"}
	}

	rulesRaw := strings.TrimSpace(req.Rules)
	if rulesRaw == "" {
		rulesRaw = strings.TrimSpace(c.PostForm("rules"))
	}
	rules, err := normalizeServerFileScanRules(rulesRaw)
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	if req.MaxDepth < 0 || req.MaxFiles < 0 || req.MaxFileBytes < 0 || req.MaxTotalBytes < 0 || req.MaxFindings < 0 || req.TimeoutSeconds < 0 {
		respondError(c, http.StatusBadRequest, "scan limits must not be negative")
		return
	}
	maxDepth := req.MaxDepth
	if maxDepth <= 0 {
		maxDepth = fileScanDefaultMaxDepth
	}
	if maxDepth > fileScanHardMaxDepth {
		maxDepth = fileScanHardMaxDepth
	}
	maxFiles := req.MaxFiles
	if maxFiles <= 0 {
		maxFiles = fileScanDefaultMaxFiles
	}
	if maxFiles > fileScanHardMaxFiles {
		maxFiles = fileScanHardMaxFiles
	}
	maxFileBytes := req.MaxFileBytes
	if maxFileBytes <= 0 {
		maxFileBytes = fileScanDefaultMaxFileBytes
	}
	if maxFileBytes > fileScanHardMaxFileBytes {
		maxFileBytes = fileScanHardMaxFileBytes
	}
	maxTotalBytes := req.MaxTotalBytes
	if maxTotalBytes <= 0 {
		maxTotalBytes = fileScanDefaultMaxTotalBytes
	}
	if maxTotalBytes > fileScanHardMaxTotalBytes {
		maxTotalBytes = fileScanHardMaxTotalBytes
	}
	if maxTotalBytes < maxFileBytes {
		maxTotalBytes = maxFileBytes
	}
	maxFindings := req.MaxFindings
	if maxFindings <= 0 {
		maxFindings = fileScanDefaultMaxFindings
	}
	if maxFindings > fileScanHardMaxFindings {
		maxFindings = fileScanHardMaxFindings
	}
	timeoutSeconds := req.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = fileScanDefaultTimeout
	}
	if timeoutSeconds > fileScanHardTimeout {
		timeoutSeconds = fileScanHardTimeout
	}

	includeHidden := req.IncludeHidden
	if raw := strings.TrimSpace(c.PostForm("include_hidden")); raw != "" {
		includeHidden = parseLooseBool(raw)
	}

	options := map[string]any{
		"patterns":        patterns,
		"rules":           rules,
		"include_hidden":  includeHidden,
		"max_depth":       maxDepth,
		"max_files":       maxFiles,
		"max_file_bytes":  maxFileBytes,
		"max_total_bytes": maxTotalBytes,
		"max_findings":    maxFindings,
		"timeout_seconds": timeoutSeconds,
	}
	data, err := json.Marshal(options)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "could not encode scan options")
		return
	}

	task := s.issueAgentTask(c, id, TaskSpec{
		Type:    protocol.TaskTypeFileScan,
		Command: strings.Join(patterns, ","),
		Path:    root,
		Data:    string(data),
	})
	if task == nil {
		return
	}

	ruleLabel := strings.Join(rules, ",")
	slog.Info("Sensitive file scan requested", "agent_id", id, "root", root, "rules", ruleLabel,
		"max_files", maxFiles, "max_file_bytes", maxFileBytes, "max_total_bytes", maxTotalBytes)
	details := fmt.Sprintf("root=%s patterns=%s rules=%s max_depth=%d max_files=%d max_file_bytes=%d max_total_bytes=%d max_findings=%d timeout_seconds=%d",
		truncateString(root, 512), truncateString(strings.Join(patterns, ","), 512), truncateString(ruleLabel, 512),
		maxDepth, maxFiles, maxFileBytes, maxTotalBytes, maxFindings, timeoutSeconds)
	s.dispatchTask(c, task, "file_scan", details)
}

func normalizeServerFileScanRules(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return append([]string{}, fileScanDefaultRules...), nil
	}
	seen := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		rule := strings.ToLower(strings.TrimSpace(part))
		if rule == "" {
			continue
		}
		if rule == "all" {
			for name := range fileScanAllowedRules {
				seen[name] = true
			}
			continue
		}
		if !fileScanAllowedRules[rule] {
			return nil, fmt.Errorf("unknown scan rule %q", rule)
		}
		seen[rule] = true
		if len(seen) > len(fileScanAllowedRules) {
			return nil, fmt.Errorf("too many scan rules")
		}
	}
	if len(seen) == 0 {
		return append([]string{}, fileScanDefaultRules...), nil
	}
	out := make([]string, 0, len(seen))
	for _, rule := range []string{"email", "phone", "bank_card", "ssn", "aws_key", "private_key", "jwt", "password"} {
		if seen[rule] {
			out = append(out, rule)
		}
	}
	return out, nil
}
