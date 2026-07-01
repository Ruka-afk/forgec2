---
name: remote-desktop
description: Extend ForgeC2 interactive remote desktop (screen stream + input relay). Use for remote control, remote_input, mouse keyboard injection.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: feature
---

## Current state (v2.1 stub)

| Layer | Status |
|-------|--------|
| Screen stream | Working (JPEG frames, `handlers_monitor.go`) |
| Input API | `POST /api/agents/:id/input` — `{type,x,y,key}` |
| Agent task | `remote_input` — log-only stub on all platforms |

## Implementation path

### 1. Server relay

**File**: `handlers_monitor.go`

- Queue input events per agent
- Deliver on beacon callback or dedicated poll endpoint
- WebSocket optional for lower latency

### 2. Agent (Windows first)

**File**: `remote_input_windows.go`

```go
// SendInput / mouse_event / keybd_event for click, move, key
```

Register in `task_registry.go`, poll pending input in main loop.

### 3. Frontend

**File**: `templates/screen.html` + JS

- Canvas over screen image
- `mousedown` / `keydown` → POST `/api/agents/:id/input`
- Coordinate mapping (scale canvas to remote resolution)

### 4. Linux/macOS

Stubs in `agent_linux.go` / `agent_darwin.go` until platform APIs added.

## Reference

`.opencode/PLAN_10_FEATURES.md` — P3.1 Remote Desktop

## Verify

- Click on canvas moves remote cursor (Windows)
- Keys typed in capture mode reach remote session
- No input queue overflow under load