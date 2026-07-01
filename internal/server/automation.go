package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/forgec2/forgec2/internal/db"
)

type AutomationRule struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Enabled     bool              `json:"enabled"`
	EventType   string            `json:"event_type"`
	Conditions  []RuleCondition   `json:"conditions"`
	Actions     []RuleAction      `json:"actions"`
	CreatedAt   string            `json:"created_at"`
}

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
	case "command":
		var params struct {
			Command string `json:"command"`
		}
		json.Unmarshal(action.Params, &params)
		if params.Command != "" && evt.AgentID != "" {
			expanded := strings.ReplaceAll(params.Command, "{{agent_id}}", evt.AgentID)
			expanded = strings.ReplaceAll(expanded, "{{hostname}}", evt.AgentHost)
			s.db.Create(&db.Task{
				AgentID:   evt.AgentID,
				Type:      "automation",
				Command:   expanded,
				Status:    "pending",
				CreatedBy: "automation",
			})
		}
	case "webhook":
		var params struct {
			URL    string `json:"url"`
			Method string `json:"method"`
		}
		json.Unmarshal(action.Params, &params)
		if params.URL != "" {
			go func() {
				body, _ := json.Marshal(evt)
				method := params.Method
				if method == "" {
					method = "POST"
				}
				req, _ := http.NewRequest(method, params.URL, bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("User-Agent", "ForgeC2-Automation/1.0")
				client := &http.Client{Timeout: 10 * time.Second}
				client.Do(req)
			}()
		}
	}
}

func (s *Server) loadAutomationRules() []AutomationRule {
	var dbRules []db.AutomationRule
	s.db.Find(&dbRules)
	
	var rules []AutomationRule
	for _, dr := range dbRules {
		var conditions []RuleCondition
		if dr.Conditions != "" {
			json.Unmarshal([]byte(dr.Conditions), &conditions)
		}
		var actions []RuleAction
		if dr.Actions != "" {
			json.Unmarshal([]byte(dr.Actions), &actions)
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
	conditionsData, _ := json.Marshal(rule.Conditions)
	actionsData, _ := json.Marshal(rule.Actions)
	
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
	s.db.Model(&db.AutomationRule{}).Count(&count)
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
		s.saveAutomationRule(rule)
	}
	
	s.db.Where("key = ?", "automation_rules").Delete(&db.ServerConfig{})
}

func (s *Server) getConfigJSON(key string) string {
	var cfg struct{ Value string }
	s.db.Model(&db.ServerConfig{}).Where("key = ?", key).Find(&cfg)
	return cfg.Value
}

func (s *Server) setConfigJSON(key, value string) {
	s.db.Model(&db.ServerConfig{}).Where("key = ?", key).Assign(db.ServerConfig{Value: value}).FirstOrCreate(&db.ServerConfig{Key: key})
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
		s.saveAutomationRule(rule)
	}
}
