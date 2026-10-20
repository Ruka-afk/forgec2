package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/forgec2/forgec2/pkg/protocol"
	"github.com/gin-gonic/gin"
)

const (
	fileSearchDefaultMaxDepth   = 8
	fileSearchHardMaxDepth      = 16
	fileSearchDefaultMaxResults = 2000
	fileSearchHardMaxResults    = 5000
	fileSearchDefaultTimeout    = 20
	fileSearchHardTimeout       = 60
	fileSearchMaxRootLength     = 4096
)

var fileSearchAllowedCategories = map[string]bool{
	"image": true, "video": true, "audio": true, "document": true,
	"archive": true, "key": true, "config": true, "other": true,
}

type fileSearchRequest struct {
	Path           string `json:"path" form:"path"`
	Pattern        string `json:"pattern" form:"pattern"`
	Command        string `json:"command" form:"command"`
	Categories     string `json:"categories" form:"categories"`
	MinSize        int64  `json:"min_size" form:"min_size"`
	MaxSize        int64  `json:"max_size" form:"max_size"`
	ModifiedAfter  string `json:"modified_after" form:"modified_after"`
	ModifiedBefore string `json:"modified_before" form:"modified_before"`
	IncludeHidden  bool   `json:"include_hidden" form:"include_hidden"`
	MaxDepth       int    `json:"max_depth" form:"max_depth"`
	MaxResults     int    `json:"max_results" form:"max_results"`
	TimeoutSeconds int    `json:"timeout_seconds" form:"timeout_seconds"`
}

// handleFileSearch queues a file_search task. It validates the bounded inputs
// and forwards only structured options; the agent returns a JSON result under
// the normal task-result size cap.
func (s *Server) handleFileSearch(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	var req fileSearchRequest
	bindJSONOrForm(c, &req)

	root := strings.TrimSpace(req.Path)
	if root == "" {
		root = strings.TrimSpace(c.PostForm("path"))
	}
	if root == "" {
		respondError(c, http.StatusBadRequest, "search root path required")
		return
	}
	if len(root) > fileSearchMaxRootLength {
		respondError(c, http.StatusBadRequest, fmt.Sprintf("search root path too long (max %d characters)", fileSearchMaxRootLength))
		return
	}
	if strings.ContainsRune(root, '\x00') {
		respondError(c, http.StatusBadRequest, "search root path contains a NUL byte")
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

	categories, err := parseServerFileSearchCategories(firstNonEmptyFileSearch(req.Categories, c.PostForm("categories")))
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	if req.MinSize < 0 || req.MaxSize < 0 {
		respondError(c, http.StatusBadRequest, "file size filters must not be negative")
		return
	}
	if req.MaxSize > 0 && req.MinSize > req.MaxSize {
		respondError(c, http.StatusBadRequest, "max_size must be greater than or equal to min_size")
		return
	}

	after, afterText, err := parseServerFileSearchTime(firstNonEmptyFileSearch(req.ModifiedAfter, c.PostForm("modified_after")), "modified_after")
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	before, beforeText, err := parseServerFileSearchTime(firstNonEmptyFileSearch(req.ModifiedBefore, c.PostForm("modified_before")), "modified_before")
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if !after.IsZero() && !before.IsZero() && after.After(before) {
		respondError(c, http.StatusBadRequest, "modified_after must not be later than modified_before")
		return
	}

	maxDepth := req.MaxDepth
	if maxDepth <= 0 {
		maxDepth = fileSearchDefaultMaxDepth
	}
	if maxDepth > fileSearchHardMaxDepth {
		maxDepth = fileSearchHardMaxDepth
	}
	maxResults := req.MaxResults
	if maxResults <= 0 {
		maxResults = fileSearchDefaultMaxResults
	}
	if maxResults > fileSearchHardMaxResults {
		maxResults = fileSearchHardMaxResults
	}
	timeoutSeconds := req.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = fileSearchDefaultTimeout
	}
	if timeoutSeconds > fileSearchHardTimeout {
		timeoutSeconds = fileSearchHardTimeout
	}

	includeHidden := req.IncludeHidden
	if raw := strings.TrimSpace(c.PostForm("include_hidden")); raw != "" {
		includeHidden = parseLooseBool(raw)
	}

	options := map[string]any{
		"patterns":        patterns,
		"include_hidden":  includeHidden,
		"max_depth":       maxDepth,
		"max_results":     maxResults,
		"timeout_seconds": timeoutSeconds,
	}
	if len(categories) > 0 {
		options["categories"] = categories
	}
	if req.MinSize > 0 {
		options["min_size"] = req.MinSize
	}
	if req.MaxSize > 0 {
		options["max_size"] = req.MaxSize
	}
	if afterText != "" {
		options["modified_after"] = afterText
	}
	if beforeText != "" {
		options["modified_before"] = beforeText
	}
	data, err := json.Marshal(options)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "could not encode search options")
		return
	}

	task := s.issueAgentTask(c, id, TaskSpec{
		Type:    protocol.TaskTypeFileSearch,
		Command: strings.Join(patterns, ","),
		Path:    root,
		Data:    string(data),
	})
	if task == nil {
		return
	}

	categoryLabel := "all"
	if len(categories) > 0 {
		categoryLabel = strings.Join(categories, ",")
	}
	slog.Info("File metadata search requested", "agent_id", id, "root", root, "categories", categoryLabel)
	details := fmt.Sprintf("root=%s patterns=%s categories=%s max_depth=%d max_results=%d",
		truncateString(root, 512), truncateString(strings.Join(patterns, ","), 512), categoryLabel, maxDepth, maxResults)
	s.dispatchTask(c, task, "file_search", details)
}

func firstNonEmptyFileSearch(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func parseLooseBool(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func normalizeServerFileSearchPatterns(raw string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, 4)
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p == "" || len(p) > 256 {
			continue
		}
		key := strings.ToLower(p)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, p)
		if len(out) >= 32 {
			break
		}
	}
	return out
}

func parseServerFileSearchCategories(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, 4)
	for _, part := range strings.Split(raw, ",") {
		cat := strings.ToLower(strings.TrimSpace(part))
		if cat == "" || cat == "all" {
			continue
		}
		if !fileSearchAllowedCategories[cat] {
			return nil, fmt.Errorf("unknown file category %q", cat)
		}
		if seen[cat] {
			continue
		}
		seen[cat] = true
		out = append(out, cat)
	}
	sort.Strings(out)
	return out, nil
}

func parseServerFileSearchTime(raw, field string) (time.Time, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, "", nil
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
	} {
		if t, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return t, raw, nil
		}
	}
	return time.Time{}, "", fmt.Errorf("%s must be RFC3339 or YYYY-MM-DD", field)
}
