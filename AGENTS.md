# AGENTS.md

Instructions for AI coding agents working in this repository.

## Important: This is a Windows PowerShell 5.1 environment
The `bash` tool runs **PowerShell 5.1**, not bash. Adapt accordingly:
- `head`, `grep`, `rm`, `&&` DO NOT work. Use `Select-Object -First N`, `Select-String`, `Remove-Item`, and chain sequential dependent commands with `if ($?) { ... }`.
- Use `-LiteralPath` for paths containing brackets/spaces.
- Do NOT use `Set-Content`/`Get-Content` on Go source files (corrupts UTF-8). Use the dedicated edit/write tools instead.
- Prefer full cmdlet names (`Get-ChildItem`, `Remove-Item`) over aliases.
- `cd` is discouraged; use the `workdir` parameter of the shell tool instead.

## Project overview
ForgeC2 is a mature Go (net/http + sqlite) C2 server plus a Vite + React TypeScript frontend.
- Server binary: `forgec2-server.exe` (built with `go build ./cmd/server`).
- Default: port **8000**, config `config.yaml`, sqlite DB `data/db/forgec2.db` (users `admin`/`labtest`).
- Version: 2.5.0.

## Frontend build & embedding (IMPORTANT)
The frontend is **Vite**, NOT Next.js (README is inaccurate on this).
- Build: `npm run dev` for dev; production: `npm run build` → outputs to `frontend/out`.
- Frontend is embedded into the Go server via `//go:embed all:dist` in `internal/webdist/webdist.go`, which embeds the directory `internal/webdist/dist`.
- **Canonical sync script**: `scripts/build-embedded.ps1` (uses `Remove-Item` + `Copy-Item -Recurse frontend/out/*` → `internal/webdist/dist`; includes assets). Use this, NOT robocopy directly.
- After syncing, verify with `scripts/check-webdist.mjs` (expects "OK: matches frontend/out").

## Frontend verification
Run from `frontend/`:
- `npm run build` (vite)
- `npm run check` (strict gate running all `check:*` scripts)
- `npm run test` (vitest)
- `npm run lint` (eslint)
- `npx tsc --noEmit` (typecheck)
- `npm run gen:openapi` regenerates `src/lib/api-schema.d.ts`

## Testing
Backend test suite via CI (`.github/workflows/ci.yml`):
```
go test ./internal/config/... ./internal/crypto/... ./internal/db/... ./internal/malleable/... ./internal/obfuscation/... ./internal/plugin/... ./internal/server/... ./pkg/... -count=1 -timeout 5m
```
- `go vet ./internal/... ./pkg/... ./cmd/...` — but **ignore `internal/payload/agent`** (known unsafe.Pointer warnings; also filtered in CI).
- CI requires **gofmt** on all changed Go files.

## Build caveats
- `go build ./...` has a **pre-existing failure** in `data/e2e/` (`reset_lab_user.go:30 func main()` and `gen_lab_exe.go:33 func main()` redeclare `main`) — unrelated to normal changes. Build the server binary with `go build ./cmd/server` only.
- Go version: **1.25.0** (go.mod).
- sqlite driver is **`github.com/glebarez/sqlite`** (pure Go, no cgo). Any DB-verification temp program MUST use this driver, NOT `gorm.io/driver/sqlite` (which needs cgo).

## Deploying the server
1. `npm run build` (from `frontend/`)
2. Sync dist via `scripts/build-embedded.ps1`
3. `go build -o forgec2-server.exe ./cmd/server`
4. Kill running instance: `taskkill /f /im forgec2-server.exe`
5. Start detached (NO redirect flags — they break the process):
   `Start-Process -WindowStyle Hidden -FilePath ".\forgec2-server.exe" -ArgumentList "-config config.yaml" -PassThru`
6. Logs go to `logs/forgec2.log`.
7. Verify with `GET /health` (expects `ok`).

## Misc
- `scripts/setup-dev.sh` generates `config.yaml` (port 8000).
- `scripts/gitleaks-pre-commit.sh` runs a gitleaks scan pre-commit.
- Frontend source lives under `frontend/src/app/(main)/` and `frontend/src/lib/`; i18n strings in `frontend/src/lib/i18n/en.ts` and `zh.ts`.
