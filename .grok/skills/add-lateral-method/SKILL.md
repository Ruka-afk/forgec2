---
name: add-lateral-method
description: Add ForgeC2 lateral movement methods (WMI, WinRM, PsExec, DCOM, etc.). Use for lateral movement, pass-the-hash, or /add-lateral-method.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: feature
---

## When to use

Add a new lateral movement technique beyond generic `add-task-type` (Windows-specific execution path).

## Key files

| Layer | File |
|-------|------|
| UI | `templates/lateral.html`, `static/js/lateral.js` (`toolkit.bundle.js`) |
| Server | `handlers_lateral.go` — page, history, result processing |
| Agent WMI | `agent/lateral_wmi_windows.go` |
| Agent WinRM | `agent/lateral_winrm_windows.go` |
| Agent PsExec | `agent/lateral_psexec_windows.go` |
| Agent DCOM | `agent/lateral_dcom_windows.go` |
| Agent SCF | `agent/lateral_scf_windows.go` |
| Registry | `agent/task_registry.go` + `task_lateral.go` |

## Checklist

1. **Agent**: implement `handleLateralYourMethod(task)` on Windows; stub on Linux/macOS.
2. **Registry**: map task type string (e.g. `lateral_wmi`) in `task_registry.go`.
3. **Server**: `handleProcessLateralResult` parses agent output → history DB.
4. **UI**: add method to lateral form `<select>`; wire `data-action="execute-lateral"` in `lateral.js`.
5. **Credentials**: lateral form uses credential vault — pass hash/user from `handlers_credentials.go`.
6. **Audit**: `logAction` on task creation.

## UI style

Use `ui-card`, `ui-select`, `ui-input` — see `fix-ui-style` skill.

## Verify

- Select source agent + target + method → task queued
- History panel shows result
- Failed auth shows actionable error in task output