package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type DiscordExternalC2 struct {
	server    *Server
	botToken  string
	channelID string
	mu        sync.Mutex
	running   bool
	stopCh    chan struct{}
	sequence  int
	sessionID string
}

func NewDiscordExternalC2(s *Server, botToken, channelID string) *DiscordExternalC2 {
	return &DiscordExternalC2{
		server:    s,
		botToken:  botToken,
		channelID: channelID,
		stopCh:    make(chan struct{}),
	}
}

func (d *DiscordExternalC2) Start() error {
	d.mu.Lock()
	if d.running {
		d.mu.Unlock()
		return nil
	}
	d.running = true
	d.stopCh = make(chan struct{})
	d.mu.Unlock()

	channelID := "extc2-discord-" + d.channelID
	slog.Info("Discord External C2 starting", "channel_id", d.channelID)

	d.server.extC2ChannelsMu.Lock()
	d.server.extC2Channels[channelID] = &extC2WSChannel{
		ID:        channelID,
		Type:      "discord",
		CreatedAt: time.Now(),
	}
	d.server.extC2ChannelsMu.Unlock()

	go d.runLoop(channelID)
	return nil
}

func (d *DiscordExternalC2) runLoop(channelID string) {
	for {
		d.connectAndRun(channelID)
		select {
		case <-d.stopCh:
			slog.Info("Discord External C2 stopped", "channel_id", d.channelID)
			return
		default:
			slog.Info("Discord External C2 reconnecting in 5s", "channel_id", d.channelID)
			time.Sleep(5 * time.Second)
		}
	}
}

func (d *DiscordExternalC2) connectAndRun(channelID string) {
	gatewayURL := "wss://gateway.discord.gg/?v=10&encoding=json"
	header := http.Header{}
	header.Set("Bot", d.botToken)

	conn, _, err := websocket.DefaultDialer.Dial(gatewayURL, header)
	if err != nil {
		slog.Error("Discord gateway connect failed", "channel_id", d.channelID, "error", err)
		return
	}
	defer conn.Close()

	_, msg, err := conn.ReadMessage()
	if err != nil {
		slog.Error("Discord gateway read Hello failed", "error", err)
		return
	}

	var hello struct {
		Op int `json:"op"`
		D  struct {
			HeartbeatInterval int `json:"heartbeat_interval"`
		} `json:"d"`
	}
	if err := json.Unmarshal(msg, &hello); err != nil {
		slog.Error("Discord gateway Hello parse failed", "error", err)
		return
	}

	heartbeatInterval := time.Duration(hello.D.HeartbeatInterval) * time.Millisecond
	if heartbeatInterval < 1000*time.Millisecond {
		heartbeatInterval = 41250 * time.Millisecond
	}

	heartbeatDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-d.stopCh:
				return
			case <-heartbeatDone:
				return
			case <-ticker.C:
				hb, _ := json.Marshal(map[string]interface{}{
					"op": 1,
					"d":  d.sequence,
				})
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteMessage(websocket.TextMessage, hb); err != nil {
					slog.Error("Discord heartbeat send failed", "error", err)
					return
				}
			}
		}
	}()

	identify := map[string]interface{}{
		"op": 2,
		"d": map[string]interface{}{
			"token":   d.botToken,
			"intents": 513,
			"properties": map[string]interface{}{
				"os":      "windows",
				"browser": "forgec2",
				"device":  "forgec2",
			},
		},
	}
	identifyJSON, _ := json.Marshal(identify)
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := conn.WriteMessage(websocket.TextMessage, identifyJSON); err != nil {
		slog.Error("Discord IDENTIFY send failed", "error", err)
		close(heartbeatDone)
		return
	}

	taskSenderDone := make(chan struct{})

	go func() {
		defer close(taskSenderDone)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-d.stopCh:
				return
			case <-ticker.C:
				d.server.extC2TaskMu.Lock()
				tasks := d.server.extC2TaskQueue[channelID]
				d.server.extC2TaskQueue[channelID] = nil
				d.server.extC2TaskMu.Unlock()
				for _, task := range tasks {
					taskJSON, _ := json.Marshal(task)
					d.sendRESTMessage(string(taskJSON))
				}
			}
		}
	}()

	for {
		select {
		case <-d.stopCh:
			close(heartbeatDone)
			<-heartbeatDone
			<-taskSenderDone
			return
		default:
		}

		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		_, msg, err := conn.ReadMessage()
		if err != nil {
			slog.Error("Discord gateway read error", "channel_id", d.channelID, "error", err)
			close(heartbeatDone)
			<-heartbeatDone
			<-taskSenderDone
			return
		}

		var payload struct {
			Op int             `json:"op"`
			D  json.RawMessage `json:"d"`
			S  *int            `json:"s"`
			T  string          `json:"t"`
		}
		if json.Unmarshal(msg, &payload) != nil {
			continue
		}

		if payload.S != nil {
			d.sequence = *payload.S
		}

		switch payload.Op {
		case 1: // Heartbeat request
			hb, _ := json.Marshal(map[string]interface{}{
				"op": 1,
				"d":  d.sequence,
			})
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			conn.WriteMessage(websocket.TextMessage, hb)
		case 7: // Reconnect
			slog.Info("Discord: reconnect requested")
			close(heartbeatDone)
			<-heartbeatDone
			<-taskSenderDone
			return
		case 9: // Invalid Session
			slog.Warn("Discord: invalid session, reconnecting")
			close(heartbeatDone)
			<-heartbeatDone
			<-taskSenderDone
			return
		case 10: // Hello (reconnect scenario, skip — already handled)
		case 11: // Heartbeat ACK
		}

		if payload.T == "READY" {
			var ready struct {
				SessionID string `json:"session_id"`
			}
			if json.Unmarshal(payload.D, &ready) == nil {
				d.sessionID = ready.SessionID
				slog.Info("Discord bot connected", "session_id", d.sessionID, "channel_id", d.channelID)
			}
		}

		if payload.T == "MESSAGE_CREATE" {
			var event struct {
				ChannelID string `json:"channel_id"`
				Content   string `json:"content"`
				Author    struct {
					ID  string `json:"id"`
					Bot bool   `json:"bot"`
				} `json:"author"`
			}
			if json.Unmarshal(payload.D, &event) == nil {
				if event.Author.Bot {
					continue
				}
				if event.ChannelID == d.channelID {
					d.processMessage(event.Content, channelID)
				}
			}
		}
	}
}

func (d *DiscordExternalC2) processMessage(content, channelID string) {
	var extMsg extC2WSMessage
	if err := json.Unmarshal([]byte(content), &extMsg); err != nil {
		return
	}
	if extMsg.Type == "result" {
		d.server.processExternalC2Result(extMsg.AgentID, extMsg.TaskID, extMsg.Result)
	}
}

func (d *DiscordExternalC2) sendRESTMessage(content string) {
	url := fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages", d.channelID)
	body, _ := json.Marshal(map[string]string{"content": content})

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		slog.Error("Discord REST request create failed", "error", err)
		return
	}
	req.Header.Set("Authorization", "Bot "+d.botToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Error("Discord REST send failed", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 429 {
		var rl struct {
			RetryAfter float64 `json:"retry_after"`
		}
		if json.NewDecoder(resp.Body).Decode(&rl) == nil {
			time.Sleep(time.Duration(rl.RetryAfter*1000) * time.Millisecond)
			d.sendRESTMessage(content)
			return
		}
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		slog.Error("Discord REST error", "status", resp.StatusCode, "body", string(bodyBytes))
	}
}

func (d *DiscordExternalC2) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.running {
		return
	}
	d.running = false
	close(d.stopCh)

	channelID := "extc2-discord-" + d.channelID
	d.server.extC2ChannelsMu.Lock()
	delete(d.server.extC2Channels, channelID)
	d.server.extC2ChannelsMu.Unlock()
}

func (d *DiscordExternalC2) IsRunning() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.running
}
