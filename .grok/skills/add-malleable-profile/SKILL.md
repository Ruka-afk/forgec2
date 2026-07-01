---
name: add-malleable-profile
description: Add malleable C2 profiles to ForgeC2 (HTTP traffic shaping, sleep, UA, profile lock on generate page). Use for malleable profiles or C2 traffic disguise.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: feature
---

## Profile JSON

**Location**: `internal/malleable/` or profile presets in generate flow.

Fields: `name`, `user_agent`, `beacon_uri`, `method`, `headers`, `sleep`, `jitter`.

Existing presets: bing, google, office365, teams, zoom, slack, etc.

## Manual override (generate page)

When profile = `default` (manual):
- User sets interval, jitter, UA on generate form
- `internal/malleable/profile.go` — `UsesManualProfileSettings()`
- Non-default profiles **lock** interval/UA fields (`generate.js` `setProfileFieldLock`)

## Generator wiring

**File**: `internal/payload/generator.go`

`NormalizeImplantConfig()` applies profile sleep/UA unless manual mode.

## Verify

- Generate with locked profile → interval/UA from preset
- Generate with `default` → form values used
- Beacon traffic matches expected headers/URI