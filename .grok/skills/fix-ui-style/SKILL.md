---
name: fix-ui-style
description: Fix ForgeC2 UI styling — dashboard alignment, ui-card/ui-input CSS, theme toggle, bundle refresh. Use when CSS missing, styles inconsistent, link underlines, theme/lang toggle broken, or /fix-ui-style.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: debug
---

## When to use

Toolkit pages (lateral, privesc, report, plugins, AI) look unstyled while Dashboard/Agents look correct; theme or language menu does not work; CSS flashes on navigation.

## CSS architecture

| Layer | File | Role |
|-------|------|------|
| JIT utilities | `templates/static/js/tailwind.min.js` in `<head>` | Generates Tailwind classes, preflight (no link underlines) |
| Component CSS | `templates/static/css/layout.css` | `.ui-card`, `.ui-input`, `.ui-select`, `.ui-modal`, `.ui-btn` |
| Bundle | `templates/static/css/app.bundle.css` | `layout.css` + `skeleton.css` + `lazyload.css` (via `build_js.ps1`) |

**Dashboard reference pattern** (inline Tailwind):

```
bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-2xl p-4 shadow-sm
```

**Toolkit pattern** (component classes — must exist in `layout.css`):

```html
<div class="ui-card p-5 shadow-sm">...</div>
<input class="ui-input w-full">
```

Do **not** add `tailwind-full.css` to the bundle without explicit approval (causes flash/underline regressions).

## Fix checklist

| Step | Action |
|------|--------|
| 1 | Add missing `.ui-*` rules to `layout.css` matching Dashboard colors |
| 2 | Run `powershell -File build_js.ps1 -SkipJS` to rebuild `app.bundle.css` |
| 3 | Run `go build -o server.exe ./cmd/server/` (templates/CSS are go:embedded) |
| 4 | Restart server; hard-refresh browser (Ctrl+Shift+R) |

## Theme / language toggle

| File | What to check |
|------|---------------|
| `layout.html` | `tailwind.min.js` in `<head>`; inline dark-class bootstrap script |
| `core.js` | `handleThemeSelect`, `handleLanguageSelect` fallbacks |
| `layout.js` | `GlobalActionHandlers.set_theme`, `GlobalActionHandlers.set_language`; `closeTopBarMenus` |
| Menu buttons | `data-action="set_theme"` / `data-action="set_language"` with `data-theme` / `data-lang` |

## Common symptoms

| Symptom | Cause | Fix |
|---------|-------|-----|
| Cards have no border/bg | `ui-card` missing from bundle | Add to `layout.css`, rebuild bundle |
| Inputs look like plain HTML | `ui-input` missing | Same |
| Link underlines everywhere | `tailwind.min.js` removed from head | Restore in `layout.html` |
| Two styles flicker | Both JIT + full Tailwind CSS loaded | Use JIT + small bundle only |
| Modals transparent | `ui-modal-backdrop` / `ui-modal` missing | Add to `layout.css` |

## Verify

- `/dashboard` and `/lateral` cards look visually consistent
- Theme toggle switches light/dark without reload errors
- `app.bundle.css` contains `.ui-card` (grep or Network tab)