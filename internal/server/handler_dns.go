package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) registerDNSRoutes(auth *gin.RouterGroup) {
	auth.GET("/dns", s.handleDNSPage)
	auth.GET("/api/dns/status", s.handleDNSStatus)
	auth.POST("/api/dns/start", s.handleDNSStart)
	auth.POST("/api/dns/stop", s.handleDNSStop)
}

func (s *Server) handleDNSPage(c *gin.Context) {
	s.renderPageOrJSON(c, gin.H{
		"Title":     "DNS C2",
		"ActiveNav": "dns",
	})
}

type dnsStatusResponse struct {
	Running     bool   `json:"running"`
	Domain      string `json:"domain"`
	Addr        string `json:"addr"`
	AgentIP     string `json:"agent_ip"`
	BeaconCount int    `json:"beacon_count"`
}

func (s *Server) handleDNSStatus(c *gin.Context) {
	resp := dnsStatusResponse{
		Running: false,
		Domain:  s.cfg.Server.DNSDomain,
		Addr:    s.cfg.Server.DNSAddr,
		AgentIP: s.cfg.Server.Host,
	}

	if dl := s.dnsListener; dl != nil {
		resp.Running = dl.IsRunning()
		resp.Domain = dl.Domain
		resp.Addr = dl.Addr
		resp.AgentIP = dl.AgentIP
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) handleDNSStart(c *gin.Context) {
	if s.dnsListener != nil && s.dnsListener.IsRunning() {
		c.JSON(http.StatusOK, gin.H{"status": "already_running"})
		return
	}

	var req struct {
		Domain    string `json:"domain"`
		Addr      string `json:"addr"`
		Server    string `json:"server"`
		TxtPrefix string `json:"txt_prefix"`
	}
	_ = c.ShouldBindJSON(&req)

	domain := req.Domain
	if domain == "" {
		domain = s.cfg.Server.DNSDomain
	}
	addr := req.Addr
	if addr == "" {
		addr = s.cfg.Server.DNSAddr
	}
	if addr == "" {
		addr = ":53"
	}
	s.cfg.Lock()
	if domain != "" {
		s.cfg.Server.DNSDomain = domain
	}
	s.cfg.Server.DNSAddr = addr
	s.cfg.Server.DNSEnabled = true
	s.cfg.Unlock()

	dl := NewDNSBeaconListener(domain, s.cfg.Server.Host, 0, addr)
	dl.SetHandler(func(agentID string, reqJSON []byte) []byte {
		var req beaconRequest
		if len(reqJSON) > 0 {
			if err := json.Unmarshal(reqJSON, &req); err != nil {
				slog.Error("DNS beacon handler unmarshal error", "err", err)
			}
		}
		if req.UUID == "" {
			req.UUID = agentID
		}
		resp := s.processBeacon(req, "")
		respJSON, ok := marshalJSONSafe(resp)
		if !ok {
			return nil
		}
		return respJSON
	})

	if err := dl.Start(); err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "DNS"))
		return
	}
	s.dnsListener = dl
	c.JSON(http.StatusOK, gin.H{"status": "started"})
}

func (s *Server) handleDNSStop(c *gin.Context) {
	if s.dnsListener == nil || !s.dnsListener.IsRunning() {
		c.JSON(http.StatusOK, gin.H{"status": "not_running"})
		return
	}

	if err := s.dnsListener.Stop(); err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "DNS"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "stopped"})
}
