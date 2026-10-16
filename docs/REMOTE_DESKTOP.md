# Remote Desktop / Screen Streaming

ForgeC2's remote-desktop surface is an **experimental assist channel**: a
screenshot stream plus `remote_input` tasks. It is not an RDP/VNC stack, and the
document exists so the boundaries are explicit instead of folklore.

## Transports

There are two transports. Only the first one is used by the UI.

### 1. Operator path (what the UI uses)

```
POST /agents/:id/screen/start        (agentCmd: agents.write + operator)
        -> screen_stream_start task -> agent captures frames
agent -> beacon result types screen_frame
        -> server caches the frame (screenFrames[agent]) and fans out a
           {"type":"screenshot", ...} message over the dashboard WebSocket
GET  /agents/:id/screenshot          (agentCmd: agents.write + operator)
        -> HTTP fallback: latest cached frame, or newest on-disk PNG
POST /api/agents/:id/input           (agentCmd: agents.write + operator)
        -> remote_input task ({"type":"move|click|key","x","y","key"})
POST /agents/:id/screen/stop         (agentCmd: agents.write + operator)
```

The dashboard socket is therefore a **screen-content channel**, not just a task
notifier, and is gated accordingly (see *Gates*).

### 2. Legacy WebSocket relay

```
GET /ws/remote-desktop?agent_id=...  (rdRead: agents.read + operator)
        -> operator input events relayed to a WebSocket-connected beacon
           (rdHub.SendInput); frames would flow back through
           rdHub.BroadcastFrame, which currently has no producer
```

Kept for API compatibility. It receives the same input normalization, origin
check, connection caps, keepalive and auditing as the main path (this batch),
but treats screen frames as best-effort.

## Gates

Every screen-content read passes three gates; a request must clear all of them.

| Gate | Mechanism | Failure mode |
| --- | --- | --- |
| Operator identity | `requireOperator` (role `viewer`/empty rejected) | `403` |
| Tenant scope | `getAgentOrFail` / `tenantScope` | `404` |
| `agents.write` | `RequirePermission(db.PermAgentsWrite)` on the route group; `wsClientConn.canViewScreen` for socket fan-out | `403` / silently not delivered |

The socket gate is resolved **once per connection** from the user row
(`userCanViewScreen`): built-in roles fall back to the static permission map,
custom roles resolve through `custom_roles`, and any lookup failure fails
closed. Fan-out is additionally tenant-filtered (`tenant_id = 0` legacy
operators plus the agent's tenant).

## Limits and rates

| Limit | Value | Enforced in |
| --- | --- | --- |
| Viewers per agent | 8 | `RDHub.Join` + pre-upgrade check (`429`) |
| Viewer connections, global | 64 | `RDHub` (`429`) |
| Monitor registrations | 100 | `handleStartScreenMonitor` (`429`) |
| Input events | 40/second per (operator, agent), fixed window | `handleAgentRemoteInput` (`429`) |
| WS input events | same normalization, 40/second per connection (excess dropped) | `handleRDWebSocket` |
| Stream interval | 1–60 seconds | `handleStartScreenMonitor` (`400`) |
| Stream quality | `low` / `medium` / `high` | `handleStartScreenMonitor` (`400`) |
| Stream width | integer 16–16384, canonicalized | `handleStartScreenMonitor` (`400`) |
| Stream mime | `webp` / `jpeg` / `png`; requires `width` (the agent command is positional) | `handleStartScreenMonitor` (`400`) |
| Coordinates | ±32767 | `remoteInputRequest.normalize` (`400`) |
| Key names | 1–32 characters, lowercased | `remoteInputRequest.normalize` (`400`) |
| Websocket keepalive | ping every 30s, 5-minute read deadline | `handleRDWebSocket` |

Two of these are deliberately fail-loud changes: an invalid `mime` used to be
silently ignored, and `width` used to be spliced into the agent's CSV stream
command (`interval,quality[,width][,mime]`) verbatim — parameter injection into
the agent-side parser. Both now answer `400`, and so does a `mime` sent without
a `width` — the agent parses that command positionally.

Screen-monitor start/stop is **single-flight**: while an operation is in flight
for an agent, another start/stop is answered `409` instead of double-queueing
(and the registry mutex is no longer held across task creation, plugin hooks,
broadcasts or audit writes, so frame ingest never queues behind an unrelated
DB insert).

## Audit trail

| Action | Written when |
| --- | --- |
| `screen_monitor_start` / `screen_monitor_stop` | monitor lifecycle accepted |
| `remote_input` | each **accepted** input event (rate limiter first) |
| `request_screenshot` | `POST /agents/:id/screenshot` task path |
| `screenshot_request` | `/rd/:id/screenshot` |
| `remote_desktop_open` / `remote_desktop_close` | legacy websocket relay join/leave |

## Known limits

- **Input latency equals beacon cadence.** `remote_input` is a task: it is
  delivered on the agent's next check-in, and the pending-task cap
  (`MaxPendingTasksPerAgent`) bounds how far a burst can run ahead. This is an
  assist channel for clicks and single keys, not interactive control.
- **No modifier chords.** The agent-side dispatchers (Windows
  `SetCursorPos`/`mouse_event`/`keybd_event`, Linux `xdotool`/`ydotool`, macOS
  `osascript`) take one key at a time; the page's modifier handling is
  best-effort.
- **Frame poll is 1s at best server-side** while the UI can poll at 250ms; the
  WebSocket push is the primary transport and the HTTP poll is a fallback
  (skipped while pushed frames are fresh).
- Reading the latest frame requires `agents.write` because the same route takes
  screenshots; listing stored screenshots is `agents.read`.

## Verification

```sh
# Hub semantics, input validation, rate limiting, socket fan-out, page scoping
go test ./internal/server/ -run 'TestRDHub|TestRemoteInput|TestHandleAgentRemoteInput|TestBroadcastScreenshot|TestScreenMonitor|TestScreenPage|TestTokenPage' -count=1

# Race detector (the hub and limiter are shared state)
go test -race ./internal/server/ -run 'TestRDHub|TestHandleAgentRemoteInput|TestBroadcastScreenshot' -count=1

# Frontend hook: interval units, WS-live polling shortcut
cd frontend && npx vitest run src/pages/agents/detail/remote-desktop/components/useRemoteDesktop.test.ts
```
