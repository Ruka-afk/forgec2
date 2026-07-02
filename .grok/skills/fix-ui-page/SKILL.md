---
name: fix-ui-page
description: Fix ForgeC2 UI page issues — button binding, tabs, and bundle refresh checklist
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: debug
---

## When to use

Button clicks do nothing, tabs fail to switch, or UI changes do not appear after edits.

**CSS / styling issues** (missing cards, wrong colors, theme toggle) → use `fix-ui-style` skill instead.

## Button binding checklist

ForgeC2 uses a delegated action system in `internal/server/templates/static/js/layout.js`:

1. **HTML**: add `data-action="your_action"` on the button, link, or form.
2. **JS**: register handler in `window.GlobalActionHandlers`:

```js
window.GlobalActionHandlers = window.GlobalActionHandlers || {};
window.GlobalActionHandlers.your_action = function(el, e) {
    // read el.dataset.* for parameters
};
```

3. **Avoid inline `onclick`** for new code — prefer `data-action` so `initGlobalActionHandler()` picks it up.
4. **Forms**: set `data-action` on the `<form>`; submit is intercepted automatically.
5. **CSRF**: POST requests need `X-CSRF-Token` header (read from cookie `csrf_token`).

## Tabs checklist

Common pattern (see `agent-detail.js`):

- Tab buttons: `class="tab-btn"` + `data-tab="overview"`
- Tab panels: `id="tab-overview"` + `class="tab-content hidden"`
- Switch function: `switchTab(tab)` toggles `.hidden` and active styles on `.tab-btn[data-tab="..."]`
- Export to `window.switchTab` if called from inline HTML
- On `DOMContentLoaded`, call `switchTab(defaultTab)` for the initial panel

## Bundle refresh checklist

| Step | Action |
|------|--------|
| 1 | Edit source in `internal/server/templates/static/js/<page>.js` |
| 2 | Register page in `pageScriptMap` (`internal/server/page_assets.go`) if new page |
| 3 | Run `powershell -File build_js.ps1` (or `make build-js`) |
| 4 | Hard-refresh browser (Ctrl+Shift+R) |
| 5 | If still stale, use `make dev` (`FORGEC2_DEV=1`) to load unbundled scripts |

## Handler / template checks

- Handler must call `s.renderPage(c, "<name>_content", data)` with all template fields
- Missing `{{tr .Lang "key"}}` keys show raw key names — add to `locales.go`
- Check browser DevTools → Network for 4xx/5xx on API calls

## Verify

- Click handlers fire (no console errors)
- Tabs switch and correct panel is visible
- Bundled JS timestamp/size changes after rebuild