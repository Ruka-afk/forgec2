---
name: add-traffic-log
description: Extend ForgeC2 beacon traffic monitor (request log, auto-refresh). Use for traffic monitor, beacon log, or /add-traffic-log.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: feature
---

## When to use

Add traffic logging fields, filters, or UI for beacon HTTP request inspection.

## Key files

| File | Role |
|------|------|
| `handlers_traffic.go` | Traffic page + API |
| `templates/traffic.html` | UI with `ui-card` table |
| `static/js/traffic.js` | Auto-refresh, `data-action="load-traffic"` |
| Beacon middleware | Log request metadata on `/api/v1/beacon` |

## Data to capture

- Timestamp, source IP, agent ID, URI, user-agent, payload size
- Optional: malleable profile name, response code

## Add field checklist

1. Store in ring buffer or DB table (`TrafficLog` model — see `add-database-model`).
2. Expose `GET /api/traffic?limit=100` JSON.
3. Update `traffic.js` table renderer.
4. i18n keys under `traffic.*` in `locales.go`.

## Verify

- Beacon from implant → row appears in `/traffic`
- Auto-refresh checkbox toggles polling
- Count label `#traffic-count` updates