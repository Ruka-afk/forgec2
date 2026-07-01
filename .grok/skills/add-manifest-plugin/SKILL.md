---
name: add-manifest-plugin
description: Create ForgeC2 manifest-based plugins (YAML + Python/Go script, plugin runtime). Use for plugins/example style plugins or /add-manifest-plugin.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: feature
---

## Layout

```
plugins/your_plugin/
  manifest.yaml
  run.py          # or run.go
```

## manifest.yaml

```yaml
name: your_plugin
version: 1.0.0
type: command
description: What it does
author: ForgeC2
category: recon
entry: run.py
interpreter: python   # python | go
timeout: 120
params:
  - name: target
    type: string
    required: true
    description: Target host
```

Reference: `plugins/example/portscan/manifest.yaml`

## Runtime

- `internal/plugin/manager.go` — loads on server start
- `handlers_plugins.go` — UI at `/plugins`, API at `/api/plugins/*`
- Execute: `POST /api/plugins/:id/execute`

## vs JSON plugins

| Type | Location | Use |
|------|----------|-----|
| Manifest | `plugins/*/manifest.yaml` | Python/Go with params |
| JSON sample | `plugins/samples/*.json` | PowerShell script templates |

See `plugin-task` skill for JSON plugins.

## Verify

- Server starts without plugin load errors
- Plugin appears in `/plugins` UI
- Execute returns output or structured error