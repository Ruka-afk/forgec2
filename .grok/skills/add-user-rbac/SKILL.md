---
name: add-user-rbac
description: Extend ForgeC2 user management and RBAC (roles, permissions). Use for users, roles, RBAC, or /add-user-rbac.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: feature
---

## When to use

Add users, roles, or permission checks.

## Key files

| File | Role |
|------|------|
| `handlers_users.go` | User CRUD |
| `handlers_auth.go` | Login, password change, session |
| `middleware/auth.go` | JWT, session, role extraction |
| `internal/db/models.go` | `User` model |
| `templates/users.html` | Admin user management |

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

## Verify

- Create user with `viewer` → write endpoints return 403
- Password change updates bcrypt hash in config/DB