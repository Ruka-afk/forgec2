---
name: fix-ui-page
description: Fix ForgeC2 UI page issues — button handlers, API wiring, tabs. Use for frontend bugs on the Vite SPA.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: debug
---

## When to use

Button clicks do nothing, API calls fail, tabs don't switch, or page changes don't appear after edits on the **frontend** (`src/pages/**`).

**CSS / theme issues** → use `fix-ui-style` skill.

## Architecture

| Layer | Path |
|-------|------|
| Pages | `frontend/src/pages/<route>/<Page>.tsx` (registered in `src/router.tsx`) |
| Components | `frontend/src/components/` (page-private ones live in `src/pages/<route>/components/`) |
| API client | `frontend/src/lib/api.ts` (`api.get`, `api.post`, `api.postJson`, `api.del`) |
| API paths | `frontend/src/lib/api-paths.ts` (`paths.*`) — enforced by `check:api-paths` |
| i18n | `frontend/src/lib/i18n/index.tsx` + `en.ts` / `zh.ts` |
| Theme | `frontend/src/lib/theme.tsx` |

## Button / action checklist

1. **Handler**: wire `onClick` or form `onSubmit` in the page component (React, not `data-action`).
2. **API call**: use `api.get(paths.your.endpoint)` / `api.postJson(paths.your.action, body)` — paths must live in `api-paths.ts`, not inline strings.
3. **Credentials**: `api.ts` helpers send session cookies and the CSRF header on mutations.
4. **Feedback**: `toast` from sonner for transient results, `<Banner>` for persistent status, `useConfirm()` for destructive actions.
5. **Reload data**: call your `load*` function after mutation, don't rely on `location.reload()`.

## Tabs checklist

- Use React state: `const [tab, setTab] = useState("overview")`
- Toggle panels with conditional render or `hidden` class
- Sub-routes alternative: `/agents/[id]/shell`, `/files`, `/tasks` as separate pages

## After code changes

```powershell
cd frontend
npm run verify   # typecheck + lint + vitest + every check:* gate
```

Then sync the embedded bundle before pushing (`scripts/build-embedded.ps1` from
the repo root) — the `check:webdist` pre-push hook fails otherwise.

If a Go handler changed too: `go build -o forgec2-server.exe ./cmd/server` and restart the API.

## Verify

- DevTools → Network: the `/api/...` request returns 200 and the UI updates
- No console errors on click
- `npm run verify` is green