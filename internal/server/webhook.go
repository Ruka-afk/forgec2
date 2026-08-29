package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"gorm.io/gorm"
)

const webhookMaxRetries = 3

func (s *Server) triggerWebhooks(evt Event) {
	var webhooks []db.WebhookConfig
	if err := s.db.Where("event_type = ? AND enabled = ?", string(evt.Type), true).Limit(200).Find(&webhooks).Error; err != nil {
		slog.Error("Failed to query webhooks", "err", err)
	}

	for _, wh := range webhooks {
		go s.fireWebhook(wh, evt)
	}
}

func (s *Server) fireWebhook(wh db.WebhookConfig, evt Event) error {
	if err := validateWebhookURL(wh.URL); err != nil {
		// No URL in the log: webhook targets may embed secret tokens.
		slog.Error("Webhook URL rejected", "name", wh.Name, "error", err)
		return fmt.Errorf("webhook URL rejected: %w", err)
	}
	payload, err := json.Marshal(map[string]interface{}{
		"event":     evt.Type,
		"agent_id":  evt.AgentID,
		"hostname":  evt.AgentHost,
		"timestamp": evt.Timestamp,
		"data":      evt.Data,
	})
	if err != nil {
		slog.Error("Webhook marshal payload failed", "error", err)
		return fmt.Errorf("webhook payload marshal failed: %w", err)
	}

	backoff := []time.Duration{time.Second, 3 * time.Second, 7 * time.Second}
	for attempt := 0; attempt <= webhookMaxRetries; attempt++ {
		select {
		case <-s.ctx.Done():
			return fmt.Errorf("webhook delivery aborted: server shutting down")
		default:
		}

		req, err := http.NewRequestWithContext(s.ctx, wh.Method, wh.URL, bytes.NewReader(payload))
		if err != nil {
			return fmt.Errorf("webhook request build failed: %w", err)
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
				slog.Warn("Webhook delivery failed, retrying", "name", wh.Name, "attempt", attempt+1, "error", err)
				select {
				case <-s.ctx.Done():
					return fmt.Errorf("webhook delivery aborted: server shutting down")
				case <-time.After(backoff[attempt]):
				}
				continue
			}
			slog.Error("Webhook delivery failed, exhausted retries", "name", wh.Name, "error", err)
			return err
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			s.flushAuditEntries([]db.AuditLog{{
				User:    "system",
				Action:  "webhook",
				Success: true,
				Details: fmt.Sprintf("Webhook %s -> %s: %d", wh.Name, wh.URL, resp.StatusCode),
			}})
			// Record the real delivery so the integrations list can show
			// truthful event_count/last_trigger instead of fabricated zeroes.
			now := time.Now()
			if err := s.db.Model(&db.WebhookConfig{}).
				Where("id = ?", wh.ID).
				Updates(map[string]interface{}{
					"event_count":  gorm.Expr("event_count + 1"),
					"last_trigger": now,
				}).Error; err != nil {
				slog.Error("Failed to update webhook delivery stats", "name", wh.Name, "error", err)
			}
			return nil
		}
		if attempt < webhookMaxRetries && resp.StatusCode >= 500 {
			slog.Warn("Webhook delivery got server error, retrying", "name", wh.Name, "status", resp.StatusCode, "attempt", attempt+1)
			select {
			case <-s.ctx.Done():
				return fmt.Errorf("webhook delivery aborted: server shutting down")
			case <-time.After(backoff[attempt]):
			}
			continue
		}

		slog.Error("Webhook delivery failed with non-retryable status", "name", wh.Name, "status", resp.StatusCode)
		s.flushAuditEntries([]db.AuditLog{{
			User:    "system",
			Action:  "webhook",
			Success: false,
			Details: fmt.Sprintf("Webhook %s -> %s: %d", wh.Name, wh.URL, resp.StatusCode),
		}})
		return fmt.Errorf("webhook delivery failed with status %d", resp.StatusCode)
	}
	return fmt.Errorf("webhook delivery failed")
}
