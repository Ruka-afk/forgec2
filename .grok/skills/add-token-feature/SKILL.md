---
name: add-token-feature
description: Extend ForgeC2 Kerberos/token operations (dump, impersonate, page UI). Use for token, kerberos, impersonation, or /add-token-feature.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: feature
---

## When to use

Add token manipulation tasks or extend the per-agent Token page.

## Key files

| File | Role |
|------|------|
| `handlers_token.go` | Token page + list/make/impersonate APIs |
| `templates/token.html` | Token UI on agent |
| Agent | Windows token APIs in credential/task handlers |
| Routes | `/agents/:id/token`, `/agents/:id/token/list` |

## Typical operations

- List tokens on agent (user sessions)
- Steal / duplicate token
- Impersonate logged-on user
- Revert to self

## Checklist

1. Agent task handler calls Windows APIs (`OpenProcessToken`, `DuplicateTokenEx`, etc.).
2. Server creates task, polls result, returns JSON list to UI.
3. UI table in `token.html` with action buttons via `data-action`.
4. High-privilege ops restricted to `operator`+ role.
5. Audit every impersonation attempt.

## Verify

- Token page loads session list from online Windows agent
- Impersonate task changes thread token (verify with `whoami` task)
- Revert restores original context