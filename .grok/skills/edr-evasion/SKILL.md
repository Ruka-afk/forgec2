---
name: edr-evasion
description: Configure and extend ForgeC2 EDR evasion (sleep obfuscation, evasion flags, AMSI/ETW stubs). Use for EDR bypass, evasion, sleep mask.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: feature
---

## Enabled evasion (v2.1)

### Build-time

Generate page → EXE form → **EDR 基础规避** checkbox

Sets ldflag: `-X "main.EvasionStr=true"`

### Runtime

```bash
FORGEC2_EVASION=1 ./agent
```

### Behavior

**File**: `evasion_windows.go` — `sleepObfuscated(duration)`

- Splits sleep into 40–420ms random chunks
- 30% chance of micro-pause between chunks
- Called from `sleepWithJitter()` in `agent.go` when evasion enabled

## Related files (Windows)

| File | Purpose |
|------|---------|
| `evasion_amsi_windows.go` | AMSI patch stubs |
| `evasion_etw_windows.go` | ETW patch stubs |
| `evasion_veh_unhook_windows.go` | VEH unhook |
| `sleepmask_windows.go` | Sleep mask |
| `obfuscator.go` | `GenerateEvasionReport()` |

## Extend evasion

1. Implement in `evasion_*_windows.go`
2. Call from agent init or task handler
3. Add generate UI toggle if user-configurable
4. Document in build log via `FormatEvasionBuildNote()`

## Verify

- Agent with evasion flag sleeps with chunked pattern (debug log)
- `go build` with `GOOS=windows`
- Evasion report generated on build when enabled