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

type WebhookEntry struct {
	db.WebhookConfig
}

func (s *Server) triggerWebhooks(evt Event) {
	var webhooks []db.WebhookConfig
	s.db.Where("event_type = ? AND enabled = ?", string(evt.Type), true).Find(&webhooks)

	for _, wh := range webhooks {
		go s.fireWebhook(wh, evt)
	}
}

func (s *Server) fireWebhook(wh db.WebhookConfig, evt Event) {
	payload, _ := json.Marshal(map[string]interface{}{
		"event":     evt.Type,
		"agent_id":  evt.AgentID,
		"hostname":  evt.AgentHost,
		"timestamp": evt.Timestamp,
		"data":      evt.Data,
	})

	req, err := http.NewRequest(wh.Method, wh.URL, bytes.NewReader(payload))
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

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("webhook delivery failed", "name", wh.Name, "error", err)
		return
	}
	resp.Body.Close()

	_ = s.db.Create(&db.AuditLog{
		User:    "system",
		Action:  "webhook",
		Success: resp.StatusCode >= 200 && resp.StatusCode < 300,
		Details: fmt.Sprintf("Webhook %s -> %s: %d", wh.Name, wh.URL, resp.StatusCode),
	})
}
