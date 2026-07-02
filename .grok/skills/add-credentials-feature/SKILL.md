---
name: add-credentials-feature
description: Extend ForgeC2 credential vault (harvest, store, browser creds, mimikatz). Use for credentials, cred vault, mimikatz, or /add-credentials-feature.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: feature
---

## When to use

Add credential harvesting, vault storage, or new cred types (hash, ticket, browser).

## Key files

| File | Role |
|------|------|
| `handlers_credentials.go` | Vault CRUD, export, search |
| `handlers_browser_creds.go` | Browser cookie/password import |
| `internal/db/models.go` | `Credential` model |
| `templates/credentials.html` | Vault UI |
| `static/js/credentials.js` | In `ops.bundle.js` |
| Agent | `task_credentials.go`, `credentials_*_windows.go` |

## Add cred harvest task

1. Agent task type (e.g. `mimikatz`, `kerberoast`) via `add-task-type` skill.
2. Server `handleProcessCredResult` or lateral-style result handler stores parsed creds.
3. Dedup by domain/user/type before insert.
4. Never log plaintext passwords in audit — mask in UI.

## Vault UI

- Table with filters; `ui-card` panels
- Link creds to lateral movement form (`handlers_lateral.go` passes `.TotalCreds`)

## AI integration

- `list_credentials` tool in `handlers_ai.go` returns summary only (no secrets).

## Verify

- Run harvest task → cred appears in `/credentials`
- Lateral form can select stored cred
- Export respects role permissions