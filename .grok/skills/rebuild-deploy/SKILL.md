---
name: rebuild-deploy
description: Rebuild ForgeC2 Next.js frontend and Go API server, then restart both on Windows
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: deploy
---

## When to use

Use after changing Go handlers/API, Next.js pages (`frontend/src/`), or legacy embedded templates.

## Architecture

| Service | Port | Build |
|---------|------|-------|
| Go API | 8080 | `go build -o forgec2-server.exe ./cmd/server` |
| Next.js UI | 3000 | `cd frontend && npm run build` |

## Windows — full restart (recommended)

```powershell
powershell -ExecutionPolicy Bypass -File .\start.ps1
```

`start.ps1` builds Go → starts API → builds Next.js → starts UI on `:3000`.

## Manual steps

```powershell
# 1. Build Go API
go build -o forgec2-server.exe ./cmd/server/

# 2. Stop existing processes
Get-Process forgec2-server,node -ErrorAction SilentlyContinue | Stop-Process -Force

# 3. Start Go API
Start-Process .\forgec2-server.exe -ArgumentList "-config config.yaml" -WindowStyle Hidden

# 4. Build + start Next.js
cd frontend
npm run build
npx next start -p 3000
```

## Development mode

```powershell
# Terminal 1 — API with live reload
go run ./cmd/server -config config.yaml

# Terminal 2 — Next.js hot reload
cd frontend
npm run dev
```

Open **http://localhost:3000** (not `:8080` for UI).

## Legacy template bundles (optional)

Only needed if editing `internal/server/templates/static/` for `:8080` HTML fallback:

```powershell
powershell -ExecutionPolicy Bypass -File build_js.ps1 -SkipCSS
go build -o forgec2-server.exe ./cmd/server
```

## Verify

- `http://127.0.0.1:8080/health` returns `{"status":"ok"}`
- `http://127.0.0.1:3000/login` returns 200
- Dashboard charts load; WebSocket shows Live in top bar