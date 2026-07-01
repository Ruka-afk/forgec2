---
name: e2e-smoke-test
description: Run ForgeC2 end-to-end smoke tests (login, agents, shell, AI, WebSocket). Use for smoke test, verify deployment, or /e2e-smoke-test.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: test
---

## Prerequisites

```bash
make build-all
./forgec2-server -config config.yaml
# or .\server.exe -config config.yaml
```

## Smoke checklist

| # | Step | Expected |
|---|------|----------|
| 1 | `GET /login` | HTTP 200 |
| 2 | `POST /login` admin/admin | Session cookie |
| 3 | `GET /dashboard` | 200, charts load |
| 4 | `GET /agents` | Agent list renders |
| 5 | `GET /api/agents` | JSON with agents array |
| 6 | Implant beacon | `server.log` shows beacon processed |
| 7 | `GET /ai` | AI page, settings work if key set |
| 8 | WebSocket | DevTools WS to `/ws` connects, pong on ping |
| 9 | `go test ./...` | All pass |

## PowerShell quick script

```powershell
$base = "http://127.0.0.1:8080"
$r = Invoke-WebRequest "$base/login" -UseBasicParsing
if ($r.StatusCode -ne 200) { throw "login page failed" }
$session = New-Object Microsoft.PowerShell.Commands.WebRequestSession
Invoke-WebRequest "$base/login" -Method POST -Body @{username="admin";password="admin"} -WebSession $session | Out-Null
(Invoke-WebRequest "$base/api/agents" -WebSession $session).Content
```

## After code changes

1. `make test`
2. `make build-all`
3. Restart server
4. Hard refresh browser (Ctrl+Shift+R)
5. Re-run checklist

## CI

GitHub Actions `.github/workflows/ci.yml` runs `go test` + `go build` on push.