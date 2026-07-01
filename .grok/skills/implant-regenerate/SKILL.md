---
name: implant-regenerate
description: When ForgeC2 implant changes require regenerate vs set_sleep. Use when heartbeat wrong, agent code changed, or old implant behavior.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: ops
---

## Requires full regenerate + redeploy

- Any change to `internal/payload/agent/*.go`
- New task types, encoding fixes, evasion flags
- Default interval/jitter in **generator** config (not yet on running implant)

**Action**: `/generate` → build new binary → deploy to target.

## Runtime update only (no redeploy)

- Sleep interval: task `set_sleep` or shell command
- Updates DB `current_interval` on next beacon

```text
set_sleep 0 20    # 0s interval, 20% jitter
```

## UI hints (v2.1+)

- Shell page: banner when `data-agent-interval` ≠ `data-expected-interval`
- Generate page: `generate.regen_hint` locale key

## DB check

```sql
SELECT id, hostname, current_interval, current_jitter FROM implants;
```

## Common mistake

User sets `default_interval: 0` in config.yaml but old implant still has `current_interval=10` in DB → shell shows 10s until regenerate or `set_sleep 0`.

## Verify

- New implant version matches `AgentVersion` in beacon metadata
- Shell timing matches expected interval