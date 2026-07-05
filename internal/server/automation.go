package server

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/scripting"
)

type AutomationRule struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Enabled     bool              `json:"enabled"`
	Priority    int               `json:"priority"` // higher = runs first
	EventType   string            `json:"event_type"`
	Conditions  []RuleCondition   `json:"conditions"`
	Actions     []RuleAction      `json:"actions"`
	Cooldown    int               `json:"cooldown"` // seconds between triggers
	LastTriggered time.Time       `json:"last_triggered"`
	RunCount    int               `json:"run_count"`
	CreatedAt   string            `json:"created_at"`
}

// Action types for automation rules
const (
	ActionRunCommand  = "command"
	ActionWebhook     = "webhook"
	ActionNotify     = "notify"
	ActionRunScript  = "script"
	ActionCreateTask = "create_task"
	ActionSetSleep   = "set_sleep"
)

type RuleCondition struct {
	Field    string `json:"field"`    // "agent.hostname", "data.*"
	Operator string `json:"operator"` // "contains", "equals", "regex"
	Value    string `json:"value"`
}

type RuleAction struct {
	Type   string          `json:"type"`   // "command", "webhook", "notify"
	Params json.RawMessage `json:"params"`
}

func (s *Server) evaluateRule(evt Event, rule AutomationRule) {
	for _, cond := range rule.Conditions {
		if !s.matchCondition(cond, evt) {
			return
		}
	}
	for _, action := range rule.Actions {
		s.executeAction(action, evt)
	}
}

func (s *Server) matchCondition(cond RuleCondition, evt Event) bool {
	var val string
	switch cond.Field {
	case "agent.hostname":
		val = evt.AgentHost
	default:
		if v, ok := evt.Data[cond.Field]; ok {
			val = fmt.Sprintf("%v", v)
		}
	}
	switch cond.Operator {
	case "contains":
		return strings.Contains(strings.ToLower(val), strings.ToLower(cond.Value))
	case "equals":
		return val == cond.Value
	default:
		return true
	}
}

func (s *Server) executeAction(action RuleAction, evt Event) {
	switch action.Type {
	case ActionRunCommand:
		var params struct {
			Command string `json:"command"`
			AgentID string `json:"agent_id"` // optional: target specific agent
		}
		if err := json.Unmarshal(action.Params, &params); err != nil {
			slog.Error("automation: unmarshal command params", "error", err)
			return
		}
		if params.Command != "" {
			targetAgent := evt.AgentID
			if params.AgentID != "" {
				targetAgent = params.AgentID
			}
			if targetAgent != "" {
				expanded := s.expandTemplate(params.Command, evt)
				if err := s.db.Create(&db.Task{
					AgentID:   targetAgent,
					Type:      "automation",
					Command:   expanded,
					Status:    "pending",
					CreatedBy: "automation",
				}).Error; err != nil {
					slog.Error("automation: failed to create command task", "error", err)
				}
			}
		}
	case ActionWebhook:
		var params struct {
			URL     string            `json:"url"`
			Method  string            `json:"method"`
			Headers map[string]string `json:"headers"`
			Secret  string            `json:"secret"` // HMAC secret
		}
		if err := json.Unmarshal(action.Params, &params); err != nil {
			slog.Error("automation: unmarshal webhook params", "error", err)
			return
		}
		if params.URL != "" {
			go s.executeWebhook(params, evt)
		}
	case ActionNotify:
		var params struct {
			Message string `json:"message"`
			Channel string `json:"channel"` // "all", "ops", "admins"
		}
		if err := json.Unmarshal(action.Params, &params); err == nil {
			msg := s.expandTemplate(params.Message, evt)
			s.broadcastToClients([]byte(fmt.Sprintf(`{"type":"notification","message":"%s","source":"automation"}`, msg)))
		}
	case ActionRunScript:
		var params struct {
			ScriptID string `json:"script_id"`
			Code     string `json:"code"`
		}
		if err := json.Unmarshal(action.Params, &params); err == nil {
			engine := scripting.GetEngine()
			context := map[string]interface{}{
				"event":    evt,
				"agent_id": evt.AgentID,
			}
			if params.ScriptID != "" {
				engine.Execute(params.ScriptID, context)
			} else if params.Code != "" {
				engine.ExecuteCode(params.Code, context)
			}
		}
	case ActionCreateTask:
		var params struct {
			AgentID string `json:"agent_id"`
			Type    string `json:"type"`
			Command string `json:"command"`
		}
		if err := json.Unmarshal(action.Params, &params); err == nil {
			targetAgent := evt.AgentID
			if params.AgentID != "" {
				targetAgent = params.AgentID
			}
			if targetAgent != "" && params.Type != "" {
				expanded := s.expandTemplate(params.Command, evt)
				if err := s.db.Create(&db.Task{
					AgentID:   targetAgent,
					Type:      params.Type,
					Command:   expanded,
					Status:    "pending",
					CreatedBy: "automation",
				}).Error; err != nil {
					slog.Error("automation: failed to create task", "error", err)
				}
			}
		}
	case ActionSetSleep:
		var params struct {
			AgentID  string `json:"agent_id"`
			Interval int    `json:"interval"`
			Jitter   int    `json:"jitter"`
		}
		if err := json.Unmarshal(action.Params, &params); err == nil {
			targetAgent := evt.AgentID
			if params.AgentID != "" {
				targetAgent = params.AgentID
			}
			if targetAgent != "" && params.Interval > 0 {
				cmd := fmt.Sprintf("%d,%d", params.Interval, params.Jitter)
				if err := s.db.Create(&db.Task{
					AgentID:   targetAgent,
					Type:      "set_sleep",
					Command:   cmd,
					Status:    "pending",
					CreatedBy: "automation",
				}).Error; err != nil {
					slog.Error("automation: failed to create set_sleep task", "error", err)
				}
			}
		}
	}
}

// expandTemplate replaces template variables in a string with event data.
func (s *Server) expandTemplate(input string, evt Event) string {
	result := input
	result = strings.ReplaceAll(result, "{{agent_id}}", evt.AgentID)
	result = strings.ReplaceAll(result, "{{hostname}}", evt.AgentHost)
	result = strings.ReplaceAll(result, "{{ip}}", evt.AgentIP)
	result = strings.ReplaceAll(result, "{{os}}", evt.AgentOS)
	result = strings.ReplaceAll(result, "{{user}}", evt.User)
	result = strings.ReplaceAll(result, "{{event_type}}", string(evt.Type))
	return result
}

// executeWebhook sends a webhook request with optional HMAC signature.
func (s *Server) executeWebhook(params struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Secret  string            `json:"secret"`
}, evt Event) {
	body, err := json.Marshal(evt)
	if err != nil {
		slog.Error("automation: marshal webhook body", "error", err)
		return
	}
	method := params.Method
	if method == "" {
		method = "POST"
	}
	req, err := http.NewRequest(method, params.URL, bytes.NewReader(body))
	if err != nil {
		slog.Error("automation: create webhook request", "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "ForgeC2-Automation/1.0")

	// Add custom headers
	for k, v := range params.Headers {
		req.Header.Set(k, v)
	}

	// Add HMAC signature if secret is configured
	if params.Secret != "" {
		h := hmac.New(sha256.New, []byte(params.Secret))
		h.Write(body)
		req.Header.Set("X-ForgeC2-Signature", hex.EncodeToString(h.Sum(nil)))
	}

	client := &http.Client{Timeout: 10 * time.Second}
	if resp, err := client.Do(req); err != nil {
		slog.Error("automation: webhook request failed", "url", params.URL, "error", err)
	} else {
		resp.Body.Close()
	}
}

func (s *Server) loadAutomationRules() []AutomationRule {
	var dbRules []db.AutomationRule
	if err := s.db.Limit(200).Find(&dbRules).Error; err != nil {
		slog.Error("automation: failed to load rules", "error", err)
	}
	
	var rules []AutomationRule
	for _, dr := range dbRules {
		var conditions []RuleCondition
		if dr.Conditions != "" {
			if err := json.Unmarshal([]byte(dr.Conditions), &conditions); err != nil {
			slog.Warn("automation: unmarshal conditions", "rule", dr.Name, "error", err)
		}
		}
		var actions []RuleAction
		if dr.Actions != "" {
			if err := json.Unmarshal([]byte(dr.Actions), &actions); err != nil {
			slog.Warn("automation: unmarshal actions", "rule", dr.Name, "error", err)
		}
		}
		rules = append(rules, AutomationRule{
			ID:         dr.ID,
			Name:       dr.Name,
			Enabled:    dr.Enabled,
			EventType:  dr.EventType,
			Conditions: conditions,
			Actions:    actions,
			CreatedAt:  dr.CreatedAt.Format(time.RFC3339),
		})
	}
	return rules
}

func (s *Server) saveAutomationRule(rule AutomationRule) error {
	conditionsData, err := json.Marshal(rule.Conditions)
	if err != nil {
		return fmt.Errorf("marshal conditions: %w", err)
	}
	actionsData, err := json.Marshal(rule.Actions)
	if err != nil {
		return fmt.Errorf("marshal actions: %w", err)
	}
	
	dbRule := db.AutomationRule{
		ID:         rule.ID,
		Name:       rule.Name,
		Enabled:    rule.Enabled,
		EventType:  rule.EventType,
		Conditions: string(conditionsData),
		Actions:    string(actionsData),
	}
	
	if rule.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, rule.CreatedAt); err == nil {
			dbRule.CreatedAt = t
		}
	}
	
	return s.db.Save(&dbRule).Error
}

func (s *Server) deleteAutomationRule(id string) error {
	return s.db.Delete(&db.AutomationRule{}, "id = ?", id).Error
}

func (s *Server) migrateAutomationRules() {
	var count int64
	if err := s.db.Model(&db.AutomationRule{}).Count(&count).Error; err != nil {
		slog.Error("automation: failed to count rules", "error", err)
	}
	if count > 0 {
		return
	}
	
	raw := s.getConfigJSON("automation_rules")
	if raw == "" {
		return
	}
	
	var rules []AutomationRule
	if err := json.Unmarshal([]byte(raw), &rules); err != nil {
		return
	}
	
	for _, rule := range rules {
		if err := s.saveAutomationRule(rule); err != nil {
			slog.Warn("automation: failed to import rule", "id", rule.ID, "error", err)
		}
	}

	if err := s.db.Where("key = ?", "automation_rules").Delete(&db.ServerConfig{}).Error; err != nil {
		slog.Error("automation: failed to delete legacy config", "error", err)
	}
}

func (s *Server) getConfigJSON(key string) string {
	var cfg struct{ Value string }
	if err := s.db.Model(&db.ServerConfig{}).Where("key = ?", key).First(&cfg).Error; err != nil {
		slog.Debug("config key not found", "key", key, "error", err)
	}
	return cfg.Value
}

func (s *Server) setConfigJSON(key, value string) {
	if err := s.db.Model(&db.ServerConfig{}).Where("key = ?", key).Assign(db.ServerConfig{Value: value}).FirstOrCreate(&db.ServerConfig{Key: key}).Error; err != nil {
		slog.Error("failed to set config", "key", key, "error", err)
	}
}

func (s *Server) registerBuiltinAutomations() {
	rule := AutomationRule{
		ID:        "auto_dc_alert",
		Name:      "DC Login Alert",
		Enabled:   true,
		EventType: string(EventImplantCheckin),
		Conditions: []RuleCondition{
			{Field: "agent.hostname", Operator: "contains", Value: "DC"},
		},
		Actions: []RuleAction{
			{
				Type: "command",
				Params: json.RawMessage(`{"command": "ldap_users"}`),
			},
			{
				Type: "webhook",
				Params: json.RawMessage(`{"url": "", "method": "POST"}`),
			},
		},
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	rules := s.loadAutomationRules()
	exists := false
	for _, r := range rules {
		if r.ID == rule.ID {
			exists = true
			break
		}
	}
	if !exists {
		if err := s.saveAutomationRule(rule); err != nil {
			slog.Warn("automation: failed to register builtin rule", "id", rule.ID, "error", err)
		}
	}
}
