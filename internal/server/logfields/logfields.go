package logfields

// Canonical slog field key constants.
// All structured logging in the server package MUST use these constants
// to ensure consistent, searchable, and aggregatable log output.
const (
	// Error: always "error"
	Error = "error"

	// Agent identifiers
	AgentID = "agent_id"
	TaskID  = "task_id"

	// User identifiers
	User     = "user"
	UserID   = "user_id"
	Username = "username"

	// Listener
	ListenerID = "listener_id"

	// Plugin
	PluginID = "plugin_id"

	// Channel
	ChannelID = "channel_id"

	// Panic recovery
	Panic   = "panic"
	Stack   = "stack"
	Recovered = "err"
)
