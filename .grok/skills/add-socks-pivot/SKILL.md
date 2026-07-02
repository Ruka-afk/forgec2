---
name: add-socks-pivot
description: Extend ForgeC2 SOCKS relay and pivoting (tunnel, relay sessions). Use for SOCKS, pivot, relay, or /add-socks-pivot.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: feature
---

## When to use

Add SOCKS5 relay, pivot chain, or tunnel management features.

## Key files

| File | Role |
|------|------|
| `handlers_socks_relay.go` | Start/stop relay, status API |
| `handlers_agents.go` | `handlePivoting` page data |
| `templates/pivoting.html` | Relay UI |
| `static/js/pivoting.js` | In `admin.bundle.js` |
| Agent | SOCKS task handler in payload agent |
| Routes | `GET /agents/:id/socks_relay/status`, relay start/stop POST |

## Relay flow

1. Operator selects relay agent + local bind port in UI.
2. Server queues SOCKS task on agent.
3. Agent opens local SOCKS listener, tunnels via beacon.
4. Status endpoint reports active sessions; stop tears down.

## Add feature checklist

1. Extend task args schema on server + agent.
2. Update `pivoting.js` table render for new session fields.
3. Audit log on start/stop.
4. Rate-limit / auth: operator+ only.

## Verify

- Start relay on online agent → status shows active
- External tool connects through SOCKS port
- Stop relay → sessions cleared