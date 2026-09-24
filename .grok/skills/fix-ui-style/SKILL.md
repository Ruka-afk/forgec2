---
name: fix-ui-style
description: Fix ForgeC2 UI styling — Tailwind tokens, dark mode, theme toggle, layout consistency
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: debug
---

## When to use

Pages look unstyled, dark mode broken, theme toggle missing, or layout inconsistent between dashboard and toolkit pages on **`:3000`**.

## CSS architecture (Vite + Tailwind v4)

| Layer | File | Role |
|-------|------|------|
| Tailwind | `frontend/src/styles/globals.css` | Tailwind v4 via PostCSS; design tokens live here |
| Primitives | `frontend/src/components/ui/` | shadcn/ui (base-nova, `@base-ui/react`) |
| Dark mode | `.dark` class on `<html>` | Toggled by `src/lib/theme.tsx`, hydrated by the inline script in `frontend/index.html` |

> Do NOT use a CDN Tailwind build or Font Awesome; the project builds Tailwind
> locally and uses lucide-react icons (`size-4` utility, see
> `docs/frontend-style-conventions.md`).

**Dashboard reference pattern:**

```
bg-card text-foreground border border-border rounded-lg p-(--card-spacing) shadow-sm
```

## Theme toggle

| File | What to check |
|------|---------------|
| `frontend/src/lib/theme.tsx` | `ThemeProvider`, `localStorage forgec2_theme` |
| `frontend/index.html` | Inline theme-init script (avoids a flash before hydration) |
| `frontend/src/components/TopBar.tsx` | Light / Dark / System menu |
| `frontend/src/components/ClientProvider.tsx` | Wraps `ThemeProvider` |

## Language

| File | Role |
|------|------|
| `frontend/src/lib/i18n/index.tsx` | `useI18n()`, `setLocale`, `t()` |
| `frontend/src/lib/i18n/en.ts` / `zh.ts` | Locale blocks — keys must exist in BOTH |

## Fix checklist

| Step | Action |
|------|--------|
| 1 | Match existing page patterns in `frontend/src/pages/dashboard/` |
| 2 | Use design tokens for color (`--primary`, `--chart-*`); raw hex is blocked by `check:tokens` |
| 3 | Add missing i18n keys to BOTH `en.ts` and `zh.ts` |
| 4 | `cd frontend && npm run verify` |
| 5 | Hard refresh browser |

## Common symptoms

| Symptom | Fix |
|---------|-----|
| No dark mode | Check `ThemeProvider` + `document.documentElement.classList` |
| Raw i18n keys shown | Add the key to both locale blocks; `check:i18n` lists what's missing |
| Sidebar overlap | `AppLayout` owns the shell; don't add page-level offsets |

## Verify

- `/dashboard` and `/toolkit` cards look consistent in light and dark
- Theme toggle persists after reload
- `npm run verify` is green