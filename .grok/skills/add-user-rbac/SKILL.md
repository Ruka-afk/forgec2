---
name: add-user-rbac
description: Extend ForgeC2 user management and RBAC (roles, TOTP, permissions). Use for users, roles, TOTP, RBAC, or /add-user-rbac.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: feature
---

## When to use

Add users, roles, permission checks, or TOTP 2FA flows.

## Key files

| File | Role |
|------|------|
| `handlers_users.go` | User CRUD |
| `handlers_auth.go` | Login, TOTP, password change, settings |
| `middleware/auth.go` | JWT, session, role extraction |
| `internal/db/models.go` | `User` model |
| `templates/users.html` | Admin user management |
| `templates/settings.html` | TOTP setup |

## Roles

| Role | Typical access |
|------|----------------|
| `admin` | Full config, users, purge |
| `operator` | Agents, tasks, listeners |
| `viewer` | Read-only pages |
| `guest` | Minimal |

Check in handlers: `c.Get("user_role")`.

## Add permission gate

```go
role, _ := c.Get("user_role")
if role != "admin" && role != "operator" {
    c.JSON(403, gin.H{"error": "forbidden"})
    return
}
```

## TOTP flow

- `handleTOTPGenerate` → QR secret
- `handleTOTPEnable` / `handleTOTPDisable`
- Login verifies TOTP when enabled on user

## Verify

- Create user with `viewer` → write endpoints return 403
- TOTP enable → login requires code
- Password change updates bcrypt hash in config/DB