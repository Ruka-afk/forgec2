---
name: add-task-type
description: Add a new ForgeC2 implant task type end-to-end (registry, agent handlers, server routes, UI, audit). Use when adding task types, new commands, or /add-task-type.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: feature
---

## When to use

Add a new implant task type (e.g. `cookie_export`, `shell`, `screenshot`).

## Checklist

### 1. Agent registry

**File**: `internal/payload/agent/task_registry.go`

```go
"your_type": handleYourType,
```

### 2. Agent handler

| Platform | File |
|----------|------|
| Windows | `agent_windows.go` or `task_*.go` |
| Linux | `agent_linux.go` (stub OK) |
| macOS | `agent_darwin.go` (stub OK) |

Build tags: `//go:build linux || windows || darwin` + `// +build linux windows darwin` (space = OR).

### 3. Server route + handler

- `internal/server/server.go` — register `auth.POST("/agents/:id/your_type", ...)`
- `internal/server/handlers_commands.go` or domain handler — `createTask`, `logAction`, `dispatchTask`

### 4. Beacon result (if special processing)

**File**: `internal/server/handlers_beacon.go` — process result if not generic.

### 5. UI

- `handlers_toolkit.go` — `buildQuickActionCommand` case
- `templates/toolkit.html` — quick action button
- `templates/tasks.html` — badge color for type

### 6. Batch commands (optional)

**File**: `handlers_agents.go` — `handleBatchCommand` switch case.

### 7. Regenerate implant

Agent code changes require **rebuilding and redeploying** the implant binary. See `implant-regenerate` skill.

## Verify

- [ ] `go build ./cmd/server`
- [ ] Cross-compile: `GOOS=windows GOARCH=amd64 go build` in agent temp dir (or generate page)
- [ ] Task appears in UI, audit log written
- [ ] Result returns on beacon callback