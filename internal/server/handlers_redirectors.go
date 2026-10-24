package server

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
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

// redirectorView is the API shape for a redirector. The model carries the SSH
// private key and the encrypted SSH password; neither may ever reach a
// response body — the list endpoint used to serialize the whole model, handing
// every operator with settings.read the SSH keys of every tenant's redirector
// infrastructure. Presence is reported instead of the secret itself, the same
// shape aiProfileView uses for AI provider API keys.
type redirectorView struct {
	ID             uint      `json:"id"`
	Name           string    `json:"name"`
	Host           string    `json:"host"`
	Type           string    `json:"type"`
	Status         string    `json:"status"`
	Config         string    `json:"config"`
	SSHUser        string    `json:"ssh_user"`
	SSHPort        int       `json:"ssh_port"`
	HasSSHKey      bool      `json:"has_ssh_key"`
	HasSSHPassword bool      `json:"has_ssh_password"`
	LastSeen       time.Time `json:"last_seen"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func makeRedirectorView(rd db.Redirector) redirectorView {
	return redirectorView{
		ID:             rd.ID,
		Name:           rd.Name,
		Host:           rd.Host,
		Type:           rd.Type,
		Status:         rd.Status,
		Config:         rd.Config,
		SSHUser:        rd.SSHUser,
		SSHPort:        rd.SSHPort,
		HasSSHKey:      strings.TrimSpace(rd.SSHKey) != "",
		HasSSHPassword: strings.TrimSpace(rd.SSHPassword) != "",
		LastSeen:       rd.LastSeen,
		CreatedAt:      rd.CreatedAt,
		UpdatedAt:      rd.UpdatedAt,
	}
}

// loadRedirectorScoped resolves a redirector within the caller's tenant. A
// redirector holds SSH credentials and the operator's target host, so an
// unscoped read/write would let one tenant edit another tenant's infrastructure.
func (s *Server) loadRedirectorScoped(c *gin.Context, dest *db.Redirector, id string) bool {
	if err := s.tenantScope(s.db, c).First(dest, "id = ?", id).Error; err != nil {
		respondError(c, http.StatusNotFound, "redirector not found")
		return false
	}
	return true
}

// handleRedirectorList lists configured redirectors, scoped to the caller's
// tenant and stripped of SSH credential material.
func (s *Server) handleRedirectorList(c *gin.Context) {
	var items []db.Redirector
	if err := s.tenantScope(s.db, c).Order("created_at desc").Limit(200).Find(&items).Error; err != nil {
		slog.Error("ListAll query failed", "error", err)
		respondError(c, http.StatusInternalServerError, "failed to query data")
		return
	}
	views := make([]redirectorView, 0, len(items))
	for _, rd := range items {
		views = append(views, makeRedirectorView(rd))
	}
	respond(c, gin.H{"redirectors": views, "total": len(views)})
}

// handleRedirectorCreate adds a new redirector.
func (s *Server) handleRedirectorCreate(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
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

	s.configMu.RLock()
	credKey := s.cfg.Crypto.TotpKey
	s.configMu.RUnlock()
	encPassword, err := encryptSecret(req.SSHPassword, credKey)
	if err != nil {
		// Never fall through to an empty SSHPassword: encryptSecret returns an
		// empty string when the key is bad, and writing that over the stored
		// one silently erases the operator's credential while reporting success.
		respondError(c, http.StatusInternalServerError, "failed to encrypt SSH credentials")
		return
	}
	rd := db.Redirector{
		TenantID:    s.currentTenantID(c),
		Name:        req.Name,
		Host:        req.Host,
		Type:        req.Type,
		Config:      req.Config,
		SSHUser:     req.SSHUser,
		SSHPort:     req.SSHPort,
		SSHKey:      req.SSHKey,
		SSHPassword: encPassword,
		Status:      "inactive",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := s.db.Create(&rd).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Redirector operation"))
		return
	}
	s.syncRedirectorProbe(&rd)
	respond(c, gin.H{"id": rd.ID})
}

// handleRedirectorUpdate edits an existing redirector.
func (s *Server) handleRedirectorUpdate(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")
	var rd db.Redirector
	if !s.loadRedirectorScoped(c, &rd, id) {
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
		s.configMu.RLock()
		credKey := s.cfg.Crypto.TotpKey
		s.configMu.RUnlock()
		encPassword, err := encryptSecret(req.SSHPassword, credKey)
		if err != nil {
			// An empty result on a bad key would wipe the stored credential, so
			// refuse instead of reporting a success that destroyed it.
			respondError(c, http.StatusInternalServerError, "failed to encrypt SSH credentials")
			return
		}
		rd.SSHPassword = encPassword
	}
	if req.Status != "" {
		rd.Status = req.Status
	}
	rd.UpdatedAt = time.Now()
	if err := s.db.Save(&rd).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to update redirector")
		return
	}
	s.syncRedirectorProbe(&rd)
	respond(c, gin.H{})
}

// handleRedirectorDelete removes a redirector.
func (s *Server) handleRedirectorDelete(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	id := c.Param("id")

	var rd db.Redirector
	if !s.loadRedirectorScoped(c, &rd, id) {
		return
	}
	if s.circuitBreaker != nil {
		s.circuitBreaker.UnregisterTarget(redirectorTargetID(&rd))
	}
	res := s.db.Delete(&db.Redirector{}, "id = ?", id)
	if res.Error != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(res.Error, "Redirector operation"))
		return
	}
	if res.RowsAffected == 0 {
		respondError(c, http.StatusNotFound, "redirector not found")
		return
	}
	respond(c, gin.H{})
}

// resolveDeployTarget maps an operator-supplied host/port onto a redirector the
// caller owns. Both SSH endpoints dial an address chosen entirely by the
// request body, which turned the C2 server into an arbitrary host:port probe;
// anchoring the probe to a redirector in the caller's own tenant removes the
// probe and the cross-tenant write in one step, while still permitting probes
// against private ranges (an internal redirector is a legitimate deployment).
func (s *Server) resolveDeployTarget(c *gin.Context, host string, port int) (db.Redirector, bool) {
	var rd db.Redirector
	query := s.tenantScope(s.db, c).Where("host = ?", host)
	if port != 0 {
		query = query.Where("ssh_port = ?", port)
	}
	if err := query.Order("id ASC").First(&rd).Error; err != nil {
		return db.Redirector{}, false
	}
	return rd, true
}

// handleRedirectorTestSSH verifies TCP reachability of the SSH endpoint of a
// redirector the caller owns.
func (s *Server) handleRedirectorTestSSH(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	var req struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		User     string `json:"user"`
		Key      string `json:"ssh_key"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid body")
		return
	}
	if req.Port == 0 {
		req.Port = DefaultSSHPort
	}
	if _, ok := s.resolveDeployTarget(c, req.Host, req.Port); !ok {
		respondError(c, http.StatusNotFound, "redirector not found")
		return
	}
	addr := net.JoinHostPort(req.Host, strconv.Itoa(req.Port))
	conn, err := net.DialTimeout("tcp", addr, ReachabilityDialTimeout)
	if err != nil {
		respondError(c, http.StatusServiceUnavailable, "SSH endpoint unreachable")
		return
	}
	conn.Close()
	s.LogAuditRecord(c, "redirector_test_ssh", "redirector", req.Host,
		fmt.Sprintf("SSH endpoint reachable on port %d", req.Port), true, nil)
	respond(c, gin.H{"reachable": true, "message": "SSH endpoint reachable"})
}

// validateRedirectorDomain bounds the values interpolated into the generated
// server config. The templates are printed straight from the request body and
// the UI then tells the operator to deploy them, so an unvalidated domain or
// protocol is config injection into the redirector itself.
func validateRedirectorDomain(domain string) error {
	if domain == "" {
		return fmt.Errorf("domain is required")
	}
	if len(domain) > 253 {
		return fmt.Errorf("domain is too long")
	}
	for _, label := range strings.Split(domain, ".") {
		if label == "" {
			return fmt.Errorf("domain has an empty label")
		}
		if len(label) > 63 {
			return fmt.Errorf("domain label is too long")
		}
		for _, r := range label {
			switch {
			case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			default:
				return fmt.Errorf("domain contains an invalid character")
			}
		}
	}
	return nil
}

// validRedirectorProtocol keeps the generated proxy_pass target to the two
// schemes a redirector can actually front; anything else (including a value
// crafted to break out of the URL) is refused.
func validRedirectorProtocol(protocol string) bool {
	return protocol == "http" || protocol == "https"
}

// handleRedirectorGenerate returns a server config template for the given type.
func (s *Server) handleRedirectorGenerate(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	rdType := c.Param("type")
	var req struct {
		Domain     string `json:"domain"`
		Listener   string `json:"listener"`
		Protocol   string `json:"protocol"`
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
	if err := validateRedirectorDomain(req.Domain); err != nil {
		respondError(c, http.StatusBadRequest, sanitizeError(err, "Redirector operation"))
		return
	}
	if !validRedirectorProtocol(req.Protocol) {
		respondError(c, http.StatusBadRequest, "protocol must be http or https")
		return
	}
	if req.TargetPort < 1 || req.TargetPort > 65535 {
		respondError(c, http.StatusBadRequest, "target_port must be between 1 and 65535")
		return
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
	respond(c, gin.H{"config": config, "type": rdType})
}

// handleRedirectorDeploySSH best-effort deploy: verifies reachability and
// reports the config to push. Actual remote execution requires an SSH
// client; here we confirm connectivity and return the generated config.
func (s *Server) handleRedirectorDeploySSH(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
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
	// The target must be a redirector the caller owns; otherwise this endpoint
	// let the C2 server dial an arbitrary address and then flip every matching
	// host to "active" across all tenants.
	rd, ok := s.resolveDeployTarget(c, req.Host, req.Port)
	if !ok {
		respondError(c, http.StatusNotFound, "redirector not found")
		return
	}
	addr := net.JoinHostPort(req.Host, strconv.Itoa(req.Port))
	conn, err := net.DialTimeout("tcp", addr, ReachabilityDialTimeout)
	if err != nil {
		respondError(c, http.StatusServiceUnavailable, sanitizeError(err, "Redirector operation"))
		return
	}
	conn.Close()

	if req.Config == "" {
		domain := req.Domain
		if domain == "" {
			domain = rd.Host
		}
		req.Config = s.generateRedirectorConfig(req.Type, domain, "https", "127.0.0.1", req.Port)
	}
	// Flip only the resolved row, by id: the previous `WHERE host = ?` matched
	// every tenant's redirector sharing that hostname.
	if err := s.db.Model(&db.Redirector{}).Where("id = ?", rd.ID).Update("status", "active").Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Redirector operation"))
		return
	}
	if err := s.db.First(&rd, "id = ?", rd.ID).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Redirector operation"))
		return
	}
	s.syncRedirectorProbe(&rd)
	s.LogAuditRecord(c, "redirector_deploy_ssh", "redirector", fmt.Sprint(rd.ID),
		fmt.Sprintf("Redirector %s reachable on port %d; config returned for manual push", rd.Host, req.Port), true, nil)
	respond(c, gin.H{
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
