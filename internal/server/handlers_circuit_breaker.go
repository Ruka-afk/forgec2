package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleAPICircuitBreakerDetail(c *gin.Context) {
	cb := GetCircuitBreaker()
	if cb == nil {
		respond(c, gin.H{"listeners": []interface{}{}})
		return
	}

	type listenerDetail struct {
		Target           string   `json:"target"`
		Scheme           string   `json:"scheme"`
		Host             string   `json:"host"`
		Port             int      `json:"port"`
		Status           string   `json:"status"`
		ConsecutiveFails int      `json:"consecutive_fails"`
		LastProbe        string   `json:"last_probe"`
		FailReasons      []string `json:"fail_reasons"`
	}

	cb.mu.RLock()
	var listeners []listenerDetail
	for id, th := range cb.targets {
		statusStr := "unknown"
		switch th.Status {
		case HealthHealthy:
			statusStr = "healthy"
		case HealthUnstable:
			statusStr = "unstable"
		case HealthBurned:
			statusStr = "burned"
		}
		listeners = append(listeners, listenerDetail{
			Target:           id,
			Scheme:           th.Target.Scheme,
			Host:             th.Target.Host,
			Port:             th.Target.Port,
			Status:           statusStr,
			ConsecutiveFails: th.ConsecutiveFails,
			LastProbe:        th.LastProbe.Format(time.RFC3339),
			FailReasons:      th.FailReasons,
		})
	}
	cb.mu.RUnlock()

	respond(c, gin.H{"listeners": listeners})
}

func (s *Server) handleAPICircuitBreakerConfig(c *gin.Context) {
	var cfg db.CircuitBreakerConfig
	if err := s.db.FirstOrCreate(&cfg, db.CircuitBreakerConfig{ID: 1}).Error; err != nil {
		respondError(c, http.StatusInternalServerError, "failed to load config")
		return
	}
	respond(c, gin.H{
		"failure_threshold":    cfg.FailureThreshold,
		"cooldown_seconds":     cfg.CooldownSeconds,
		"half_open_max_reqs":   cfg.HalfOpenMaxReqs,
		"health_check_seconds": cfg.HealthCheckSeconds,
	})
}

func (s *Server) handleAPICircuitBreakerEvents(c *gin.Context) {
	var events []db.CircuitBreakerEvent
	s.db.Order("created_at DESC").Limit(100).Find(&events)
	respond(c, gin.H{"events": events})
}

func (s *Server) handleAPICircuitBreakerSaveConfig(c *gin.Context) {
	var req struct {
		FailureThreshold   int `json:"failure_threshold"`
		CooldownSeconds    int `json:"cooldown_seconds"`
		HalfOpenMaxReqs    int `json:"half_open_max_reqs"`
		HealthCheckSeconds int `json:"health_check_seconds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}
	cfg := db.CircuitBreakerConfig{ID: 1}
	s.db.FirstOrCreate(&cfg, db.CircuitBreakerConfig{ID: 1})
	s.db.Model(&cfg).Updates(map[string]interface{}{
		"failure_threshold":    req.FailureThreshold,
		"cooldown_seconds":     req.CooldownSeconds,
		"half_open_max_reqs":   req.HalfOpenMaxReqs,
		"health_check_seconds": req.HealthCheckSeconds,
	})
	respond(c, gin.H{"success": true})
}

func (s *Server) handleAPICircuitBreakerReset(c *gin.Context) {
	listenerID := c.Param("id")
	cb := GetCircuitBreaker()
	if cb == nil {
		respondError(c, http.StatusServiceUnavailable, "circuit breaker not initialized")
		return
	}

	cb.mu.Lock()
	if th, ok := cb.targets[listenerID]; ok {
		th.ConsecutiveFails = 0
		th.FailReasons = nil
		th.Status = HealthUnknown
	} else {
		cb.mu.Unlock()
		respondError(c, http.StatusNotFound, "listener not found")
		return
	}
	cb.mu.Unlock()

	if err := s.db.Create(&db.CircuitBreakerEvent{
		ListenerID: listenerID,
		OldState:   "burned",
		NewState:   "unknown",
		Reason:     "manual reset by operator",
		CreatedAt:  time.Now(),
	}).Error; err != nil {
		slog.Error("Failed to record circuit breaker event", "err", err)
	}
	s.LogAuditRecord(c, "circuit_breaker_reset", "listener", listenerID, "Circuit breaker reset for listener", true, nil)
	respond(c, gin.H{"success": true})
}

func (s *Server) handleAPICircuitBreakerToggle(c *gin.Context) {
	listenerID := c.Param("id")
	var req struct {
		State string `json:"state"` // "enabled" or "disabled"
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}

	cb := GetCircuitBreaker()
	if cb == nil {
		respondError(c, http.StatusServiceUnavailable, "circuit breaker not initialized")
		return
	}

	cb.mu.Lock()
	th, ok := cb.targets[listenerID]
	if !ok {
		cb.mu.Unlock()
		respondError(c, http.StatusNotFound, "listener not found")
		return
	}

	var oldState string
	switch th.Status {
	case HealthHealthy:
		oldState = "healthy"
	case HealthUnstable:
		oldState = "unstable"
	case HealthBurned:
		oldState = "burned"
	default:
		oldState = "unknown"
	}

	if req.State == "disabled" {
		th.Status = HealthBurned
	} else {
		th.ConsecutiveFails = 0
		th.FailReasons = nil
		th.Status = HealthHealthy
	}
	cb.mu.Unlock()

	newState := "healthy"
	if req.State == "disabled" {
		newState = "burned"
	}

	if err := s.db.Create(&db.CircuitBreakerEvent{
		ListenerID: listenerID,
		OldState:   oldState,
		NewState:   newState,
		Reason:     fmt.Sprintf("manual toggle to %s by operator", req.State),
		CreatedAt:  time.Now(),
	}).Error; err != nil {
		slog.Error("Failed to record circuit breaker event", "err", err)
	}
	respond(c, gin.H{"success": true})
}
