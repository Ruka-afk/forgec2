# ForgeC2 — Agent Guide

## Build & Quick Commands

```powershell
# Full verification (do this before committing):
go build ./... ; go vet ./internal/... ./pkg/... ./cmd/... ; go test ./internal/... -count=1 -timeout 5m

# Frontend verification (inside frontend/):
npm run build             # Next.js static export -> frontend/out
npm run gen:openapi       # regenerate src/lib/api-schema.d.ts after api/openapi.yaml changes
npm run check             # css + i18n(en/zh) + api-paths + openapi-types + webdist + bundle gates
npx tsc --noEmit          # type check (noUnusedLocals is OFF: unused imports are NOT flagged — remove them by hand)
npm test                  # vitest unit tests

# Backend-only dev loop:
powershell -File scripts\dev-backend.ps1

# Full rebuild (frontend + backend):
powershell -File scripts\build-embedded.ps1
```

## Architecture

- **Module**: `github.com/forgec2/forgec2` (Go 1.25, Gin + GORM + SQLite)
- **Entrypoint**: `cmd/server/main.go`
- **Implant source**: `internal/payload/agent/` (Windows/Linux/macOS)
- **Frontend**: Next.js 16, `frontend/` directory, embedded via `//go:embed all:dist` into Go binary
- **Config**: `config.yaml` at root (also `config.example.yaml` for CI)
- **OpenAPI spec**: `api/openapi.yaml`, served at `/api/docs`, validated by `cmd/checkopenapi`
- **Initial account**: username `admin`; password auto-generated on first boot if `auth.default_password` is empty (weak passwords like `admin` are rejected by `isWeakDefaultPassword`)

## API & Routing

| Group | Base Path | Auth |
|-------|-----------|------|
| Auth (login/logout) | `/` | None |
| Beacon | `/api/v1/beacon` | Implant encryption |
| REST API | `/api/` | Session cookie |
| Static/SPA | `/*` (catch-all) | None |

**Frontend API calls** must use `buildUrl("/api/...")` from `@/lib/api.ts` — never hardcode `/api/v1/` paths.

**CSRF**: Double-submit cookie. Token in `forgec2_csrf` cookie. Send as `X-CSRF-Token` header on all state-changing requests from authenticated routes.

## Testing

- **In-memory SQLite**: `testutil.SetupTestDB(t)` auto-migrates all models
- **Server tests**: `<handler>_test.go` in `internal/server/`, e.g., `handlers_beacon_test.go`
- **Fuzz tests**: `handlers_beacon_fuzz_test.go`
- **Always use** `-count=1` to disable test caching
- **CI test scope**: `go test ./internal/config/... ./internal/crypto/... ./internal/db/... ./internal/logger/... ./internal/malleable/... ./internal/obfuscation/... ./internal/plugin/... ./internal/report/... ./internal/server/... ./pkg/... -count=1 -timeout 5m`

## Known Gotchas

1. **go vet unsafe.Pointer**: All warnings in `internal/payload/agent/` are expected (Windows syscall patterns). Filter with `go vet ./... 2>&1 | Select-String -NotMatch "internal[\\/]payload[\\/]agent"`
2. **golangci-lint**: Enabled linters (errcheck, gosimple, govet, ineffassign, staticcheck, unused, gosec, misspell, unconvert). Gosec excludes G115/G101/G107/G204/G301/G302/G304. Errcheck excludes Close calls by default.
3. **DB concurrency**: SQLite supports one writer — reduce `MaxOpenConns` if lock contention
4. **Handler patterns**: Use `slog.Error("msg", "error", err)` for error logging, check all DB/JSON errors
5. **Frontend check**: `npm run check` validates CSS (Tailwind v4 PostCSS) and i18n key coverage (en/zh)
6. **Login CSRF**: Login page does NOT require CSRF (public route). CSRF is enforced only on authenticated routes.
7. **Frontend embed sync**: `check:webdist` compares `frontend/out` to the embedded `internal/webdist/dist`. After editing frontend source, run `powershell -File scripts/build-embedded.ps1` (builds + copies to webdist + builds Go). Plain `npm run build` leaves `check:webdist` failing because the embed is stale.
8. **tsc unused imports**: `noUnusedLocals` is not enabled, so dead imports pass `tsc` silently. Before deleting an exported helper, grep the whole `src` tree and drop now-unused imports by hand.
9. **OpenAPI contract types**: `frontend/src/lib/api-schema.d.ts` is GENERATED from `api/openapi.yaml` via `npm run gen:openapi` (openapi-typescript) and is compile-time only — unlike the removed Zod `lib/api-schemas.ts`, it adds zero runtime bytes. Never hand-edit it; `check:openapi-types` (part of `npm run check`) fails if it drifts from the spec. After any spec edit, run `npm run gen:openapi` from `frontend/`. Contract-backed fetchers/DTO aliases live in `src/lib/typed-api.ts` — reuse it instead of hand-typing `api.get<{...}>` inline.

## i18n Policy

- **Only two locales are supported: `en` and `zh`.** Do NOT add any other language (es, fr, de, ja, etc.). All UI strings must be present in BOTH `frontend/src/lib/i18n/en.ts` and `frontend/src/lib/i18n/zh.ts`, or `npm run check:i18n` will fail.
- **i18n files are UTF-8, no BOM**: Never use PowerShell `Get-Content`/`Set-Content` (or any GBK-encoding write) on `zh.ts` — it corrupts Chinese and adds a BOM. Edit via the Edit tool or Node `fs` (`fs.readFileSync`/`writeFileSync`). Gate: `npm run check:i18n` enforces used-key consistency across en/zh.
- New UI strings must go through `t("key")` — no hardcoded English literals in JSX; add the key to both locales in the same change.

## Frontend Conventions

- **Page layout**: All standard `(main)` pages wrap content in `PageContainer` (`@/components/ui/page-container.tsx`) instead of a raw `<div className="max-w-(--content-width) mx-auto pb-12 …">` + `<PageHeader>`. Pass `title`/`subtitle` and use the `actions` slot for header buttons.
  - **Embedded sub-views** (panel-rendered page-contents such as `tasks`/`builds`/`profiles`/`packer`/`stager`/`notifications`) pass `embedded={embedded}` and render the header conditionally: `title={!embedded ? t("x.title") : undefined}` so the header is omitted when embedded.
  - **Intentionally full-bleed (do NOT wrap)**: `agents/[id]/{AgentDetailPage,files,persistence,remote-desktop,screen,shell}` and `chat/page` are terminal/chat views under the passthrough `agents/[id]/layout.tsx`.
  - After migrating a page to `PageContainer`, delete its now-unused `PageHeader` import (PageContainer renders `PageHeader` internally). Never import `PageHeader` for a page that does not render it.
- **Shared state components**: Use `EmptyState` (`@/components/ui/empty-state.tsx`) for empty lists, `ErrorState` (`@/components/ui/error-state.tsx`) for error alerts (icon + title + message + optional `action`), and `Banner` (`@/components/ui/banner.tsx`, `tone` + optional `icon`/`action`) for success/warning/info/destructive result strips. Do NOT hand-roll inline `<div className="…bg-destructive/10…">` boxes or ad-hoc success/error banners.
- **Confirmations**: Use `ConfirmModal` (`@/components/ui/confirm-modal.tsx`) or the Promise-based `useConfirm()` hook (`@/lib/hooks/useConfirm.tsx`) for delete/destructive confirmations. Never use the native `window.confirm`.
- **Loading states**: A single route-level spinner lives at `frontend/src/app/(main)/loading.tsx`. Per-route `loading.tsx` is kept ONLY for custom skeletons: `dns`, `groups`, `listeners`, `tags`, `tasks`. Do NOT add a `loading` fallback prop to page components or create new generic `loading.tsx`.
- **Design tokens**: Prefer semantic classes / CSS variables (`text-destructive`, `bg-success`, `border-border`, chart palette via `--chart-1..6`). Avoid raw hex colors and hardcoded `emerald/amber/red/blue/rose/orange` status shades except inside `vis.js` graph configs (`TopologyGraph.tsx`). Keep buttons at the default `rounded-lg` (do not override with `rounded-xl`).
- **Dead code**: Before deleting an exported helper, grep the whole `src` tree; remove callers and now-orphaned module vars together. Keep only exports that are actually referenced (tests included — pure functions tested directly, like `esc`/`NAV_BY_HREF`, keep their `export`); prefer removing the `export` keyword over deleting a function that a unit test validates directly. `lib/api-schemas.ts` was removed (round-5) — do not re-add Zod response schemas unless a caller appears.
