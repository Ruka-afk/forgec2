---
name: e2e-smoke-test
description: Run ForgeC2 end-to-end smoke tests (Next.js UI on :3000, Go API on :8080)
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: test
---

## Prerequisites

```powershell
powershell -ExecutionPolicy Bypass -File .\start.ps1
# or manually start Go :8080 + Next.js :3000
```

## Smoke checklist

| # | Step | Expected |
|---|------|----------|
| 1 | `GET http://127.0.0.1:8080/health` | `{"status":"ok"}` |
| 2 | `GET http://127.0.0.1:3000/login` | HTTP 200 |
| 3 | Login admin/admin on `:3000` | Redirect to dashboard |
| 4 | `GET http://127.0.0.1:3000/dashboard` | Charts load (network → `/api/go?p=/api/dashboard/...`) |
| 5 | `GET http://127.0.0.1:3000/agents` | Table renders, batch bar on select |
| 6 | WebSocket | DevTools WS to `ws://127.0.0.1:8080/ws` — Live indicator in top bar |
| 7 | Theme toggle | Light/dark/system persists after reload |
| 8 | Language switch | UI strings change (en/zh) |
| 9 | Implant beacon | `data/server.log` shows beacon processed |
| 10 | `go test ./...` | All pass |

## PowerShell quick script

```powershell
# API health
(Invoke-WebRequest "http://127.0.0.1:8080/health" -UseBasicParsing).Content

# UI reachable
(Invoke-WebRequest "http://127.0.0.1:3000/login" -UseBasicParsing).StatusCode

# API via Next.js proxy (needs session cookie after login)
$session = New-Object Microsoft.PowerShell.Commands.WebRequestSession
Invoke-WebRequest "http://127.0.0.1:3000/login" -Method POST -Body @{username="admin";password="admin"} -WebSession $session | Out-Null
(Invoke-WebRequest "http://127.0.0.1:3000/api/go?p=/dashboard&format=json" -WebSession $session).Content
```

## After code changes

1. `go test ./...`
2. `cd frontend && npm run build` (if UI changed)
3. Restart via `start.ps1`
4. Hard refresh browser (Ctrl+Shift+R)
5. Re-run checklist

## CI

GitHub Actions `.github/workflows/ci.yml` runs `go test` + `go build`. Frontend build can be added separately.

## Ports reference

| URL | Purpose |
|-----|---------|
| `http://localhost:3000` | **Primary UI** |
| `http://localhost:8080` | API, beacons, WebSocket, legacy HTML |