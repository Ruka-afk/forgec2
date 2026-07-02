---
name: add-collab-feature
description: Extend ForgeC2 multi-operator collaboration (WS presence, agent locks, page sync). Use for collab, operator online, agent lock, or /add-collab-feature.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: feature
---

## When to use

Add real-time operator presence, agent locking, or page-awareness for multi-user ops.

## Key files

| File | Role |
|------|------|
| `internal/server/collab.go` | `collabState`, WS conn map, agent locks |
| `internal/server/handlers_websocket.go` | `/ws` upgrade, message routing |
| `internal/db/models.go` | `AgentLock` model |
| `static/js/layout.js` | WS client, `sendWSPageUpdate`, online operators UI |
| `handlers_agents.go` | `handleLockAgent`, `handleUnlockAgent` |

## WS message flow

1. Client connects to `/ws` with session cookie.
2. Server registers `wsConn` with username, role, `currentPage`.
3. Page changes → `htmx:afterSettle` → `sendWSPageUpdate()` in `layout.js`.
4. Broadcast presence / locks to other connected operators.

## Add collab event

1. Define JSON event type in `collab.go` broadcast helper.
2. Server: emit on relevant handler (lock acquired, shell opened).
3. Client: handle in `layout.js` WS `onmessage` → toast or UI update.
4. Optional i18n keys for notifications.

## Agent lock checklist

- `POST /agents/:id/lock` — sets lock owner in DB + collab state.
- `POST /agents/:id/unlock` — clears if same user or admin.
- UI shows lock badge on agent row / detail page.

## Verify

- Two browsers logged in as different users see each other online
- Lock agent in browser A → browser B sees lock indicator
- Unlock restores shared access