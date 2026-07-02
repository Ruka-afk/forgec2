---
name: add-shell-feature
description: Extend ForgeC2 interactive shell (real-time mode, heartbeat, UTF-8, task polling). Use for shell bugs, shell features, or /add-shell-feature.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: feature
---

## When to use

Shell shows wrong interval, garbled Chinese output, commands not returning, or adding shell UX features.

## Key files

| Layer | File |
|-------|------|
| UI | `templates/static/js/shell.js` (bundled in `agents.bundle.js`) |
| Page | `templates/shell.html` or agent detail shell tab |
| Server command | `handlers_commands.go` — `handleSendCommand` |
| Task status | `handlers_agents.go` — `handleGetTaskStatus` |
| Agent shell | `internal/payload/agent/task_execution.go` |
| UTF-8 Windows | `internal/payload/agent/encoding_windows.go` |
| Interval | DB `implants.current_interval` vs build-time ldflags |

## Real-time (0s) mode

- Agent `current_interval = 0` → poll every ~1s on server and client.
- Old implants may still have DB interval 10 — use `set_sleep` task or **regenerate** (see `implant-regenerate` skill).
- `shell.js`: `getShellTiming()` reads `window.agentInterval`.

## UTF-8 on Windows

- Agent may base64-encode shell output on Windows.
- `shell.js`: `decodeShellResultText()` decodes before display.
- Server must not double-encode.

## Add a shell feature checklist

1. Agent handler if new task behavior needed (`add-task-type` skill).
2. Server route: `POST /agents/:id/command` or dedicated endpoint.
3. `shell.js`: send command, poll task ID, append output to terminal.
4. Rebuild `agents.bundle.js`; `go build`; restart.

## Debug

| Symptom | Fix |
|---------|-----|
| 10s delay with 0s config | Regenerate implant or `set_sleep 0,20` |
| Mojibake / `???` | Check `encoding_windows.go` + `decodeShellResultText` |
| Command hangs | Task stuck pending — check beacon + `server.log` |
| No output | Task failed — inspect task result in DB or UI |

## Verify

- Open `/agents/:id/shell`, run `whoami`, output appears within expected interval
- Chinese paths/commands render correctly on Windows agent