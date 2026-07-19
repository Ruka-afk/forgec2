package server

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/forgec2/forgec2/internal/db"
)

type redirectorRequest struct {
	Name        string `json:"name"`
	Host        string `json:"host"`
	Type        string `json:"type"`
	Config      string `json:"config"`
	SSHUser     string `json:"ssh_user"`
	SSHPort     int    `json:"ssh_port"`
	SSHKey      string `json:"ssh_key"`
	SSHPassword string `json:"ssh_password"`
	Status      string `json:"status"`
}

// handleRedirectorList lists configured redirectors.
func (s *Server) handleRedirectorList(c *gin.Context) {
	var items []db.Redirector
	s.listAll(c, &items, "created_at desc")
}

// handleRedirectorCreate adds a new redirector.
func (s *Server) handleRedirectorCreate(c *gin.Context) {
	var req redirectorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid body")
		return
	}
	if req.Name == "" || req.Host == "" {
		respondError(c, http.StatusBadRequest, "name and host required")
		return
	}
	if req.Type == "" {
		req.Type = "nginx"
	}
	if req.SSHPort == 0 {
		req.SSHPort = DefaultSSHPort
	}

	rd := db.Redirector{
		Name:      req.Name,
		Host:      req.Host,
		Type:      req.Type,
		Config:    req.Config,
		SSHUser:   req.SSHUser,
		SSHPort:   req.SSHPort,
		SSHKey:    req.SSHKey,
		SSHPassword: req.SSHPassword,
		Status:    "inactive",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := s.db.Create(&rd).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Redirector operation"))
		return
	}
	respond(c, gin.H{"success": true, "id": rd.ID})
}

// handleRedirectorUpdate edits an existing redirector.
func (s *Server) handleRedirectorUpdate(c *gin.Context) {
	id := c.Param("id")
	var rd db.Redirector
	if !s.findOrFail(c, &rd, id, "redirector") {
		return
	}
	var req redirectorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid body")
		return
	}
	if req.Name != "" {
		rd.Name = req.Name
	}
	if req.Host != "" {
		rd.Host = req.Host
	}
	if req.Type != "" {
		rd.Type = req.Type
	}
	if req.Config != "" {
		rd.Config = req.Config
	}
	if req.SSHUser != "" {
		rd.SSHUser = req.SSHUser
	}
	if req.SSHPort != 0 {
		rd.SSHPort = req.SSHPort
	}
	if req.SSHKey != "" {
		rd.SSHKey = req.SSHKey
	}
	if req.SSHPassword != "" {
		rd.SSHPassword = req.SSHPassword
	}
	if req.Status != "" {
		rd.Status = req.Status
	}
	rd.UpdatedAt = time.Now()
	if err := s.db.Save(&rd).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to update redirector")
		return
	}
	respond(c, gin.H{"success": true})
}

// handleRedirectorDelete removes a redirector.
func (s *Server) handleRedirectorDelete(c *gin.Context) {
	id := c.Param("id")
	if err := s.db.Delete(&db.Redirector{}, "id = ?", id).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Redirector operation"))
		return
	}
	respond(c, gin.H{"success": true})
}

// handleRedirectorTestSSH verifies TCP reachability of the SSH endpoint.
func (s *Server) handleRedirectorTestSSH(c *gin.Context) {
	var req struct {
		Host   string `json:"host"`
		Port   int    `json:"port"`
		User   string `json:"user"`
		Key    string `json:"ssh_key"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid body")
		return
	}
	if req.Port == 0 {
		req.Port = DefaultSSHPort
	}
	addr := net.JoinHostPort(req.Host, strconv.Itoa(req.Port))
	conn, err := net.DialTimeout("tcp", addr, ReachabilityDialTimeout)
	if err != nil {
		respond(c, gin.H{"success": false, "reachable": false, "error": sanitizeError(err, "Redirector operation")})
		return
	}
	conn.Close()
	respond(c, gin.H{"success": true, "reachable": true, "message": "SSH endpoint reachable"})
}

// handleRedirectorGenerate returns a server config template for the given type.
func (s *Server) handleRedirectorGenerate(c *gin.Context) {
	rdType := c.Param("type")
	var req struct {
		Domain    string `json:"domain"`
		Listener  string `json:"listener"`
		Protocol  string `json:"protocol"`
		TargetHost string `json:"target_host"`
		TargetPort int    `json:"target_port"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Domain == "" {
		req.Domain = "c2.example.com"
	}
	if req.TargetPort == 0 {
		req.TargetPort = 8080
	}
	if req.Protocol == "" {
		req.Protocol = "https"
	}

	var config string
	switch rdType {
	case "nginx":
		config = fmt.Sprintf(`server {
    listen 80;
    server_name %s;
    location / {
        proxy_pass %s://127.0.0.1:%d;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}`, req.Domain, req.Protocol, req.TargetPort)
	case "apache":
		config = fmt.Sprintf(`<VirtualHost *:80>
    ServerName %s
    ProxyPreserveHost On
    ProxyPass / %s://127.0.0.1:%d/
    ProxyPassReverse / %s://127.0.0.1:%d/
</VirtualHost>`, req.Domain, req.Protocol, req.TargetPort, req.Protocol, req.TargetPort)
	case "haproxy":
		config = fmt.Sprintf(`frontend forgec2
    bind *:80
    default_backend c2_nodes

backend c2_nodes
    server c2 127.0.0.1:%d check`, req.TargetPort)
	default:
		config = "# unknown redirector type: " + rdType
	}
	respond(c, gin.H{"success": true, "config": config, "type": rdType})
}

// handleRedirectorDeploySSH best-effort deploy: verifies reachability and
// reports the config to push. Actual remote execution requires an SSH
// client; here we confirm connectivity and return the generated config.
func (s *Server) handleRedirectorDeploySSH(c *gin.Context) {
	var req struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		User     string `json:"user"`
		Key      string `json:"ssh_key"`
		Password string `json:"password"`
		Type     string `json:"type"`
		Domain   string `json:"domain"`
		Config   string `json:"config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid body")
		return
	}
	if req.Port == 0 {
		req.Port = DefaultSSHPort
	}
	addr := net.JoinHostPort(req.Host, strconv.Itoa(req.Port))
	conn, err := net.DialTimeout("tcp", addr, ReachabilityDialTimeout)
	if err != nil {
		respond(c, gin.H{"success": false, "error": sanitizeError(err, "Redirector operation")})
		return
	}
	conn.Close()

	if req.Config == "" {
		gen := s.generateRedirectorConfig(req.Type, req.Domain, "https", "127.0.0.1", req.Port)
		req.Config = gen
	}
	// Mark the matching redirector active when identifiable.
	if req.Host != "" {
		s.db.Model(&db.Redirector{}).Where("host = ?", req.Host).Update("status", "active")
	}
	respond(c, gin.H{
		"success":  true,
		"message": "Redirector reachable. Push the generated config via your SSH client.",
		"config":  req.Config,
	})
}

// generateRedirectorConfig is a shared helper for config text.
func (s *Server) generateRedirectorConfig(rdType, domain, protocol, targetHost string, targetPort int) string {
	switch rdType {
	case "nginx":
		return fmt.Sprintf("server {\n    listen 80;\n    server_name %s;\n    location / {\n        proxy_pass %s://%s:%d;\n    }\n}", domain, protocol, targetHost, targetPort)
	case "apache":
		return fmt.Sprintf("<VirtualHost *:80>\n    ServerName %s\n    ProxyPass / %s://%s:%d/\n</VirtualHost>", domain, protocol, targetHost, targetPort)
	case "haproxy":
		return fmt.Sprintf("backend c2_nodes\n    server c2 %s:%d check", targetHost, targetPort)
	default:
		return ""
	}
}
