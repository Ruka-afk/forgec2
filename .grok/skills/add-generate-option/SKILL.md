---
name: add-generate-option
description: Add ForgeC2 implant generator options (ldflags, obfuscation, artifacts). Use for generate page, build flags, or /add-generate-option.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: feature
---

## When to use

Add a new checkbox/option on the Generate page that affects implant build output.

## Key files

| File | Role |
|------|------|
| `handlers_generate.go` | 20+ handlers — build queue, download, one-liner |
| `internal/payload/generator.go` | `go build` invocation, ldflags, tags |
| `internal/payload/obfuscator.go` | String/binary obfuscation |
| `internal/payload/artifact_kit.go` | Executable templates, icons |
| `templates/generate.html` | Build form |
| `static/js/generate.js` | Form submit, profile lock |

## Add option checklist

1. **UI**: add input in `generate.html` (checkbox/select).
2. **JS**: include field in POST body in `generate.js`.
3. **Handler**: parse in `handleGenerate` / `handleBuildImplant`.
4. **Generator**: pass as `-ldflags "-X main.YourFlag=value"` or build tag.
5. **Agent**: read ldflags variable in `agent.go` init or config struct.
6. **DB**: store build record with option snapshot (`db.Build`).

## Common ldflags

```
-X main.callbackURL=...
-X main.sleepInterval=...
-X main.evasionEnabled=true
```

## Profile lock

- Malleable profile selected on generate → locked to listener profile name.
- See `add-malleable-profile` skill.

## Verify

- Toggle new option → downloaded implant behaves differently
- Build history shows option in metadata
- `go build` succeeds for win/linux/darwin targets requested