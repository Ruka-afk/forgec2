---
name: add-listener
description: Add or extend ForgeC2 C2 listeners (HTTP/TCP/DNS/ICMP/SMB). Use when adding listeners, beacon protocols, or /add-listener.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: feature
---

## When to use

Add a new listener type, extend listener config, or wire beacon ingestion for a transport.

## Key files

| Area | File |
|------|------|
| DB model | `internal/db/models.go` — `Listener` struct |
| CRUD + start/stop | `internal/server/handlers_generate.go` (listener cache, create/delete) |
| UI | `internal/server/templates/listeners.html`, `static/js/listeners.js` |
| Routes | `internal/server/server.go` — `/listeners`, `/api/listeners` |
| Beacon ingress | `internal/server/handlers_beacon.go` |
| Transport servers | `dns_listener.go`, `icmp_listener.go`, `smb_listener.go`, TCP in server startup |

## Add HTTP listener (typical)

1. **DB**: ensure `Listener` has fields for new config (host, port, protocol, enabled).
2. **Handler**: `handleCreateListener` / `handleDeleteListener` in `handlers_generate.go`.
3. **Start**: server goroutine binds port when listener enabled.
4. **UI**: form in `listeners.html`; POST via `data-action` in `listeners.js`.
5. **Generate page**: `getListeners()` cache used by implant generator.
6. **i18n**: add keys to `locales.go` (see `add-i18n` skill).
7. **OpenAPI**: `api/openapi.yaml` if exposing REST.

## Beacon route rules

- Beacon endpoints (`POST /api/v1/beacon`, etc.) must stay **outside** auth middleware.
- Malleable profile shapes traffic — see `add-malleable-profile` skill.

## Verify

```bash
go test ./internal/server/...
curl -b cookies.txt http://localhost:8080/api/listeners
```

- Create listener in UI → appears in list
- Implant callbacks → `server.log` shows beacon processed
- Disable listener → beacon stops accepting