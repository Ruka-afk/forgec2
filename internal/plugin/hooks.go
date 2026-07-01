package plugin

import "context"

// Standard hook event types.
const (
	EventAgentConnect    = "agent.connect"
	EventAgentDisconnect = "agent.disconnect"
	EventTaskCreated     = "task.created"
	EventTaskCompleted   = "task.completed"
	EventUserLogin       = "user.login"
	EventUserLogout      = "user.logout"
)

// EmitEvent dispatches an event to all loaded hook plugins that subscribe to it.
func EmitEvent(ctx context.Context, m *Manager, event Event) error {
	if m == nil {
		return nil
	}
	return m.ExecuteHook(ctx, event)
}
