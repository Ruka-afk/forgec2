package integrations

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"gorm.io/gorm"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
)

type SlackBot struct {
	client       *slack.Client
	socketClient *socketmode.Client
	botToken     string
	appToken     string
	enabled      bool
	db           *gorm.DB
	server       interface{}
	ctx          context.Context
	cancel       context.CancelFunc
	mu           sync.Mutex
}

func NewSlackBot(cfg config.SlackConfig, database *gorm.DB, srv interface{}) *SlackBot {
	client := slack.New(cfg.BotToken, slack.OptionAppLevelToken(cfg.AppToken))
	socketClient := socketmode.New(client)
	ctx, cancel := context.WithCancel(context.Background())
	return &SlackBot{
		client:       client,
		socketClient: socketClient,
		botToken:     cfg.BotToken,
		appToken:     cfg.AppToken,
		enabled:      cfg.Enabled,
		db:           database,
		server:       srv,
		ctx:          ctx,
		cancel:       cancel,
	}
}

func (b *SlackBot) Start() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("recovered from panic", "err", r, "stack", string(debug.Stack()))
			}
		}()
		err := b.socketClient.RunContext(b.ctx)
		if err != nil {
			slog.Error("SlackBot RunEventLoop error", "err", err)
		}
	}()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("recovered from panic", "err", r, "stack", string(debug.Stack()))
			}
		}()
		for {
			select {
			case <-b.ctx.Done():
				return
			case event, ok := <-b.socketClient.Events:
				if !ok {
					return
				}
				b.handleSocketEvent(event)
			}
		}
	}()
}

func (b *SlackBot) Stop() {
	b.cancel()
}

func (b *SlackBot) handleSocketEvent(event socketmode.Event) {
	switch event.Type {
	case socketmode.EventTypeEventsAPI:
		eventsAPIEvent, ok := event.Data.(slackevents.EventsAPIEvent)
		if !ok {
			return
		}
		b.socketClient.Ack(*event.Request)
		switch ev := eventsAPIEvent.InnerEvent.Data.(type) {
		case *slackevents.AppMentionEvent:
			b.handleMention(ev)
		case *slackevents.MessageEvent:
			if ev.ChannelType == "im" {
				b.handleDirectMessage(ev)
			}
		}
	case socketmode.EventTypeInteractive:
		callback, ok := event.Data.(slack.InteractionCallback)
		if !ok {
			return
		}
		b.socketClient.Ack(*event.Request)
		b.handleInteraction(callback)
	case socketmode.EventTypeSlashCommand:
		cmd, ok := event.Data.(slack.SlashCommand)
		if !ok {
			return
		}
		b.socketClient.Ack(*event.Request)
		b.handleSlashCommand(cmd)
	}
}

func (b *SlackBot) handleDirectMessage(ev *slackevents.MessageEvent) {
	if ev.BotID != "" {
		return
	}
	text := strings.TrimSpace(ev.Text)
	if text == "" {
		return
	}
	b.handleTextCommand(ev.Channel, text)
}

func (b *SlackBot) handleMention(ev *slackevents.AppMentionEvent) {
	text := strings.TrimSpace(ev.Text)
	text = stripMention(text)
	if text == "" {
		b.sendMessage(ev.Channel, "Hello! Try: `agents`, `tasks <agent_id>`, `exec <agent_id> <command>`")
		return
	}
	b.handleTextCommand(ev.Channel, text)
}

func (b *SlackBot) handleSlashCommand(cmd slack.SlashCommand) {
	b.handleTextCommand(cmd.ChannelID, cmd.Text)
}

func stripMention(text string) string {
	for _, prefix := range []string{"<@", "<!@"} {
		if strings.Contains(text, prefix) {
			start := strings.Index(text, ">")
			if start != -1 && start+1 < len(text) {
				text = strings.TrimSpace(text[start+1:])
			} else {
				text = ""
			}
		}
	}
	return text
}

// handleTextCommand parses "agents", "tasks <id>", "exec <id> <cmd...>"
func (b *SlackBot) handleTextCommand(channel, text string) {
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return
	}
	cmd := strings.ToLower(parts[0])
	args := parts[1:]

	switch cmd {
	case "agents":
		b.handleListAgents(channel)
	case "tasks":
		if len(args) < 1 {
			b.sendMessage(channel, "Usage: tasks <agent_id>")
			return
		}
		b.handleListTasks(channel, args[0])
	case "exec":
		if len(args) < 2 {
			b.sendMessage(channel, "Usage: exec <agent_id> <command>")
			return
		}
		b.handleExecCommand(channel, args[0], strings.Join(args[1:], " "))
	case "help":
		b.sendMessage(channel, "Commands:\n• `agents` - list online agents\n• `tasks <agent_id>` - list tasks for an agent\n• `exec <agent_id> <command>` - execute a command on an agent")
	default:
		b.sendMessage(channel, "Unknown command. Try: `agents`, `tasks <agent_id>`, `exec <agent_id> <command>`, `help`")
	}
}

func (b *SlackBot) handleListAgents(channel string) {
	var implants []db.Implant
	if err := b.db.Where("status = ?", "online").Find(&implants).Error; err != nil {
		b.sendMessage(channel, fmt.Sprintf("Error querying agents: %v", err))
		return
	}
	if len(implants) == 0 {
		b.sendMessage(channel, "No online agents.")
		return
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("*Online Agents (%d)*\n", len(implants)))
	for _, a := range implants {
		sb.WriteString(fmt.Sprintf("• `%s` | %s | %s | %s\n", a.ID, a.Hostname, a.IP, a.OS))
	}
	b.sendMessage(channel, sb.String())
}

func (b *SlackBot) handleListTasks(channel, agentID string) {
	var tasks []db.Task
	if err := b.db.Where("agent_id = ?", agentID).Order("created_at DESC").Limit(10).Find(&tasks).Error; err != nil {
		b.sendMessage(channel, fmt.Sprintf("Error querying tasks: %v", err))
		return
	}
	if len(tasks) == 0 {
		b.sendMessage(channel, fmt.Sprintf("No tasks for agent `%s`.", agentID))
		return
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("*Recent Tasks for `%s`*\n", agentID))
	for _, t := range tasks {
		resultPreview := ""
		if t.Result != "" {
			if len(t.Result) > 100 {
				resultPreview = t.Result[:100] + "..."
			} else {
				resultPreview = t.Result
			}
		}
		sb.WriteString(fmt.Sprintf("• `#%d` %s | %s | %s\n", t.ID, t.Type, t.Status, resultPreview))
	}
	b.sendMessage(channel, sb.String())
}

func (b *SlackBot) handleExecCommand(channel, agentID, command string) {
	var agent db.Implant
	if err := b.db.First(&agent, "id = ?", agentID).Error; err != nil {
		b.sendMessage(channel, fmt.Sprintf("Agent `%s` not found.", agentID))
		return
	}

	b.sendApprovalRequest(channel, agentID, command)
}

func (b *SlackBot) sendApprovalRequest(channel, agentID, command string) {
	headerText := slack.NewTextBlockObject("mrkdwn",
		fmt.Sprintf("*High-Risk Operation*\nAgent: `%s`\nCommand: `%s`", agentID, command), false, false)
	section := slack.NewSectionBlock(headerText, nil, nil)

	approveBtn := slack.NewButtonBlockElement("approve", fmt.Sprintf("%s|%s", agentID, command),
		slack.NewTextBlockObject("plain_text", "Approve", false, false))
	approveBtn.Style = slack.StylePrimary
	rejectBtn := slack.NewButtonBlockElement("reject", fmt.Sprintf("%s|%s", agentID, command),
		slack.NewTextBlockObject("plain_text", "Reject", false, false))
	rejectBtn.Style = slack.StyleDanger

	actionBlock := slack.NewActionBlock("approval_actions", approveBtn, rejectBtn)
	blocks := []slack.Block{section, actionBlock}

	_, _, err := b.client.PostMessage(channel, slack.MsgOptionBlocks(blocks...))
	if err != nil {
		slog.Error("SlackBot send approval request failed", "err", err)
	}
}

func (b *SlackBot) handleInteraction(callback slack.InteractionCallback) {
	if callback.Type != slack.InteractionTypeBlockActions {
		return
	}
	for _, action := range callback.ActionCallback.BlockActions {
		if action.ActionID == "approve" {
			b.executeApprovedTask(callback.Channel.ID, action.Value)
			b.updateMessage(callback.Channel.ID, callback.MessageTs, "Task approved and dispatched.")
		} else if action.ActionID == "reject" {
			b.updateMessage(callback.Channel.ID, callback.MessageTs, "Task rejected.")
		}
	}
}

func (b *SlackBot) executeApprovedTask(channel, value string) {
	parts := strings.SplitN(value, "|", 2)
	if len(parts) != 2 {
		b.sendMessage(channel, "Invalid approval payload.")
		return
	}
	agentID := parts[0]
	command := parts[1]

	task := db.Task{
		AgentID: agentID,
		Type:    "shell",
		Command: command,
		Shell:   "cmd.exe",
		Status:  "pending",
	}
	if err := b.db.Create(&task).Error; err != nil {
		b.sendMessage(channel, fmt.Sprintf("Failed to create task: %v", err))
		return
	}
	b.sendMessage(channel, fmt.Sprintf("Task #%d dispatched to `%s`: `%s`", task.ID, agentID, command))
}

func (b *SlackBot) sendMessage(channel, text string) {
	_, _, err := b.client.PostMessage(channel, slack.MsgOptionText(text, false))
	if err != nil {
		slog.Error("SlackBot sendMessage error", "err", err)
	}
}

func (b *SlackBot) updateMessage(channel, timestamp, text string) {
	_, _, _, err := b.client.UpdateMessage(channel, timestamp, slack.MsgOptionText(text, false))
	if err != nil {
		slog.Error("SlackBot updateMessage error", "err", err)
	}
}

func maskToken(token string) string {
	if len(token) <= 4 {
		return token
	}
	return token[:4] + "..." + token[len(token)-4:]
}
