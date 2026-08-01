# ForgeC2 — Agent Guide

## Build & Quick Commands

```powershell
# Full verification (do this before committing):
go build ./... ; go vet ./internal/... ./pkg/... ./cmd/... ; go test ./internal/... -count=1 -timeout 5m

# Frontend verification:
cd frontend; npm run build; npm run check
npx tsc --noEmit          # TypeScript check

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
