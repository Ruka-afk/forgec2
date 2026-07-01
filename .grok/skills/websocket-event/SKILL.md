---
name: websocket-event
description: Add WebSocket real-time events in ForgeC2 (server broadcast, layout.js handler, locales). Use for live notifications, WS events, or /websocket-event.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: feature
---

## Event flow

```
broadcastToClients(JSON) → /ws → layout.js onmessage → showToast / UI update
```

## Server

```go
func (s *Server) broadcastYourEvent(agentID string) {
    payload, _ := json.Marshal(map[string]interface{}{
        "type":     "your_event",
        "agent_id": agentID,
    })
    s.broadcastToClients(payload)
}
```

Call from handler when event occurs. For alerts use `broadcastSystemAlert()` in `audit_alerts.go`.

### Existing events

| type | Source |
|------|--------|
| `agent_online` / `agent_offline` | beacon status |
| `task_update` | task completion |
| `system_alert` | audit_alerts.go |
| `user_online` / `user_offline` | collab.go |
| `agent_locked` / `agent_unlocked` | collab.go |
| `ping` / `pong` | collab.go (keepalive) |

## Client

**File**: `internal/server/templates/static/js/layout.js` — inside `_ws.onmessage`:

```js
} else if (data.type === 'your_event') {
    showToast(data.message || 'Event', 'info');
}
```

Then `make bundle` and rebuild server.

## Locales

Add to all 5 maps in `locales.go`: en, zh, ja, ko, ar.

## Verify

- WS connects (DevTools → Network → WS)
- Event JSON parses without console errors
- Toast/notification appears