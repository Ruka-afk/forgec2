package server

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/scripting"
)

const (
	scriptHTTPMaxBody    = 2 << 20 // 2 MiB response cap
	scriptHTTPMaxTimeout = 20      // seconds, hard cap
	scriptQueryLimit     = 500
)

// scriptingBridge implements scripting.Bridge against the server: it enforces
// permissions from the Caller, reuses the existing task pipeline, and guards
// httpRequest against SSRF. Installed once at startup via
// scripting.GetEngine().SetBridge.
type scriptingBridge struct {
	s *Server
}

// installScriptingBridge wires the server capability layer into the scripting
// engine. Must be called during server startup, before scripts can execute.
func (s *Server) installScriptingBridge() {
	scripting.GetEngine().SetBridge(&scriptingBridge{s: s})
}

func (b *scriptingBridge) hasPerm(caller scripting.Caller, perm string) bool {
	if caller.Role == db.RoleAdmin {
		return true
	}
	return db.RoleHasPermissionDB(b.s.db, caller.Role, perm)
}

func (b *scriptingBridge) SendTask(caller scripting.Caller, agentID, taskType, params string) (uint64, error) {
	if !b.hasPerm(caller, db.PermAgentsWrite) {
		return 0, errors.New("permission denied: agents.write required")
	}
	if agentID == "" {
		return 0, errors.New("agent_id is required")
	}
	var agent db.Implant
	if err := b.s.db.First(&agent, "id = ?", agentID).Error; err != nil {
		return 0, errors.New("agent not found")
	}
	task, err := b.s.createTask(agentID, taskType, params, "", "", "", 0, 0)
	if err != nil {
		return 0, err
	}
	b.s.broadcastTaskUpdate(agentID, *task)
	return uint64(task.ID), nil
}

func (b *scriptingBridge) GetAgent(caller scripting.Caller, agentID string) (map[string]interface{}, error) {
	if !b.hasPerm(caller, db.PermAgentsRead) {
		return nil, errors.New("permission denied: agents.read required")
	}
	var agent db.Implant
	if err := b.s.db.First(&agent, "id = ?", agentID).Error; err != nil {
		return nil, errors.New("agent not found")
	}
	return implantSummary(&agent), nil
}

func (b *scriptingBridge) ListAgents(caller scripting.Caller) ([]map[string]interface{}, error) {
	if !b.hasPerm(caller, db.PermAgentsRead) {
		return nil, errors.New("permission denied: agents.read required")
	}
	var agents []db.Implant
	if err := b.s.db.Order("last_seen desc").Limit(scriptQueryLimit).Find(&agents).Error; err != nil {
		return nil, err
	}
	out := make([]map[string]interface{}, 0, len(agents))
	for i := range agents {
		out = append(out, implantSummary(&agents[i]))
	}
	return out, nil
}

// HTTPRequest is deliberately admin-only: it is the single script entry point
// with outbound network reach. SSRF guard resolves the host and rejects any
// private/loopback address (including DNS rebinding cases).
func (b *scriptingBridge) HTTPRequest(caller scripting.Caller, method, rawURL string, headers map[string]string, body string, timeoutSecs int) (map[string]interface{}, error) {
	if caller.Role != db.RoleAdmin {
		return nil, errors.New("permission denied: httpRequest requires admin")
	}
	if rawURL == "" {
		return nil, errors.New("url is required")
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, errors.New("invalid url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, errors.New("only http/https urls are allowed")
	}
	host := u.Hostname()
	if host == "" || strings.EqualFold(host, "localhost") || isPrivateIP(host) {
		return nil, errors.New("request to private/local address is not allowed")
	}
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return nil, errors.New("failed to resolve host")
	}
	for _, ip := range ips {
		if isPrivateIP(ip.String()) {
			return nil, errors.New("request to private/local address is not allowed")
		}
	}

	if timeoutSecs <= 0 {
		timeoutSecs = 10
	}
	if timeoutSecs > scriptHTTPMaxTimeout {
		timeoutSecs = scriptHTTPMaxTimeout
	}
	client := &http.Client{
		Timeout: time.Duration(timeoutSecs) * time.Second,
		// Do not follow redirects: a redirect could point at an internal host.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req, err := http.NewRequest(strings.ToUpper(method), rawURL, bodyReader)
	if err != nil {
		return nil, errors.New("failed to build request")
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.New("request failed: " + err.Error())
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, scriptHTTPMaxBody))
	if err != nil {
		return nil, errors.New("failed to read response")
	}
	respHeaders := make(map[string]interface{})
	for k, vs := range resp.Header {
		respHeaders[k] = vs
	}
	return map[string]interface{}{
		"status_code": resp.StatusCode,
		"headers":     respHeaders,
		"body":        string(data),
	}, nil
}

// Query is a whitelist-only data accessor: scripts can never run raw SQL.
// Each kind is permission-gated and bound to a fixed query shape.
func (b *scriptingBridge) Query(caller scripting.Caller, kind string, args map[string]interface{}) (interface{}, error) {
	switch kind {
	case "agents":
		if !b.hasPerm(caller, db.PermAgentsRead) {
			return nil, errors.New("permission denied: agents.read required")
		}
		q := b.s.db.Order("last_seen desc")
		if status := argString(args, "status"); status != "" {
			q = q.Where("status = ?", status)
		}
		q = q.Limit(queryLimit(args))
		var agents []db.Implant
		if err := q.Find(&agents).Error; err != nil {
			return nil, err
		}
		out := make([]map[string]interface{}, 0, len(agents))
		for i := range agents {
			out = append(out, implantSummary(&agents[i]))
		}
		return out, nil

	case "tasks":
		if !b.hasPerm(caller, db.PermAgentsRead) {
			return nil, errors.New("permission denied: agents.read required")
		}
		q := b.s.db.Order("created_at desc").Limit(queryLimit(args))
		if agentID := argString(args, "agent_id"); agentID != "" {
			q = q.Where("agent_id = ?", agentID)
		}
		var tasks []db.Task
		if err := q.Select("id, agent_id, type, command, status, created_at").Find(&tasks).Error; err != nil {
			return nil, err
		}
		return tasks, nil

	case "credentials":
		if !b.hasPerm(caller, db.PermCredsRead) {
			return nil, errors.New("permission denied: credentials.read required")
		}
		q := b.s.db.Order("created_at desc").Limit(queryLimit(args))
		if agentID := argString(args, "agent_id"); agentID != "" {
			q = q.Where("agent_id = ?", agentID)
		}
		var creds []db.CredentialEntry
		if err := q.Find(&creds).Error; err != nil {
			return nil, err
		}
		return creds, nil

	case "count_agents":
		if !b.hasPerm(caller, db.PermAgentsRead) {
			return nil, errors.New("permission denied: agents.read required")
		}
		var count int64
		q := b.s.db.Model(&db.Implant{})
		if status := argString(args, "status"); status != "" {
			q = q.Where("status = ?", status)
		}
		if err := q.Count(&count).Error; err != nil {
			return nil, err
		}
		return count, nil

	default:
		return nil, fmt.Errorf("unknown query kind: %s", kind)
	}
}

func implantSummary(a *db.Implant) map[string]interface{} {
	return map[string]interface{}{
		"id":           a.ID,
		"hostname":     a.Hostname,
		"username":     a.Username,
		"os":           a.OS,
		"arch":         a.Arch,
		"ip":           a.IP,
		"public_ip":    a.PublicIP,
		"status":       a.Status,
		"trusted":      a.Trusted,
		"domain":       a.Domain,
		"elevated":     a.Elevated,
		"integrity":    a.Integrity,
		"pid":          a.PID,
		"process_name": a.ProcessName,
		"version":      a.Version,
		"listener_id":  a.ListenerID,
		"interval":     a.CurrentInterval,
		"jitter":       a.CurrentJitter,
		"last_seen":    a.LastSeen,
		"notes":        a.Notes,
		"tags":         a.Tags,
	}
}

func argString(args map[string]interface{}, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

func queryLimit(args map[string]interface{}) int {
	if v, ok := args["limit"]; ok {
		if n, ok := toInt64(v); ok && n > 0 && n <= scriptQueryLimit {
			return int(n)
		}
	}
	return scriptQueryLimit
}

func toInt64(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	case float64:
		return int64(n), true
	}
	return 0, false
}
