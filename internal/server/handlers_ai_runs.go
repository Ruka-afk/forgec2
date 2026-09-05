package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	aiRunStatusQueued          = "queued"
	aiRunStatusRunning         = "running"
	aiRunStatusWaitingApproval = "waiting_approval"
	aiRunStatusCompleted       = "completed"
	aiRunStatusFailed          = "failed"
	aiRunStatusCancelled       = "cancelled"
	aiRunStatusInterrupted     = "interrupted"
	aiRunEventRetentionDefault = 30
	aiRunUserLimitDefault      = 2
	aiRunTenantLimitDefault    = 8
)

var aiActiveRunStatuses = []string{aiRunStatusQueued, aiRunStatusRunning, aiRunStatusWaitingApproval}

type aiPrincipal struct {
	UserID   uint
	Username string
	TenantID uint
	Role     string
}

func (p aiPrincipal) hasPermission(database *gorm.DB, permission string) bool {
	if p.Role == db.RoleAdmin {
		return true
	}
	return db.RoleHasPermissionDB(database, p.Role, permission)
}

func (s *Server) currentAIPrincipal(c *gin.Context) (aiPrincipal, bool) {
	username, _ := c.Get("user")
	role, _ := c.Get("user_role")
	p := aiPrincipal{Username: fmt.Sprint(username), Role: fmt.Sprint(role)}
	if p.Username == "" {
		return p, false
	}
	var user db.User
	if err := s.db.Select("id", "username", "role", "tenant_id").Where("username = ?", p.Username).First(&user).Error; err != nil {
		return p, false
	}
	p.UserID, p.TenantID = user.ID, user.TenantID
	if p.Role == "" {
		p.Role = user.Role
	}
	return p, p.hasPermission(s.db, db.PermAIUse)
}

type aiLiveEvent struct {
	Sequence int64
	Type     string
	Payload  string
}

type aiRunBroker struct {
	mu          sync.Mutex
	subscribers map[string]map[chan aiLiveEvent]struct{}
	cancels     map[string]context.CancelFunc
}

func newAIRunBroker() *aiRunBroker {
	return &aiRunBroker{
		subscribers: make(map[string]map[chan aiLiveEvent]struct{}),
		cancels:     make(map[string]context.CancelFunc),
	}
}

func (b *aiRunBroker) subscribe(runID string) (<-chan aiLiveEvent, func()) {
	ch := make(chan aiLiveEvent, 256)
	b.mu.Lock()
	if b.subscribers[runID] == nil {
		b.subscribers[runID] = make(map[chan aiLiveEvent]struct{})
	}
	b.subscribers[runID][ch] = struct{}{}
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		if subs := b.subscribers[runID]; subs != nil {
			delete(subs, ch)
			if len(subs) == 0 {
				delete(b.subscribers, runID)
			}
		}
		close(ch)
		b.mu.Unlock()
	}
}

func (b *aiRunBroker) publish(runID string, event aiLiveEvent, ephemeral bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	_ = ephemeral // reasoning is live-only per choice #3 (single session visible).
	for ch := range b.subscribers[runID] {
		select {
		case ch <- event:
		default:
			// Buffer full: try blocking 100ms before dropping to favor latency-first but avoid stall.
			// Single-session reasoning stays ephemeral; durable events are replayed from SQLite.
			go func(dst chan aiLiveEvent, ev aiLiveEvent, rid string) {
				select {
				case dst <- ev:
				case <-time.After(100 * time.Millisecond):
					slog.Warn("AI broker drop: stalled subscriber", "run_id", rid, "event_id", ev.Sequence, "type", ev.Type)
				}
			}(ch, event, runID)
		}
	}
}

func (b *aiRunBroker) registerCancel(runID string, cancel context.CancelFunc) {
	b.mu.Lock()
	b.cancels[runID] = cancel
	b.mu.Unlock()
}

func (b *aiRunBroker) finish(runID string) {
	b.mu.Lock()
	delete(b.cancels, runID)
	b.mu.Unlock()
}

func (b *aiRunBroker) cancel(runID string) bool {
	b.mu.Lock()
	cancel := b.cancels[runID]
	b.mu.Unlock()
	if cancel == nil {
		return false
	}
	cancel()
	return true
}

type aiRunRequest struct {
	SessionID              uint          `json:"session_id"`
	ProfileID              *uint         `json:"profile_id"`
	IdempotencyKey         string        `json:"idempotency_key"`
	Messages               []chatMessage `json:"messages"`
	AttachmentIDs          []string      `json:"attachment_ids"`
	KnowledgeCollectionIDs []uint        `json:"knowledge_collection_ids"`
	Context                struct {
		Page               string `json:"page"`
		AgentID            string `json:"agent_id"`
		AllowLowRiskWrites bool   `json:"allow_low_risk_writes"`
	} `json:"context"`
}

type aiResolvedProfile struct {
	ID                *uint
	Name              string
	Provider          string
	Model             string
	Endpoint          string
	APIKey            string
	SupportsReasoning bool
	SupportsTools     bool
}

func (s *Server) resolveAIRunProfile(principal aiPrincipal, profileID *uint) (aiResolvedProfile, error) {
	if profileID == nil || *profileID == 0 {
		var preferred db.AIProviderProfile
		if err := s.db.Where("tenant_id = ? AND enabled = ?", principal.TenantID, true).
			Order("is_default DESC, id ASC").First(&preferred).Error; err == nil {
			profileID = &preferred.ID
		}
	}
	if profileID != nil && *profileID != 0 {
		var profile db.AIProviderProfile
		query := s.db.Where("id = ? AND enabled = ? AND tenant_id = ?", *profileID, true, principal.TenantID)
		if err := query.First(&profile).Error; err != nil {
			return aiResolvedProfile{}, errors.New("AI profile not found")
		}
		if strings.TrimSpace(profile.APIKey) == "" {
			return aiResolvedProfile{}, errors.New("AI profile has no API key")
		}
		endpoint := profile.Endpoint
		if endpoint == "" {
			endpoint = aiDefaultEndpoint(profile.Provider)
		}
		return aiResolvedProfile{
			ID: &profile.ID, Name: profile.Name, Provider: profile.Provider,
			Model: profile.Model, Endpoint: endpoint, APIKey: profile.APIKey,
			SupportsReasoning: profile.SupportsReasoning, SupportsTools: profile.SupportsTools,
		}, nil
	}

	snapshot, err := s.aiProviderRequestConfigSnapshot()
	if err != nil {
		return aiResolvedProfile{}, err
	}
	if !snapshot.enabled || strings.TrimSpace(snapshot.apiKey) == "" {
		return aiResolvedProfile{}, errors.New("AI is not configured")
	}
	return aiResolvedProfile{
		Name: "Legacy default", Provider: snapshot.provider, Model: snapshot.model,
		Endpoint: snapshot.endpoint, APIKey: snapshot.apiKey,
		SupportsReasoning: aiShouldRequestReasoning(snapshot.provider, snapshot.endpoint), SupportsTools: true,
	}, nil
}

func aiDefaultEndpoint(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openai":
		return "https://api.openai.com/v1"
	case "claude", "anthropic":
		return "https://api.anthropic.com/v1"
	case "qianwen":
		return "https://dashscope.aliyuncs.com/compatible-mode/v1"
	case "zhipu":
		return "https://open.bigmodel.cn/api/paas/v4"
	case "longcat":
		return "https://api.longcat.chat/openai/v1"
	default:
		return "https://api.deepseek.com/v1"
	}
}

func (s *Server) handleAIRunsCreate(c *gin.Context) {
	principal, ok := s.currentAIPrincipal(c)
	if !ok {
		respondError(c, http.StatusForbidden, "AI use permission required")
		return
	}
	var req aiRunRequest
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, AIChatRequestMaxBytes)
	if err := c.ShouldBindJSON(&req); err != nil || req.SessionID == 0 || len(req.Messages) == 0 {
		respondError(c, http.StatusBadRequest, "session_id and messages are required")
		return
	}
	if len(req.Messages) > AIChatMaxMessages {
		respondError(c, http.StatusBadRequest, "too many messages")
		return
	}
	for _, message := range req.Messages {
		if message.Role != "user" && message.Role != "assistant" {
			respondError(c, http.StatusBadRequest, "invalid message role")
			return
		}
	}
	var session db.AIChatSession
	query := s.db.Where("id = ? AND owner_id = ? AND owner = ?", req.SessionID, principal.UserID, principal.Username)
	if principal.TenantID != 0 {
		query = query.Where("tenant_id = ?", principal.TenantID)
	}
	if err := query.First(&session).Error; err != nil {
		respondError(c, http.StatusNotFound, "session not found")
		return
	}
	if contextAgent := strings.TrimSpace(req.Context.AgentID); contextAgent != "" {
		resolved := s.resolveAgentID(contextAgent)
		var count int64
		agentQuery := s.db.Model(&db.Implant{}).Where("id = ?", resolved)
		if principal.TenantID != 0 {
			agentQuery = agentQuery.Where("tenant_id = ?", principal.TenantID)
		}
		agentQuery.Count(&count)
		if resolved == "" || count == 0 {
			respondError(c, http.StatusForbidden, "context agent is not visible to this tenant")
			return
		}
		req.Context.AgentID = resolved
	}
	if req.ProfileID == nil && session.ProfileID != nil {
		req.ProfileID = session.ProfileID
	}
	if req.IdempotencyKey == "" {
		req.IdempotencyKey = uuid.NewString()
	}
	if len(req.IdempotencyKey) > 100 {
		respondError(c, http.StatusBadRequest, "idempotency_key too long")
		return
	}
	// Serialize the short count-and-create critical section so concurrent
	// requests cannot all pass the per-user/per-tenant active-run limits.
	// Local mutex is fast path; DB transaction with BEGIN IMMEDIATE serializes
	// across multi-instance deployments (sqlite WAL).
	s.aiRunCreateMu.Lock()
	defer s.aiRunCreateMu.Unlock()
	var existing db.AIChatRun
	if err := s.db.Where("tenant_id = ? AND owner_id = ? AND idempotency_key = ?", principal.TenantID, principal.UserID, req.IdempotencyKey).First(&existing).Error; err == nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": existing, "reused": true})
		return
	}
	userLimit, tenantLimit := aiRunUserLimitDefault, aiRunTenantLimitDefault
	s.configMu.RLock()
	if s.cfg.AI.MaxRunsPerUser > 0 {
		userLimit = s.cfg.AI.MaxRunsPerUser
	}
	if s.cfg.AI.MaxRunsPerTenant > 0 {
		tenantLimit = s.cfg.AI.MaxRunsPerTenant
	}
	s.configMu.RUnlock()
	profile, err := s.resolveAIRunProfile(principal, req.ProfileID)
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	input, err := json.Marshal(req)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid run input")
		return
	}
	// Transaction with immediate lock to prevent race across processes
	var run db.AIChatRun
	txErr := s.db.Transaction(func(tx *gorm.DB) error {
		// BEGIN IMMEDIATE for sqlite to acquire reserved lock early
		if tx.Dialector.Name() == "sqlite" {
			if err := tx.Exec("BEGIN IMMEDIATE").Error; err != nil {
				// Non-fatal: the dialect may auto-handle locking, but a
				// failed BEGIN means the cross-process race guard is off.
				slog.Warn("AI run BEGIN IMMEDIATE failed, continuing without early lock", "error", err)
			}
		}
		var userActive, tenantActive int64
		if err := tx.Model(&db.AIChatRun{}).Where("owner_id = ? AND status IN ?", principal.UserID, aiActiveRunStatuses).Count(&userActive).Error; err != nil {
			return err
		}
		if err := tx.Model(&db.AIChatRun{}).Where("tenant_id = ? AND status IN ?", principal.TenantID, aiActiveRunStatuses).Count(&tenantActive).Error; err != nil {
			return err
		}
		if userActive >= int64(userLimit) || tenantActive >= int64(tenantLimit) {
			return fmt.Errorf("concurrency limit")
		}
		now := time.Now()
		run = db.AIChatRun{
			ID: uuid.NewString(), TenantID: principal.TenantID, OwnerID: principal.UserID,
			Owner: principal.Username, SessionID: session.ID, ProfileID: profile.ID,
			Provider: profile.Provider, Model: profile.Model, IdempotencyKey: req.IdempotencyKey,
			Status: aiRunStatusQueued, Input: string(input), ContextAgentID: req.Context.AgentID,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := tx.Create(&run).Error; err != nil {
			return err
		}
		return nil
	})
	if txErr != nil {
		if txErr.Error() == "concurrency limit" {
			c.Header("Retry-After", "5")
			respondError(c, http.StatusTooManyRequests, "AI run concurrency limit reached")
			return
		}
		if errors.Is(txErr, gorm.ErrDuplicatedKey) {
			if s.db.Where("tenant_id = ? AND owner_id = ? AND idempotency_key = ?", principal.TenantID, principal.UserID, req.IdempotencyKey).First(&existing).Error == nil {
				c.JSON(http.StatusOK, gin.H{"success": true, "data": existing, "reused": true})
				return
			}
		}
		// Retry once on SQLITE_BUSY
		if strings.Contains(txErr.Error(), "database is locked") || strings.Contains(txErr.Error(), "busy") {
			time.Sleep(50 * time.Millisecond)
			// Fallback to single insert attempt
			now := time.Now()
			fallback := db.AIChatRun{
				ID: uuid.NewString(), TenantID: principal.TenantID, OwnerID: principal.UserID,
				Owner: principal.Username, SessionID: session.ID, ProfileID: profile.ID,
				Provider: profile.Provider, Model: profile.Model, IdempotencyKey: req.IdempotencyKey,
				Status: aiRunStatusQueued, Input: string(input), ContextAgentID: req.Context.AgentID,
				CreatedAt: now, UpdatedAt: now,
			}
			if err := s.db.Create(&fallback).Error; err == nil {
				run = fallback
			} else {
				respondError(c, http.StatusInternalServerError, "failed to create AI run")
				return
			}
		} else {
			respondError(c, http.StatusInternalServerError, "failed to create AI run")
			return
		}
	}
	if session.ProfileID == nil && profile.ID != nil {
		s.db.Model(&db.AIChatSession{}).Where("id = ?", session.ID).Update("profile_id", *profile.ID)
	}
	s.startAIBackgroundRun(run, req, principal, profile)
	c.JSON(http.StatusAccepted, gin.H{"success": true, "data": run})
}

func (s *Server) startAIBackgroundRun(run db.AIChatRun, req aiRunRequest, principal aiPrincipal, profile aiResolvedProfile) {
	if s.aiRuns == nil {
		s.aiRuns = newAIRunBroker()
	}
	base := s.ctx
	if base == nil {
		base = context.Background()
	}
	ctx, cancel := context.WithCancel(base)
	s.aiRuns.registerCancel(run.ID, cancel)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer cancel()
		defer s.aiRuns.finish(run.ID)
		s.executeAIBackgroundRun(ctx, run, req, principal, profile)
	}()
}

func (s *Server) executeAIBackgroundRun(ctx context.Context, run db.AIChatRun, req aiRunRequest, principal aiPrincipal, profile aiResolvedProfile) {
	now := time.Now()
	s.db.Model(&db.AIChatRun{}).Where("id = ?", run.ID).Updates(map[string]interface{}{"status": aiRunStatusRunning, "started_at": now})
	seq := int64(0)
	emit := func(eventType, payload string, persist bool) {
		seq++
		live := aiLiveEvent{Sequence: seq, Type: eventType, Payload: payload}
		if persist {
			event := db.AIChatRunEvent{RunID: run.ID, Sequence: seq, Type: eventType, Payload: payload}
			if err := s.db.Create(&event).Error; err == nil {
				s.db.Model(&db.AIChatRun{}).Where("id = ?", run.ID).Update("last_event_seq", seq)
			}
		}
		s.aiRuns.publish(run.ID, live, !persist)
	}
	emit("run", fmt.Sprintf(`{"id":%q,"status":"running","model":%q,"provider":%q}`, run.ID, profile.Model, profile.Provider), true)

	lastUser := ""
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			lastUser = req.Messages[i].Content
			break
		}
	}
	if lastUser != "" {
		message := db.AIChatMessage{SessionID: run.SessionID, RunID: run.ID, Role: "user", Content: lastUser}
		if err := s.db.Create(&message).Error; err != nil {
			completed := time.Now()
			s.db.Model(&db.AIChatRun{}).Where("id = ?", run.ID).Updates(map[string]interface{}{
				"status": aiRunStatusFailed, "completed_at": completed, "error_code": "storage_encryption_unavailable",
			})
			emit("error", "Encrypted AI storage is unavailable", true)
			return
		}
	}

	s.configMu.RLock()
	sysPrompt := effectiveAISystemPrompt(s.cfg.AI.SystemPrompt)
	limits := aiConversationOptions{
		provider: profile.Provider, endpoint: profile.Endpoint, apiKey: profile.APIKey,
		maxConversationTurns:  s.cfg.AI.MaxConversationTurns,
		maxToolRounds:         s.cfg.AI.MaxToolRounds,
		maxDuplicateToolCalls: s.cfg.AI.MaxDuplicateToolCalls,
	}
	s.configMu.RUnlock()
	if contextText := s.buildAIRunContext(principal, run.SessionID, lastUser, req.AttachmentIDs, req.KnowledgeCollectionIDs); contextText != "" {
		sysPrompt += "\n\n" + contextText
	}
	// Inject live situation snapshot for runs to match legacy chat parity (tenant-aware)
	if snap := s.buildSituationSnapshot(); snap != "" {
		sysPrompt += "\n\n" + snap
	}
	reqCtx := &aiReqCtx{
		DefaultAgentID:         req.Context.AgentID,
		Principal:              principal,
		SessionID:              run.SessionID,
		RunID:                  run.ID,
		AllowLowRiskWrites:     req.Context.AllowLowRiskWrites && sessionAllowsLowRisk(s.db, run.SessionID),
		KnowledgeCollectionIDs: req.KnowledgeCollectionIDs,
		DisableTools:           !profile.SupportsTools,
	}
	events := s.converse(profile.Model, sysPrompt, trimConversationHistory(req.Messages), ctx, reqCtx, limits)
	finalText := ""
	failed := false
	for event := range events {
		if event.Type == "reasoning" {
			emit(event.Type, event.Data, false)
			continue
		}
		if event.Type == "text" {
			finalText = event.Data
		}
		if event.Type == "usage" {
			var usage aiUsage
			if json.Unmarshal([]byte(event.Data), &usage) == nil {
				s.db.Model(&db.AIChatRun{}).Where("id = ?", run.ID).Updates(map[string]interface{}{
					"prompt_tokens":     gorm.Expr("prompt_tokens + ?", usage.PromptTokens),
					"completion_tokens": gorm.Expr("completion_tokens + ?", usage.CompletionTokens),
				})
			}
		}
		if event.Type == "error" {
			failed = true
		}
		emit(event.Type, event.Data, true)
	}
	completed := time.Now()
	status := aiRunStatusCompleted
	errorCode, errorMessage := "", ""
	if ctx.Err() != nil {
		status, errorCode, errorMessage = aiRunStatusCancelled, "cancelled", "Run cancelled by operator"
	} else if failed {
		status, errorCode, errorMessage = aiRunStatusFailed, "provider_error", "AI provider or tool execution failed"
	}
	var pendingIntents int64
	s.db.Model(&db.AIExecutionIntent{}).Where("run_id = ? AND status = ?", run.ID, "pending").Count(&pendingIntents)
	if pendingIntents > 0 && status == aiRunStatusCompleted {
		status = aiRunStatusWaitingApproval
	}
	if finalText != "" {
		message := db.AIChatMessage{SessionID: run.SessionID, RunID: run.ID, Role: "assistant", Content: finalText}
		if err := s.db.Create(&message).Error; err != nil {
			status, errorCode, errorMessage = aiRunStatusFailed, "storage_encryption_unavailable", "Encrypted AI storage is unavailable"
		}
	}
	s.db.Model(&db.AIChatRun{}).Where("id = ?", run.ID).Updates(map[string]interface{}{
		"status": status, "completed_at": completed, "error_code": errorCode, "error_message": errorMessage,
	})
	s.db.Model(&db.AIChatSession{}).Where("id = ?", run.SessionID).Update("updated_at", completed)
	if status != aiRunStatusCompleted {
		emit("error", errorMessage, true)
	}
}

func sessionAllowsLowRisk(database *gorm.DB, sessionID uint) bool {
	var policy string
	if err := database.Model(&db.AIChatSession{}).Where("id = ?", sessionID).Pluck("write_policy", &policy).Error; err != nil {
		return false
	}
	return policy == "low_risk_auto"
}

func (s *Server) findAuthorizedAIRun(c *gin.Context) (db.AIChatRun, aiPrincipal, bool) {
	principal, ok := s.currentAIPrincipal(c)
	if !ok {
		respondError(c, http.StatusForbidden, "AI use permission required")
		return db.AIChatRun{}, principal, false
	}
	var run db.AIChatRun
	query := s.db.Where("id = ? AND owner_id = ?", c.Param("id"), principal.UserID)
	if principal.TenantID != 0 {
		query = query.Where("tenant_id = ?", principal.TenantID)
	}
	if err := query.First(&run).Error; err != nil {
		respondError(c, http.StatusNotFound, "AI run not found")
		return db.AIChatRun{}, principal, false
	}
	return run, principal, true
}

func (s *Server) handleAIRunsGet(c *gin.Context) {
	run, _, ok := s.findAuthorizedAIRun(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": run})
}

func (s *Server) handleAIRunsList(c *gin.Context) {
	principal, ok := s.currentAIPrincipal(c)
	if !ok {
		respondError(c, http.StatusForbidden, "AI use permission required")
		return
	}
	query := s.db.Where("owner_id = ? AND tenant_id = ?", principal.UserID, principal.TenantID)
	if sessionID := strings.TrimSpace(c.Query("session_id")); sessionID != "" {
		query = query.Where("session_id = ?", sessionID)
	}
	if c.Query("status") == "active" {
		query = query.Where("status IN ?", aiActiveRunStatuses)
	}
	var runs []db.AIChatRun
	if err := query.Order("created_at DESC").Limit(100).Find(&runs).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to list AI runs")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": runs})
}

func aiRunTerminal(status string) bool {
	return status == aiRunStatusCompleted || status == aiRunStatusFailed || status == aiRunStatusCancelled || status == aiRunStatusInterrupted
}

func (s *Server) handleAIRunEvents(c *gin.Context) {
	run, _, ok := s.findAuthorizedAIRun(c)
	if !ok {
		return
	}
	after, _ := strconv.ParseInt(c.GetHeader("Last-Event-ID"), 10, 64)
	if queryAfter := c.Query("after"); queryAfter != "" {
		if parsed, err := strconv.ParseInt(queryAfter, 10, 64); err == nil && parsed > after {
			after = parsed
		}
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	clearHTTPWriteDeadline(c.Writer)
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return
	}
	if s.aiRuns == nil {
		s.aiRuns = newAIRunBroker()
	}
	live, unsubscribe := s.aiRuns.subscribe(run.ID)
	defer unsubscribe()
	send := func(event aiLiveEvent) bool {
		if event.Sequence <= after {
			return true
		}
		if _, err := fmt.Fprintf(c.Writer, "id: %d\nevent: %s\ndata: %s\n\n", event.Sequence, event.Type, strings.ReplaceAll(event.Payload, "\n", "\ndata: ")); err != nil {
			return false
		}
		flusher.Flush()
		after = event.Sequence
		return true
	}
	loadDurable := func() bool {
		for {
			var events []db.AIChatRunEvent
			if err := s.db.Where("run_id = ? AND sequence > ?", run.ID, after).Order("sequence asc").Limit(500).Find(&events).Error; err != nil {
				return false
			}
			for _, event := range events {
				if !send(aiLiveEvent{Sequence: event.Sequence, Type: event.Type, Payload: event.Payload}) {
					return false
				}
			}
			if len(events) < 500 {
				return true
			}
		}
	}
	if !loadDurable() {
		return
	}
	heartbeat := time.NewTicker(AIStreamHeartbeatInterval)
	defer heartbeat.Stop()
	for {
		var status string
		if err := s.db.Model(&db.AIChatRun{}).Where("id = ?", run.ID).Pluck("status", &status).Error; err != nil {
			slog.Warn("AI run status poll failed", "run_id", run.ID, "error", err)
			return
		}
		if aiRunTerminal(status) {
			if !loadDurable() {
				slog.Warn("AI run terminal replay missed events", "run_id", run.ID)
			}
			return
		}
		select {
		case <-c.Request.Context().Done():
			return
		case event, open := <-live:
			if !open || !send(event) {
				return
			}
		case <-heartbeat.C:
			if _, err := fmt.Fprintf(c.Writer, "event: ping\ndata: %d\n\n", time.Now().Unix()); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *Server) handleAIRunCancel(c *gin.Context) {
	run, principal, ok := s.findAuthorizedAIRun(c)
	if !ok {
		return
	}
	if aiRunTerminal(run.Status) {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": run})
		return
	}
	if s.aiRuns != nil {
		s.aiRuns.cancel(run.ID)
	}
	now := time.Now()
	s.db.Model(&db.AIChatRun{}).Where("id = ? AND status IN ?", run.ID, aiActiveRunStatuses).Updates(map[string]interface{}{
		"status": aiRunStatusCancelled, "completed_at": now, "error_code": "cancelled", "error_message": "Run cancelled by operator",
	})
	s.db.Model(&db.AIExecutionIntent{}).Where("run_id = ? AND status = ?", run.ID, "pending").Updates(map[string]interface{}{
		"status": "cancelled", "decided_by": principal.Username, "decided_at": now, "updated_at": now,
	})
	s.LogAuditRecord(c, "ai_run_cancel", "ai_run", run.ID, "Cancelled AI run", true, nil)
	_ = principal
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) initializeAIRuns() {
	if s.aiRuns == nil {
		s.aiRuns = newAIRunBroker()
	}
	now := time.Now()
	s.db.Model(&db.AIChatRun{}).Where("status IN ?", aiActiveRunStatuses).Updates(map[string]interface{}{
		"status": aiRunStatusInterrupted, "completed_at": now,
		"error_code": "server_restart", "error_message": "Server restarted while the run was active",
	})
	retention := aiRunEventRetentionDefault
	s.configMu.RLock()
	if s.cfg.AI.RunRetentionDays > 0 {
		retention = s.cfg.AI.RunRetentionDays
	}
	s.configMu.RUnlock()
	cutoff := now.AddDate(0, 0, -retention)
	s.db.Where("created_at < ?", cutoff).Delete(&db.AIChatRunEvent{})
	s.db.Model(&db.AIExecutionIntent{}).Where("status = ? AND created_at < ?", "pending", now.Add(-24*time.Hour)).Updates(map[string]interface{}{
		"status": "timed_out", "updated_at": now,
	})
	s.migrateLegacyAIStorage()
	s.initializeAIKnowledgeIndex()
	if s.ctx != nil {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			ticker := time.NewTicker(24 * time.Hour)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					s.db.Where("created_at < ?", time.Now().AddDate(0, 0, -retention)).Delete(&db.AIChatRunEvent{})
					s.db.Model(&db.AIExecutionIntent{}).Where("status = ? AND created_at < ?", "pending", time.Now().Add(-24*time.Hour)).Updates(map[string]interface{}{
						"status": "timed_out", "updated_at": time.Now(),
					})
				case <-s.ctx.Done():
					return
				}
			}
		}()
	}
}

func (s *Server) migrateLegacyAIStorage() {
	var sessions []db.AIChatSession
	if err := s.db.Session(&gorm.Session{SkipHooks: true}).Where("owner_id = 0 AND owner <> ''").Find(&sessions).Error; err == nil {
		for _, session := range sessions {
			var user db.User
			if s.db.Select("id", "tenant_id").Where("username = ?", session.Owner).First(&user).Error == nil {
				s.db.Model(&db.AIChatSession{}).Where("id = ? AND owner_id = 0", session.ID).Updates(map[string]interface{}{
					"owner_id": user.ID, "tenant_id": user.TenantID,
				})
			}
		}
	}
	var legacyMessages []db.AIChatMessage
	if err := s.db.Session(&gorm.Session{SkipHooks: true}).Where("content <> '' AND content NOT LIKE ?", "FC2ENC:%").Find(&legacyMessages).Error; err == nil {
		for i := range legacyMessages {
			if err := s.db.Save(&legacyMessages[i]).Error; err != nil {
				slog.Error("AI storage migration paused: encryption unavailable", "error", err, "message_id", legacyMessages[i].ID)
				break
			}
		}
	}
	var profileCount int64
	s.db.Model(&db.AIProviderProfile{}).Count(&profileCount)
	if profileCount == 0 && s.cfg != nil {
		s.configMu.RLock()
		enabled, apiKey := s.cfg.AI.Enabled, strings.TrimSpace(s.cfg.AI.APIKey)
		provider, model, endpoint := s.cfg.AI.Provider, s.cfg.AI.Model, s.cfg.AI.Endpoint
		s.configMu.RUnlock()
		if enabled && apiKey != "" {
			profile := db.AIProviderProfile{
				Name: "Default", Provider: provider, Model: model, Endpoint: endpoint, APIKey: apiKey,
				ContextLimit: 48000, OutputLimit: 4096, SupportsTools: true,
				SupportsReasoning: aiShouldRequestReasoning(provider, endpoint), Enabled: true, IsDefault: true, CreatedBy: "migration",
			}
			if err := s.db.Create(&profile).Error; err != nil {
				slog.Error("Failed to migrate legacy AI provider config", "error", err)
			}
		}
	}
}

// API keys may be supplied by environment even when the legacy config field is
// blank. Keep this helper near the run profile resolver to avoid exposing it.
func aiConfiguredAPIKey(configured string) string {
	if strings.TrimSpace(configured) != "" {
		return configured
	}
	return strings.TrimSpace(os.Getenv("FORGEC2_AI_API_KEY"))
}
