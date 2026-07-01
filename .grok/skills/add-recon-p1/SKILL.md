---
name: add-recon-p1
description: Add ForgeC2 P1 recon features (cookie_export, vpn_creds, file hunter, screen trigger). Use for recon tasks, browser cookies, VPN credentials.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: feature
---

## Implemented (v2.1)

| Task | Agent file | Server route |
|------|------------|--------------|
| `cookie_export` | `cookies_windows.go` | `POST /agents/:id/cookie_export` |
| `vpn_creds` | `vpn_creds_windows.go` | `POST /agents/:id/vpn_creds` |
| Enhanced keylog | `agent_windows.go` keyloggerLoop | existing keylogger tasks |

Toolkit buttons in `toolkit.html` + `handlers_toolkit.go`.

## Adding similar recon task

1. Windows implementation (SQLite: `modernc.org/sqlite` in agent go.mod)
2. Linux/darwin stub returning clear message
3. `task_registry.go` + `task_credentials.go` or `task_recon.go`
4. Server handler + credential vault parsing if needed
5. AI tool in `handlers_ai.go` (optional)

## Planned (not yet done)

- **File hunter** — recursive match + auto download (`task_fileops.go`)
- **Screen trigger** — foreground window match → auto screenshot
- **USB propagate** — removable drive detection + copy

See `.opencode/PLAN_10_FEATURES.md` Phase 1–2.

## Verify

- Task created, runs on Windows agent
- Results stored or returned on beacon
- Credentials parsed into vault if applicable