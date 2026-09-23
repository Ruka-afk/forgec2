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
- Version: 2.6.1 (see CHANGELOG.md).

## Frontend build & embedding (IMPORTANT)
The frontend is **Vite** (not Next.js).
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
- `go build ./...` passes (the `data/e2e/` helpers have `//go:build ignore` tags).
- Go version: **1.25.0** (go.mod).
- sqlite driver is **`github.com/glebarez/sqlite`** (pure Go, no cgo). Any DB-verification temp program MUST use this driver, NOT `gorm.io/driver/sqlite` (which needs cgo).

## CSRF Protection (IMPORTANT)
- Login (`POST /api/login`) now sets `forgec2_csrf` cookie to the session-derived token (`DeriveCSRFToken(session, csrf_key)`), not a random value.
- `CSRFProtect` middleware validates **both** `X-CSRF-Token` header **and** `forgec2_csrf` cookie against the derived token (true double-submit).
- On GET/HEAD, the middleware rotates the cookie to the derived token.
- Frontend `src/lib/api.ts` reads the cookie via `readCsrfCookie()` and echoes it as `X-CSRF-Token` on mutations.
- E2E automation must: login → GET `/api/v1/tasks` (or any authenticated GET) → POST with `X-CSRF-Token` = cookie value.

## C-Implant E2E
- One-click script: `scripts/e2e-c-implant.bat <HOST> <PORT> <USER> <PASS> <AGENT_ID> [TASK_TYPE] [COMMAND]`
- Supported task types: `shell` (default), `wechat_history`
- Release build: `proto/c-implant/build-release.bat` (uses `-Os -s -Wl,--strip-all -ffunction-sections -fdata-sections -Wl,--gc-sections`)
- Local E2E build: `proto/c-implant/build-e2e-local.bat` (gitignored; takes `SECRET_ID`/`SECRET_B64` from env vars or args — no secret is stored in the file)
- Tracked template `build-e2e.bat` contains placeholders only (`YOUR_SECRET_ID`, `YOUR_SECRET_B64`, port 8001).

## Reset Password
CLI tool: `go run ./cmd/reset-password -db data/db/forgec2.db -user <user> -pass <newpass>`
Or build: `go build -o reset-password.exe ./cmd/reset-password`

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
- Frontend source lives under `frontend/src/pages/` and `frontend/src/lib/`; i18n strings in `frontend/src/lib/i18n/en.ts` and `zh.ts`.