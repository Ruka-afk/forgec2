---
name: e2e-smoke-test
description: Run ForgeC2 end-to-end smoke tests on the single-binary server (:8000)
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: test
---

## Prerequisites

```powershell
powershell -File scripts\build-embedded.ps1
# or manually start the binary: .\forgec2-server.exe -config config.yaml
```

## Smoke checklist

| # | Step | Expected |
|---|------|----------|
| 1 | `GET http://127.0.0.1:8000/health` | `{"status":"ok"}` |
| 2 | `GET http://127.0.0.1:8000/login` | HTTP 200 (SPA HTML) |
| 3 | Login admin/admin on `:8000` | Redirect to dashboard |
| 4 | `GET http://127.0.0.1:8000/dashboard` | Dashboard renders (API fetched same-origin) |
| 5 | `GET http://127.0.0.1:8000/agents` | Table renders, batch bar on select |
| 6 | WebSocket | DevTools WS to `ws://127.0.0.1:8000/ws` — Live indicator in top bar |
| 7 | Theme toggle | Light/dark/system persists after reload |
| 8 | Language switch | UI strings change (en/zh) |
| 9 | Implant beacon | `data/server.log` shows beacon processed |
| 10 | `go test ./...` | All pass |

## PowerShell quick script

```powershell
# Prefer the packaged smoke script
powershell -File scripts/api-smoke.ps1
# Auth suite: session + CSRF + read-only agents/modules/listeners/dashboard/profiles/attack
powershell -File scripts/api-smoke.ps1 -TryDefaultAdmin
powershell -File scripts/api-smoke.ps1 -User admin -Password 'your-password'
# Or env: $env:FORGEC2_SMOKE_USER='admin'; $env:FORGEC2_SMOKE_PASS='...'

# OpenAPI hygiene (from repo root)
go run ./cmd/checkopenapi -min-coverage 0.80
go run ./cmd/checkopenapi -list-stale   # OpenAPI-only orphans
go run ./cmd/checkopenapi -list-missing # backend routes still undocumented

# Manual one-liners
(Invoke-WebRequest "http://127.0.0.1:8000/health" -UseBasicParsing).Content
(Invoke-WebRequest "http://127.0.0.1:8000/login" -UseBasicParsing).StatusCode
```

## After code changes

1. `go test ./...`
2. `powershell -File scripts\build-embedded.ps1` (if UI or Go changed)
3. `powershell -File scripts\dev-backend.ps1` (if only Go changed)
4. Hard refresh browser (Ctrl+Shift+R)
5. Re-run checklist

## CI

GitHub Actions `.github/workflows/ci.yml` runs frontend build → embed → Go build/vet/test → final binary.

## Ports reference

| URL | Purpose |
|-----|---------|
| `http://localhost:8000` | **Single binary — UI, API, beacons, WebSocket** |
