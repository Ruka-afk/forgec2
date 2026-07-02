---
name: add-timeline-loot
description: Extend ForgeC2 operation timeline and loot collection pages. Use for timeline, loot, operation history, or /add-timeline-loot.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: feature
---

## When to use

Add timeline events, loot artifacts, or operation history visualization.

## Key files

| Feature | Handler | Template | JS |
|---------|---------|----------|-----|
| Timeline | `handlers_timeline.go` | `timeline.html` | `timeline.js` (`report.bundle.js`) |
| Loot | handlers in ops/report | loot template | `loot.js` (`report.bundle.js`) |
| Report | `handlers_report.go` | `report.html` | `report.js` |

## Timeline event sources

- Task create/complete (audit + tasks table)
- Agent online/offline (beacon handler)
- Credential harvested
- Lateral/privesc task results

## Add timeline entry

1. Emit normalized event struct `{time, type, agent, summary, detail}`.
2. `handleTimelineAPI` aggregates from DB queries.
3. `timeline.js` renders vertical feed with icons per type.

## Loot collection

- Files downloaded via `handlers_files.go`
- Screenshots, cred exports, report attachments
- Store metadata in DB; blob on disk under `data/loot/`

## Verify

- Complete task → appears on `/timeline`
- Downloaded file listed in loot view with download link
- Report generator includes new section (see `report-section` skill)