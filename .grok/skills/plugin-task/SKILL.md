---
name: plugin-task
description: Create JSON-defined ForgeC2 custom task plugins (PowerShell script, parameters, plugins/samples). Use for JSON plugins or /plugin-task.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: feature
---

## File location

`plugins/samples/<name>.json`

## Format

```json
{
  "name": "Plugin Name",
  "description": "Description",
  "version": "1.0.0",
  "author": "ForgeC2",
  "task_type": "unique_task_type",
  "script": "function Do-Work { param($Target) ... }; Do-Work -Target '{{target}}'",
  "category": "recon",
  "icon": "fa-solid fa-search",
  "color": "indigo",
  "requires_elevated": false,
  "parameters": [
    {
      "name": "target",
      "type": "string",
      "description": "Target host",
      "required": true
    }
  ]
}
```

## Parameter types

`string`, `select` (+ `options`), `number`, `boolean`

## Script rules

- Use `{{param_name}}` placeholders matching `parameters[].name`
- Return JSON via `ConvertTo-Json` for structured results

## Samples (v2.1+)

- `plugins/samples/port_scan.json`
- `plugins/samples/env_collect.json`
- `plugins/samples/process_list.json`

## vs manifest plugins

JSON = PowerShell templates. Manifest = `plugins/*/manifest.yaml` with Python/Go. See `add-manifest-plugin`.

## Verify

- Valid JSON, unique `task_type`
- Server starts, plugin listed in `/plugins`
- Execute from UI returns output