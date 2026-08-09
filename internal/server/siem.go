package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

type SIEMEvent struct {
	Timestamp  time.Time              `json:"timestamp"`
	Action     string                 `json:"action"`
	Resource   string                 `json:"resource"`
	AgentID    string                 `json:"agent_id,omitempty"`
	User       string                 `json:"user"`
	IP         string                 `json:"ip"`
	Success    bool                   `json:"success"`
	Error      string                 `json:"error,omitempty"`
	Details    string                 `json:"details,omitempty"`
	Hostname   string                 `json:"hostname,omitempty"`
}

type SIEMWebhook struct {
	URL        string
	Token      string
	Enabled    bool
	client     *http.Client
	actions    map[string]bool
	correlator *EventCorrelator
	batch      []SIEMEvent
	batchMu    sync.Mutex
	batchTicker *time.Ticker
	stopCh     chan struct{}
}

func NewSIEMWebhook(url, token, actions string) *SIEMWebhook {
	if url == "" {
		return nil
	}
	sw := &SIEMWebhook{
		URL:         url,
		Token:       token,
		Enabled:     true,
		client:      &http.Client{Timeout: 10 * time.Second},
		correlator:  NewEventCorrelator(),
		batch:       make([]SIEMEvent, 0, 100),
		stopCh:      make(chan struct{}),
		batchTicker: time.NewTicker(10 * time.Second),
	}
	if actions != "" {
		sw.actions = make(map[string]bool)
		for _, a := range strings.Split(actions, ",") {
			sw.actions[strings.TrimSpace(a)] = true
		}
	}
	go sw.flushLoop()
	return sw
}

func (sw *SIEMWebhook) Send(event SIEMEvent) {
	if !sw.Enabled || sw.URL == "" {
		return
	}
	if sw.actions != nil && !sw.actions[event.Action] {
		return
	}
	alerts := sw.correlator.ProcessEvent(event)
	sw.batchMu.Lock()
	sw.batch = append(sw.batch, event)
	for _, a := range alerts {
		sw.batch = append(sw.batch, a)
	}
	sw.batchMu.Unlock()
}

func (sw *SIEMWebhook) flushLoop() {
	for {
		select {
		case <-sw.batchTicker.C:
			sw.flush()
		case <-sw.stopCh:
			sw.flush()
			return
		}
	}
}

func (sw *SIEMWebhook) flush() {
	sw.batchMu.Lock()
	if len(sw.batch) == 0 {
		sw.batchMu.Unlock()
		return
	}
	events := make([]SIEMEvent, len(sw.batch))
	copy(events, sw.batch)
	sw.batch = sw.batch[:0]
	sw.batchMu.Unlock()

	data, err := json.Marshal(events)
	if err != nil {
		slog.Error("SIEM: failed to marshal events", "error", err)
		return
	}
	sw.sendWithRetry(data)
}

func (sw *SIEMWebhook) sendWithRetry(data []byte) {
	backoff := time.Second
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequest("POST", sw.URL, bytes.NewReader(data))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		if sw.Token != "" {
			req.Header.Set("Authorization", "Bearer "+sw.Token)
		}
		resp, err := sw.client.Do(req)
		if err != nil {
			slog.Warn("SIEM webhook delivery failed", "attempt", attempt+1, "err", err)
			time.Sleep(backoff)
			backoff *= 2
			continue
		}
		resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return
		}
		slog.Warn("SIEM webhook returned non-2xx", "status", resp.StatusCode, "attempt", attempt+1)
		time.Sleep(backoff)
		backoff *= 2
	}
}

func (sw *SIEMWebhook) Stop() {
	close(sw.stopCh)
	sw.batchTicker.Stop()
}

type CorrelationRule struct {
	Name        string
	Action      string
	Window      time.Duration
	Threshold   int
	AlertAction string
	AlertDetails string
}

type eventWindow struct {
	events []SIEMEvent
	Latest time.Time
}

type EventCorrelator struct {
	mu        sync.Mutex
	rules     []CorrelationRule
	windows   map[string]*eventWindow
	dedup     map[string]time.Time
	maxWindow time.Duration
}

func NewEventCorrelator() *EventCorrelator {
	rules := []CorrelationRule{
		{
			Name:         "multi_source_login_failure",
			Action:       "login_failed",
			Window:       5 * time.Minute,
			Threshold:    5,
			AlertAction:  "siem_alert",
			AlertDetails: "Multiple login failures from different sources within 5 minutes",
		},
		{
			Name:         "agent_beacon_anomaly",
			Action:       "implant_checkin",
			Window:       10 * time.Minute,
			Threshold:    10,
			AlertAction:  "siem_alert",
			AlertDetails: "Agent beacon frequency anomaly detected",
		},
		{
			Name:         "credential_dump_spike",
			Action:       "credential_found",
			Window:       1 * time.Minute,
			Threshold:    3,
			AlertAction:  "siem_alert",
			AlertDetails: "Multiple credentials found in short window",
		},
		{
			Name:         "task_cancel_spike",
			Action:       "task_cancelled",
			Window:       2 * time.Minute,
			Threshold:    5,
			AlertAction:  "siem_alert",
			AlertDetails: "Multiple task cancellations in short window",
		},
		{
			Name:         "privilege_escalation",
			Action:       "agent_elevated",
			Window:       1 * time.Second,
			Threshold:    1,
			AlertAction:  "siem_alert",
			AlertDetails: "Agent privilege escalation detected",
		},
	}
	var maxWindow time.Duration
	for _, r := range rules {
		if r.Window > maxWindow {
			maxWindow = r.Window
		}
	}
	return &EventCorrelator{
		rules:     rules,
		windows:   make(map[string]*eventWindow),
		dedup:     make(map[string]time.Time),
		maxWindow: maxWindow,
	}
}

func (ec *EventCorrelator) cleanup() {
	now := time.Now()
	for key, w := range ec.windows {
		if now.Sub(w.Latest) > ec.maxWindow && ec.maxWindow > 0 {
			delete(ec.windows, key)
		}
	}
}

func (ec *EventCorrelator) ProcessEvent(event SIEMEvent) []SIEMEvent {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	ec.cleanup()

	var alerts []SIEMEvent
	for _, rule := range ec.rules {
		if rule.Action != event.Action {
			continue
		}
		key := rule.Name + ":" + event.AgentID
		w, ok := ec.windows[key]
		if !ok {
			w = &eventWindow{}
			ec.windows[key] = w
		}
		now := time.Now()
		cutoff := now.Add(-rule.Window)
		filtered := w.events[:0]
		for _, e := range w.events {
			if e.Timestamp.After(cutoff) {
				filtered = append(filtered, e)
			}
		}
		w.events = filtered
		w.events = append(w.events, event)
		w.Latest = now

		if len(w.events) >= rule.Threshold {
			dedupKey := rule.Name + ":" + event.AgentID
			if last, ok := ec.dedup[dedupKey]; ok && now.Sub(last) < rule.Window {
				continue
			}
			ec.dedup[dedupKey] = now
			alerts = append(alerts, SIEMEvent{
				Timestamp: now,
				Action:    rule.AlertAction,
				Resource:  "siem_correlation",
				AgentID:   event.AgentID,
				User:      event.User,
				IP:        event.IP,
				Success:   true,
				Details:   rule.AlertDetails,
				Hostname:  event.Hostname,
			})
		}
	}
	return alerts
}
