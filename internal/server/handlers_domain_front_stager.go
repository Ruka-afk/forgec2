package server

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// frontDomainState mirrors the frontend's FrontDomain shape.
type frontDomainState struct {
	Domain    string `json:"domain"`
	Healthy   bool   `json:"healthy"`
	Active    bool   `json:"active"`
	LastCheck string `json:"last_check"`
	Error     string `json:"error,omitempty"`
}

func (s *Server) frontHasDomain(d string) bool {
	for _, x := range s.domainFrontDomains {
		if x == d {
			return true
		}
	}
	return false
}

// frontCheckDomain performs a lightweight HTTPS health probe.
func (s *Server) frontCheckDomain(domain string) frontDomainState {
	st := frontDomainState{Domain: domain, LastCheck: time.Now().Format(time.RFC3339)}

	// Validate: resolve DNS first and reject private/loopback IPs
	ips, err := net.LookupHost(domain)
	if err != nil {
		st.Error = sanitizeError(err, "Domain front operation")
		st.Healthy = false
		return st
	}
	for _, ip := range ips {
		if parsed := net.ParseIP(ip); parsed != nil && (parsed.IsPrivate() || parsed.IsLoopback() || parsed.IsLinkLocalUnicast()) {
			st.Error = "domain resolves to private IP: " + ip
			st.Healthy = false
			return st
		}
	}

	resp, err := s.httpClient.Get("https://" + domain)
	if err != nil {
		st.Error = sanitizeError(err, "Domain front operation")
		st.Healthy = false
		return st
	}
	defer resp.Body.Close()
	st.Healthy = resp.StatusCode < 400
	return st
}

// frontRefresh re-checks every configured domain and updates status.
func (s *Server) frontRefresh() {
	s.domainFrontMu.Lock()
	defer s.domainFrontMu.Unlock()
	for _, d := range s.domainFrontDomains {
		st := s.frontCheckDomain(d)
		if s.domainFrontAuto {
			st.Active = true
		}
		s.domainFrontStatus[d] = &st
	}
}

func (s *Server) frontDomains() []frontDomainState {
	s.domainFrontMu.Lock()
	defer s.domainFrontMu.Unlock()
	out := make([]frontDomainState, 0, len(s.domainFrontDomains))
	for _, d := range s.domainFrontDomains {
		if st, ok := s.domainFrontStatus[d]; ok {
			out = append(out, *st)
		} else {
			out = append(out, frontDomainState{Domain: d})
		}
	}
	return out
}

func (s *Server) handleAPIInfraFrontList(c *gin.Context) {
	respond(c, gin.H{
		"domains":       s.frontDomains(),
		"auto_failover": s.domainFrontAuto,
	})
}

func (s *Server) handleAPIInfraFrontCheck(c *gin.Context) {
	s.frontRefresh()
	respond(c, gin.H{"domains": s.frontDomains()})
}

func (s *Server) handleAPIInfraFrontConfig(c *gin.Context) {
	var req struct {
		Domains      []string `json:"domains"`
		AutoFailover bool     `json:"auto_failover"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}

	s.domainFrontMu.Lock()
	s.domainFrontDomains = req.Domains
	s.domainFrontAuto = req.AutoFailover
	for k := range s.domainFrontStatus {
		if !s.frontHasDomain(k) {
			delete(s.domainFrontStatus, k)
		}
	}
	s.domainFrontMu.Unlock()

	s.frontRefresh()
	respond(c, gin.H{
		"domains":       s.frontDomains(),
		"auto_failover": s.domainFrontAuto,
	})
}

func (s *Server) handleAPIDomainFronting(c *gin.Context) {
	respond(c, gin.H{
		"items":         s.frontDomains(),
		"auto_failover": s.domainFrontAuto,
	})
}

func (s *Server) handleAPIRPortFwdStatus(c *gin.Context) {
	s.rportfwdMu.Lock()
	defer s.rportfwdMu.Unlock()

	type fwdSession struct {
		AgentID   string `json:"agent_id"`
		LocalPort int    `json:"local_port"`
		ForwardTo string `json:"forward_to"`
		Active    bool   `json:"active"`
	}
	sessions := make([]fwdSession, 0, len(s.rportfwdListeners))
	for _, relay := range s.rportfwdListeners {
		sessions = append(sessions, fwdSession{
			AgentID:   relay.agentID,
			LocalPort: relay.localPort,
			ForwardTo: relay.forwardTarget,
			Active:    relay.listener != nil,
		})
	}
	respond(c, gin.H{"sessions": sessions})
}

func (s *Server) handleAPIStagerTokens(c *gin.Context) {
	var tokens []db.StagerToken
	if err := s.db.Order("created_at desc").Limit(500).Find(&tokens).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Domain front operation"))
		return
	}
	respond(c, gin.H{"success": true, "data": tokens})
}

type stagerRegisterRequest struct {
	ListenerID    uint   `json:"listener_id"`
	Arch          string `json:"arch"`
	OS            string `json:"os"`
	Format        string `json:"format"`
	TTLMinutes    int    `json:"ttl_minutes"`
	UserAgent     string `json:"user_agent"`
	Profile       string `json:"profile"`
	SkipTLSVerify bool   `json:"skip_tls_verify"`
	DNSDomain     string `json:"dns_domain"`
	DNSServer     string `json:"dns_server"`
}

func (s *Server) handleAPIStagerRegister(c *gin.Context) {
	var req stagerRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, sanitizeError(err, "Domain front operation"))
		return
	}
	if req.OS == "" {
		req.OS = "windows"
	}
	if req.Arch == "" {
		req.Arch = "amd64"
	}
	if req.Format == "" {
		req.Format = "exe"
	}
	if req.TTLMinutes <= 0 {
		req.TTLMinutes = 60
	}

	var listener db.Listener
	if err := s.db.First(&listener, req.ListenerID).Error; err != nil {
		respondError(c, http.StatusBadRequest, "listener not found")
		return
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Domain front operation"))
		return
	}
	tokenHex := hex.EncodeToString(buf)

	expiresAt := time.Now().Add(time.Duration(req.TTLMinutes) * time.Minute)
	st := db.StagerToken{
		Token:        tokenHex,
		ListenerID:   req.ListenerID,
		Architecture: req.Arch,
		OS:           req.OS,
		Format:       req.Format,
		ExpiresAt:    expiresAt,
		CreatedBy:    s.currentUsername(c),
	}
	if err := s.db.Create(&st).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Domain front operation"))
		return
	}

	scheme := listener.Scheme
	if scheme == "" {
		scheme = "http"
	}
	stagerURL := fmt.Sprintf("%s://%s:%d/stage/%s", scheme, listener.Host, listener.Port, tokenHex)

	respond(c, gin.H{
		"token":       tokenHex,
		"stager_url":  stagerURL,
		"stage2_size": 0,
		"expires_at":  expiresAt.Format(time.RFC3339),
		"token_id":    st.ID,
	})
}

func (s *Server) handleAPIStagerDelete(c *gin.Context) {
	id := c.Param("id")
	if err := s.db.Delete(&db.StagerToken{}, "id = ?", id).Error; err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "Domain front operation"))
		return
	}
	s.LogAuditRecord(c, "delete_stager_token", "stager_token", id, "Stager token deleted", true, nil)
	respond(c, gin.H{"success": true})
}

func (s *Server) handleAPITokenRevert(c *gin.Context) {
	result := s.db.Model(&db.TokenEntry{}).Where("active = ?", true).Update("active", false)
	if result.Error != nil {
		slog.Error("Failed to deactivate all tokens", "err", result.Error)
		respondError(c, http.StatusInternalServerError, "failed to revert tokens")
		return
	}
	s.LogAuditRecord(c, "token_revert_all", "system", "", fmt.Sprintf("revoked %d active token(s)", result.RowsAffected), true, nil)
	respond(c, gin.H{"success": true, "revoked": result.RowsAffected})
}
