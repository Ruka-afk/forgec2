package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/forgec2/forgec2/internal/db"
)

const webhookMaxRetries = 3

func (s *Server) triggerWebhooks(evt Event) {
	var webhooks []db.WebhookConfig
	s.db.Where("event_type = ? AND enabled = ?", string(evt.Type), true).Limit(200).Find(&webhooks)

	for _, wh := range webhooks {
		go s.fireWebhook(wh, evt)
	}
}

func (s *Server) fireWebhook(wh db.WebhookConfig, evt Event) {
	if err := validateWebhookURL(wh.URL); err != nil {
		slog.Error("webhook URL rejected", "name", wh.Name, "url", wh.URL, "error", err)
		return
	}
	payload, err := json.Marshal(map[string]interface{}{
		"event":     evt.Type,
		"agent_id":  evt.AgentID,
		"hostname":  evt.AgentHost,
		"timestamp": evt.Timestamp,
		"data":      evt.Data,
	})
	if err != nil {
		slog.Error("webhook marshal payload failed", "error", err)
		return
	}

	backoff := []time.Duration{time.Second, 3 * time.Second, 7 * time.Second}
	for attempt := 0; attempt <= webhookMaxRetries; attempt++ {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		req, err := http.NewRequestWithContext(s.ctx, wh.Method, wh.URL, bytes.NewReader(payload))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "ForgeC2-Webhook/1.0")

		if wh.Headers != "" {
			var hdr map[string]string
			if json.Unmarshal([]byte(wh.Headers), &hdr) == nil {
				for k, v := range hdr {
					req.Header.Set(k, v)
				}
			}
		}

		resp, err := s.httpClient.Do(req)
		if err != nil {
			if attempt < webhookMaxRetries {
				slog.Warn("webhook delivery failed, retrying", "name", wh.Name, "attempt", attempt+1, "error", err)
				select {
				case <-s.ctx.Done():
					return
				case <-time.After(backoff[attempt]):
				}
				continue
			}
			slog.Error("webhook delivery failed, exhausted retries", "name", wh.Name, "error", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			s.db.Create(&db.AuditLog{
				User:    "system",
				Action:  "webhook",
				Success: true,
				Details: fmt.Sprintf("Webhook %s -> %s: %d", wh.Name, wh.URL, resp.StatusCode),
			})
			return
		}
		if attempt < webhookMaxRetries && resp.StatusCode >= 500 {
			slog.Warn("webhook delivery got server error, retrying", "name", wh.Name, "status", resp.StatusCode, "attempt", attempt+1)
			select {
			case <-s.ctx.Done():
				return
			case <-time.After(backoff[attempt]):
			}
			continue
		}

		slog.Error("webhook delivery failed with non-retryable status", "name", wh.Name, "status", resp.StatusCode)
		s.db.Create(&db.AuditLog{
			User:    "system",
			Action:  "webhook",
			Success: false,
			Details: fmt.Sprintf("Webhook %s -> %s: %d", wh.Name, wh.URL, resp.StatusCode),
		})
		return
	}
}
