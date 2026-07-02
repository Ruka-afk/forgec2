---
name: add-extc2
description: Extend ForgeC2 external C2 bridge (third-party connectors, ExtC2 protocol). Use for external C2, ExtC2, or /add-extc2.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: feature
---

## When to use

Bridge external C2 frameworks or custom connectors into ForgeC2 tasking.

## Key files

| File | Role |
|------|------|
| `internal/server/extc2.go` | `handleExtC2Receive`, `handleExtC2Send`, route registration |
| `server.go` | `registerExtC2Routes` — typically unauthenticated or API-key gated |

## Protocol pattern

1. External connector POSTs beacon/metadata to `/api/extc2/receive`.
2. Server maps external agent ID → internal implant record (or creates shadow agent).
3. Tasks queued for external agent retrieved via `/api/extc2/send` poll.
4. Results POST back through receive endpoint.

## Security

- Require shared API key or mTLS — do not expose ExtC2 routes without auth.
- Rate-limit receive/send endpoints.
- Validate payload sizes.

## Verify

- Connector simulation sends receive → agent appears or updates
- Queued task delivered on send poll
- Result round-trips to task record in DB