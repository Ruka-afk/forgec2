---
name: fix-ui-style
description: Fix ForgeC2 Next.js UI styling — Tailwind, dark mode, theme toggle, layout consistency
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: debug
---

## When to use

Pages look unstyled, dark mode broken, theme toggle missing, or layout inconsistent between dashboard and toolkit pages on **`:3000`**.

## CSS architecture (Next.js)

| Layer | File | Role |
|-------|------|------|
| Tailwind JIT | `frontend/public/js/tailwind.min.js` | Utility classes via `<Script>` in `app/layout.tsx` |
| Global CSS | `frontend/src/app/globals.css` | Base styles |
| Legacy CSS | `frontend/public/css/layout.css` | `.ui-card`, `.nav-active`, shared with Go templates |
| Dark mode | `darkMode: 'class'` | Toggle `dark` on `<html>` via `frontend/src/lib/theme.tsx` |

**Dashboard reference pattern:**

```
bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-2xl p-4 shadow-sm
```

## Theme toggle

| File | What to check |
|------|---------------|
| `frontend/src/lib/theme.tsx` | `ThemeProvider`, `localStorage forgec2_theme` |
| `frontend/src/app/layout.tsx` | Inline `theme-init` script (hydration) |
| `frontend/src/components/TopBar.tsx` | Light / Dark / System menu |
| `frontend/src/components/ClientProvider.tsx` | Wraps `ThemeProvider` |

## Language / RTL

| File | Role |
|------|------|
| `frontend/src/lib/i18n.tsx` | `setLocale`, `dir` for Arabic RTL |
| `frontend/src/components/TopBar.tsx` | Language menu |

## Fix checklist

| Step | Action |
|------|--------|
| 1 | Match existing page patterns in `frontend/src/app/dashboard/page.tsx` |
| 2 | Use `dark:` variants for all bg/border/text |
| 3 | Add missing i18n keys to `frontend/src/lib/i18n.tsx` |
| 4 | `cd frontend && npm run build` (or `npm run dev`) |
| 5 | Hard refresh browser |

## Common symptoms

| Symptom | Fix |
|---------|-----|
| No dark mode | Check `ThemeProvider` + `document.documentElement.classList` |
| Raw i18n keys shown | Add key to all locales in `i18n.tsx` |
| Sidebar overlap | `AppLayout.tsx` uses `ml-56` + fixed sidebar |
| Font Awesome missing | `layout.tsx` links `/css/font-awesome.min.css` |

## Legacy note

Go template styling (`internal/server/templates/static/css/`) applies only to `:8080` HTML fallback. Primary UI is Next.js.

## Verify

- `/dashboard` and `/toolkit` cards look consistent in light and dark
- Theme toggle persists after reload
- Arabic locale sets `dir="rtl"` on `<html>`