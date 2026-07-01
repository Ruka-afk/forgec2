---
name: add-agent-feature
description: Add cross-platform implant features in ForgeC2 (Windows/Linux/macOS build tags, generator ldflags, task handlers). Use for agent changes, darwin/linux stubs, or /add-agent-feature.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: feature
---

## Platform files

| GOOS | File |
|------|------|
| windows | `agent_windows.go`, `*_windows.go` |
| linux | `agent_linux.go`, `encoding_unix.go`, `smb_transport_unix.go` |
| darwin | `agent_darwin.go` (shares unix helpers) |

Shared: `agent.go`, `task_registry.go`, `task_*.go` with `//go:build linux || windows || darwin`.

**Build tag rule**: `//go:build A || B` must pair with `// +build A B` (spaces = OR, not commas).

## Add feature workflow

1. Implement handler in platform file(s)
2. Register in `task_registry.go`
3. Add server endpoint (see `add-task-type`)
4. Update `generator.go` if new ldflags needed:

```go
// Examples already wired:
// IntervalStr, JitterStr, EvasionStr, UserAgent, Protocol, ...
ldflags := fmt.Sprintf(`-X "main.EvasionStr=%s" ...`, evasion)
```

5. **Regenerate and redeploy implant** — existing deployed binaries won't pick up code changes.

## Evasion flag

- Build-time: `EvasionStr=true` in ldflags
- Runtime: env `FORGEC2_EVASION=1`
- Implementation: `evasion_windows.go` → `sleepObfuscated()`

## Cross-compile check

```bash
GOOS=windows GOARCH=amd64 go build   # via generate page
GOOS=linux GOARCH=amd64 go build
GOOS=darwin GOARCH=amd64 go build
```

## Verify

- [ ] All three platforms compile (stubs OK on non-Windows)
- [ ] `AgentVersion` bumped in `agent.go` on release
- [ ] New implant generated from `/generate` page