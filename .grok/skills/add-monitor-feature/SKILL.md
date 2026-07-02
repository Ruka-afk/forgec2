---
name: add-monitor-feature
description: Add ForgeC2 monitoring features (screenshot, keylogger, screen stream). Use for screenshot, keylog, screen monitor, or /add-monitor-feature.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: feature
---

## When to use

Extend screenshot, keylogger, clipboard, or live screen monitoring.

## Key files

| Feature | Server | Agent | UI |
|---------|--------|-------|-----|
| Screenshot | `handlers_agents.go` `handleRequestScreenshot` | `task_recon.go` | agent detail |
| Window shot | `handleRequestScreenshotWindow` | Windows capture API | agent detail |
| Keylogger | `handleStartKeylogger` / `Stop` / `Dump` | agent keylog handlers | agent detail |
| Screen stream | `handlers_monitor.go` | screen capture loop | `screen.html`, `screen.js` |
| Remote input | `handleAgentRemoteInput` | `remote_input_windows.go` | `remote-desktop` skill |

## Add monitor command checklist

1. Route in `server.go` under `agentCmd` group: `POST /agents/:id/your_monitor`.
2. Handler creates task with type + args JSON.
3. Agent handler captures data → returns base64 or JSON in task result.
4. Server stores artifact if large (files table) or returns inline.
5. UI button with `data-action` + poll task status.

## Screen monitor page

- `templates/screen.html` — stream viewer
- `static/js/screen.js` in `admin.bundle.js`
- Poll or WS for frame updates; respect agent interval

## Verify

- Screenshot task completes → image visible in UI
- Keylog start/stop/dump cycle works
- Screen page loads frames from online Windows agent