package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/forgec2/forgec2/internal/infrastructure"
	"github.com/gin-gonic/gin"
)

type infraGenerateRequest struct {
	Domain     string   `json:"domain"`
	ListenPort int      `json:"listen_port"`
	BackendURL string   `json:"backend_url"`
	CertPath   string   `json:"cert_path"`
	KeyPath    string   `json:"key_path"`
	WSEnabled  bool     `json:"ws_enabled"`
	ExtC2Paths []string `json:"extc2_paths"`
	BlockedIPs []string `json:"blocked_ips"`
	UserAgent  string   `json:"user_agent"`
	Profile    string   `json:"profile"`
}

type acmeProvisionRequest struct {
	Domain    string `json:"domain"`
	Email     string `json:"email"`
	Port      int    `json:"port"`
	UseStaging bool  `json:"use_staging"`
}

func (s *Server) handleInfrastructurePage(c *gin.Context) {
	listeners := s.getListeners()
	s.renderPage(c, "infrastructure_content", gin.H{
		"Title":     "基础设施自动化",
		"ActiveNav": "infrastructure",
		"Listeners": listeners,
	})
}

func (s *Server) handleGenerateNginx(c *gin.Context) {
	var req infraGenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	rc := toRedirectorConfig(req, "nginx")
	config := infrastructure.GenerateNginxConfig(rc)
	c.JSON(http.StatusOK, gin.H{"config": config})
}

func (s *Server) handleGenerateApache(c *gin.Context) {
	var req infraGenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	rc := toRedirectorConfig(req, "apache")
	config := infrastructure.GenerateApacheConfig(rc)
	c.JSON(http.StatusOK, gin.H{"config": config})
}

func (s *Server) handleGenerateHAProxy(c *gin.Context) {
	var req infraGenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	rc := toRedirectorConfig(req, "haproxy")
	config := infrastructure.GenerateHAProxyConfig(rc)
	c.JSON(http.StatusOK, gin.H{"config": config})
}

func (s *Server) handleACMECertProvision(c *gin.Context) {
	var req acmeProvisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}
	if req.Domain == "" || req.Email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "domain and email are required"})
		return
	}

	dataDir := filepath.Join(s.cfg.Server.DataDir, "certs", req.Domain)
	if err := os.MkdirAll(dataDir, 0750); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "mkdir: " + err.Error()})
		return
	}

	acmeCfg := infrastructure.ACMEConfig{
		Domain:     req.Domain,
		Email:      req.Email,
		DataDir:    dataDir,
		UseStaging: req.UseStaging,
		Port:       req.Port,
	}

	client := infrastructure.NewACMEClient(acmeCfg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	certPEM, keyPEM, err := client.Provision(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "acme provision: " + err.Error()})
		return
	}

	_ = certPEM
	_ = keyPEM

	certFile := filepath.Join(dataDir, "fullchain.pem")
	keyFile := filepath.Join(dataDir, "privkey.pem")

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"cert_file": certFile,
		"key_file":  keyFile,
		"expires":   time.Now().Add(80 * 24 * time.Hour).Format(time.RFC3339),
	})
}

func (s *Server) handleProfileExport(c *gin.Context) {
	format := c.DefaultQuery("format", "json")

	profile := s.cfg.Malleable

	var content string
	switch format {
	case "json":
		data, _ := json.MarshalIndent(profile, "", "  ")
		content = string(data)
	case "nginx":
		content = fmt.Sprintf(`# ForgeC2 Malleable Profile — Nginx map
# Apply to your nginx redirector to replicate C2 response behavior

map $request_uri $c2_content_type {
    default "%s";
}

map $request_uri $c2_status_code {
    default %d;
}
`, profile.ContentType, profile.StatusCode)
	case "env":
		content = fmt.Sprintf(`# ForgeC2 Malleable Profile — Environment variables
FORGEC2_PROFILE_NAME=%s
FORGEC2_STATUS_CODE=%d
FORGEC2_CONTENT_TYPE=%s
`, profile.ProfileName, profile.StatusCode, profile.ContentType)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported format: " + format})
		return
	}

	c.JSON(http.StatusOK, gin.H{"content": content})
}

func toRedirectorConfig(req infraGenerateRequest, rtype string) infrastructure.RedirectorConfig {
	if req.ExtC2Paths == nil {
		req.ExtC2Paths = []string{}
	}
	if req.ListenPort == 0 {
		req.ListenPort = 443
	}
	return infrastructure.RedirectorConfig{
		Type:       rtype,
		Domain:     req.Domain,
		ListenPort: req.ListenPort,
		BackendURL: req.BackendURL,
		CertPath:   req.CertPath,
		KeyPath:    req.KeyPath,
		ExtC2Paths: req.ExtC2Paths,
		WSEnabled:  req.WSEnabled,
		BlockedIPs: req.BlockedIPs,
		UserAgent:  req.UserAgent,
		Profile:    req.Profile,
	}
}
