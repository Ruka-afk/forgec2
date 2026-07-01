---
name: debug-forgec2
description: Troubleshoot ForgeC2 server, WebSocket, shell heartbeat, bundles, and connection issues. Use when connection refused, UI not updating, WebSocket down, or /debug-forgec2.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: debug
---

## When to use

Server won't start, UI broken, implant not callbacking, or changes not visible.

## Quick diagnostics

```powershell
# Server up?
Invoke-WebRequest http://127.0.0.1:8080/login -UseBasicParsing

# Port in use?
Get-NetTCPConnection -LocalPort 8080 -ErrorAction SilentlyContinue

# Logs
Get-Content server.log -Tail 30
Get-Content server.err -Tail 20
```

## Common issues

| Symptom | Cause | Fix |
|---------|-------|-----|
| Connection refused | Server not running or wrong port | `.\server.exe -config config.yaml` |
| JS changes ignored | Bundles stale / go:embed | `make bundle` + `go build` + Ctrl+Shift+R |
| Shell shows 10s, config is 0s | DB `current_interval` on old implant | Regenerate implant or `set_sleep 0,20` |
| No online toast | WS failed, polling fallback | Check `/ws` in logs; layout.js polls every 5s |
| AI buttons dead | Missing direct bind | See `fix-ui-page`; AI uses `bindAIActions()` |
| Beacon 401 | Auth middleware on wrong path | Beacon routes must be outside auth group |
| Chinese garbled in shell | UTF-8 encoding | `encoding_windows.go` base64 path |

## WebSocket (v2.1+)

- Client: `layout.js` — ping every 25s, 20 reconnect attempts
- WS healthy → HTTP agent poll **stopped**
- WS dead → `startAgentStatusPolling()` resumes

## Dev mode (unbundled JS)

```bash
FORGEC2_DEV=1 go run ./cmd/server -config config.yaml
```

## Verify fix

1. `go test ./...`
2. Hard refresh browser
3. Check `server.log` for errors on reproduced action