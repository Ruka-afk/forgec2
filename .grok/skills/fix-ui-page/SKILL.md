---
name: fix-ui-page
description: Fix ForgeC2 Next.js UI page issues — button handlers, API wiring, tabs. Use for frontend bugs on :3000.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: debug
---

## When to use

Button clicks do nothing, API calls fail, tabs don't switch, or page changes don't appear after edits on the **Next.js UI** (`:3000`).

**CSS / theme issues** → use `fix-ui-style` skill.

## Architecture

| Layer | Path |
|-------|------|
| Pages | `frontend/src/app/<route>/page.tsx` |
| Components | `frontend/src/components/` |
| API client | `frontend/src/lib/api.ts` (`apiGet`, `apiPostJson`, `apiSend`, `apiDelete`) |
| Proxy | `frontend/src/app/api/go/route.ts` → Go `:8080` |
| i18n | `frontend/src/lib/i18n.tsx` |
| Theme | `frontend/src/lib/theme.tsx` |

## Button / action checklist

1. **Handler**: wire `onClick` or form `onSubmit` in the page component (React, not `data-action`).
2. **API call**: use `apiGet("/path")` or `apiPostJson("/path", body)` — paths match Go routes (e.g. `/agents/batch`).
3. **Credentials**: `api.ts` helpers include `credentials: "include"` for session cookies.
4. **Feedback**: toast, `actionMsg` banner, or `ConfirmModal` from `@/components/UI`.
5. **Reload data**: call your `load*` function after mutation, don't rely on `location.reload()`.

## Tabs checklist

- Use React state: `const [tab, setTab] = useState("overview")`
- Toggle panels with conditional render or `hidden` class
- Sub-routes alternative: `/agents/[id]/shell`, `/files`, `/tasks` as separate pages

## After code changes

```powershell
cd frontend
npm run build    # production
# or npm run dev  # development (hot reload)
```

If Go handler changed too: `go build -o forgec2-server.exe ./cmd/server` and restart API.

## Legacy Go templates

Old `data-action` + `layout.js` pattern still exists in `internal/server/templates/static/js/` for `:8080` fallback. **Do not use for new UI work** — edit Next.js instead.

## Verify

- DevTools → Network: `/api/go?p=...` returns 200
- No console errors on click
- Hard refresh if testing production build (`next start`)