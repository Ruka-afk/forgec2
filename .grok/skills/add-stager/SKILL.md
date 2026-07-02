---
name: add-stager
description: Add ForgeC2 staged payload delivery (stager, stage loader, dropper). Use for stager, stage payload, or /add-stager.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: feature
---

## When to use

Add or modify staged implant delivery (small stager downloads full agent).

## Key files

| File | Role |
|------|------|
| `internal/payload/agent_stager/stager.go` | Stager binary logic |
| `internal/server/handlers_stage.go` | Stage download endpoint |
| `internal/payload/generator.go` | Stager vs full agent build modes |
| `templates/generate.html` | Stager option in UI |
| `static/js/generate.js` | Build form flags |

## Stager flow

1. Operator generates stager pointing at stage URL.
2. Stager runs on target → HTTP GET stage endpoint.
3. Server serves encrypted/signed agent blob.
4. Stager loads in memory → full agent beacons.

## Checklist

1. Stage handler validates token/session, serves payload bytes.
2. Stager verifies checksum/key before execution.
3. Audit log on stage download (IP, user agent).
4. Rate-limit stage endpoint to prevent abuse.

## Verify

- Generate stager artifact from `/generate`
- Run stager on test host → full agent registers
- Stage URL without auth fails appropriately